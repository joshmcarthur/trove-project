package main

import (
	"context"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type httpIngestModule struct {
	ready atomic.Bool
}

func (m *httpIngestModule) Run(ctx context.Context, emit trovemodule.Emitter) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	blobs, err := openBlobStore()
	if err != nil {
		return err
	}
	m.ready.Store(true)
	defer m.ready.Store(false)
	return runHTTPServer(ctx, emit, cfg, blobs)
}

func (m *httpIngestModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "http server listening"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "http server not listening"}, nil
}

func main() {
	trovemodule.Serve(&httpIngestModule{})
}
