package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldCreatesConfigAndBlobs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result, err := Scaffold(dir, false)
	if err != nil {
		t.Fatalf("Scaffold() error = %v", err)
	}

	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Fatalf("config file: %v", err)
	}
	if info, err := os.Stat(result.BlobsPath); err != nil || !info.IsDir() {
		t.Fatalf("blobs dir: err=%v isDir=%v", err, info != nil && info.IsDir())
	}

	cfg, err := Load(result.ConfigPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Journal.Path != "./trove.db" {
		t.Errorf("Journal.Path = %q, want %q", cfg.Journal.Path, "./trove.db")
	}
	if cfg.Blobs.Backend != "filesystem" {
		t.Errorf("Blobs.Backend = %q, want %q", cfg.Blobs.Backend, "filesystem")
	}
	if cfg.Blobs.Path != "./blobs" {
		t.Errorf("Blobs.Path = %q, want %q", cfg.Blobs.Path, "./blobs")
	}
	if cfg.HTTP.Listen != "127.0.0.1:8080" {
		t.Errorf("HTTP.Listen = %q, want %q", cfg.HTTP.Listen, "127.0.0.1:8080")
	}
	if len(cfg.Modules.Paths) != 1 || cfg.Modules.Paths[0] != filepath.Join(mustHome(t), ".local/lib/trove/modules") {
		t.Errorf("Modules.Paths = %v, want ~/.local/lib/trove/modules only", cfg.Modules.Paths)
	}
}

func TestScaffoldIncludesLocalModulesDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "modules"), 0o755); err != nil {
		t.Fatalf("mkdir modules: %v", err)
	}

	result, err := Scaffold(dir, false)
	if err != nil {
		t.Fatalf("Scaffold() error = %v", err)
	}

	cfg, err := Load(result.ConfigPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Modules.Paths) != 2 {
		t.Fatalf("Modules.Paths len = %d, want 2", len(cfg.Modules.Paths))
	}
	if cfg.Modules.Paths[0] != "./modules" {
		t.Errorf("Modules.Paths[0] = %q, want %q", cfg.Modules.Paths[0], "./modules")
	}
}

func TestScaffoldRefusesExistingConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := Scaffold(dir, false); err != nil {
		t.Fatalf("first Scaffold() error = %v", err)
	}
	if _, err := Scaffold(dir, false); err == nil {
		t.Fatal("second Scaffold() error = nil, want already exists error")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second Scaffold() error = %v, want already exists", err)
	}
}

func TestScaffoldForceOverwritesConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := Scaffold(dir, false); err != nil {
		t.Fatalf("first Scaffold() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trove.toml"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale config: %v", err)
	}

	result, err := Scaffold(dir, true)
	if err != nil {
		t.Fatalf("Scaffold(force) error = %v", err)
	}
	data, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "[journal]") {
		t.Fatalf("config = %q, want scaffolded content", string(data))
	}
}

func mustHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	return home
}
