package modules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleExecCmdSetsBlobsPathEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")
	if err := os.WriteFile(binary, []byte{0}, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	mod := Module{Dir: dir, Binary: binary}
	blobsPath := filepath.Join(t.TempDir(), "blobs")

	cmd, err := moduleExecCmd(mod, blobsPath)
	if err != nil {
		t.Fatalf("moduleExecCmd() error = %v", err)
	}

	want := EnvBlobsPath + "=" + blobsPath
	found := false
	for _, env := range cmd.Env {
		if env == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cmd.Env = %v, want %q", cmd.Env, want)
	}
}

func TestModuleExecCmdOmitsBlobsPathWhenEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")
	if err := os.WriteFile(binary, []byte{0}, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	mod := Module{Dir: dir, Binary: binary}

	cmd, err := moduleExecCmd(mod, "")
	if err != nil {
		t.Fatalf("moduleExecCmd() error = %v", err)
	}

	for _, env := range cmd.Env {
		if len(env) > len(EnvBlobsPath) && env[:len(EnvBlobsPath)] == EnvBlobsPath {
			t.Fatalf("cmd.Env = %v, want no %s entry", cmd.Env, EnvBlobsPath)
		}
	}
}
