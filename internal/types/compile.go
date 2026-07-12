package types

import (
	"encoding/json"
	"fmt"

	jtd "github.com/jsontypedef/json-typedef-go"
)

const jtdMaxDepth = 32

// CompiledType holds a validated TTD ready for payload checks.
type CompiledType struct {
	ID         string
	Definition TypeDefinition
	Schema     jtd.Schema
}

// Compile parses the JTD definition inside a TTD envelope.
func Compile(td TypeDefinition) (*CompiledType, error) {
	var schema jtd.Schema
	if err := json.Unmarshal(td.Definition, &schema); err != nil {
		return nil, fmt.Errorf("types: compile %s: parse JTD: %w", td.ID, err)
	}
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("types: compile %s: JTD not well-formed: %w", td.ID, err)
	}
	return &CompiledType{ID: td.ID, Definition: td, Schema: schema}, nil
}

// ValidatePayload checks payload JSON against the compiled JTD schema.
func (c *CompiledType) ValidatePayload(payload json.RawMessage) error {
	var instance any
	if err := json.Unmarshal(payload, &instance); err != nil {
		return fmt.Errorf("types: payload for %s: invalid JSON: %w", c.ID, err)
	}
	errs, err := jtd.Validate(c.Schema, instance, jtd.WithMaxDepth(jtdMaxDepth))
	if err != nil {
		return fmt.Errorf("types: payload for %s: %w", c.ID, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("types: payload for %s: %v", c.ID, errs[0])
	}
	return nil
}
