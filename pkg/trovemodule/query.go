package trovemodule

import (
	"context"
	"fmt"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Querier reads journal and record data via Core query RPCs.
type Querier interface {
	GetEvent(ctx context.Context, id string) (*troverpc.Event, error)
	SearchEvents(ctx context.Context, req *troverpc.SearchEventsRequest) ([]*troverpc.Event, error)
	GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error)
	SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error)
	GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error)
	SearchRecords(ctx context.Context, req *troverpc.SearchRecordsRequest) ([]*troverpc.Record, error)
	ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error)
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
