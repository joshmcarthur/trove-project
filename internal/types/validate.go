package types

import (
	"fmt"

	"github.com/joshmcarthur/trove/internal/journal"
)

// ValidateEmit checks allowed patterns, catalog registration, and payload shape.
// It returns the catalog schema_ref for the event type on success.
func (c *Catalog) ValidateEmit(event journal.Event, allowedPatterns []string) (schemaRef string, err error) {
	if c == nil {
		return "", fmt.Errorf("types: catalog is required")
	}
	if !MatchAnyPattern(allowedPatterns, event.Type) {
		return "", fmt.Errorf("type %q not allowed", event.Type)
	}
	entry, ok := c.Lookup(event.Type)
	if !ok {
		return "", fmt.Errorf("type %q is not registered in catalog", event.Type)
	}
	if entry.Compiled == nil {
		return "", fmt.Errorf("type %q has no compiled schema", event.Type)
	}
	if err := entry.Compiled.ValidatePayload(event.Payload); err != nil {
		return "", err
	}
	return entry.SchemaRef, nil
}
