package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ncodysoftware/eps-go/base58"
	"github.com/ncodysoftware/eps-go/dotenv"
	"github.com/ncodysoftware/eps-go/log"
	"github.com/ncodysoftware/eps-go/os2"
	"github.com/ncodysoftware/eps-go/stackerr"
)

var (
	Version         string
	protocolVersion = "1.4"
)

func must(err error) {
	if err == nil {
		return
	}
	panic(err)
}

type config struct {
	listenAddr           string
	bitcoindAddr         string
	bitcoindUser         string
	bitcoindPassword     string
	bitcoindWalletPrefix string
	descriptors          []string
}

func initConfig() config {
	homedir, err := os.UserHomeDir()
	must(err)
	configDir := homedir + "/.config/eps-go"
	must(os.MkdirAll(configDir, 0o755))
	cfgFilePath := configDir + "/eps-go.conf"
	cfgExampleFilePath := configDir + "/eps-go.conf.example"
	exampleConfig, err := os.OpenFile(
		cfgExampleFilePath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o644,
	)
	must(err)
	defer exampleConfig.Close()
	staticExampleConfig, err := static.Open("static/eps-go.conf.example")
	must(err)
	defer staticExampleConfig.Close()
	_, err = io.Copy(exampleConfig, staticExampleConfig)
	must(err)
	dotenv.Load(cfgFilePath)
	descriptorsEnv := os2.MustEnv("DESCRIPTORS")
	descriptors := strings.Split(descriptorsEnv, ";")
	for i, v := range descriptors {
		v = strings.ReplaceAll(v, " ", "")
		if len(v) == 0 {
			descriptors[i] = descriptors[len(descriptors)-1]
			descriptors = descriptors[:len(descriptors)-1]
			i = 0
		}
		descriptors[i] = v
	}
	cfg := config{
		listenAddr:           os2.EnvOrDefault("LISTEN_ADDR", "0.0.0.0:50002"),
		descriptors:          descriptors,
		bitcoindAddr:         os2.MustEnv("BITCOIND_ADDR"),
		bitcoindUser:         os2.MustEnv("BITCOIND_USER"),
		bitcoindPassword:     os2.MustEnv("BITCOIND_PASSWORD"),
		bitcoindWalletPrefix: os2.MustEnv("BITCOIND_WALLET_PREFIX"),
	}
	return cfg
}

func main() {
	err := handleCmd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()
	logger := log.New(log.LVL_DEBUG, "eps-go")
	cfg := initConfig()
	err = run(ctx, cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getArg(idx int) string {
	if idx >= len(os.Args) {
		return ""
	}
	return os.Args[idx]
}

func handleCmd() error {
	switch getArg(1) {
	case "version", "-v", "--version":
		fmt.Printf("\neps-go %s\n\ngithub.com/ncodysoftware/eps-go\n\n", Version)
	case "toxpub":
		version := [4]byte{0x04, 0x88, 0xB2, 0x1E}
		wif := getArg(2)
		if wif == "" {
			fmt.Fprintln(os.Stderr, "usage: toxpub <public_wif> [mainnet|testnet]")
			os.Exit(1)
		}
		if getArg(3) == "testnet" {
			version = [4]byte{0x04, 0x35, 0x87, 0xCF}
		}
		xpub, err := convertWIF(wif, version)
		if err != nil {
			return stackerr.Wrap(err)
		}
		fmt.Printf("%s", xpub)
	case "toxpriv":
		version := [4]byte{0x04, 0x88, 0xAD, 0xE4}
		wif := getArg(2)
		if wif == "" {
			fmt.Fprintln(os.Stderr, "usage: toxpriv <private_wif> [mainnet|testnet]")
			os.Exit(1)
		}
		if getArg(3) == "testnet" {
			version = [4]byte{0x04, 0x35, 0x83, 0x94}
		}
		xpub, err := convertWIF(wif, version)
		if err != nil {
			return stackerr.Wrap(err)
		}
		fmt.Printf("%s", xpub)
	case "help", "-h", "--help":
		fmt.Println(`usage: <version|toxpub|toxpriv>`)
	default:
		return nil
	}
	os.Exit(0)
	return nil
}

func convertWIF(wif string, version [4]byte) (string, error) {
	decoded, err := base58.CheckDecode(wif)
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	copy(decoded[:], version[:])
	return base58.CheckEncode(decoded), nil
}

func run(ctx context.Context, cfg config, logger *log.Logger) error {
	ls, err := net.Listen("tcp", cfg.listenAddr)
	must(err)
	listener := ls.(*net.TCPListener)
	bcli := newBtcCli(
		cfg.bitcoindAddr,
		cfg.bitcoindUser,
		cfg.bitcoindPassword,
		cfg.bitcoindWalletPrefix,
	)
	wman := newWalletManager(
		bcli, logger, cfg.bitcoindWalletPrefix, cfg.descriptors,
	)
	srv := newServer(logger, bcli, wman)
	var wg sync.WaitGroup
	wg.Go(func() {
		wman.loop(ctx)
	})
	for ctx.Err() == nil {
		listener.SetDeadline(time.Now().Add(time.Millisecond * 100))
		conn, err := listener.Accept()
		if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		}
		if err != nil {
			logger.Err(stackerr.Wrap(err))
			continue
		}
		wg.Go(func() {
			srv.handleConn(ctx, conn)
		})
	}
	stopCtx, cancel := context.WithTimeout(
		context.Background(), time.Second*5,
	)
	defer cancel()
	go func() {
		defer cancel()
		wg.Wait()
	}()
	<-stopCtx.Done()
	return nil
}
