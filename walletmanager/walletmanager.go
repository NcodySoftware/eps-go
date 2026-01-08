package walletmanager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

type wallet struct {
	kind             scriptpubkey.Kind
	reqSigs          byte
	masterPubs       []bip32.ExtendedKey
	nextReceiveIndex uint32
	nReceiveDerived  uint32
	nextChangeIndex  uint32
	nChangeDerived   uint32
	height           int
	hash             [32]byte
}

type scriptPubkeyInfo struct {
	walletIdx int
	account   accountKind
	pubkeyIdx uint32
}

type WalletConfig struct {
	Kind       scriptpubkey.Kind
	Reqsigs    byte
	MasterPubs []bip32.ExtendedKey
	Height     int
}

type txidVout [32 + 4]byte

type W struct {
	db             sql.Database
	log            *log.Logger
	bcli           *bitcoin.Client
	repo           *repository
	net            bitcoin.Network
	bestHeader     int
	bestHeaderHash [32]byte
	//
	cancel        func()
	done          chan struct{}
	initCompleted chan struct{}
	//
	mu sync.Mutex
	//
	wallets       []wallet
	scriptPubkeys map[[32]byte]scriptPubkeyInfo
	//
	shSubs map[[32]byte]map[uint32]func([32]byte)
	hSubs  map[uint32]func(int, [80]byte)
}

func New(
	ctx context.Context,
	db sql.Database,
	log *log.Logger,
	bcli *bitcoin.Client,
	wallets []WalletConfig,
	net bitcoin.Network,
) (*W, error) {
	repo, err := newRepository(ctx, db)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &W{
		db:            db,
		log:           log,
		bcli:          bcli,
		repo:          repo,
		net:           net,
		scriptPubkeys: make(map[[32]byte]scriptPubkeyInfo),
		cancel:        cancel,
		done:          make(chan struct{}),
		initCompleted: make(chan struct{}),
		wallets:       make([]wallet, len(wallets)),
		shSubs:        make(map[[32]byte]map[uint32]func([32]byte)),
		hSubs:         make(map[uint32]func(int, [80]byte)),
	}
	var buf []byte
	for i := range wallets {
		err := w.setupWallet(ctx, i, &wallets[i], &buf)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
	}
	go func() {
		defer close(w.done)
		err := w.run(ctx)
		if err != nil {
			w.log.Err(stackerr.Wrap(err))
		}
	}()
	return w, nil
}

func (w *W) Close(ctx context.Context) error {
	w.cancel()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out while closing")
	}
}

func (w *W) WaitInit(ctx context.Context) error {
	select {
	case <-w.initCompleted:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out while initializing")
	}
}

func (w *W) GetBlockHeader(
	ctx context.Context, height int, out *[80]byte,
) error {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	if height == 0 {
		copy(out[:], genesisBlockData[w.net].Serialized[:80])
		return nil
	}
	err := w.repo.selectRawBlockHeaderByHeight(ctx, w.db, height, out)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *W) GetBlockHeaders(
	ctx context.Context, height, limit int,
) ([][80]byte, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	headers, err := w.repo.selectRawBlockHeadersByHeight(
		ctx, w.db, height, limit,
	)
	if height == 0 {
		var h [80]byte
		copy(h[:], genesisBlockData[w.net].Serialized[:80])
		headers = slices.Insert(headers, 0, h)
		if len(headers) > limit {
			headers = headers[:len(headers)-1]
		}
	}
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return headers, nil
}

