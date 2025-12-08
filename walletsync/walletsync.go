package walletsync

import (
	"context"
	"errors"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

type blockGetter interface {
	GetBlock(
		ctx context.Context, prevhash *[32]byte, out *bitcoin.Block,
	) error
}

type transactionHandler interface {
	HandleTransaction(
		ctx context.Context,
		db sql.Database,
		height int,
		blockHash *[32]byte,
		txid *[32]byte,
		tx *bitcoin.Transaction,
	) error
}

type synchronizer struct {
	db sql.Database
	bGetter blockGetter
	txHandlers []transactionHandler 
	blockBuf []byte
	checkpoint *blockData
}

func NewSynchronizer(
	db sql.Database,
	bGetter blockGetter,
	txHandlers []transactionHandler,
	checkpoint *blockData,
) *synchronizer {
	return &synchronizer{
		db: db,
		bGetter: bGetter,
		txHandlers: txHandlers,
		checkpoint: checkpoint,
	}
}

func (s *synchronizer) Run(ctx context.Context) error {
	for {
		var (
			block bitcoin.Block
			blockHash [32]byte
			err error
		)
		err = s.bGetter.GetBlock(ctx, &s.checkpoint.Hash, &block)
		if err != nil {
			return stackerr.Wrap(err)
		}
		blockHash = block.Hash(&s.blockBuf)
		err = sql.Execute(ctx, s.db, func(db sql.Database) error {
			return s.processBlock(
				ctx,
				db,
				&blockHash,
				s.checkpoint.Height+1,
				&block,
			)
		})
		if err != nil {
			return stackerr.Wrap(err)
		}
		s.checkpoint.Height++
		s.checkpoint.Hash = blockHash
	}
}

func (s *synchronizer) processBlock(
	ctx context.Context,
	db sql.Database,
	blockHash *[32]byte,
	height int,
	block *bitcoin.Block,
) error {
	bh := bitcoin.Header{
		BlockVersion:  block.Version,
		PreviousBlock: block.PreviousBlock,
		MerkleRoot:    block.MerkleRoot,
		Timestamp:     block.Time,
		NBits:         block.Bits,
		Nonce:         block.Nonce,
		TxCount:       block.TransactionCount,
		Transactions:  make([][32]byte, block.TransactionCount),
	}
	for i := range bh.TxCount {
		s.blockBuf = s.blockBuf[:0]
		bh.Transactions[i] = block.Transactions[i].Txid(&s.blockBuf)
	}
	serializedHeader := bh.Serialize(s.blockBuf)
	err := rInsertBlockHeader(ctx, db, blockHash, height, serializedHeader)
	if err != nil {
		return stackerr.Wrap(err)
	}
	for i := range block.Transactions {
		for _, txHandler := range s.txHandlers {
			err := txHandler.HandleTransaction(
				ctx,
				db,
				height,
				blockHash,
				&bh.Transactions[i],
				&block.Transactions[i],
			)
			if err != nil {
				return stackerr.Wrap(err)
			}
		}
	}
	return nil
}

type network byte

const (
	main network = iota
	testnet
	regtest
)

var genesisBlockHash = [3][32]byte{
	{}, // TODO
	{}, // TODO
	{
		0x06,0x22,0x6e,0x46,0x11,0x1a,0x0b,0x59,
		0xca,0xaf,0x12,0x60,0x43,0xeb,0x5b,0xbf,
		0x28,0xc3,0x4f,0x3a,0x5e,0x33,0x2a,0x1f,
		0xc7,0xb2,0xb7,0x3c,0xf1,0x88,0x91,0x0f,
	},
}

func lastBlockData(
	ctx context.Context, db sql.Database, n network, out *blockData,
) error {
	err := rSelectLastBlockHeaderData(ctx, db, out) 
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		out.Height = 0
		out.Hash = genesisBlockHash[int(n)]
	} else if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}
