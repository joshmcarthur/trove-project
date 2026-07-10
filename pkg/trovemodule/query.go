package trovemodule

import (
	"context"
	"fmt"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Querier reads journal data via the Trove core CoreServices RPC.
type Querier interface {
	GetEvent(ctx context.Context, id string) (*troverpc.Event, error)
	SearchEvents(ctx context.Context, req *troverpc.SearchEventsRequest) ([]*troverpc.Event, error)
	GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error)
	SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error)
}

// QueryRunner is implemented by modules that use core query services.
type QueryRunner interface {
	RunWithQuery(ctx context.Context, q Querier) error
}

type coreQuerier struct {
	client troverpc.CoreServicesClient
}

func newCoreQuerier(client troverpc.CoreServicesClient) Querier {
	return &coreQuerier{client: client}
}

func (q *coreQuerier) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	return q.client.GetEvent(ctx, &troverpc.GetEventRequest{Id: id})
}

func (q *coreQuerier) SearchEvents(ctx context.Context, req *troverpc.SearchEventsRequest) ([]*troverpc.Event, error) {
	resp, err := q.client.SearchEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (q *coreQuerier) GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error) {
	resp, err := q.client.GetEventsByType(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (q *coreQuerier) SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return q.client.SummarizeRange(ctx, req)
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
