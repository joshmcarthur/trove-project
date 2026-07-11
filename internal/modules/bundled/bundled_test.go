package bundled_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joshmcarthur/trove/internal/modules"
	_ "github.com/joshmcarthur/trove/internal/modules/bundled"
)

func TestDiscoverIncludesBundledModules(t *testing.T) {
	t.Parallel()

	mods, err := modules.Discover(nil)
	if err != nil {
		t.Fatalf("Discover(nil) error = %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("Discover(nil) len = %d, want 2 bundled modules", len(mods))
	}

	names := []string{mods[0].Manifest.Name, mods[1].Manifest.Name}
	slices.Sort(names)
	want := []string{"http-ingest", "mcp-query"}
	if !slices.Equal(names, want) {
		t.Errorf("module names = %v, want %v", names, want)
	}
	for _, mod := range mods {
		if !mod.Bundled {
			t.Errorf("module %q Bundled = false, want true", mod.Manifest.Name)
		}
	}
}

func TestDiscoverDiskOverridesBundled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	modDir := filepath.Join(root, "http-ingest")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	overrideManifest := `name = "http-ingest"
version = "9.9"
kind = "source"
provides = ["override.event"]
`
	if err := os.WriteFile(filepath.Join(modDir, "manifest.toml"), []byte(overrideManifest), 0o644); err != nil {
		t.Fatalf("write override manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "module"), []byte{0}, 0o755); err != nil {
		t.Fatalf("write module binary: %v", err)
	}

	mods, err := modules.Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	var httpIngest *modules.Module
	for i := range mods {
		if mods[i].Manifest.Name == "http-ingest" {
			httpIngest = &mods[i]
			break
		}
	}
	if httpIngest == nil {
		t.Fatal("http-ingest module not discovered")
	}
	if httpIngest.Bundled {
		t.Fatal("http-ingest should be disk override, not bundled")
	}
	if httpIngest.Manifest.Version != "9.9" {
		t.Errorf("override version = %q, want 9.9", httpIngest.Manifest.Version)
	}

	var bundledMCP bool
	for _, mod := range mods {
		if mod.Manifest.Name == "mcp-query" && mod.Bundled {
			bundledMCP = true
		}
	}
	if !bundledMCP {
		t.Fatal("mcp-query bundled fallback missing")
	}
}
