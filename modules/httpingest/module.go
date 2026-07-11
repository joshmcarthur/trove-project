package httpingest

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// Module implements HTTP ingest routes for Trove.
type Module struct {
	ready atomic.Bool
	cfg   config
	core  trovemodule.Core
}

// New constructs an http-ingest module instance.
func New() trovemodule.Module {
	return &Module{}
}

func (m *Module) Run(ctx context.Context, core trovemodule.Core) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if core == nil {
		return fmt.Errorf("http-ingest: core connection is required")
	}

	m.cfg = cfg
	m.core = core
	m.ready.Store(true)
	defer m.ready.Store(false)

	<-ctx.Done()
	return nil
}

func (m *Module) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if !m.ready.Load() {
		return textResponse(http.StatusServiceUnavailable, "service unavailable"), nil
	}
	return dispatchHTTP(ctx, m.core, m.core, m.cfg, req)
}

func (m *Module) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "http handlers ready"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "http handlers not ready"}, nil
}
