package types_test

import (
	"encoding/json"
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestCompileAndValidatePayload(t *testing.T) {
	t.Parallel()
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
	if err := ct.ValidatePayload(json.RawMessage(`{"title":"ok"}`)); err != nil {
		t.Fatalf("ValidatePayload() valid error = %v", err)
	}
	if err := ct.ValidatePayload(json.RawMessage(`{}`)); err == nil {
		t.Fatal("ValidatePayload() missing title: error = nil, want validation error")
	}
}
