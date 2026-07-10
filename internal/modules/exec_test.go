package modules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleExecCmd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "module")
	if err := os.WriteFile(binary, []byte{0}, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	mod := Module{Dir: dir, Binary: binary}

	cmd, err := moduleExecCmd(mod)
	if err != nil {
		t.Fatalf("moduleExecCmd() error = %v", err)
	}
	if cmd.Path != binary {
		t.Errorf("cmd.Path = %q, want %q", cmd.Path, binary)
	}
}
