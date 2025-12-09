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
	"ncody.com/ncgo.git/stackerr"
)

const gap = 2000

type walletAccount struct {
	AccountMaster []bip32.ExtendedKey
	Hash [32]byte
	ScriptKind scriptpubkey.Kind
	Reqsigs byte
	NextIndex int
	NDerived int
	Height int
}

type walletMaster struct {
	CreatedAtHeight int
	ScriptKind scriptpubkey.Kind
	Reqsigs byte
	Master []bip32.ExtendedKey
}

type scriptPubkeyInfo struct {
	ScriptPubkey []byte
	Account int
	Index int
}

type walletManager struct {
	db sql.Database
	accounts [2]walletAccount
	scriptPubkeys map[[32]byte]scriptPubkeyInfo
	bufPool sync.Pool
}

func NewWalletManager(
	ctx context.Context,
	db sql.Database,
	key walletMaster,
) (*walletManager, error) {
	var (
		w walletManager
	)
	w.bufPool.New = func() any {
		return make([]byte, 0)
	}
	for i := range w.accounts {
		err := w.setupAccount(ctx, db, i, &key)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		err = w.refillScriptPubkey(i)
	}
	return &w, nil
}

func (w *walletManager) setupAccount(
	ctx context.Context, db sql.Database, accIdx int, key *walletMaster,
) error {
	path := [1]uint32{uint32(accIdx)}
	var accHash [32]byte
	buf := w.bufPool.Get().([]byte)
	defer w.bufPool.Put(buf[:0])
	accountHash(key.ScriptKind, key.Reqsigs, key.Master, &buf, &accHash)
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
			Height:    max(1, key.CreatedAtHeight-1),
		}
	}
	for _, k := range key.Master {
		account, err := bip32.DeriveXpub(
			&k, path[:],
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		w.accounts[accIdx].AccountMaster = append(
			w.accounts[accIdx].AccountMaster, account,
		)
	}
	w.accounts[accIdx].Hash = accHash
	w.accounts[accIdx].ScriptKind = key.ScriptKind
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

func (w *walletManager) refillScriptPubkey(
	accIdx int,
) error {
	ntarget := gap + w.accounts[accIdx].NextIndex
	if ntarget >= w.accounts[accIdx].NDerived {
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
		w.scriptPubkeys[sha256.Sum256(d)] = scriptPubkeyInfo{
			ScriptPubkey: d,
			Account: accIdx,
			Index:   w.accounts[accIdx].NDerived+i,
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
	blockHeight int,
	blockHash *[32]byte, 
	txid *[32]byte,
	txidVout *[32+4]byte,
	serializedTx []byte,
	satoshi uint64,
) error {
	err := rInsertTransaction(ctx, db, txid, blockHash, serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	err = rInsertUtxo(ctx, db, txidVout, satoshi, sInfo.ScriptPubkey)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	err = rInsertScriptPubkeyTransaction(ctx, db, sHash, txid)
	if err != nil {
		return stackerr.Wrap(err)
	}
	//
	w.accounts[sInfo.Account].NextIndex = max(
		w.accounts[sInfo.Account].NextIndex,
		sInfo.Index,
	)
	err = w.refillScriptPubkey(sInfo.Account)
	if err != nil {
		return stackerr.Wrap(err)
	}
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
	return  nil
}

func (w *walletManager) processSpentUtxo(
	ctx context.Context,
	db sql.Database,
	blockHash *[32]byte,
	txid *[32]byte,
	serializedTx []byte,
	txidVout *[32+4]byte,
) error {
	err := rInsertTransaction(ctx, db, txid, blockHash, serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = rDeleteUtxo(ctx, db, txidVout)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
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
		if w.accounts[i].Height >= height {
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
		var txidVout [32+4]byte
		makeTxidVout(txid, uint32(i), txidVout[:])
		w.processNewUtxo(
			ctx,
			db,
			&sh,
			&sInfo,
			height,
			blockHash,
			txid,
			&txidVout,
			serialized,
			out.Amount,
		)
	}
	for _, in := range tx.Inputs {
		var txidVout [32+4]byte
		makeTxidVout(&in.Txid, in.Vout, txidVout[:])
		var utxo utxoData
		err := rSelectUtxo(ctx, db, &txidVout, &utxo)
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			continue
		} else if err != nil {
			return stackerr.Wrap(err)
		}
		err = w.processSpentUtxo(
			ctx, db, blockHash, txid, serialized, &txidVout,
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
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
	binary.LittleEndian.AppendUint32(out, vout)
} 
