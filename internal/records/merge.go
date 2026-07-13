package records

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// MergePatch applies RFC 7396 merge-patch semantics: patch keys merge into base;
// null in patch removes keys from base.
func MergePatch(base json.RawMessage, patch json.RawMessage) (json.RawMessage, error) {
	if len(patch) == 0 || string(patch) == "null" {
		if len(base) == 0 || string(base) == "null" {
			return json.RawMessage(`{}`), nil
		}
		return base, nil
	}
	if !json.Valid(patch) {
		return nil, fmt.Errorf("records: merge patch: invalid JSON")
	}

	if len(base) == 0 || string(base) == "null" {
		base = json.RawMessage(`{}`)
	} else if !json.Valid(base) {
		return nil, fmt.Errorf("records: merge patch: invalid base JSON")
	}

	merged, err := jsonpatch.MergePatch(base, patch)
	if err != nil {
		return nil, fmt.Errorf("records: merge patch: %w", err)
	}
	return merged, nil
}
