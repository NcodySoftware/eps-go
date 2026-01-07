package base58

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	base58 "ncody.com/ncgo.git/bitcoin/base58/internal"
	"ncody.com/ncgo.git/stackerr"
)

func Encode(input []byte) string {
	return base58.Encode(input)
}

func Decode(input string) (output []byte, err error) {
	return base58.Decode(input)
}

func CheckEncode(data []byte) string {
	checksum := sha256.Sum256(data)
	checksum = sha256.Sum256(checksum[:])
	data = append(data, checksum[:4]...)
	return Encode(data)
}

func CheckDecode(s string) ([]byte, error) {
	data, err := Decode(s)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	if len(data) < 5 {
		return nil, fmt.Errorf("bad base58 data size")
	}
	checksum := sha256.Sum256(data[:len(data)-4])
	checksum = sha256.Sum256(checksum[:])
	if !bytes.Equal(checksum[:4], data[len(data)-4:]) {
		return nil, fmt.Errorf("bad checksum")
	}
	return data[:len(data)-4], nil
}
