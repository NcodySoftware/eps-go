package main

import (
	"os"
	"testing"
)

type testingConfig struct {
	startFreshBitcoind     string
	saveBitcoindState      string
	loadBitcoindSavedState string
}

func initTestingConfig(t *testing.T) testingConfig {
	envOrSkip := func(env string) string {
		val := os.Getenv(env)
		if val != "" {
			return val
		}
		t.Skipf("missing env: %s", env)
		return ""
	}
	return testingConfig{
		startFreshBitcoind:     envOrSkip("CMD_START_FRESH_BITCOIND"),
		saveBitcoindState:      envOrSkip("CMD_SAVE_BITCOIND_STATE"),
		loadBitcoindSavedState: envOrSkip("CMD_LOAD_BITCOIND_SAVED_STATE"),
	}
}
