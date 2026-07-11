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
	Bundled  bool
}

var bundledDiscover func() ([]Module, error)

// Discover scans paths and appends built-in modules not overridden on disk.
func Discover(paths []string) ([]Module, error) {
	mods, err := discoverPaths(paths)
	if err != nil {
		return mods, err
	}

	bundled, err := bundledModules()
	if err != nil {
		return mods, err
	}

	seen := make(map[string]struct{}, len(mods))
	for _, mod := range mods {
		seen[mod.Manifest.Name] = struct{}{}
	}

	for _, mod := range bundled {
		if _, exists := seen[mod.Manifest.Name]; exists {
			continue
		}
		mods = append(mods, mod)
	}

	return mods, nil
}

func bundledModules() ([]Module, error) {
	if bundledDiscover == nil {
		return nil, nil
	}
	return bundledDiscover()
}

// SetBundledDiscover registers the built-in module provider.
func SetBundledDiscover(fn func() ([]Module, error)) {
	bundledDiscover = fn
}

func discoverPaths(paths []string) ([]Module, error) {
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
