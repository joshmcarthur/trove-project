package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidExample(t *testing.T) {
	t.Parallel()

	cfg, err := Load(filepath.Join("testdata", "valid.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	want := Config{
		Journal: JournalConfig{Path: "/data/trove.db"},
		Blobs: BlobsConfig{
			Backend: "filesystem",
			Path:    "/data/blobs",
		},
		Modules: ModulesConfig{
			Paths: []string{
				"/usr/local/lib/trove/modules",
				filepath.Join(home, ".local/lib/trove/modules"),
			},
			Remote: RemoteConfig{Listen: "tailscale:trove"},
		},
		HTTP: HTTPConfig{Listen: ":8080", MaxBodyBytes: 10 << 20},
		MCP:  MCPConfig{Listen: ":8081"},
	}

	if cfg.Journal != want.Journal {
		t.Errorf("Journal = %#v, want %#v", cfg.Journal, want.Journal)
	}
	if cfg.Blobs != want.Blobs {
		t.Errorf("Blobs = %#v, want %#v", cfg.Blobs, want.Blobs)
	}
	if cfg.HTTP != want.HTTP {
		t.Errorf("HTTP = %#v, want %#v", cfg.HTTP, want.HTTP)
	}
	if cfg.MCP != want.MCP {
		t.Errorf("MCP = %#v, want %#v", cfg.MCP, want.MCP)
	}
	if cfg.Modules.Remote != want.Modules.Remote {
		t.Errorf("Modules.Remote = %#v, want %#v", cfg.Modules.Remote, want.Modules.Remote)
	}
	if len(cfg.Modules.Paths) != len(want.Modules.Paths) {
		t.Fatalf("Modules.Paths len = %d, want %d", len(cfg.Modules.Paths), len(want.Modules.Paths))
	}
	for i := range want.Modules.Paths {
		if cfg.Modules.Paths[i] != want.Modules.Paths[i] {
			t.Errorf("Modules.Paths[%d] = %q, want %q", i, cfg.Modules.Paths[i], want.Modules.Paths[i])
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(filepath.Join("testdata", "defaults.toml"))
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
	if cfg.HTTP.Listen != ":9090" {
		t.Errorf("HTTP.Listen = %q, want %q", cfg.HTTP.Listen, ":9090")
	}
	if cfg.MCP.Listen != ":8081" {
		t.Errorf("MCP.Listen = %q, want %q", cfg.MCP.Listen, ":8081")
	}
	if len(cfg.Modules.Paths) != 0 {
		t.Errorf("Modules.Paths = %v, want empty slice", cfg.Modules.Paths)
	}
}

func TestLoadTildeExpansion(t *testing.T) {
	t.Parallel()

	cfg, err := Load(filepath.Join("testdata", "tilde-expand.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	wantJournal := filepath.Join(home, "trove-data/journal.db")
	if cfg.Journal.Path != wantJournal {
		t.Errorf("Journal.Path = %q, want %q", cfg.Journal.Path, wantJournal)
	}

	wantModules := filepath.Join(home, "modules")
	if len(cfg.Modules.Paths) != 1 || cfg.Modules.Paths[0] != wantModules {
		t.Errorf("Modules.Paths = %v, want [%q]", cfg.Modules.Paths, wantModules)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "missing file",
			path:    filepath.Join("testdata", "does-not-exist.toml"),
			wantErr: "config: read",
		},
		{
			name:    "invalid syntax",
			path:    filepath.Join("testdata", "invalid-syntax.toml"),
			wantErr: "config: parse",
		},
		{
			name:    "empty journal path",
			path:    filepath.Join("testdata", "invalid-empty-journal-path.toml"),
			wantErr: "config: journal.path is required",
		},
		{
			name:    "unknown blob backend",
			path:    filepath.Join("testdata", "invalid-blob-backend.toml"),
			wantErr: `config: blobs.backend "s3" is not supported`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(tt.path)
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}
