package types_test

import (
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestFormatParseTypeURI(t *testing.T) {
	t.Parallel()
	uri := types.FormatTypeURI("note/created", 1)
	if uri != "trove://type/note/created/1" {
		t.Fatalf("FormatTypeURI() = %q", uri)
	}
	path, ver, err := types.ParseTypeURI(uri)
	if err != nil {
		t.Fatalf("ParseTypeURI() error = %v", err)
	}
	if path != "note/created" || ver != 1 {
		t.Fatalf("ParseTypeURI() = %q %d, want note/created 1", path, ver)
	}
}

func TestParseTypeURIRejectsBadVersion(t *testing.T) {
	t.Parallel()
	_, _, err := types.ParseTypeURI("trove://type/note/created/v2")
	if err == nil {
		t.Fatal("ParseTypeURI() error = nil, want error for non-numeric version")
	}
}

func TestNameToPath(t *testing.T) {
	t.Parallel()
	if got := types.NameToPath("note.created"); got != "note/created" {
		t.Fatalf("NameToPath() = %q, want note/created", got)
	}
}
