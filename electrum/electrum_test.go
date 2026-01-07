package electrum

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ncodysoftware/eps-go/internal/testdata"
	"github.com/ncodysoftware/eps-go/jsonrpc"
	"github.com/ncodysoftware/eps-go/testutil"
	"github.com/ncodysoftware/eps-go/walletmanager"
	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
)

func TestIntegrationBlockHeader(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	var h [80]byte
	err := c.m.GetBlockHeader(c.t.C, 1, &h)
	assert.Must(t, err)
	r := jsonrpc.Request{
		Method: toJson("blockchain.block.header"),
		Params: toJson([]int{1}),
	}
	exp := jsonrpc.Response{
		Result: toJson(hex.EncodeToString(h[:])),
	}
	jrpcT(t, c.c, &r, &exp)
	jrpcT(t, c.c2, &r, &exp)
}

func TestIntegrationBlockHeaders(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	r := jsonrpc.Request{
		Method: toJson("blockchain.block.headers"),
		Params: toJson([]int{6, 5}),
	}
	headers, err := c.m.GetBlockHeaders(c.t.C, 6, 5)
	assert.Must(t, err)
	sHeaders := make([]byte, 0, 80*5)
	for i := range headers {
		sHeaders = append(sHeaders, headers[i][:]...)
	}
	var result = struct {
		Headers string `json:"hex"`
		Count   int    `json:"count"`
		Max     int    `json:"max"`
	}{
		Count:   5,
		Headers: hex.EncodeToString(sHeaders),
		Max:     2016,
	}
	exp := jsonrpc.Response{
		Result: toJson(result),
	}
	jrpcT(t, c.c, &r, &exp)
	jrpcT(t, c.c2, &r, &exp)
}

func TestIntegrationEstimateFee(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	r := jsonrpc.Request{
		Method: toJson("blockchain.estimatefee"),
		Params: toJson([]int{1}),
	}
	exp := jsonrpc.Response{
		Result: []byte(`0.0001`),
	}
	jrpcT(t, c.c, &r, &exp)
	jrpcT(t, c.c2, &r, &exp)
}

func TestIntegrationHeadersSubscribe(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	r := jsonrpc.Request{
		Method: toJson("blockchain.headers.subscribe"),
		Params: []byte("[]"),
	}
	var res struct {
		Hex    string `json:"hex"`
		Height int    `json:"height"`
	}
	var h [80]byte
	err := c.m.GetTipHeader(c.t.C, &res.Height, &h)
	assert.Must(t, err)
	res.Hex = hex.EncodeToString(h[:])
	exp := jsonrpc.Response{
		Result: toJson(res),
	}
	jrpcT(t, c.c, &r, &exp)
	jrpcT(t, c.c2, &r, &exp)
}

func TestIntegrationScriptHashGetBalance(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	var res struct {
		Conf   uint64 `json:"confirmed"`
		Unconf uint64 `json:"unconfirmed"`
	}
	spkh := deriveScriptHash(1, 0)
	err := c.m.GetScriptHashBalance(c.t.C, &spkh, &res.Conf, &res.Unconf)
	assert.Must(t, err)
	slices.Reverse(spkh[:])
	r := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.get_balance"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	var exp jsonrpc.Response
	exp.Result = toJson(res)
	jrpcT(t, c.c, &r, &exp)
	jrpcT(t, c.c2, &r, &exp)
}

