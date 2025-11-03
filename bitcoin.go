package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ncodysoftware/eps-go/stackerr"
	"net/http"
	"strings"
	"sync"
)

var (
	ErrInsufficientData    = errors.New("bitcoind: insufficient data")
	ErrWalletAlreadyLoaded = errors.New("bitcoind: wallet already loaded")
	ErrWalletDoesNotExist  = errors.New("bitcoind: wallet does not exist")
	ErrUnreachable         = errors.New("unreachable")
)

type btcCli struct {
	baseURI, username, password string
	mu                          sync.Mutex
	hc                          http.Client
	count                       int64
	defaultWallet               string
}

func newBtcCli(baseURI, username, password string, wallet string) *btcCli {
	b := &btcCli{
		baseURI:       baseURI,
		username:      username,
		password:      password,
		defaultWallet: wallet,
	}
	//	if wallet != "" {
	//		b.baseURI = baseURI + "/wallet/" + wallet
	//	}
	return b
}

func (b *btcCli) getBlockHash(height int64) (string, error) {
	var blockHash string
	r, err := b.call(
		"getblockhash",
		fmt.Appendf([]byte{}, `[%d]`, height),
	)
	err = checkRPCErr(err, r)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	err = json.Unmarshal(r.Result, &blockHash)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return blockHash, nil
}

func (b *btcCli) getBlockHeader(blockHash string) (string, error) {
	const verbose = false
	var blockHeader string
	r, err := b.call(
		"getblockheader",
		fmt.Appendf([]byte{}, `["%s",%t]`, blockHash, verbose),
	)
	err = checkRPCErr(err, r)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	err = json.Unmarshal(r.Result, &blockHeader)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return blockHeader, nil
}

type btcBlockHeader struct {
	PreviousBlockHash string `json:"previousblockhash"`
}

func (b *btcCli) getBlockHeaderVerbose(
	blockHash string,
) (btcBlockHeader, error) {
	const verbose = true
	var bh btcBlockHeader
	r, err := b.call(
		"getblockheader",
		fmt.Appendf([]byte{}, `["%s",%t]`, blockHash, verbose),
	)
	err = checkRPCErr(err, r)
	if err != nil {
		return bh, stackerr.Wrap(err)
	}
	err = json.Unmarshal(r.Result, &bh)
	if err != nil {
		return bh, stackerr.Wrap(err)
	}
	return bh, nil
}

type blockChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
	Pruned        bool   `json:"pruned"`
	PruneHeight   int64  `json:"pruneheight"`
}

func (b *btcCli) getBlockChainInfo() (blockChainInfo, error) {
	const verbose = false
	var res blockChainInfo
	r, err := b.call(
		"getblockchaininfo",
		nil,
	)
	err = checkRPCErr(err, r)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	err = json.Unmarshal(r.Result, &res)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	return res, nil
}

func (b *btcCli) estimateSmartFee(nblocks int64) (float64, error) {
	var result struct {
		F float64  `json:"feerate"`
		E []string `json:"errors"`
	}
	res, err := b.call(
		"estimatesmartfee", fmt.Appendf(nil, `[%d]`, nblocks),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &result)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	if len(result.E) != 0 && strings.Contains(
		result.E[0], "Insufficient data or no feerate found",
	) {
		return 0, ErrInsufficientData
	}
	if len(result.E) != 0 {
		return 0, stackerr.Wrap(fmt.Errorf(`%s`, result.E[0]))
	}
	return result.F, nil
}

type mempoolInfo struct {
	Loaded              bool    `json:"loaded"`
	Size                int64   `json:"size"`
	Bytes               int64   `json:"bytes"`
	Usage               int64   `json:"usage"`
	TotalFee            float64 `json:"total_fee"`
	MaxMempool          int64   `json:"maxmempool"`
	MempoolMinFee       float64 `json:"mempoolminfee"`
	MinRelayTXFee       float64 `json:"minrelaytxfee"`
	IncrementalRelayFee float64 `json:"incrementalrelayfee"`
	UnbroadcastCount    int64   `json:"unbroadcastcount"`
	FullRBF             bool    `json:"fullrbf"`
}

func (b *btcCli) getMempoolInfo() (mempoolInfo, error) {
	var r mempoolInfo
	res, err := b.call("getmempoolinfo", nil)
	err = checkRPCErr(err, res)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &r)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	return r, nil
}

func (b *btcCli) sendRawTransaction(rawtx string) (string, error) {
	var txHash string
	res, err := b.call(
		"sendrawtransaction", fmt.Appendf(nil, `["%s"]`, rawtx),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &txHash)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return txHash, nil
}

