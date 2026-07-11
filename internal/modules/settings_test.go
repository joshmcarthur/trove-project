package modules

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/joshmcarthur/trove/internal/config"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func TestNewSettingsStoreInlineOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "trove.toml")
	if err := os.WriteFile(tomlPath, []byte(`
[journal]
path = "./trove.db"

[blobs]
backend = "filesystem"

[http]
listen = ":8080"

[modules.settings.mqtt-source]
broker = "tcp://override:1883"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loaded, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	store, err := NewSettingsStore(loaded.Modules)
	if err != nil {
		t.Fatalf("NewSettingsStore() error = %v", err)
	}
	defer store.Close()

	path := store.OverlayPath("mqtt-source")
	if path == "" {
		t.Fatal("OverlayPath() is empty")
	}

	cmd := &exec.Cmd{Env: os.Environ()}
	applyModuleSettingsEnv(cmd, store, "mqtt-source")
	want := trovemodule.ModuleSettingsEnv + "=" + path
	if !envContains(cmd.Env, want) {
		t.Fatalf("cmd.Env missing %q", want)
	}
}

func TestNewSettingsStoreExternalConfigFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "mqtt.toml")
	if err := os.WriteFile(overlayPath, []byte(`broker = "tcp://external:1883"`), 0o600); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	store, err := NewSettingsStore(config.ModulesConfig{
		Config: map[string]string{"mqtt-source": overlayPath},
	})
	if err != nil {
		t.Fatalf("NewSettingsStore() error = %v", err)
	}
	defer store.Close()

	if got := store.OverlayPath("mqtt-source"); got != overlayPath {
		t.Fatalf("OverlayPath() = %q, want %q", got, overlayPath)
	}
}

func envContains(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
