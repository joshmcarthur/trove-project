package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/joshmcarthur/trove/internal/config"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// SettingsStore resolves per-module settings overlays from trove.toml and
// exposes them to subprocesses via TROVE_MODULE_SETTINGS.
type SettingsStore struct {
	overlays  map[string]string
	tempFiles []string
}

// NewSettingsStore builds overlay file paths from [modules.config] and
// [modules.settings] in trove.toml.
func NewSettingsStore(cfg config.ModulesConfig) (*SettingsStore, error) {
	store := &SettingsStore{overlays: make(map[string]string)}
	names := moduleSettingNames(cfg)
	for _, name := range names {
		path, temp, err := resolveModuleOverlay(cfg, name)
		if err != nil {
			return nil, err
		}
		if path == "" {
			continue
		}
		store.overlays[name] = path
		if temp {
			store.tempFiles = append(store.tempFiles, path)
		}
	}
	return store, nil
}

// OverlayPath returns the settings overlay path for moduleName, if any.
func (s *SettingsStore) OverlayPath(moduleName string) string {
	if s == nil {
		return ""
	}
	return s.overlays[moduleName]
}

// Close removes temporary overlay files created for inline settings.
func (s *SettingsStore) Close() error {
	if s == nil {
		return nil
	}
	var errs []error
	for _, path := range s.tempFiles {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	s.tempFiles = nil
	if len(errs) > 0 {
		return fmt.Errorf("modules: remove settings overlays: %v", errs)
	}
	return nil
}

func moduleSettingNames(cfg config.ModulesConfig) []string {
	seen := make(map[string]struct{})
	var names []string
	for name := range cfg.Config {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for name := range cfg.Settings {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func resolveModuleOverlay(cfg config.ModulesConfig, moduleName string) (path string, temp bool, err error) {
	filePath := strings.TrimSpace(cfg.Config[moduleName])
	inline, hasInline := cfg.Settings[moduleName]

	if filePath == "" && !hasInline {
		return "", false, nil
	}
	if !hasInline && filePath != "" {
		return filePath, false, nil
	}

	var overlayMap map[string]any
	if filePath != "" {
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return "", false, fmt.Errorf("modules: settings config for %q: read %q: %w", moduleName, filePath, readErr)
		}
		if _, decodeErr := toml.Decode(string(data), &overlayMap); decodeErr != nil {
			return "", false, fmt.Errorf("modules: settings config for %q: parse %q: %w", moduleName, filePath, decodeErr)
		}
	}
	if hasInline {
		var inlineMap map[string]any
		if err := toml.PrimitiveDecode(inline, &inlineMap); err != nil {
			return "", false, fmt.Errorf("modules: settings for %q: parse inline settings: %w", moduleName, err)
		}
		if overlayMap == nil {
			overlayMap = make(map[string]any)
		}
		deepMergeSettings(overlayMap, inlineMap)
	}
	if len(overlayMap) == 0 {
		return "", false, nil
	}

	data, err := toml.Marshal(overlayMap)
	if err != nil {
		return "", false, fmt.Errorf("modules: settings for %q: marshal overlay: %w", moduleName, err)
	}

	f, err := os.CreateTemp("", "trove-module-settings-*")
	if err != nil {
		return "", false, fmt.Errorf("modules: settings for %q: temp file: %w", moduleName, err)
	}
	path = f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", false, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", false, err
	}
	return path, true, nil
}

func deepMergeSettings(base, overlay map[string]any) {
	for key, overlayValue := range overlay {
		baseValue, ok := base[key]
		if !ok {
			base[key] = overlayValue
			continue
		}
		baseMap, baseOK := baseValue.(map[string]any)
		overlayMap, overlayOK := overlayValue.(map[string]any)
		if baseOK && overlayOK {
			deepMergeSettings(baseMap, overlayMap)
			continue
		}
		base[key] = overlayValue
	}
}

func applyModuleSettingsEnv(cmd *exec.Cmd, settings *SettingsStore, moduleName string) {
	if settings == nil {
		return
	}
	path := settings.OverlayPath(moduleName)
	if path == "" {
		return
	}
	cmd.Env = append(cmd.Env, trovemodule.ModuleSettingsEnv+"="+path)
}
