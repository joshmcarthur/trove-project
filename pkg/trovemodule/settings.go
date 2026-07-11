package trovemodule

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ModuleSettingsEnv is set by the Trove parent when trove.toml provides module
// settings overlays for a subprocess.
const ModuleSettingsEnv = "TROVE_MODULE_SETTINGS"

// BundledModuleEnv is set when the Trove parent reexecs itself to run a
// built-in module subprocess. Must not use TROVE_MODULE — that key is reserved
// for the go-plugin handshake magic cookie.
const BundledModuleEnv = "TROVE_BUNDLED_MODULE"

// LoadModuleConfig decodes manifest.toml from dir into dest and merges an
// optional settings overlay from TROVE_MODULE_SETTINGS when set.
func LoadModuleConfig(dir string, dest any) error {
	manifestPath := filepath.Join(dir, "manifest.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("trovemodule: read manifest: %w", err)
	}
	return LoadModuleConfigBytes(data, dest)
}

// LoadModuleConfigBytes decodes manifest TOML from base and merges an optional
// settings overlay from TROVE_MODULE_SETTINGS when set.
func LoadModuleConfigBytes(base []byte, dest any) error {
	data := base

	overlayPath, err := moduleSettingsPathFromEnv()
	if err != nil {
		return err
	}
	if overlayPath != "" {
		overlayData, err := os.ReadFile(overlayPath) //nolint:gosec // G703: path validated by moduleSettingsPathFromEnv; parent-controlled env
		if err != nil {
			return fmt.Errorf("trovemodule: read module settings overlay: %w", err)
		}
		data, err = MergeTOMLBytes(data, overlayData)
		if err != nil {
			return err
		}
	}

	if _, err := toml.Decode(string(data), dest); err != nil {
		return fmt.Errorf("trovemodule: parse module config: %w", err)
	}
	return nil
}

func moduleSettingsPathFromEnv() (string, error) {
	raw := strings.TrimSpace(os.Getenv(ModuleSettingsEnv))
	if raw == "" {
		return "", nil
	}
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("trovemodule: %s must be an absolute path", ModuleSettingsEnv)
	}
	clean := filepath.Clean(raw)
	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("trovemodule: %s: %w", ModuleSettingsEnv, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("trovemodule: %s must point to a file", ModuleSettingsEnv)
	}
	return clean, nil
}

// MergeTOMLBytes deep-merges overlay onto base at the map level. Overlay keys
// replace scalars and whole arrays or tables in base.
func MergeTOMLBytes(base, overlay []byte) ([]byte, error) {
	var baseMap map[string]any
	if _, err := toml.Decode(string(base), &baseMap); err != nil {
		return nil, fmt.Errorf("trovemodule: parse base config: %w", err)
	}
	if len(overlay) == 0 {
		return base, nil
	}

	var overlayMap map[string]any
	if _, err := toml.Decode(string(overlay), &overlayMap); err != nil {
		return nil, fmt.Errorf("trovemodule: parse settings overlay: %w", err)
	}
	if len(overlayMap) == 0 {
		return base, nil
	}
	if baseMap == nil {
		baseMap = make(map[string]any)
	}
	deepMergeMaps(baseMap, overlayMap)

	merged, err := toml.Marshal(baseMap)
	if err != nil {
		return nil, fmt.Errorf("trovemodule: marshal merged config: %w", err)
	}
	return merged, nil
}

func deepMergeMaps(base, overlay map[string]any) {
	for key, overlayValue := range overlay {
		baseValue, ok := base[key]
		if !ok {
			base[key] = overlayValue
			continue
		}

		baseMap, baseOK := baseValue.(map[string]any)
		overlayMap, overlayOK := overlayValue.(map[string]any)
		if baseOK && overlayOK {
			deepMergeMaps(baseMap, overlayMap)
			continue
		}
		base[key] = overlayValue
	}
}
