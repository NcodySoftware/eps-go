package scriptpubkey

import (
	"crypto/sha256"
	"github.com/ncodysoftware/eps-go/assert"
	"github.com/ncodysoftware/eps-go/bip32"
	"github.com/ncodysoftware/eps-go/script"
	"github.com/ncodysoftware/eps-go/testutil"
	"testing"
)

func Test_Make(t *testing.T) {
	tests := []struct {
		pubkeys         [][]byte
		requiredSigs    byte
		scriptKind      byte
		expScriptPubkey []byte
	}{
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
			),
			scriptKind: SK_P2PK,
			expScriptPubkey: appendSlices(
				[]byte{script.OP_PUSHBYTES_33},
				testutil.MustHexDecode(
					"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				),
				[]byte{script.OP_CHECKSIG},
			),
		},
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
			),
			scriptKind: SK_P2PKH,
			expScriptPubkey: appendSlices(
				[]byte{
					script.OP_DUP,
					script.OP_HASH160,
					script.OP_PUSHBYTES_20,
				},
				h160(testutil.MustHexDecode(
					"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				)),
				[]byte{script.OP_EQUALVERIFY},
				[]byte{script.OP_CHECKSIG},
			),
		},
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				"0253e261f7b3f416d48caadcc145a0cc0cfa777ea662faa3380e5080e6d8ced958",
				"034dd1542d0e135fab37204d3742e7997ea6738a5362e9c1f9c68b230f0991c19c",
			),
			requiredSigs: 2,
			scriptKind:   SK_P2MS,
			expScriptPubkey: appendSlices(
				[]byte{script.OP_2, script.OP_PUSHBYTES_33},
				testutil.MustHexDecode(
					"0253e261f7b3f416d48caadcc145a0cc0cfa777ea662faa3380e5080e6d8ced958",
				),
				[]byte{script.OP_PUSHBYTES_33},
				testutil.MustHexDecode(
					"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				),
				[]byte{script.OP_PUSHBYTES_33},
				testutil.MustHexDecode(
					"034dd1542d0e135fab37204d3742e7997ea6738a5362e9c1f9c68b230f0991c19c",
				),
				[]byte{script.OP_3},
				[]byte{script.OP_CHECKMULTISIG},
			),
		},
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				"0253e261f7b3f416d48caadcc145a0cc0cfa777ea662faa3380e5080e6d8ced958",
				"034dd1542d0e135fab37204d3742e7997ea6738a5362e9c1f9c68b230f0991c19c",
			),
			scriptKind:   SK_P2SH_MULTISIG,
			requiredSigs: 2,
			expScriptPubkey: appendSlices(
				[]byte{script.OP_HASH160},
				[]byte{script.OP_PUSHBYTES_20},
				h160(appendSlices(
					[]byte{
						script.OP_2,
						script.OP_PUSHBYTES_33,
					},
					testutil.MustHexDecode(
						"0253e261f7b3f416d48caadcc145a0cc0cfa777ea662faa3380e5080e6d8ced958",
					),
					[]byte{script.OP_PUSHBYTES_33},
					testutil.MustHexDecode(
						"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
					),
					[]byte{script.OP_PUSHBYTES_33},
					testutil.MustHexDecode(
						"034dd1542d0e135fab37204d3742e7997ea6738a5362e9c1f9c68b230f0991c19c",
					),
					[]byte{script.OP_3},
					[]byte{script.OP_CHECKMULTISIG},
				)),
				[]byte{script.OP_EQUAL},
			),
		},
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
			),
			scriptKind: SK_P2SH_WPKH,
			expScriptPubkey: appendSlices(
				[]byte{script.OP_HASH160},
				[]byte{script.OP_PUSHBYTES_20},
				h160(appendSlices(
					[]byte{
						script.OP_0,
						script.OP_PUSHBYTES_20,
					},
					h160(testutil.MustHexDecode(
						"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
					)),
				)),
				[]byte{script.OP_EQUAL},
			),
		},
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
			),
			scriptKind: SK_P2WPKH,
			expScriptPubkey: appendSlices(
				[]byte{script.OP_0, script.OP_PUSHBYTES_20},
				h160(testutil.MustHexDecode(
					"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				)),
			),
		},
		{
			pubkeys: mustHexDecodeMulti(
				"031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c",
				"0253e261f7b3f416d48caadcc145a0cc0cfa777ea662faa3380e5080e6d8ced958",
				"034dd1542d0e135fab37204d3742e7997ea6738a5362e9c1f9c68b230f0991c19c",
			),
			scriptKind:   SK_P2WSH_MULTISIG,
			requiredSigs: 2,
			expScriptPubkey: appendSlices(
				[]byte{script.OP_0, script.OP_PUSHBYTES_32},
				s256(appendSlices(
					[]byte{script.OP_2, script.OP_PUSHBYTES_33},
					testutil.MustHexDecode("0253e261f7b3f416d48caadcc145a0cc0cfa777ea662faa3380e5080e6d8ced958"),
					[]byte{script.OP_PUSHBYTES_33},
					testutil.MustHexDecode("031f1239ab686edaa31971f16eceef4b66a0844d052367f52d3c6cbc8f7a9ca49c"),
					[]byte{script.OP_PUSHBYTES_33},
					testutil.MustHexDecode("034dd1542d0e135fab37204d3742e7997ea6738a5362e9c1f9c68b230f0991c19c"),
					[]byte{script.OP_3, script.OP_CHECKMULTISIG},
				)),
			),
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			spk, err := Make(
				test.scriptKind,
				test.requiredSigs,
				test.pubkeys,
			)
			assert.Must(t, err)
			testutil.MustEqualHex(t, test.expScriptPubkey, spk)
		})
	}
}

