package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPHandler returns the MCP streamable HTTP handler for svc.
func MCPHandler(svc *Service) http.Handler {
	return newMCPHandler(svc)
}

// Serve starts the MCP query server on listen until ctx is cancelled.
func Serve(ctx context.Context, listen string, svc *Service) error {
	if svc == nil {
		return fmt.Errorf("query: service is required")
	}

	handler := MCPHandler(svc)

	httpServer := &http.Server{
		Addr:    listen,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("trove: mcp listening on %s", listen)

	err := httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newMCPHandler(svc *Service) http.Handler {
	server := newMCPServer(svc)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
}

func newMCPServer(svc *Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "trove",
		Version: "0.1.0",
	}, nil)

	type getEventParams struct {
		ID string `json:"id" jsonschema:"ULID of the event to retrieve"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Description: "Return a journal event by ULID",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params getEventParams) (*mcp.CallToolResult, any, error) {
		event, err := svc.GetEvent(ctx, params.ID)
		if err != nil {
			return nil, nil, err
		}
		data, err := json.Marshal(event)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	type searchEventsParams struct {
		Query      string `json:"query" jsonschema:"required,Keyword search query"`
		TypePrefix string `json:"type_prefix,omitempty" jsonschema:"Optional event type prefix filter"`
		Source     string `json:"source,omitempty" jsonschema:"Optional source filter"`
		TimeFrom   string `json:"time_from,omitempty" jsonschema:"Optional RFC3339 start of time range"`
		TimeTo     string `json:"time_to,omitempty" jsonschema:"Optional RFC3339 end of time range"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_events",
		Description: "Search journal events by keyword using FTS5",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params searchEventsParams) (*mcp.CallToolResult, any, error) {
		searchParams, err := searchParamsFromMCP(params.TypePrefix, params.Source, params.TimeFrom, params.TimeTo)
		if err != nil {
			return nil, nil, err
		}
		events, err := svc.SearchEvents(ctx, params.Query, searchParams)
		if err != nil {
			return nil, nil, err
		}
		data, err := json.Marshal(events)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	type getEventsByTypeParams struct {
		Type     string `json:"type" jsonschema:"required,Exact event type to retrieve"`
		TimeFrom string `json:"time_from,omitempty" jsonschema:"Optional RFC3339 start of time range"`
		TimeTo   string `json:"time_to,omitempty" jsonschema:"Optional RFC3339 end of time range"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_events_by_type",
		Description: "Return journal events matching an exact event type",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params getEventsByTypeParams) (*mcp.CallToolResult, any, error) {
		timeFrom, err := parseOptionalRFC3339(params.TimeFrom)
		if err != nil {
			return nil, nil, err
		}
		timeTo, err := parseOptionalRFC3339(params.TimeTo)
		if err != nil {
			return nil, nil, err
		}
		events, err := svc.GetEventsByType(ctx, params.Type, timeFrom, timeTo)
		if err != nil {
			return nil, nil, err
		}
		data, err := json.Marshal(events)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
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
		summary, err := svc.SummarizeRange(ctx, timeFrom, timeTo)
		if err != nil {
			return nil, nil, err
		}
		data, err := json.Marshal(summary)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	return server
}

func searchParamsFromMCP(typePrefix, source, timeFrom, timeTo string) (SearchParams, error) {
	from, err := parseOptionalRFC3339(timeFrom)
	if err != nil {
		return SearchParams{}, err
	}
	to, err := parseOptionalRFC3339(timeTo)
	if err != nil {
		return SearchParams{}, err
	}
	return SearchParams{
		TypePrefix: typePrefix,
		Source:     source,
		TimeFrom:   from,
		TimeTo:     to,
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
