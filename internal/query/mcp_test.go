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
