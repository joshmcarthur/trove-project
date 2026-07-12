package types_test

import (
	"bytes"
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestCanonicalBytesStable(t *testing.T) {
	t.Parallel()
	a := []byte(`{"$id":"trove://type/note/created/1","definition":{"properties":{"title":{"type":"string"}}}}`)
	b := []byte(`{
	  "$id": "trove://type/note/created/1",
	  "definition": { "properties": { "title": { "type": "string" } } }
	}`)
	tdA, err := types.ParseTypeDefinition(a)
	if err != nil {
		t.Fatal(err)
	}
	tdB, err := types.ParseTypeDefinition(b)
	if err != nil {
		t.Fatal(err)
	}
	canA, err := types.CanonicalBytes(tdA)
	if err != nil {
		t.Fatal(err)
	}
	canB, err := types.CanonicalBytes(tdB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canA, canB) {
		t.Fatalf("canonical bytes mismatch: %s vs %s", canA, canB)
	}
}