type block struct {
	Hash       string   `json:"hash"`
	Height     int64    `json:"height"`
	MerkleRoot string   `json:"merkleroot"`
	Tx         []string `json:"tx"`
}

func (b *btcCli) getBlock(bhash string) (block, error) {
	var blockd block
	res, err := b.call("getblock", fmt.Appendf(nil, `["%s"]`, bhash))
	err = checkRPCErr(err, res)
	if err != nil {
		return blockd, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &blockd)
	if err != nil {
		return blockd, stackerr.Wrap(err)
	}
	return blockd, nil
}

func (b *btcCli) getTransaction(wallet, txid string) (string, error) {
	var tx struct {
		Hex string `json:"hex"`
	}
	res, err := b.walletCall(
		wallet, "gettransaction", fmt.Appendf(nil, `["%s"]`, txid),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &tx)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return tx.Hex, nil
}

type transaction struct {
	address       string
	fee           float64
	blockhash     [32]byte
	blockheight   int
	blockIndex    int
	txid          [32]byte
	confirmations int
}

func (b *btcCli) listSinceBlock(wallet, bhash string) ([]transaction, error) {
	type btctx struct {
		Addr          string  `json:"address"`
		Amount        float64 `json:"amount"`
		Fee           float64 `json:"fee"`
		BlockHash     string  `json:"blockhash"`
		BlockHeight   int64   `json:"blockheight"`
		BlockIndex    int64   `json:"blockindex"`
		Txid          string  `json:"txid"`
		Confirmations int64   `json:"confirmations"`
	}
	var txs struct {
		Txs []btctx `json:"transactions"`
	}
	res, err := b.walletCall(
		wallet,
		"listsinceblock",
		fmt.Appendf(nil, `["%s"]`, bhash),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &txs)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	transactions := make([]transaction, 0, len(txs.Txs))
	for _, v := range txs.Txs {
		transactions = append(transactions, transaction{
			address:       v.Addr,
			fee:           v.Fee,
			blockhash:     hexDecodeReverse(v.BlockHash),
			blockheight:   int(v.BlockHeight),
			blockIndex:    int(v.BlockIndex),
			txid:          hexDecodeReverse(v.Txid),
			confirmations: int(v.Confirmations),
		})
	}
	return transactions, nil
}

func (b *btcCli) getScriptHash(wallet, address string) ([32]byte, error) {
	var (
		addrInfo struct {
			Spbk string `json:"scriptPubKey"`
		}
	)
	res, err := b.walletCall(
		wallet, "getaddressinfo", fmt.Appendf(nil, `["%s"]`, address),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return [32]byte{}, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &addrInfo)
	if err != nil {
		return [32]byte{}, stackerr.Wrap(err)
	}
	spbkBytes, err := hex.DecodeString(addrInfo.Spbk)
	if err != nil {
		return [32]byte{}, stackerr.Wrap(err)
	}
	hash := sha256.Sum256(spbkBytes)
	return hash, nil
}

func (b *btcCli) deriveAddresses(
	descriptor string, start, end int64,
) ([]string, error) {
	var addresses []string
	res, err := b.call(
		"deriveaddresses",
		fmt.Appendf(nil, `["%s",[%d,%d]]`, descriptor, start, end),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &addresses)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return addresses, nil
}

type btcTxInput struct {
	Txid string `json:"txid"`
	Vout int    `json:"vout"`
}

type btcTxOutputScriptPubkey struct {
	Hex string `json:"hex"`
}

type btcTxOutput struct {
	Txid     string                  `json:"txid"`
	Vout     int                     `json:"n"`
	BtcValue float64                 `json:"value"`
	Spk      btcTxOutputScriptPubkey `json:"scriptPubKey"`
}

type btcDecodedTransaction struct {
	Txid string        `json:"txid"`
	Vin  []btcTxInput  `json:"vin"`
	Vout []btcTxOutput `json:"vout"`
}

func (b *btcCli) decodeRawTransaction(
	rawtx string,
) (btcDecodedTransaction, error) {
	var bdt btcDecodedTransaction
	res, err := b.call(
		"decoderawtransaction",
		fmt.Appendf(nil, `["%s"]`, rawtx),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return bdt, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &bdt)
	if err != nil {
		return bdt, stackerr.Wrap(err)
	}
	return bdt, nil
}

