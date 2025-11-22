package hmacsha512

import (
	"crypto/sha512"
)

func Sum512(key []byte, msg []byte, buf *[]byte) [64]byte {
	const (
		opad    = 0x5c
		ipad    = 0x36
		blkSize = 128
	)
	var (
		ikey  [blkSize]byte
		opadA [blkSize]byte
		ipadA [blkSize]byte
		fh    [blkSize + (blkSize / 2)]byte
		buf2  []byte
	)
	if buf == nil {
		buf = &buf2
	} else {
		*buf = (*buf)[:0]
	}
	if len(key) > blkSize {
		kh := sha512.Sum512(key)
		copy(ikey[:], kh[:])
	} else {
		copy(ikey[:], key[:])
	}
	for i := range blkSize {
		opadA[i] = ikey[i] ^ opad
		ipadA[i] = ikey[i] ^ ipad
	}
	*buf = append(*buf, ipadA[:]...)
	*buf = append(*buf, msg...)
	msgH := sha512.Sum512(*buf)
	copy(fh[:], opadA[:])
	copy(fh[blkSize:], msgH[:])
	return sha512.Sum512(fh[:])
}
