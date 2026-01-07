package scriptpubkey

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/script"
	"ncody.com/ncgo.git/crypto/ripemd160"
	"ncody.com/ncgo.git/stackerr"
)

type Kind byte

const (
	MaxMultisigKeys = 16
)

const (
	sK_NONE Kind = iota
	SK_P2PK
	SK_P2PKH
	SK_P2MS
	SK_P2SH_MULTISIG
	SK_P2SH_WPKH
	SK_P2WPKH
	SK_P2WSH_MULTISIG
)

func KindFromString(data string) (Kind, bool) {
	switch strings.ToLower(data) {
	case "p2pk":
		return SK_P2PK, true
	case "p2pkh":
		return SK_P2PKH, true
	case "p2ms":
		return SK_P2MS, true
	case "p2sh":
		return SK_P2SH_MULTISIG, true
	case "p2sh_wpkh":
		return SK_P2SH_WPKH, true
	case "p2wpkh":
		return SK_P2WPKH, true
	case "p2wsh":
		return SK_P2WSH_MULTISIG, true
	default:
		return sK_NONE, false
	}
}

func Make(scriptKind Kind, reqsigs byte, pubkeys [][]byte) ([]byte, error) {
	switch scriptKind {
	case SK_P2PK:
		return p2pk(pubkeys[0])
	case SK_P2PKH:
		return p2pkh(pubkeys[0])
	case SK_P2MS:
		return p2ms(reqsigs, bytesSorted(pubkeys))
	case SK_P2SH_MULTISIG:
		return p2shMultisig(reqsigs, bytesSorted(pubkeys))
	case SK_P2SH_WPKH:
		return p2shWpkh(pubkeys[0])
	case SK_P2WPKH:
		return p2wpkh(pubkeys[0])
	case SK_P2WSH_MULTISIG:
		return p2wshMulti(reqsigs, bytesSorted(pubkeys))
	default:
		return nil, fmt.Errorf("bad scriptKind")
	}
}

func MakeMulti(
	scriptKind Kind,
	reqsigs byte,
	offset uint32,
	count uint32,
	baseKeys []bip32.ExtendedKey,
) ([][]byte, error) {
	var (
		basePubKeys   []bip32.ExtendedKey
		scriptPubkeys [][]byte = make([][]byte, 0, int(count))
	)
	for i := range baseKeys {
		if baseKeys[i].Key[0] == 0x00 {
			pubkey, err := bip32.XpubFromXprv(&baseKeys[i])
			if err != nil {
				return nil, stackerr.Wrap(err)
			}
			basePubKeys = append(basePubKeys, pubkey)
			continue
		}
		basePubKeys = append(basePubKeys, baseKeys[i])
	}
	for i := range count {
		var pubkeys [][]byte
		dpath := [1]uint32{i + offset}
		for _, k := range basePubKeys {
			extPubkey, err := bip32.DeriveXpub(&k, dpath[:])
			if err != nil {
				return nil, stackerr.Wrap(err)
			}
			pubkeys = append(pubkeys, extPubkey.Key[:])
		}
		scriptPubkey, err := Make(scriptKind, reqsigs, pubkeys)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		scriptPubkeys = append(scriptPubkeys, scriptPubkey)
	}
	return scriptPubkeys, nil
}

