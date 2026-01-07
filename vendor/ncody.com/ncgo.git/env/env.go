package env

import (
	"fmt"
	"os"
)

func Getenv(env string) string {
	return os.Getenv(env)
}

func EnvOrDefault(env, def string) string {
	env = os.Getenv(env)
	if env == "" {
		env = def
	}
	return env
}

func MustEnv(env string) string {
	envv := os.Getenv(env)
	if envv == "" {
		panic(fmt.Errorf("Undefined env: %s", env))
	}
	return envv
}
