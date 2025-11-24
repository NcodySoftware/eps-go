package bip32

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/ncodysoftware/eps-go/base58"
	"github.com/ncodysoftware/eps-go/hmacsha512"
	"github.com/ncodysoftware/eps-go/ripemd160"
	"github.com/ncodysoftware/eps-go/secp256k1"
	"github.com/ncodysoftware/eps-go/stackerr"
	"strconv"
	"strings"
)

const hardened uint32 = uint32(1) << ((4 * 8) - 1)

var bitcoinSeed = [...]byte{
	// "Bitcoin seed"
	0x42, 0x69, 0x74, 0x63, 0x6f, 0x69,
	0x6e, 0x20, 0x73, 0x65, 0x65, 0x64,
}

var (
	versionMainnetPublic  = [4]byte{0x04, 0x88, 0xb2, 0x1e}
	versionMainnetPrivate = [4]byte{0x04, 0x88, 0xad, 0xe4}
	versionTestnetPublic  = [4]byte{0x04, 0x35, 0x87, 0xcf}
	versionTestnetPrivate = [4]byte{0x04, 0x35, 0x83, 0x94}
)

func DeriveSeed(seed []byte, path string) (string, string, error) {
	var rootExt extendedKey
	dpath, err := parseDerivationPath(path)
	if err != nil {
		return "", "", stackerr.Wrap(err)
	}
	root := hmacsha512.Sum512(bitcoinSeed[:], seed, nil)
	rootExt.Version = versionMainnetPrivate
	copy(rootExt.Key[1:], root[:32])
	copy(rootExt.Chaincode[:], root[32:])
	xprvExt, err := deriveXprv(&rootExt, dpath)
	if err != nil {
		return "", "", stackerr.Wrap(err)
	}
	xpubExt, err := xpubFromXprv(&xprvExt)
	if err != nil {
		return "", "", stackerr.Wrap(err)
	}
	encodedXpub := extendedEncode(xpubExt)
	encodedXprv := extendedEncode(xprvExt)
	return encodedXpub, encodedXprv, nil
}

func DeriveXprv(xpriv string, path string) (string, error) {
	dpath, err := parseDerivationPath(path)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	decoded, err := extendedDecode(xpriv)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	exKey, err := deriveXprv(&decoded, dpath)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	encodedXpriv := extendedEncode(exKey)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return encodedXpriv, nil
}

func deriveXprv(xpriv *extendedKey, path []uint32) (extendedKey, error) {
	var (
		privkeyChaincode  [64]byte
		parentXpriv       [64]byte
		parentFingerprint [4]byte
		exKey             extendedKey
		childNum          [4]byte
		err               error
	)
	if len(path) == 0 {
		return *xpriv, nil
	}
	copy(privkeyChaincode[:], xpriv.Key[1:])
	copy(privkeyChaincode[32:], xpriv.Chaincode[:])
	for _, derivation := range path {
		parentXpriv = privkeyChaincode
		if derivation&hardened != 0 {
			privkeyChaincode, err = deriveHardenedPrivFromPriv(
				privkeyChaincode, derivation,
			)
			if err != nil {
				return exKey, stackerr.Wrap(err)
			}
		} else {
			privkeyChaincode, err = deriveUnhardenedPrivFromPriv(
				privkeyChaincode, derivation,
			)
			if err != nil {
				return exKey, stackerr.Wrap(err)
			}
		}
	}
	if len(path) != 0 {
		parentXpub, err := pubFromPriv(parentXpriv)
		if err != nil {
			return exKey, stackerr.Wrap(err)
		}
		parentHash := hash160(parentXpub[:33])
		copy(parentFingerprint[:], parentHash[:4])
		childNum = serializeUint32(path[len(path)-1])
	}
	exKey.Version = versionMainnetPrivate
	exKey.Depth = byte(len(path)) + xpriv.Depth
	exKey.Fingerprint = parentFingerprint
	exKey.ChildNum = childNum
	copy(exKey.Chaincode[:], privkeyChaincode[32:])
	copy(exKey.Key[1:], privkeyChaincode[0:32])
	return exKey, nil
}

func DeriveXpub(xpub string, path string) (string, error) {
	dpath, err := parseDerivationPath(path)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	decoded, err := extendedDecode(xpub)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	exKey, err := deriveXpub(&decoded, dpath)
	encodedXpub := extendedEncode(exKey)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	return encodedXpub, nil
}

