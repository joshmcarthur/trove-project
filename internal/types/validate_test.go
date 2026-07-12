package types_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/types"
)

const (
	testNoteCreatedURI        = "trove://type/note/created/1"
	testHTTPIngestReceivedURI = "trove://type/http/ingest/received/1"
)

func registerNoteCreated(t *testing.T, c *types.Catalog) {
	t.Helper()
	td, err := types.ParseTypeDefinition([]byte(`{
	  "$id": "trove://type/note/created/1",
	  "definition": {
	    "properties": { "title": { "type": "string" } }
	  }
	}`))
	if err != nil {
		t.Fatalf("ParseTypeDefinition() error = %v", err)
	}
	ct, err := types.Compile(td)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if _, err := c.Register(types.Entry{
		URI:       testNoteCreatedURI,
		SchemaRef: "blob:note-created",
		Compiled:  ct,
		Source:    "test",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
}

func registerPermissiveType(t *testing.T, c *types.Catalog, uri string) {
	t.Helper()
	if err := c.RegisterPermissive(uri); err != nil {
		t.Fatalf("RegisterPermissive(%q) error = %v", uri, err)
	}
}

func TestValidateEmitSuccess(t *testing.T) {
	t.Parallel()

	c := types.NewCatalog()
	registerNoteCreated(t, c)

	ref, err := c.ValidateEmit(journal.Event{
		Type:    testNoteCreatedURI,
		Payload: json.RawMessage(`{"title":"ok"}`),
	}, []string{"trove://type/note/*"})
	if err != nil {
		t.Fatalf("ValidateEmit() error = %v", err)
	}
	if ref != "blob:note-created" {
		t.Fatalf("schemaRef = %q, want %q", ref, "blob:note-created")
	}
}

func TestValidateEmitUnknownType(t *testing.T) {
	t.Parallel()

	c := types.NewCatalog()
	registerNoteCreated(t, c)

	_, err := c.ValidateEmit(journal.Event{
		Type:    "trove://type/mqtt/foo/1",
		Payload: json.RawMessage(`{}`),
	}, []string{"trove://type/mqtt/*"})
	if err == nil || !strings.Contains(err.Error(), "not registered in catalog") {
		t.Fatalf("ValidateEmit() error = %v, want not registered in catalog", err)
	}
}

func TestValidateEmitNotAllowed(t *testing.T) {
	t.Parallel()

	c := types.NewCatalog()
	registerNoteCreated(t, c)

	_, err := c.ValidateEmit(journal.Event{
		Type:    testNoteCreatedURI,
		Payload: json.RawMessage(`{"title":"ok"}`),
	}, []string{"trove://type/mqtt/*"})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("ValidateEmit() error = %v, want not allowed", err)
	}
}

func TestValidateEmitPayloadFailure(t *testing.T) {
	t.Parallel()

	c := types.NewCatalog()
	registerNoteCreated(t, c)

	_, err := c.ValidateEmit(journal.Event{
		Type:    testNoteCreatedURI,
		Payload: json.RawMessage(`{}`),
	}, []string{"trove://type/note/*"})
	if err == nil {
		t.Fatal("ValidateEmit() error = nil, want payload validation error")
	}
}

func TestValidateEmitDottedPatterns(t *testing.T) {
	t.Parallel()

	const eventType = "http.ingest.received"
	c := types.NewCatalog()
	registerPermissiveType(t, c, eventType)

	ref, err := c.ValidateEmit(journal.Event{
		Type:    eventType,
		Payload: json.RawMessage(`{"any":"value"}`),
	}, []string{eventType})
	if err != nil {
		t.Fatalf("ValidateEmit() error = %v", err)
	}
	if ref == "" {
		t.Fatal("schemaRef is empty, want blob ref")
	}
}
