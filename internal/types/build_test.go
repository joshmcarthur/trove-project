package types_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/types"
)

func TestBuildCatalogLoadsBuiltins(t *testing.T) {
	t.Parallel()

	builtinDir := filepath.Join("..", "..", "types", "builtin")
	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	catalog, warnings, err := types.BuildCatalog(context.Background(), store, builtinDir, nil, nil)
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("BuildCatalog() warnings = %v, want none", warnings)
	}

	for _, uri := range []string{
		"trove://type/classify/pending/1",
		"trove://type/classify/assigned/1",
		"trove://type/note/created/1",
		"trove://type/http/ingest/received/1",
	} {
		if _, ok := catalog.Lookup(uri); !ok {
			t.Fatalf("Lookup(%q) ok = false, want true", uri)
		}
	}
}

func TestBuildCatalogLoadsModuleTypes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "custom.ttd.json")
	const raw = `{
  "$id": "trove://type/custom/event/1",
  "definition": {
    "properties": { "value": { "type": "string" } }
  }
}`
	if err := os.WriteFile(schemaPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	moduleTypes := []types.ModuleTypesInput{{
		ModuleName: "custom-module",
		ModuleDir:  dir,
		Types: []types.TypeDecl{{
			Name:    "custom.event",
			Version: 1,
			Schema:  "custom.ttd.json",
		}},
	}}

	catalog, warnings, err := types.BuildCatalog(context.Background(), store, "", moduleTypes, nil)
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("BuildCatalog() warnings = %v, want none", warnings)
	}

	entry, ok := catalog.Lookup("trove://type/custom/event/1")
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	if entry.Source != "custom-module" {
		t.Fatalf("Source = %q, want %q", entry.Source, "custom-module")
	}
	if !strings.HasPrefix(entry.SchemaRef, "sha256-") {
		t.Fatalf("SchemaRef = %q, want sha256- prefix", entry.SchemaRef)
	}
}

func TestBuildCatalogUserOverrideWarning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTTD := func(name, id, field string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		raw := `{
  "$id": "` + id + `",
  "definition": { "properties": { ` + field + `: { "type": "string" } } }
}`
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
		return path
	}

	moduleSchema := writeTTD("module.ttd.json", "trove://type/note/created/1", `"title"`)
	userSchema := writeTTD("user.ttd.json", "trove://type/note/created/1", `"headline"`)

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	moduleTypes := []types.ModuleTypesInput{{
		ModuleName: "notes-module",
		ModuleDir:  dir,
		Types: []types.TypeDecl{{
			Name:    "note.created",
			Version: 1,
			Schema:  filepath.Base(moduleSchema),
		}},
	}}
	userTypes := []types.TypeDecl{{
		Name:    "note.created",
		Version: 1,
		Schema:  userSchema,
	}}

	catalog, warnings, err := types.BuildCatalog(context.Background(), store, "", moduleTypes, userTypes)
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("BuildCatalog() warnings len = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0], "user override replaces") {
		t.Fatalf("warnings[0] = %q, want user override warning", warnings[0])
	}

	entry, ok := catalog.Lookup("trove://type/note/created/1")
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	if entry.Source != "user" {
		t.Fatalf("Source = %q, want %q", entry.Source, "user")
	}
}

func TestBuildCatalogSkipsMissingBuiltinDir(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	catalog, warnings, err := types.BuildCatalog(
		context.Background(),
		store,
		filepath.Join(t.TempDir(), "missing-builtin"),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("BuildCatalog() warnings = %v, want none", warnings)
	}
	if catalog == nil {
		t.Fatal("catalog = nil, want non-nil empty catalog")
	}
}
