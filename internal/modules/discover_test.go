package modules

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDiscoverValidSource(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t, moduleSetup{name: "valid-source", binary: true})

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("Discover() len = %d, want 1", len(mods))
	}

	got := mods[0]
	want := Module{
		Dir:    filepath.Join(root, "valid-source"),
		Binary: filepath.Join(root, "valid-source", "module"),
		Manifest: Manifest{
			Name:     "mqtt-source",
			Version:  "1.0",
			Kind:     KindSource,
			Provides: []string{"mqtt.message.received"},
		},
	}
	assertModule(t, got, want)
}

func TestDiscoverMultipleInOnePath(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t,
		moduleSetup{name: "valid-source", binary: true},
		moduleSetup{name: "another-module", binary: true},
	)

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("Discover() len = %d, want 2", len(mods))
	}

	names := []string{mods[0].Manifest.Name, mods[1].Manifest.Name}
	slices.Sort(names)
	want := []string{"enricher", "mqtt-source"}
	if !slices.Equal(names, want) {
		t.Errorf("module names = %v, want %v", names, want)
	}
}

func TestDiscoverMultiplePaths(t *testing.T) {
	t.Parallel()

	root1 := setupDiscoveryRoot(t, moduleSetup{name: "valid-source", binary: true})
	root2 := setupDiscoveryRoot(t, moduleSetup{name: "another-module", binary: true})

	mods, err := Discover([]string{root1, root2})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("Discover() len = %d, want 2", len(mods))
	}
	if mods[0].Manifest.Name != "mqtt-source" {
		t.Errorf("mods[0].Name = %q, want mqtt-source", mods[0].Manifest.Name)
	}
	if mods[1].Manifest.Name != "enricher" {
		t.Errorf("mods[1].Name = %q, want enricher", mods[1].Manifest.Name)
	}
}

func TestDiscoverDuplicateNameFirstWins(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t,
		moduleSetup{name: "duplicate-second", binary: true},
		moduleSetup{name: "duplicate-first", binary: true},
	)

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("Discover() len = %d, want 1", len(mods))
	}
	if mods[0].Manifest.Version != "1.0" {
		t.Errorf("Manifest.Version = %q, want 1.0 (first wins)", mods[0].Manifest.Version)
	}
	if mods[0].Manifest.Provides[0] != "first.event" {
		t.Errorf("Manifest.Provides = %v, want first.event", mods[0].Manifest.Provides)
	}
}

func TestDiscoverDuplicateNameAcrossPaths(t *testing.T) {
	t.Parallel()

	root1 := setupDiscoveryRoot(t, moduleSetup{name: "duplicate-first", binary: true})
	root2 := setupDiscoveryRoot(t, moduleSetup{name: "duplicate-second", binary: true})

	mods, err := Discover([]string{root1, root2})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("Discover() len = %d, want 1", len(mods))
	}
	if mods[0].Manifest.Version != "1.0" {
		t.Errorf("Manifest.Version = %q, want 1.0 (earlier path wins)", mods[0].Manifest.Version)
	}
}

func TestDiscoverSkipsWithoutManifest(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t,
		moduleSetup{name: "no-manifest", binary: false},
		moduleSetup{name: "valid-source", binary: true},
	)

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("Discover() len = %d, want 1", len(mods))
	}
	if mods[0].Manifest.Name != "mqtt-source" {
		t.Errorf("Manifest.Name = %q, want mqtt-source", mods[0].Manifest.Name)
	}
}

func TestDiscoverSkipsInvalidManifest(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t,
		moduleSetup{name: "invalid-manifest", binary: true},
		moduleSetup{name: "valid-source", binary: true},
	)

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("Discover() len = %d, want 1", len(mods))
	}
	if mods[0].Manifest.Name != "mqtt-source" {
		t.Errorf("Manifest.Name = %q, want mqtt-source", mods[0].Manifest.Name)
	}
}

func TestDiscoverSkipsWithoutBinary(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t,
		moduleSetup{name: "no-binary", binary: false},
		moduleSetup{name: "valid-source", binary: true},
	)

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("Discover() len = %d, want 1", len(mods))
	}
	if mods[0].Manifest.Name != "mqtt-source" {
		t.Errorf("Manifest.Name = %q, want mqtt-source", mods[0].Manifest.Name)
	}
}

func TestDiscoverSkipsNonExecutableBinary(t *testing.T) {
	t.Parallel()

	root := setupDiscoveryRoot(t, moduleSetup{name: "valid-source", binary: false})
	writeModuleBinary(t, filepath.Join(root, "valid-source", "module"), 0644)

	mods, err := Discover([]string{root})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(mods) != 0 {
		t.Fatalf("Discover() len = %d, want 0", len(mods))
	}
}

func TestDiscoverEmptyPaths(t *testing.T) {
	t.Parallel()

	mods, err := discoverPaths(nil)
	if err != nil {
		t.Fatalf("discoverPaths(nil) error = %v", err)
	}
	if mods != nil {
		t.Fatalf("discoverPaths(nil) = %v, want nil", mods)
	}

	mods, err = discoverPaths([]string{})
	if err != nil {
		t.Fatalf("discoverPaths([]) error = %v", err)
	}
	if mods != nil {
		t.Fatalf("discoverPaths([]) = %v, want nil", mods)
	}
}

func TestDiscoverMissingRoot(t *testing.T) {
	t.Parallel()

	mods, err := Discover([]string{filepath.Join(t.TempDir(), "does-not-exist")})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if mods != nil {
		t.Fatalf("Discover() = %v, want nil", mods)
	}
}

type moduleSetup struct {
	name   string
	binary bool
}

func setupDiscoveryRoot(t *testing.T, setups ...moduleSetup) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "modules")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir modules root: %v", err)
	}

	srcRoot := filepath.Join("testdata", "discovery")
	for _, setup := range setups {
		dst := filepath.Join(root, setup.name)
		if err := copyDir(filepath.Join(srcRoot, setup.name), dst); err != nil {
			t.Fatalf("copy module dir %q: %v", setup.name, err)
		}
		if setup.binary {
			writeModuleBinary(t, filepath.Join(dst, "module"), 0o755)
		}
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs modules root: %v", err)
	}
	return abs
}

func writeModuleBinary(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatalf("write module binary %q: %v", path, err)
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func assertModule(t *testing.T, got, want Module) {
	t.Helper()

	if got.Dir != want.Dir {
		t.Errorf("Dir = %q, want %q", got.Dir, want.Dir)
	}
	if got.Binary != want.Binary {
		t.Errorf("Binary = %q, want %q", got.Binary, want.Binary)
	}
	if got.Manifest.Name != want.Manifest.Name {
		t.Errorf("Manifest.Name = %q, want %q", got.Manifest.Name, want.Manifest.Name)
	}
	if got.Manifest.Version != want.Manifest.Version {
		t.Errorf("Manifest.Version = %q, want %q", got.Manifest.Version, want.Manifest.Version)
	}
	if got.Manifest.Kind != want.Manifest.Kind {
		t.Errorf("Manifest.Kind = %q, want %q", got.Manifest.Kind, want.Manifest.Kind)
	}
	if len(got.Manifest.Provides) != len(want.Manifest.Provides) {
		t.Fatalf("Provides len = %d, want %d", len(got.Manifest.Provides), len(want.Manifest.Provides))
	}
	for i := range want.Manifest.Provides {
		if got.Manifest.Provides[i] != want.Manifest.Provides[i] {
			t.Errorf("Provides[%d] = %q, want %q", i, got.Manifest.Provides[i], want.Manifest.Provides[i])
		}
	}
}
