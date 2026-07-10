package modules

import (
	"fmt"
	"os"
	"path"

	"github.com/BurntSushi/toml"
)

// Kind identifies a module's role in the Trove pipeline.
type Kind string

const (
	KindSource    Kind = "source"
	KindProcessor Kind = "processor"
	KindSink      Kind = "sink"
)

// Manifest describes a Trove module from manifest.toml.
type Manifest struct {
	Name     string            `toml:"name"`
	Version  string            `toml:"version"`
	Kind     Kind              `toml:"kind"`
	Provides []string          `toml:"provides"`
	Schemas  map[string]string `toml:"schemas"`
}

// ParseManifest parses and validates manifest TOML from data.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if _, err := toml.Decode(string(data), &m); err != nil {
		return Manifest{}, fmt.Errorf("modules: manifest: parse: %w", err)
	}
	if err := validateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ParseManifestFile reads and parses manifest.toml at path.
func ParseManifestFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("modules: manifest: read %q: %w", path, err)
	}
	return ParseManifest(data)
}

func validateManifest(m Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("modules: manifest: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("modules: manifest: version is required")
	}
	switch m.Kind {
	case KindSource, KindProcessor, KindSink:
	default:
		if m.Kind == "" {
			return fmt.Errorf("modules: manifest: kind is required")
		}
		return fmt.Errorf("modules: manifest: invalid kind %q", m.Kind)
	}

	if m.Kind == KindSource && len(m.Provides) == 0 {
		return fmt.Errorf("modules: manifest: provides is required for source modules")
	}

	for _, pattern := range m.Provides {
		if err := validateProvidesPattern(pattern); err != nil {
			return err
		}
	}
	for pattern := range m.Schemas {
		if err := validateProvidesPattern(pattern); err != nil {
			return fmt.Errorf("modules: manifest: schemas key: %w", err)
		}
	}

	return nil
}

func validateProvidesPattern(pattern string) error {
	if pattern == "*" {
		return fmt.Errorf("modules: manifest: pattern %q is not allowed", pattern)
	}
	if _, err := path.Match(pattern, "x"); err != nil {
		return fmt.Errorf("modules: manifest: invalid pattern %q: %w", pattern, err)
	}
	return nil
}
