package main

import (
	"encoding/hex"
	"testing"

	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/crypto/secp256k1"
)

func Test_ECDSA(t *testing.T) {
	privKeyBytes := [32]byte{}
	for i := range 32 {
		privKeyBytes[i] = 0xAA
	}
	expectedSig, _ := hex.DecodeString("304402206478f279ea191a4649209c12724ed0be466297fae39d67172d1e72fc59391689022076eb15d8bfa25316868444b9edf1af0ac46c96a00454c53cd0bc356847406db8")
	message := []byte("potato")
	sig := secp256k1.Sign(privKeyBytes[:], message)
	assert.MustEqual(t, expectedSig, sig)
}
