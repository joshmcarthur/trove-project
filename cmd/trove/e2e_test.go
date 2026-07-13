package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
	payload := fmt.Sprintf(`{"source":"e2e","type":"trove://type/http/ingest/received/1","text":%q}`, unique)
	postRecordsUntilOK(t, "http://"+addr+"/records", payload, 15*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectMCPWithRetry(t, ctx, addr, 20*time.Second)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_records",
		Arguments: map[string]any{
			"query": unique,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(search_records) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("search_records tool error: %#v", result)
	}
	if len(result.Content) == 0 {
		t.Fatal("search_records returned no content")
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
	postRecordsUntilOK(t, "http://"+addr+"/records", `{"source":"e2e","type":"trove://type/http/ingest/received/1","text":"probe"}`, timeout)
}

func postRecordsUntilOK(t *testing.T, url, payload string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastStatus int
	var lastBody string
	for time.Now().Before(deadline) {
		resp, err := http.Post(url, "application/json", strings.NewReader(payload))
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastStatus = resp.StatusCode
			lastBody = strings.TrimSpace(string(body))
			if resp.StatusCode == http.StatusCreated {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastBody != "" {
		t.Fatalf("POST %s status = %d, want 204; body = %q", url, lastStatus, lastBody)
	}
	t.Fatalf("POST %s status = %d, want 204", url, lastStatus)
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
	builtinDst := filepath.Join(filepath.Dir(root), "types", "builtin")
	if err := copyDir(filepath.Join(repoRoot, "types", "builtin"), builtinDst); err != nil {
		t.Fatalf("copy builtin types: %v", err)
	}
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
