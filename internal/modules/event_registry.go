package modules

import (
	"context"
	"sync"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// EventProcessorClient processes journal events in an event-routing module.
type EventProcessorClient interface {
	Process(ctx context.Context, event journal.Event, dispatch DispatchContext) ([]journal.Event, error)
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

// EventSinkClient handles journal events in a sink module.
type EventSinkClient interface {
	Handle(ctx context.Context, event journal.Event, dispatch DispatchContext) error
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

// EventRegistry tracks running event-routing modules.
type EventRegistry struct {
	mu         sync.RWMutex
	processors map[string]eventProcessorBinding
	sinks      map[string]eventSinkBinding
}

type eventProcessorBinding struct {
	name               string
	consumes           []string
	consumesOperations []string
	policy             EmitPolicy
	client             EventProcessorClient
}

type eventSinkBinding struct {
	name               string
	consumes           []string
	consumesOperations []string
	client             EventSinkClient
}

// NewEventRegistry returns an empty event module registry.
func NewEventRegistry() *EventRegistry {
	return &EventRegistry{
		processors: make(map[string]eventProcessorBinding),
		sinks:      make(map[string]eventSinkBinding),
	}
}

// RegisterProcessor adds a processor module to the registry.
func (r *EventRegistry) RegisterProcessor(name string, consumes, consumesOperations []string, policy EmitPolicy, client EventProcessorClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[name] = eventProcessorBinding{
		name:               name,
		consumes:           append([]string(nil), consumes...),
		consumesOperations: defaultConsumesOperations(consumesOperations),
		policy:             policy,
		client:             client,
	}
}

// RegisterSink adds a sink module to the registry.
func (r *EventRegistry) RegisterSink(name string, consumes, consumesOperations []string, client EventSinkClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sinks[name] = eventSinkBinding{
		name:               name,
		consumes:           append([]string(nil), consumes...),
		consumesOperations: defaultConsumesOperations(consumesOperations),
		client:             client,
	}
}

func defaultConsumesOperations(ops []string) []string {
	if len(ops) == 0 {
		return []string{journal.OpApply}
	}
	return append([]string(nil), ops...)
}

// UnregisterProcessor removes a processor module from the registry.
func (r *EventRegistry) UnregisterProcessor(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.processors, name)
}

// UnregisterSink removes a sink module from the registry.
func (r *EventRegistry) UnregisterSink(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sinks, name)
}

func (r *EventRegistry) processorsSnapshot() []eventProcessorBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]eventProcessorBinding, 0, len(r.processors))
	for _, binding := range r.processors {
		out = append(out, binding)
	}
	return out
}

func (r *EventRegistry) sinksSnapshot() []eventSinkBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]eventSinkBinding, 0, len(r.sinks))
	for _, binding := range r.sinks {
		out = append(out, binding)
	}
	return out
}
