package query

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oklog/ulid"
)

func TestMCPGetEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	id := ulid.MustNew(ulid.Now(), rand.Reader).String()
	want := journal.Event{
		ID:      id,
		Time:    when,
		Type:    "http.ingest.received",
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"hello"}`),
	}
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	svc := &Service{Journal: store}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
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
			"id": id,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got Event
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
			t.Fatalf("unmarshal tool output: %v", err)
		}
		break
	}
	if got.ID == "" {
		t.Fatal("expected text content with event JSON")
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Type != want.Type {
		t.Errorf("Type = %q, want %q", got.Type, want.Type)
	}
	if got.Source != want.Source {
		t.Errorf("Source = %q, want %q", got.Source, want.Source)
	}
}

func TestMCPGetEventNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
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

func TestMCPSearchEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	id := ulid.MustNew(ulid.Now(), rand.Reader).String()
	want := journal.Event{
		ID:      id,
		Time:    when,
		Type:    "http.ingest.received",
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"hello world"}`),
	}
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	svc := &Service{Journal: store}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_events",
		Arguments: map[string]any{
			"query":       "hello",
			"type_prefix": "http.",
			"source":      "shortcuts",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got []Event
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
			t.Fatalf("unmarshal tool output: %v", err)
		}
		break
	}
	if len(got) != 1 {
		t.Fatalf("SearchEvents returned %d events, want 1", len(got))
	}
	if got[0].ID != want.ID {
		t.Errorf("ID = %q, want %q", got[0].ID, want.ID)
	}
}

func TestMCPSearchEventsEmptyQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_events",
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

func TestMCPGetEventsByType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)

	t1 := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)

	seed := []journal.Event{
		{ID: "01JEVT00000000000000000001", Time: t1, Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"v":1}`)},
		{ID: "01JEVT00000000000000000002", Time: t2, Type: "mqtt.sensor.humidity", Source: "sensor-a", Payload: json.RawMessage(`{"v":2}`)},
	}
	for _, e := range seed {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	svc := &Service{Journal: store}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_events_by_type",
		Arguments: map[string]any{
			"type":      "mqtt.sensor.temp",
			"time_from": t1.Format(time.RFC3339),
			"time_to":   t2.Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result)
	}

	var got []Event
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
			t.Fatalf("unmarshal tool output: %v", err)
		}
		break
	}
	if len(got) != 1 {
		t.Fatalf("GetEventsByType returned %d events, want 1", len(got))
	}
	if got[0].ID != seed[0].ID {
		t.Errorf("ID = %q, want %q", got[0].ID, seed[0].ID)
	}
}

func TestMCPGetEventsByTypeEmptyType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_events_by_type",
		Arguments: map[string]any{
			"type": "   ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool() = %#v, want tool error", result)
	}
}

func TestMCPSummarizeRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	want := journal.Event{
		ID:      ulid.MustNew(ulid.Now(), rand.Reader).String(),
		Time:    when,
		Type:    "http.ingest.received",
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"hello"}`),
	}
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	svc := &Service{Journal: store}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
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
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
			t.Fatalf("unmarshal tool output: %v", err)
		}
		break
	}
	if got.Total != 1 {
		t.Errorf("Total = %d, want 1", got.Total)
	}
	if got.ByType["http.ingest.received"] != 1 {
		t.Errorf("ByType[http.ingest.received] = %d, want 1", got.ByType["http.ingest.received"])
	}
	if len(got.Notable) != 1 || got.Notable[0].ID != want.ID {
		t.Errorf("Notable = %#v, want event %q", got.Notable, want.ID)
	}
}

func TestMCPSummarizeRangeInvalidRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}
	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
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
