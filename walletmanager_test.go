package main

import (
	"encoding/hex"
	"github.com/ncodysoftware/eps-go/assert"
	"github.com/ncodysoftware/eps-go/os2"
	"os"
	"testing"
)

func Test_txidVout(t *testing.T) {
	var txid [32]byte
	for i := range txid {
		txid[i] = 0xAA
	}
	var vout uint = 0xBBBBBBBBBBBBBBBB
	v := makeTxidVout(txid, int(vout))
	decTxid, decVout := v.decode()
	assert.MustEqual(t, txid, decTxid)
	assert.MustEqual(t, int(vout), decVout)
}

func Test_status(t *testing.T) {
	jstr := `[{"tx_hash": "b797e92fdfed4becd77828b2468fc003739db52296abcfb76650ff2fcfded219", "height": 102}, {"tx_hash": "9780cbb805c7980902efd941178513942a599bee5260b3f3bc46f2c52c1c6645", "height": 0, "fee": 1000}]`
	var jtxs []struct {
		TxHash string `json:"tx_hash"`
		Height int    `json:"height"`
	}
	mustUnmarshal(t, []byte(jstr), &jtxs)
	exp := "afb1b63af69c37d133830d859e4206a69abffb61f990f32538152b1a089b69b2"
	wtxs := make([]walletTx, 0, len(jtxs))
	for _, jtx := range jtxs {
		wtxs = append(wtxs, walletTx{
			blockHash:  blockHash{},
			height:     jtx.Height,
			blockIndex: 0,
			feeSats:    0,
			txid:       hexDecodeReverse(jtx.TxHash),
		})
	}
	status := scriptHashStatus2(wtxs)
	statusStr := hex.EncodeToString(status[:])
	assert.MustEqual(t, exp, statusStr)
}

func testBlock(t *testing.T) block {
	rpath := os2.MustEnv("ROOT_PATH")
	path := rpath + "/testdata/919939.json"
	data, err := os.ReadFile(path)
	assert.Must(t, err)
	var b block
	mustUnmarshal(t, data, &b)
	return b
}

func Test_merkle(t *testing.T) {
	block := testBlock(t)
	expMerkleRoot := hexDecodeReverse(block.MerkleRoot)
	txids := hexHashesToBytes(block.Tx)
	mroot := merkleRoot(txids)
	assert.MustEqual(t, expMerkleRoot, mroot)
}

func Test_checkMerkleProof(t *testing.T) {
	block := testBlock(t)
	txid := "82cf7f355dfdcc1ccd4423a61f86bf43c7a9ebe5c091eda8bd932141f5db4fda"
	expectPos := 55
	root := hexDecodeReverse(block.MerkleRoot)
	expectMerkle := []string{
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
	var decodedMerkle [][32]byte = hexHashesToBytes(expectMerkle)
	decodedTxid := hexDecodeReverse(txid)
	assert.MustEqual(
		t,
		true,
		checkMerkleProof(
			decodedTxid, expectPos, root, decodedMerkle,
		),
	)
}

func Test_merkleProof(t *testing.T) {
	block := testBlock(t)
	txid := "82cf7f355dfdcc1ccd4423a61f86bf43c7a9ebe5c091eda8bd932141f5db4fda"
	expectPos := 55
	txids := hexHashesToBytes(block.Tx)
	expectMerkle := []string{
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
	decodedMerkle := hexHashesToBytes(expectMerkle)
	pos, merkle := merkleProof(txids, hexDecodeReverse(txid))
	assert.MustEqual(t, expectPos, pos)
	assert.MustEqual(t, len(decodedMerkle), len(merkle))
	assert.MustEqual(t, decodedMerkle, merkle)
}
