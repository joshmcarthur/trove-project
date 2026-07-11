package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func moduleExecCmd(mod Module, settings *SettingsStore) (*exec.Cmd, error) {
	if mod.Dir == "" {
		return nil, fmt.Errorf("modules: module dir is required")
	}

	absDir, err := filepath.Abs(mod.Dir)
	if err != nil {
		return nil, fmt.Errorf("modules: module dir: %w", err)
	}

	binary := filepath.Join(absDir, binaryFileName)
	if mod.Binary != "" && mod.Binary != binary {
		return nil, fmt.Errorf("modules: module binary %q does not match %q", mod.Binary, binary)
	}

	info, err := os.Stat(binary)
	if err != nil {
		return nil, fmt.Errorf("modules: module binary: %w", err)
	}
	if !isExecutable(info) {
		return nil, fmt.Errorf("modules: module binary is not executable")
	}

	cmd := exec.Command(binary) //nolint:gosec // G204: path is not taken from untrusted input
	cmd.Env = os.Environ()
	applyModuleSettingsEnv(cmd, settings, mod.Manifest.Name)
	return cmd, nil
}
