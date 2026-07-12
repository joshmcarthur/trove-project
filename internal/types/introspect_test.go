package types_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/types"
)

func TestCatalogListSorted(t *testing.T) {
	t.Parallel()
	c := types.NewCatalog()
	for _, uri := range []string{
		"trove://type/z/last/1",
		"trove://type/a/first/1",
		"trove://type/m/middle/1",
	} {
		if _, err := c.Register(types.Entry{URI: uri, SchemaRef: "blob:" + uri, Source: "test"}); err != nil {
			t.Fatalf("Register(%q) error = %v", uri, err)
		}
	}
	list := c.List()
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	if list[0].URI != "trove://type/a/first/1" {
		t.Fatalf("List()[0].URI = %q, want first sorted URI", list[0].URI)
	}
}

func TestCatalogSummary(t *testing.T) {
	t.Parallel()
	c := types.NewCatalog()
	td := types.TypeDefinition{
		ID:          noteCreatedURI,
		Title:       "Note created",
		Description: "A note",
		Definition:  []byte(`{"properties":{"title":{"type":"string"}}}`),
		Status:      "active",
	}
	ct, err := types.Compile(td)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if _, err := c.Register(types.Entry{
		URI:        noteCreatedURI,
		SchemaRef:  "blob:abc",
		Compiled:   ct,
		Source:     "builtin",
		SourcePath: "types/builtin/note.created.ttd.json",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	summary, err := c.Summary(noteCreatedURI)
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary.Title != "Note created" {
		t.Fatalf("Summary().Title = %q, want Note created", summary.Title)
	}
	if summary.Source != "builtin" {
		t.Fatalf("Summary().Source = %q, want builtin", summary.Source)
	}
}

func TestCatalogExport(t *testing.T) {
	t.Parallel()
	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}
	raw := []byte(`{
	  "$id": "trove://type/note/created/1",
	  "title": "Note created",
	  "definition": { "properties": { "title": { "type": "string" } } }
	}`)
	td, err := types.ParseTypeDefinition(raw)
	if err != nil {
		t.Fatalf("ParseTypeDefinition() error = %v", err)
	}
	ctx := context.Background()
	schemaRef, err := types.StoreTypeDefinition(ctx, store, td)
	if err != nil {
		t.Fatalf("StoreTypeDefinition() error = %v", err)
	}
	ct, err := types.Compile(td)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	c := types.NewCatalog()
	if _, err := c.Register(types.Entry{
		URI:       noteCreatedURI,
		SchemaRef: schemaRef,
		Compiled:  ct,
		Source:    "builtin",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := c.Export(ctx, store, noteCreatedURI)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	want, err := types.CanonicalBytes(td)
	if err != nil {
		t.Fatalf("CanonicalBytes() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Export() = %q, want %q", got, want)
	}
}

func TestValidateTypeDefinition(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
	  "$id": "trove://type/note/created/1",
	  "definition": { "properties": { "title": { "type": "string" } } }
	}`)
	td, err := types.ValidateTypeDefinition(raw)
	if err != nil {
		t.Fatalf("ValidateTypeDefinition() error = %v", err)
	}
	if td.ID != noteCreatedURI {
		t.Fatalf("ValidateTypeDefinition().ID = %q, want %q", td.ID, noteCreatedURI)
	}

	_, err = types.ValidateTypeDefinition([]byte(`{"definition":{}}`))
	if err == nil {
		t.Fatal("ValidateTypeDefinition() missing $id error = nil, want error")
	}

	_, err = types.ValidateTypeDefinition([]byte(`{
	  "$id": "trove://type/bad/1",
	  "definition": { "properties": { "n": { "type": "not-a-type" } } }
	}`))
	if err == nil {
		t.Fatal("ValidateTypeDefinition() invalid JTD error = nil, want error")
	}
}

func TestCatalogExportMissingType(t *testing.T) {
	t.Parallel()
	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}
	c := types.NewCatalog()
	_, err = c.Export(context.Background(), store, noteCreatedURI)
	if err == nil {
		t.Fatal("Export() error = nil, want not registered error")
	}
}
