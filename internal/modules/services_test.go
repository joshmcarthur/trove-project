package modules

import (
	"context"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
	"github.com/joshmcarthur/trove/internal/types"
)

func TestCoreServicesAppendRevision(t *testing.T) {
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

	catalog := types.NewCatalog()
	registerPermissiveCatalogType(t, catalog, "test.event")

	policy, err := NewWritePolicy([]string{"test.event"}, catalog, "test")
	if err != nil {
		t.Fatalf("NewWritePolicy() error = %v", err)
	}

	srv := &coreServicesServer{
		journal: j,
		store:   j,
		policy:  policy,
		writer:  NewWriteService(j),
		blobs:   store,
	}
	_, err = srv.AppendRevision(context.Background(), &troverpc.AppendRevisionRequest{
		Operation: "apply",
		Type:      "test.event",
		Source:    "src",
		Payload:   []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("AppendRevision() error = %v", err)
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

func TestCoreServicesGetRevision(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/journal.db"
	j, err := journal.Open(path)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	if err := j.Append(context.Background(), journal.Revision{
		Type:    "trove://type/note/created/1",
		Source:  "test",
		Time:    now,
		Payload: []byte(`{"title":"hello"}`),
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	events, err := j.Query(context.Background(), journal.Filter{TypePrefix: "trove://type/note/"})
	if err != nil || len(events) != 1 {
		t.Fatalf("Query() = %v, %d events", err, len(events))
	}

	srv := &coreServicesServer{query: &query.Service{Journal: j}}
	resp, err := srv.GetRevision(context.Background(), &troverpc.GetRevisionRequest{Id: events[0].ID})
	if err != nil {
		t.Fatalf("GetRevision() error = %v", err)
	}
	if resp.Id != events[0].ID {
		t.Errorf("Id = %q, want %q", resp.Id, events[0].ID)
	}
}

func TestCoreServicesCallMCPTool(t *testing.T) {
	t.Parallel()

	registry := NewMCPRegistry()
	dispatcher := &stubMCPDispatcher{}
	registry.Register("example-module", dispatcher)

	srv := &coreServicesServer{
		toolModules: map[string]string{"example_tool": "example-module"},
		mcpRegistry: registry,
	}

	resp, err := srv.CallMCPTool(context.Background(), &troverpc.MCPToolCallRequest{
		Name:          "example_tool",
		ArgumentsJson: []byte(`{"query":"test"}`),
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