func deriveXpub(xpub *extendedKey, path []uint32) (extendedKey, error) {
	var (
		pubkeyChaincode   [65]byte
		childXpub         [65]byte
		parentXpub        [65]byte
		parentFingerprint [4]byte
		exKey             extendedKey
		childNum          [4]byte
		err               error
	)
	copy(pubkeyChaincode[:], xpub.Key[:])
	copy(pubkeyChaincode[33:], xpub.Chaincode[:])
	for _, derivation := range path {
		if derivation&hardened != 0 {
			return exKey, fmt.Errorf(
				"hardened derivation from pubkey",
			)
		}
		parentXpub = childXpub
		childXpub, err = deriveUnhardenedPubFromPub(
			pubkeyChaincode, derivation,
		)
		if err != nil {
			return exKey, stackerr.Wrap(err)
		}
		pubkeyChaincode = childXpub
	}
	if len(path) != 0 {
		parentHash := hash160(parentXpub[:33])
		copy(parentFingerprint[:], parentHash[:4])
		childNum = serializeUint32(path[len(path)-1])
	}
	exKey.Version = versionMainnetPublic
	exKey.Depth = byte(len(path)) + xpub.Depth
	exKey.Fingerprint = parentFingerprint
	exKey.ChildNum = childNum
	copy(exKey.Chaincode[:], childXpub[33:])
	copy(exKey.Key[:], childXpub[0:33])
	return exKey, nil
}

func XpubFromXprv(xprv string) (string, error) {
	extXprv, err := extendedDecode(xprv)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	extPub, err := xpubFromXprv(&extXprv)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	xpub := extendedEncode(extPub)
	return xpub, nil
}

func xpubFromXprv(xprv *extendedKey) (extendedKey, error) {
	var (
		keyChaincode [64]byte
		xpub         extendedKey = *xprv
	)
	copy(keyChaincode[:], xprv.Key[1:])
	copy(keyChaincode[32:], xprv.Chaincode[:])
	pubkeyChaincode, err := pubFromPriv(keyChaincode)
	if err != nil {
		return extendedKey{}, stackerr.Wrap(err)
	}
	switch xpub.Version {
	case versionMainnetPrivate:
		xpub.Version = versionMainnetPublic
	case versionTestnetPrivate:
		xpub.Version = versionTestnetPublic
	default:
		return extendedKey{}, errBadKeyVersion
	}
	copy(xpub.Key[:], pubkeyChaincode[:33])
	copy(xpub.Chaincode[:], pubkeyChaincode[33:])
	return xpub, nil
}

func derivePrivFromPriv(
	keyChaincode [64]byte, dpath []uint32,
) ([64]byte, error) {
	var (
		d [64]byte
	)
	for _, node := range dpath {
	start:
		if node&hardened == 0 {
			kc, err := deriveUnhardenedPrivFromPriv(
				keyChaincode, node,
			)
			if err != nil && errors.Is(err, errOverflow) {
				node++
				goto start
			}
			if err != nil {
				return d, stackerr.Wrap(err)
			}
			keyChaincode = kc
			continue
		}
		kc, err := deriveHardenedPrivFromPriv(keyChaincode, node)
		if err != nil && errors.Is(err, errOverflow) {
			node++
			goto start
		}
		if err != nil {
			return d, stackerr.Wrap(err)
		}
		keyChaincode = kc
	}
	return keyChaincode, nil
}

func deriveUnhardenedPrivFromPriv(
	keyChaincode [64]byte, index uint32,
) ([64]byte, error) {
	/*
	   I = HMAC-SHA512(Key = cpar, Data = serP(point(kpar)) || ser32(i)).
	*/
	var data [33 + 4]byte
	pointCompressed, err := compressedFromBasePointMul(
		[32]byte(keyChaincode[:32]),
	)
	if err != nil {
		return [64]byte{}, stackerr.Wrap(err)
	}
	serializedIndex := serializeUint32(index)
	copy(data[:], pointCompressed[:])
	copy(data[33:], serializedIndex[:])
	I := hmacsha512.Sum512(keyChaincode[32:], data[:], nil)
	IL, IR := I[:32], I[32:]

	return finalizeDerivePriv(keyChaincode, [32]byte(IL), [32]byte(IR))
}

func deriveHardenedPrivFromPriv(
	keyChaincode [64]byte, index uint32,
) ([64]byte, error) {
	/*
	   I = HMAC-SHA512(Key = cpar, Data = 0x00 || ser256(kpar) || ser32(i)).
	*/
	var data [1 + 32 + 4]byte
	serializedIndex := serializeUint32(index)
	copy(data[1:], keyChaincode[:32])
	copy(data[33:], serializedIndex[:])
	I := hmacsha512.Sum512(keyChaincode[32:], data[:], nil)
	IL, IR := I[:32], I[32:]
	return finalizeDerivePriv(keyChaincode, [32]byte(IL), [32]byte(IR))
}

