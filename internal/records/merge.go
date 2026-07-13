package records

import (
	"encoding/json"
	"fmt"
)

// MergePatch applies RFC 7396 merge-patch semantics: patch keys merge into base;
// null in patch removes keys from base.
func MergePatch(base json.RawMessage, patch json.RawMessage) (json.RawMessage, error) {
	if len(patch) == 0 || string(patch) == "null" {
		if len(base) == 0 {
			return json.RawMessage(`{}`), nil
		}
		return base, nil
	}
	if !json.Valid(patch) {
		return nil, fmt.Errorf("records: merge patch: invalid JSON")
	}

	var baseObj map[string]any
	if len(base) == 0 || string(base) == "null" {
		baseObj = map[string]any{}
	} else {
		if !json.Valid(base) {
			return nil, fmt.Errorf("records: merge patch: invalid base JSON")
		}
		if err := json.Unmarshal(base, &baseObj); err != nil {
			return nil, fmt.Errorf("records: merge patch: base must be object: %w", err)
		}
		if baseObj == nil {
			baseObj = map[string]any{}
		}
	}

	var patchObj map[string]any
	if err := json.Unmarshal(patch, &patchObj); err != nil {
		return nil, fmt.Errorf("records: merge patch: patch must be object: %w", err)
	}

	merged := mergeObjects(baseObj, patchObj)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("records: merge patch: marshal: %w", err)
	}
	return out, nil
}

func mergeObjects(base, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		if v == nil {
			delete(out, k)
			continue
		}
		existing, ok := out[k]
		if !ok {
			out[k] = v
			continue
		}
		existingMap, existingOK := existing.(map[string]any)
		patchMap, patchOK := v.(map[string]any)
		if existingOK && patchOK {
			out[k] = mergeObjects(existingMap, patchMap)
			continue
		}
		out[k] = v
	}
	return out
}
