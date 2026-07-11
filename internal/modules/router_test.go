package modules

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

type stubProcessor struct {
	fn func(event journal.Event, dispatch DispatchContext) ([]journal.Event, error)
}

func (s stubProcessor) Process(ctx context.Context, event journal.Event, dispatch DispatchContext) ([]journal.Event, error) {
	return s.fn(event, dispatch)
}

func (s stubProcessor) Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error) {
	return &troverpc.HealthcheckResponse{Ok: true}, nil
}

func testPolicy(t *testing.T, name string, provides []string) IngestPolicy {
	t.Helper()
	policy, err := LoadIngestPolicy(Manifest{
		Name:     name,
		Version:  "1.0",
		Kind:     KindProcessor,
		Provides: provides,
		Consumes: []string{"unused"},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("LoadIngestPolicy() error = %v", err)
	}
	return policy
}

func TestRouterProcessorChain(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	defer store.Close()

	registry := NewEventRegistry()
	registry.RegisterProcessor("step-a", []string{"test.input"}, testPolicy(t, "step-a", []string{"test.step.a"}), stubProcessor{
		fn: func(event journal.Event, dispatch DispatchContext) ([]journal.Event, error) {
			return []journal.Event{{
				Type:    "test.step.a",
				Source:  "step-a",
				Payload: event.Payload,
			}}, nil
		},
	})
	registry.RegisterProcessor("step-b", []string{"test.step.a"}, testPolicy(t, "step-b", []string{"test.step.b"}), stubProcessor{
		fn: func(event journal.Event, dispatch DispatchContext) ([]journal.Event, error) {
			return []journal.Event{{
				Type:    "test.step.b",
				Source:  "step-b",
				Payload: event.Payload,
			}}, nil
		},
	})

	router := NewRouter(store, registry)
	go func() {
		_ = router.Run(ctx)
	}()

	err = store.Append(ctx, journal.Event{
		Type:    "test.input",
		Source:  "test",
		Payload: json.RawMessage(`{"n":1}`),
	})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events, err := store.Query(ctx, journal.Filter{Type: "test.step.b"})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for processor chain")
}

func TestRouterSuppressesLoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	defer store.Close()

	var calls atomic.Int32
	registry := NewEventRegistry()
	registry.RegisterProcessor("looper", []string{"test.loop"}, testPolicy(t, "looper", []string{"test.loop"}), stubProcessor{
		fn: func(event journal.Event, dispatch DispatchContext) ([]journal.Event, error) {
			calls.Add(1)
			return []journal.Event{{
				Type:    "test.loop",
				Source:  "looper",
				Payload: event.Payload,
			}}, nil
		},
	})

	router := NewRouter(store, registry)
	go func() {
		_ = router.Run(ctx)
	}()

	err = store.Append(ctx, journal.Event{
		Type:    "test.loop",
		Source:  "test",
		Payload: json.RawMessage(`{"n":1}`),
	})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("processor calls = %d, want 1", calls.Load())
	}

	events, err := store.Query(ctx, journal.Filter{Type: "test.loop"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2 (original + one derived)", len(events))
	}
}

type stubSink struct {
	fn func(event journal.Event, dispatch DispatchContext) error
}

func (s stubSink) Handle(ctx context.Context, event journal.Event, dispatch DispatchContext) error {
	return s.fn(event, dispatch)
}

func (s stubSink) Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error) {
	return &troverpc.HealthcheckResponse{Ok: true}, nil
}