var errOverflow = errors.New("overflow")

func finalizeDerivePriv(keyChaincode [64]byte, IL, IR [32]byte) ([64]byte, error) {
	/*
		child key = parse256(IL) + kpar (mod n).
		chain code = IR.
	*/
	var result [64]byte
	key := secp256k1.ModNScalarFromSlice(IL[:])
	keyAdd := secp256k1.ModNScalarFromSlice(keyChaincode[:32])
	key = secp256k1.ModNScalarAdd(&key, &keyAdd)
	keyBytes := secp256k1.ModNScalarBytes(&key)
	copy(result[:32], keyBytes[:])
	copy(result[32:], IR[:])
	return result, nil
}

func deriveUnhardenedPubFromPub(
	keyChaincode [65]byte, index uint32,
) ([65]byte, error) {
	/*
		I = HMAC-SHA512(Key = cpar, Data = serP(Kpar) || ser32(i))
		key = point(parse256(IL)) + Kpar
		chaincode = IR
	*/
	var (
		result [65]byte
		data   [33 + 4]byte
	)
	copy(data[:], keyChaincode[:33])
	serializedIndex := serializeUint32(index)
	copy(data[33:], serializedIndex[:])
	I := hmacsha512.Sum512(keyChaincode[33:], data[:], nil)
	IL, IR := I[:32], I[32:]
	p, err := compressedFromBasePointMul([32]byte(IL))
	if err != nil {
		return result, stackerr.Wrap(err)
	}
	key, err := compressedFromPointAdd([33]byte(keyChaincode[:33]), p)
	if err != nil {
		return result, stackerr.Wrap(err)
	}
	copy(result[:], key[:])
	copy(result[33:], IR[:])
	return result, nil
}

func compressedFromPointAdd(
	pointA, pointB [33]byte,
) ([33]byte, error) {
	var (
		d               [33]byte
		pa, pb, presult secp256k1.Point
	)
	pa, err := secp256k1.PointDeserialize(pointA[:])
	if err != nil {
		return d, stackerr.Wrap(err)
	}
	pb, err = secp256k1.PointDeserialize(pointB[:])
	if err != nil {
		return d, stackerr.Wrap(err)
	}
	presult = secp256k1.PointAdd(&pa, &pb)
	secp256k1.PointToAffine(&presult)
	return secp256k1.PointSerializeCompressed(&presult), nil
}

func compressedFromBasePointMul(scalar [32]byte) ([33]byte, error) {
	var (
		s     secp256k1.ModNScalar
		point secp256k1.Point
	)
	s = secp256k1.ModNScalarFromSlice(scalar[:])
	point = secp256k1.ModNScalarBaseMult(&s)
	secp256k1.PointToAffine(&point)
	if secp256k1.PointAtInfinity(&point) {
		return [33]byte{}, fmt.Errorf("point at infinity")
	}
	return secp256k1.PointSerializeCompressed(&point), nil
}

func pubFromPriv(keyChaincode [64]byte) ([65]byte, error) {
	var result [65]byte
	pub, err := compressedFromBasePointMul([32]byte(keyChaincode[:32]))
	if err != nil {
		return result, stackerr.Wrap(err)
	}
	copy(result[:], pub[:])
	copy(result[33:], keyChaincode[32:])
	return result, nil
}

func parseDerivationPath(path string) ([]uint32, error) {
	var parsed []uint32
	if path == "" {
		return nil, fmt.Errorf("empty derivation path")
	}
	path = strings.ToLower(path)
	if path[0] != 'm' {
		return nil, fmt.Errorf("bad derivation path")
	}
	if len(path) < 3 {
		return parsed, nil
	}
	path = path[2:]
	tree := strings.Split(path, "/")
	for _, node := range tree {
		var isHardened uint32
		nodeNumberString := strings.ReplaceAll(node, "h", "")
		if nodeNumberString != node {
			isHardened = hardened
		}
		nodeNumberI64, err := strconv.ParseInt(nodeNumberString, 10, 32)
		if err != nil {
			return nil, fmt.Errorf(
				"bad derivation path node: %s", node,
			)
		}
		parsed = append(parsed, uint32(uint64(nodeNumberI64))+isHardened)
	}
	return parsed, nil
}

