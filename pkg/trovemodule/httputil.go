package trovemodule

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// ServeHTTPViaRPC adapts an http.Handler to the HandleHTTP RPC contract.
func ServeHTTPViaRPC(handler http.Handler, req *troverpc.HTTPRequest) *troverpc.HTTPResponse {
	if req == nil {
		return &troverpc.HTTPResponse{Status: http.StatusBadRequest, Body: []byte("request is required")}
	}

	httpReq, err := http.NewRequest(req.Method, req.Path, bytes.NewReader(req.Body))
	if err != nil {
		return &troverpc.HTTPResponse{Status: http.StatusBadRequest, Body: []byte("invalid request")}
	}
	if httpReq.URL, err = url.Parse(req.Path); err != nil {
		return &troverpc.HTTPResponse{Status: http.StatusBadRequest, Body: []byte("invalid path")}
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	rec := &responseRecorder{header: make(http.Header), status: http.StatusOK}
	handler.ServeHTTP(rec, httpReq)

	headers := make(map[string]string, len(rec.header))
	for key, values := range rec.header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &troverpc.HTTPResponse{
		Status:  int32(rec.status),
		Headers: headers,
		Body:    rec.body.Bytes(),
	}
}

type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

// RouteKey returns the gateway dispatch key for an HTTP request.
func RouteKey(method, matchedPattern string) string {
	return strings.ToUpper(method) + " " + matchedPattern
}
