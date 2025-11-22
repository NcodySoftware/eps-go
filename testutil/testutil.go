package testutil

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"
)

type BenchParams struct {
	Name string
	Fn   func(t *testing.T)
}

func Bench(t *testing.T, count int, params ...BenchParams) {
	for _, b := range params {
		start := time.Now()
		for range count {
			b.Fn(t)
		}
		elapsed := time.Since(start)
		fmt.Fprintf(
			os.Stderr,
			"%s elapsed: %dms\n",
			b.Name,
			elapsed/time.Millisecond,
		)
	}
}

func MustHexDecode(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func MustEqualHex(t *testing.T, exp []byte, current []byte) {
	ok := bytes.Equal(exp, current)
	if !ok {
		t.Fatalf("\nexpected: %x\n     got: %x\n", exp, current)
	}
}
