package epsgo

import (
	"fmt"
	"os"
	"sync"

	"ncody.com/ncgo.git/env"
	"ncody.com/ncgo.git/xdg"
)

var appName = "eps-go"

type Config struct {
	SqliteDBPath string
	LogLevel string
	MigrateFresh string
	BTCNodeAddr string
	XDGDirs xdg.Dirs
}

var (
	cfg Config
	cfgErr error
	cfgOnce sync.Once
)

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
	cfg.SqliteDBPath = env.EnvOrDefault(
		"SQLITE_DB_PATH", cfg.XDGDirs.XDGDataHome+"/db.sqlite3",
	)
	cfg.MigrateFresh = env.Getenv("MIGRATE_FRESH")
	cfg.LogLevel = env.EnvOrDefault("LOG_LEVEL", "INFO")
	cfg.BTCNodeAddr, cfgErr = getEnv("BTC_NODE_ADDR")
	if cfgErr != nil {
		return
	}
}
