package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/joshmcarthur/trove/internal/journal"
)

// IngestPolicy enforces module provides patterns and optional payload schemas.
type IngestPolicy struct {
	patterns   []string
	schemaKeys []string
	schemas    map[string]*jsonschema.Resolved
	moduleName string
}

// LoadIngestPolicy builds a policy from manifest and module directory.
func LoadIngestPolicy(m Manifest, dir string, bundled bool) (IngestPolicy, error) {
	if bundled && dir == "" {
		return IngestPolicy{
			patterns:   append([]string(nil), m.Provides...),
			schemaKeys: schemaKeys(m.Schemas),
			schemas:    make(map[string]*jsonschema.Resolved),
			moduleName: m.Name,
		}, nil
	}

	policy := IngestPolicy{
		patterns:   append([]string(nil), m.Provides...),
		schemaKeys: schemaKeys(m.Schemas),
		schemas:    make(map[string]*jsonschema.Resolved, len(m.Schemas)),
		moduleName: m.Name,
	}

	for pattern, relPath := range m.Schemas {
		path := filepath.Join(dir, relPath)
		data, err := os.ReadFile(path)
		if err != nil {
			return IngestPolicy{}, fmt.Errorf("modules: policy: read schema %q for pattern %q: %w", relPath, pattern, err)
		}

		var schema jsonschema.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return IngestPolicy{}, fmt.Errorf("modules: policy: parse schema %q for pattern %q: %w", relPath, pattern, err)
		}

		resolved, err := schema.Resolve(nil)
		if err != nil {
			return IngestPolicy{}, fmt.Errorf("modules: policy: resolve schema %q for pattern %q: %w", relPath, pattern, err)
		}
		policy.schemas[pattern] = resolved
	}

	return policy, nil
}

func schemaKeys(schemas map[string]string) []string {
	if len(schemas) == 0 {
		return nil
	}
	keys := make([]string, 0, len(schemas))
	for key := range schemas {
		keys = append(keys, key)
	}
	return keys
}

// AllowsType reports whether the module may emit eventType.
func (p IngestPolicy) AllowsType(eventType string) bool {
	return MatchType(p.patterns, eventType)
}

// ValidateEvent checks type allowlist and optional payload schema.
func (p IngestPolicy) ValidateEvent(event journal.Event) error {
	if !p.AllowsType(event.Type) {
		return fmt.Errorf("type %q not allowed for module %q", event.Type, p.moduleName)
	}

	pattern, ok := ResolveSchemaPattern(p.schemaKeys, event.Type)
	if !ok {
		return nil
	}

	resolved := p.schemas[pattern]
	if resolved == nil {
		return nil
	}

	var instance any
	if err := json.Unmarshal(event.Payload, &instance); err != nil {
		return fmt.Errorf("payload does not match schema for type %q: invalid JSON: %w", event.Type, err)
	}
	if err := resolved.Validate(instance); err != nil {
		return fmt.Errorf("payload does not match schema for type %q: %w", event.Type, err)
	}
	return nil
}
