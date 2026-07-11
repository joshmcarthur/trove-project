package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestE2EIngestAndMCPQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping host E2E in -short mode")
	}

	repoRoot := findRepoRoot(t)
	bin := buildTroveBinary(t, repoRoot)
	modulesRoot := buildE2EModules(t, repoRoot)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "trove.db")
	blobsPath := filepath.Join(workDir, "blobs")
	configPath := filepath.Join(workDir, "trove.toml")
	config := fmt.Sprintf(`[journal]
path = %q

[blobs]
path = %q

[modules]
paths = [%q]

[http]
listen = %q
`, journalPath, blobsPath, modulesRoot, addr)
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(bin, "-config", configPath)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start trove: %v", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	waitForTCP(t, addr, 15*time.Second)
	waitForIngest(t, addr, 15*time.Second)

	unique := fmt.Sprintf("e2e-marker-%d", time.Now().UnixNano())
	payload := fmt.Sprintf(`{"type":"http.ingest.received","text":%q}`, unique)
	resp, err := http.Post(
		"http://"+addr+"/ingest/e2e",
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST ingest status = %d, want 204", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectMCPWithRetry(t, ctx, addr, 20*time.Second)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_events",
		Arguments: map[string]any{
			"query": unique,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(search_events) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("search_events tool error: %#v", result)
	}
	if len(result.Content) == 0 {
		t.Fatal("search_events returned no content")
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	select {
	case err := <-waitErr:
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 0 {
				t.Fatalf("trove exit: %v\nstderr: %s", err, errBuf.String())
			}
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("trove did not exit after SIGTERM\nstderr: %s", errBuf.String())
	}
}

func waitForIngest(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Post(
			"http://"+addr+"/ingest/e2e",
			"application/json",
			strings.NewReader(`{"text":"probe"}`),
		)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("ingest endpoint not ready within %s", timeout)
}

func connectMCPWithRetry(t *testing.T, ctx context.Context, addr string, timeout time.Duration) *mcp.ClientSession {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		client := mcp.NewClient(&mcp.Implementation{Name: "e2e-test", Version: "0.1.0"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint: "http://" + addr + "/mcp",
		}, nil)
		if err == nil {
			return session
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("MCP Connect() not ready within %s: %v", timeout, lastErr)
	return nil
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func buildTroveBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "trove")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/trove")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build trove: %v\n%s", err, out)
	}
	return bin
}

func buildE2EModules(t *testing.T, repoRoot string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "modules")
	for _, mod := range []string{"http-ingest", "mcp-query"} {
		dst := filepath.Join(root, mod)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", mod, err)
		}
		manifest, err := os.ReadFile(filepath.Join(repoRoot, "modules", mod, "manifest.toml"))
		if err != nil {
			t.Fatalf("read manifest %s: %v", mod, err)
		}
		if err := os.WriteFile(filepath.Join(dst, "manifest.toml"), manifest, 0o644); err != nil {
			t.Fatalf("write manifest %s: %v", mod, err)
		}
		binary := filepath.Join(dst, "module")
		src := "./modules/http-ingest/cmd"
		if mod == "mcp-query" {
			src = "./modules/mcp-query/cmd"
		}
		cmd := exec.Command("go", "build", "-o", binary, src)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", mod, err, out)
		}
		if err := os.Chmod(binary, 0o755); err != nil {
			t.Fatalf("chmod %s: %v", mod, err)
		}
	}
	return root
}

func waitForTCP(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready within %s", addr, timeout)
}
