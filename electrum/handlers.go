package electrum

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ncodysoftware/eps-go/jsonrpc"
	"ncody.com/ncgo.git/stackerr"
)

func (m *mux) defaultHandlers() map[string]func(ctx *jsonrpc.Ctx) error {
	return map[string]func(ctx *jsonrpc.Ctx) error{
		"blockchain.block.header":           m.blockHeaderHandler,
		"blockchain.block.headers":          m.blockHeadersHandler,
		"blockchain.estimatefee":            m.estimateFeeHandler,
		"blockchain.headers.subscribe":      m.headersSubscribeHandler,
		"blockchain.scripthash.get_balance": m.scriptHashGetBalanceHandler,
		"blockchain.scripthash.get_history": m.scriptHashGetHistoryHandler,
		"blockchain.scripthash.get_mempool": m.scriptHashGetMempoolHandler,
		"blockchain.scripthash.listunspent": m.scriptHashListUnspentHandler,
		"blockchain.scripthash.subscribe":   m.scriptHashSubscribeHandler,
		"blockchain.scripthash.unsubscribe": m.scriptHashUnsubscribeHandler,
		"blockchain.transaction.broadcast":  m.transactionBroadcastHandler,
		//"blockchain.transaction.broadcast_package": nil,
		"blockchain.transaction.get":         m.transactionGetHandler,
		"blockchain.transaction.get_merkle":  m.transactionGetMerkleHandler,
		"blockchain.transaction.id_from_pos": m.transactionIdFromPosHandler,
		"blockchain.relayfee":                m.blockchainRelayFeeHandler,
		"mempool.get_fee_histogram":          m.mempoolGetFeeHistogramHandler,
		"mempool.get_info":                   m.mempoolGetInfo,
		//"server.add_peer": nil,
		"server.banner":           m.serverBannerHandler,
		"server.donation_address": m.serverDonationAddressHandler,
		//"server.features": nil,
		"server.peers.subscribe": m.serverPeersSubscribeHandler,
		"server.ping":            m.pingHandler,
		"server.version":         m.serverVersionHandler,
	}
}

func (m *mux) blockHeaderHandler(ctx *jsonrpc.Ctx) error {
	var (
		params []int
		height int
	)
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(params) == 0 {
		return stackerr.Wrap(errBadParams)
	}
	height = params[0]
	var header [80]byte
	err = m.w.GetBlockHeader(m.ctx, height, &header)
	if err != nil {
		return stackerr.Wrap(err)
	}
	ctx.Response.Result = fmt.Appendf(
		nil, `"%s"`, hex.EncodeToString(header[:]),
	)
	return nil
}

func (m *mux) blockHeadersHandler(ctx *jsonrpc.Ctx) error {
	var result = struct {
		Hex   string `json:"hex"`
		Count int    `json:"count"`
		Max   int    `json:"max"`
	}{}
	const max = 2016
	var (
		startHeight, count int
		params             []int
	)
	err := json.Unmarshal(ctx.Request.Params, &params)
	if len(params) < 2 {
		return fmt.Errorf("protocol violation")
	}
	startHeight, count = params[0], params[1]
	headers, err := m.w.GetBlockHeaders(m.ctx, startHeight, min(count, max))
	if err != nil {
		return stackerr.Wrap(err)
	}
	serializedHeaders := m.bGet()
	defer m.bPut(serializedHeaders)
	bClear(&serializedHeaders)
	for i := range headers {
		serializedHeaders = append(serializedHeaders, headers[i][:]...)
	}
	result.Count = len(headers)
	result.Max = max
	result.Hex = hex.EncodeToString(serializedHeaders)
	ctx.Response.Result, err = json.Marshal(result)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) estimateFeeHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = []byte(`0.0001`)
	return nil
}

