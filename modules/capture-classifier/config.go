package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

const defaultMaxBodyBytes = 10 << 20 // 10 MiB

type config struct {
	MaxBodyBytes int64    `toml:"max_body_bytes"`
	Provides     []string `toml:"provides"`
}

func loadConfig() (config, error) {
	exe, err := os.Executable()
	if err != nil {
		return config{}, fmt.Errorf("capture-classifier: executable path: %w", err)
	}
	return loadConfigFromDir(filepath.Dir(exe))
}

func loadConfigFromDir(dir string) (config, error) {
	var cfg config
	if err := trovemodule.LoadModuleConfig(dir, &cfg); err != nil {
		return config{}, fmt.Errorf("capture-classifier: %w", err)
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	return cfg, nil
}