func (w *W) GetTipHeader(
	ctx context.Context, outHeight *int, outHeader *[80]byte,
) error {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.repo.selectLastBlockHeaderHeightAndRaw(
		ctx, w.db, outHeight, outHeader,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *W) GetScriptHashBalance(
	ctx context.Context, sh *[32]byte, outConf, outUnconf *uint64,
) error {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.scriptPubkeys[*sh]
	if !ok {
		*outConf, *outUnconf = 0, 0
		return nil
	}
	err := w.repo.selectScriptHashBalance(ctx, w.db, sh, outConf)
	if err != nil {
		return stackerr.Wrap(err)
	}
	*outUnconf = 0
	return nil
}

func (w *W) GetScriptHashHistory(
	ctx context.Context, sh *[32]byte,
) ([]TxData, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	hist, err := w.repo.selectScriptHashHistory(ctx, w.db, sh)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return hist, nil
}

func (w *W) GetScriptHashUnspent(
	ctx context.Context, sh *[32]byte,
) ([]UtxoData, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	utxos, err := w.repo.selectScriptHashUnspent(ctx, w.db, sh)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return utxos, nil
}

func (w *W) GetScriptHashStatus(
	ctx context.Context, sh *[32]byte, buf *[]byte,
) ([]byte, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.getScriptHashStatus(ctx, w.db, sh, buf)
}

func (w *W) getScriptHashStatus(
	ctx context.Context, db sql.Database, sh *[32]byte, buf *[]byte,
) ([]byte, error) {
	hist, err := w.repo.selectScriptHashHistory(ctx, db, sh)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return shStatus(hist, buf), nil
}

func (w *W) BroadcastTX(
	ctx context.Context, rawTx []byte, buf *[]byte,
) ([32]byte, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	var txid [32]byte
	var tx bitcoin.Transaction
	err := tx.Deserialize(bytes.NewReader(rawTx))
	if err != nil {
		return txid, stackerr.Wrap(err)
	}
	txid = tx.Txid(buf)
	err = w.bcli.BroadcastWTX(ctx, rawTx)
	if err != nil {
		return txid, stackerr.Wrap(err)
	}
	return txid, nil
}

func (w *W) GetRawTx(
	ctx context.Context, txid *[32]byte,
) ([]byte, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	raw, err := w.repo.selectRawTransaction(ctx, w.db, txid)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return raw, nil
}

type MerkleData struct {
	Merkle [][32]byte
	Pos    int
}

func (w *W) GetTransactionMerkle(
	ctx context.Context, txid *[32]byte,
) (MerkleData, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	var md MerkleData
	txData, err := w.repo.selectTransactionFromTxid(ctx, w.db, txid)
	if err != nil {
		return md, stackerr.Wrap(err)
	}
	branch := make([][32]byte, 0, len(txData.MerkleProof)/32)
	for i := 0; i < len(txData.MerkleProof); i += 32 {
		var h [32]byte
		copy(h[:], txData.MerkleProof[i:i+32])
		branch = append(branch, h)
	}
	return MerkleData{
		Merkle: branch,
		Pos:    txData.Pos,
	}, nil
}

type MerkleFromPosData struct {
	Merkle [][32]byte
	Txid   [32]byte
}

func (w *W) GetTransactionMerkleFromPos(
	ctx context.Context, height, pos int,
) (MerkleFromPosData, error) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	var md MerkleFromPosData
	txData, err := w.repo.selectTransactionFromHeightPos(
		ctx, w.db, height, pos,
	)
	if err != nil {
		return md, stackerr.Wrap(err)
	}
	branch := make([][32]byte, 0, len(txData.MerkleProof)/32)
	for i := 0; i < len(txData.MerkleProof); i += 32 {
		var h [32]byte
		copy(h[:], txData.MerkleProof[i:i+32])
		branch = append(branch, h)
	}
	return MerkleFromPosData{
		Merkle: branch,
		Txid:   txData.Txid,
	}, nil
}

func (w *W) HeadersSubscribe(id uint32, cb func(height int, header [80]byte)) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hSubs[id] = cb
}

func (w *W) notifyHeaderSubscribers(height int, buf *[]byte) {
	var (
		buf2 [80]byte
	)
	if len(w.hSubs) == 0 {
		return
	}
	copy(buf2[:], (*buf)[:])
	for _, cb := range w.hSubs {
		cb(height, buf2)
	}
}

func (w *W) ScriptHashSubscribe(
	id uint32, sh [32]byte, cb func(status [32]byte),
) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	m := w.shSubs[sh]
	if m == nil {
		m = make(map[uint32]func([32]byte))
	}
	m[id] = cb
	w.shSubs[sh] = m
}

