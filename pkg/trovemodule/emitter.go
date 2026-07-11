package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Emitter appends events to the Trove journal via Core.Emit.
type Emitter interface {
	Emit(ctx context.Context, event *troverpc.Event) error
}

// HealthChecker reports module liveness for periodic parent healthchecks.
type HealthChecker interface {
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}
