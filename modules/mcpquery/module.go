package mcpquery

import (
	"context"
	_ "embed"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/joshmcarthur/trove/internal/modules"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
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

func (a *queryAdapter) GetRecord(ctx context.Context, recordRef string, version int) (query.Record, error) {
	record, err := a.q.GetRecord(ctx, &troverpc.GetRecordRequest{
		RecordRef: recordRef,
		Version:   int32(version), //nolint:gosec // G115: MCP tool version filter
	})
	if err != nil {
		return query.Record{}, err
	}
	return protoToQueryRecord(record)
}

func (a *queryAdapter) SearchRecords(ctx context.Context, text string, params query.RecordSearchParams) ([]query.Record, error) {
	records, err := a.q.SearchRecords(ctx, &troverpc.SearchRecordsRequest{
		Query:          text,
		TypePrefix:     params.TypePrefix,
		Source:         params.Source,
		TimeFrom:       formatOptionalTime(params.TimeFrom),
		TimeTo:         formatOptionalTime(params.TimeTo),
		IncludeDeleted: params.IncludeDeleted,
	})
	if err != nil {
		return nil, err
	}
	return protoToQueryRecords(records)
}

func (a *queryAdapter) ListIncompleteRecords(ctx context.Context, source string, limit int) ([]query.Record, error) {
	records, err := a.q.ListIncompleteRecords(ctx, &troverpc.ListIncompleteRecordsRequest{
		Source: source,
		Limit:  int32(limit), //nolint:gosec // G115: MCP tool limit
	})
	if err != nil {
		return nil, err
	}
	return protoToQueryRecords(records)
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
		ID:         event.Id,
		Time:       t,
		Operation:  event.Operation,
		RecordRef:  event.RecordRef,
		Type:       event.Type,
		Source:     event.Source,
		Payload:    event.Payload,
		Transforms: event.Transforms,
	}
	if event.BlobRef != "" {
		ref := event.BlobRef
		out.BlobRef = &ref
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
	notable := make([]query.Event, 0, len(summary.Notable))
	for _, event := range summary.Notable {
		converted, err := protoToQueryEvent(event)
		if err != nil {
			return query.Summary{}, err
		}
		notable = append(notable, converted)
	}
	return query.Summary{
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
		Total:    int(summary.Total),
		ByType:   byType,
		Notable:  notable,
	}, nil
}

func protoToQueryRecord(record *troverpc.Record) (query.Record, error) {
	if record == nil {
		return query.Record{}, query.ErrRecordNotFound
	}
	updatedAt, err := time.Parse(time.RFC3339, record.UpdatedAt)
	if err != nil {
		return query.Record{}, err
	}
	out := query.Record{
		RecordRef:    record.RecordRef,
		Version:      int(record.Version),
		Completeness: record.Completeness,
		Type:         record.Type,
		Source:       record.Source,
		Body:         record.Body,
		UpdatedAt:    updatedAt,
	}
	if record.ContentRef != "" {
		ref := record.ContentRef
		out.ContentRef = &ref
	}
	return out, nil
}

func protoToQueryRecords(records []*troverpc.Record) ([]query.Record, error) {
	out := make([]query.Record, 0, len(records))
	for _, record := range records {
		converted, err := protoToQueryRecord(record)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