func (b *btcCli) loadWallet(wallet string) error {
	null := []byte("null")
	codeDoesNotExist := []byte(`"code":-18`)
	codeAlreadyLoaded := []byte(`"code":-35`)
	res, err := b.walletCall(
		wallet, "loadwallet", fmt.Appendf(nil, `["%s"]`, wallet),
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(res.Error) == 0 || bytes.Equal(res.Error, null) {
		return nil
	}
	if bytes.Contains(res.Error, codeDoesNotExist) {
		return ErrWalletDoesNotExist
	}
	if bytes.Contains(res.Error, codeAlreadyLoaded) {
		return ErrWalletAlreadyLoaded
	}
	return stackerr.Wrap(errors.New(string(res.Error)))
}

func (b *btcCli) createWallet(wallet string) error {
	rq := struct {
		Wn  string `json:"wallet_name"`
		Bl  bool   `json:"blank"`
		D   bool   `json:"descriptors"`
		DPK bool   `json:"disable_private_keys"`
	}{wallet, true, true, true}
	params, err := json.Marshal(rq)
	if err != nil {
		return stackerr.Wrap(err)
	}
	res, err := b.walletCall(wallet, "createwallet", params)
	err = checkRPCErr(err, res)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

type descriptorInfo struct {
	Desc string `json:"descriptor"`
}

func (b *btcCli) getDescriptorInfo(desc string) (descriptorInfo, error) {
	var r descriptorInfo
	res, err := b.call(
		"getdescriptorinfo", fmt.Appendf(nil, `["%s"]`, desc),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &r)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	return r, nil
}

func (b *btcCli) listDescriptors(wallet string) ([]string, error) {
	type descriptor struct {
		Desc string `json:"desc"`
	}
	var result struct {
		Descs []descriptor `json:"descriptors"`
	}
	res, err := b.walletCall(wallet, "listdescriptors", nil)
	err = checkRPCErr(err, res)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	err = json.Unmarshal(res.Result, &result)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	descs := make([]string, 0, len(result.Descs))
	for _, desc := range result.Descs {
		descs = append(descs, desc.Desc)
	}
	return descs, nil
}

type descriptorReq struct {
	descriptor string
	change     bool
}

func (b *btcCli) importDescriptors(
	wallet string, descs []descriptorReq,
) error {
	type descReq struct {
		D  string   `json:"desc"`
		A  bool     `json:"active"`
		R  []uint64 `json:"range"`
		Ni int64    `json:"next_index"`
		T  string   `json:"timestamp"`
		I  bool     `json:"internal"`
	}
	var params struct {
		R []descReq `json:"requests"`
	}
	for _, d := range descs {
		params.R = append(params.R, descReq{
			D:  d.descriptor,
			A:  true,
			R:  []uint64{0, 0},
			Ni: 0,
			T:  "now",
			I:  d.change,
		})
	}
	pjson, err := json.Marshal(params)
	if err != nil {
		return stackerr.Wrap(err)
	}
	res, err := b.walletCall(wallet, "importdescriptors", pjson)
	err = checkRPCErr(err, res)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (b *btcCli) rescanBlockchain(wallet string, startHeight int) error {
	res, err := b.walletCall(
		wallet,
		"rescanblockchain",
		fmt.Appendf(nil, `[%d]`, startHeight),
	)
	err = checkRPCErr(err, res)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (b *btcCli) call(method string, params []byte) (jsonRPCResponse, error) {
	res, err := b.walletCall(b.defaultWallet, method, params)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	return res, nil
}

func (b *btcCli) walletCall(
	wallet, method string, params []byte,
) (jsonRPCResponse, error) {
	var (
		req    jsonRPCRequest
		reqBuf bytes.Buffer
		enc    = json.NewEncoder(&reqBuf)
		resBuf bytes.Buffer
		res    jsonRPCResponse
	)
	b.mu.Lock()
	defer b.mu.Unlock()
	req.JsonRPC = []byte(`"2.0"`)
	req.Id = fmt.Appendf([]byte{}, "%d", b.count)
	b.count++
	req.Method = fmt.Appendf([]byte{}, `"%s"`, method)
	req.Params = params
	err := enc.Encode(req)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	uri := fmt.Sprintf(`%s/wallet/%s`, b.baseURI, wallet)
	hreq, err := http.NewRequest("POST", uri, &reqBuf)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	hreq.SetBasicAuth(b.username, b.password)
	hres, err := b.hc.Do(hreq)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	defer hres.Body.Close()
	_, err = resBuf.ReadFrom(hres.Body)
	if err != nil {
		return res, stackerr.Wrap(err)
	}
	err = json.Unmarshal(resBuf.Bytes(), &res)
	if err != nil {
		return res, errors.Join(
			stackerr.Wrap(err), fmt.Errorf("%s", resBuf.String()),
		)
	}
	return res, nil
}
