package modules

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/oklog/ulid"
)

// Router dispatches journal events to event-routing processors and sinks.
type Router struct {
	journal  journal.Journal
	registry *EventRegistry
	claims   sync.Map
	pending  sync.Map
}

// NewRouter returns a router for the given journal and event registry.
func NewRouter(j journal.Journal, registry *EventRegistry) *Router {
	return &Router{
		journal:  j,
		registry: registry,
	}
}

// Run subscribes to the journal and dispatches events until ctx is cancelled.
func (r *Router) Run(ctx context.Context) error {
	ch, err := r.journal.Subscribe(ctx, journal.Filter{})
	if err != nil {
		return fmt.Errorf("modules: router subscribe: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			r.deliver(ctx, event)
		}
	}
}

func (r *Router) deliver(ctx context.Context, event journal.Event) {
	if _, loaded := r.claims.LoadOrStore(event.ID, struct{}{}); loaded {
		return
	}
	defer r.claims.Delete(event.ID)

	dctx := DispatchContext{RootID: event.ID}
	if pending, ok := r.pending.LoadAndDelete(event.ID); ok {
		dctx = pending.(DispatchContext)
	}
	if err := r.dispatch(ctx, event, dctx); err != nil {
		log.Printf("modules: router dispatch %q: %v", event.ID, err)
	}
}

func (r *Router) dispatch(ctx context.Context, event journal.Event, dctx DispatchContext) error {
	if dctx.RootID == "" {
		dctx.RootID = event.ID
	}

	for _, binding := range r.registry.processorsSnapshot() {
		if !MatchType(binding.consumes, event.Type) {
			continue
		}
		if seenContains(dctx.Seen, binding.name) {
			continue
		}

		derived, err := binding.client.Process(ctx, event, dctx)
		if err != nil {
			return fmt.Errorf("processor %q: %w", binding.name, err)
		}

		childSeen := withSeen(dctx.Seen, binding.name)
		for _, out := range derived {
			if err := binding.policy.ValidateEvent(out); err != nil {
				return fmt.Errorf("processor %q derived event: %w", binding.name, err)
			}
			childCtx := DispatchContext{
				RootID: dctx.RootID,
				Seen:   childSeen,
			}
			if err := r.appendDerived(ctx, out, childCtx); err != nil {
				return err
			}
		}
	}

	for _, binding := range r.registry.sinksSnapshot() {
		if !MatchType(binding.consumes, event.Type) {
			continue
		}
		if seenContains(dctx.Seen, binding.name) {
			continue
		}
		if err := binding.client.Handle(ctx, event, dctx); err != nil {
			return fmt.Errorf("sink %q: %w", binding.name, err)
		}
	}

	return nil
}

func (r *Router) appendDerived(ctx context.Context, event journal.Event, dctx DispatchContext) error {
	if event.ID == "" {
		event.ID = ulid.MustNew(ulid.Now(), rand.Reader).String()
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if dctx.RootID == "" {
		dctx.RootID = event.ID
	}
	r.pending.Store(event.ID, dctx)
	if err := r.journal.Append(ctx, event); err != nil {
		r.pending.Delete(event.ID)
		return fmt.Errorf("append derived event: %w", err)
	}
	return nil
}
