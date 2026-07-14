package mcpquery

import (
	"context"
	"net/http"
	"testing"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type stubRecords struct{}

func (s *stubRecords) GetRecord(context.Context, *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	return nil, query.ErrRecordNotFound
}

func (s *stubRecords) SearchRecords(context.Context, *troverpc.SearchRecordsRequest) ([]*troverpc.Record, error) {
	return nil, nil
}

func (s *stubRecords) ListIncompleteRecords(context.Context, *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	return nil, nil
}

func TestHandleHTTPNotReady(t *testing.T) {
	t.Parallel()

	mod := &Module{}
	resp, err := mod.HandleHTTP(context.Background(), &troverpc.HTTPRequest{
		Method:         http.MethodPost,
		Path:           "/mcp",
		MatchedPattern: "/mcp",
	})
	if err != nil {
		t.Fatalf("HandleHTTP() error = %v", err)
	}
	if resp.Status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", resp.Status, http.StatusServiceUnavailable)
	}
}

func TestHandleHTTPDispatchesMCP(t *testing.T) {
	t.Parallel()

	mod := &Module{}
	mod.handler = query.MCPHandler(query.MCPDeps{Records: &recordAdapter{records: &stubRecords{}}})
	mod.ready.Store(true)

	resp, err := mod.HandleHTTP(context.Background(), &troverpc.HTTPRequest{
		Method:         http.MethodPost,
		Path:           "/mcp",
		MatchedPattern: "/mcp",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json, text/event-stream",
		},
		Body: []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}`),
	})
	if err != nil {
		t.Fatalf("HandleHTTP() error = %v", err)
	}
	if resp.Status != http.StatusOK && resp.Status != http.StatusAccepted {
		t.Errorf("status = %d, want 200 or 202; body = %q", resp.Status, resp.Body)
	}
}

var (
	_ trovemodule.Module      = (*Module)(nil)
	_ trovemodule.HTTPHandler = (*Module)(nil)
)
