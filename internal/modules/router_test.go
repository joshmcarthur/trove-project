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
	}, t.TempDir(), false)
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
