package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/modules"
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

	blobsPath := os.Getenv(modules.EnvBlobsPath)
	if blobsPath == "" {
		return fmt.Errorf("http-ingest: %s is required (set by trove core from [blobs].path)", modules.EnvBlobsPath)
	}

	blobs, err := blob.OpenFilesystem(blobsPath)
	if err != nil {
		return fmt.Errorf("http-ingest: open blob store: %w", err)
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
