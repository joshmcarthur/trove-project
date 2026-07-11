package mcpquery

import (
	"context"
	_ "embed"
	"net/http"
	"sync/atomic"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/modules"
	"github.com/joshmcarthur/trove/internal/query"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

//go:embed manifest.toml
var manifestBytes []byte

// Module serves MCP over the HTTP gateway.
type Module struct {
	ready   atomic.Bool
	handler http.Handler
}

// New constructs an mcp-query module instance.
func New() trovemodule.Module {
	return &Module{}
}

func (m *Module) Run(ctx context.Context, core trovemodule.Core) error {
	tools, err := core.ListMCPTools(ctx)
	if err != nil {
		return err
	}
	m.handler = query.MCPHandler(query.MCPDeps{
		Querier: &queryAdapter{q: core},
		Tools:   tools,
		Caller:  core,
	})
	m.ready.Store(true)
	defer m.ready.Store(false)
	<-ctx.Done()
	return nil
}

func (m *Module) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if !m.ready.Load() || m.handler == nil {
		return &troverpc.HTTPResponse{
			Status: http.StatusServiceUnavailable,
			Body:   []byte("service unavailable"),
		}, nil
	}
	if trovemodule.RouteKey(req.GetMethod(), req.GetMatchedPattern()) != "POST /mcp" {
		return &troverpc.HTTPResponse{
			Status: http.StatusNotFound,
			Body:   []byte("not found"),
		}, nil
	}
	return trovemodule.ServeHTTPViaRPC(m.handler, req), nil
}

func (m *Module) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "mcp handler ready"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "mcp handler not ready"}, nil
}

// Manifest returns the embedded module manifest.
func Manifest() (modules.Manifest, error) {
	return modules.ParseManifest(manifestBytes)
}

type queryAdapter struct {
	q trovemodule.Querier
}

func (a *queryAdapter) GetEvent(ctx context.Context, id string) (query.Event, error) {
	event, err := a.q.GetEvent(ctx, id)
	if err != nil {
		return query.Event{}, err
	}
	return protoToQueryEvent(event)
}

func (a *queryAdapter) SearchEvents(ctx context.Context, text string, params query.SearchParams) ([]query.Event, error) {
	events, err := a.q.SearchEvents(ctx, &troverpc.SearchEventsRequest{
		Query:      text,
		TypePrefix: params.TypePrefix,
		Source:     params.Source,
		TimeFrom:   formatOptionalTime(params.TimeFrom),
		TimeTo:     formatOptionalTime(params.TimeTo),
	})
	if err != nil {
		return nil, err
	}
	return protoToQueryEvents(events)
}

func (a *queryAdapter) GetEventsByType(ctx context.Context, eventType string, timeFrom, timeTo *time.Time) ([]query.Event, error) {
	events, err := a.q.GetEventsByType(ctx, &troverpc.GetEventsByTypeRequest{
		Type:     eventType,
		TimeFrom: formatOptionalTime(timeFrom),
		TimeTo:   formatOptionalTime(timeTo),
	})
	if err != nil {
		return nil, err
	}
	return protoToQueryEvents(events)
}

func (a *queryAdapter) SummarizeRange(ctx context.Context, timeFrom, timeTo time.Time) (query.Summary, error) {
	summary, err := a.q.SummarizeRange(ctx, &troverpc.SummarizeRangeRequest{
		TimeFrom: timeFrom.Format(time.RFC3339),
		TimeTo:   timeTo.Format(time.RFC3339),
	})
	if err != nil {
		return query.Summary{}, err
	}
	return protoToQuerySummary(summary)
}

func protoToQueryEvent(event *troverpc.Event) (query.Event, error) {
	if event == nil {
		return query.Event{}, query.ErrNotFound
	}
	t, err := time.Parse(time.RFC3339, event.Time)
	if err != nil {
		return query.Event{}, err
	}
	out := query.Event{
		ID:      event.Id,
		Time:    t,
		Type:    event.Type,
		Source:  event.Source,
		Payload: event.Payload,
	}
	if event.BlobRef != "" {
		ref := event.BlobRef
		out.BlobRef = &ref
	}
	return out, nil
}

func protoToQueryEvents(events []*troverpc.Event) ([]query.Event, error) {
	out := make([]query.Event, 0, len(events))
	for _, event := range events {
		converted, err := protoToQueryEvent(event)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func protoToQuerySummary(summary *troverpc.Summary) (query.Summary, error) {
	if summary == nil {
		return query.Summary{}, nil
	}
	timeFrom, err := time.Parse(time.RFC3339, summary.TimeFrom)
	if err != nil {
		return query.Summary{}, err
	}
	timeTo, err := time.Parse(time.RFC3339, summary.TimeTo)
	if err != nil {
		return query.Summary{}, err
	}
	byType := make(map[string]int, len(summary.ByType))
	for k, v := range summary.ByType {
		byType[k] = int(v)
	}
	notable, err := protoToQueryEvents(summary.Notable)
	if err != nil {
		return query.Summary{}, err
	}
	return query.Summary{
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
		Total:    int(summary.Total),
		ByType:   byType,
		Notable:  notable,
	}, nil
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
