package main

import (
	"context"
	"encoding/json"
	"testing"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

const noteQuickType = "trove://type/note/quick/1"

type stubCore struct {
	journal *stubJournal
}

func (s *stubCore) Emit(ctx context.Context, event *troverpc.Event) error {
	return s.journal.Emit(ctx, event)
}

func (s *stubCore) Put(_ context.Context, _ []byte) (string, error) {
	return "blob:stub", nil
}

func (s *stubCore) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	return s.journal.GetEvent(ctx, id)
}

func (s *stubCore) SearchEvents(_ context.Context, _ *troverpc.SearchEventsRequest) ([]*troverpc.Event, error) {
	return nil, nil
}

func (s *stubCore) GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error) {
	return s.journal.GetEventsByType(ctx, req.Type)
}

func (s *stubCore) SummarizeRange(_ context.Context, _ *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return nil, nil
}

func (s *stubCore) ListMCPTools(_ context.Context) ([]trovemodule.MCPToolDescriptor, error) {
	return nil, nil
}

func (s *stubCore) CallMCPTool(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

var _ trovemodule.Core = (*stubCore)(nil)

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
		return nil, classify.ErrNotFound
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
	return config{
		AllowedChatIDs: []int64{100},
		Types: []typeOption{
			{Label: "Quick note", TargetType: noteQuickType},
		},
		Commands: []commandConfig{{
			Command:     "note",
			Description: "Quick note",
			TargetType:  noteQuickType,
			FastPath:    true,
		}},
	}
}

func TestHandleCaptureBlocksWhenPendingActive(t *testing.T) {
	t.Parallel()

	store := newSessionStore(30)
	chatID := int64(100)
	store.set(chatID, &session{
		Mode:           modeClassify,
		PendingEventID: "01JACTIVE",
	})

	if id, busy := store.activePendingID(chatID); !busy || id != "01JACTIVE" {
		t.Fatalf("activePendingID() = %q, %v; want 01JACTIVE, true", id, busy)
	}
}

func TestFinishClassifyEmitsTypedEvent(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	core := &stubCore{journal: j}
	svc := newBotService(testConfig(), core)
	chatID := int64(100)

	result, err := classify.CapturePendingWithResult(context.Background(), j, "telegram", []byte(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("CapturePendingWithResult() error = %v", err)
	}

	sess := &session{
		Mode:           modeClassify,
		PendingEventID: result.EventID,
		TargetType:     noteQuickType,
		Collected:      map[string]string{"text": "hello"},
	}
	svc.finishClassify(context.Background(), nil, chatID, sess)

	if _, ok := svc.sessions.get(chatID); ok {
		t.Fatal("session not cleared after classify")
	}
	if len(j.byType[noteQuickType]) != 1 {
		t.Fatalf("typed events = %#v", j.byType[noteQuickType])
	}
	if len(j.byType[classify.AssignedType]) != 1 {
		t.Fatalf("assigned events = %#v", j.byType[classify.AssignedType])
	}
}

func TestFinishFastPathEmitsDirectly(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	core := &stubCore{journal: j}
	svc := newBotService(testConfig(), core)
	chatID := int64(100)

	sess := &session{
		Mode:       modeFastPath,
		TargetType: noteQuickType,
		Collected:  map[string]string{"text": "fast"},
		Draft: &captureDraft{
			Time:        "2026-07-10T10:00:00Z",
			CaptureJSON: []byte(`{"text":"fast"}`),
		},
	}
	svc.finishFastPath(context.Background(), nil, chatID, sess)

	if len(j.byType[classify.PendingType]) != 0 {
		t.Fatal("fast path should not create pending event")
	}
	if len(j.byType[noteQuickType]) != 1 {
		t.Fatalf("typed events = %#v", j.byType[noteQuickType])
	}
	var payload map[string]any
	if err := json.Unmarshal(j.byType[noteQuickType][0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["text"] != "fast" {
		t.Fatalf("payload = %#v", payload)
	}
}
