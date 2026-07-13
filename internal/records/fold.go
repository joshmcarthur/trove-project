package records

import "encoding/json"

// ApplyInput is the fold input for an apply operation (pure logic).
type ApplyInput struct {
	PreviousBody json.RawMessage
	Payload      json.RawMessage
	Transforms   json.RawMessage
}

// FoldApply merges payload, applies transforms, and returns the new body.
func FoldApply(in ApplyInput) (json.RawMessage, error) {
	merged, err := MergePatch(in.PreviousBody, in.Payload)
	if err != nil {
		return nil, err
	}
	return ApplyTransforms(merged, in.Transforms, DefaultMaxTransforms)
}
