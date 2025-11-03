package main

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
	"time"
	"unsafe"

	"github.com/ncodysoftware/eps-go/log"
	"github.com/ncodysoftware/eps-go/stackerr"
)

const gap = 50

var (
	ErrReorg = errors.New("reorg")
)

type txid [32]byte
type scriptHash [32]byte
type scriptPubKey []byte
type blockHash [32]byte

type walletDescriptor struct {
	desc         string
	scriptHashes []scriptHash
	lastUsed     int
}

type walletState struct {
	name        string
	descriptors []walletDescriptor
	//
	ms mempoolState
	cs confirmedState
}

type scriptHashInfo struct {
	WalletIndex     int
	DescriptorIndex int
	ScriptHashIndex int
}

type walletTx struct {
	blockHash  blockHash
	height     int
	blockIndex int
	feeSats    int64
	txid       txid
}

type decodedTransaction struct {
	txid txid
	vin  []txInput
	vout []txOutput
}

type txInput struct {
	txid txid
	vout int
}

type txOutput struct {
	txid         txid
	vout         int
	satoshiValue uint64
	scriptPubKey scriptPubKey
}

type walletManager struct {
	log  *log.Logger
	bcli *btcCli
	//
	mu              sync.RWMutex
	ws              []walletState
	scriptHashIndex map[scriptHash]scriptHashInfo
	ss              subscriptionsState
	bestBlockHeight int
	bestBlockHash   blockHash
}

func newWalletManager(
	bcli *btcCli,
	log *log.Logger,
	walletPrefix string,
	descriptorPairs []string,
) *walletManager {
	wallets := make([]walletState, 0, len(descriptorPairs))
	for i, v := range descriptorPairs {
		descriptors := strings.Split(v, ":")
		if len(descriptors) > 2 {
			panic("bad descriptor pair")
		}
		wdescs := make([]walletDescriptor, 0, len(descriptors))
		for _, dstr := range descriptors {
			wdescs = append(wdescs, walletDescriptor{
				desc: dstr,
			})
		}
		wallets = append(wallets, walletState{
			name:        fmt.Sprintf(`%s_%d`, walletPrefix, i),
			descriptors: wdescs,
			ms:          initMempoolState(),
			cs:          initConfirmedState(),
		})
	}
	wm := &walletManager{
		bcli:            bcli,
		log:             log,
		ws:              wallets,
		ss:              initSubscriptionsState(),
		scriptHashIndex: make(map[scriptHash]scriptHashInfo),
	}
	err := wm._init()
	must(err)
	wm.log.Info("wallet manager initialized")
	return wm
}

