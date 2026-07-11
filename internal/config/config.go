package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the root Trove configuration loaded from TOML.
type Config struct {
	Journal JournalConfig `toml:"journal"`
	Blobs   BlobsConfig   `toml:"blobs"`
	Modules ModulesConfig `toml:"modules"`
	HTTP    HTTPConfig    `toml:"http"`
}

// JournalConfig holds SQLite journal settings.
type JournalConfig struct {
	Path string `toml:"path"`
}

// BlobsConfig holds blob storage settings.
type BlobsConfig struct {
	Backend string `toml:"backend"`
	Path    string `toml:"path"`
}

// ModulesConfig holds module discovery and remote listener settings.
type ModulesConfig struct {
	Paths    []string                  `toml:"paths"`
	Remote   RemoteConfig              `toml:"remote"`
	Config   map[string]string         `toml:"config"`
	Settings map[string]toml.Primitive `toml:"settings"`
}

// RemoteConfig holds remote module transport settings.
type RemoteConfig struct {
	Listen string `toml:"listen"`
}

// HTTPConfig holds the core HTTP gateway listener settings.
type HTTPConfig struct {
	Listen       string `toml:"listen"`
	MaxBodyBytes int64  `toml:"max_body_bytes"`
}

// Load reads and validates configuration from path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("config: parse %q: %w", path, err)
	}

	if md.IsDefined("journal", "path") && cfg.Journal.Path == "" {
		return Config{}, fmt.Errorf("config: journal.path is required")
	}
	if md.IsDefined("blobs", "backend") && cfg.Blobs.Backend == "" {
		return Config{}, fmt.Errorf("config: blobs.backend is required")
	}

	applyDefaults(&cfg)

	if err := expandPaths(&cfg); err != nil {
		return Config{}, err
	}

	if err := validate(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Journal.Path == "" {
		cfg.Journal.Path = "./trove.db"
	}
	if cfg.Blobs.Backend == "" {
		cfg.Blobs.Backend = "filesystem"
	}
	if cfg.Blobs.Path == "" {
		cfg.Blobs.Path = "./blobs"
	}
	if cfg.HTTP.Listen == "" {
		cfg.HTTP.Listen = ":8080"
	}
	if cfg.HTTP.MaxBodyBytes == 0 {
		cfg.HTTP.MaxBodyBytes = 10 << 20
	}
	if cfg.Modules.Paths == nil {
		cfg.Modules.Paths = []string{}
	}
	if cfg.Modules.Config == nil {
		cfg.Modules.Config = map[string]string{}
	}
	if cfg.Modules.Settings == nil {
		cfg.Modules.Settings = map[string]toml.Primitive{}
	}
}

func expandPaths(cfg *Config) error {
	var err error

	cfg.Journal.Path, err = expandPath(cfg.Journal.Path)
	if err != nil {
		return fmt.Errorf("config: journal.path: %w", err)
	}

	cfg.Blobs.Path, err = expandPath(cfg.Blobs.Path)
	if err != nil {
		return fmt.Errorf("config: blobs.path: %w", err)
	}

	for i, p := range cfg.Modules.Paths {
		cfg.Modules.Paths[i], err = expandPath(p)
		if err != nil {
			return fmt.Errorf("config: modules.paths[%d]: %w", i, err)
		}
	}

	for name, path := range cfg.Modules.Config {
		cfg.Modules.Config[name], err = expandPath(path)
		if err != nil {
			return fmt.Errorf("config: modules.config[%q]: %w", name, err)
		}
	}

	return nil
}

func expandPath(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		return home + path[1:], nil
	}
	return path, nil
}

func validate(cfg *Config) error {
	if cfg.Journal.Path == "" {
		return fmt.Errorf("config: journal.path is required")
	}

	switch cfg.Blobs.Backend {
	case "filesystem":
	case "":
		return fmt.Errorf("config: blobs.backend is required")
	default:
		return fmt.Errorf("config: blobs.backend %q is not supported (want filesystem)", cfg.Blobs.Backend)
	}

	if cfg.HTTP.Listen == "" {
		return fmt.Errorf("config: http.listen is required")
	}

	return nil
}
