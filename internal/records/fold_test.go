package records_test

import (
	"encoding/json"
	"testing"

	"github.com/joshmcarthur/trove/internal/records"
)

func TestMergePatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		base   string
		patch  string
		expect string
	}{
		{
			name:   "empty base",
			base:   `{}`,
			patch:  `{"a":1}`,
			expect: `{"a":1}`,
		},
		{
			name:   "merge nested",
			base:   `{"x":{"y":1}}`,
			patch:  `{"x":{"z":2}}`,
			expect: `{"x":{"y":1,"z":2}}`,
		},
		{
			name:   "null removes key",
			base:   `{"a":1,"b":2}`,
			patch:  `{"b":null}`,
			expect: `{"a":1}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := records.MergePatch(json.RawMessage(tc.base), json.RawMessage(tc.patch))
			if err != nil {
				t.Fatalf("MergePatch() error = %v", err)
			}
			if string(got) != tc.expect {
				t.Fatalf("MergePatch() = %s, want %s", got, tc.expect)
			}
		})
	}
}

func TestApplyTransforms(t *testing.T) {
	t.Parallel()

	body := `{"members":["a"]}`
	transforms := `[{"op":"add","path":"/members/-","value":"b"}]`
	got, err := records.ApplyTransforms(json.RawMessage(body), json.RawMessage(transforms), 32)
	if err != nil {
		t.Fatalf("ApplyTransforms() error = %v", err)
	}
	expect := `{"members":["a","b"]}`
	if string(got) != expect {
		t.Fatalf("ApplyTransforms() = %s, want %s", got, expect)
	}
}

func TestApplyTransformsRejectsTooManyOps(t *testing.T) {
	t.Parallel()

	transforms := `[{"op":"add","path":"/a","value":1},{"op":"add","path":"/b","value":2}]`
	_, err := records.ApplyTransforms(json.RawMessage(`{}`), json.RawMessage(transforms), 1)
	if err == nil {
		t.Fatal("expected error for op count over max")
	}
}

func TestFoldApplyOrder(t *testing.T) {
	t.Parallel()

	got, err := records.FoldApply(records.ApplyInput{
		PreviousBody: json.RawMessage(`{"title":"old","tags":[]}`),
		Payload:      json.RawMessage(`{"text":"hello"}`),
		Transforms:   json.RawMessage(`[{"op":"add","path":"/tags/-","value":"note"}]`),
	})
	if err != nil {
		t.Fatalf("FoldApply() error = %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["title"] != "old" || m["text"] != "hello" {
		t.Fatalf("merge failed: %#v", m)
	}
	tags, ok := m["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "note" {
		t.Fatalf("transform failed: %#v", m["tags"])
	}
}
