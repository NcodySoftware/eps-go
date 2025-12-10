package walletsync

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

const gap = 2000

var errNotFound = errors.New("not found")

type walletAccount struct {
	AccountMaster []bip32.ExtendedKey
	Hash          [32]byte
	ScriptKind    scriptpubkey.Kind
	Reqsigs       byte
	NextIndex     int
	NDerived      int
	Height        int
}

type walletParams struct {
	CreatedAtHeight int
	ScriptKind      scriptpubkey.Kind
	Reqsigs         byte
	KeySet          []bip32.ExtendedKey
}

type scriptPubkeyInfo struct {
	ScriptPubkey []byte
	Account      int
	Index        int
}

type walletManager struct {
	db            sql.Database
	accounts      [2]walletAccount
	bufPool       sync.Pool
	log           *log.Logger
	scriptPubkeys map[[32]byte]scriptPubkeyInfo
	utxoMan       *utxoManager
}

func NewWalletManager(
	ctx context.Context,
	db sql.Database,
	key walletParams,
	log *log.Logger,
) (*walletManager, error) {
	var (
		w   walletManager
		err error
	)
	w.bufPool.New = func() any {
		return make([]byte, 0)
	}
	w.utxoMan, err = newUtxoManager(ctx, db)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	w.log = log
	w.scriptPubkeys = make(map[[32]byte]scriptPubkeyInfo)
	for i := range w.accounts {
		err := w.setupAccount(ctx, db, i, &key)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		err = w.refillScriptPubkeys(i)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
	}
	return &w, nil
}

