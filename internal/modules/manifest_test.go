package modules

import (
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
				Provides: []string{"http.ingest.received"},
			},
		},
		{
			name: "sink",
			file: "valid-sink.toml",
			want: Manifest{
				Name:     "webhook-sink",
				Version:  "2.0",
				Kind:     KindSink,
				Provides: []string{},
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
