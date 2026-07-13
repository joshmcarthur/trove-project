package modules

import (
	"fmt"
	"strings"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/types"
)

// WritePolicy enforces module provides patterns and catalog-backed payload validation.
type WritePolicy struct {
	patterns   []string
	catalog    *types.Catalog
	moduleName string
}

// NewWritePolicy builds a policy from allowed type patterns and the global catalog.
func NewWritePolicy(patterns []string, catalog *types.Catalog, moduleName string) (WritePolicy, error) {
	if catalog == nil {
		return WritePolicy{}, fmt.Errorf("modules: policy: catalog is required")
	}
	return WritePolicy{
		patterns:   append([]string(nil), patterns...),
		catalog:    catalog,
		moduleName: moduleName,
	}, nil
}

// LoadWritePolicy builds a write policy from manifest provides and the type catalog.
func LoadWritePolicy(m Manifest, catalog *types.Catalog, bundled bool) (WritePolicy, error) {
	_ = bundled
	return NewWritePolicy(m.Provides, catalog, m.Name)
}

// AllowsType reports whether the module may write eventType on apply.
func (p WritePolicy) AllowsType(eventType string) bool {
	return types.MatchAnyPattern(p.patterns, eventType)
}

// ValidateApply checks type allowlist and catalog payload validation when type is set.
func (p WritePolicy) ValidateApply(event *journal.Event) error {
	if event == nil {
		return fmt.Errorf("modules: policy: event is nil")
	}
	if event.Type == "" {
		return nil
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

// ValidateDelete performs delete-specific policy checks.
func (p WritePolicy) ValidateDelete(event *journal.Event) error {
	if event == nil {
		return fmt.Errorf("modules: policy: event is nil")
	}
	return nil
}

// EmitPolicy is an alias kept for transitional call sites.
type EmitPolicy = WritePolicy

// NewEmitPolicy builds a write policy.
func NewEmitPolicy(patterns []string, catalog *types.Catalog, moduleName string) (EmitPolicy, error) {
	return NewWritePolicy(patterns, catalog, moduleName)
}

// LoadIngestPolicy builds a write policy from manifest provides and the type catalog.
func LoadIngestPolicy(m Manifest, catalog *types.Catalog, bundled bool) (EmitPolicy, error) {
	return LoadWritePolicy(m, catalog, bundled)
}

// ValidateEvent validates an apply event. Prefer ValidateApply for new code.
func (p WritePolicy) ValidateEvent(event *journal.Event) error {
	return p.ValidateApply(event)
}
