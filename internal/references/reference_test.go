package references_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/references"
	"github.com/oklog/ulid"
)

func TestValidateRef(t *testing.T) {
	t.Parallel()

	valid := []string{
		"trove://record/01JREC00000000000000000001",
		"trove://revision/01JREV00000000000000000001",
		"trove://blob/sha256-" + strings.Repeat("ab", 32),
		"https://example.com/article",
		"mailto:user@example.com",
	}
	for _, ref := range valid {
		if err := references.ValidateRef(ref); err != nil {
			t.Errorf("ValidateRef(%q) error = %v, want nil", ref, err)
		}
	}

	invalid := []string{
		"",
		"trove://type/note/quick/1",
		"trove://record/not-a-ulid",
		"trove://blob/sha256-UPPER",
		"relative/path",
	}
	for _, ref := range invalid {
		if err := references.ValidateRef(ref); err == nil {
			t.Errorf("ValidateRef(%q) = nil, want error", ref)
		}
	}
}

func TestUnionAndSubtract(t *testing.T) {
	t.Parallel()

	base := []references.Reference{
		{Ref: "trove://record/01JREC00000000000000000001", Rel: "mentions"},
		{Ref: "https://example.com", Rel: "source"},
	}

	add := []references.Reference{
		{Ref: "https://example.com", Rel: "source"},
		{Ref: "trove://blob/sha256-" + strings.Repeat("cd", 32), Rel: "cover"},
	}
	merged := references.Union(base, add)
	if len(merged) != 3 {
		t.Fatalf("Union() len = %d, want 3", len(merged))
	}

	removed := references.Subtract(merged, []references.Reference{
		{Ref: "https://example.com"},
	})
	if len(removed) != 2 {
		t.Fatalf("Subtract(all rels) len = %d, want 2", len(removed))
	}

	removed = references.Subtract(merged, []references.Reference{
		{Ref: "https://example.com", Rel: "source"},
	})
	if len(removed) != 2 {
		t.Fatalf("Subtract(exact rel) len = %d, want 2", len(removed))
	}
}

func TestParseListDedupes(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal([]references.Reference{
		{Ref: "https://example.com"},
		{Ref: "https://example.com"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	refs, err := references.ParseList(data)
	if err != nil {
		t.Fatalf("ParseList() error = %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("ParseList() len = %d, want 1", len(refs))
	}
}

func TestValidateRecordULID(t *testing.T) {
	t.Parallel()

	id := ulid.MustNew(ulid.Now(), nil).String()
	if err := references.ValidateRef("trove://record/" + id); err != nil {
		t.Fatalf("ValidateRef() error = %v", err)
	}
}