func p2pk(pubkey []byte) ([]byte, error) {
	var (
		spk  []byte
		klen int = len(pubkey)
	)
	if !(klen == 33 || klen == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	klen = len(pubkey)
	spk = make([]byte, 0, 1+klen+1)
	spk = append(spk, script.OP_PUSHBYTES_1-1+byte(klen))
	spk = append(spk, pubkey...)
	spk = append(spk, script.OP_CHECKSIG)
	return spk, nil
}

func p2pkh(pubkey []byte) ([]byte, error) {
	var (
		spk [1 + 1 + 1 + 20 + 1 + 1]byte
	)
	if !(len(pubkey) == 33 || len(pubkey) == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	pkh := hash160(pubkey)
	spk[0] = script.OP_DUP
	spk[1] = script.OP_HASH160
	spk[2] = script.OP_PUSHBYTES_20
	copy(spk[3:], pkh[:])
	spk[23] = script.OP_EQUALVERIFY
	spk[24] = script.OP_CHECKSIG
	return spk[:], nil
}

func p2ms(reqsigs byte, pubkeys [][]byte) ([]byte, error) {
	var (
		spk   []byte
		klen  int
		nkeys = len(pubkeys)
	)
	for i, k := range pubkeys {
		if i == 0 {
			klen = len(k)
		}
		if len(k) == klen {
			continue
		}
		return nil, fmt.Errorf("all pubkeys must have the same length")
	}
	if !(klen == 33 || klen == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	if nkeys > 16 {
		return nil, fmt.Errorf("max n multisig keys is 16")
	}
	if int(reqsigs) > nkeys {
		return nil, fmt.Errorf("required signatures > n pubkeys")
	}
	if int(reqsigs) < 1 {
		return nil, fmt.Errorf("required signatures < 1")
	}
	spk = append(spk, script.OP_1-1+reqsigs)
	for _, k := range pubkeys {
		spk = append(spk, script.OP_PUSHBYTES_1-1+byte(klen))
		spk = append(spk, k...)
	}
	spk = append(spk, script.OP_1-1+byte(nkeys))
	spk = append(spk, script.OP_CHECKMULTISIG)
	return spk[:], nil
}

func p2shMultisig(reqsigs byte, pubkeys [][]byte) ([]byte, error) {
	var (
		spk        [1 + 1 + 20 + 1]byte
		scriptData []byte
		klen       int
		nkeys      = len(pubkeys)
	)
	for i, k := range pubkeys {
		if i == 0 {
			klen = len(k)
		}
		if len(k) == klen {
			continue
		}
		return nil, fmt.Errorf("all pubkeys must have the same length")
	}
	if !(klen == 33 || klen == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	if nkeys > 16 {
		return nil, fmt.Errorf("max n multisig keys is 16")
	}
	if int(reqsigs) > nkeys {
		return nil, fmt.Errorf("required signatures > n pubkeys")
	}
	if int(reqsigs) < 1 {
		return nil, fmt.Errorf("required signatures < 1")
	}
	scriptData = append(scriptData, script.OP_1-1+reqsigs)
	for _, k := range pubkeys {
		scriptData = append(scriptData, script.OP_PUSHBYTES_1-1+byte(klen))
		scriptData = append(scriptData, k...)
	}
	scriptData = append(scriptData, script.OP_1-1+byte(nkeys))
	scriptData = append(scriptData, script.OP_CHECKMULTISIG)
	scriptDataHash := hash160(scriptData)
	spk[0] = script.OP_HASH160
	spk[1] = script.OP_PUSHBYTES_20
	copy(spk[2:], scriptDataHash[:])
	spk[22] = script.OP_EQUAL
	return spk[:], nil
}

func p2shWpkh(pubkey []byte) ([]byte, error) {
	var (
		spk        [1 + 1 + 20 + 1]byte
		scriptData [1 + 1 + 20]byte
		klen       = len(pubkey)
	)
	if !(klen == 33 || klen == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	scriptData[0] = script.OP_0
	scriptData[1] = script.OP_PUSHBYTES_20
	pkh := hash160(pubkey)
	copy(scriptData[2:], pkh[:])
	scriptDataHash := hash160(scriptData[:])
	spk[0] = script.OP_HASH160
	spk[1] = script.OP_PUSHBYTES_20
	copy(spk[2:], scriptDataHash[:])
	spk[22] = script.OP_EQUAL
	return spk[:], nil
}

func p2wpkh(pubkey []byte) ([]byte, error) {
	var spk [2 + 20]byte
	if !(len(pubkey) == 33 || len(pubkey) == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	pkh := hash160(pubkey)
	spk[0] = script.OP_0
	spk[1] = script.OP_PUSHBYTES_20
	copy(spk[2:], pkh[:])
	return spk[:], nil
}

func p2wshMulti(reqsigs byte, pubkeys [][]byte) ([]byte, error) {
	var (
		spk   []byte
		klen  int
		nkeys = len(pubkeys)
		wsh   [1 + 1 + 32]byte
	)
	for i, k := range pubkeys {
		if i == 0 {
			klen = len(k)
		}
		if len(k) == klen {
			continue
		}
		return nil, fmt.Errorf("all pubkeys must have the same length")
	}
	if !(klen == 33 || klen == 65) {
		return nil, fmt.Errorf("bad pubkey length")
	}
	if nkeys > 16 {
		return nil, fmt.Errorf("max n multisig keys is 16")
	}
	if int(reqsigs) > nkeys {
		return nil, fmt.Errorf("required signatures > n pubkeys")
	}
	if int(reqsigs) < 1 {
		return nil, fmt.Errorf("required signatures < 1")
	}
	spk = append(spk, script.OP_1-1+reqsigs)
	for _, k := range pubkeys {
		spk = append(spk, byte(klen))
		spk = append(spk, k...)
	}
	spk = append(spk, script.OP_1-1+byte(nkeys))
	spk = append(spk, script.OP_CHECKMULTISIG)
	spkh := sha256.Sum256(spk)
	wsh[0] = script.OP_0
	wsh[1] = script.OP_PUSHBYTES_32
	copy(wsh[2:], spkh[:])
	return wsh[:], nil
}

func hash160(data []byte) [20]byte {
	s256 := sha256.Sum256(data)
	return ripemd160.Sum160(s256[:])
}

func bytesSorted(slicesData [][]byte) [][]byte {
	slices.SortFunc(slicesData, func(a, b []byte) int {
		return bytes.Compare(a, b)
	})
	return slicesData
}
