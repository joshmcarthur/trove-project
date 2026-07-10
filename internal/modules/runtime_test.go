package modules

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

func TestStartSourceInvokesHealthcheck(t *testing.T) {
	oldInterval := healthcheckInterval
	healthcheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { healthcheckInterval = oldInterval })

	binary := buildHealthcheckLoopModule(t)
	counterFile := filepath.Join(t.TempDir(), "healthchecks")
	t.Setenv("TROVE_TEST_HEALTHCHECK_FILE", counterFile)

	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	mod := Module{
		Dir:    filepath.Dir(binary),
		Binary: binary,
		Manifest: Manifest{
			Name:    "healthcheck-loop",
			Version: "0.1.0",
			Kind:    KindSource,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		handle, err := StartSource(ctx, store, mod)
		if handle != nil {
			_ = handle.Close()
		}
		if err != nil && ctx.Err() == nil {
			t.Errorf("StartSource() error = %v", err)
		}
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	var count string
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(counterFile)
		if err == nil && len(data) > 0 {
			count = string(data)
			if count != "0" {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	if count == "" || count == "0" {
		t.Fatalf("healthcheck counter = %q, want at least 1 invocation", count)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StartSource did not return after context cancellation")
	}
}

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

func TestStartSourceHTTPIngest(t *testing.T) {
	t.Parallel()

	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	modDir, listenAddr := buildHTTPIngestModule(t)
	mod := Module{
		Dir:    modDir,
		Binary: filepath.Join(modDir, "module"),
		Manifest: Manifest{
			Name:    "http-ingest",
			Version: "1.0",
			Kind:    KindSource,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		handle, err := StartSource(ctx, store, mod)
		if handle != nil {
			_ = handle.Close()
		}
		if err != nil && ctx.Err() == nil {
			t.Errorf("StartSource() error = %v", err)
		}
		close(done)
	}()

	waitForTCP(t, listenAddr)

	resp, err := http.Post(
		"http://"+listenAddr+"/ingest/shortcuts",
		"application/json",
		strings.NewReader(`{"title":"test"}`),
	)
	if err != nil {
		t.Fatalf("POST /ingest/shortcuts: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	events, err := store.Query(context.Background(), journal.Filter{TypePrefix: "http.ingest.received"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Query() len = %d, want 1", len(events))
	}

	got := events[0]
	if got.Source != "shortcuts" {
		t.Errorf("Source = %q, want shortcuts", got.Source)
	}
	if string(got.Payload) != `{"title":"test"}` {
		t.Errorf("Payload = %s, want %s", got.Payload, `{"title":"test"}`)
	}
	if got.ID == "" {
		t.Error("ID is empty, want generated ULID")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StartSource did not return after context cancellation")
	}
}

func buildHealthcheckLoopModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")

	cmd := exec.Command("go", "build", "-o", binary, "./internal/modules/testdata/plugin/healthcheck-loop")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build healthcheck-loop module: %v\n%s", err, out)
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		t.Fatalf("chmod module binary: %v", err)
	}
	return binary
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

func buildHTTPIngestModule(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	listenAddr := ln.Addr().String()
	ln.Close()

	manifest := fmt.Sprintf(`name = "http-ingest"
version = "1.0"
kind = "source"
provides = ["http.ingest.received"]
listen = %q
`, listenAddr)
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", binary, "./modules/http-ingest")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build http-ingest module: %v\n%s", err, out)
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		t.Fatalf("chmod module binary: %v", err)
	}
	return dir, listenAddr
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready", addr)
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
