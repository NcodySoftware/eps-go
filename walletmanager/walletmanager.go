package walletmanager

import (
	"context"
	"crypto/sha256"
	"errors"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

var (
	errNotFound = errors.New("not found")
)

type walletManager struct {
	wallets []wallet
	scriptPubkeyHashIndex map[[32]byte]scriptPubkeyInfo
	db sql.Database
}

func (w *walletManager) processBlock(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
) error {
	/*
	for each block:
		save blockheader
		for each transaction:
			for each output matching scriptpubkey:
				update receive/change index
				refill receive/change scriptpubkeys
				save transaction
				save output
				save scriptpubkey_transaction
			for each input spending matching utxo:
				save transaction
				delete output
				save scriptpubkey_transaction
				
	*/
	panic("TODO"); err := w.storeBlockHeader(ctx, db, &storeBlockHeaderData{})
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i, _ := range block.Transactions {
		err := w.processTransaction(
			ctx, db, block, &block.Transactions[i],
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

type storeBlockHeaderData struct {
	Hash [32]byte
	Height int
	Serialized []byte
}

func (w *walletManager) storeBlockHeader(
	ctx context.Context, db sql.Database, data *storeBlockHeaderData,
) error {
	s := `
	INSERT INTO blockheader
	(hash, height, data)
	VALUES
	($1, $2, $3)
	;
	`
	_, err := db.Exec(
		ctx, s, data.Hash[:], data.Height, data.Serialized,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) processTransaction(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	tx *bitcoin.Transaction,
) error {
	for vout := range tx.Outputs {
		err := w.processOutput(
			ctx, db, block, tx, vout, &tx.Outputs[vout],
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	for vin := range tx.Inputs {
		err := w.processInput(ctx, db, block, tx, &tx.Inputs[vin])
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	panic("TODO")
}

func (w *walletManager) processOutput(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	tx *bitcoin.Transaction,
	vout int,
	output *bitcoin.Output,
) error {
	/*
	for each output matching scriptpubkey:
		update receive/change index
		refill receive/change scriptpubkeys
		save transaction
		save output
		save scriptpubkey_transaction
	*/
	info, err := w.getScriptPubkeyInfo(output.ScriptPubkey)
	if err == errNotFound {
		return nil
	}
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.refillAccount(ctx, db, info)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.storeTransaction(ctx, db, block, tx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.storeUnspent(ctx, db, tx, vout, output)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.storeScriptPubkeyTransaction(ctx, db, output.ScriptPubkey, tx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) getScriptPubkeyInfo(
	scriptpubkey []byte,
) (scriptPubkeyInfo, error) {
	info, ok := w.scriptPubkeyHashIndex[sha256.Sum256(scriptpubkey)]
	if !ok {
		return info, errNotFound
	}
	return info, nil
}

func (w *walletManager) refillAccount(
	ctx context.Context, db sql.Database, info scriptPubkeyInfo,
) error {
	panic("TODO")
}

func (w *walletManager) storeTransaction(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	tx *bitcoin.Transaction,
) error {
	panic("TODO")
}

func (w *walletManager) storeUnspent(
	ctx context.Context,
	db sql.Database,
	tx *bitcoin.Transaction,
	vout int,
	output *bitcoin.Output,
) error {
	panic("TODO")
}

func (w *walletManager) storeScriptPubkeyTransaction(
	ctx context.Context,
	db sql.Database,
	scriptPubkey []byte,
	tx *bitcoin.Transaction,
) error {
	panic("TODO")
}

func (w *walletManager) processInput(
	ctx context.Context,
	db sql.Database,
	block *bitcoin.Block,
	tx *bitcoin.Transaction,
	input *bitcoin.Input,
) error {
	/*
	for each input spending matching utxo:
		save transaction
		delete output
		save scriptpubkey_transaction
	*/
	output, err := w.getUnspent(ctx, db, input.Txid, int(input.Vout))
	if err == errNotFound {
		return nil
	} 
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.storeTransaction(ctx, db, block, tx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.deleteOutput(ctx, db, input.Txid, int(input.Vout))
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = w.storeScriptPubkeyTransaction(ctx, db, output.ScriptPubkey, tx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (w *walletManager) getUnspent(
	ctx context.Context,
	db sql.Database,
	txid [32]byte,
	vout int,
) (unspentOutput, error) {
	panic("TODO")
}

func (w *walletManager) deleteOutput(
	ctx context.Context,
	db sql.Database,
	txid [32]byte,
	vout int,
) error {
	panic("TODO")
}

type wallet struct {
	Accounts [2]walletAccount
}

type walletAccount struct {
	ScriptPubkeys [][]byte
	BaseKeys []bip32.ExtendedKey
	ScriptKind byte
	NextIndex int
}

type scriptPubkeyInfo struct {
	WalletIdx int
	AccountIdx int
	ScriptPubkeyIdx int
}

type unspentOutput struct {
	Txid [32]byte
	Vout int
	Satoshi uint64
	ScriptPubkey []byte
}
