package epsgo

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"sync"

	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/dotenv"
	"ncody.com/ncgo.git/env"
	"ncody.com/ncgo.git/xdg"
)

var appName = "eps-go"

type Config struct {
	SqliteDBPath  string
	LogLevel      string
	MigrateFresh  string
	BTCNodeAddr   string
	XDGDirs       xdg.Dirs
	ListenAddress string
	ConfigFile    string
	Network       bitcoin.Network
}

var (
	cfg     Config
	cfgErr  error
	cfgOnce sync.Once
)

//go:embed eps-go.conf.example
var cfgExample []byte

func GetConfig() (*Config, error) {
	cfgOnce.Do(cfgInit)
	return &cfg, cfgErr
}

func getEnv(env string) (string, error) {
	v, ok := os.LookupEnv(env)
	if !ok {
		return "", fmt.Errorf("undefined env: %s", env)
	}
	return v, nil
}

func cfgInit() {
	cfg.XDGDirs, cfgErr = xdg.GetDirs(appName)
	if cfgErr != nil {
		return
	}
	cfg.ConfigFile = env.EnvOrDefault(
		"CONFIG_FILE", cfg.XDGDirs.XDGConfigHome+"/eps-go.conf",
	)
	dotenv.Load(cfg.ConfigFile)
	err := os.WriteFile(cfg.ConfigFile+".example", cfgExample, 0o644)
	if err != nil {
		panic(err)
	}
	cfg.SqliteDBPath = env.EnvOrDefault(
		"SQLITE_DB_PATH", cfg.XDGDirs.XDGDataHome+"/db.sqlite3",
	)
	if strings.HasPrefix(cfg.SqliteDBPath, cfg.XDGDirs.XDGDataHome) {
		os.MkdirAll(cfg.XDGDirs.XDGDataHome, 0o755)
	}
	cfg.MigrateFresh = env.Getenv("MIGRATE_FRESH")
	cfg.LogLevel = env.EnvOrDefault("LOG_LEVEL", "INFO")
	cfg.BTCNodeAddr = env.EnvOrDefault("BTC_NODE_ADDR", "127.0.0.1:8333")
	cfg.ListenAddress = env.EnvOrDefault(
		"LISTEN_ADDRESS", "127.0.0.1:50002",
	)
	cfg.Network = bitcoin.NetworkFromString(
		env.EnvOrDefault("BTC_NETWORK", "mainnet"),
	)
}