func waitFor(t *testing.T, deadline time.Duration, fn func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestRouterStartupCatchUp(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	defer store.Close()

	appendCtx := context.Background()
	for range 5 {
		if err := store.Append(appendCtx, journal.Event{
			Type:    "test.catchup",
			Source:  "test",
			Payload: json.RawMessage(`{"n":1}`),
		}); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	var handled atomic.Int32
	registry := NewEventRegistry()
	registry.RegisterSink("counter", []string{"test.catchup"}, stubSink{
		fn: func(event journal.Event, dispatch DispatchContext) error {
			handled.Add(1)
			return nil
		},
	})

	router := NewRouter(store, registry)
	go func() {
		_ = router.Run(ctx)
	}()

	waitFor(t, 2*time.Second, func() bool { return handled.Load() == 5 })

	events, err := store.Query(appendCtx, journal.Filter{Type: "test.catchup"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("Query() len = %d, want 5", len(events))
	}
	watermark, err := store.LoadRouterWatermark(ctx)
	if err != nil {
		t.Fatalf("LoadRouterWatermark() error = %v", err)
	}
	if watermark != events[len(events)-1].ID {
		t.Fatalf("watermark = %q, want %q", watermark, events[len(events)-1].ID)
	}
}

func TestRouterCatchesUpViaPollAfterPubSubDrop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	defer store.Close()

	// Block a subscriber channel so notify drops for new appends.
	blockCh, err := store.Subscribe(ctx, journal.Filter{})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	for range 33 {
		if err := store.Append(ctx, journal.Event{
			Type:    "test.block",
			Source:  "test",
			Payload: json.RawMessage(`{"fill":true}`),
		}); err != nil {
			t.Fatalf("Append(block) error = %v", err)
		}
	}
	_ = blockCh

	var handled atomic.Int32
	registry := NewEventRegistry()
	registry.RegisterSink("counter", []string{"test.catchup-drop"}, stubSink{
		fn: func(event journal.Event, dispatch DispatchContext) error {
			handled.Add(1)
			return nil
		},
	})

	router := NewRouter(store, registry)
	go func() {
		_ = router.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	if err := store.Append(ctx, journal.Event{
		Type:    "test.catchup-drop",
		Source:  "test",
		Payload: json.RawMessage(`{"n":1}`),
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	waitFor(t, 3*time.Second, func() bool { return handled.Load() == 1 })

	events, err := store.Query(ctx, journal.Filter{Type: "test.catchup-drop"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Query() len = %d, want 1", len(events))
	}
	watermark, err := store.LoadRouterWatermark(ctx)
	if err != nil {
		t.Fatalf("LoadRouterWatermark() error = %v", err)
	}
	if watermark != events[0].ID {
		t.Fatalf("watermark = %q, want %q", watermark, events[0].ID)
	}
}

func TestRouterRestartPreservesDerivedDispatchContext(t *testing.T) {
	t.Parallel()

	appendCtx := context.Background()
	store, err := journal.Open(t.TempDir() + "/journal.db")
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	defer store.Close()

	const (
		sourceID  = "01JAAA0000000000000000001"
		derivedID = "01JBBB0000000000000000002"
	)

	if err := store.Append(appendCtx, journal.Event{
		ID: sourceID, Type: "test.loop", Source: "test", Payload: json.RawMessage(`{"n":1}`),
	}); err != nil {
		t.Fatalf("Append(source) error = %v", err)
	}
	if err := store.SaveEventDispatch(appendCtx, derivedID, sourceID, []string{"looper"}); err != nil {
		t.Fatalf("SaveEventDispatch() error = %v", err)
	}
	if err := store.Append(appendCtx, journal.Event{
		ID: derivedID, Type: "test.loop", Source: "looper", Payload: json.RawMessage(`{"n":2}`),
	}); err != nil {
		t.Fatalf("Append(derived) error = %v", err)
	}
	if err := store.SaveRouterWatermark(appendCtx, sourceID); err != nil {
		t.Fatalf("SaveRouterWatermark() error = %v", err)
	}

	var calls atomic.Int32
	registry := NewEventRegistry()
	registry.RegisterProcessor("looper", []string{"test.loop"}, testPolicy(t, "looper", []string{"test.loop"}), stubProcessor{
		fn: func(event journal.Event, dispatch DispatchContext) ([]journal.Event, error) {
			calls.Add(1)
			return nil, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := NewRouter(store, registry)
	go func() {
		_ = router.Run(ctx)
	}()

	waitFor(t, 2*time.Second, func() bool {
		watermark, err := store.LoadRouterWatermark(ctx)
		return err == nil && watermark == derivedID
	})

	if calls.Load() != 0 {
		t.Fatalf("processor calls = %d, want 0 (derived dispatch should honor persisted seen)", calls.Load())
	}
}
