package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Emitter appends events to the Trove journal via Core.Emit.
type Emitter interface {
	Emit(ctx context.Context, event *troverpc.Event) error
}

// Runner is a legacy module entry point that only uses Core.Emit.
// Prefer implementing Module with Run(ctx, core Core).
type Runner interface {
	Run(ctx context.Context, emit Emitter) error
}

// RunFunc adapts a function to Runner.
type RunFunc func(ctx context.Context, emit Emitter) error

func (f RunFunc) Run(ctx context.Context, emit Emitter) error {
	return f(ctx, emit)
}

// HealthChecker reports module liveness for periodic parent healthchecks.
type HealthChecker interface {
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}
