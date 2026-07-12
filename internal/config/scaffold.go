package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultConfigName = "trove.toml"

// ScaffoldResult describes files created by Scaffold.
type ScaffoldResult struct {
	Dir         string
	ConfigPath  string
	BlobsPath   string
	JournalPath string
}

// Scaffold writes a default trove.toml and creates the blobs directory in dir.
// When dir contains a modules/ subdirectory, ./modules is included in module paths.
func Scaffold(dir string, force bool) (ScaffoldResult, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ScaffoldResult{}, fmt.Errorf("config: init dir: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(absDir, 0o755); mkErr != nil {
				return ScaffoldResult{}, fmt.Errorf("config: init create dir %q: %w", absDir, mkErr)
			}
		} else {
			return ScaffoldResult{}, fmt.Errorf("config: init stat %q: %w", absDir, err)
		}
	} else if !info.IsDir() {
		return ScaffoldResult{}, fmt.Errorf("config: init %q is not a directory", absDir)
	}

	configPath := filepath.Join(absDir, defaultConfigName)
	if _, err := os.Stat(configPath); err == nil && !force {
		return ScaffoldResult{}, fmt.Errorf("config: init %q already exists (use --force to overwrite)", configPath)
	} else if err != nil && !os.IsNotExist(err) {
		return ScaffoldResult{}, fmt.Errorf("config: init stat %q: %w", configPath, err)
	}

	blobsPath := filepath.Join(absDir, "blobs")
	if err := os.MkdirAll(blobsPath, 0o755); err != nil {
		return ScaffoldResult{}, fmt.Errorf("config: init create blobs dir %q: %w", blobsPath, err)
	}

	content := defaultConfigTOML(modulePathsForDir(absDir))
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return ScaffoldResult{}, fmt.Errorf("config: init write %q: %w", configPath, err)
	}

	return ScaffoldResult{
		Dir:         absDir,
		ConfigPath:  configPath,
		BlobsPath:   blobsPath,
		JournalPath: filepath.Join(absDir, "trove.db"),
	}, nil
}

func modulePathsForDir(dir string) []string {
	paths := []string{"~/.local/lib/trove/modules"}
	modulesDir := filepath.Join(dir, "modules")
	if info, err := os.Stat(modulesDir); err == nil && info.IsDir() {
		paths = append([]string{"./modules"}, paths...)
	}
	return paths
}

func defaultConfigTOML(modulePaths []string) string {
	var paths strings.Builder
	for i, p := range modulePaths {
		if i > 0 {
			paths.WriteString(", ")
		}
		fmt.Fprintf(&paths, "%q", p)
	}

	return fmt.Sprintf(`[journal]
path = "./trove.db"

[blobs]
backend = "filesystem"
path = "./blobs"

[modules]
paths = [%s]

[http]
listen = "127.0.0.1:8080"
`, paths.String())
}
