package types

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/joshmcarthur/trove/internal/blob"
)

// TypeSummary is catalog metadata for a registered type.
type TypeSummary struct {
	URI         string
	Title       string
	Description string
	Source      string
	SourcePath  string
	SchemaRef   string
	Status      string
}

// List returns all catalog entries sorted by URI.
func (c *Catalog) List() []Entry {
	if c == nil || len(c.entries) == 0 {
		return nil
	}
	out := make([]Entry, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].URI < out[j].URI
	})
	return out
}

// Summary returns metadata for a registered type URI.
func (c *Catalog) Summary(uri string) (TypeSummary, error) {
	if c == nil {
		return TypeSummary{}, fmt.Errorf("types: catalog is required")
	}
	entry, ok := c.Lookup(uri)
	if !ok {
		return TypeSummary{}, fmt.Errorf("types: type %q is not registered in catalog", uri)
	}
	return entrySummary(entry), nil
}

// Export returns canonical TTD bytes for a registered type from the blob store.
func (c *Catalog) Export(ctx context.Context, blobs blob.Store, uri string) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("types: catalog is required")
	}
	if blobs == nil {
		return nil, fmt.Errorf("types: blob store is required")
	}
	entry, ok := c.Lookup(uri)
	if !ok {
		return nil, fmt.Errorf("types: type %q is not registered in catalog", uri)
	}
	rc, err := blobs.Get(ctx, entry.SchemaRef)
	if err != nil {
		return nil, fmt.Errorf("types: export %s: %w", uri, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("types: export %s: read blob: %w", uri, err)
	}
	return data, nil
}

// ValidateTypeDefinition parses and compiles a TTD without registering it.
func ValidateTypeDefinition(data []byte) (TypeDefinition, error) {
	td, err := ParseTypeDefinition(data)
	if err != nil {
		return TypeDefinition{}, err
	}
	if _, err := Compile(td); err != nil {
		return TypeDefinition{}, err
	}
	return td, nil
}

// SummaryFromEntry returns catalog metadata from a catalog entry.
func SummaryFromEntry(entry Entry) TypeSummary {
	return entrySummary(entry)
}

func entrySummary(entry Entry) TypeSummary {
	summary := TypeSummary{
		URI:        entry.URI,
		Source:     entry.Source,
		SourcePath: entry.SourcePath,
		SchemaRef:  entry.SchemaRef,
	}
	if entry.Compiled != nil {
		summary.Title = entry.Compiled.Definition.Title
		summary.Description = entry.Compiled.Definition.Description
		summary.Status = entry.Compiled.Definition.Status
		if summary.Status == "" {
			summary.Status = "active"
		}
	}
	return summary
}
