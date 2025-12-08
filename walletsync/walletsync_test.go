package walletsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ncodysoftware/eps-go/testutil"
	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/database/sql"
)

func TestIntegration_walletsync(t *testing.T) {
	tc, cls := testutil.GetTCtx(t)
	defer cls()
	bg := newFakeBlockGetter(t)
	th := newFakeTxHandler(t)
	var cpt blockData 
	err := lastBlockData(tc.C, tc.D, regtest, &cpt)
	assert.Must(t, err)
	s := NewSynchronizer(tc.D, bg, []transactionHandler{th}, &cpt)
	err = s.Run(tc.C)
	assert.MustEqual(
		t, true, errors.Is(err, context.Canceled),
	)
	var count int64
	query := `
	SELECT COUNT(*)
	FROM blockheader
	;
	`
	row := tc.D.QueryRow(tc.C, query)
	err = row.Scan(&count)
	assert.Must(t, err)
	assert.MustEqual(t, 2, count)
}

type fakeBlockGetter struct {
	blocks []bitcoin.Block
}

func newFakeBlockGetter(t *testing.T) *fakeBlockGetter {
	rawBlk0 := testutil.MustHexDecode(
		"0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4adae5494dffff7f20020000000101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4d04ffff001d0104455468652054696d65732030332f4a616e2f32303039204368616e63656c6c6f72206f6e206272696e6b206f66207365636f6e64206261696c6f757420666f722062616e6b73ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000",
	)
	rawBlk1 := testutil.MustHexDecode(
		"0000002006226e46111a0b59caaf126043eb5bbf28c34f3a5e332a1fc7b2b73cf188910f367b600012ff9328bf557a060ea2f7228d55acdae4d50a15bb14ce34eb197425d3e32969ffff7f200300000001020000000001010000000000000000000000000000000000000000000000000000000000000000ffffffff025100feffffff0200f2052a01000000160014def499d01bb1d20ba0aa48c5697ad620bbc3feae0000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf90120000000000000000000000000000000000000000000000000000000000000000000000000",
	)
	rawBlk2 := testutil.MustHexDecode(
		"0000002022774aab29b7d0d1d35a698285e873e6566acc51217fb01e0d5b70685d1afb3beb6c69d3ea7c5b22d9131e63d382601d965b865ab00108bfc63a8131ad40214ed4e32969ffff7f200000000001020000000001010000000000000000000000000000000000000000000000000000000000000000ffffffff025200feffffff0200f2052a01000000160014def499d01bb1d20ba0aa48c5697ad620bbc3feae0000000000000000266a24aa21a9ede2f61c3f71d1defd3fa999dfa36953755c690689799962b48bebd836974e8cf90120000000000000000000000000000000000000000000000000000000000000000001000000",
	)
	var (
		blocks = []bitcoin.Block{2: {}}
		err error
	)
	err = blocks[0].Deserialize(bytes.NewReader(rawBlk0))
	assert.Must(t, err)
	err = blocks[1].Deserialize(bytes.NewReader(rawBlk1))
	assert.Must(t, err)
	err = blocks[2].Deserialize(bytes.NewReader(rawBlk2))
	assert.Must(t, err)
	return &fakeBlockGetter{
		blocks: []bitcoin.Block(blocks),
	}
}

func (f *fakeBlockGetter) GetBlock(
	ctx context.Context,
	prevHash *[32]byte,
	out *bitcoin.Block,
) error {
	var (
		i int
		b bitcoin.Block
	)
	for i, b = range f.blocks {
		if b.PreviousBlock != *prevHash {
			continue
		}
		*out = b
		return nil
	}
	if i != len(f.blocks)-1 {
		return fmt.Errorf("unknown block")
	}
	return context.Canceled
}

type fakeTxHandler struct {
	t *testing.T
}

func newFakeTxHandler(t *testing.T) *fakeTxHandler {
	return &fakeTxHandler{ t }
}

func (ft *fakeTxHandler) HandleTransaction(
	ctx context.Context, 
	db sql.Database,
	height int,
	blockHash *[32]byte,
	txid *[32]byte,
	tx *bitcoin.Transaction,
) error {
	return nil
}
