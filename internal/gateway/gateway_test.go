package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/modules"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

type stubHTTPClient struct {
	resp *troverpc.HTTPResponse
	err  error
	last *troverpc.HTTPRequest
}

func (c *stubHTTPClient) HandleHTTP(_ context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	c.last = req
	if c.err != nil {
		return nil, c.err
	}
	return c.resp, nil
}

func TestGatewayNotFound(t *testing.T) {
	t.Parallel()

	registry := modules.NewHTTPRegistry()
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024}, nil, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGatewayMethodNotAllowed(t *testing.T) {
	t.Parallel()

	registry := modules.NewHTTPRegistry()
	routes := []modules.HTTPRouteEntry{{
		Route:  modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}"},
		Module: "http-ingest",
	}}
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024}, routes, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ingest/shortcuts", nil)
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGatewayDispatchesToModule(t *testing.T) {
	t.Parallel()

	client := &stubHTTPClient{
		resp: &troverpc.HTTPResponse{Status: http.StatusNoContent},
	}
	registry := modules.NewHTTPRegistry()
	registry.Register("http-ingest", client)

	routes := []modules.HTTPRouteEntry{{
		Route:  modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}"},
		Module: "http-ingest",
	}}
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024}, routes, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	body := `{"title":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest/shortcuts", strings.NewReader(body))
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d; body = %q", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if client.last == nil {
		t.Fatal("HandleHTTP was not called")
	}
	if client.last.MatchedPattern != "/ingest/{source}" {
		t.Errorf("MatchedPattern = %q, want /ingest/{source}", client.last.MatchedPattern)
	}
	if client.last.PathValues["source"] != "shortcuts" {
		t.Errorf("PathValues[source] = %q, want shortcuts", client.last.PathValues["source"])
	}
	if string(client.last.Body) != body {
		t.Errorf("Body = %q, want %q", client.last.Body, body)
	}
}

func TestGatewayServiceUnavailable(t *testing.T) {
	t.Parallel()

	registry := modules.NewHTTPRegistry()
	routes := []modules.HTTPRouteEntry{{
		Route:  modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}"},
		Module: "http-ingest",
	}}
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024}, routes, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest/shortcuts", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestGatewayBodyLimit(t *testing.T) {
	t.Parallel()

	client := &stubHTTPClient{resp: &troverpc.HTTPResponse{Status: http.StatusOK}}
	registry := modules.NewHTTPRegistry()
	registry.Register("http-ingest", client)

	routes := []modules.HTTPRouteEntry{{
		Route:  modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}", MaxBodyBytes: 8},
		Module: "http-ingest",
	}}
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024}, routes, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest/x", strings.NewReader("123456789"))
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestValidateRoutesDuplicate(t *testing.T) {
	t.Parallel()

	routes := []modules.HTTPRouteEntry{
		{Route: modules.HTTPRoute{Method: "POST", Path: "/a"}, Module: "m1"},
		{Route: modules.HTTPRoute{Method: "POST", Path: "/a"}, Module: "m2"},
	}
	if err := ValidateRoutes(routes, nil); err == nil {
		t.Fatal("ValidateRoutes() error = nil, want duplicate route error")
	}
}

func TestValidateRoutesBuiltinConflict(t *testing.T) {
	t.Parallel()

	routes := []modules.HTTPRouteEntry{
		{Route: modules.HTTPRoute{Method: "POST", Path: "/mcp"}, Module: "m1"},
	}
	builtins := []BuiltinRoute{{Method: "POST", Pattern: "/mcp", Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}}
	if err := ValidateRoutes(routes, builtins); err == nil {
		t.Fatal("ValidateRoutes() error = nil, want conflict error")
	}
}
