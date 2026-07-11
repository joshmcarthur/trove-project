package httpingest

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshmcarthur/trove/internal/modules"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

//go:embed manifest.toml
var manifestBytes []byte

const defaultMaxBodyBytes = 10 << 20 // 10 MiB

type config struct {
	MaxBodyBytes int64    `toml:"max_body_bytes"`
	Provides     []string `toml:"provides"`
}

func loadConfig() (config, error) {
	if strings.TrimSpace(os.Getenv(trovemodule.BundledModuleEnv)) != "" {
		return loadConfigFromBytes(manifestBytes)
	}

	exe, err := os.Executable()
	if err != nil {
		return config{}, fmt.Errorf("http-ingest: executable path: %w", err)
	}

	return loadConfigFromDir(filepath.Dir(exe))
}

func loadConfigFromDir(dir string) (config, error) {
	var cfg config
	if err := trovemodule.LoadModuleConfig(dir, &cfg); err != nil {
		return config{}, fmt.Errorf("http-ingest: %w", err)
	}
	return normalizeConfig(cfg)
}

func loadConfigFromBytes(base []byte) (config, error) {
	var cfg config
	if err := trovemodule.LoadModuleConfigBytes(base, &cfg); err != nil {
		return config{}, fmt.Errorf("http-ingest: %w", err)
	}
	return normalizeConfig(cfg)
}

func normalizeConfig(cfg config) (config, error) {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	return cfg, nil
}

// Manifest returns the embedded module manifest.
func Manifest() (modules.Manifest, error) {
	return modules.ParseManifest(manifestBytes)
}
