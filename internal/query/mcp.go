package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Querier is the journal query API used by MCP handlers.
type Querier interface {
	GetEvent(ctx context.Context, id string) (Event, error)
	SummarizeRange(ctx context.Context, timeFrom, timeTo time.Time) (Summary, error)
	GetRecord(ctx context.Context, recordRef string, version int) (Record, error)
	SearchRecords(ctx context.Context, query string, params RecordSearchParams) ([]Record, error)
	ListIncompleteRecords(ctx context.Context, source string, limit int) ([]Record, error)
}

// MCPDeps bundles dependencies for the MCP HTTP handler.
type MCPDeps struct {
	Querier Querier
	Tools   []trovemodule.MCPToolDescriptor
	Caller  trovemodule.MCPToolCaller
}

// MCPHandler returns the MCP streamable HTTP handler.
func MCPHandler(deps MCPDeps) http.Handler {
	return newMCPHandler(deps)
}

func newMCPHandler(deps MCPDeps) http.Handler {
	server := newMCPServer(deps)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
}

func newMCPServer(deps MCPDeps) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "trove",
		Version: "0.1.0",
	}, nil)

	if deps.Querier != nil {
		registerQueryTools(server, deps.Querier)
	}
	if deps.Caller != nil {
		registerModuleTools(server, deps.Tools, deps.Caller)
	}

	return server
}

func registerQueryTools(server *mcp.Server, q Querier) {
	type getEventParams struct {
		ID string `json:"id" jsonschema:"ULID of the event to retrieve"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Description: "Return a journal event by ULID for audit",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params getEventParams) (*mcp.CallToolResult, any, error) {
		event, err := q.GetEvent(ctx, params.ID)
		if err != nil {
			return nil, nil, err
		}
		return textToolResult(event)
	})

	type getRecordParams struct {
		RecordRef string `json:"record_ref" jsonschema:"required,Record reference to retrieve"`
		Version   int    `json:"version,omitempty" jsonschema:"Optional version; omit for latest"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_record",
		Description: "Return a folded record by record_ref",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params getRecordParams) (*mcp.CallToolResult, any, error) {
		record, err := q.GetRecord(ctx, params.RecordRef, params.Version)
		if err != nil {
			return nil, nil, err
		}
		return textToolResult(record)
	})

	type searchRecordsParams struct {
		Query          string `json:"query" jsonschema:"required,Keyword search query"`
		TypePrefix     string `json:"type_prefix,omitempty" jsonschema:"Optional record type prefix filter"`
		Source         string `json:"source,omitempty" jsonschema:"Optional source filter"`
		TimeFrom       string `json:"time_from,omitempty" jsonschema:"Optional RFC3339 start of updated_at range"`
		TimeTo         string `json:"time_to,omitempty" jsonschema:"Optional RFC3339 end of updated_at range"`
		IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"Include deleted records in search results"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_records",
		Description: "Search folded records by keyword using FTS5",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params searchRecordsParams) (*mcp.CallToolResult, any, error) {
		searchParams, err := recordSearchParamsFromMCP(params.TypePrefix, params.Source, params.TimeFrom, params.TimeTo, params.IncludeDeleted)
		if err != nil {
			return nil, nil, err
		}
		records, err := q.SearchRecords(ctx, params.Query, searchParams)
		if err != nil {
			return nil, nil, err
		}
		return textToolResult(records)
	})

	type listIncompleteRecordsParams struct {
		Source string `json:"source,omitempty" jsonschema:"Optional source filter"`
		Limit  int    `json:"limit,omitempty" jsonschema:"Maximum records to return"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_incomplete_records",
		Description: "List records with completeness incomplete",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params listIncompleteRecordsParams) (*mcp.CallToolResult, any, error) {
		records, err := q.ListIncompleteRecords(ctx, params.Source, params.Limit)
		if err != nil {
			return nil, nil, err
		}
		return textToolResult(records)
	})

	type summarizeRangeParams struct {
		TimeFrom string `json:"time_from" jsonschema:"required,RFC3339 start of time range"`
		TimeTo   string `json:"time_to" jsonschema:"required,RFC3339 end of time range"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "summarize_range",
		Description: "Return aggregated event counts by type and notable events for a time window",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params summarizeRangeParams) (*mcp.CallToolResult, any, error) {
		timeFrom, timeTo, err := parseRequiredTimeRange(params.TimeFrom, params.TimeTo)
		if err != nil {
			return nil, nil, err
		}
		summary, err := q.SummarizeRange(ctx, timeFrom, timeTo)
		if err != nil {
			return nil, nil, err
		}
		return textToolResult(summary)
	})
}

func registerModuleTools(server *mcp.Server, tools []trovemodule.MCPToolDescriptor, caller trovemodule.MCPToolCaller) {
	for _, tool := range tools {
		tool := tool
		mcp.AddTool(server, &mcp.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		}, func(ctx context.Context, req *mcp.CallToolRequest, params map[string]any) (*mcp.CallToolResult, any, error) {
			_ = req
			args, err := json.Marshal(params)
			if err != nil {
				return nil, nil, err
			}
			result, err := caller.CallMCPTool(ctx, tool.Name, args)
			if err != nil {
				return nil, nil, err
			}
			if len(result) == 0 {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "{}"}},
				}, nil, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(result)}},
			}, nil, nil
		})
	}
}

func textToolResult(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		},
	}, nil, nil
}

func recordSearchParamsFromMCP(typePrefix, source, timeFrom, timeTo string, includeDeleted bool) (RecordSearchParams, error) {
	from, err := parseOptionalRFC3339(timeFrom)
	if err != nil {
		return RecordSearchParams{}, err
	}
	to, err := parseOptionalRFC3339(timeTo)
	if err != nil {
		return RecordSearchParams{}, err
	}
	return RecordSearchParams{
		TypePrefix:     typePrefix,
		Source:         source,
		TimeFrom:       from,
		TimeTo:         to,
		IncludeDeleted: includeDeleted,
	}, nil
}

func parseOptionalRFC3339(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("query: parse time %q: %w", s, err)
	}
	return &t, nil
}

func parseRequiredRFC3339(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("query: time is required")
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("query: parse time %q: %w", s, err)
	}
	return t, nil
}

func parseRequiredTimeRange(timeFrom, timeTo string) (time.Time, time.Time, error) {
	from, err := parseRequiredRFC3339(timeFrom)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseRequiredRFC3339(timeTo)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return from, to, nil
}
