package gateway_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/gateway"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/modules"
)

func TestHTTPIngestViaGateway(t *testing.T) {
	t.Parallel()

	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	blobStore, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	modDir := buildHTTPIngestModule(t)
	mod := modules.Module{
		Dir:    modDir,
		Binary: filepath.Join(modDir, "module"),
		Manifest: modules.Manifest{
			Name:     "http-ingest",
			Version:  "1.0",
			Kind:     modules.KindSource,
			Provides: []string{"http.ingest.received", "note.*", "shortcut.*"},
		},
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	gwAddr := ln.Addr().String()
	ln.Close()

	routes := []modules.HTTPRouteEntry{
		{Route: modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}"}, Module: "http-ingest"},
		{Route: modules.HTTPRoute{Method: "PUT", Path: "/blobs"}, Module: "http-ingest"},
	}
	registry := modules.NewHTTPRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gw, err := gateway.New(gateway.Config{Listen: gwAddr, MaxBodyBytes: 10 << 20}, routes, registry, modules.NewAuthRegistry(), nil)
	if err != nil {
		t.Fatalf("gateway.New() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		handle, err := modules.StartSource(ctx, store, mod, blobStore, registry, modules.NewAuthRegistry(), modules.NewMCPRegistry(), nil, nil, map[string]string{}, nil)
		if err != nil && ctx.Err() == nil {
			t.Errorf("StartSource() error = %v", err)
		}
		if handle != nil {
			<-ctx.Done()
			_ = handle.Close()
		}
		close(done)
	}()

	go func() {
		_ = gw.Serve(ctx)
	}()

	waitForTCP(t, gwAddr)

	var resp *http.Response
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Post(
			"http://"+gwAddr+"/ingest/shortcuts",
			"application/json",
			strings.NewReader(`{"title":"test"}`),
		)
		if err == nil && resp.StatusCode == http.StatusNoContent {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
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

func openTestJournal(t *testing.T) *journal.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "journal.db")
	store, err := journal.Open(path)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	return store
}

func buildHTTPIngestModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")

	manifest := `name = "http-ingest"
version = "1.0"
kind = "source"
provides = ["http.ingest.received", "note.*", "shortcut.*"]

[[http.routes]]
method = "POST"
path = "/ingest/{source}"

[[http.routes]]
method = "PUT"
path = "/blobs"
`
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
	return dir
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
