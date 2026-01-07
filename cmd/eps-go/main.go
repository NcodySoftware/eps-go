package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	//"runtime/pprof"
	//"runtime/trace"

	epsgo "github.com/ncodysoftware/eps-go"
	"github.com/ncodysoftware/eps-go/electrum"
	"github.com/ncodysoftware/eps-go/walletmanager"
	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
	"ncody.com/ncgo.git/log"
	"ncody.com/ncgo.git/stackerr"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}

func run() error {
	//	fd, err := os.OpenFile("/tmp/prof2", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer fd.Close()
	//	err = pprof.StartCPUProfile(fd)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer pprof.StopCPUProfile()

	//	fd, err := os.OpenFile("/tmp/trace.out", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer fd.Close()
	//	err = trace.Start(fd)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer trace.Stop()

	cfg, err := epsgo.GetConfig()
	if err != nil {
		return stackerr.Wrap(err)
	}
	wallets := getWallets()
	if len(wallets) == 0 {
		return fmt.Errorf("no wallets to track")
	}
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := log.New(log.LevelFromString(cfg.LogLevel), "eps-go")
	db, err := epsgo.OpenDB(ctx, cfg.SqliteDBPath, 0)
	if err != nil {
		return stackerr.Wrap(err)
	}
	bcli := bitcoin.NewClient(ctx, cfg.BTCNodeAddr, logger, cfg.Network)
	err = bcli.Start()
	if err != nil {
		return stackerr.Wrap(err)
	}
	w, err := walletmanager.New(
		ctx, db, logger, bcli, wallets, cfg.Network,
	)
	defer w.Close(ctx)
	err = electrum.ListenAndServe(ctx, cfg.ListenAddress, logger, w, func() {})
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func getWallets() []walletmanager.WalletConfig {
	var w []walletmanager.WalletConfig
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "WALLET") {
			continue
		}
		kv := strings.Split(v, "=")
		walletConfig, ok := wLoad(kv[1])
		if !ok {
			continue
		}
		w = append(w, walletConfig)
	}
	return w
}

func wLoad(data string) (walletmanager.WalletConfig, bool) {
	var (
		w    walletmanager.WalletConfig
		tmpi int64
		err  error
		ok   bool
	)
	wdata := strings.Split(data, " ")
	if len(data) < 2 {
		return w, false
	}
	i := 0
	tmpi, err = strconv.ParseInt(wdata[0], 10, 32)
	w.Height = int(tmpi)
	if err == nil {
		i++
	}
	w.Kind, ok = scriptpubkey.KindFromString(wdata[i])
	if !ok {
		return w, false
	}
	i++
	tmpi, err = strconv.ParseInt(wdata[i], 10, 8)
	w.Reqsigs = byte(tmpi)
	if err == nil {
		i++
	}
	for _, v := range wdata[i:] {
		mpub, err := bip32.ExtendedDecodeUnchecked(v)
		if err != nil {
			panic(stackerr.Wrap(err))
		}
		w.MasterPubs = append(w.MasterPubs, mpub)
	}
	return w, true
}
