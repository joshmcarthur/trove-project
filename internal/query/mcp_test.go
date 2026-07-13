package query

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpTestQuerier struct {
	events  *Service
	records *RecordService
}

func (q *mcpTestQuerier) GetEvent(ctx context.Context, id string) (Event, error) {
	return q.events.GetEvent(ctx, id)
}

func (q *mcpTestQuerier) SummarizeRange(ctx context.Context, timeFrom, timeTo time.Time) (Summary, error) {
	return q.events.SummarizeRange(ctx, timeFrom, timeTo)
}

func (q *mcpTestQuerier) GetRecord(ctx context.Context, recordRef string, version int) (Record, error) {
	return q.records.GetRecord(ctx, recordRef, version)
}

func (q *mcpTestQuerier) SearchRecords(ctx context.Context, text string, params RecordSearchParams) ([]Record, error) {
	return q.records.SearchRecords(ctx, text, params)
}

func (q *mcpTestQuerier) ListIncompleteRecords(ctx context.Context, source string, limit int) ([]Record, error) {
	return q.records.ListIncompleteRecords(ctx, source, limit)
}

func newMCPTestQuerier(store *journal.Store) *mcpTestQuerier {
	return &mcpTestQuerier{
		events:  &Service{Journal: store},
		records: &RecordService{DB: store.DB()},
	}
}

func TestMCPGetEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	want := journal.Event{
		ID:        "01JEVT00000000000000000001",
		Time:      when,
		Type:      "trove://type/http/ingest/received/1",
		Source:    "shortcuts",
		Payload:   json.RawMessage(`{"text":"hello"}`),
		Operation: journal.OpApply,
	}
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_event",
		Arguments: map[string]any{
			"id": want.ID,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got Event
	decodeToolResult(t, result, &got)
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
}

func TestMCPGetEventNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)
	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_event",
		Arguments: map[string]any{
			"id": "01J0000000000000000000000",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool() = %#v, want tool error", result)
	}
}

func TestMCPGetRecord(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)
	ref := "01JREC00000000000000000040"
	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	seedRecord(t, store, "01JEVT00000000000000000040", ref, when, "shortcuts", `{"text":"hello"}`, "trove://type/note/quick/1")

	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_record",
		Arguments: map[string]any{
			"record_ref": ref,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got Record
	decodeToolResult(t, result, &got)
	if got.RecordRef != ref {
		t.Errorf("RecordRef = %q, want %q", got.RecordRef, ref)
	}
	if got.Completeness != records.CompletenessComplete {
		t.Errorf("Completeness = %q, want %q", got.Completeness, records.CompletenessComplete)
	}
}

func TestMCPSearchRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)
	ref := "01JREC00000000000000000041"
	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	seedRecord(t, store, "01JEVT00000000000000000041", ref, when, "shortcuts", `{"text":"hello world"}`, "trove://type/note/quick/1")

	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_records",
		Arguments: map[string]any{
			"query":       "hello",
			"type_prefix": "trove://type/note/",
			"source":      "shortcuts",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got []Record
	decodeToolResult(t, result, &got)
	if len(got) != 1 {
		t.Fatalf("SearchRecords returned %d records, want 1", len(got))
	}
	if got[0].RecordRef != ref {
		t.Errorf("RecordRef = %q, want %q", got[0].RecordRef, ref)
	}
}

func TestMCPSearchRecordsEmptyQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)
	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_records",
		Arguments: map[string]any{
			"query": "   ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool() = %#v, want tool error", result)
	}
}

func TestMCPListIncompleteRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)
	ref := "01JREC00000000000000000042"
	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	seedRecord(t, store, "01JEVT00000000000000000042", ref, when, "shortcuts", `{"text":"draft"}`, "")

	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_incomplete_records",
		Arguments: map[string]any{
			"source": "shortcuts",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got []Record
	decodeToolResult(t, result, &got)
	if len(got) != 1 || got[0].RecordRef != ref {
		t.Fatalf("ListIncompleteRecords = %#v, want record %q", got, ref)
	}
}

func TestMCPSummarizeRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	want := journal.Event{
		ID:        "01JEVT00000000000000000002",
		Time:      when,
		Type:      "trove://type/http/ingest/received/1",
		Source:    "shortcuts",
		Payload:   json.RawMessage(`{"text":"hello"}`),
		Operation: journal.OpApply,
	}
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	timeFrom := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	timeTo := time.Date(2026, 7, 10, 23, 59, 59, 0, time.UTC).Format(time.RFC3339)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "summarize_range",
		Arguments: map[string]any{
			"time_from": timeFrom,
			"time_to":   timeTo,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got Summary
	decodeToolResult(t, result, &got)
	if got.Total != 1 {
		t.Errorf("Total = %d, want 1", got.Total)
	}
}

func TestMCPSummarizeRangeInvalidRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, _ := openTestRecordService(t)
	handler := newMCPHandler(MCPDeps{Querier: newMCPTestQuerier(store)})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "summarize_range",
		Arguments: map[string]any{
			"time_from": time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"time_to":   time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool() = %#v, want tool error", result)
	}
}

func decodeToolResult(t *testing.T, result *mcp.CallToolResult, out any) {
	t.Helper()

	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(text.Text), out); err != nil {
			t.Fatalf("unmarshal tool output: %v", err)
		}
		return
	}
	t.Fatal("expected text content with JSON output")
}