func (w *W) notifyScriptHashSubscribers(sh [32]byte, status [32]byte) {
	if len(w.shSubs[sh]) == 0 {
		return
	}
	for _, cb := range w.shSubs[sh] {
		cb(status)
	}
}

func (w *W) UnsubscribeAll(id uint32) {
	<-w.initCompleted
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.hSubs, id)
	for k, v := range w.shSubs {
		delete(v, id)
		if len(v) == 0 {
			delete(w.shSubs, k)
		}
	}
	delete(w.hSubs, id)
}

func (w *W) setupWallet(
	ctx context.Context, i int, wc *WalletConfig, buf *[]byte,
) error {
	clearBuf(buf)
	var whash [32]byte
	walletHash(
		wc.Kind,
		wc.Reqsigs,
		wc.MasterPubs,
		&whash,
		buf,
	)
	wdata, err := w.repo.selectWalletData(ctx, w.db, &whash)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		wdata = walletData{
			Hash:             whash,
			Height:           wc.Height,
			NextReceiveIndex: 0,
			NextChangeIndex:  0,
		}
		err := w.repo.insertWalletData(ctx, w.db, &wdata)
		if err != nil {
			return stackerr.Wrap(err)
		}
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	w.wallets[i] = wallet{
		kind:             wc.Kind,
		reqSigs:          wc.Reqsigs,
		masterPubs:       wc.MasterPubs,
		nextReceiveIndex: wdata.NextReceiveIndex,
		nextChangeIndex:  wdata.NextChangeIndex,
		height:           wdata.Height,
		hash:             whash,
	}
	w.refillWallet(i)
	return nil
}

