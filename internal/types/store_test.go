package types_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/types"
)

func TestStoreTypeDefinitionPutsBlob(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	raw := []byte(`{
	  "$id": "trove://type/note/created/1",
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
	if !strings.HasPrefix(schemaRef, "sha256-") {
		t.Fatalf("schemaRef = %q, want sha256- prefix", schemaRef)
	}

	rc, err := store.Get(ctx, schemaRef)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	want, err := types.CanonicalBytes(td)
	if err != nil {
		t.Fatalf("CanonicalBytes() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stored bytes = %q, want %q", got, want)
	}
}
