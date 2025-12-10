package walletsync

import (
	"context"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

type utxoData2 struct {
	Satoshi          uint64
	ScriptPubkeyHash [32]byte
}

type utxoManager struct {
	cache map[[32 + 4]byte]utxoData2
}

func newUtxoManager(
	ctx context.Context, db sql.Database,
) (*utxoManager, error) {
	var (
		w     utxoManager
		utxos []utxoData
	)
	err := rSelectAllUtxos(ctx, db, &utxos)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	w.cache = make(map[[32 + 4]byte]utxoData2, len(utxos))
	for _, u := range utxos {
		w.cache[u.TxidVout] = utxoData2{u.Satoshi, u.ScriptPubkeyHash}
	}
	return &w, nil
}

func (self *utxoManager) store(
	ctx context.Context,
	db sql.Database,
	txidVout *[32 + 4]byte,
	satoshi uint64,
	scriptPubkeyHash *[32]byte,
) error {
	err := rInsertUtxo(
		ctx, db, txidVout, satoshi, scriptPubkeyHash,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	self.cache[*txidVout] = utxoData2{
		Satoshi:          satoshi,
		ScriptPubkeyHash: *scriptPubkeyHash,
	}
	return nil
}

func (self *utxoManager) load(
	txidVout *[32 + 4]byte,
	out *utxoData,
) error {
	var (
		ok bool
		ud utxoData2
	)
	ud, ok = self.cache[*txidVout]
	if !ok {
		return errNotFound
	}
	*out = utxoData{*txidVout, ud.Satoshi, ud.ScriptPubkeyHash}
	return nil
}

func (self *utxoManager) delete(
	ctx context.Context,
	db sql.Database,
	txidVout *[32 + 4]byte,
) error {
	err := rDeleteUtxo(ctx, db, txidVout)
	if err != nil {
		return stackerr.Wrap(err)
	}
	delete(self.cache, *txidVout)
	return nil
}