func (w *W) run(ctx context.Context) error {
	var buf []byte
	var errReorg reorgError
	err := w.syncHeaders(ctx, &buf)
	if err != nil && errors.As(err, &errReorg) {
		err := w.processReorg(ctx, errReorg)
		if err != nil {
			return stackerr.Wrap(err)
		}
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.syncWallets(ctx, &buf)
	if err != nil {
		return stackerr.Wrap(err)
	}
	close(w.initCompleted)
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
		err := func() error {
			w.mu.Lock()
			defer w.mu.Unlock()
			var errReorg reorgError
			err := w.syncHeaders(ctx, &buf)
			if err != nil && errors.As(err, &errReorg) {
				err := w.processReorg(ctx, errReorg)
				if err != nil {
					return stackerr.Wrap(err)
				}
			} else if err != nil {
				return stackerr.Wrap(err)
			}
			err = w.syncWallets(ctx, &buf)
			if err != nil {
				return stackerr.Wrap(err)
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
}

func (w *W) syncHeaders(ctx context.Context, buf *[]byte) error {
	var hd blockHeaderData
	err := w.repo.selectLastBlockHeaderData(ctx, w.db, &hd)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		hd = genesisBlockData[w.net]
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	var (
		headerHashes = [1][32]byte{hd.Hash}
	)
	w.bestHeader = hd.Height
	w.bestHeaderHash = hd.Hash
	for ctx.Err() == nil {
		headerHashes[0] = w.bestHeaderHash
		headers, err := w.bcli.GetHeaders(
			ctx, headerHashes[:], [32]byte{},
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		if len(headers) == 0 {
			// up to date
			return nil
		}
		for i := range headers {
			if !bytes.Equal(
				headers[i].PreviousBlock[:],
				w.bestHeaderHash[:],
			) {
				err := w.checkReorg(ctx)
				if err != nil {
					return stackerr.Wrap(err)
				}
				return fmt.Errorf("unexpected block")
			}
			if i == 0 {
				w.log.Debugf(
					"NEW HEADERS: best header: %d",
					w.bestHeader+len(headers),
				)
			}
			clearBuf(buf)
			hash := headers[i].Hash(buf)
			clearBuf(buf)
			*buf = headers[i].Serialize(*buf)
			w.notifyHeaderSubscribers(w.bestHeader+1, buf)
			err = w.repo.insertBlockHeader(
				ctx, w.db, &hash, w.bestHeader+1, *buf,
			)
			if err != nil {
				return stackerr.Wrap(err)
			}
			w.bestHeader++
			w.bestHeaderHash = hash
		}
	}
	return nil
}

func (w *W) processReorg(ctx context.Context, errReorg reorgError) error {
	w.log.Warnf(
		"PROCESSING REORG ROLLBACK TO BLOCK %d",
		errReorg.LastHeightOnChain,
	)
	var hashes [][32]byte
	err := w.repo.selectBlockHashesAtHeight(
		ctx, w.db, errReorg.LastHeightOnChain, 1, &hashes,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if len(hashes) < 1 {
		panic("unreachable")
	}
	err = w.repo.deleteAllSinceBlock(ctx, w.db, errReorg.LastHeightOnChain)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.repo.reloadCaches(ctx, w.db)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i := range w.wallets {
		wal := &w.wallets[i]
		wal.height = min(errReorg.LastHeightOnChain, wal.height)
	}
	w.bestHeader = min(errReorg.LastHeightOnChain)
	w.bestHeaderHash = hashes[0]
	w.log.Warn("REORG PROCESSED")
	return nil
}

type reorgError struct {
	LastHeightOnChain int
}

func (r reorgError) Error() string {
	return fmt.Sprintf("REORG AT HEIGHT: %x", r.LastHeightOnChain)
}

func (w *W) checkReorg(ctx context.Context) error {
	if w.bestHeader == 0 {
		return nil
	}
	height := w.bestHeader
	var (
		blockHashes [][32]byte
		headersGet  [][32]byte
	)
	for ; ; height-- {
		if w.bestHeader-(height-1) > 6 {
			panic("REORG WITH DELTA > 6")
		}
		clearBuf(&blockHashes)
		err := w.repo.selectBlockHashesAtHeight(
			ctx, w.db, height-1, 1, &blockHashes,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		if len(blockHashes) < 1 {
			panic("unreachable")
		}
		clearBuf(&headersGet)
		headersGet = append(headersGet, blockHashes[0])
		headers, err := w.bcli.GetHeaders(ctx, headersGet, [32]byte{})
		if err != nil {
			return stackerr.Wrap(err)
		}
		if len(headers) == 0 || len(headers) > 0 &&
			headers[0].PreviousBlock == blockHashes[0] {
			return reorgError{height - 1}
		}
	}
	return nil
}

func (w *W) syncWallets(ctx context.Context, buf *[]byte) error {
	if len(w.wallets) == 0 {
		return nil
	}
	height := w.wallets[0].height
	for i := range w.wallets {
		height = min(height, w.wallets[i].height)
	}
	height = max(height, 1)
	if height >= w.bestHeader {
		return nil
	}
	hbuf := make([][32]byte, 0, 2000)
	var (
		buf2    []byte
		txidBuf [][32]byte
		rem     estimateTime
	)
	height++
	for height <= w.bestHeader {
		hbuf := hbuf[:0]
		err := w.repo.selectBlockHashesAtHeight(
			ctx, w.db, height, cap(hbuf), &hbuf,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		for _, bh := range hbuf {
			block, err := w.bcli.GetBlock(ctx, bh)
			if err != nil {
				return stackerr.Wrap(err)
			}
			err = sql.Execute(
				ctx,
				w.db,
				func(db sql.Database) error {
					return w.processBlock(
						ctx,
						db,
						&block,
						height,
						&rem,
						buf,
						&buf2,
						&txidBuf,
					)
				},
			)
			if err != nil {
				return stackerr.Wrap(err)
			}
			height++
		}
	}
	return nil
}

func (w *W) processBlock(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	height int,
	est *estimateTime,
	buf *[]byte,
	buf2 *[]byte,
	txidBuf *[][32]byte,
) error {
	updateRemaining(height, w.bestHeader, est)
	if est.remainingHours > -1 {
		w.log.Debugf(
			"NEW BLOCK; height: %d; remaining time ~%dh:%dm",
			height,
			est.remainingHours,
			est.remainingMinutes,
		)
	} else {
		w.log.Debugf("NEW BLOCK; height: %d", height)
	}
	updatedSH := make(map[[32]byte]struct{})
	for i := range block.Transactions {
		tx := &block.Transactions[i]
		for j := range tx.Outputs {
			err := w.processOutput(
				ctx,
				db,
				block,
				height,
				tx,
				i,
				&tx.Outputs[j],
				uint32(j),
				updatedSH,
				buf,
				buf2,
				txidBuf,
			)
			if err != nil {
				return stackerr.Wrap(err)
			}
		}
		for j := range tx.Inputs {
			err := w.processInput(
				ctx,
				db,
				block,
				height,
				tx,
				i,
				&tx.Inputs[j],
				updatedSH,
				buf,
				buf2,
				txidBuf,
			)
			if err != nil {
				return stackerr.Wrap(err)
			}
		}
	}
	for i := range w.wallets {
		wl := &w.wallets[i]
		if wl.height >= height {
			continue
		}
		err := w.repo.updateWalletHeight(ctx, db, &wl.hash, height)
		if err != nil {
			return stackerr.Wrap(err)
		}
		wl.height = height
	}
	for sh := range updatedSH {
		clearBuf(buf)
		var status2 [32]byte
		status, err := w.getScriptHashStatus(ctx, db, &sh, buf)
		if err != nil {
			return stackerr.Wrap(err)
		}
		copy(status2[:], status)
		w.notifyScriptHashSubscribers(sh, status2)
	}
	return nil
}

func (w *W) processOutput(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	height int,
	tx *bitcoin.Transaction,
	txPos int,
	out *bitcoin.Output,
	vout uint32,
	updatedSH map[[32]byte]struct{},
	buf *[]byte,
	buf2 *[]byte,
	txidBuf *[][32]byte,
) error {
	sh := sha256.Sum256(out.ScriptPubkey)
	info, ok := w.scriptPubkeys[sh]
	if !ok {
		return nil
	}
	updatedSH[sh] = struct{}{}
	clearBuf(buf)
	blockHash := block.Hash(buf)
	clearBuf(buf)
	txid := tx.Txid(buf)
	clearBuf(buf)
	rawTx := tx.Serialize(*buf)
	*buf = rawTx
	clearBuf(buf2)
	clearBuf(txidBuf)
	*txidBuf = block.TXIDs(*txidBuf, buf2)
	pos, merkleP := MerkleProof(*txidBuf, txid)
	if pos != txPos {
		panic("unreachable")
	}
	clearBuf(buf2)
	for i := range merkleP {
		*buf2 = append(*buf2, merkleP[i][:]...)
	}
	sMerkle := *buf2
	err := w.repo.insertTransaction(
		ctx, db, &txid, &blockHash, txPos, rawTx, sMerkle,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.repo.insertScriptPubkeyTransaction(
		ctx, db, &sh, &txid,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	var txVout txidVout
	makeTxidVout(&txid, vout, &txVout)
	w.log.Debugf(
		"ADD OUTPUT; height %d; outpoint: %x:%d; sat: %d",
		height,
		txid,
		vout,
		out.Amount,
	)
	err = w.repo.insertUnspentOutput(
		ctx, db, &txVout, out.Amount, &sh,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.walletUpdateIndexes(ctx, db, info)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.refillWallet(info.walletIdx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *W) processInput(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	height int,
	tx *bitcoin.Transaction,
	txPos int,
	in *bitcoin.Input,
	updatedSH map[[32]byte]struct{},
	buf *[]byte,
	buf2 *[]byte,
	txidBuf *[][32]byte,
) error {
	var txVout txidVout
	makeTxidVout(&in.Txid, in.Vout, &txVout)
	var utxoD utxoData
	err := w.repo.selectUnspentOutput(
		ctx, db, &txVout, &utxoD,
	)
	if err != nil && errors.Is(err, errNotFound) {
		return nil
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	updatedSH[utxoD.ScriptPubkeyHash] = struct{}{}
	w.log.Debugf(
		"SPENT OUTPUT; height: %d; outpoint: %x:%d; sat: %d",
		height,
		in.Txid,
		in.Vout,
		utxoD.Satoshi,
	)
	clearBuf(buf)
	blockHash := block.Hash(buf)
	clearBuf(buf)
	txid := tx.Txid(buf)
	clearBuf(buf)
	*buf = tx.Serialize(*buf)
	rawTx := *buf
	clearBuf(buf2)
	clearBuf(txidBuf)
	*txidBuf = block.TXIDs(*txidBuf, buf2)
	pos, merkleP := MerkleProof(*txidBuf, txid)
	if pos != txPos {
		panic("unreachable")
	}
	clearBuf(buf2)
	for i := range merkleP {
		*buf2 = append(*buf2, merkleP[i][:]...)
	}
	sMerkle := *buf2
	err = w.repo.insertTransaction(
		ctx, db, &txid, &blockHash, txPos, rawTx, sMerkle,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.repo.insertScriptPubkeyTransaction(
		ctx, db, &utxoD.ScriptPubkeyHash, &txid,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.repo.deleteUnspentOutput(ctx, db, &txVout, height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *W) walletUpdateIndexes(
	ctx context.Context,
	db sql.Database,
	info scriptPubkeyInfo,
) error {
	wallet := &w.wallets[info.walletIdx]
	switch info.account {
	case receiveAccount:
		if info.pubkeyIdx < wallet.nextReceiveIndex {
			return nil
		}
		err := w.repo.updateWalletIndexes(
			ctx,
			db,
			&wallet.hash,
			info.pubkeyIdx+1,
			wallet.nextChangeIndex,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		wallet.nextReceiveIndex = info.pubkeyIdx + 1
		return nil
	case changeAccount:
		if info.pubkeyIdx < wallet.nextChangeIndex {
			return nil
		}
		err := w.repo.updateWalletIndexes(
			ctx,
			db,
			&wallet.hash,
			wallet.nextReceiveIndex,
			info.pubkeyIdx+1,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		wallet.nextChangeIndex = info.pubkeyIdx + 1
		return nil
	default:
		panic("unreachable")
	}
}

func (w *W) refillWallet(wIdx int) error {
	wl := &w.wallets[wIdx]
	offAndCount := func(
		nextIndex uint32, nDerived uint32,
	) (uint32, uint32) {
		// if nextIndex is 0 and gap is 1, ntarget is 1
		// if nextIndex is 1 and gap is 1, ntarget is 2
		nTarget := nextIndex + gap
		// if nDerived is 0 and nTarget is 1, continue
		// if nDerived is 1 and nTarget is 1, skip
		if nDerived >= nTarget {
			return 0, 0
		}
		// if nDerived is 0, offset is 0
		// if nDerived is 1, offset is 1
		offset := nDerived
		// if nDerived is 1 and nTarget is 2 count is 1
		// if nDerived is 1 and nTarget is 3 count is 2
		// if nDerived is 1 and nTarget is 4 count is 3
		count := nTarget - nDerived
		return offset, count
	}
	addToSKIdx := func(
		kind accountKind,
		nDerivedPrev uint32,
		scriptPubkeys [][]byte,
	) {
		for i, p := range scriptPubkeys {
			w.scriptPubkeys[sha256.Sum256(p)] = scriptPubkeyInfo{
				walletIdx: wIdx,
				account:   kind,
				// if nprev is 1, then next index is 1
				pubkeyIdx: nDerivedPrev + uint32(i),
			}
		}
	}
	offset, count := offAndCount(wl.nextReceiveIndex, wl.nReceiveDerived)
	if count != 0 {
		pubkeys, err := deriveScriptPubkeys(
			wl, receiveAccount, offset, count,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		addToSKIdx(receiveAccount, wl.nReceiveDerived, pubkeys)
		wl.nReceiveDerived += count
	}
	offset, count = offAndCount(wl.nextChangeIndex, wl.nChangeDerived)
	if count != 0 {
		pubkeys, err := deriveScriptPubkeys(
			wl, changeAccount, offset, count,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		addToSKIdx(changeAccount, wl.nChangeDerived, pubkeys)
		wl.nChangeDerived += count
	}
	return nil
}

func deriveScriptPubkeys(
	w *wallet,
	account accountKind,
	offset,
	count uint32,
) ([][]byte, error) {
	var err error
	path := [1]uint32{uint32(account)}
	accountKeys := make([]bip32.ExtendedKey, len(w.masterPubs))
	for i := range w.masterPubs {
		accountKeys[i], err = bip32.DeriveXpub(
			&w.masterPubs[i], path[:],
		)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
	}
	r, err := scriptpubkey.MakeMulti(
		w.kind, w.reqSigs, offset, count, accountKeys,
	)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return r, nil
}

func walletHash(
	kind scriptpubkey.Kind,
	reqsigs byte,
	masterKeys []bip32.ExtendedKey,
	out *[32]byte,
	buf *[]byte,
) {
	var buf2 []byte
	if buf == nil {
		buf = &buf2
	}
	clearBuf(buf)
	*buf = append(*buf, byte(kind))
	*buf = append(*buf, byte(reqsigs))
	for i := range masterKeys {
		*buf = append(*buf, masterKeys[i].Key[:]...)
	}
	*out = sha256.Sum256(*buf)
}

func makeTxidVout(txid *[32]byte, vout uint32, out *txidVout) {
	copy(out[:], txid[:])
	binary.LittleEndian.PutUint32(out[32:], vout)
}

func clearBuf[T any](buf *[]T) {
	*buf = (*buf)[:0]
}

func shStatus(txs []TxData, buf *[]byte) []byte {
	var buf2 []byte
	if buf == nil {
		buf = &buf2
	}
	if len(txs) == 0 {
		return nil
	}
	for i := range txs {
		var hx [64]byte
		slices.Reverse(txs[i].Txid[:])
		hex.Encode(hx[:], txs[i].Txid[:])
		*buf = fmt.Appendf(*buf, `%s:%d:`, hx[:], txs[i].Height)
	}
	hash := sha256.Sum256(*buf)
	clearBuf(buf)
	*buf = append(*buf, hash[:]...)
	return *buf
}

func catDoubleSha256(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:], left[:])
	copy(buf[32:], right[:])
	hash := sha256.Sum256(buf[:])
	return sha256.Sum256(hash[:])
}

func merkleRoot(hashes [][32]byte) [32]byte {
	lvl := make([][32]byte, len(hashes))
	copy(lvl, hashes)
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

func MerkleProof(txids [][32]byte, txid [32]byte) (int, [][32]byte) {
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

type estimateTime struct {
	sampleHeight     int
	sampleTime       time.Time
	remainingHours   int
	remainingMinutes int
}

func updateRemaining(
	currentHeight int, bestHeight int, est *estimateTime,
) {
	if est.sampleTime.IsZero() {
		est.sampleTime = time.Now()
		est.sampleHeight = currentHeight
		est.remainingHours = -1
		est.remainingMinutes = -1
	}
	if time.Since(est.sampleTime) < time.Second*60 {
		return
	}
	blocks := currentHeight - est.sampleHeight
	elapsedSeconds := time.Since(est.sampleTime) / time.Second
	blocksPerSec := float64(blocks) / float64(elapsedSeconds)
	remainingBlocks := bestHeight - currentHeight
	remainingSeconds := float64(remainingBlocks) / blocksPerSec
	remainingHours := int(remainingSeconds) / 60 / 60
	remainingMinutes := int(remainingSeconds/60) % 60
	est.remainingHours = remainingHours
	est.remainingMinutes = remainingMinutes
	est.sampleTime = time.Now()
	est.sampleHeight = currentHeight
}
