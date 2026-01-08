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
	"ncody.com/ncgo.git/database/sql/migrator"
	"ncody.com/ncgo.git/log"
)

type TCtx struct {
	C   context.Context
	D   sql.Database
	L   *log.Logger
	Cfg *epsgo.Config
	err error
}

var (
	tCtx   TCtx
	tCtxMu sync.Mutex
)

func GetTCtx(t *testing.T) (*TCtx, func()) {
	tCtxMu.Lock()
	setup(t)
	if tCtx.err != nil {
		t.Skip(tCtx.err)
	}
	f := func() {
		defer tCtxMu.Unlock()
		tCtx.D.Close(tCtx.C)
	}
	return &tCtx, f
}

func setup(t *testing.T) {
	var (
		err error
	)
	tCtx.C = t.Context()
	tCtx.Cfg, err = epsgo.GetConfig()
	if err != nil {
		tCtx.err = err
		return
	}
	tCtx.L = log.New(log.LevelFromString(tCtx.Cfg.LogLevel), "eps-go")
	tCtx.D, tCtx.err = epsgo.OpenDB(
		tCtx.C,
		tCtx.L,
		tCtx.Cfg.SqliteDBPath,
		migrator.FlagMigrateFresh,
	)
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
