package records

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// DefaultMaxTransforms is the default cap on transforms per apply.
const DefaultMaxTransforms = 32

// ApplyTransforms applies RFC 6902 JSON Patch operations to body.
// Body is the patch document root — paths are relative to the record body object.
func ApplyTransforms(body json.RawMessage, transforms json.RawMessage, maxOps int) (json.RawMessage, error) {
	if len(transforms) == 0 || string(transforms) == "null" {
		return body, nil
	}
	if !json.Valid(transforms) {
		return nil, fmt.Errorf("records: transforms: invalid JSON")
	}

	patch, err := jsonpatch.DecodePatch(transforms)
	if err != nil {
		return nil, fmt.Errorf("records: transforms: %w", err)
	}

	if maxOps <= 0 {
		maxOps = DefaultMaxTransforms
	}
	if len(patch) > maxOps {
		return nil, fmt.Errorf("records: transforms: exceeds max %d operations", maxOps)
	}

	if len(body) == 0 || string(body) == "null" {
		body = json.RawMessage(`{}`)
	}

	out, err := patch.Apply(body)
	if err != nil {
		return nil, fmt.Errorf("records: transforms: %w", err)
	}
	return out, nil
}
