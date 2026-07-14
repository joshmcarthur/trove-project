package modules

import (
	"context"
	"sync"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RevisionProcessorClient processes journal events in an event-routing module.
type RevisionProcessorClient interface {
	Process(ctx context.Context, event journal.Revision, dispatch DispatchContext) ([]journal.Revision, error)
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

// RevisionSinkClient handles journal events in a sink module.
type RevisionSinkClient interface {
	Handle(ctx context.Context, event journal.Revision, dispatch DispatchContext) error
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

// RevisionRegistry tracks running event-routing modules.
type RevisionRegistry struct {
	mu         sync.RWMutex
	processors map[string]revisionProcessorBinding
	sinks      map[string]revisionSinkBinding
}

type revisionProcessorBinding struct {
	name     string
	consumes []string
	policy   EmitPolicy
	client   RevisionProcessorClient
}

type revisionSinkBinding struct {
	name     string
	consumes []string
	client   RevisionSinkClient
}

// NewRevisionRegistry returns an empty event module registry.
func NewRevisionRegistry() *RevisionRegistry {
	return &RevisionRegistry{
		processors: make(map[string]revisionProcessorBinding),
		sinks:      make(map[string]revisionSinkBinding),
	}
}

// RegisterProcessor adds a processor module to the registry.
func (r *RevisionRegistry) RegisterProcessor(name string, consumes []string, policy EmitPolicy, client RevisionProcessorClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[name] = revisionProcessorBinding{
		name:     name,
		consumes: append([]string(nil), consumes...),
		policy:   policy,
		client:   client,
	}
}

// RegisterSink adds a sink module to the registry.
func (r *RevisionRegistry) RegisterSink(name string, consumes []string, client RevisionSinkClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sinks[name] = revisionSinkBinding{
		name:     name,
		consumes: append([]string(nil), consumes...),
		client:   client,
	}
}

// UnregisterProcessor removes a processor module from the registry.
func (r *RevisionRegistry) UnregisterProcessor(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.processors, name)
}

// UnregisterSink removes a sink module from the registry.
func (r *RevisionRegistry) UnregisterSink(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sinks, name)
}

func (r *RevisionRegistry) processorsSnapshot() []revisionProcessorBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]revisionProcessorBinding, 0, len(r.processors))
	for _, binding := range r.processors {
		out = append(out, binding)
	}
	return out
}

func (r *RevisionRegistry) sinksSnapshot() []revisionSinkBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]revisionSinkBinding, 0, len(r.sinks))
	for _, binding := range r.sinks {
		out = append(out, binding)
	}
	return out
}
