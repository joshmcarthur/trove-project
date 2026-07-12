package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubCore struct {
	journal *stubJournal
}

func (s *stubCore) Emit(ctx context.Context, event *troverpc.Event) error {
	return s.journal.Emit(ctx, event)
}

func (s *stubCore) Put(context.Context, []byte) (string, error) {
	return "", nil
}

func (s *stubCore) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	return s.journal.GetEvent(ctx, id)
}

func (s *stubCore) SearchEvents(context.Context, *troverpc.SearchEventsRequest) ([]*troverpc.Event, error) {
	return nil, nil
}

func (s *stubCore) GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error) {
	return s.journal.GetEventsByType(ctx, req.GetType())
}

func (s *stubCore) SummarizeRange(context.Context, *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return nil, nil
}

func (s *stubCore) ListMCPTools(context.Context) ([]trovemodule.MCPToolDescriptor, error) {
	return nil, nil
}

func (s *stubCore) CallMCPTool(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func TestCaptureClassifierHandleHTTPWhenReady(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	mod := &captureClassifierModule{
		cfg:  testConfig(),
		core: &stubCore{journal: j},
	}
	mod.ready.Store(true)

	resp, err := mod.HandleHTTP(context.Background(), &troverpc.HTTPRequest{
		Method:         "POST",
		MatchedPattern: "/capture/{source}",
		PathValues:     map[string]string{"source": "shortcuts"},
		Body:           []byte(`{"text":"capture me"}`),
	})
	if err != nil {
		t.Fatalf("HandleHTTP() error = %v", err)
	}
	if resp.Status != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.Status)
	}
}

func TestCaptureClassifierHandleHTTPWhenNotReady(t *testing.T) {
	t.Parallel()

	mod := &captureClassifierModule{cfg: testConfig()}
	resp, err := mod.HandleHTTP(context.Background(), &troverpc.HTTPRequest{
		Method:         "POST",
		MatchedPattern: "/capture/{source}",
		PathValues:     map[string]string{"source": "shortcuts"},
		Body:           []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("HandleHTTP() error = %v", err)
	}
	if resp.Status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.Status)
	}
}

func TestCaptureClassifierCallToolClassifyEvent(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	pendingID := "01JPENDING000000000000010"
	j.events[pendingID] = &troverpc.Event{
		Id:      pendingID,
		Type:    classify.PendingType,
		Source:  "shortcuts",
		Time:    time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Payload: json.RawMessage(`{"text":"hello"}`),
	}
	j.byType[classify.PendingType] = []*troverpc.Event{j.events[pendingID]}

	mod := &captureClassifierModule{
		cfg:  testConfig(),
		core: &stubCore{journal: j},
	}
	mod.ready.Store(true)

	args, _ := json.Marshal(map[string]string{
		"source_event_id": pendingID,
		"target_type":     "trove://type/note/created/1",
	})
	out, err := mod.CallTool(context.Background(), "classify_event", args)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	var result classify.ClassifyResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.TargetEventID == "" {
		t.Fatal("TargetEventID is empty")
	}
}

func TestCaptureClassifierCallToolListUnclassified(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	pending := &troverpc.Event{
		Id:      "01JPENDING000000000000011",
		Type:    classify.PendingType,
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"pending"}`),
	}
	j.events[pending.Id] = pending
	j.byType[classify.PendingType] = []*troverpc.Event{pending}

	mod := &captureClassifierModule{
		cfg:  testConfig(),
		core: &stubCore{journal: j},
	}
	mod.ready.Store(true)

	out, err := mod.CallTool(context.Background(), "list_unclassified_captures", nil)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	var events []map[string]any
	if err := json.Unmarshal(out, &events); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
}

func TestJournalAdapterMapsNotFound(t *testing.T) {
	t.Parallel()

	adapter := &journalAdapter{core: &stubCore{journal: newStubJournal()}}
	_, err := adapter.GetEvent(context.Background(), "missing")
	if err == nil {
		t.Fatal("GetEvent() error = nil, want ErrNotFound")
	}
	if err != classify.ErrNotFound {
		t.Fatalf("GetEvent() error = %v, want ErrNotFound", err)
	}
}

func TestJournalAdapterMapsGRPCNotFound(t *testing.T) {
	t.Parallel()

	core := &notFoundCore{}
	adapter := &journalAdapter{core: core}
	_, err := adapter.GetEvent(context.Background(), "missing")
	if err != classify.ErrNotFound {
		t.Fatalf("GetEvent() error = %v, want ErrNotFound", err)
	}
}

type notFoundCore struct{}

func (notFoundCore) Emit(context.Context, *troverpc.Event) error { return nil }
func (notFoundCore) Put(context.Context, []byte) (string, error) { return "", nil }
func (notFoundCore) GetEvent(context.Context, string) (*troverpc.Event, error) {
	return nil, status.Error(codes.NotFound, "not found")
}
func (notFoundCore) SearchEvents(context.Context, *troverpc.SearchEventsRequest) ([]*troverpc.Event, error) {
	return nil, nil
}
func (notFoundCore) GetEventsByType(context.Context, *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error) {
	return nil, nil
}
func (notFoundCore) SummarizeRange(context.Context, *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return nil, nil
}
func (notFoundCore) ListMCPTools(context.Context) ([]trovemodule.MCPToolDescriptor, error) {
	return nil, nil
}
func (notFoundCore) CallMCPTool(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}