func (w *walletManager) HandleTransaction(
	ctx context.Context,
	db sql.Database,
	height int,
	blockHash *[32]byte,
	txid *[32]byte,
	tx *bitcoin.Transaction,
) error {
	needSync := false
	for i := range w.accounts {
		if w.accounts[i].Height > height {
			continue
		}
		needSync = true
		w.accounts[i].Height = height
		err := rUpdateAccount(
			ctx,
			db,
			&w.accounts[i].Hash,
			w.accounts[i].NextIndex,
			height,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	if !needSync {
		return nil
	}
	serialized := tx.Serialize(w.bufPool.Get().([]byte))
	defer w.bufPool.Put(serialized[:0])
	for i, out := range tx.Outputs {
		sh := sha256.Sum256(out.ScriptPubkey)
		sInfo, ok := w.scriptPubkeys[sh]
		if !ok {
			continue
		}
		w.log.Infof(
			"NEW UTXO: block %d; txid: %x; scriptPubkey: %x; satoshi: %d",
			height,
			txid[:],
			out.ScriptPubkey,
			out.Amount,
		)
		var txidVout [32 + 4]byte
		makeTxidVout(txid, uint32(i), txidVout[:0])
		err := w.processNewUtxo(
			ctx,
			db,
			&sh,
			&sInfo,
			blockHash,
			txid,
			&txidVout,
			serialized,
			out.Amount,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	for _, in := range tx.Inputs {
		var txidVout [32 + 4]byte
		makeTxidVout(&in.Txid, in.Vout, txidVout[:0])
		var utxo utxoData
		err := w.utxoMan.load(&txidVout, &utxo)
		if err == errNotFound {
			continue
		} else if err != nil {
			return stackerr.Wrap(err)
		}
		w.log.Infof(
			"UTXO SPENT: block %d; txid: %x; scriptPubkey: %x; satoshi: %d",
			height,
			txid[:],
			w.scriptPubkeys[utxo.ScriptPubkeyHash].ScriptPubkey,
			utxo.Satoshi,
		)
		err = w.processSpentUtxo(
			ctx,
			db,
			blockHash,
			txid,
			serialized,
			&utxo.ScriptPubkeyHash,
			&txidVout,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (w *walletManager) setupAccount(
	ctx context.Context,
	db sql.Database,
	accIdx int,
	walletParams *walletParams,
) error {
	path := [1]uint32{uint32(accIdx)}
	accountKeys := make([]bip32.ExtendedKey, 0, len(walletParams.KeySet))
	for _, k := range walletParams.KeySet {
		account, err := bip32.DeriveXpub(
			&k, path[:],
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		accountKeys = append(
			w.accounts[accIdx].AccountMaster, account,
		)
	}
	var accHash [32]byte
	buf := w.bufPool.Get().([]byte)
	defer w.bufPool.Put(buf[:0])
	accountHash(
		walletParams.ScriptKind,
		walletParams.Reqsigs,
		accountKeys,
		&buf,
		&accHash,
	)
	var accData accountData
	var isNewAcc bool
	err := rSelectAccountData(ctx, db, &accHash, &accData)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		isNewAcc = true
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	if isNewAcc {
		accData = accountData{
			Hash:      accHash,
			NextIndex: 0,
			Height:    max(1, walletParams.CreatedAtHeight-1),
		}
	}
	w.accounts[accIdx].AccountMaster = accountKeys
	w.accounts[accIdx].Hash = accHash
	w.accounts[accIdx].ScriptKind = walletParams.ScriptKind
	w.accounts[accIdx].NextIndex = accData.NextIndex
	w.accounts[accIdx].Height = accData.Height
	if !isNewAcc {
		return nil
	}
	err = rInsertAccountData(ctx, db, &accData.Hash, accData.Height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) refillScriptPubkeys(
	accIdx int,
) error {
	ntarget := gap + w.accounts[accIdx].NextIndex
	if ntarget <= w.accounts[accIdx].NDerived {
		return nil
	}
	var offset uint32 = uint32(w.accounts[accIdx].NDerived)
	var count uint32 = uint32(ntarget) - uint32(w.accounts[accIdx].NDerived)
	var baseKeys = make(
		[]*bip32.ExtendedKey, len(w.accounts[accIdx].AccountMaster),
	)
	for i := range baseKeys {
		baseKeys[i] = &w.accounts[accIdx].AccountMaster[i]
	}
	derived, err := scriptpubkey.MakeMulti(
		w.accounts[accIdx].ScriptKind,
		w.accounts[accIdx].Reqsigs,
		offset,
		count,
		baseKeys,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i, d := range derived {
		if w.accounts[accIdx].NDerived == 0 && i < 3 {
			w.log.Infof(
				"account: %d, scripubkey: %d, %x, %x",
				accIdx,
				i,
				d,
				sha256.Sum256(d),
			)
		}
		w.scriptPubkeys[sha256.Sum256(d)] = scriptPubkeyInfo{
			ScriptPubkey: d,
			Account:      accIdx,
			Index:        w.accounts[accIdx].NDerived + i,
		}
	}
	w.accounts[accIdx].NDerived += int(count)
	return nil
}

func (w *walletManager) processNewUtxo(
	ctx context.Context,
	db sql.Database,
	sHash *[32]byte,
	sInfo *scriptPubkeyInfo,
	blockHash *[32]byte,
	txid *[32]byte,
	txidVout *[32 + 4]byte,
	serializedTx []byte,
	satoshi uint64,
) error {
	err := rInsertTransaction(ctx, db, txid, blockHash, serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	err = w.utxoMan.store(ctx, db, txidVout, satoshi, sHash)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	err = rInsertScriptPubkeyTx(ctx, db, sHash, txid)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	w.accounts[sInfo.Account].NextIndex = max(
		w.accounts[sInfo.Account].NextIndex,
		sInfo.Index+1,
	)
	//
	err = w.refillScriptPubkeys(sInfo.Account)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	err = rUpdateAccount(
		ctx,
		db,
		&w.accounts[sInfo.Account].Hash,
		w.accounts[sInfo.Account].NextIndex,
		w.accounts[sInfo.Account].Height,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) processSpentUtxo(
	ctx context.Context,
	db sql.Database,
	blockHash *[32]byte,
	txid *[32]byte,
	serializedTx []byte,
	spentPubkeyHash *[32]byte,
	spentTxidVout *[32 + 4]byte,
) error {
	err := rInsertTransaction(ctx, db, txid, blockHash, serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.utxoMan.delete(ctx, db, spentTxidVout)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = rInsertScriptPubkeyTx(ctx, db, spentPubkeyHash, txid)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func accountHash(
	kind scriptpubkey.Kind,
	reqSigs byte,
	masterKeys []bip32.ExtendedKey,
	buf *[]byte,
	dest *[32]byte,
) {
	var (
		buf2 []byte
	)
	if buf == nil {
		buf = &buf2
	}
	*buf = (*buf)[:0]
	*buf = append(*buf, byte(kind))
	*buf = append(*buf, reqSigs)
	for _, k := range masterKeys {
		*buf = append(*buf, k.Key[:]...)
	}
	*dest = sha256.Sum256(*buf)
}

func makeTxidVout(txid *[32]byte, vout uint32, out []byte) {
	out = append(out, txid[:]...)
	out = binary.LittleEndian.AppendUint32(out, vout)
}
