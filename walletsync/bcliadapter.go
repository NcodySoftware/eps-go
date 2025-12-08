package walletsync

import (
	"context"
	"errors"
	"fmt"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/stackerr"
)

var (
	errNoBlocks = errors.New("no blocks")
)

type bcliAdapter struct {
	bcli *bitcoin.Client
	cache map[[32]byte]bitcoin.Header
	buf []byte
}

func NewBcliAdapter(bcli *bitcoin.Client) *bcliAdapter {
	return &bcliAdapter{
		bcli: bcli,
		cache: map[[32]byte]bitcoin.Header{},
	}
}

func (b *bcliAdapter) GetBlock(
	ctx context.Context, prevhash *[32]byte, out *bitcoin.Block,
) error {
	const maxCacheLen = 2000
	var (
		header bitcoin.Header
		headers []bitcoin.Header
		ok bool
		err error
	)
	header, ok = b.cache[*prevhash]
	if !ok {
		headers, err = b.bcli.GetHeaders(
			ctx, [][32]byte{*prevhash}, [32]byte{},
		)
		if err != nil {
			return stackerr.Wrap(err)
		}
		if len(headers) == 0 {
			return errNoBlocks
		}
		if len(headers) + len(b.cache) > maxCacheLen {
			b.cleanCache()
		}
		for i := 0; i < maxCacheLen && i < len(headers); i++ {
			b.cache[headers[i].PreviousBlock] = headers[i]
		}
		header, ok = b.cache[*prevhash]
		if !ok {
			return fmt.Errorf("bad headers returned from bcli")
		}
	} 	
	hash := header.Hash(&b.buf)
	*out, err = b.bcli.GetBlock(ctx, hash)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (b bcliAdapter) cleanCache() {
	for k := range b.cache {
		delete(b.cache, k)
	}
}
