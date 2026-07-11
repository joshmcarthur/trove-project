package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseManifestValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		want Manifest
	}{
		{
			name: "source spec example",
			file: "valid-source.toml",
			want: Manifest{
				Name:     "mqtt-source",
				Version:  "1.0",
				Kind:     KindSource,
				Provides: []string{"mqtt.message.received"},
			},
		},
		{
			name: "processor",
			file: "valid-processor.toml",
			want: Manifest{
				Name:     "enricher",
				Version:  "0.1.0",
				Kind:     KindProcessor,
				Consumes: []string{"http.ingest.received"},
				Provides: []string{"http.ingest.enriched"},
			},
		},
		{
			name: "http-only processor",
			file: "valid-http-processor.toml",
			want: Manifest{
				Name:    "mcp-query",
				Version: "1.0",
				Kind:    KindProcessor,
			},
		},
		{
			name: "sink",
			file: "valid-sink.toml",
			want: Manifest{
				Name:     "webhook-sink",
				Version:  "2.0",
				Kind:     KindSink,
				Consumes: []string{"note.*"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", "manifests", tt.file)
			got, err := ParseManifestFile(path)
			if err != nil {
				t.Fatalf("ParseManifestFile() error = %v", err)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Version != tt.want.Version {
				t.Errorf("Version = %q, want %q", got.Version, tt.want.Version)
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("Kind = %q, want %q", got.Kind, tt.want.Kind)
			}
			if len(got.Provides) != len(tt.want.Provides) {
				t.Fatalf("Provides len = %d, want %d", len(got.Provides), len(tt.want.Provides))
			}
			for i := range tt.want.Provides {
				if got.Provides[i] != tt.want.Provides[i] {
					t.Errorf("Provides[%d] = %q, want %q", i, got.Provides[i], tt.want.Provides[i])
				}
			}
			if len(got.Consumes) != len(tt.want.Consumes) {
				t.Fatalf("Consumes len = %d, want %d", len(got.Consumes), len(tt.want.Consumes))
			}
			for i := range tt.want.Consumes {
				if got.Consumes[i] != tt.want.Consumes[i] {
					t.Errorf("Consumes[%d] = %q, want %q", i, got.Consumes[i], tt.want.Consumes[i])
				}
			}
		})
	}
}

func TestParseManifestInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		wantErr string
	}{
		{
			name:    "missing name",
			file:    "invalid-missing-name.toml",
			wantErr: "name is required",
		},
		{
			name:    "missing version",
			file:    "invalid-missing-version.toml",
			wantErr: "version is required",
		},
		{
			name:    "missing kind",
			file:    "invalid-missing-kind.toml",
			wantErr: "kind is required",
		},
		{
			name:    "unknown kind",
			file:    "invalid-kind.toml",
			wantErr: `invalid kind "listener"`,
		},
		{
			name:    "malformed toml",
			file:    "invalid-malformed.toml",
			wantErr: "parse:",
		},
		{
			name:    "empty provides",
			file:    "invalid-empty-provides.toml",
			wantErr: "provides is required",
		},
		{
			name:    "bare star",
			file:    "invalid-bare-star.toml",
			wantErr: `pattern "*" is not allowed`,
		},
		{
			name:    "sink provides",
			file:    "invalid-sink-provides.toml",
			wantErr: "provides is not allowed for sink modules",
		},
		{
			name:    "sink without consumes",
			file:    "invalid-sink-no-consumes.toml",
			wantErr: "consumes is required for sink modules",
		},
		{
			name:    "source consumes",
			file:    "invalid-source-consumes.toml",
			wantErr: "consumes is not allowed for source modules",
		},
		{
			name:    "processor without routes",
			file:    "invalid-processor-no-routes.toml",
			wantErr: "must declare consumes, http.routes, and/or mcp.tools",
		},
		{
			name:    "consumes bare star",
			file:    "invalid-consumes-star.toml",
			wantErr: `pattern "*" is not allowed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", "manifests", tt.file)
			_, err := ParseManifestFile(path)
			if err == nil {
				t.Fatal("ParseManifestFile() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseManifestFile() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseManifestHTTPRoutes(t *testing.T) {
	t.Parallel()

	data := []byte(`name = "http-ingest"
version = "1.0"
kind = "source"
provides = ["http.ingest.received"]

[[http.routes]]
method = "POST"
path = "/ingest/{source}"
`)
	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if len(got.HTTPRoutes()) != 1 {
		t.Fatalf("HTTPRoutes len = %d, want 1", len(got.HTTPRoutes()))
	}
	if got.HTTPRoutes()[0].Method != "POST" || got.HTTPRoutes()[0].Path != "/ingest/{source}" {
		t.Errorf("route = %#v, want POST /ingest/{source}", got.HTTPRoutes()[0])
	}
}

func TestParseManifestRejectsListen(t *testing.T) {
	t.Parallel()

	data := []byte(`name = "old"
version = "1.0"
kind = "source"
provides = ["x"]
listen = ":8080"
`)
	_, err := ParseManifest(data)
	if err == nil || !strings.Contains(err.Error(), "listen must not be set") {
		t.Fatalf("ParseManifest() error = %v, want listen rejection", err)
	}
}

func TestCollectHTTPRoutesDuplicate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeManifest := func(name, body string) {
		sub := filepath.Join(dir, name)
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sub, "manifest.toml"), []byte(body), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sub, "module"), []byte{0}, 0o755); err != nil {
			t.Fatalf("write module binary: %v", err)
		}
	}

	writeManifest("a", `name = "a"
version = "1.0"
kind = "source"
provides = ["x"]

[[http.routes]]
method = "POST"
path = "/same"
`)
	writeManifest("b", `name = "b"
version = "1.0"
kind = "source"
provides = ["y"]

[[http.routes]]
method = "POST"
path = "/same"
`)

	mods, err := Discover([]string{dir})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	_, err = CollectHTTPRoutes(mods)
	if err == nil || !strings.Contains(err.Error(), "duplicate http route") {
		t.Fatalf("CollectHTTPRoutes() error = %v, want duplicate", err)
	}
}

func TestParseManifestInline(t *testing.T) {
	t.Parallel()

	_, err := ParseManifest([]byte(`name = "x"`))
	if err == nil || !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("ParseManifest() error = %v, want version is required", err)
	}
}

func TestParseManifestFileMissing(t *testing.T) {
	t.Parallel()

	_, err := ParseManifestFile(filepath.Join("testdata", "manifests", "does-not-exist.toml"))
	if err == nil {
		t.Fatal("ParseManifestFile() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("ParseManifestFile() error = %q, want read error", err.Error())
	}
}
