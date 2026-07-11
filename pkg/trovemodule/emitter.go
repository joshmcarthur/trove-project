package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Emitter appends events to the Trove journal via CoreServices.Emit.
type Emitter interface {
	Emit(ctx context.Context, event *troverpc.Event) error
}

// Runner executes module logic and emits events through the core.
type Runner interface {
	Run(ctx context.Context, emit Emitter) error
}

// HealthChecker reports module liveness for periodic core healthchecks.
type HealthChecker interface {
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

// RunFunc adapts a function to Runner.
type RunFunc func(ctx context.Context, emit Emitter) error

func (f RunFunc) Run(ctx context.Context, emit Emitter) error {
	return f(ctx, emit)
}

type coreEmitter struct {
	client troverpc.CoreServicesClient
}

func (e *coreEmitter) Emit(ctx context.Context, event *troverpc.Event) error {
	_, err := e.client.Emit(ctx, event)
	return err
}
