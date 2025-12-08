package testutil

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	epsgo "github.com/ncodysoftware/eps-go"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/database/sql/sqlite"
	"ncody.com/ncgo.git/log"
)

type TCtx struct {
	C context.Context
	D sql.Database
	L *log.Logger
	err error
}

var (
	tCtx TCtx
	tCtxOnce sync.Once
)

func GetTCtx(t *testing.T) {
	tCtxOnce.Do(func() {
		setup(t)
	})
	if tCtx.err != nil {
		t.Skip(tCtx.err)
	}
}

func setup(t *testing.T) {
	var (
		err error
	)
	tCtx.C = t.Context()
	config, err := epsgo.GetConfig()
	if err != nil {
		tCtx.err = err
		return
	}
	tCtx.D, err = sqlite.New(config.SqliteDBPath)
	if err != nil {
		tCtx.err = err
		return
	}
	tCtx.L = log.New(log.LevelFromString(config.LogLevel), "eps-go")
}

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
