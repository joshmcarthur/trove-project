package modules

import (
	"fmt"
	"strings"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/types"
)

// EmitPolicy enforces module provides patterns and catalog-backed payload validation.
type EmitPolicy struct {
	patterns   []string
	catalog    *types.Catalog
	moduleName string
}

// NewEmitPolicy builds a policy from allowed type patterns and the global catalog.
func NewEmitPolicy(patterns []string, catalog *types.Catalog, moduleName string) (EmitPolicy, error) {
	if catalog == nil {
		return EmitPolicy{}, fmt.Errorf("modules: policy: catalog is required")
	}
	return EmitPolicy{
		patterns:   append([]string(nil), patterns...),
		catalog:    catalog,
		moduleName: moduleName,
	}, nil
}

// LoadIngestPolicy builds an emit policy from manifest provides and the type catalog.
func LoadIngestPolicy(m Manifest, catalog *types.Catalog, bundled bool) (EmitPolicy, error) {
	_ = bundled
	return NewEmitPolicy(m.Provides, catalog, m.Name)
}

// AllowsType reports whether the module may emit eventType.
func (p EmitPolicy) AllowsType(eventType string) bool {
	return types.MatchAnyPattern(p.patterns, eventType)
}

// ValidateEvent checks type allowlist and catalog payload validation, setting SchemaRef.
func (p EmitPolicy) ValidateEvent(event *journal.Event) error {
	if event == nil {
		return fmt.Errorf("modules: policy: event is nil")
	}
	ref, err := p.catalog.ValidateEmit(*event, p.patterns)
	if err != nil {
		if strings.Contains(err.Error(), "not allowed") {
			return fmt.Errorf("type %q not allowed for module %q", event.Type, p.moduleName)
		}
		return err
	}
	event.SchemaRef = ref
	return nil
}