func Test_MakeMulti(t *testing.T) {
	priv := testutil.MustHexDecode(
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	)
	key0, _, err := bip32.DeriveSeed(priv, []uint32{0 | bip32.KEY_HARDENED})
	assert.Must(t, err)
	_, key1, err := bip32.DeriveSeed(priv, []uint32{1 | bip32.KEY_HARDENED})
	assert.Must(t, err)
	key2, _, err := bip32.DeriveSeed(priv, []uint32{2 | bip32.KEY_HARDENED})
	assert.Must(t, err)

	tests := []struct {
		baseKeys   []*bip32.ExtendedKey
		scriptKind byte
		reqsigs    byte
		offset     uint32
		count      uint32
		exp        [][]byte
	}{
		{
			baseKeys:   []*bip32.ExtendedKey{&key0, &key1, &key2},
			scriptKind: SK_P2WSH_MULTISIG,
			reqsigs:    2,
			offset:     1,
			count:      5,
			exp: [][]byte{
				testutil.MustHexDecode("0020f8dbe3d2577db3bb62bad4b6403f3d0084f22fe554db3d967158b007c50fbbb4"),
				testutil.MustHexDecode("0020f596b6962a60c7484b90de53d007ac1a594056f11a3da993a1423c36f0d5c5c9"),
				testutil.MustHexDecode("00205fd14b264815635d5a55f163b7baaaeea5822e31bcb48412fa42f8530913fcb0"),
				testutil.MustHexDecode("0020eae114ba976aa8964ff37c35842d21dfddb194e75e4b8a2e17554b792d7e5190"),
				testutil.MustHexDecode("00205e26f803fb8a15f900ea96068e5f45b6cbb134ff0f95eb3d1ecded95ff77b3b8"),
			},
		},
		{
			baseKeys:   []*bip32.ExtendedKey{&key0},
			scriptKind: SK_P2WPKH,
			offset:     1,
			count:      5,
			exp: [][]byte{
				testutil.MustHexDecode("00148533a30c4725acaad1e1d3a6385251640489f941"),
				testutil.MustHexDecode("00148cb50de7184b1bfa5545f8914b9b299a6fb7c4cf"),
				testutil.MustHexDecode("001437f461535fdb6a443aa9640abacaa5ad767dd963"),
				testutil.MustHexDecode("0014cde8f0aa43d5badf31e8982db8228a1b35853b95"),
				testutil.MustHexDecode("00140527bf353abe79e7d3a594338e684de198cda6d7"),
			},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			spks, err := MakeMulti(
				test.scriptKind,
				test.reqsigs,
				test.offset,
				test.count,
				test.baseKeys,
			)
			assert.Must(t, err)
			assert.MustEqual(t, test.exp, spks)
		})
	}
}

func s256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func h160(data []byte) []byte {
	h := hash160(data)
	return h[:]
}

func appendSlices(slicesData ...[]byte) []byte {
	var tlen int
	for _, s := range slicesData {
		tlen += len(s)
	}
	r := make([]byte, 0, tlen)
	for _, s := range slicesData {
		r = append(r, s...)
	}
	return r
}

func mustHexDecodeMulti(xs ...string) [][]byte {
	r := make([][]byte, 0, len(xs))
	for _, x := range xs {
		r = append(r, testutil.MustHexDecode(x))
	}
	return r
}
