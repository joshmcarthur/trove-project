package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	defaultListen       = ":8080"
	defaultMaxBodyBytes = 10 << 20 // 10 MiB
)

type config struct {
	Listen       string `toml:"listen"`
	MaxBodyBytes int64  `toml:"max_body_bytes"`
}

func loadConfig() (config, error) {
	exe, err := os.Executable()
	if err != nil {
		return config{}, fmt.Errorf("http-ingest: executable path: %w", err)
	}

	return loadConfigFromDir(filepath.Dir(exe))
}

func loadConfigFromDir(dir string) (config, error) {
	manifestPath := filepath.Join(dir, "manifest.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return config{}, fmt.Errorf("http-ingest: read manifest: %w", err)
	}

	var cfg config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return config{}, fmt.Errorf("http-ingest: parse manifest: %w", err)
	}

	if cfg.Listen == "" {
		cfg.Listen = defaultListen
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}

	return cfg, nil
}
