package testdata

import (
	"encoding/hex"

	"ncody.com/ncgo.git/bitcoin/bip32"
)

type KeySet struct {
	Bip32Seed []byte
	RootAccount bip32.ExtendedKey
	ReceiveAccount bip32.ExtendedKey
	ChangeAccount bip32.ExtendedKey
}

var DefaultKeySet  KeySet

func init() {
	initDefaultKeyset(&DefaultKeySet)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func initDefaultKeyset(w *KeySet) {
	/*
	Mnemonics:

	arena pig zone defense curious total soap girl soon project devote fall gossip very arctic sample visa attend banana roof midnight tackle carry spray

	first receive scriptpubkey: 0014a6922dd13b979cbfd31054cb913cbb7508601675
	first change scriptpubkey: 001425c7dbc175795ab90a6d4902f3a107cf8cfb9bcc
	*/
	seed := "1843632f1211e9c5c832bf09127b093696407ca4dae86cb35b622554c5a912e589544048e13c89b0dc7140adcad8e387ce51afc11e218037c4f8f40630c7ed30"
	seedBytes, err := hex.DecodeString(seed)
	must(err)
	rt, _, err := bip32.DeriveSeed(
		seedBytes, 
		[]uint32{
			84|bip32.KEY_HARDENED,
			0|bip32.KEY_HARDENED,
			0|bip32.KEY_HARDENED,
		},
	)
	must(err)
	recv, _, err := bip32.DeriveSeed(
		seedBytes,
		[]uint32{
			84|bip32.KEY_HARDENED,
			0|bip32.KEY_HARDENED,
			0|bip32.KEY_HARDENED,
			0,
		},
	)
	must(err)
	chg, _, err := bip32.DeriveSeed(
		seedBytes,
		[]uint32{
			84|bip32.KEY_HARDENED,
			0|bip32.KEY_HARDENED,
			0|bip32.KEY_HARDENED,
			1,
		},
	)
	must(err)
	w.Bip32Seed = seedBytes
	w.ReceiveAccount = recv
	w.ChangeAccount = chg
	w.RootAccount = rt
}
