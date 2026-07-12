package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TypeDefinition is the Trove Type Definition (TTD) envelope.
// The inner "definition" field is RFC 8927 JTD (validated separately).
type TypeDefinition struct {
	ID          string          `json:"$id"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Definition  json.RawMessage `json:"definition"`
	Supersedes  string          `json:"supersedes,omitempty"`
	Status      string          `json:"status,omitempty"`
}

// ParseTypeDefinition parses and validates a TTD envelope.
func ParseTypeDefinition(data []byte) (TypeDefinition, error) {
	var td TypeDefinition
	if err := json.Unmarshal(data, &td); err != nil {
		return TypeDefinition{}, fmt.Errorf("types: parse TTD: %w", err)
	}
	if strings.TrimSpace(td.ID) == "" {
		return TypeDefinition{}, fmt.Errorf("types: TTD: $id is required")
	}
	if len(td.Definition) == 0 || !json.Valid(td.Definition) {
		return TypeDefinition{}, fmt.Errorf("types: TTD %s: definition is required and must be JSON", td.ID)
	}
	if _, _, err := ParseTypeURI(td.ID); err != nil {
		return TypeDefinition{}, fmt.Errorf("types: TTD: $id: %w", err)
	}
	if td.Supersedes != "" {
		if _, _, err := ParseTypeURI(td.Supersedes); err != nil {
			return TypeDefinition{}, fmt.Errorf("types: TTD %s: supersedes: %w", td.ID, err)
		}
	}
	if td.Status != "" && td.Status != "active" && td.Status != "deprecated" {
		return TypeDefinition{}, fmt.Errorf("types: TTD %s: invalid status %q", td.ID, td.Status)
	}
	if td.Status == "" {
		td.Status = "active"
	}
	return td, nil
}
