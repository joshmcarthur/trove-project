package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type config struct {
	Token    string `toml:"token"`
	TokenEnv string `toml:"token_env"`
}

func loadConfig() (config, error) {
	exe, err := os.Executable()
	if err != nil {
		return config{}, fmt.Errorf("http-gateway: executable path: %w", err)
	}
	return loadConfigFromDir(filepath.Dir(exe))
}

func loadConfigFromDir(dir string) (config, error) {
	var cfg config
	if err := trovemodule.LoadModuleConfig(dir, &cfg); err != nil {
		return config{}, fmt.Errorf("http-gateway: %w", err)
	}
	if cfg.Token == "" && cfg.TokenEnv != "" {
		cfg.Token = os.Getenv(cfg.TokenEnv)
	}
	return cfg, nil
}
