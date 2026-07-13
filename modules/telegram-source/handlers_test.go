package main

import (
	"context"
	"encoding/json"
	"testing"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/records"
	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

const noteQuickType = "trove://type/note/quick/1"

type stubCore struct {
	journal *stubJournal
	records map[string]*troverpc.Record
}

func (s *stubCore) AppendRevision(ctx context.Context, req *troverpc.AppendRevisionRequest) (*troverpc.AppendRevisionResponse, error) {
	resp, err := s.journal.AppendRevision(ctx, req)
	if err != nil {
		return nil, err
	}
	ref := resp.GetRecordRef()
	if ref == "" {
		ref = req.GetRecordRef()
	}
	completeness := records.CompletenessIncomplete
	if req.GetType() != "" {
		completeness = records.CompletenessComplete
	}
	version := resp.GetVersion()
	if existing, ok := s.records[ref]; ok && req.GetType() != "" {
		version = existing.GetVersion() + 1
	}
	s.records[ref] = &troverpc.Record{
		RecordRef:    ref,
		Version:      version,
		Completeness: completeness,
		Type:         req.GetType(),
		Source:       req.GetSource(),
		Body:         req.GetPayload(),
	}
	return resp, nil
}

func (s *stubCore) Put(_ context.Context, _ []byte) (string, error) {
	return "blob:stub", nil
}

func (s *stubCore) GetRevision(ctx context.Context, id string) (*troverpc.Revision, error) {
	return s.journal.GetRevision(ctx, id)
}

func (s *stubCore) SearchRevisions(_ context.Context, _ *troverpc.SearchRevisionsRequest) ([]*troverpc.Revision, error) {
	return nil, nil
}

func (s *stubCore) GetRevisionsByType(ctx context.Context, req *troverpc.GetRevisionsByTypeRequest) ([]*troverpc.Revision, error) {
	return s.journal.GetRevisionsByType(ctx, req.Type)
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

func (s *stubCore) ListTypes(context.Context, string) ([]trovemodule.TypeSummary, error) {
	return nil, nil
}

func (s *stubCore) GetType(context.Context, string) (trovemodule.TypeSummary, json.RawMessage, error) {
	return trovemodule.TypeSummary{}, nil, nil
}

func (s *stubCore) ExportType(context.Context, string) ([]byte, string, error) {
	return nil, "", nil
}

func (s *stubCore) ValidateTypeDefinition(context.Context, []byte) (bool, string, string, error) {
	return false, "", "", nil
}

func (s *stubCore) GetRecord(_ context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	rec, ok := s.records[req.GetRecordRef()]
	if !ok {
		return nil, classify.ErrNotFound
	}
	return rec, nil
}

func (s *stubCore) SearchRecords(context.Context, *troverpc.SearchRecordsRequest) ([]*troverpc.Record, error) {
	return nil, nil
}

func (s *stubCore) ListIncompleteRecords(context.Context, *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	return nil, nil
}

var (
	_ trovemodule.Core                   = (*stubCore)(nil)
	_ trovemodule.RecordProjectionReader = (*stubCore)(nil)
)

func mustNewBotService(t *testing.T, cfg config, core *stubCore) *botService {
	t.Helper()
	return &botService{
		cfg:      cfg,
		core:     core,
		store:    &captureStore{core: core, records: core},
		sessions: newSessionStore(cfg.SessionTTLMin),
	}
}

type stubJournal struct {
	events map[string]*troverpc.Revision
	byType map[string][]*troverpc.Revision
}

func newStubJournal() *stubJournal {
	return &stubJournal{
		events: make(map[string]*troverpc.Revision),
		byType: make(map[string][]*troverpc.Revision),
	}
}

func (s *stubJournal) GetRevision(_ context.Context, id string) (*troverpc.Revision, error) {
	event, ok := s.events[id]
	if !ok {
		return nil, classify.ErrNotFound
	}
	return event, nil
}

func (s *stubJournal) GetRevisionsByType(_ context.Context, eventType string) ([]*troverpc.Revision, error) {
	return append([]*troverpc.Revision(nil), s.byType[eventType]...), nil
}

func (s *stubJournal) AppendRevision(_ context.Context, req *troverpc.AppendRevisionRequest) (*troverpc.AppendRevisionResponse, error) {
	event := &troverpc.Revision{
		Id:        req.GetRecordRef(),
		Type:      req.GetType(),
		Source:    req.GetSource(),
		Payload:   req.GetPayload(),
		Time:      req.GetTime(),
		BlobRef:   req.GetBlobRef(),
		Operation: req.GetOperation(),
		RecordRef: req.GetRecordRef(),
	}
	if event.Id == "" {
		event.Id = "01JEMIT" + event.Type
	}
	if event.RecordRef == "" {
		event.RecordRef = event.Id
	}
	s.events[event.Id] = event
	s.byType[event.Type] = append(s.byType[event.Type], event)
	return &troverpc.AppendRevisionResponse{RevisionId: event.Id, RecordRef: event.RecordRef, Version: 1, Operation: req.GetOperation()}, nil
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
		Mode:             modeClassify,
		PendingRecordRef: "01JACTIVE",
	})

	if id, busy := store.activePendingID(chatID); !busy || id != "01JACTIVE" {
		t.Fatalf("activePendingID() = %q, %v; want 01JACTIVE, true", id, busy)
	}
}

func TestFinishClassifyEmitsTypedEvent(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	core := &stubCore{journal: j, records: make(map[string]*troverpc.Record)}
	svc := mustNewBotService(t, testConfig(), core)
	chatID := int64(100)

	result, err := classify.Capture(context.Background(), svc.store, "telegram", []byte(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}

	sess := &session{
		Mode:             modeClassify,
		PendingRecordRef: result.RecordRef,
		TargetType:       noteQuickType,
		Collected:        map[string]string{"text": "hello"},
	}
	svc.finishClassify(context.Background(), nil, chatID, sess)

	if _, ok := svc.sessions.get(chatID); ok {
		t.Fatal("session not cleared after classify")
	}
	if len(j.byType[noteQuickType]) != 1 {
		t.Fatalf("typed events = %#v", j.byType[noteQuickType])
	}
}

func TestFinishFastPathEmitsDirectly(t *testing.T) {
	t.Parallel()

	j := newStubJournal()
	core := &stubCore{journal: j, records: make(map[string]*troverpc.Record)}
	svc := mustNewBotService(t, testConfig(), core)
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
