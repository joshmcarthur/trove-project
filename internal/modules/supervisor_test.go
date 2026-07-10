package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

func TestRunSourcesSurvivesModuleCrash(t *testing.T) {
	binary := buildCrashRestartModule(t)
	counterFile := filepath.Join(t.TempDir(), "counter")
	if err := os.WriteFile(counterFile, []byte("0"), 0o644); err != nil {
		t.Fatalf("write counter file: %v", err)
	}
	t.Setenv("TROVE_TEST_COUNTER_FILE", counterFile)

	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	mod := Module{
		Dir:    filepath.Dir(binary),
		Binary: binary,
		Manifest: Manifest{
			Name:    "crash-restart",
			Version: "0.1.0",
			Kind:    KindSource,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		RunSources(ctx, store, []Module{mod})
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events, err := store.Query(context.Background(), journal.Filter{TypePrefix: "test.crash.restart"})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) >= 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	events, err := store.Query(context.Background(), journal.Filter{TypePrefix: "test.crash.restart"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("Query() len = %d, want at least 2 restarts", len(events))
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunSources did not return after context cancellation")
	}
}

func buildCrashRestartModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")

	cmd := exec.Command("go", "build", "-o", binary, "./internal/modules/testdata/plugin/crash-restart")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build crash-restart module: %v\n%s", err, out)
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		t.Fatalf("chmod module binary: %v", err)
	}
	return binary
}
