package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type config struct {
	Broker   string   `toml:"broker"`
	ClientID string   `toml:"client_id"`
	Topics   []string `toml:"topics"`
	QoS      byte     `toml:"qos"`
	Username string   `toml:"username"`
	Password string   `toml:"password"`
}

func loadConfig() (config, error) {
	exe, err := os.Executable()
	if err != nil {
		return config{}, fmt.Errorf("mqtt-source: executable path: %w", err)
	}

	return loadConfigFromDir(filepath.Dir(exe))
}

func loadConfigFromDir(dir string) (config, error) {
	manifestPath := filepath.Join(dir, "manifest.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return config{}, fmt.Errorf("mqtt-source: read manifest: %w", err)
	}

	var cfg config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return config{}, fmt.Errorf("mqtt-source: parse manifest: %w", err)
	}

	if cfg.ClientID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return config{}, fmt.Errorf("mqtt-source: hostname: %w", err)
		}
		cfg.ClientID = "trove-mqtt-source-" + hostname
	}

	if cfg.Broker == "" {
		return config{}, fmt.Errorf("mqtt-source: broker is required")
	}
	if len(cfg.Topics) == 0 {
		return config{}, fmt.Errorf("mqtt-source: at least one topic is required")
	}
	if cfg.QoS > 2 {
		return config{}, fmt.Errorf("mqtt-source: qos must be 0, 1, or 2")
	}

	return cfg, nil
}
