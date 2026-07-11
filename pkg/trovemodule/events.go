package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// EventProcessor handles journal events and may return derived events.
type EventProcessor interface {
	Process(ctx context.Context, event *troverpc.Event, dispatch *troverpc.DispatchContext) ([]*troverpc.Event, error)
}

// EventProcessorFunc adapts a function to EventProcessor.
type EventProcessorFunc func(ctx context.Context, event *troverpc.Event, dispatch *troverpc.DispatchContext) ([]*troverpc.Event, error)

func (f EventProcessorFunc) Process(ctx context.Context, event *troverpc.Event, dispatch *troverpc.DispatchContext) ([]*troverpc.Event, error) {
	return f(ctx, event, dispatch)
}

// EventSink handles journal events without emitting derived events.
type EventSink interface {
	Handle(ctx context.Context, event *troverpc.Event, dispatch *troverpc.DispatchContext) error
}

// EventSinkFunc adapts a function to EventSink.
type EventSinkFunc func(ctx context.Context, event *troverpc.Event, dispatch *troverpc.DispatchContext) error

func (f EventSinkFunc) Handle(ctx context.Context, event *troverpc.Event, dispatch *troverpc.DispatchContext) error {
	return f(ctx, event, dispatch)
}

// WaitCore blocks until ctx is cancelled. Use in Run for event-routing modules
// that do not stream events from Run itself.
func WaitCore(ctx context.Context, _ Core) error {
	<-ctx.Done()
	return ctx.Err()
}
