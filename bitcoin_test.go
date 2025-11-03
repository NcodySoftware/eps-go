package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/ncodysoftware/eps-go/assert"
	"github.com/ncodysoftware/eps-go/stackerr"
)

func setupBitcoindInitialState(t *testing.T, bcli *btcCli, tcfg testingConfig) {
	cmd := exec.Command("bash", "-c", tcfg.startFreshBitcoind)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		t.Fatal(
			errors.Join(
				stackerr.Wrap(err),
				fmt.Errorf("%s", stderrBuf.String()),
			),
		)
	}
	deadline := time.Now().Add(time.Second * 60)
	for time.Until(deadline) > 0 {
		res, _ := bcli.call("getblockchaininfo", nil)
		if len(res.Result) > 0 {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	crtDescWalParams := struct {
		Wn string `json:"wallet_name"`
		Bl bool   `json:"blank"`
		D  bool   `json:"descriptors"`
	}{
		"eps_desc", true, true,
	}
	type descReq struct {
		D  string   `json:"desc"`
		A  bool     `json:"active"`
		R  []uint64 `json:"range"`
		Ni int64    `json:"next_index"`
		T  string   `json:"timestamp"`
		I  bool     `json:"internal"`
	}
	descRecv := "wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/0/*)#f03g2f34"
	descChg := "wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/1/*)#cm5fhupd"
	impDescParams := struct {
		R []descReq `json:"requests"`
	}{
		R: []descReq{
			{
				D:  descRecv,
				A:  true,
				R:  []uint64{0, 0},
				Ni: 0,
				T:  "now",
				I:  false,
			},
			{
				D:  descChg,
				A:  true,
				R:  []uint64{0, 0},
				Ni: 0,
				T:  "now",
				I:  true,
			},
		},
	}
	res, err := bcli.call("createwallet", mustMarshal(t, crtDescWalParams))
	noError(t, err, res)
	res, err = bcli.call("importdescriptors", mustMarshal(t, impDescParams))
	noError(t, err, res)

	mustGetNewAddress := func(
		t *testing.T, b *btcCli,
	) string {
		res, err := b.call(
			"getnewaddress",
			fmt.Appendf([]byte{}, `[]`),
		)
		noError(t, err, res)
		var addr string
		mustUnmarshal(t, res.Result, &addr)
		return addr
	}
	mustSetLabel := func(
		t *testing.T, b *btcCli, address, label string,
	) {
		res, err := b.call(
			"setlabel",
			fmt.Appendf([]byte{}, `["%s","%s"]`, address, label),
		)
		noError(t, err, res)
	}
	mustGenerateToAddress := func(
		t *testing.T, b *btcCli, addr string, nblocks int64,
	) {
		res, err := b.call(
			"generatetoaddress",
			fmt.Appendf(
				[]byte{},
				`[%d,"%s"]`,
				nblocks,
				addr,
			),
		)
		noError(t, err, res)
	}
	type txInput struct {
		Txid     string `json:"txid"`
		Vout     int64  `json:"vout"`
		Sequence int64  `json:"sequence,omitempty"`
	}
	type txOutput struct {
		Address   string
		AmountBTC float64
	}
	mustCreateRawTx := func(
		t *testing.T,
		b *btcCli,
		inputs []txInput,
		outputs []txOutput,
	) string {
		var (
			outputsJ []byte
			rawTx    string
		)
		outputsJ = fmt.Append(outputsJ, "[")
		for i, v := range outputs {
			outputsJ = fmt.Appendf(
				outputsJ, `{"%s":"%f"}`, v.Address, v.AmountBTC,
			)
			if i < len(outputs)-1 {
				outputsJ = fmt.Appendf(outputsJ, `,`)
			}
		}
		outputsJ = fmt.Append(outputsJ, "]")
		params := fmt.Appendf(
			[]byte{}, `[%s,%s,0,true]`,
			mustMarshal(t, inputs),
			outputsJ,
		)
		res, err := b.call(
			"createrawtransaction",
			params,
		)
		noError(t, err, res)
		mustUnmarshal(t, res.Result, &rawTx)
		return rawTx
	}
	mustSignRawTx := func(
		t *testing.T, b *btcCli, rawTx string,
	) string {
		var signedTx struct {
			Hex      string `json:"hex"`
			Complete bool   `json:"complete"`
		}
		params := fmt.Appendf(
			[]byte{}, `["%s"]`,
			rawTx,
		)
		res, err := b.call(
			"signrawtransactionwithwallet",
			params,
		)
		noError(t, err, res)
		mustUnmarshal(t, res.Result, &signedTx)
		assert.MustEqual(t, true, signedTx.Complete)
		return signedTx.Hex
	}
	addr := mustGetNewAddress(t, bcli)
	mustSetLabel(t, bcli, addr, "0")
	mustGenerateToAddress(t, bcli, addr, 101)
	addr1 := mustGetNewAddress(t, bcli)
	mustSetLabel(t, bcli, addr1, "1")
	unsigned := mustCreateRawTx(
		t,
		bcli,
		[]txInput{
			{
				Txid:     "433f2be7a1b4ad349a28bfbeb196bf5dbbebde133bc8d76d77e5f0cccd114b4d",
				Vout:     0,
				Sequence: 0,
			},
		},
		[]txOutput{
			{
				Address:   addr1,
				AmountBTC: 50 - 0.00001,
			},
		},
	)
	signed := mustSignRawTx(t, bcli, unsigned)
	txHash, err := bcli.sendRawTransaction(signed)
	assert.Must(t, err)
	assert.MustEqual(
		t,
		"c62ad7dcf08c1993c5c732030971348327affa1002ceb174283ad7e3c76cf819",
		txHash,
	)
	mustGenerateToAddress(t, bcli, addr, 1)

	cmd = exec.Command("bash", "-c", tcfg.saveBitcoindState)
	stderrBuf.Reset()
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	if err != nil {
		t.Fatal(
			errors.Join(
				stackerr.Wrap(err),
				fmt.Errorf("%s", stderrBuf.String()),
			),
		)
	}
	deadline = time.Now().Add(time.Second * 60)
	for time.Until(deadline) > 0 {
		res, _ := bcli.call("getblockchaininfo", nil)
		if len(res.Result) > 0 {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func restoreBitcoindState(t *testing.T, bcli *btcCli, tcfg testingConfig) {
	cmd := exec.Command("bash", "-c", tcfg.loadBitcoindSavedState)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		t.Fatal(
			errors.Join(
				stackerr.Wrap(err),
				fmt.Errorf("%s", stderrBuf.String()),
			),
		)
	}
	deadline := time.Now().Add(time.Second * 60)
	for time.Until(deadline) > 0 {
		res, _ := bcli.call("getblockchaininfo", nil)
		if len(res.Result) > 0 {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	deadline = time.Now().Add(time.Second * 60)
	for time.Until(deadline) > 0 {
		res, err := bcli.call("loadwallet", []byte(`["eps_desc"]`))
		assert.Must(t, err)
		if !bytes.Contains(res.Error, []byte(`already loaded`)) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
}

type initOpts struct {
	freshBitcoind bool
}

func withFreshBitcoind(o *initOpts) {
	o.freshBitcoind = true
}

func setupBitcoind(t *testing.T, opts ...func(*initOpts)) *btcCli {
	var iopts initOpts
	for _, fn := range opts {
		fn(&iopts)
	}
	cfg := initConfig()
	tcfg := initTestingConfig(t)
	bcli := newBtcCli(
		cfg.bitcoindAddr,
		cfg.bitcoindUser,
		cfg.bitcoindPassword,
		cfg.bitcoindWalletPrefix,
	)
	if iopts.freshBitcoind {
		setupBitcoindInitialState(t, bcli, tcfg)
	}
	restoreBitcoindState(t, bcli, tcfg)
	return bcli
}

func mustMarshal(t *testing.T, v any) []byte {
	b, err := json.Marshal(v)
	assert.Must(t, err)
	return b
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	err := json.Unmarshal(data, v)
	assert.Must(t, err)
}

func TestIntegration_bitcoin(t *testing.T) {
	bcli := setupBitcoind(t)
	res, err := bcli.call("getblockchaininfo", nil)
	assert.Must(t, err)
	assert.MustEqual(t, true, len(res.Result) > 0)
}

func TestIntegration_deriveAddresses(t *testing.T) {
	bcli := setupBitcoind(t)
	receiveDesc := "wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/0/*)#f03g2f34"
	changeDesc := "wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/1/*)#cm5fhupd"
	expRcv := []string{
		"bcrt1q56fzm5fmj7wtl5cs2n9ez09mw5yxq9n4ahwv7n",
		"bcrt1qmrd57csmzkfcvkrl8msczweesztsrn7xnzfvnd",
		"bcrt1qe53z23hc328djnkg3t3zmteftnt90x3nf3eudf",
	}
	expChg := []string{
		"bcrt1qyhrahst409dtjzndfyp08gg8e7x0hx7vr867nm",
		"bcrt1q82re7495729d0xdcamcvykqyryf82mtp9lp3mq",
		"bcrt1qjm5vzwkkyc9q796kynykmn3qgyns65vrzw3egl",
	}

	rcvAddrs, err := bcli.deriveAddresses(receiveDesc, 0, 2)
	assert.Must(t, err)
	chgAddrs, err := bcli.deriveAddresses(changeDesc, 0, 2)
	assert.Must(t, err)
	assert.MustEqual(t, expRcv, rcvAddrs)
	assert.MustEqual(t, expChg, chgAddrs)
}
