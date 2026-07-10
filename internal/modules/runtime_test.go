package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/joshmcarthur/trove/internal/journal"
)

func TestStartSourceReceivesEmit(t *testing.T) {
	t.Parallel()

	binary := buildEmitOnceModule(t)
	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	mod := Module{
		Dir:    filepath.Dir(binary),
		Binary: binary,
		Manifest: Manifest{
			Name:    "emit-once",
			Version: "0.1.0",
			Kind:    KindSource,
		},
	}

	handle, err := StartSource(context.Background(), store, mod)
	if err != nil {
		t.Fatalf("StartSource() error = %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })

	events, err := store.Query(context.Background(), journal.Filter{TypePrefix: "test.emit.once"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Query() len = %d, want 1", len(events))
	}

	got := events[0]
	if got.Type != "test.emit.once" {
		t.Errorf("Type = %q, want %q", got.Type, "test.emit.once")
	}
	if got.Source != "emit-once" {
		t.Errorf("Source = %q, want %q", got.Source, "emit-once")
	}
	if string(got.Payload) != `{"hello":"world"}` {
		t.Errorf("Payload = %s, want %s", got.Payload, `{"hello":"world"}`)
	}
	if got.ID == "" {
		t.Error("ID is empty, want generated ULID")
	}
}

func buildEmitOnceModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")

	cmd := exec.Command("go", "build", "-o", binary, "./internal/modules/testdata/plugin/emit-once")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build emit-once module: %v\n%s", err, out)
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		t.Fatalf("chmod module binary: %v", err)
	}
	return binary
}

func openTestJournal(t *testing.T) *journal.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "journal.db")
	store, err := journal.Open(path)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	return store
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
