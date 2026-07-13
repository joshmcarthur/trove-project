package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RevisionProcessor handles journal events and may return derived events.
type RevisionProcessor interface {
	Process(ctx context.Context, event *troverpc.Revision, dispatch *troverpc.DispatchContext) ([]*troverpc.Revision, error)
}

// RevisionProcessorFunc adapts a function to RevisionProcessor.
type RevisionProcessorFunc func(ctx context.Context, event *troverpc.Revision, dispatch *troverpc.DispatchContext) ([]*troverpc.Revision, error)

func (f RevisionProcessorFunc) Process(ctx context.Context, event *troverpc.Revision, dispatch *troverpc.DispatchContext) ([]*troverpc.Revision, error) {
	return f(ctx, event, dispatch)
}

// RevisionSink handles journal events without emitting derived events.
type RevisionSink interface {
	Handle(ctx context.Context, event *troverpc.Revision, dispatch *troverpc.DispatchContext) error
}

// RevisionSinkFunc adapts a function to RevisionSink.
type RevisionSinkFunc func(ctx context.Context, event *troverpc.Revision, dispatch *troverpc.DispatchContext) error

func (f RevisionSinkFunc) Handle(ctx context.Context, event *troverpc.Revision, dispatch *troverpc.DispatchContext) error {
	return f(ctx, event, dispatch)
}

// WaitCore blocks until ctx is cancelled. Use in Run for revision-routing modules
// that do not stream revisions from Run itself.
func WaitCore(ctx context.Context, _ Core) error {
	<-ctx.Done()
	return ctx.Err()
}

// HealthChecker reports module liveness for periodic parent healthchecks.
type HealthChecker interface {
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}
