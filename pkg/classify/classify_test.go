package classify_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/classify"
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

func TestCapturePendingWithResultReturnsID(t *testing.T) {
	j := newStubJournal()
	got, err := classify.CapturePendingWithResult(context.Background(), j, "telegram", []byte(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("CapturePendingWithResult() error = %v", err)
	}
	if got.EventID == "" {
		t.Fatal("CapturePendingWithResult() EventID is empty")
	}
	event := j.events[got.EventID]
	if event == nil {
		t.Fatalf("event %q not stored", got.EventID)
	}
	if event.Type != classify.PendingType {
		t.Fatalf("type = %q, want %q", event.Type, classify.PendingType)
	}
	if event.Id != got.EventID {
		t.Fatalf("stored id = %q, result id = %q", event.Id, got.EventID)
	}
}

func TestClassifyHappyPath(t *testing.T) {
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

	got, err := classify.Classify(context.Background(), j, classify.ClassifyRequest{
		SourceEventID: pendingID,
		TargetType:    "shortcuts.note.created",
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if got.TargetEventID == "" || got.ClassificationEventID == "" {
		t.Fatalf("Classify() = %+v, want event ids", got)
	}

	target := j.events[got.TargetEventID]
	if target.Type != "shortcuts.note.created" {
		t.Fatalf("target type = %q", target.Type)
	}
	var payload map[string]any
	if err := json.Unmarshal(target.Payload, &payload); err != nil {
		t.Fatalf("unmarshal target payload: %v", err)
	}
	trove, ok := payload["_trove"].(map[string]any)
	if !ok || trove["derived_from"] != pendingID {
		t.Fatalf("target payload _trove = %#v", payload["_trove"])
	}

	link := j.events[got.ClassificationEventID]
	if link.Type != classify.AssignedType {
		t.Fatalf("link type = %q", link.Type)
	}
}

func TestClassifyRejectsDoubleClassify(t *testing.T) {
	j := newStubJournal()
	pendingID := "01JPENDING000000000000001"
	j.events[pendingID] = &troverpc.Event{
		Id:      pendingID,
		Type:    classify.PendingType,
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"hello"}`),
	}
	j.byType[classify.PendingType] = []*troverpc.Event{j.events[pendingID]}
	j.byType[classify.AssignedType] = []*troverpc.Event{{
		Id:      "01JASSIGNED00000000000000",
		Type:    classify.AssignedType,
		Payload: json.RawMessage(`{"source_event_id":"` + pendingID + `"}`),
	}}

	_, err := classify.Classify(context.Background(), j, classify.ClassifyRequest{
		SourceEventID: pendingID,
		TargetType:    "shortcuts.note.created",
	})
	if !errors.Is(err, classify.ErrAlreadyClassified) {
		t.Fatalf("Classify() error = %v, want %v", err, classify.ErrAlreadyClassified)
	}
}

func TestListUnclassified(t *testing.T) {
	j := newStubJournal()
	pendingDone := "01JPENDING000000000000010"
	pendingOpen := "01JPENDING000000000000011"
	j.byType[classify.PendingType] = []*troverpc.Event{
		{Id: pendingDone, Type: classify.PendingType, Payload: json.RawMessage(`{}`)},
		{Id: pendingOpen, Type: classify.PendingType, Payload: json.RawMessage(`{}`)},
	}
	j.byType[classify.AssignedType] = []*troverpc.Event{{
		Id:      "01JASSIGNED00000000000001",
		Type:    classify.AssignedType,
		Payload: json.RawMessage(`{"source_event_id":"` + pendingDone + `"}`),
	}}

	got, err := classify.ListUnclassified(context.Background(), j)
	if err != nil {
		t.Fatalf("ListUnclassified() error = %v", err)
	}
	if len(got) != 1 || got[0].Id != pendingOpen {
		t.Fatalf("ListUnclassified() = %#v, want only %q", got, pendingOpen)
	}
}
