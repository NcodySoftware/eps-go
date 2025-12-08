package walletmanager

import (
	"bytes"
	"context"
	"errors"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

var (
	errNoMatchingScriptPubkey = errors.New("no matching scriptPubkey")
)


type ct = context.Context
type dt = sql.Database

type walletManager struct {
	network network
	scriptPubkeyMan *scriptPubkeyMan
}

type scriptPubkeyMan struct {
	masterKeys []bip32.ExtendedKey
	scriptKind scriptpubkey.Kind
	scriptPubkeys [][]byte
}

func New() *walletManager {
	return &walletManager{}
}

func (w *walletManager) processBlock(
	ctx ct,
	db dt,
	block *bitcoin.Block,
	buf *[]byte,
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
	err := storeBlockHeader(ctx, db, block, w.network, buf)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i := range block.Transactions {
		processTransaction()
	}
}

type network byte

const (
	main network = iota
	testnet
	regtest
)

var genesisRegtest = [32]byte{
	0x06,0x22,0x6e,0x46,0x11,0x1a,0x0b,0x59,
	0xca,0xaf,0x12,0x60,0x43,0xeb,0x5b,0xbf,
	0x28,0xc3,0x4f,0x3a,0x5e,0x33,0x2a,0x1f,
	0xc7,0xb2,0xb7,0x3c,0xf1,0x88,0x91,0x0f,
}

func getGenesis(n network) *[32]byte {
	switch n {
	case regtest:
		return &genesisRegtest
	default:
		panic("TODO")
	}
}

func storeBlockHeader(
	ctx ct,
	db dt,
	block *bitcoin.Block,
	n network,
	buf *[]byte,
) error {
	var (
		prevBlockHeight int
		err error
		buf2 []byte
	)
	if buf == nil {
		buf = &buf2
	}
	prevBlockHeight, err = rSelectBlockHeight(
		ctx, db, block.PreviousBlock,
	)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		if !bytes.Equal(getGenesis(n)[:], block.PreviousBlock[:]) {
			return stackerr.Wrap(err)
		}
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	bh := bitcoin.Header{
		BlockVersion: block.Version,
		PreviousBlock: block.PreviousBlock,
		MerkleRoot: block.MerkleRoot,
		Timestamp: block.Time,
		NBits: block.Bits,
		Nonce: block.Nonce,
		TxCount: block.TransactionCount,
	}
	if bh.TxCount != 0 {
		bh.Transactions = make([][32]byte, bh.TxCount)
		for i := range bh.TxCount {
			bh.Transactions[i] = block.Transactions[i].Txid(buf)
		}
	}
	bhash := block.Hash()
	err = rInsertBlockHeader(
		ctx,
		db,
		bhash,
		prevBlockHeight+1,
		bh.Serialize((*buf)[:0]),
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func processTransaction(tx) error {
	for i := range tx.Outputs {
		processOutput()
	}
	for i := range tx.Inputs {
		processTxInput()
	}
}

func processTxOutput(
	ctx ct,
	db dt,
	blockhash [32]byte,
	txid [32]byte,
	serializedTx []byte,
	scriptPubkey []byte,
	scriptPubkeyHash [32]byte,
	txidVout [32+4]byte,
	satoshi uint64,
) error {
	info, ok := getScriptPubkeyInfo(scriptPubkey)
	if !ok {
		return nil
	}
	
	err := rInsertTransaction(ctx, db, txid, blockhash, serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = rInsertScriptPubkeyTransaction(ctx, db, scriptPubkeyHash, txid)
	if err != nil {
		return stackerr.Wrap(err)
	}
	err = rInsertUnspentOutput(ctx, db, txidvout, satoshi, scriptPubkey)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func processTxInput(input) error {
}
