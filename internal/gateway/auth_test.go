package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/modules"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

func TestGatewayAuthRejectsMissingToken(t *testing.T) {
	t.Parallel()

	client := &stubHTTPClient{resp: &troverpc.HTTPResponse{Status: http.StatusNoContent}}
	registry := modules.NewHTTPRegistry()
	registry.Register("http-ingest", client)

	routes := []modules.HTTPRouteEntry{{
		Route:  modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}"},
		Module: "http-ingest",
	}}
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024, AuthToken: "secret"}, routes, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest/shortcuts", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if client.last != nil {
		t.Fatal("HandleHTTP should not be called without auth")
	}
}

func TestGatewayAuthAcceptsBearerToken(t *testing.T) {
	t.Parallel()

	client := &stubHTTPClient{resp: &troverpc.HTTPResponse{Status: http.StatusNoContent}}
	registry := modules.NewHTTPRegistry()
	registry.Register("http-ingest", client)

	routes := []modules.HTTPRouteEntry{{
		Route:  modules.HTTPRoute{Method: "POST", Path: "/ingest/{source}"},
		Module: "http-ingest",
	}}
	gw, err := New(Config{Listen: ":0", MaxBodyBytes: 1024, AuthToken: "secret"}, routes, registry, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest/shortcuts", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	gw.handle(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