func (m *mux) headersSubscribeHandler(ctx *jsonrpc.Ctx) error {
	var r struct {
		Hex    string `json:"hex"`
		Height int    `json:"height"`
	}
	var h [80]byte
	err := m.w.GetTipHeader(m.ctx, &r.Height, &h)
	if err != nil {
		return stackerr.Wrap(err)
	}
	connId := ctx.ConnId
	m.w.HeadersSubscribe(connId, func(height int, header [80]byte) {
		err := ctx.Notifier.Notify(connId, &jsonrpc.Notification{
			Method: []byte(`"blockchain.headers.subscribe"`),
			Params: fmt.Appendf(
				nil,
				`[{"height":%d,"hex":"%s"}]`,
				height,
				hex.EncodeToString(header[:]),
			),
		})
		if err != nil {
			m.log.Err(stackerr.Wrap(err))
		}
	})
	r.Hex = hex.EncodeToString(h[:])
	ctx.Response.Result, err = json.Marshal(r)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) scriptHashGetBalanceHandler(ctx *jsonrpc.Ctx) error {
	sh, err := parseScriptHash(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	var res struct {
		Conf   uint64 `json:"confirmed"`
		Unconf uint64 `json:"unconfirmed"`
	}
	err = m.w.GetScriptHashBalance(m.ctx, &sh, &res.Conf, &res.Unconf)
	if err != nil {
		return stackerr.Wrap(err)
	}
	ctx.Response.Result, err = json.Marshal(res)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) scriptHashGetHistoryHandler(ctx *jsonrpc.Ctx) error {
	sh, err := parseScriptHash(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	type txData struct {
		TxHash string `json:"tx_hash"`
		Height int    `json:"height"`
	}
	var res []txData
	hist, err := m.w.GetScriptHashHistory(m.ctx, &sh)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i := range hist {
		slices.Reverse(hist[i].Txid[:])
		res = append(
			res,
			txData{
				Height: hist[i].Height,
				TxHash: hex.EncodeToString(hist[i].Txid[:]),
			},
		)
	}
	ctx.Response.Result, err = json.Marshal(res)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) scriptHashGetMempoolHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = []byte(`[]`)
	return nil
}

func (m *mux) scriptHashListUnspentHandler(ctx *jsonrpc.Ctx) error {
	sh, err := parseScriptHash(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	unspentD, err := m.w.GetScriptHashUnspent(m.ctx, &sh)
	type utxo struct {
		Height int    `json:"height"`
		TxPos  int    `json:"tx_pos"`
		TxHash string `json:"tx_hash"`
		Value  uint64 `json:"value"`
	}
	res := make([]utxo, 0, len(unspentD))
	for i := range unspentD {
		slices.Reverse(unspentD[i].Txid[:])
		res = append(res, utxo{
			Height: unspentD[i].Height,
			TxPos:  unspentD[i].TxPos,
			TxHash: hex.EncodeToString(unspentD[i].Txid[:]),
			Value:  unspentD[i].Satoshi,
		})
	}
	ctx.Response.Result, err = json.Marshal(res)
	return nil
}

func (m *mux) scriptHashSubscribeHandler(ctx *jsonrpc.Ctx) error {
	sh, err := parseScriptHash(ctx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	buf := m.bGet()
	defer m.bPut(buf)
	bClear(&buf)
	status, err := m.w.GetScriptHashStatus(m.ctx, &sh, &buf)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if status == nil {
		ctx.Response.Result = []byte(`null`)
	} else {
		ctx.Response.Result = fmt.Appendf(
			nil, `"%s"`, hex.EncodeToString(status),
		)
	}
	connId := ctx.ConnId
	m.w.ScriptHashSubscribe(connId, sh, func(status2 [32]byte) {
		sh1 := sh
		slices.Reverse(sh1[:])
		shHex := hex.EncodeToString(sh1[:])
		statusHex := hex.EncodeToString(status2[:])
		err := ctx.Notifier.Notify(connId, &jsonrpc.Notification{
			Method: []byte(`"blockchain.scripthash.subscribe"`),
			Params: fmt.Appendf(
				nil, `["%s","%s"]`, shHex, statusHex,
			),
		})
		if err != nil {
			m.log.Err(stackerr.Wrap(err))
		}
	})
	return nil
}

func (m *mux) scriptHashUnsubscribeHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = []byte("false")
	return nil
}

func (m *mux) transactionBroadcastHandler(ctx *jsonrpc.Ctx) error {
	var params []string
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(params) < 1 {
		return fmt.Errorf("bad param len")
	}
	rawTx, err := hex.DecodeString(params[0])
	if err != nil {
		return stackerr.Wrap(err)
	}
	buf := m.bGet()
	defer m.bPut(buf)
	bClear(&buf)
	txid, err := m.w.BroadcastTX(m.ctx, rawTx, &buf)
	if err != nil {
		return stackerr.Wrap(err)
	}
	slices.Reverse(txid[:])
	ctx.Response.Result, err = json.Marshal(hex.EncodeToString(txid[:]))
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) transactionGetHandler(ctx *jsonrpc.Ctx) error {
	var params []json.RawMessage
	var txidStr string
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(params) < 1 || len(params[0]) != 64+2 {
		return fmt.Errorf("bad param len")
	}
	err = json.Unmarshal(params[0], &txidStr)
	if err != nil {
		return stackerr.Wrap(err)
	}
	txidS, err := hex.DecodeString(txidStr)
	if err != nil {
		return stackerr.Wrap(err)
	}
	slices.Reverse(txidS)
	var txid [32]byte
	copy(txid[:], txidS)
	raw, err := m.w.GetRawTx(m.ctx, &txid)
	if err != nil {
		return stackerr.Wrap(err)
	}
	ctx.Response.Result, err = json.Marshal(hex.EncodeToString(raw))
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) transactionGetMerkleHandler(ctx *jsonrpc.Ctx) error {
	var (
		params []json.RawMessage
		txHash string
		height int
	)
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(params) < 2 || len(params[0]) != 64+2 {
		return fmt.Errorf("bad params")
	}
	err = json.Unmarshal(params[0], &txHash)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = json.Unmarshal(params[1], &height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	txidB, err := hex.DecodeString(txHash)
	if err != nil {
		return stackerr.Wrap(err)
	}
	slices.Reverse(txidB)
	var txid [32]byte
	copy(txid[:], txidB[:])
	buf := m.bGet()
	defer m.bPut(buf)
	bClear(&buf)
	merkle, err := m.w.GetTransactionMerkle(m.ctx, &txid)
	var mRes struct {
		BlockHeight int      `json:"block_height"`
		Pos         int      `json:"pos"`
		Merkle      []string `json:"merkle"`
	}
	mRes.BlockHeight = height
	mRes.Pos = merkle.Pos
	mRes.Merkle = make([]string, 0, len(mRes.Merkle))
	for i := range merkle.Merkle {
		slices.Reverse(merkle.Merkle[i][:])
		mRes.Merkle = append(
			mRes.Merkle, hex.EncodeToString(merkle.Merkle[i][:]),
		)
	}
	ctx.Response.Result, err = json.Marshal(mRes)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) transactionIdFromPosHandler(ctx *jsonrpc.Ctx) error {
	height, txpos, merkle := 0, 0, false
	params := []any{&height, &txpos, &merkle}
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(params) < 2 {
		return fmt.Errorf("bad params")
	}
	buf := m.bGet()
	defer m.bPut(buf)
	bClear(&buf)
	mrk, err := m.w.GetTransactionMerkleFromPos(
		m.ctx, height, txpos,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	var result struct {
		Txid   string   `json:"tx_hash"`
		Merkle []string `json:"merkle"`
	}
	slices.Reverse(mrk.Txid[:])
	result.Txid = hex.EncodeToString(mrk.Txid[:])
	result.Merkle = make([]string, 0, len(mrk.Merkle))
	for i := range mrk.Merkle {
		m := &mrk.Merkle[i]
		slices.Reverse(m[:])
		result.Merkle = append(result.Merkle, hex.EncodeToString(m[:]))
	}
	if !merkle {
		ctx.Response.Result = fmt.Appendf(nil, `"%s"`, result.Txid)
		return nil
	}
	ctx.Response.Result, err = json.Marshal(result)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) blockchainRelayFeeHandler(ctx *jsonrpc.Ctx) error {
	// TODO: real relay fee
	ctx.Response.Result = []byte(`0.00001000`)
	return nil
}

func (m *mux) mempoolGetFeeHistogramHandler(ctx *jsonrpc.Ctx) error {
	// TODO: implement real histogram
	var err error
	// [2]any should be [satVbyte float64, cummulativeVirtualSize int]
	result := [][2]any{}
	ctx.Response.Result, err = json.Marshal(result)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) mempoolGetInfo(ctx *jsonrpc.Ctx) error {
	var err error
	result := struct {
		MinFee              float64 `json:"mempoolminfee"`
		MinRelayTxFee       float64 `json:"minrelaytxfee"`
		IncrementalRelayFee float64 `json:"incrementalrelayfee"`
	}{
		0.00001000,
		0.00001000,
		0.00001000,
	}
	ctx.Response.Result, err = json.Marshal(result)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *mux) serverBannerHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = []byte(`"eps-go"`)
	return nil
}

func (m *mux) serverDonationAddressHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = []byte(`"none"`)
	return nil
}

func (m *mux) serverPeersSubscribeHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = []byte(`[]`)
	return nil
}

func (m *mux) pingHandler(ctx *jsonrpc.Ctx) error {
	ctx.Response.Result = nullResult
	return nil
}

func (m *mux) serverVersionHandler(ctx *jsonrpc.Ctx) error {
	var clientName, protoVersion string
	params := []any{&clientName, &protoVersion}
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if protoVersion != "1.4" {
		return fmt.Errorf("bad client version: %s", protoVersion)
	}
	ctx.Response.Result = []byte(`["eps-go", "1.4"]`)
	return nil
}

func parseScriptHash(ctx *jsonrpc.Ctx) ([32]byte, error) {
	var (
		params []string
		sh     [32]byte
	)
	err := json.Unmarshal(ctx.Request.Params, &params)
	if err != nil {
		return sh, stackerr.Wrap(err)
	}
	if len(params) < 1 || len(params[0]) != 64 {
		return sh, fmt.Errorf("protocol violation")
	}
	shs, err := hex.DecodeString(params[0])
	if err != nil {
		return sh, stackerr.Wrap(err)
	}
	slices.Reverse(shs)
	copy(sh[:], shs)
	return sh, nil
}