var (
	errBadKeyVersion               = errors.New("errBadKeyVersion")
	errBadPubKeyPrefix             = errors.New("errBadPubKeyPrefix")
	errBadPrivKeyPrefix            = errors.New("errBadPrivKeyPrefix")
	errZeroDepthNonZeroFingerprint = errors.New("errZeroDepthNonZeroFingerprint")
	errZeroDepthNonZeroIndex       = errors.New("errZeroDepthNonZeroIndex")
	errUnknownVersion              = errors.New("errUnknownVersion")
	errPrivKeyNotInCurveOrder      = errors.New("errPrivKeyNotInCurveOrder")
	errInvalidPubKey               = errors.New("errInvalidPubKey")
	errInvalidChecksum             = errors.New("errInvalidChecksum")
)

type extendedKey struct {
	Version     [4]byte
	Depth       byte
	Fingerprint [4]byte
	ChildNum    [4]byte
	Chaincode   [32]byte
	Key         [33]byte
}

func extendedDecode(b58x string) (extendedKey, error) {
	var r extendedKey
	data, err := base58.CheckDecode(b58x)
	if err != nil {
		return r, errInvalidChecksum
	}
	if len(data) != 4+1+4+4+32+33 {
		return r, fmt.Errorf("bad extended key data")
	}
	copy(r.Version[:], data[:4])
	r.Depth = data[4]
	copy(r.Fingerprint[:], data[4+1:4+1+4])
	copy(r.ChildNum[:], data[4+1+4:4+1+4+4])
	copy(r.Chaincode[:], data[4+1+4+4:4+1+4+4+32])
	copy(r.Key[:], data[4+1+4+4+32:4+1+4+4+32+33])
	switch r.Version {
	case versionMainnetPublic, versionTestnetPublic:
		if r.Key[0] == 0x0 {
			return r, errBadKeyVersion
		}
		if !(r.Key[0] == 0x02 || r.Key[0] == 0x03) {
			return r, errBadPubKeyPrefix
		}
	case versionMainnetPrivate, versionTestnetPrivate:
		if r.Key[0] == 0x02 || r.Key[0] == 0x03 {
			return r, errBadKeyVersion
		}
		if r.Key[0] != 0x0 {
			return r, errBadPrivKeyPrefix
		}
	default:
		return r, errUnknownVersion
	}
	if r.Depth == 0 &&
		!bytes.Equal(r.Fingerprint[:], []byte{0x0, 0x0, 0x0, 0x0}) {
		return r, errZeroDepthNonZeroFingerprint
	}
	if r.Depth == 0 &&
		!bytes.Equal(r.ChildNum[:], []byte{0x0, 0x0, 0x0, 0x0}) {
		return r, errZeroDepthNonZeroIndex
	}
	if r.Key[0] == 0x0 {
		var zr [33]byte
		if bytes.Equal(r.Key[:], zr[:]) {
			return r, errPrivKeyNotInCurveOrder
		}
		_, err := compressedFromBasePointMul([32]byte(r.Key[1:]))
		if err != nil {
			return r, errPrivKeyNotInCurveOrder
		}
	}
	if r.Key[0] == 0x02 || r.Key[0] == 0x03 {
		var zr [33]byte
		if bytes.Equal(r.Key[:], zr[:]) {
			return r, errInvalidPubKey
		}
		_, err := secp256k1.PointDeserialize(r.Key[:])
		if err != nil {
			return r, errInvalidPubKey
		}
	}
	return r, nil
}

func extendedEncode(e extendedKey) string {
	var data [4 + 1 + 4 + 4 + 32 + 33]byte
	copy(data[:], e.Version[:])
	data[4] = e.Depth
	copy(data[4+1:], e.Fingerprint[:])
	copy(data[4+1+4:], e.ChildNum[:])
	copy(data[4+1+4+4:], e.Chaincode[:])
	copy(data[4+1+4+4+32:], e.Key[:])
	return base58.CheckEncode(data[:])
}

func hash160(data []byte) [20]byte {
	h256 := sha256.Sum256(data)
	return ripemd160.Sum160(h256[:])
}

func serializeUint32(v uint32) [4]byte {
	var r [4]byte
	r[0] = byte(v >> (8 * 3))
	r[1] = byte(v >> (8 * 2))
	r[2] = byte(v >> (8 * 1))
	r[3] = byte(v >> (8 * 0))
	return r
}

func deserializeUint32(v [4]byte) uint32 {
	var r uint32
	r |= uint32(v[0])
	r <<= 8
	r |= uint32(v[1])
	r <<= 8
	r |= uint32(v[2])
	r <<= 8
	r |= uint32(v[3])
	return r
}
