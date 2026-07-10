package modules

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

const (
	manifestFileName = "manifest.toml"
	binaryFileName   = "module"
)

// Module is a discovered local module ready for go-plugin launch.
type Module struct {
	Dir      string
	Binary   string
	Manifest Manifest
}

// Discover scans paths and returns valid modules. Invalid entries are skipped;
// per-path read errors are aggregated into a returned error (if any modules
// were still found, return both slice and error).
func Discover(paths []string) ([]Module, error) {
	var modules []Module
	seen := make(map[string]struct{})
	var errs []error

	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("modules: discover: stat %q: %w", root, err))
			continue
		}
		if !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			errs = append(errs, fmt.Errorf("modules: discover: readdir %q: %w", root, err))
			continue
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			mod, ok := discoverModule(filepath.Join(root, entry.Name()))
			if !ok {
				continue
			}
			if _, exists := seen[mod.Manifest.Name]; exists {
				continue
			}
			seen[mod.Manifest.Name] = struct{}{}
			modules = append(modules, mod)
		}
	}

	if len(errs) > 0 {
		return modules, errors.Join(errs...)
	}
	return modules, nil
}

func discoverModule(dir string) (Module, bool) {
	manifestPath := filepath.Join(dir, manifestFileName)
	if _, err := os.Stat(manifestPath); err != nil {
		return Module{}, false
	}

	manifest, err := ParseManifestFile(manifestPath)
	if err != nil {
		return Module{}, false
	}

	binaryPath := filepath.Join(dir, binaryFileName)
	info, err := os.Stat(binaryPath)
	if err != nil {
		return Module{}, false
	}
	if !isExecutable(info) {
		return Module{}, false
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	return Module{
		Dir:      absDir,
		Binary:   filepath.Join(absDir, binaryFileName),
		Manifest: manifest,
	}, true
}

func isExecutable(info fs.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}
