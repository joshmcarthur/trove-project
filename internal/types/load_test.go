package types_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestLoadTypeFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "note-created.json")
	const raw = `{
  "$id": "trove://type/note/created/1",
  "definition": {
    "properties": { "title": { "type": "string" } }
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	td, err := types.LoadTypeFile(path, "notes-module")
	if err != nil {
		t.Fatalf("LoadTypeFile() error = %v", err)
	}
	if td.ID != "trove://type/note/created/1" {
		t.Fatalf("LoadTypeFile() ID = %q", td.ID)
	}
}

func TestDeclURI(t *testing.T) {
	t.Parallel()
	uri, err := types.DeclURI(types.TypeDecl{
		Name:    "journal.entry",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("DeclURI() error = %v", err)
	}
	want := "trove://type/journal/entry/1"
	if uri != want {
		t.Fatalf("DeclURI() = %q, want %q", uri, want)
	}
}

func TestDeclURIRequiresName(t *testing.T) {
	t.Parallel()
	_, err := types.DeclURI(types.TypeDecl{Version: 1})
	if err == nil {
		t.Fatal("DeclURI() error = nil, want name required error")
	}
}

func TestDeclURIRequiresPositiveVersion(t *testing.T) {
	t.Parallel()
	_, err := types.DeclURI(types.TypeDecl{Name: "note.created", Version: 0})
	if err == nil {
		t.Fatal("DeclURI() error = nil, want positive version error")
	}
}
