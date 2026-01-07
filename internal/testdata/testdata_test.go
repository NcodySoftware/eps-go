package testdata

import (
	"testing"

	"github.com/ncodysoftware/eps-go/testutil"
	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
)

/*
commands to generate the test wallet on bitcoin core

bitcoin-cli --named createwallet wallet_name=test disable_private_keys=false blank=true descriptors=true load_on_startup=true

bitcoin-cli -rpcwallet=test --named importdescriptors requests=[{\"desc\":\"wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/0/*)#f03g2f34\",\"active\":true,\"range\":[0,0],\"next_index\":0,\"timestamp\":\"now\",\"internal\":false},{\"desc\":\"wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/1/*)#cm5fhupd\",\"active\":true,\"range\":[0,0],\"next_index\":0,\"timestamp\":\"now\",\"internal\":true}]

*/

func Test_assertSameWalletIsBeingUsed(t *testing.T) {
	path := []uint32{0}
	frp, err := bip32.DeriveXpub(&DefaultKeySet.ReceiveAccount, path)
	assert.Must(t, err)
	fcp, err := bip32.DeriveXpub(&DefaultKeySet.ChangeAccount, path)
	assert.Must(t, err)
	expFirstReceiveScript := testutil.MustHexDecode(
		"0014a6922dd13b979cbfd31054cb913cbb7508601675",
	)
	expFirstChangeScript := testutil.MustHexDecode(
		"001425c7dbc175795ab90a6d4902f3a107cf8cfb9bcc",
	)
	frs, err := scriptpubkey.Make(
		scriptpubkey.SK_P2WPKH, 1, [][]byte{frp.Key[:]},
	)
	assert.Must(t, err)
	fcs, err := scriptpubkey.Make(
		scriptpubkey.SK_P2WPKH, 1, [][]byte{fcp.Key[:]},
	)
	assert.Must(t, err)
	assert.MustEqual(t, expFirstReceiveScript, frs)
	assert.MustEqual(t, expFirstChangeScript, fcs)
}

func Test_tmp(t *testing.T) {
	t.Log(bip32.ExtendedEncode(DefaultKeySet.RootAccount))
}
