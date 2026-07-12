package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// TypeDecl declares a user-contributed type in trove.toml.
type TypeDecl struct {
	Name    string `toml:"name"`
	Version int    `toml:"version"`
	Schema  string `toml:"schema"`
}

// Config is the root Trove configuration loaded from TOML.
type Config struct {
	Journal JournalConfig `toml:"journal"`
	Blobs   BlobsConfig   `toml:"blobs"`
	Modules ModulesConfig `toml:"modules"`
	HTTP    HTTPConfig    `toml:"http"`
	Types   []TypeDecl    `toml:"types"`
}

// JournalConfig holds SQLite journal settings.
type JournalConfig struct {
	Path          string `toml:"path"`
	RetentionDays int    `toml:"retention_days"`
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
	Settings map[string]map[string]any `toml:"-"`
}

type modulesConfigRaw struct {
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
	Listen       string   `toml:"listen"`
	MaxBodyBytes int64    `toml:"max_body_bytes"`
	Auth         HTTPAuth `toml:"auth"`
}

// HTTPAuth holds gateway auth validator configuration.
type HTTPAuth struct {
	Validator string `toml:"validator"`
}

// Load reads and validates configuration from path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	var modulesRaw modulesConfigRaw
	var decode struct {
		Journal JournalConfig    `toml:"journal"`
		Blobs   BlobsConfig      `toml:"blobs"`
		Modules modulesConfigRaw `toml:"modules"`
		HTTP    HTTPConfig       `toml:"http"`
		Types   []TypeDecl       `toml:"types"`
	}
	md, err := toml.Decode(string(data), &decode)
	if err != nil {
		return Config{}, fmt.Errorf("config: parse %q: %w", path, err)
	}
	cfg.Journal = decode.Journal
	cfg.Blobs = decode.Blobs
	cfg.HTTP = decode.HTTP
	cfg.Types = decode.Types
	modulesRaw = decode.Modules

	settings, err := decodeModuleSettings(md, modulesRaw.Settings)
	if err != nil {
		return Config{}, err
	}
	cfg.Modules = ModulesConfig{
		Paths:    modulesRaw.Paths,
		Remote:   modulesRaw.Remote,
		Config:   modulesRaw.Config,
		Settings: settings,
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
		cfg.Modules.Settings = map[string]map[string]any{}
	}
	if cfg.Types == nil {
		cfg.Types = []TypeDecl{}
	}
}

func decodeModuleSettings(md toml.MetaData, raw map[string]toml.Primitive) (map[string]map[string]any, error) {
	if len(raw) == 0 {
		return map[string]map[string]any{}, nil
	}
	out := make(map[string]map[string]any, len(raw))
	for name, prim := range raw {
		var table map[string]any
		if err := md.PrimitiveDecode(prim, &table); err != nil {
			return nil, fmt.Errorf("config: modules.settings[%q]: %w", name, err)
		}
		out[name] = table
	}
	return out, nil
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

	for i, td := range cfg.Types {
		cfg.Types[i].Schema, err = expandPath(td.Schema)
		if err != nil {
			return fmt.Errorf("config: types[%d].schema: %w", i, err)
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
	if cfg.HTTP.Auth.Validator != "" {
		if err := validateAuthValidatorRef(cfg.HTTP.Auth.Validator); err != nil {
			return fmt.Errorf("config: http.auth.validator: %w", err)
		}
	}
	if cfg.Journal.RetentionDays < 0 {
		return fmt.Errorf("config: journal.retention_days must be >= 0")
	}

	for i, td := range cfg.Types {
		if strings.TrimSpace(td.Name) == "" {
			return fmt.Errorf("config: types[%d].name is required", i)
		}
		if strings.TrimSpace(td.Schema) == "" {
			return fmt.Errorf("config: types[%d].schema is required", i)
		}
		if td.Version < 1 {
			return fmt.Errorf("config: types[%d].version must be >= 1", i)
		}
	}

	return nil
}

func validateAuthValidatorRef(ref string) error {
	const prefix = "module."
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return fmt.Errorf("invalid auth validator ref %q (want module.<name>.<validator>)", ref)
	}
	rest := ref[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] != '.' {
			continue
		}
		if rest[:i] == "" || rest[i+1:] == "" {
			break
		}
		return nil
	}
	return fmt.Errorf("invalid auth validator ref %q (want module.<name>.<validator>)", ref)
}
