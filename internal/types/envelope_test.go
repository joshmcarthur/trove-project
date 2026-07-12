package types_test

import (
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

const validTTD = `{
  "$id": "trove://type/note/created/1",
  "title": "Note created",
  "definition": {
    "properties": {
      "title": { "type": "string" }
    }
  }
}`

func TestParseTypeDefinition(t *testing.T) {
	t.Parallel()
	td, err := types.ParseTypeDefinition([]byte(validTTD))
	if err != nil {
		t.Fatalf("ParseTypeDefinition() error = %v", err)
	}
	if td.ID != "trove://type/note/created/1" {
		t.Fatalf("ID = %q", td.ID)
	}
}

func TestParseTypeDefinitionRequiresID(t *testing.T) {
	t.Parallel()
	_, err := types.ParseTypeDefinition([]byte(`{"definition":{"properties":{}}}`))
	if err == nil {
		t.Fatal("ParseTypeDefinition() error = nil, want missing $id error")
	}
}

func TestParseTypeDefinitionIDMustMatchURI(t *testing.T) {
	t.Parallel()
	raw := `{
	  "$id": "http://example.com/not-a-trove-uri",
	  "definition": { "properties": { "title": { "type": "string" } } }
	}`
	_, err := types.ParseTypeDefinition([]byte(raw))
	if err == nil {
		t.Fatal("ParseTypeDefinition() error = nil, want invalid $id URI error")
	}
}
