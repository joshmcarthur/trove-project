package types_test

import (
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
	ha, err := types.CanonicalHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := types.CanonicalHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hash mismatch: %s vs %s", ha, hb)
	}
}
