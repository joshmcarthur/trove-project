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

const routerPollInterval = 500 * time.Millisecond

// routingJournal extends Journal with cursor-based dispatch support.
type routingJournal interface {
	journal.Journal
	QueryAfter(ctx context.Context, afterID string, limit int) ([]journal.Revision, error)
	LoadRouterWatermark(ctx context.Context) (string, error)
	SaveRouterWatermark(ctx context.Context, id string) error
	SaveRevisionDispatch(ctx context.Context, eventID, rootID string, seen []string) error
	LoadRevisionDispatch(ctx context.Context, eventID string) (rootID string, seen []string, ok bool, err error)
	DeleteRevisionDispatch(ctx context.Context, eventID string) error
}

// Router dispatches journal events to event-routing processors and sinks.
type Router struct {
	journal  journal.Journal
	registry *RevisionRegistry
	writer   *WriteService
	claims   sync.Map
	pending  sync.Map
}

// NewRouter returns a router for the given journal and event registry.
func NewRouter(j journal.Journal, registry *RevisionRegistry, writer *WriteService) *Router {
	return &Router{
		journal:  j,
		registry: registry,
		writer:   writer,
	}
}

// Run pulls events from the journal in ULID order and dispatches them until ctx
// is cancelled. Watch provides low-latency wakeups; a poll interval is
// the fallback when a coalesced signal is missed.
func (r *Router) Run(ctx context.Context) error {
	routeStore, ok := r.journal.(routingJournal)
	if !ok {
		return fmt.Errorf("modules: router requires routing-capable journal store")
	}

	wakeCh, err := routeStore.Watch(ctx)
	if err != nil {
		return fmt.Errorf("modules: router watch: %w", err)
	}

	watermark, err := routeStore.LoadRouterWatermark(ctx)
	if err != nil {
		return fmt.Errorf("modules: router load watermark: %w", err)
	}

	poll := time.NewTicker(routerPollInterval)
	defer poll.Stop()

	for {
		events, err := routeStore.QueryAfter(ctx, watermark, 1)
		if err != nil {
			return fmt.Errorf("modules: router query after: %w", err)
		}
		if len(events) > 0 {
			event := events[0]
			if err := r.deliver(ctx, routeStore, event); err != nil {
				log.Printf("modules: router dispatch %q: %v", event.ID, err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
				}
				continue
			}
			watermark = event.ID
			if err := routeStore.SaveRouterWatermark(ctx, watermark); err != nil {
				return fmt.Errorf("modules: router save watermark: %w", err)
			}
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wakeCh:
		case <-poll.C:
		}
	}
}

func (r *Router) deliver(ctx context.Context, routeStore routingJournal, event journal.Revision) error {
	if _, loaded := r.claims.LoadOrStore(event.ID, struct{}{}); loaded {
		return nil
	}
	defer r.claims.Delete(event.ID)

	dctx := DispatchContext{RootID: event.ID}
	persistedDispatch := false
	if pending, ok := r.pending.LoadAndDelete(event.ID); ok {
		dctx = pending.(DispatchContext)
		persistedDispatch = true
	} else {
		rootID, seen, ok, err := routeStore.LoadRevisionDispatch(ctx, event.ID)
		if err != nil {
			return err
		}
		if ok {
			dctx = DispatchContext{RootID: rootID, Seen: seen}
			persistedDispatch = true
		}
	}

	if err := r.dispatch(ctx, routeStore, event, dctx); err != nil {
		return err
	}

	if persistedDispatch {
		if err := routeStore.DeleteRevisionDispatch(ctx, event.ID); err != nil {
			return fmt.Errorf("delete event dispatch %q: %w", event.ID, err)
		}
	}
	return nil
}

func (r *Router) dispatch(ctx context.Context, routeStore routingJournal, event journal.Revision, dctx DispatchContext) error {
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
			if out.Operation == "" {
				out.Operation = journal.OpApply
			}
			if err := binding.policy.ValidateApply(&out); err != nil {
				return fmt.Errorf("processor %q derived event: %w", binding.name, err)
			}
			childCtx := DispatchContext{
				RootID: dctx.RootID,
				Seen:   childSeen,
			}
			if err := r.appendDerived(ctx, routeStore, out, childCtx); err != nil {
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

func (r *Router) appendDerived(ctx context.Context, routeStore routingJournal, event journal.Revision, dctx DispatchContext) error {
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
	if err := routeStore.SaveRevisionDispatch(ctx, event.ID, dctx.RootID, dctx.Seen); err != nil {
		r.pending.Delete(event.ID)
		return fmt.Errorf("save event dispatch: %w", err)
	}
	if r.writer == nil {
		return fmt.Errorf("append derived event: record writer is not configured")
	}
	if _, err := r.writer.Write(ctx, event, nil); err != nil {
		r.pending.Delete(event.ID)
		_ = routeStore.DeleteRevisionDispatch(ctx, event.ID)
		return fmt.Errorf("append derived event: %w", err)
	}
	return nil
}
