package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func troveBin(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	bin := filepath.Join(dir, "trove")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v\n%s", err, stderr.String())
	}
	return bin
}

func runTrove(t *testing.T, bin string, args ...string) (stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return errBuf.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run trove %v: %v", args, err)
	}
	return errBuf.String(), exitErr.ExitCode()
}

func writeConfig(t *testing.T, journalPath string) string {
	t.Helper()

	dir := filepath.Dir(journalPath)
	path := filepath.Join(dir, "trove.toml")
	content := fmt.Sprintf(`[journal]
path = %q

[blobs]
path = %q

[http]
listen = ":18080"
`, journalPath, filepath.Join(dir, "blobs"))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestCLI(t *testing.T) {
	t.Parallel()

	bin := troveBin(t)

	t.Run("version", func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command(bin, "-version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("run -version: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("expected version output")
		}
	})

	tests := []struct {
		name       string
		args       func(t *testing.T) []string
		wantExit   int
		wantStderr string
	}{
		{
			name: "missing config",
			args: func(t *testing.T) []string {
				return nil
			},
			wantExit:   1,
			wantStderr: "-config is required",
		},
		{
			name: "invalid config",
			args: func(t *testing.T) []string {
				path := filepath.Join(t.TempDir(), "bad.toml")
				if err := os.WriteFile(path, []byte("not valid toml [[[["), 0o644); err != nil {
					t.Fatalf("write config: %v", err)
				}
				return []string{"-config", path}
			},
			wantExit:   1,
			wantStderr: "config:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stderr, code := runTrove(t, bin, tt.args(t)...)
			if code != tt.wantExit {
				t.Errorf("exit code = %d, want %d\nstderr: %s", code, tt.wantExit, stderr)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantStderr)
			}
		})
	}

	t.Run("valid config runs until signal", func(t *testing.T) {
		t.Parallel()

		journalPath := filepath.Join(t.TempDir(), "trove.db")
		configPath := writeConfig(t, journalPath)

		cmd := exec.Command(bin, "-config", configPath)
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf
		if err := cmd.Start(); err != nil {
			t.Fatalf("start trove: %v", err)
		}

		time.Sleep(200 * time.Millisecond)

		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("signal SIGTERM: %v", err)
		}

		err := cmd.Wait()
		if err == nil {
			// clean exit
		} else {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 0 {
				t.Fatalf("wait trove: %v\nstderr: %s", err, errBuf.String())
			}
		}

		stderr := errBuf.String()
		if !strings.Contains(stderr, "starting modules:") || !strings.Contains(stderr, "http-ingest") || !strings.Contains(stderr, "mcp-query") || !strings.Contains(stderr, "type-catalog") {
			t.Errorf("stderr = %q, want bundled modules started", stderr)
		}
		if !strings.Contains(stderr, "http gateway listening on :18080") {
			t.Errorf("stderr = %q, want substring %q", stderr, "http gateway listening on :18080")
		}
		if !strings.Contains(stderr, "shutting down") {
			t.Errorf("stderr = %q, want substring %q", stderr, "shutting down")
		}
	})

	t.Run("types list", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		journalPath := filepath.Join(dir, "trove.db")
		configPath := writeConfig(t, journalPath)

		cmd := exec.Command(bin, "-config", configPath, "types", "list")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("run types list: %v\nstderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "trove://type/note/created/1") {
			t.Errorf("stdout = %q, want note.created type", out)
		}
	})

	t.Run("types validate file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		journalPath := filepath.Join(dir, "trove.db")
		configPath := writeConfig(t, journalPath)
		ttdPath := filepath.Join(dir, "test.ttd.json")
		ttd := `{
  "$id": "trove://type/example/test/1",
  "definition": { "properties": { "name": { "type": "string" } } }
}`
		if err := os.WriteFile(ttdPath, []byte(ttd), 0o644); err != nil {
			t.Fatalf("write ttd: %v", err)
		}

		cmd := exec.Command(bin, "-config", configPath, "types", "validate", "--file", ttdPath)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("run types validate: %v\nstderr: %s", err, stderr.String())
		}
		if !strings.Contains(stdout.String(), "trove://type/example/test/1") {
			t.Errorf("stdout = %q, want validated uri", stdout.String())
		}
	})
}