func TestIntegrationScriptHashGetHistory(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	spkh := deriveScriptHash(1, 0)
	hist, err := c.m.GetScriptHashHistory(c.t.C, &spkh)
	assert.Must(t, err)
	slices.Reverse(spkh[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.get_history"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	type txData struct {
		TxHash string `json:"tx_hash"`
		Height int    `json:"height"`
	}
	expD := make([]txData, 0, len(hist))
	for i := range hist {
		slices.Reverse(hist[i].Txid[:])
		expD = append(expD, txData{
			Height: hist[i].Height,
			TxHash: hex.EncodeToString(hist[i].Txid[:]),
		})
	}
	exp := jsonrpc.Response{
		Result: toJson(expD),
	}
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationScriptHashGetMempool(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	spkh := deriveScriptHash(1, 0)
	slices.Reverse(spkh[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.get_mempool"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	exp := jsonrpc.Response{
		Result: []byte(`[]`),
	}
	jrpcT(t, c.c, &req, &exp)
	//jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationScriptHashListUnspent(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	spkh := deriveScriptHash(1, 0)
	utxos, err := c.m.GetScriptHashUnspent(c.t.C, &spkh)
	assert.Must(t, err)
	type utxo struct {
		Height int    `json:"height"`
		TxPos  int    `json:"tx_pos"`
		TxHash string `json:"tx_hash"`
		Value  uint64 `json:"value"`
	}
	expD := make([]utxo, 0, len(utxos))
	for i := range utxos {
		slices.Reverse(utxos[i].Txid[:])
		expD = append(expD, utxo{
			Height: utxos[i].Height,
			TxPos:  utxos[i].TxPos,
			TxHash: hex.EncodeToString(utxos[i].Txid[:]),
			Value:  utxos[i].Satoshi,
		})
	}
	slices.Reverse(spkh[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.listunspent"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	exp := jsonrpc.Response{Result: toJson(expD)}
	jrpcT(t, c.c, &req, &exp)
	//jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationScriptHashSubscribe(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	spkh := deriveScriptHash(1, 0)
	status, err := c.m.GetScriptHashStatus(c.t.C, &spkh, nil)
	assert.Must(t, err)
	slices.Reverse(spkh[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.subscribe"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	exp := jsonrpc.Response{
		Result: toJson(hex.EncodeToString(status)),
	}
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationScriptHashSubscribe2(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	spkh := deriveScriptHash(1, 1)
	slices.Reverse(spkh[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.subscribe"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	exp := jsonrpc.Response{
		Result: []byte(`null`),
	}
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationScriptHashUnsubscribe(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	spkh := deriveScriptHash(1, 0)
	slices.Reverse(spkh[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.scripthash.unsubscribe"),
		Params: toJson([]string{hex.EncodeToString(spkh[:])}),
	}
	exp := jsonrpc.Response{
		Result: []byte(`false`),
	}
	jrpcT(t, c.c, &req, &exp)
	//jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationTransactionBroadcast(t *testing.T) {
	t.Skip("")
	c, cls := setup(t)
	defer cls()
	rawWTX := testutil.MustHexDecode("02000000000101542c6f9dd6e80f6d0fd0c01a8bd58f18069b92185bd05ad0ed17b89af220f9790000000000fdffffff0100301a1e0100000016001425c7dbc175795ab90a6d4902f3a107cf8cfb9bcc0247304402200784b04efb013107af72542ccead049778a3d715de024c1115b1882b4c48956302200859d549774baf8a506007cdbb0d2709f08b69f3df0c1a6d95c2a94a83a48e6d0121038a397b26d42ff3e0c6ab821d15891f549b07e1d8a3d5cd4621265624ac478a3400000000")
	var tx bitcoin.Transaction
	err := tx.Deserialize(bytes.NewReader(rawWTX))
	assert.Must(t, err)
	txid := tx.Txid(nil)
	slices.Reverse(txid[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.transaction.broadcast"),
		Params: toJson([]string{hex.EncodeToString(rawWTX)}),
	}
	exp := jsonrpc.Response{
		Result: toJson(hex.EncodeToString(txid[:])),
	}
	jrpcT(t, c.c, &req, &exp)
	//jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationTransactionGet(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	raw := tSelectSomeTx(c.t)
	var tx bitcoin.Transaction
	err := tx.Deserialize(bytes.NewReader(raw))
	assert.Must(t, err)
	txid := tx.Txid(nil)
	slices.Reverse(txid[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.transaction.get"),
		Params: toJson([]string{hex.EncodeToString(txid[:])}),
	}
	exp := jsonrpc.Response{
		Result: toJson(hex.EncodeToString(raw)),
	}
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationTransactionGetMerkle(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	txD := tSelectSomeTxData(c.t)
	sbranch := make([]string, 0, len(txD.MerkleProof))
	for i := range txD.MerkleProof {
		slices.Reverse(txD.MerkleProof[i][:])
		sbranch = append(
			sbranch, hex.EncodeToString(txD.MerkleProof[i][:]),
		)
	}
	var mRes struct {
		BlockHeight int      `json:"block_height"`
		Pos         int      `json:"pos"`
		Merkle      []string `json:"merkle"`
	}
	mRes.BlockHeight = txD.Height
	mRes.Pos = txD.Pos
	mRes.Merkle = sbranch
	slices.Reverse(txD.Txid[:])
	req := jsonrpc.Request{
		Method: toJson("blockchain.transaction.get_merkle"),
		Params: fmt.Appendf(
			nil,
			`["%s",%d]`,
			hex.EncodeToString(txD.Txid[:]),
			txD.Height,
		),
	}
	exp := jsonrpc.Response{
		Result: toJson(mRes),
	}
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationTransactionIdFromPos(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	mkReq := func(method string, height, txpos int, merkle bool) jsonrpc.Request {
		var req jsonrpc.Request
		req.Method = toJson(method)
		req.Params = toJson([]any{height, txpos, merkle})
		return req
	}
	mkRes := func(txid [32]byte, branch [][32]byte) jsonrpc.Response {
		var res jsonrpc.Response
		var result struct {
			Txid   string   `json:"tx_hash"`
			Merkle []string `json:"merkle"`
		}
		slices.Reverse(txid[:])
		result.Txid = hex.EncodeToString(txid[:])
		result.Merkle = make([]string, 0, len(branch))
		for _, b := range branch {
			slices.Reverse(b[:])
			result.Merkle = append(
				result.Merkle,
				hex.EncodeToString(b[:]),
			)
		}
		res.Result = toJson(result)
		return res
	}
	txD := tSelectSomeTxData(c.t)
	var sbranch []string
	for i := range txD.MerkleProof {
		slices.Reverse(txD.MerkleProof[i][:])
		sbranch = append(
			sbranch, hex.EncodeToString(txD.MerkleProof[i][:]),
		)
	}
	req := mkReq(
		"blockchain.transaction.id_from_pos", txD.Height, txD.Pos, true,
	)
	exp := mkRes(txD.Txid, txD.MerkleProof)
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationTransactionIdFromPos2(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	mkReq := func(method string, height, txpos int, merkle bool) jsonrpc.Request {
		var req jsonrpc.Request
		req.Method = toJson(method)
		req.Params = toJson([]any{height, txpos, merkle})
		return req
	}
	mkRes := func(txid [32]byte) jsonrpc.Response {
		var res jsonrpc.Response
		slices.Reverse(txid[:])
		res.Result = toJson(hex.EncodeToString(txid[:]))
		return res
	}
	txD := tSelectSomeTxData(c.t)
	req := mkReq(
		"blockchain.transaction.id_from_pos",
		txD.Height,
		txD.Pos,
		false,
	)
	exp := mkRes(txD.Txid)
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationMempoolGetFeeHistogram(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	mkReq := func(method string) jsonrpc.Request {
		var req jsonrpc.Request
		req.Method = toJson(method)
		req.Params = []byte(`[]`)
		return req
	}
	mkRes := func(histogram [][2]any) jsonrpc.Response {
		var res jsonrpc.Response
		res.Result = toJson(histogram)
		return res
	}
	req := mkReq("mempool.get_fee_histogram")
	exp := mkRes([][2]any{})
	jrpcT(t, c.c, &req, &exp)
	jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationMempoolGetInfo(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	mkReq := func(method string) jsonrpc.Request {
		var req jsonrpc.Request
		req.Method = toJson(method)
		req.Params = []byte(`[]`)
		return req
	}
	mkRes := func(
		minFee, minRelayFee, incrementalFee float64,
	) jsonrpc.Response {
		var res jsonrpc.Response
		result := struct {
			MinFee              float64 `json:"mempoolminfee"`
			MinRelayTxFee       float64 `json:"minrelaytxfee"`
			IncrementalRelayFee float64 `json:"incrementalrelayfee"`
		}{
			minFee,
			minRelayFee,
			incrementalFee,
		}
		res.Result = toJson(result)
		return res
	}
	req := mkReq("mempool.get_info")
	exp := mkRes(0.00001000, 0.00001000, 0.00001000)
	jrpcT(t, c.c, &req, &exp)
	//jrpcT(t, c.c2, &req, &exp)
}

func TestIntegrationPing(t *testing.T) {
	c, cls := setup(t)
	defer cls()
	r := jsonrpc.Request{
		Method: toJson("server.ping"),
		Params: []byte("[]"),
	}
	exp := jsonrpc.Response{
		Result: []byte("null"),
	}
	jrpcT(t, c.c, &r, &exp)
	jrpcT(t, c.c2, &r, &exp)
}

//type debugHandler struct {
//	cli *jsonrpc.Client
//	notChan <-chan jsonrpc.Notification
//
//	mu sync.Mutex
//}
//
//func (d *debugHandler) OnConnect(connId uint32){
//	d.mu.Lock()
//}
//
//func (d *debugHandler) OnRequest(ctx *jsonrpc.Ctx) error{
//	res, err := d.cli.Send(ctx.Request)
//	if err != nil {
//		return stackerr.Wrap(err)
//	}
//	ctx.Response = res
//	return nil
//}
//
//func (d *debugHandler) OnDisconnect(connId uint32){
//	d.mu.Unlock()
//}
//
//func TestIntegrationProxy(t *testing.T) {
//	tc, cls := testutil.GetTCtx(t)
//	defer cls()
//	notFromServer := make(chan jsonrpc.Notification, 1)
//	client, err := jsonrpc.NewClient(
//		tc.C, tc.L, "127.0.0.1:50004", jsonrpc.ClientOpts{
//			NHandler: func(n *jsonrpc.Notification) {
//				notFromServer <- *n
//			},
//			Flags:    jsonrpc.TLS|jsonrpc.TLSNoVerify,
//		},
//	)
//	assert.Must(t, err)
//	defer client.Close(tc.C)
//	dh := &debugHandler{
//		cli:     client,
//		notChan: notFromServer,
//	}
//	srv, err := jsonrpc.NewServer(tc.C, tc.L, "0.0.0.0:50002", dh)
//	assert.Must(t, err)
//	defer srv.Close(tc.C)
//	select{}
//}

type Ctx struct {
	t  *testutil.TCtx
	c  *jsonrpc.Client
	c2 *jsonrpc.Client
	m  *walletmanager.W
}

func setup(t *testing.T) (Ctx, func()) {
	tc, cls := testutil.GetTCtx(t)
	bcli := bitcoin.NewClient(
		tc.C, tc.Cfg.BTCNodeAddr, tc.L, bitcoin.Regtest,
	)
	err := bcli.Start()
	assert.Must(t, err)
	wallets := []walletmanager.WalletConfig{
		{
			Kind:    scriptpubkey.SK_P2WPKH,
			Reqsigs: 0,
			MasterPubs: []bip32.ExtendedKey{
				testdata.DefaultKeySet.RootAccount,
			},
			Height: 0,
		},
	}
	wm, err := walletmanager.New(
		tc.C, tc.D, tc.L, bcli, wallets, bitcoin.Regtest,
	)
	assert.Must(t, err)
	wm.WaitInit(tc.C)
	ctx, cancel := context.WithCancel(tc.C)
	var wg sync.WaitGroup
	srvStarted := make(chan struct{})
	wg.Go(func() {
		err := ListenAndServe(
			ctx,
			tc.Cfg.ListenAddress,
			tc.L,
			wm,
			func() {
				close(srvStarted)
			},
		)
		if err != nil && errors.Is(err, context.Canceled) {
			return
		}
		assert.Must(t, err)
	})
	<-srvStarted
	ndh := func(n *jsonrpc.Notification) {
		tc.L.Debugf("jsonrpc-cli dropping notification %s", n)
	}
	cli, err := jsonrpc.NewClient(
		tc.C,
		tc.L,
		tc.Cfg.ListenAddress,
		jsonrpc.ClientOpts{
			NHandler: ndh,
		},
	)
	assert.Must(t, err)
	c2Addr := "127.0.0.1:50004"
	cli2, err := jsonrpc.NewClient(
		tc.C,
		tc.L,
		c2Addr,
		jsonrpc.ClientOpts{
			NHandler: ndh,
			Flags:    jsonrpc.TLS | jsonrpc.TLSNoVerify,
		},
	)
	assert.Must(t, err)
	_, err = cli2.Send(jsonrpc.Request{
		Id:      toJson(0),
		JsonRPC: toJson("2.0"),
		Method:  toJson("server.version"),
		Params:  []byte(`["cli v0.0","1.4"]`),
	})
	assert.Must(t, err)
	closeFunc := func() {
		cli.Close(tc.C)
		cli2.Close(tc.C)
		wm.Close(tc.C)
		bcli.Stop()
		cancel()
		wg.Wait()
		cls()
	}
	return Ctx{tc, cli, cli2, wm}, closeFunc
}

func toJson(v any) json.RawMessage {
	j, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return j
}

func jrpcT(
	t *testing.T,
	cli *jsonrpc.Client,
	r *jsonrpc.Request,
	exp *jsonrpc.Response,
) {
	if r.Id == nil {
		r.Id = toJson(0)
	}
	if r.JsonRPC == nil {
		r.JsonRPC = toJson("2.0")
	}
	res, err := cli.Send(*r)
	assert.Must(t, err)
	assert.MustEqual(t, r.Id, res.Id)
	assert.MustEqual(
		t,
		string(exp.Result),
		strings.ReplaceAll(string(res.Result), " ", ""),
	)
	assert.MustEqual(t, string(exp.Error), string(res.Error))
}

func deriveScriptHash(account, index uint32) [32]byte {
	firstRecv, err := bip32.DeriveXpub(
		&testdata.DefaultKeySet.RootAccount,
		[]uint32{account, index},
	)
	if err != nil {
		panic(err)
	}
	spk, err := scriptpubkey.Make(
		scriptpubkey.SK_P2WPKH,
		0,
		[][]byte{firstRecv.Key[:]},
	)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(spk)
}

func tSelectSomeTx(tc *testutil.TCtx) []byte {
	s := `
	SELECT serialized
	FROM tx
	LIMIT 1
	;
	`
	var raw []byte
	err := tc.D.QueryRow(tc.C, s).Scan(&raw)
	if err != nil {
		panic(err)
	}
	return raw
}

type tTxData struct {
	Txid        [32]byte
	Height      int
	BlockHash   [32]byte
	Pos         int
	Raw         []byte
	MerkleProof [][32]byte
}

func tSelectSomeTxData(tc *testutil.TCtx) tTxData {
	s := `
	SELECT 
		tx.txid,
		bh.height,
		tx.blockhash,
		tx.pos,
		tx.serialized,
		tx.merkle_proof
	FROM tx
	JOIN blockheader AS bh
	ON bh.hash = tx.blockhash
	LIMIT 1
	;
	`
	var t tTxData
	txid := bufWrapper(t.Txid[:])
	bh := bufWrapper(t.BlockHash[:])
	var mproof []byte
	err := tc.D.QueryRow(tc.C, s).Scan(
		&txid, &t.Height, &bh, &t.Pos, &t.Raw, &mproof,
	)
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(mproof); i += 32 {
		var h [32]byte
		copy(h[:], mproof[i:i+32])
		t.MerkleProof = append(t.MerkleProof, h)
	}
	return t
}

type bufWrapper []byte

func (self *bufWrapper) Scan(src any) error {
	data, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("bufWrapper: expecting []byte, got %T", src)
	}
	if len(data) != len(*self) {
		return fmt.Errorf("bufWrapper: len of src != len dest")
	}
	copy(*self, data)
	return nil
}
