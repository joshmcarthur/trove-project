package journal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oklog/ulid"
	_ "modernc.org/sqlite"
)

func TestOpenCreatesDatabase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trove.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file missing: %v", err)
	}

	var name string
	err = store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'events'`).Scan(&name)
	if err != nil {
		t.Fatalf("events table missing: %v", err)
	}

	err = store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'events_fts'`).Scan(&name)
	if err != nil {
		t.Fatalf("events_fts table missing: %v", err)
	}
}

func TestAppendAndGet(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.FixedZone("NZST", 12*60*60))
	blobRef := "sha256:abc123"
	id := ulid.MustNew(ulid.Now(), rand.Reader).String()
	want := Event{
		ID:      id,
		Time:    when,
		Type:    "meshtastic.message.received",
		Source:  "radio-node-1",
		Payload: json.RawMessage(`{"text":"hello"}`),
		BlobRef: &blobRef,
	}

	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if _, err := ulid.Parse(id); err != nil {
		t.Fatalf("assigned invalid ULID %q: %v", id, err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if !got.Time.Equal(when.UTC()) {
		t.Errorf("Time = %v, want %v", got.Time, when.UTC())
	}
	if got.Type != want.Type {
		t.Errorf("Type = %q, want %q", got.Type, want.Type)
	}
	if got.Source != want.Source {
		t.Errorf("Source = %q, want %q", got.Source, want.Source)
	}
	if string(got.Payload) != string(want.Payload) {
		t.Errorf("Payload = %s, want %s", got.Payload, want.Payload)
	}
	if got.BlobRef == nil || *got.BlobRef != blobRef {
		t.Errorf("BlobRef = %v, want %q", got.BlobRef, blobRef)
	}
}

func TestAppendDefaults(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	before := time.Now().UTC().Add(-time.Second)
	id := ulid.MustNew(ulid.Now(), rand.Reader).String()
	event := Event{
		ID:      id,
		Type:    "mqtt.test.event",
		Source:  "sensor-1",
		Payload: json.RawMessage(`{"value":42}`),
	}

	if err := store.Append(ctx, event); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	after := time.Now().UTC().Add(time.Second)

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID == "" {
		t.Fatal("stored event has empty ID")
	}
	if got.Time.Before(before) || got.Time.After(after) {
		t.Errorf("Time = %v, want between %v and %v", got.Time, before, after)
	}
	if got.BlobRef != nil {
		t.Errorf("BlobRef = %v, want nil", got.BlobRef)
	}
}

func TestAppendValidation(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	validPayload := json.RawMessage(`{"ok":true}`)

	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "missing type",
			event: Event{
				Source:  "src",
				Payload: validPayload,
			},
		},
		{
			name: "missing source",
			event: Event{
				Type:    "test.event",
				Payload: validPayload,
			},
		},
		{
			name: "missing payload",
			event: Event{
				Type:   "test.event",
				Source: "src",
			},
		},
		{
			name: "invalid payload json",
			event: Event{
				Type:    "test.event",
				Source:  "src",
				Payload: json.RawMessage(`{not-json`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := store.Append(ctx, tt.event); err == nil {
				t.Fatal("Append() error = nil, want validation error")
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "01J0000000000000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestAppendDuplicateID(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	id := ulid.MustNew(ulid.Now(), rand.Reader).String()
	event := Event{
		ID:      id,
		Type:    "test.event",
		Source:  "src",
		Payload: json.RawMessage(`{"n":1}`),
	}

	if err := store.Append(ctx, event); err != nil {
		t.Fatalf("first Append() error = %v", err)
	}

	duplicate := Event{
		ID:      id,
		Type:    "test.event",
		Source:  "src",
		Payload: json.RawMessage(`{"n":2}`),
	}
	if err := store.Append(ctx, duplicate); err == nil {
		t.Fatal("second Append() error = nil, want duplicate key error")
	}
}

func TestQuery(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)

	seed := []Event{
		{ID: "01JEVT00000000000000000001", Time: t1, Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"v":1}`)},
		{ID: "01JEVT00000000000000000002", Time: t2, Type: "mqtt.sensor.humidity", Source: "sensor-a", Payload: json.RawMessage(`{"v":2}`)},
		{ID: "01JEVT00000000000000000003", Time: t3, Type: "ha.light.on", Source: "sensor-b", Payload: json.RawMessage(`{"v":3}`)},
		{ID: "01JEVT00000000000000000004", Time: t4, Type: "mqtt.sensor.temp", Source: "sensor-c", Payload: json.RawMessage(`{"v":4}`)},
	}
	for _, e := range seed {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	tests := []struct {
		name    string
		filter  Filter
		wantIDs []string
	}{
		{
			name:    "empty filter returns all ordered by time",
			filter:  Filter{},
			wantIDs: []string{seed[0].ID, seed[1].ID, seed[2].ID, seed[3].ID},
		},
		{
			name:    "type prefix mqtt.",
			filter:  Filter{TypePrefix: "mqtt."},
			wantIDs: []string{seed[0].ID, seed[1].ID, seed[3].ID},
		},
		{
			name:    "exact type match",
			filter:  Filter{Type: "mqtt.sensor.temp"},
			wantIDs: []string{seed[0].ID, seed[3].ID},
		},
		{
			name: "exact type with time range",
			filter: Filter{
				Type:     "mqtt.sensor.temp",
				TimeFrom: &t1,
				TimeTo:   &t2,
			},
			wantIDs: []string{seed[0].ID},
		},
		{
			name:    "type prefix ha. excludes mqtt",
			filter:  Filter{TypePrefix: "ha."},
			wantIDs: []string{seed[2].ID},
		},
		{
			name:    "source exact match",
			filter:  Filter{Source: "sensor-a"},
			wantIDs: []string{seed[0].ID, seed[1].ID},
		},
		{
			name:    "time from inclusive",
			filter:  Filter{TimeFrom: &t2},
			wantIDs: []string{seed[1].ID, seed[2].ID, seed[3].ID},
		},
		{
			name:    "time to inclusive",
			filter:  Filter{TimeTo: &t3},
			wantIDs: []string{seed[0].ID, seed[1].ID, seed[2].ID},
		},
		{
			name:    "time range",
			filter:  Filter{TimeFrom: &t2, TimeTo: &t3},
			wantIDs: []string{seed[1].ID, seed[2].ID},
		},
		{
			name: "combined filters",
			filter: Filter{
				TypePrefix: "mqtt.",
				Source:     "sensor-a",
				TimeFrom:   &t1,
				TimeTo:     &t2,
			},
			wantIDs: []string{seed[0].ID, seed[1].ID},
		},
		{
			name:    "no matches",
			filter:  Filter{Source: "missing-sensor"},
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := store.Query(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}

			if len(got) != len(tt.wantIDs) {
				t.Fatalf("Query() returned %d events, want %d", len(got), len(tt.wantIDs))
			}

			for i, e := range got {
				if e.ID != tt.wantIDs[i] {
					t.Errorf("event[%d].ID = %q, want %q", i, e.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestQueryText(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)

	seed := []Event{
		{ID: "01JEVT00000000000000000001", Time: t1, Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"reading":"balmy"}`)},
		{ID: "01JEVT00000000000000000002", Time: t2, Type: "mqtt.sensor.humidity", Source: "sensor-a", Payload: json.RawMessage(`{"reading":"dry"}`)},
		{ID: "01JEVT00000000000000000003", Time: t3, Type: "ha.light.on", Source: "kitchen-light", Payload: json.RawMessage(`{"room":"kitchen"}`)},
	}
	for _, e := range seed {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	tests := []struct {
		name    string
		filter  Filter
		wantIDs []string
	}{
		{
			name:    "match keyword in payload",
			filter:  Filter{Text: "balmy"},
			wantIDs: []string{seed[0].ID},
		},
		{
			name:    "match keyword in type",
			filter:  Filter{Text: "humidity"},
			wantIDs: []string{seed[1].ID},
		},
		{
			name:    "match keyword in source",
			filter:  Filter{Text: "kitchen-light"},
			wantIDs: []string{seed[2].ID},
		},
		{
			name: "text with type prefix",
			filter: Filter{
				Text:       "balmy",
				TypePrefix: "mqtt.",
			},
			wantIDs: []string{seed[0].ID},
		},
		{
			name: "text with time range",
			filter: Filter{
				Text:     "reading",
				TimeFrom: &t2,
				TimeTo:   &t3,
			},
			wantIDs: []string{seed[1].ID},
		},
		{
			name:    "no match returns empty",
			filter:  Filter{Text: "missing-keyword"},
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := store.Query(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}

			if len(got) != len(tt.wantIDs) {
				t.Fatalf("Query() returned %d events, want %d", len(got), len(tt.wantIDs))
			}

			for i, e := range got {
				if e.ID != tt.wantIDs[i] {
					t.Errorf("event[%d].ID = %q, want %q", i, e.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestMigrateFTSBackfill(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	if _, err := db.Exec(schemaDDL); err != nil {
		t.Fatalf("create events schema: %v", err)
	}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	_, err = db.Exec(`
		INSERT INTO events (id, time, type, source, payload, blob_ref)
		VALUES (?, ?, ?, ?, ?, NULL)`,
		"01JEVT00000000000000000099",
		when.UTC().Format(time.RFC3339),
		"legacy.event",
		"legacy-source",
		`{"note":"backfill-me"}`,
	)
	if err != nil {
		t.Fatalf("seed legacy event: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	got, err := store.Query(context.Background(), Filter{Text: "backfill-me"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Query() returned %d events, want 1", len(got))
	}
	if got[0].ID != "01JEVT00000000000000000099" {
		t.Errorf("ID = %q, want legacy event id", got[0].ID)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "trove.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func testEvent(id string, when time.Time, typ, source string) Event {
	return Event{
		ID:      id,
		Time:    when,
		Type:    typ,
		Source:  source,
		Payload: json.RawMessage(`{"v":1}`),
	}
}

func recvEvent(t *testing.T, ch <-chan Event, timeout time.Duration) (Event, bool) {
	t.Helper()

	select {
	case e, ok := <-ch:
		return e, ok
	case <-time.After(timeout):
		return Event{}, false
	}
}

func TestSubscribeReceivesMatchingEvent(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	ch, err := store.Subscribe(ctx, Filter{TypePrefix: "mqtt."})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	want := testEvent("01JEVT00000000000000000001", when, "mqtt.sensor.temp", "sensor-a")
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got, ok := recvEvent(t, ch, time.Second)
	if !ok {
		t.Fatal("timed out waiting for subscribed event")
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
}

func TestSubscribeFiltersEvents(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	ch, err := store.Subscribe(ctx, Filter{
		TypePrefix: "mqtt.",
		Source:     "sensor-a",
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	events := []Event{
		testEvent("01JEVT00000000000000000001", when, "ha.light.on", "sensor-a"),
		testEvent("01JEVT00000000000000000002", when, "mqtt.sensor.temp", "sensor-b"),
		testEvent("01JEVT00000000000000000003", when, "mqtt.sensor.temp", "sensor-a"),
	}
	for _, e := range events {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	got, ok := recvEvent(t, ch, time.Second)
	if !ok {
		t.Fatal("timed out waiting for filtered event")
	}
	if got.ID != events[2].ID {
		t.Errorf("ID = %q, want %q", got.ID, events[2].ID)
	}

	if _, ok := recvEvent(t, ch, 100*time.Millisecond); ok {
		t.Fatal("received unexpected extra event")
	}
}

func TestSubscribeNoReplay(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	before := testEvent("01JEVT00000000000000000001", when, "mqtt.sensor.temp", "sensor-a")
	if err := store.Append(ctx, before); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	ch, err := store.Subscribe(ctx, Filter{})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	after := testEvent("01JEVT00000000000000000002", when.Add(time.Minute), "mqtt.sensor.humidity", "sensor-a")
	if err := store.Append(ctx, after); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got, ok := recvEvent(t, ch, time.Second)
	if !ok {
		t.Fatal("timed out waiting for post-subscribe event")
	}
	if got.ID != after.ID {
		t.Errorf("ID = %q, want %q", got.ID, after.ID)
	}

	if _, ok := recvEvent(t, ch, 100*time.Millisecond); ok {
		t.Fatal("received replayed historical event")
	}
}

func TestSubscribeContextCancel(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := store.Subscribe(ctx, Filter{})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel still open after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestSubscribeMultipleSubscribers(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	chA, err := store.Subscribe(ctx, Filter{TypePrefix: "mqtt."})
	if err != nil {
		t.Fatalf("Subscribe(A) error = %v", err)
	}
	chB, err := store.Subscribe(ctx, Filter{Source: "sensor-b"})
	if err != nil {
		t.Fatalf("Subscribe(B) error = %v", err)
	}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	events := []Event{
		testEvent("01JEVT00000000000000000001", when, "mqtt.sensor.temp", "sensor-a"),
		testEvent("01JEVT00000000000000000002", when.Add(time.Minute), "ha.light.on", "sensor-b"),
	}
	for _, e := range events {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	gotA, ok := recvEvent(t, chA, time.Second)
	if !ok {
		t.Fatal("subscriber A timed out")
	}
	if gotA.ID != events[0].ID {
		t.Errorf("subscriber A ID = %q, want %q", gotA.ID, events[0].ID)
	}

	gotB, ok := recvEvent(t, chB, time.Second)
	if !ok {
		t.Fatal("subscriber B timed out")
	}
	if gotB.ID != events[1].ID {
		t.Errorf("subscriber B ID = %q, want %q", gotB.ID, events[1].ID)
	}

	if _, ok := recvEvent(t, chA, 100*time.Millisecond); ok {
		t.Fatal("subscriber A received unexpected event")
	}
	if _, ok := recvEvent(t, chB, 100*time.Millisecond); ok {
		t.Fatal("subscriber B received unexpected event")
	}
}

func TestSubscribeConflictingFilter(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	_, err := store.Subscribe(ctx, Filter{
		Type:       "mqtt.sensor.temp",
		TypePrefix: "mqtt.",
	})
	if !errors.Is(err, ErrConflictingFilter) {
		t.Fatalf("Subscribe() error = %v, want ErrConflictingFilter", err)
	}
}

func TestSubscribeFiltersByText(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	ch, err := store.Subscribe(ctx, Filter{Text: "kitchen"})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	events := []Event{
		testEvent("01JEVT00000000000000000001", when, "mqtt.sensor.temp", "sensor-a"),
		testEvent("01JEVT00000000000000000002", when, "ha.light.on", "kitchen-light"),
	}
	events[0].Payload = json.RawMessage(`{"room":"bedroom"}`)
	events[1].Payload = json.RawMessage(`{"room":"kitchen"}`)

	for _, e := range events {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	got, ok := recvEvent(t, ch, time.Second)
	if !ok {
		t.Fatal("timed out waiting for text-filtered event")
	}
	if got.ID != events[1].ID {
		t.Errorf("ID = %q, want %q", got.ID, events[1].ID)
	}

	if _, ok := recvEvent(t, ch, 100*time.Millisecond); ok {
		t.Fatal("received unexpected extra event")
	}
}

func TestSubscribeFiltersByTextAndType(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	ch, err := store.Subscribe(ctx, Filter{
		Text:       "balmy",
		TypePrefix: "mqtt.",
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	events := []Event{
		{ID: "01JEVT00000000000000000001", Time: when, Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"reading":"balmy"}`)},
		{ID: "01JEVT00000000000000000002", Time: when, Type: "mqtt.sensor.humidity", Source: "sensor-a", Payload: json.RawMessage(`{"reading":"dry"}`)},
		{ID: "01JEVT00000000000000000003", Time: when, Type: "ha.light.on", Source: "kitchen-light", Payload: json.RawMessage(`{"reading":"balmy"}`)},
	}
	for _, e := range events {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	got, ok := recvEvent(t, ch, time.Second)
	if !ok {
		t.Fatal("timed out waiting for combined-filter event")
	}
	if got.ID != events[0].ID {
		t.Errorf("ID = %q, want %q", got.ID, events[0].ID)
	}

	if _, ok := recvEvent(t, ch, 100*time.Millisecond); ok {
		t.Fatal("received unexpected extra event")
	}
}

func TestPruneBefore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	if err := store.Append(ctx, Event{
		ID: "01JOLD0000000000000000001", Time: oldTime, Type: "test.old", Source: "test", Payload: json.RawMessage(`{"v":1}`),
	}); err != nil {
		t.Fatalf("Append(old) error = %v", err)
	}
	if err := store.Append(ctx, Event{
		ID: "01JNEW0000000000000000002", Time: newTime, Type: "test.new", Source: "test", Payload: json.RawMessage(`{"v":2}`),
	}); err != nil {
		t.Fatalf("Append(new) error = %v", err)
	}

	n, err := store.PruneBefore(ctx, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("PruneBefore() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("PruneBefore() deleted %d rows, want 1", n)
	}

	events, err := store.Query(ctx, Filter{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 1 || events[0].ID != "01JNEW0000000000000000002" {
		t.Fatalf("Query() = %+v, want only new event", events)
	}
}
