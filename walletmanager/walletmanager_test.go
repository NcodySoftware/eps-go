package walletmanager

import (
	"bytes"
	"crypto/sha256"
	"os"
	"slices"
	"testing"

	"github.com/ncodysoftware/eps-go/internal/testdata"
	"github.com/ncodysoftware/eps-go/testutil"
	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
)

func TestIntegration_W(t *testing.T) {
	_, err := os.Stat("/tmp/eps-go/bitcoind-ok")
	if err != nil {
		t.Skip("bitcoind setup not executed")
	}
	tc, cls := testutil.GetTCtx(t)
	defer cls()
	bcli := bitcoin.NewClient(
		tc.C, tc.Cfg.BTCNodeAddr, tc.L, bitcoin.Regtest,
	)
	err = bcli.Start()
	assert.Must(t, err)
	defer bcli.Stop()
	wc := WalletConfig{
		Kind:    scriptpubkey.SK_P2WPKH,
		Reqsigs: 0,
		MasterPubs: []bip32.ExtendedKey{
			testdata.DefaultKeySet.RootAccount,
		},
		Height: 0,
	}
	w, err := New(
		tc.C, tc.D, tc.L, bcli, []WalletConfig{wc}, bitcoin.Regtest,
	)
	assert.Must(t, err)
	defer func() {
		err := w.Close(tc.C)
		assert.Must(t, err)
	}()
	err = w.WaitInit(tc.C)
	assert.Must(t, err)
	//
	recv, err := deriveScriptPubkeys(&w.wallets[0], receiveAccount, 0, 1)
	assert.Must(t, err)
	change, err := deriveScriptPubkeys(&w.wallets[0], changeAccount, 0, 1)
	assert.Must(t, err)
	recvSh := sha256.Sum256(recv[0])
	changeSh := sha256.Sum256(change[0])
	rBalance := tSelectScriptBalance(t, tc, &recvSh)
	cBalance := tSelectScriptBalance(t, tc, &changeSh)
	rNtx := tSelectScriptTxCount(t, tc, &recvSh)
	cNtx := tSelectScriptTxCount(t, tc, &changeSh)
	assert.MustEqual(t, 5000, rBalance/100_000_000)
	assert.MustEqual(t, 49, cBalance/100_000_000)
	assert.MustEqual(t, 1, cNtx)
	assert.MustEqual(t, 102, rNtx)
	assert.MustEqual(t, 2001, w.wallets[0].nReceiveDerived)
	assert.MustEqual(t, 2001, w.wallets[0].nChangeDerived)
}

func TestMerkle(t *testing.T) {
	block := testBlock()
	leaves := make([][32]byte, 0, len(block.Transactions))
	var buf []byte
	for i := range block.Transactions {
		tx := &block.Transactions[i]
		buf = buf[:0]
		txid := tx.Txid(&buf)
		leaves = append(leaves, txid)
	}
	expMerkle := [32]byte(testutil.MustHexDecode("9069c71ae9f343c2cc162cf1c4f57705d0273cad673bb191b6133e826b04f0c5"))
	slices.Reverse(expMerkle[:])
	assert.MustEqual(t, expMerkle, block.MerkleRoot)
	// test merkleRoot
	merkle := merkleRoot(leaves)
	assert.MustEqual(t, expMerkle, merkle)
	txidS := "82cf7f355dfdcc1ccd4423a61f86bf43c7a9ebe5c091eda8bd932141f5db4fda"
	txid := [32]byte(testutil.MustHexDecode(txidS))
	slices.Reverse(txid[:])
	expectPos := 55
	expectMerkleBranchS := []string{
		"2088a30ce59f371d3718adb760404f62cd199d70e4590941767619a94907ebcc",
		"25e0a851821857293ad3820d3c129478ec69a593e5b15d9426df7fa11dbd9070",
		"106c884b2d4512ae8466b261c1412e3f9f7968408c936750442ae02b7999e9ca",
		"0b0dc683e8a95403de3b67b629499bb0a96f2545b53e5171c2346e4a9ad76148",
		"24f56392bd1d53c0fc11f1f46ae8e173deeee82c90b575ca1ce4379e60cc0ba0",
		"3c13953adcd6d6188ddacbb18deea41d77e1f247745747df612275d2e7c149de",
		"0a1c69298e81a1fa224b16c8cc9d9ddc6e77ba2241be6984441aa04a9785a64a",
		"761615ca5c59cdd1068f51cd0cdce51c01df51041e75126df5434695b4dfbf49",
		"0d699e789d01d8e71fd2317e7397f4f427d187ac0eec1ecd00b70ce986cb8e47",
		"d37dabe556550f622614bdef46fb7fa2816e6412fcc15398e0fa84af29894752",
		"d835a6b4d3c975f46c6489b1e88dafa62b350538bed409209b34fa87bfee405c",
		"2c75b749773d84c29c76b78bedcf6f0e36ab0882901cc13bc59f7f0fd9e5bacd",
		"b609c46fd7b00f3b90bb21ab5ab9fdef86871a603dac91124b471df95bf84e6c",
	}
	expectMerkleBranch := make([][32]byte, 0, len(expectMerkleBranchS))
	for i := range expectMerkleBranchS {
		d := testutil.MustHexDecode(expectMerkleBranchS[i])
		slices.Reverse(d)
		var r [32]byte
		copy(r[:], d[:])
		expectMerkleBranch = append(expectMerkleBranch, r)
	}
	// test merkleProof
	pos, proof := MerkleProof(leaves, txid)
	assert.MustEqual(t, expectPos, pos)
	assert.MustEqual(t, expectMerkleBranch, proof)
	// test checkMerkleProof
	ok := checkMerkleProof(txid, pos, block.MerkleRoot, proof)
	assert.MustEqual(t, true, ok)
}

func testBlock() bitcoin.Block {
	rawBlock := testutil.MustHexDecode(
		string(bytes.Trim(testdata.Block919939, "\n")),
	)
	var block bitcoin.Block
	err := block.Deserialize(bytes.NewReader(rawBlock))
	if err != nil {
		panic(err)
	}
	return block
}

func tSelectScriptBalance(
	t *testing.T, tc *testutil.TCtx, sh *[32]byte,
) uint64 {
	s := `
	SELECT COALESCE(SUM(satoshi), 0) FROM unspent_output
	WHERE scriptpubkey_hash = $1;
	`
	var b uint64
	err := tc.D.QueryRow(tc.C, s, sh[:]).Scan(&b)
	assert.Must(t, err)
	return b
}

func tSelectScriptTxCount(t *testing.T, tc *testutil.TCtx, sh *[32]byte) int {
	s := `
	SELECT COUNT(*) FROM scriptpubkey_tx
	WHERE scriptpubkey_hash = $1;
	`
	var b int
	err := tc.D.QueryRow(tc.C, s, sh[:]).Scan(&b)
	assert.Must(t, err)
	return b
}
