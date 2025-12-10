package walletsync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ncodysoftware/eps-go/testutil"
	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/bitcoin"
)

func TestIntegration_bcliAdapter(t *testing.T) {
	tc, cls := testutil.GetTCtx(t)
	defer cls()
	var (
		wg     sync.WaitGroup
		cancel func()
	)
	defer wg.Wait()
	tc.C, cancel = context.WithCancel(tc.C)
	defer cancel()
	bcli := bitcoin.NewClient(
		tc.C, tc.Cfg.BTCNodeAddr, tc.L, bitcoin.MagicRegtest,
	)
	wg.Go(func() {
		err := bcli.Start()
		assert.Must(t, err)
	})
	bg := NewBcliAdapter(bcli)
	var _ blockGetter = bg
	bh := genesisBlockHash[regtest]
	time.Sleep(time.Second)
	var buf []byte
	for range 200 {
		var blk bitcoin.Block
		err := bg.GetBlock(tc.C, &bh, &blk)
		if err != nil && errors.Is(err, errNoBlocks) {
			break
		}
		assert.Must(t, err)
		assert.MustEqual(t, bh, blk.PreviousBlock)
		bh = blk.Hash(&buf)
	}
}
