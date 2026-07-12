package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/classify"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubJournal struct {
	events map[string]*troverpc.Event
	byType map[string][]*troverpc.Event
}

func newStubJournal() *stubJournal {
	return &stubJournal{
		events: make(map[string]*troverpc.Event),
		byType: make(map[string][]*troverpc.Event),
	}
}

func (s *stubJournal) GetEvent(_ context.Context, id string) (*troverpc.Event, error) {
	event, ok := s.events[id]
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}
	return event, nil
}

func (s *stubJournal) GetEventsByType(_ context.Context, eventType string) ([]*troverpc.Event, error) {
	return append([]*troverpc.Event(nil), s.byType[eventType]...), nil
}

func (s *stubJournal) Emit(_ context.Context, event *troverpc.Event) error {
	if event.Id == "" {
		event.Id = "01JEMIT" + event.Type
	}
	s.events[event.Id] = event
	s.byType[event.Type] = append(s.byType[event.Type], event)
	return nil
}

func testConfig() config {
	return config{MaxBodyBytes: defaultMaxBodyBytes}
}

func TestHandleCapture(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	resp, err := handleCapture(context.Background(), j, testConfig(), &troverpc.HTTPRequest{
		Method:         "POST",
		MatchedPattern: "/capture/{source}",
		PathValues:     map[string]string{"source": "shortcuts"},
		Body:           []byte(`{"text":"hello"}`),
	})
	if err != nil {
		t.Fatalf("handleCapture() error = %v", err)
	}
	if resp.Status != 204 {
		t.Fatalf("status = %d, want 204", resp.Status)
	}
	if len(j.byType[classify.PendingType]) != 1 {
		t.Fatalf("pending events = %d, want 1", len(j.byType[classify.PendingType]))
	}
}

func TestHandleCaptureRejectsEmptyBody(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	resp, err := handleCapture(context.Background(), j, testConfig(), &troverpc.HTTPRequest{
		Method:         "POST",
		MatchedPattern: "/capture/{source}",
		PathValues:     map[string]string{"source": "shortcuts"},
	})
	if err != nil {
		t.Fatalf("handleCapture() error = %v", err)
	}
	if resp.Status != 400 {
		t.Fatalf("status = %d, want 400", resp.Status)
	}
}

func TestHandleClassify(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	pendingID := "01JPENDING000000000000000"
	j.events[pendingID] = &troverpc.Event{
		Id:      pendingID,
		Type:    classify.PendingType,
		Source:  "shortcuts",
		Time:    time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Payload: json.RawMessage(`{"text":"hello"}`),
	}
	j.byType[classify.PendingType] = []*troverpc.Event{j.events[pendingID]}

	body, _ := json.Marshal(map[string]string{
		"source_event_id": pendingID,
		"target_type":     "trove://type/note/created/1",
	})
	resp, err := handleClassify(context.Background(), j, testConfig(), &troverpc.HTTPRequest{
		Method:         "POST",
		MatchedPattern: "/classify",
		Body:           body,
	})
	if err != nil {
		t.Fatalf("handleClassify() error = %v", err)
	}
	if resp.Status != 201 {
		t.Fatalf("status = %d, want 201; body=%s", resp.Status, resp.Body)
	}
}

func TestHandlePending(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	pending := &troverpc.Event{
		Id:      "01JPENDING000000000000001",
		Type:    classify.PendingType,
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"pending"}`),
	}
	j.events[pending.Id] = pending
	j.byType[classify.PendingType] = []*troverpc.Event{pending}

	resp, err := handlePending(context.Background(), j)
	if err != nil {
		t.Fatalf("handlePending() error = %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	var events []map[string]any
	if err := json.Unmarshal(resp.Body, &events); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
}

func TestDispatchHTTPNotFound(t *testing.T) {
	t.Parallel()

	resp, err := dispatchHTTP(context.Background(), newStubJournal(), testConfig(), &troverpc.HTTPRequest{
		Method:         "GET",
		MatchedPattern: "/unknown",
	})
	if err != nil {
		t.Fatalf("dispatchHTTP() error = %v", err)
	}
	if resp.Status != 404 {
		t.Fatalf("status = %d, want 404", resp.Status)
	}
}
