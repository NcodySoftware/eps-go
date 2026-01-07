package ripemd160

import "golang.org/x/crypto/ripemd160"

func Sum160(data []byte) [20]byte {
	hasher := ripemd160.New()
	hasher.Write(data)
	hash := hasher.Sum(nil)
	return [20]byte(hash)
}
