package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ncodysoftware/eps-go/assert"
	"github.com/ncodysoftware/eps-go/log"
	"github.com/ncodysoftware/eps-go/stackerr"
	"net"
	"os"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestIntegration_ElectrumProtocol(t *testing.T) {
	bcli := setupBitcoind(
		t,
		//withFreshBitcoind,
	)
	srv, cfg, close := runSrv(t)
	listenAddr := cfg.listenAddr
	defer close()
	type test struct {
		name  string
		setup func(t *testing.T, td *epsTD)
	}
	tests := []test{
		{
			name:  "blockchain.block.header",
			setup: testEPBlockchainBlockHeader,
		},
		{
			name:  "blockchain.block.headers",
			setup: testEPBlockchainBlockHeaders,
		},
		{
			name:  "blockchain.estimatefee",
			setup: testEPBlockchainEstimatefee,
		},
		{
			name:  "blockchain.headers.subscribe",
			setup: testEPBlockchainHeadersSubscribe,
		},
		{
			name:  "blockchain.relayfee",
			setup: testEPBlockchainRelayfee,
		},
		{
			name:  "blockchain.scripthash.get_balance",
			setup: testEPBlockchainScripthashGetBalance,
		},
		{
			name:  "blockchain.scripthash.get_history",
			setup: testEPBlockchainScripthashGetHistory,
		},
		{
			name:  "blockchain.scripthash.subscribe",
			setup: testEPBlockchainScripthashSubscribe,
		},
		{
			name:  "blockchain.transaction.broadcast",
			setup: testEPBlockchainTransactionBroadcast,
		},
		{
			name:  "blockchain.transaction.get",
			setup: testEPBlockchainTransactionGet,
		},
		{
			name:  "blockchain.transaction.get_merkle",
			setup: testEPTransactionGetMerkle,
		},
		{
			name:  "blockchain.transaction.id_from_pos",
			setup: testEPBlockchainTransactionIdFromPos,
		},
		{
			name:  "mempool.get_fee_histogram",
			setup: testEPMempoolGetFreeHistogram,
		},
		{
			name:  "server.banner",
			setup: testEPServerBanner,
		},
		{
			name:  "server.donation_address",
			setup: testEPServerDonationAddress,
		},
		{
			name:  "server.peers.subscribe",
			setup: testEPServerPeersSubscribe,
		},
		{
			name:  "server.ping",
			setup: testEPServerPing,
		},
		{
			name:  "server.version",
			setup: testEPServerVersion,
		},
		// NOT IMPLEMENTED
		// electrum-personal-server by Chris Belcher does not implement
		// these methods, so I did not implement them either.
		//
		// blockchain.scripthash.get_mempool
		// blockchain.scripthash.listunspent
		// blockchain.scripthash.unsubscribe
		// server.add_peer
		// server.features
	}
	var (
		conn  net.Conn
		err   error
		idBuf = make([]byte, 0, 4)
		jrpc  = []byte(`"2.0"`)
	)
	for {
		conn, err = net.Dial("tcp", listenAddr)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 100)
		t.Log("retrying connection")
	}
	defer conn.Close()
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			idBuf = idBuf[:0]
			idBuf = fmt.Appendf(idBuf, `%d`, i)
			var td epsTD = epsTD{
				bcli: bcli,
				//
				req: jsonRPCRequest{JsonRPC: jrpc, Id: idBuf},
				expRes: jsonRPCResponse{
					Id: idBuf, JsonRPC: jrpc,
				},
				tname: test.name,
			}
			test.setup(t, &td)
			err := srv.wman._scan()
			assert.Must(t, err)
			res := sendRequest(t, conn, td.req)
			if reflect.DeepEqual(td.expRes, res) {
				return
			}
			t.Fatalf(
				`
		expected:	{"id":%s,"jsonrpc":%s,"result":%s,"error":%s}
		got:		{"id":%s,"jsonrpc":%s,"result":%s,"error":%s}
				`,
				td.expRes.Id,
				td.expRes.JsonRPC,
				td.expRes.Result,
				td.expRes.Error,
				res.Id,
				res.JsonRPC,
				res.Result,
				res.Error,
			)
			assert.MustEqual(t, td.expRes, res)
		})
	}
}

type epsTD struct {
	bcli *btcCli
	//
	req    jsonRPCRequest
	expRes jsonRPCResponse
	tname  string
}

