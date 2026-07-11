package modules

import (
	"context"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
)

func TestCoreServicesEmit(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	j, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })

	srv := &coreServicesServer{
		journal: j,
		policy: IngestPolicy{
			patterns:   []string{"test.event"},
			moduleName: "test",
		},
		blobs: store,
	}
	_, err = srv.Emit(context.Background(), &troverpc.Event{
		Type:    "test.event",
		Source:  "src",
		Payload: []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
}

func TestCoreServicesBlobPut(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	srv := &coreServicesServer{blobs: store}
	resp, err := srv.BlobPut(context.Background(), &troverpc.BlobPutRequest{Data: []byte("hello")})
	if err != nil {
		t.Fatalf("BlobPut() error = %v", err)
	}
	if resp.BlobRef == "" {
		t.Fatal("BlobRef is empty")
	}

	rc, err := store.Get(context.Background(), resp.BlobRef)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer rc.Close()

	buf := make([]byte, 5)
	if _, err := rc.Read(buf); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("stored data = %q, want hello", buf)
	}
}

func TestCoreServicesBlobPutEmpty(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	srv := &coreServicesServer{blobs: store}
	_, err = srv.BlobPut(context.Background(), &troverpc.BlobPutRequest{})
	if err == nil {
		t.Fatal("BlobPut() error = nil, want error")
	}
}

func TestCoreServicesGetEvent(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/journal.db"
	j, err := journal.Open(path)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	if err := j.Append(context.Background(), journal.Event{
		Type:    "note.created",
		Source:  "test",
		Time:    now,
		Payload: []byte(`{"title":"hello"}`),
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	events, err := j.Query(context.Background(), journal.Filter{TypePrefix: "note."})
	if err != nil || len(events) != 1 {
		t.Fatalf("Query() = %v, %d events", err, len(events))
	}

	srv := &coreServicesServer{query: &query.Service{Journal: j}}
	resp, err := srv.GetEvent(context.Background(), &troverpc.GetEventRequest{Id: events[0].ID})
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
	}
	if resp.Id != events[0].ID {
		t.Errorf("Id = %q, want %q", resp.Id, events[0].ID)
	}
}

func TestCoreServicesCallMCPTool(t *testing.T) {
	t.Parallel()

	registry := NewMCPRegistry()
	dispatcher := &stubMCPDispatcher{}
	registry.Register("capture-classifier", dispatcher)

	srv := &coreServicesServer{
		toolModules: map[string]string{"classify_event": "capture-classifier"},
		mcpRegistry: registry,
	}

	resp, err := srv.CallMCPTool(context.Background(), &troverpc.MCPToolCallRequest{
		Name:          "classify_event",
		ArgumentsJson: []byte(`{"source_event_id":"01JTEST","target_type":"note.created"}`),
	})
	if err != nil {
		t.Fatalf("CallMCPTool() error = %v", err)
	}
	if !dispatcher.called {
		t.Fatal("dispatcher was not called")
	}
	if string(resp.ResultJson) != `{"ok":true}` {
		t.Fatalf("ResultJson = %s", resp.ResultJson)
	}
}

type stubMCPDispatcher struct {
	called bool
}

func (s *stubMCPDispatcher) CallTool(ctx context.Context, req *troverpc.MCPToolCallRequest) (*troverpc.MCPToolCallResponse, error) {
	_ = ctx
	_ = req
	s.called = true
	return &troverpc.MCPToolCallResponse{ResultJson: []byte(`{"ok":true}`)}, nil
}
