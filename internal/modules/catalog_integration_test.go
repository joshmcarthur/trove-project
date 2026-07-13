package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/types"
)

func TestRuntimeBuildsCatalogFromModules(t *testing.T) {
	t.Parallel()

	pluginDir := filepath.Join(moduleRoot(t), "internal", "modules", "testdata", "plugin", "typed-emit")
	modDir, binary := buildTypedEmitModule(t, pluginDir)

	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	blobStore, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	moduleTypes, err := CollectModuleTypesInputs([]Module{{
		Dir:    modDir,
		Binary: binary,
		Manifest: Manifest{
			Name:     "typed-emit",
			Version:  "0.1.0",
			Kind:     KindSource,
			Provides: []string{"trove://type/test/typed/emit/1"},
		},
	}})
	if err != nil {
		t.Fatalf("CollectModuleTypesInputs() error = %v", err)
	}

	catalog, warnings, err := types.BuildCatalog(context.Background(), blobStore, "", moduleTypes, nil)
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("BuildCatalog() warnings = %v, want none", warnings)
	}

	mod := Module{
		Dir:    modDir,
		Binary: binary,
		Manifest: Manifest{
			Name:     "typed-emit",
			Version:  "0.1.0",
			Kind:     KindSource,
			Provides: []string{"trove://type/test/typed/emit/1"},
		},
	}

	handle, err := StartSource(context.Background(), store, mod, blobStore, nil, NewMCPRegistry(), nil, nil, map[string]string{}, nil, catalog)
	if err != nil {
		t.Fatalf("StartSource() error = %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })

	const eventType = "trove://type/test/typed/emit/1"
	var events []journal.Revision
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		events, err = store.Query(context.Background(), journal.Filter{TypePrefix: eventType})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) == 1 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if len(events) != 1 {
		t.Fatalf("Query() len = %d, want 1", len(events))
	}

	got := events[0]
	if got.Type != eventType {
		t.Errorf("Type = %q, want %q", got.Type, eventType)
	}
	if got.Source != "typed-emit" {
		t.Errorf("Source = %q, want %q", got.Source, "typed-emit")
	}
	if got.SchemaRef == "" {
		t.Error("SchemaRef is empty, want catalog schema_ref")
	}
	if string(got.Payload) != `{"message":"hello"}` {
		t.Errorf("Payload = %s, want %s", got.Payload, `{"message":"hello"}`)
	}
}

func buildTypedEmitModule(t *testing.T, pluginDir string) (string, string) {
	t.Helper()

	modDir := t.TempDir()
	copyTestFile(t, filepath.Join(pluginDir, "manifest.toml"), filepath.Join(modDir, "manifest.toml"))
	copyTestDir(t, filepath.Join(pluginDir, "types"), filepath.Join(modDir, "types"))

	binary := filepath.Join(modDir, "module")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = pluginDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build typed-emit module: %v\n%s", err, out)
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		t.Fatalf("chmod module binary: %v", err)
	}
	return modDir, binary
}

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", dst, err)
	}
}

func copyTestDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dst, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", src, err)
	}
	for _, entry := range entries {
		copyTestFile(t, filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()))
	}
}
