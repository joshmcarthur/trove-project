package main

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type httpIngestModule struct {
	ready atomic.Bool
	cfg   config
	emit  trovemodule.Emitter
	blobs trovemodule.BlobPutter
}

func (m *httpIngestModule) RunWithBlobs(ctx context.Context, emit trovemodule.Emitter, blobs trovemodule.BlobPutter) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if blobs == nil {
		return fmt.Errorf("http-ingest: blob putter is required")
	}

	m.cfg = cfg
	m.emit = emit
	m.blobs = blobs
	m.ready.Store(true)
	defer m.ready.Store(false)

	<-ctx.Done()
	return nil
}

func (m *httpIngestModule) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if !m.ready.Load() {
		return textResponse(http.StatusServiceUnavailable, "service unavailable"), nil
	}
	return dispatchHTTP(ctx, m.emit, m.blobs, m.cfg, req)
}

func (m *httpIngestModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "http handlers ready"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "http handlers not ready"}, nil
}

func main() {
	trovemodule.Serve(&httpIngestModule{})
}
