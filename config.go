package epsgo

import (
	"fmt"
	"os"
	"sync"

	"ncody.com/ncgo.git/env"
)

type Config struct {
	SqliteDBPath string
	LogLevel string
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

func cfgInit() {
	var ok bool
	cfg.SqliteDBPath, ok = os.LookupEnv("SQLITE_DB_PATH")
	if !ok {
		cfgErr = fmt.Errorf("undefined env: SQLITE_DB_PATH")
	}
	cfg.LogLevel = env.EnvOrDefault("LOG_LEVEL", "INFO")
}
