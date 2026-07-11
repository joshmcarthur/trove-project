package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
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
	manifestPath := filepath.Join(dir, "manifest.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return config{}, fmt.Errorf("capture-classifier: read manifest: %w", err)
	}

	var cfg config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return config{}, fmt.Errorf("capture-classifier: parse manifest: %w", err)
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	return cfg, nil
}
