package types

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joshmcarthur/trove/internal/blob"
)

// ModuleTypesInput groups type declarations contributed by a module manifest.
type ModuleTypesInput struct {
	ModuleName string
	ModuleDir  string
	Types      []TypeDecl // paths relative to ModuleDir unless absolute
}

// BuildCatalog loads builtin, module, and user type definitions into a catalog.
// Warnings are non-fatal notices (e.g. user overrides). builtinDir may be empty;
// when the directory does not exist, builtins are skipped.
func BuildCatalog(
	ctx context.Context,
	blobs blob.Store,
	builtinDir string,
	moduleTypes []ModuleTypesInput,
	userTypes []TypeDecl,
) (*Catalog, []string, error) {
	catalog := NewCatalog()
	var warnings []string

	if builtinDir != "" {
		builtinWarnings, err := loadBuiltinDir(ctx, blobs, catalog, builtinDir)
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, builtinWarnings...)
	}

	for _, mod := range moduleTypes {
		for _, decl := range mod.Types {
			schemaPath, err := resolveSchemaPath(mod.ModuleDir, decl.Schema)
			if err != nil {
				return nil, nil, err
			}
			warning, err := registerDeclaredType(ctx, blobs, catalog, decl, schemaPath, mod.ModuleName)
			if err != nil {
				return nil, nil, fmt.Errorf("types: module %q: %w", mod.ModuleName, err)
			}
			if warning != "" {
				warnings = append(warnings, warning)
			}
		}
	}

	for _, decl := range userTypes {
		schemaPath, err := resolveSchemaPath("", decl.Schema)
		if err != nil {
			return nil, nil, err
		}
		warning, err := registerDeclaredType(ctx, blobs, catalog, decl, schemaPath, "user")
		if err != nil {
			return nil, nil, fmt.Errorf("types: user type %q: %w", decl.Name, err)
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}

	return catalog, warnings, nil
}

// DefaultBuiltinDir returns the first existing types/builtin directory from common locations.
func DefaultBuiltinDir() string {
	candidates := []string{"types/builtin"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "types/builtin"))
	}
	for _, dir := range candidates {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
	}
	return "types/builtin"
}

func loadBuiltinDir(ctx context.Context, blobs blob.Store, catalog *Catalog, dir string) ([]string, error) {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("types: builtin dir %q: %w", dir, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("types: builtin dir %q is not a directory", dir)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.ttd.json"))
	if err != nil {
		return nil, fmt.Errorf("types: glob builtin types: %w", err)
	}
	sort.Strings(matches)

	var warnings []string
	for _, path := range matches {
		warning, err := registerBuiltinType(ctx, blobs, catalog, path)
		if err != nil {
			return nil, fmt.Errorf("types: builtin %q: %w", path, err)
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

func registerBuiltinType(ctx context.Context, blobs blob.Store, catalog *Catalog, path string) (string, error) {
	td, err := LoadTypeFile(path, "builtin")
	if err != nil {
		return "", err
	}
	return registerTypeDefinition(ctx, blobs, catalog, td, "builtin", path)
}

func registerDeclaredType(
	ctx context.Context,
	blobs blob.Store,
	catalog *Catalog,
	decl TypeDecl,
	schemaPath string,
	source string,
) (string, error) {
	td, err := LoadTypeFile(schemaPath, source)
	if err != nil {
		return "", err
	}
	uri, err := DeclURI(decl)
	if err != nil {
		return "", err
	}
	if td.ID != uri {
		return "", fmt.Errorf("$id %q does not match declaration URI %q", td.ID, uri)
	}
	return registerTypeDefinition(ctx, blobs, catalog, td, source, schemaPath)
}

func registerTypeDefinition(
	ctx context.Context,
	blobs blob.Store,
	catalog *Catalog,
	td TypeDefinition,
	source string,
	sourcePath string,
) (string, error) {
	ct, err := Compile(td)
	if err != nil {
		return "", err
	}
	schemaRef, err := StoreTypeDefinition(ctx, blobs, td)
	if err != nil {
		return "", err
	}
	return catalog.Register(Entry{
		URI:        td.ID,
		SchemaRef:  schemaRef,
		Compiled:   ct,
		Source:     source,
		SourcePath: sourcePath,
	})
}

func resolveSchemaPath(baseDir, schema string) (string, error) {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return "", fmt.Errorf("types: schema path is required")
	}
	if filepath.IsAbs(schema) {
		return schema, nil
	}
	if baseDir == "" {
		return schema, nil
	}
	return filepath.Join(baseDir, schema), nil
}
