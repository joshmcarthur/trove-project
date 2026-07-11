package modules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func TestModuleExecCmd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")
	if err := os.WriteFile(binary, []byte{0}, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	mod := Module{Dir: dir, Binary: binary}

	cmd, err := moduleExecCmd(mod, nil)
	if err != nil {
		t.Fatalf("moduleExecCmd() error = %v", err)
	}
	if cmd.Path != binary {
		t.Errorf("cmd.Path = %q, want %q", cmd.Path, binary)
	}
}

func TestModuleExecCmdBundled(t *testing.T) {
	t.Parallel()

	mod := Module{
		Bundled:  true,
		Manifest: Manifest{Name: "http-ingest"},
	}

	cmd, err := moduleExecCmd(mod, nil)
	if err != nil {
		t.Fatalf("moduleExecCmd() error = %v", err)
	}
	if cmd.Path == "" {
		t.Fatal("cmd.Path is empty")
	}
	found := false
	for _, kv := range cmd.Env {
		if kv == trovemodule.BundledModuleEnv+"=http-ingest" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("cmd.Env = %v, want %s=http-ingest", cmd.Env, trovemodule.BundledModuleEnv)
	}
}
