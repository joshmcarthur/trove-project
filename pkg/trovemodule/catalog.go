package trovemodule

import (
	"context"
	"encoding/json"
)

// TypeSummary describes a registered type in the catalog.
type TypeSummary struct {
	URI         string
	Title       string
	Description string
	Source      string
	SourcePath  string
	SchemaRef   string
	Status      string
}

// TypeCatalogReader reads from the host type catalog.
type TypeCatalogReader interface {
	ListTypes(ctx context.Context, sourceFilter string) ([]TypeSummary, error)
	GetType(ctx context.Context, uri string) (TypeSummary, json.RawMessage, error)
	ExportType(ctx context.Context, uri string) ([]byte, string, error)
	ValidateTypeDefinition(ctx context.Context, ttdJSON []byte) (valid bool, uri string, errMsg string, err error)
}
