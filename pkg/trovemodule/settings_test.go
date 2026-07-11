package trovemodule_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func TestMergeTOMLBytesOverlayReplacesScalars(t *testing.T) {
	t.Parallel()

	base := []byte(`
broker = "tcp://localhost:1883"
topics = ["home/#"]
qos = 0
`)
	overlay := []byte(`
broker = "tcp://mosquitto:1883"
`)

	merged, err := trovemodule.MergeTOMLBytes(base, overlay)
	if err != nil {
		t.Fatalf("MergeTOMLBytes() error = %v", err)
	}

	var cfg struct {
		Broker string   `toml:"broker"`
		Topics []string `toml:"topics"`
		QoS    int      `toml:"qos"`
	}
	if _, err := toml.Decode(string(merged), &cfg); err != nil {
		t.Fatalf("decode merged: %v", err)
	}
	if cfg.Broker != "tcp://mosquitto:1883" {
		t.Fatalf("broker = %q", cfg.Broker)
	}
	if len(cfg.Topics) != 1 || cfg.Topics[0] != "home/#" {
		t.Fatalf("topics = %#v", cfg.Topics)
	}
}

func TestLoadModuleConfigRejectsRelativeOverlayPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(`broker = "tcp://localhost:1883"`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Setenv(trovemodule.ModuleSettingsEnv, "relative/overlay.toml")

	var cfg struct {
		Broker string `toml:"broker"`
	}
	if err := trovemodule.LoadModuleConfig(dir, &cfg); err == nil {
		t.Fatal("LoadModuleConfig() error = nil, want error")
	}
}

func TestLoadModuleConfigAppliesOverlayEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(`
broker = "tcp://localhost:1883"
topics = ["home/#"]
`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	overlayPath := filepath.Join(dir, "overlay.toml")
	if err := os.WriteFile(overlayPath, []byte(`broker = "tcp://override:1883"`), 0o600); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	t.Setenv(trovemodule.ModuleSettingsEnv, overlayPath)

	var cfg struct {
		Broker string   `toml:"broker"`
		Topics []string `toml:"topics"`
	}
	if err := trovemodule.LoadModuleConfig(dir, &cfg); err != nil {
		t.Fatalf("LoadModuleConfig() error = %v", err)
	}
	if cfg.Broker != "tcp://override:1883" {
		t.Fatalf("broker = %q", cfg.Broker)
	}
}
