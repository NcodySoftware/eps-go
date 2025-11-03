// contains code from github.com/akamensky/base58
package base58

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ncodysoftware/eps-go/stackerr"
)

var (
	bigIntermediateRadix = big.NewInt(430804206899405824) // 58**10
	alphabet             = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b58table             = [256]byte{
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 0, 1, 2, 3, 4, 5, 6,
		7, 8, 255, 255, 255, 255, 255, 255,
		255, 9, 10, 11, 12, 13, 14, 15,
		16, 255, 17, 18, 19, 20, 21, 255,
		22, 23, 24, 25, 26, 27, 28, 29,
		30, 31, 32, 255, 255, 255, 255, 255,
		255, 33, 34, 35, 36, 37, 38, 39,
		40, 41, 42, 43, 255, 44, 45, 46,
		47, 48, 49, 50, 51, 52, 53, 54,
		55, 56, 57, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255,
	}
)

func Encode(input []byte) string {
	output := make([]byte, 0)
	num := new(big.Int).SetBytes(input)
	mod := new(big.Int)
	var primitiveNum int64
	for num.Sign() > 0 {
		num.DivMod(num, bigIntermediateRadix, mod)
		primitiveNum = mod.Int64()
		for i := 0; (num.Sign() > 0 || primitiveNum > 0) && i < 10; i++ {
			output = append(output, alphabet[primitiveNum%58])
			primitiveNum /= 58
		}
	}
	for i := 0; i < len(input) && input[i] == 0; i++ {
		output = append(output, alphabet[0])
	}
	for i := 0; i < len(output)/2; i++ {
		output[i], output[len(output)-1-i] = output[len(output)-1-i], output[i]
	}
	return string(output)
}

func Decode(input string) (output []byte, err error) {
	result := big.NewInt(0)
	tmpBig := new(big.Int)
	for i := 0; i < len(input); {
		var a, m int64 = 0, 58
		for f := true; i < len(input) && (f || i%10 != 0); i++ {
			tmp := b58table[input[i]]
			if tmp == 255 {
				msg := "invalid Base58 input string at character \"%c\", position %d"
				return output, fmt.Errorf(msg, input[i], i)
			}
			a = a*58 + int64(tmp)
			if !f {
				m *= 58
			}
			f = false
		}
		result.Mul(result, tmpBig.SetInt64(m))
		result.Add(result, tmpBig.SetInt64(a))
	}
	tmpBytes := result.Bytes()
	var numZeros int
	for numZeros = 0; numZeros < len(input); numZeros++ {
		if input[numZeros] != '1' {
			break
		}
	}
	length := numZeros + len(tmpBytes)
	output = make([]byte, length)
	copy(output[numZeros:], tmpBytes)
	return
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