func testEPBlockchainBlockHeader(t *testing.T, td *epsTD) {
	var height int64 = 2
	blockHeader := mustGetBlockHeader(
		t, td.bcli, height, "",
	)
	params := fmt.Appendf(
		[]byte{},
		`[%d,0]`,
		height,
	)
	result := fmt.Appendf(
		[]byte{},
		`"%s"`,
		blockHeader,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = result
}

func testEPBlockchainBlockHeaders(t *testing.T, td *epsTD) {
	blkheader1 := mustGetBlockHeader(t, td.bcli, 0, "")
	blkheader2 := mustGetBlockHeader(t, td.bcli, 1, "")
	startHeight, count, checkpointHeight := 0, 2, 0
	params := fmt.Appendf(
		[]byte{}, `[%d,%d,%d]`,
		startHeight,
		count,
		checkpointHeight,
	)
	result := struct {
		Hex    string `json:"hex"`
		Count  int64  `json:"count"`
		Max    int64  `json:"max"`
		Merkle struct {
			Root   string   `json:"root"`
			Branch []string `json:"branch"`
		} `json:"merkle,omitzero"`
	}{
		Hex:   blkheader1 + blkheader2,
		Count: 2,
		Max:   2016,
	}
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = mustMarshal(t, result)
}

func testEPBlockchainEstimatefee(t *testing.T, td *epsTD) {
	nblocks := 2
	btcperkb := 0.000001
	params := fmt.Appendf([]byte{}, `[%d]`, nblocks)
	result := fmt.Appendf(
		[]byte{}, `%.8f`, btcperkb,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = result
}

func testEPBlockchainHeadersSubscribe(t *testing.T, td *epsTD) {
	blkHeader := mustGetBlockHeader(t, td.bcli, 102, "")
	params := fmt.Appendf([]byte{}, `[]`)
	result := fmt.Appendf(
		[]byte{},
		`{"hex":"%s","height":%d}`,
		blkHeader,
		102,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = result
}

func testEPBlockchainRelayfee(t *testing.T, td *epsTD) {
	params := fmt.Appendf([]byte{}, `[]`)
	relayFeeBTC := 0.000001
	result := fmt.Appendf(
		[]byte{},
		`%.8f`,
		relayFeeBTC,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = result
}

func testEPBlockchainScripthashGetBalance(t *testing.T, td *epsTD) {
	addr := mustGetAddressByLabel(t, td.bcli, "1")
	scriptHash := mustGetScriptHash(t, td.bcli, addr)
	params := fmt.Appendf([]byte{}, `["%s"]`, scriptHash)
	result := fmt.Appendf(
		[]byte{},
		`{"confirmed":4999999000,"unconfirmed":0}`,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = result
}

func testEPBlockchainScripthashGetHistory(t *testing.T, td *epsTD) {
	addr := mustGetAddressByLabel(t, td.bcli, "1")
	scriptHash := mustGetScriptHash(t, td.bcli, addr)
	params := fmt.Appendf([]byte{}, `["%s"]`, scriptHash)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = []byte(`[{"tx_hash":"c62ad7dcf08c1993c5c732030971348327affa1002ceb174283ad7e3c76cf819","height":102}]`)
}

func testEPBlockchainScripthashSubscribe(t *testing.T, td *epsTD) {
	addr := mustGetAddressByLabel(t, td.bcli, "1")
	scriptHash := mustGetScriptHash(t, td.bcli, addr)
	type hashHeight struct {
		hash   string
		height int64
	}
	shStatusData := []hashHeight{
		{
			"c62ad7dcf08c1993c5c732030971348327affa1002ceb174283ad7e3c76cf819",
			102,
		},
	}
	var statusStr []byte
	for _, v := range shStatusData {
		statusStr = fmt.Appendf(
			statusStr, `%s:%d:`,
			v.hash,
			v.height,
		)
	}
	statusHash := sha256.Sum256(statusStr)
	params := fmt.Appendf(
		[]byte{}, `["%s"]`, scriptHash,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = fmt.Appendf(
		[]byte{},
		`"%s"`,
		hex.EncodeToString(statusHash[:]),
	)
}

func testEPBlockchainTransactionBroadcast(t *testing.T, td *epsTD) {
	inputs := []testTxInput{
		{
			Txid: "c62ad7dcf08c1993c5c732030971348327affa1002ceb174283ad7e3c76cf819",
			Vout: 0,
		},
	}
	outputs := []testTxOutput{
		{
			Address:   "bcrt1qe53z23hc328djnkg3t3zmteftnt90x3nf3eudf",
			AmountBTC: 1,
		},
		{
			Address:   "bcrt1q82re7495729d0xdcamcvykqyryf82mtp9lp3mq",
			AmountBTC: 49 - 0.00002000,
		},
	}
	rawTx := mustCreateRawTx(
		t, td.bcli, inputs, outputs,
	)
	signedTx := mustSignRawTx(
		t, td.bcli, rawTx,
	)
	txHash := "56be7378e109dc71833620cd9d0790ba4b0fddbbfc25add01cf1146152e6914d"
	params := fmt.Appendf(
		[]byte{}, `["%s"]`, signedTx,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = fmt.Appendf(
		[]byte{}, `"%s"`, txHash,
	)
}

func testEPBlockchainTransactionGet(t *testing.T, td *epsTD) {
	txHash := "56be7378e109dc71833620cd9d0790ba4b0fddbbfc25add01cf1146152e6914d"
	params := fmt.Appendf([]byte{}, `["%s"]`, txHash)
	testSetMP(&td.req, td.tname, params)
	rawTx := "0200000000010119f86cc7e3d73a2874b1ce0210faaf27833471090332c7c593198cf0dcd72ac60000000000fdffffff0200e1f50500000000160014cd222546f88a8ed94ec88ae22daf295cd6579a3330091024010000001600143a879f54b4f28ad799b8eef0c258041912756d610247304402207fb82e3dd625f1b74e658f3c6db5cca1aba02418973ba4a78bf1e2eca1f1b51c02202552793e883e1494f7a93662b42d58401dff22dde198e79ef11bab5c3f854a060121027206705c63bd1a1831ffa7a888d4d730d823e8df3d94488c6d0dca5d614d9ec500000000"
	td.expRes.Result = fmt.Appendf(
		[]byte{}, `"%s"`, rawTx,
	)
}

func testEPTransactionGetMerkle(t *testing.T, td *epsTD) {
	hash := "c62ad7dcf08c1993c5c732030971348327affa1002ceb174283ad7e3c76cf819"
	height := 102
	params := fmt.Appendf(
		[]byte{}, `["%s",%d]`, hash, height,
	)
	testSetMP(&td.req, td.tname, params)
	td.expRes.Result = []byte(`{"block_height":102,"pos":1,"merkle":["f7f3745bcc89ec093db3f547ea71803493759141efcd8bb70d0b1aa683eb790b"]}`)
}

func testEPBlockchainTransactionIdFromPos(t *testing.T, td *epsTD) {
	height := 102
	pos := 1
	merkle := true
	params := fmt.Appendf(
		[]byte{},
		`[%d,%d,%t]`,
		height,
		pos,
		merkle,
	)
	testSetMP(&td.req, td.tname, params)
	hash := "c62ad7dcf08c1993c5c732030971348327affa1002ceb174283ad7e3c76cf819"
	mrk := "f7f3745bcc89ec093db3f547ea71803493759141efcd8bb70d0b1aa683eb790b"
	result := fmt.Appendf(
		[]byte{},
		`{"tx_hash":"%s","merkle":["%s"]}`,
		hash,
		mrk,
	)
	td.expRes.Result = result
}

func testEPMempoolGetFreeHistogram(t *testing.T, td *epsTD) {
	testSetMP(&td.req, td.tname, nil)
	td.expRes.Result = []byte(`[]`)
}

func testEPServerBanner(t *testing.T, td *epsTD) {
	testSetMP(&td.req, td.tname, nil)
	td.expRes.Result = []byte(`"Welcome to eps-go"`)
}

func testEPServerDonationAddress(t *testing.T, td *epsTD) {
	testSetMP(&td.req, td.tname, nil)
	td.expRes.Result = []byte(`"to be added"`)
}

func testEPServerPeersSubscribe(t *testing.T, td *epsTD) {
	testSetMP(&td.req, td.tname, nil)
	td.expRes.Result = []byte(`[]`)
}

func testEPServerPing(t *testing.T, td *epsTD) {
	testSetMP(&td.req, td.tname, nil)
	td.expRes.Result = []byte(`null`)
}

func testEPServerVersion(t *testing.T, td *epsTD) {
	method := td.tname
	params := fmt.Appendf(
		[]byte{},
		`["testClient %s","%s"]`,
		Version,
		protocolVersion,
	)
	result := fmt.Appendf(
		[]byte{},
		`["eps-go %s","%s"]`,
		Version,
		protocolVersion,
	)
	testSetMP(&td.req, method, params)
	td.expRes.Result = result
}

func runSrv(t *testing.T) (*server, config, func()) {
	cfg := initConfig()
	logger := log.New(log.LVL_DEBUG, "eps-go")
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	ls, err := net.Listen("tcp", cfg.listenAddr)
	must(err)
	listener := ls.(*net.TCPListener)
	bcli := newBtcCli(
		cfg.bitcoindAddr,
		cfg.bitcoindUser,
		cfg.bitcoindPassword,
		cfg.bitcoindWalletPrefix,
	)
	wman := newWalletManager(bcli, logger, cfg.bitcoindWalletPrefix, cfg.descriptors)
	srv := newServer(logger, bcli, wman)
	wg.Go(func() {
		var wg sync.WaitGroup
		for ctx.Err() == nil {
			listener.SetDeadline(time.Now().Add(time.Millisecond * 100))
			conn, err := listener.Accept()
			if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			}
			if err != nil {
				logger.Err(stackerr.Wrap(err))
				continue
			}
			wg.Go(func() {
				//srv.handleConnOld(ctx, conn, cfg.fwdAddr)
				srv.handleConn(ctx, conn)
			})
		}
		stopCtx, cancel := context.WithTimeout(
			context.Background(), time.Second*5,
		)
		defer cancel()
		go func() {
			defer cancel()
			wg.Wait()
		}()
		<-stopCtx.Done()
	})
	return srv, cfg, func() {
		cancel()
		wg.Wait()
	}
}

func sendRequest(
	t *testing.T, conn net.Conn, req jsonRPCRequest,
) jsonRPCResponse {
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	err := enc.Encode(req)
	assert.Must(t, err)
	var (
		resdata json.RawMessage
		check   map[string]json.RawMessage
		res     jsonRPCResponse
	)
	for {
		err = dec.Decode(&resdata)
		assert.Must(t, err)
		// check if is notification
		err = json.Unmarshal(resdata, &check)
		_, ok := check["id"]
		if !ok {
			continue
		}
		break
	}
	err = json.Unmarshal(resdata, &res)
	assert.Must(t, err)
	return res
}

func mustGetBlockHeader(
	t *testing.T, b *btcCli, height int64, hash string,
) string {
	var (
		blkHeader string
	)
	if hash == "" {
		res, err := b.call(
			"getblockhash",
			fmt.Appendf([]byte{}, `[%d]`, height),
		)
		noError(t, err, res)
		mustUnmarshal(t, res.Result, &hash)
	}
	res, err := b.call(
		"getblockheader",
		fmt.Appendf([]byte{}, `["%s",%t]`, hash, false),
	)
	noError(t, err, res)
	mustUnmarshal(t, res.Result, &blkHeader)
	return blkHeader
}

func mustGetAddressByLabel(
	t *testing.T, b *btcCli, label string,
) string {
	res, err := b.call(
		"getaddressesbylabel",
		fmt.Appendf([]byte{}, `["%s"]`, label),
	)
	noError(t, err, res)
	idx := bytes.Index(res.Result, []byte(`"`))
	assert.MustEqual(t, 1, idx)
	idx2 := bytes.Index(res.Result[2:], []byte(`"`))
	assert.MustEqual(t, true, idx2 > 0)
	return string(res.Result[idx+1 : idx+1+idx2])
}

func mustGetScriptHash(t *testing.T, b *btcCli, addr string) string {
	reverse := func(b []byte) []byte {
		r := make([]byte, 0, len(b))
		for i := len(b) - 1; i >= 0; i-- {
			r = append(r, b[i])
		}
		return r
	}
	var addrInfo struct {
		ScriptPub string `json:"scriptPubKey"`
	}
	res, err := b.call(
		"getaddressinfo",
		fmt.Appendf([]byte{}, `["%s"]`, addr),
	)
	noError(t, err, res)
	mustUnmarshal(t, res.Result, &addrInfo)
	spbkBytes, err := hex.DecodeString(addrInfo.ScriptPub)
	assert.Must(t, err)
	hash := sha256.Sum256(spbkBytes)
	rhash := reverse(hash[:])
	hexHash := hex.EncodeToString(rhash)
	return hexHash
}

type testTxInput struct {
	Txid     string `json:"txid"`
	Vout     int64  `json:"vout"`
	Sequence int64  `json:"sequence,omitempty"`
}

type testTxOutput struct {
	Address   string
	AmountBTC float64
}

func mustCreateRawTx(
	t *testing.T,
	b *btcCli,
	inputs []testTxInput,
	outputs []testTxOutput,
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

func mustSignRawTx(
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

func testSetMP(rq *jsonRPCRequest, method string, params []byte) {
	rq.Method = fmt.Appendf([]byte{}, `"%s"`, method)
	rq.Params = params
}

func noError(t *testing.T, err error, res jsonRPCResponse) {
	assert.Must(t, err)
	if len(res.Error) == 0 || bytes.Equal([]byte(`null`), res.Error) {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	assert.MustEqual(t, true, ok)
	t.Fatalf(
		"call from: %s:%d: rpc error: %s",
		file, line, string(res.Error),
	)
}
