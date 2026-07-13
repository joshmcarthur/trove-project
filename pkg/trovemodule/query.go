package trovemodule

import (
	"context"
	"fmt"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RevisionQuerier reads journal revisions via Core query RPCs.
type RevisionQuerier interface {
	GetRevision(ctx context.Context, id string) (*troverpc.Revision, error)
	SearchRevisions(ctx context.Context, req *troverpc.SearchRevisionsRequest) ([]*troverpc.Revision, error)
	GetRevisionsByType(ctx context.Context, req *troverpc.GetRevisionsByTypeRequest) ([]*troverpc.Revision, error)
	SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error)
}

// ParseRFC3339Optional parses an optional RFC3339 timestamp string.
func ParseRFC3339Optional(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("parse time %q: %w", s, err)
	}
	return &t, nil
}
