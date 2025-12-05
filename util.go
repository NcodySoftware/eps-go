package epsgo

import (
	"os"

	"ncody.com/ncgo.git/env"
)

var appName = "eps-go"

type xdgDirs struct {
	XDGDataHome string
	XDGConfigHome string
	XDGStateHome string
	XDGCacheHome string
}

func getXdgDirs(appName string) (xdgDirs, error) {
	var r xdgDirs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return r, err
	}
	r.XDGDataHome = env.EnvOrDefault(
		"XDG_DATA_HOME", homeDir+"/.local/share",
	)+"/"+appName
	r.XDGConfigHome = env.EnvOrDefault(
		"XDG_CONFIG_HOME", homeDir+"/.config",
	)+"/"+appName
	r.XDGStateHome = env.EnvOrDefault(
		"XDG_STATE_HOME", homeDir+"/.local/state",
	)+"/"+appName
	r.XDGCacheHome = env.EnvOrDefault(
		"XDG_CACHE_HOME", homeDir+"/.cache",
	)+"/"+appName
	os.MkdirAll(r.XDGDataHome, 0o755)
	os.MkdirAll(r.XDGConfigHome, 0o755)
	os.MkdirAll(r.XDGStateHome, 0o755)
	os.MkdirAll(r.XDGCacheHome, 0o755)
	return r, nil
}