func (w *walletManager) _init() error {
	err := w._setupWallets()
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w._scan()
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) _scan() error {
	prevBlockTip := w.bestBlockHeight
	err := w._updateBlockTip()
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i := range w.ws {
		err := w._processWalletTxs(i, prevBlockTip, w.bestBlockHeight)
		if err != nil {
			return stackerr.Wrap(err)
		}
		err = w._shNotify()
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (w *walletManager) _processWalletTxs(
	walletIdx, prevTip, currentTip int,
) error {
	if prevTip != currentTip {
		err := w._scanConfirmedTxs(
			walletIdx, prevTip, w.bestBlockHeight,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	err := w._processMempoolTxs(walletIdx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) _setupWallets() error {
	for i := range w.ws {
		err := w._setupWallet(i)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	for i := range w.ws {
		for j := range w.ws[i].descriptors {
			err := w._deriveFirstDescriptorAddresses(i, j)
			if err != nil {
				return stackerr.Wrap(err)
			}
		}
	}
	return nil
}

func (w *walletManager) _setupWallet(index int) error {
	var loadedOnEpsInit bool
	err := w.bcli.loadWallet(w.ws[index].name)
	if err == nil {
		loadedOnEpsInit = true
	}
	if err != nil && errors.Is(err, ErrWalletDoesNotExist) {
		err = w.bcli.createWallet(w.ws[index].name)
		if err != nil {
			return stackerr.Wrap(err)
		}
		err = nil
	}
	if err != nil && !errors.Is(err, ErrWalletAlreadyLoaded) {
		return stackerr.Wrap(err)
	}
	//	descs, err := w.bcli.listDescriptors(w.ws[index].name)
	//	if err != nil {
	//		return stackerr.Wrap(err)
	//	}
	toImport := make([]descriptorReq, 0, len(w.ws[index].descriptors))
	for i, v := range w.ws[index].descriptors {
		change := true
		if i == 0 {
			change = false
		}
		descinfo, err := w.bcli.getDescriptorInfo(v.desc)
		if err != nil {
			return stackerr.Wrap(err)
		}
		w.ws[index].descriptors[i].desc = descinfo.Desc
		toImport = append(toImport, descriptorReq{
			descriptor: descinfo.Desc,
			change:     change,
		})
	}
	err = w.bcli.importDescriptors(
		w.ws[index].name,
		toImport,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	descs, err := w.bcli.listDescriptors(w.ws[index].name)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(descs) < len(w.ws[index].descriptors) {
		return stackerr.Wrap(
			fmt.Errorf(
				`failed to import descriptors for wallet %d, desclen is %d`,
				index,
				len(descs),
			),
		)
	}
	if loadedOnEpsInit {
		bci, err := w.bcli.getBlockChainInfo()
		if err != nil {
			return stackerr.Wrap(err)
		}
		err = w.bcli.rescanBlockchain(
			w.ws[index].name,
			max(int(bci.Blocks)-144, 0),
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (w *walletManager) _deriveFirstDescriptorAddresses(
	wIdx int, descIdx int,
) error {
	const nshow = 3
	addrs, err := w.bcli.deriveAddresses(
		w.ws[wIdx].descriptors[descIdx].desc, 0, gap-1,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	shs, err := getScriptHashes(w.bcli, w.ws[wIdx].name, addrs)
	if err != nil {
		return stackerr.Wrap(err)
	}
	w.log.Debugf(
		"%s first %d addresses:",
		w.ws[wIdx].descriptors[descIdx].desc,
		nshow,
	)
	for i := range nshow {
		w.log.Debugf(
			"%s => %s",
			addrs[i],
			hexEncodeReverse(shs[i]),
		)
	}
	w.ws[wIdx].descriptors[descIdx].scriptHashes = shs
	for i, sh := range shs {
		w.scriptHashIndex[sh] = scriptHashInfo{
			WalletIndex:     wIdx,
			DescriptorIndex: descIdx,
			ScriptHashIndex: i,
		}
	}
	return nil
}

func (w *walletManager) _refillWallet(walletIdx, descIdx, scriptIdx int) error {
	if w.ws[walletIdx].descriptors[descIdx].lastUsed >= scriptIdx {
		return nil
	}
	w.ws[walletIdx].descriptors[descIdx].lastUsed = scriptIdx
	nTracked := len(w.ws[walletIdx].descriptors[descIdx].scriptHashes)
	nTarget := scriptIdx + gap
	start := nTracked
	nDerive := nTarget - nTracked
	if nDerive == 0 {
		return nil
	}
	end := nTarget - 1
	w.log.Debugf(
		"descriptor %d: expanding gap to %d addresses", walletIdx, nTarget,
	)
	newAddrs, err := w.bcli.deriveAddresses(
		w.ws[walletIdx].descriptors[descIdx].desc,
		int64(start),
		int64(end),
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for _, addr := range newAddrs {
		sh, err := w.bcli.getScriptHash(w.ws[walletIdx].name, addr)
		if err != nil {
			return stackerr.Wrap(err)
		}
		w.log.Debugf("%s => %s", addr, hexEncodeReverse(sh))
		w.ws[walletIdx].descriptors[descIdx].scriptHashes = append(
			w.ws[walletIdx].descriptors[descIdx].scriptHashes, sh,
		)
		w.scriptHashIndex[sh] = scriptHashInfo{
			WalletIndex:     walletIdx,
			DescriptorIndex: descIdx,
			ScriptHashIndex: len(
				w.ws[walletIdx].descriptors[descIdx].scriptHashes,
			) - 1,
		}
	}
	return nil
}

func (w *walletManager) _updateBlockTip() error {
	chainData, err := w.bcli.getBlockChainInfo()
	if err != nil {
		return stackerr.Wrap(err)
	}
	btcBestBlockHash := hexDecodeReverse(chainData.BestBlockHash)
	if w.bestBlockHeight == 0 {
		w.bestBlockHeight = int(chainData.Blocks)
		w.bestBlockHash = btcBestBlockHash
	}
	reorgHeight, _, err := reorgAtBlock(
		w.bcli,
		w.bestBlockHeight,
		w.bestBlockHash,
		int(chainData.Blocks),
		btcBestBlockHash,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if reorgHeight >= 0 {
		return ErrReorg
	}
	if w.bestBlockHeight != int(chainData.Blocks) {
		header, err := w.bcli.getBlockHeader(chainData.BestBlockHash)
		if err != nil {
			return stackerr.Wrap(err)
		}
		ntfy := jsonRPCNotification{
			JsonRPC: []byte(`"2.0"`),
			Method:  []byte(`"blockchain.headers.subscribe"`),
			Params: fmt.Appendf(
				nil,
				`[{"height":%d,"hex":"%s"}]`,
				chainData.Blocks,
				header,
			),
		}
		for sub := range w.ss.blockHeaderSubscriptions {
			sendWithTimeout(sub, ntfy, time.Second*10)
		}
	}
	w.bestBlockHash = btcBestBlockHash
	w.bestBlockHeight = int(chainData.Blocks)
	return nil
}

func (w *walletManager) _processMempoolTxs(wIdx int) error {
	var (
		mempoolTxs []transaction
	)
	txs, err := w.bcli.listSinceBlock(
		w.ws[wIdx].name, hexEncodeReverse(w.bestBlockHash),
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for _, tx := range txs {
		if tx.blockheight > 0 {
			continue
		}
		mempoolTxs = append(mempoolTxs, tx)
	}
	w.ws[wIdx].ms._deleteNotPresent(mempoolTxs)
	for _, tx := range mempoolTxs {
		err = w._processMempoolTx(wIdx, tx)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (w *walletManager) _processMempoolTx(wIdx int, tx transaction) error {
	txidHex := hexEncodeReverse(tx.txid)
	fee := int64(tx.fee * 100_000_000)
	fee = max(fee, fee*-1)
	savedTx, ok := w.ws[wIdx].ms.transactions[tx.txid]
	if ok && savedTx.feeSats >= fee {
		return nil
	}
	w.log.Debugf(
		"Mempool TX, wallet: %d, txid: %s, fee: %d", wIdx, txidHex, fee,
	)
	raw, err := w.bcli.getTransaction(w.ws[wIdx].name, txidHex)
	if err != nil {
		return stackerr.Wrap(err)
	}
	bdtx, err := w.bcli.decodeRawTransaction(raw)
	if err != nil {
		return stackerr.Wrap(err)
	}
	dtx := btcToDecodedTx(bdtx)
	w.ws[wIdx].ms.transactions[tx.txid] = walletTx{
		blockHash:  blockHash{},
		height:     0,
		blockIndex: 0,
		feeSats:    fee,
		txid:       tx.txid,
	}
	var cpfp bool
	for _, vin := range dtx.vin {
		isCpfp := w._processMempoolTxVin(wIdx, tx.txid, vin)
		if isCpfp {
			cpfp = true
		}
	}
	for _, vout := range dtx.vout {
		w._processMempoolTxVout(wIdx, vout)
	}
	if cpfp {
		savedTx := w.ws[wIdx].ms.transactions[tx.txid]
		savedTx.height = -1
		w.ws[wIdx].ms.transactions[tx.txid] = savedTx
	}
	return nil
}

func (w *walletManager) _processMempoolTxVin(
	wIdx int, tx txid, vin txInput,
) bool {
	var (
		cpfp        bool
		spentOutput txOutput
	)
	spentTxidVout := makeTxidVout(vin.txid, vin.vout)
	// check if spending known unconfirmed txout
	unconfTxout, okUnconf := w.ws[wIdx].ms.txos[spentTxidVout]
	if okUnconf {
		spentOutput = unconfTxout
		cpfp = true
		delete(w.ws[wIdx].ms.utxos, spentTxidVout)
	}
	// check if spending known confirmed txout
	confTxout, okConf := w.ws[wIdx].cs.txos[spentTxidVout]
	if okConf {
		spentOutput = confTxout
	}
	if !okUnconf && !okConf {
		return false
	}
	sh := sha256.Sum256(spentOutput.scriptPubKey)
	w.ss.updatedScriptHashes[sh] = struct{}{}
	shTxs := w.ws[wIdx].ms.shTransactions[sh]
	shTxs = uniqueAppend(shTxs, tx, func(t1, t2 txid) bool {
		return bytes.Equal(t1[:], t2[:])
	})
	w.ws[wIdx].ms.shTransactions[sh] = shTxs
	shUtxos := w.ws[wIdx].ms.shUtxos[sh]
	shUtxos = unorderedDeleteFirst(shUtxos, func(tv txidVout) bool {
		return bytes.Equal(spentTxidVout[:], tx[:])
	})
	if len(shUtxos) == 0 {
		delete(w.ws[wIdx].ms.shUtxos, sh)
	} else {
		w.ws[wIdx].ms.shUtxos[sh] = shUtxos
	}
	return cpfp
}

func (w *walletManager) _processMempoolTxVout(wIdx int, vout txOutput) {
	txVout := makeTxidVout(vout.txid, vout.vout)
	sh := sha256.Sum256(vout.scriptPubKey)
	w.ss.updatedScriptHashes[sh] = struct{}{}
	w.ws[wIdx].ms.txos[txVout] = vout
	w.ws[wIdx].ms.utxos[txVout] = struct{}{}
	shTxs := w.ws[wIdx].ms.shTransactions[sh]
	shTxs = uniqueAppend(shTxs, vout.txid, func(t1, t2 txid) bool {
		return bytes.Equal(t1[:], t2[:])
	})
	w.ws[wIdx].ms.shTransactions[sh] = shTxs
	shUtxos := w.ws[wIdx].ms.shUtxos[sh]
	shUtxos = uniqueAppend(shUtxos, txVout, func(tv1, tv2 txidVout) bool {
		return bytes.Equal(tv1[:], tv2[:])
	})
	w.ws[wIdx].ms.shUtxos[sh] = shUtxos
}

func (w *walletManager) _scanConfirmedTxs(
	wIdx, startHeight, endHeight int,
) error {
	var (
		txs []transaction
	)
	bhash, err := w.bcli.getBlockHash(int64(startHeight))
	if err != nil {
		return stackerr.Wrap(err)
	}
	btcTxs, err := w.bcli.listSinceBlock(w.ws[wIdx].name, bhash)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(btcTxs) == 0 {
		return nil
	}
	for _, tx := range btcTxs {
		if tx.blockheight < startHeight {
			continue
		}
		if tx.blockheight > endHeight {
			continue
		}
		if tx.blockheight < 1 {
			continue
		}
		txs = append(txs, tx)
	}
	slices.SortFunc(txs, func(a, b transaction) int {
		if a.blockheight > b.blockheight {
			return 1
		}
		if a.blockheight < b.blockheight {
			return -1
		}
		if a.blockIndex > b.blockIndex {
			return 1
		}
		if a.blockIndex < b.blockIndex {
			return -1
		}
		return 0
	})
	for _, tx := range txs {
		err := w._processConfirmedTx(wIdx, tx)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (w *walletManager) _processConfirmedTx(wIdx int, tx transaction) error {
	if tx.blockheight > w.bestBlockHeight {
		return nil
	}
	_, ok := w.ws[wIdx].ms.transactions[tx.txid]
	if ok {
		w.ws[wIdx].ms._delete(tx.txid)
	}
	stx, ok := w.ws[wIdx].cs.transactions[tx.txid]
	if ok && stx.blockIndex > 0 && stx.feeSats != 0 {
		// already processed
		return nil
	}
	fee := int64(tx.fee * 100_000_000)
	fee = max(fee, fee*-1)
	w.log.Debugf(
		"Confirmed TX, wallet: %d, txid: %s, height: %d, fee: %d",
		wIdx,
		hexEncodeReverse(tx.txid),
		tx.blockheight,
		fee,
	)
	w.ws[wIdx].cs.transactions[tx.txid] = walletTx{
		blockHash:  tx.blockhash,
		height:     tx.blockheight,
		blockIndex: tx.blockIndex,
		feeSats:    fee,
		txid:       tx.txid,
	}
	rawTx, err := w.bcli.getTransaction(
		w.ws[wIdx].name, hexEncodeReverse(tx.txid),
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	btcDecodedTx, err := w.bcli.decodeRawTransaction(rawTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	decodedTx := btcToDecodedTx(btcDecodedTx)
	for _, txin := range decodedTx.vin {
		err = w._processTxIn(wIdx, decodedTx.txid, txin)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	for _, txout := range decodedTx.vout {
		err = w._processTxOut(wIdx, txout)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (w *walletManager) _processTxIn(wIdx int, tx txid, txin txInput) error {
	prevTxVout := makeTxidVout(txin.txid, txin.vout)
	prevTxOut, ok := w.ws[wIdx].cs.txos[prevTxVout]
	if !ok {
		return nil
	}
	shash := sha256.Sum256(prevTxOut.scriptPubKey)
	w.ss.updatedScriptHashes[shash] = struct{}{}
	wtxs := w.ws[wIdx].cs.scriptHashTransactions[shash]
	found := false
	for _, wtx := range wtxs {
		if !bytes.Equal(wtx[:], tx[:]) {
			continue
		}
		found = true
		break
	}
	if !found {
		wtxs = append(wtxs, tx)
		w.ws[wIdx].cs.scriptHashTransactions[shash] = wtxs
	}
	shUtxos := w.ws[wIdx].cs.scriptHashUtxos[shash]
	for i, utxo := range shUtxos {
		if !bytes.Equal(utxo[:], prevTxVout[:]) {
			continue
		}
		shUtxos[i] = shUtxos[max(0, len(shUtxos)-1)]
		shUtxos = shUtxos[:len(shUtxos)-1]
		w.ws[wIdx].cs.scriptHashUtxos[shash] = shUtxos
	}
	delete(w.ws[wIdx].cs.utxos, prevTxVout)
	return nil
}

func (w *walletManager) _processTxOut(wIdx int, txout txOutput) error {
	txVout := makeTxidVout(txout.txid, txout.vout)
	w.ws[wIdx].cs.txos[txVout] = txout
	w.ws[wIdx].cs.utxos[txVout] = struct{}{}
	shash := sha256.Sum256(txout.scriptPubKey)
	w.ss.updatedScriptHashes[shash] = struct{}{}
	shUtxos := w.ws[wIdx].cs.scriptHashUtxos[shash]
	found := false
	for _, shutxo := range shUtxos {
		if !bytes.Equal(shutxo[:], txVout[:]) {
			continue
		}
		found = true
		break
	}
	if !found {
		shUtxos = append(shUtxos, txVout)
		w.ws[wIdx].cs.scriptHashUtxos[shash] = shUtxos
	}
	shInfo := w.scriptHashIndex[shash]
	w._refillWallet(
		shInfo.WalletIndex,
		shInfo.DescriptorIndex,
		shInfo.ScriptHashIndex,
	)
	shTxs := w.ws[wIdx].cs.scriptHashTransactions[shash]
	found = false
	for _, shTx := range shTxs {
		if !bytes.Equal(shTx[:], txout.txid[:]) {
			continue
		}
		found = true
		break
	}
	if !found {
		shTxs = append(shTxs, txout.txid)
		w.ws[wIdx].cs.scriptHashTransactions[shash] = shTxs
	}
	return nil
}

func (w *walletManager) _shNotify() error {
	for sh := range w.ss.updatedScriptHashes {
		delete(w.ss.updatedScriptHashes, sh)
		subs := w.ss.scriptHashSubscriptions[sh]
		if len(subs) == 0 {
			continue
		}
		status := "null"
		shTxs, err := w._sortedScriptHistory(sh)
		if err != nil {
			return stackerr.Wrap(err)
		}
		if len(shTxs) != 0 {
			shs := scriptHashStatus3(shTxs)
			status = fmt.Sprintf(
				`"%s"`,
				hex.EncodeToString(shs[:]),
			)
		}
		ntfy := jsonRPCNotification{
			JsonRPC: []byte(`"2.0"`),
			Method:  []byte(`"blockchain.scripthash.subscribe"`),
			Params: fmt.Appendf(
				nil,
				`["%s",%s]`,
				hexEncodeReverse(sh),
				status,
			),
		}
		for _, sub := range subs {
			sendWithTimeout(sub, ntfy, time.Second*10)
		}
	}
	return nil
}

func (w *walletManager) _sortedScriptHistory(sh scriptHash) ([]txData, error) {
	wInfo, ok := w.scriptHashIndex[sh]
	if !ok {
		w.log.Warnf("unknown scripthash: %s", hexEncodeReverse(sh))
		return nil, nil
	}
	conf := w.ws[wInfo.WalletIndex].cs.scriptHashTransactions[sh]
	unconf := w.ws[wInfo.WalletIndex].ms.shTransactions[sh]
	fetch := make(map[txid]struct{}, len(conf)+len(unconf))
	for _, conf := range conf {
		fetch[conf] = struct{}{}
	}
	for _, unconf := range unconf {
		fetch[unconf] = struct{}{}
	}
	txs := make([]walletTx, 0, len(fetch))
	for txid := range fetch {
		tx, ok := w.ws[wInfo.WalletIndex].cs.transactions[txid]
		if ok {
			txs = append(txs, tx)
			continue
		}
		tx, ok = w.ws[wInfo.WalletIndex].ms.transactions[txid]
		if !ok {
			continue
		}
		txs = append(txs, tx)
	}
	sortTxes(txs)
	txsData := make([]txData, 0, len(txs))
	for _, tx := range txs {
		fee := tx.feeSats
		if tx.height > 0 {
			fee = 0
		}
		txsData = append(txsData, txData{
			Txid:   hexEncodeReverse(tx.txid),
			Height: int64(tx.height),
			Fee:    fee,
		})
	}
	return txsData, nil
}

func (w *walletManager) _findTxidWalletIndex(tx txid) int {
	for i, wallet := range w.ws {
		for k := range wallet.ms.transactions {
			if !bytes.Equal(tx[:], k[:]) {
				continue
			}
			return i
		}
		for k := range wallet.cs.transactions {
			if !bytes.Equal(tx[:], k[:]) {
				continue
			}
			return i
		}
	}
	return -1
}

func (w *walletManager) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			goto end
		case <-ticker.C:
			func() {
				w.mu.Lock()
				defer w.mu.Unlock()
				err := w._scan()
				if err != nil {
					w.log.Err(stackerr.Wrap(err))
				}
			}()
		}
	}
end:
}

func (w *walletManager) unsubscribeAll(cctx *clientCtx) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.ss.blockHeaderSubscriptions, cctx.notifChan)
	type shSubIndex struct {
		sh    scriptHash
		index int
	}
	var shSubsToRemove []shSubIndex
	for _, sh := range cctx.scriptHashSubs {
		subs := w.ss.scriptHashSubscriptions[sh]
		for i, sub := range subs {
			if sub != cctx.notifChan {
				continue
			}
			shSubsToRemove = append(
				shSubsToRemove, shSubIndex{sh, i},
			)
		}
	}
	for _, shSub := range shSubsToRemove {
		subs := w.ss.scriptHashSubscriptions[shSub.sh]
		subs[shSub.index] = subs[len(subs)-1]
		subs = subs[:max(0, len(subs)-1)]
		w.ss.scriptHashSubscriptions[shSub.sh] = subs
	}
	cctx.scriptHashSubs = nil
}

func (w *walletManager) subscribeHeaders(cctx *clientCtx) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ss.blockHeaderSubscriptions[cctx.notifChan] = struct{}{}
}

type scriptBalance struct {
	Confirmed   int64 `json:"confirmed"`
	Unconfirmed int64 `json:"unconfirmed"`
}

func (w *walletManager) getScriptBalance(
	sh scriptHash,
) (scriptBalance, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	wInfo, ok := w.scriptHashIndex[sh]
	if !ok {
		w.log.Warnf("unknown scripthash: %s", hexEncodeReverse(sh))
		return scriptBalance{}, nil
	}
	var s scriptBalance
	confirmedIndexes := w.ws[wInfo.WalletIndex].cs.scriptHashUtxos[sh]
	for _, i := range confirmedIndexes {
		vout, ok := w.ws[wInfo.WalletIndex].cs.txos[i]
		if !ok {
			panic("utxo not present on txos list")
		}
		s.Confirmed += int64(vout.satoshiValue)
	}
	unconfirmedIndexes := w.ws[wInfo.WalletIndex].ms.shUtxos[sh]
	for _, i := range unconfirmedIndexes {
		vout, ok := w.ws[wInfo.WalletIndex].ms.txos[i]
		if !ok {
			panic("utxo not present on mempool txos list")
		}
		s.Unconfirmed += int64(vout.satoshiValue)
	}
	return s, nil
}

type txData struct {
	Txid   string `json:"tx_hash"`
	Height int64  `json:"height"`
	Fee    int64  `json:"-"`
}

func (w *walletManager) getScriptHistory(sh scriptHash) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	null := []byte("null")
	hist, err := w._sortedScriptHistory(sh)
	if err != nil {
		return null, stackerr.Wrap(err)
	}
	if len(hist) == 0 {
		return null, nil
	}
	var histJson []byte
	type mTxData struct {
		Txid   string `json:"tx_hash"`
		Height int64  `json:"height"`
		Fee    int64  `json:"fee"`
	}
	for i, tx := range hist {
		var txj []byte
		if tx.Height < 1 {
			tx1 := mTxData{
				Txid:   tx.Txid,
				Height: tx.Height,
				Fee:    tx.Fee,
			}
			txj, err = json.Marshal(tx1)
			if err != nil {
				return null, stackerr.Wrap(err)
			}
		} else {
			txj, err = json.Marshal(tx)
			if err != nil {
				return null, stackerr.Wrap(err)
			}
		}
		if i == 0 {
			histJson = fmt.Appendf(histJson, `[`)
		}
		histJson = fmt.Appendf(histJson, `%s`, txj)
		if i == len(hist)-1 {
			histJson = fmt.Appendf(histJson, `]`)
		} else {
			histJson = fmt.Appendf(histJson, `,`)
		}
	}
	return histJson, nil
}

func (w *walletManager) subscribeScriptHash(cctx *clientCtx, sh scriptHash) {
	w.mu.Lock()
	defer w.mu.Unlock()
	found := false
	for _, v := range cctx.scriptHashSubs {
		if !bytes.Equal(v[:], sh[:]) {
			continue
		}
		found = true
		break
	}
	if found {
		return
	}
	cctx.scriptHashSubs = append(cctx.scriptHashSubs, sh)
	subs := w.ss.scriptHashSubscriptions[sh]
	for _, v := range subs {
		if cctx.notifChan != v {
			continue
		}
		found = true
		break
	}
	if found {
		return
	}
	subs = append(subs, cctx.notifChan)
	w.ss.scriptHashSubscriptions[sh] = subs
}

func (w *walletManager) scriptHashStatus(sh scriptHash) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	txs, err := w._sortedScriptHistory(sh)
	if err != nil {
		return ""
	}
	if len(txs) == 0 {
		return ""
	}
	statusB := scriptHashStatus3(txs)
	return hex.EncodeToString(statusB[:])
}

func (w *walletManager) getTransaction(dtx txid) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	wIdx := w._findTxidWalletIndex(dtx)
	if wIdx < 0 {
		return "", nil
	}
	rawTx, err := w.bcli.getTransaction(
		w.ws[wIdx].name, hexEncodeReverse(dtx),
	)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return rawTx, nil
}

type txMerkleProof struct {
	Height      int64    `json:"block_height"`
	Pos         int64    `json:"pos"`
	MerkleProof []string `json:"merkle"`
}

func (w *walletManager) getTxMerkleProof(
	tx txid, height int64,
) (txMerkleProof, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var (
		r   txMerkleProof
		err error
	)
	blockHash, err := w.bcli.getBlockHash(height)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	block, err := w.bcli.getBlock(blockHash)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	txids := hexHashesToBytes(block.Tx)
	pos, proof := merkleProof(txids, tx)
	if pos < 0 {
		return r, fmt.Errorf("bad txid")
	}
	proofHex := make([]string, 0, len(proof))
	for _, v := range proof {
		proofHex = append(proofHex, hexEncodeReverse(v))
	}
	r.Height = height
	r.Pos = int64(pos)
	r.MerkleProof = proofHex
	return r, nil
}

type txidData struct {
	Txid   string   `json:"tx_hash"`
	Merkle []string `json:"merkle,omitempty"`
}

func (w *walletManager) txidFromPos(
	height, pos int64, merkle bool,
) (txidData, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var (
		r   txidData
		err error
	)
	blockHash, err := w.bcli.getBlockHash(height)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	block, err := w.bcli.getBlock(blockHash)
	if err != nil {
		return r, stackerr.Wrap(err)
	}
	txids := hexHashesToBytes(block.Tx)
	if len(txids)-1 < int(pos) {
		return r, ErrBadRequest
	}
	txid := txids[pos]
	if merkle {
		pos, proof := merkleProof(txids, txid)
		if pos < 0 {
			return r, fmt.Errorf("bad txid")
		}
		proofHex := make([]string, 0, len(proof))
		for _, v := range proof {
			proofHex = append(proofHex, hexEncodeReverse(v))
		}
		r.Merkle = proofHex
	}
	r.Txid = hexEncodeReverse(txid)
	return r, nil
}

type mempoolState struct {
	transactions   map[txid]walletTx
	txos           map[txidVout]txOutput
	utxos          map[txidVout]struct{}
	shTransactions map[scriptHash][]txid
	shUtxos        map[scriptHash][]txidVout
}

func initMempoolState() mempoolState {
	return mempoolState{
		transactions:   map[txid]walletTx{},
		txos:           map[txidVout]txOutput{},
		utxos:          map[txidVout]struct{}{},
		shTransactions: map[scriptHash][]txid{},
		shUtxos:        map[scriptHash][]txidVout{},
	}
}

func (m *mempoolState) _delete(tx txid) {
	delete(m.transactions, tx)
	for txVout := range m.txos {
		cmp, _ := txVout.decode()
		if !bytes.Equal(cmp[:], tx[:]) {
			continue
		}
		delete(m.txos, txVout)
	}
	for txVout := range m.utxos {
		cmp, _ := txVout.decode()
		if !bytes.Equal(cmp[:], tx[:]) {
			continue
		}
		delete(m.utxos, txVout)
	}
	for sh, shtxs := range m.shTransactions {
		shtxs = unorderedDeleteFirst(shtxs, func(t txid) bool {
			return bytes.Equal(tx[:], t[:])
		})
		if len(shtxs) == 0 {
			delete(m.shTransactions, sh)
		} else {
			m.shTransactions[sh] = shtxs
		}
	}
	for sh, shutxos := range m.shUtxos {
		shutxos = unorderedDeleteFirst(shutxos, func(tv txidVout) bool {
			txid, _ := tv.decode()
			return bytes.Equal(tx[:], txid[:])
		})
		if len(shutxos) == 0 {
			delete(m.shUtxos, sh)
		} else {
			m.shUtxos[sh] = shutxos
		}
	}
}

func (m *mempoolState) _deleteNotPresent(txs []transaction) {
	cmp := make(map[txid]struct{}, len(txs))
	for _, tx := range txs {
		cmp[tx.txid] = struct{}{}
	}
	for tx := range m.transactions {
		_, present := cmp[tx]
		if present {
			continue
		}
		m._delete(tx)
	}
}

type confirmedState struct {
	transactions           map[txid]walletTx
	txos                   map[txidVout]txOutput
	utxos                  map[txidVout]struct{}
	scriptHashTransactions map[scriptHash][]txid
	scriptHashUtxos        map[scriptHash][]txidVout
}

func initConfirmedState() confirmedState {
	return confirmedState{
		transactions:           map[txid]walletTx{},
		txos:                   map[txidVout]txOutput{},
		utxos:                  map[txidVout]struct{}{},
		scriptHashTransactions: map[scriptHash][]txid{},
		scriptHashUtxos:        map[scriptHash][]txidVout{},
	}
}

type subscriptionsState struct {
	scriptHashSubscriptions  map[scriptHash][]chan jsonRPCNotification
	blockHeaderSubscriptions map[chan jsonRPCNotification]struct{}
	updatedScriptHashes      map[scriptHash]struct{}
}

func initSubscriptionsState() subscriptionsState {
	return subscriptionsState{
		scriptHashSubscriptions:  map[scriptHash][]chan jsonRPCNotification{},
		blockHeaderSubscriptions: map[chan jsonRPCNotification]struct{}{},
		updatedScriptHashes:      map[scriptHash]struct{}{},
	}
}

func reorgAtBlock(
	bcli *btcCli,
	startBlock int,
	startBlockHash blockHash,
	tipBlock int,
	tipBlockHash blockHash,
) (int, blockHash, error) {
	if bytes.Equal(startBlockHash[:], tipBlockHash[:]) {
		return -1, blockHash{}, nil
	}
	prevBlockHash := hexEncodeReverse(startBlockHash)
	for nextHeight := startBlock + 1; nextHeight <= tipBlock; nextHeight++ {
		h, err := bcli.getBlockHash(int64(nextHeight))
		if err != nil {
			return -1, blockHash{}, stackerr.Wrap(err)
		}
		bh, err := bcli.getBlockHeaderVerbose(h)
		if err != nil {
			return -1, blockHash{}, stackerr.Wrap(err)
		}
		if bh.PreviousBlockHash == prevBlockHash {
			prevBlockHash = h
			continue
		}
		return nextHeight - 1, hexDecodeReverse(prevBlockHash), nil
	}
	return -1, blockHash{}, nil
}

func btcToDecodedTx(btcTx btcDecodedTransaction) decodedTransaction {
	var dt decodedTransaction
	dt.txid = hexDecodeReverse(btcTx.Txid)
	for _, vin := range btcTx.Vin {
		dt.vin = append(dt.vin, txInput{
			txid: hexDecodeReverse(vin.Txid),
			vout: vin.Vout,
		})
	}
	for _, vout := range btcTx.Vout {
		spk, err := hex.DecodeString(vout.Spk.Hex)
		if err != nil {
			panic(stackerr.Wrap(err))
		}
		dt.vout = append(dt.vout, txOutput{
			txid:         dt.txid,
			vout:         vout.Vout,
			satoshiValue: uint64(vout.BtcValue * 100_000_000),
			scriptPubKey: spk,
		})
	}
	return dt
}

type txidVout [32 + 8]byte

func (t *txidVout) decode() (txid, int) {
	var (
		txidd *txid
		voutd *int
	)
	txidd = (*txid)(unsafe.Pointer(&t[0]))
	voutd = (*int)(unsafe.Pointer(&t[32]))
	return *txidd, *voutd
}

func makeTxidVout(txid txid, vout int) txidVout {
	var (
		txidVoutd txidVout
		voutd     *[8]byte
	)
	voutd = (*[8]byte)(unsafe.Pointer(&vout))
	copy(txidVoutd[:], txid[:])
	copy(txidVoutd[32:], voutd[:])
	return txidVoutd
}

func sortTxes(txs []walletTx) {
	var conf []walletTx
	var unconf []walletTx
	for _, v := range txs {
		if v.height < 1 {
			unconf = append(unconf, v)
			continue
		}
		conf = append(conf, v)
	}
	slices.SortFunc(conf, func(a, b walletTx) int {
		if a.height > b.height {
			return 1
		}
		if a.height < b.height {
			return -1
		}
		if a.blockIndex > b.blockIndex {
			return -1
		}
		if a.blockIndex < b.blockIndex {
			return 1
		}
		return 0
	})
	txs = txs[:0]
	txs = append(txs, conf...)
	txs = append(txs, unconf...)
}

func scriptHashStatus2(txs []walletTx) [32]byte {
	sortTxes(txs)
	var r []byte
	for _, tx := range txs {
		r = fmt.Appendf(
			r, "%s:%d:", hexEncodeReverse(tx.txid), tx.height,
		)
	}
	return sha256.Sum256(r)
}

func scriptHashStatus3(txs []txData) [32]byte {
	var r []byte
	for _, tx := range txs {
		r = fmt.Appendf(
			r, "%s:%d:", tx.Txid, tx.Height,
		)
	}
	return sha256.Sum256(r)
}

func getScriptHashes(bcli *btcCli, wallet string, addrs []string) ([]scriptHash, error) {
	var shashes = make([]scriptHash, 0, len(addrs))
	for _, v := range addrs {
		shash, err := bcli.getScriptHash(wallet, v)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		shashes = append(shashes, shash)
	}
	return shashes, nil
}

func catDoubleSha256(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:], left[:])
	copy(buf[32:], right[:])
	hash := sha256.Sum256(buf[:])
	return sha256.Sum256(hash[:])
}

func merkleRoot(hashes [][32]byte) [32]byte {
	var lvl [][32]byte
	lvl = hashPairs(hashes)
	for len(lvl) != 1 {
		lvl = hashPairs(lvl)
	}
	return lvl[0]
}

func findPos(txids [][32]byte, txid [32]byte) int {
	for i, v := range txids {
		if !bytes.Equal(txid[:], v[:]) {
			continue
		}
		return i
	}
	return -1
}

func merkleProof(txids [][32]byte, txid [32]byte) (int, [][32]byte) {
	var (
		branch     [][32]byte
		level      = make([][32]byte, len(txids))
		txpos, pos int
	)
	copy(level[:], txids[:])
	txpos = findPos(txids, txid)
	if txpos < 0 {
		return -1, nil
	}
	pos = txpos
	if len(level) == 1 {
		return txpos, [][32]byte{}
	}
	if len(level)%2 != 0 {
		level = append(level, level[len(level)-1])
	}
	for len(level) > 1 {
		if pos%2 == 0 {
			branch = append(branch, level[pos+1])
		} else {
			branch = append(branch, level[pos-1])
		}
		level = hashPairs(level)
		pos /= 2
	}
	return txpos, branch
}

func hashPairs(hashes [][32]byte) [][32]byte {
	hlen := len(hashes)
	rlen := (hlen / 2) + (hlen % 2)
	for i := range rlen {
		pos := i * 2
		if pos+1 > hlen-1 {
			hashes[i] = catDoubleSha256(hashes[pos], hashes[pos])
			continue
		}
		hashes[i] = catDoubleSha256(hashes[pos], hashes[pos+1])
	}
	hashes = hashes[:rlen]
	if rlen%2 != 0 && rlen != 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}
	return hashes
}

func checkMerkleProof(
	txid [32]byte, pos int, merkleRoot [32]byte, branch [][32]byte,
) bool {
	current := txid
	for _, v := range branch {
		if pos%2 == 0 {
			current = catDoubleSha256(current, v)
		} else {
			current = catDoubleSha256(v, current)
		}
		pos /= 2
	}
	return bytes.Equal(merkleRoot[:], current[:])
}

func hexDecodeReverse(s string) [32]byte {
	var buf [32]byte
	if s == "" {
		return buf
	}
	decoded, _ := hex.DecodeString(s)
	for i, j := 31, 0; i >= 0; i, j = i-1, j+1 {
		buf[j] = decoded[i]
	}
	return buf
}

func hexEncodeReverse(data [32]byte) string {
	var buf [32]byte
	for i, j := 31, 0; i >= 0; i, j = i-1, j+1 {
		buf[j] = data[i]
	}
	return hex.EncodeToString(buf[:])
}

func hexHashesToBytes(s []string) [][32]byte {
	r := make([][32]byte, 0, len(s))
	for _, v := range s {
		r = append(r, hexDecodeReverse(v))
	}
	return r
}

func uniqueAppend[T any](s []T, val T, eqFn func(T, T) bool) []T {
	for _, v := range s {
		if eqFn(v, val) {
			return s
		}
	}
	return append(s, val)
}

func unorderedDeleteFirst[T any](s []T, eqFn func(T) bool) []T {
	for i, v := range s {
		if !eqFn(v) {
			continue
		}
		s[i] = s[len(s)-1]
		s = s[:len(s)-1]
		break
	}
	return s
}

func sendWithTimeout[T any](ch chan T, msg T, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case ch <- msg:
		return true
	case <-timer.C:
		return false
	}
}
