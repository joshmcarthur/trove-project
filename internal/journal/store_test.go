package journal

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oklog/ulid"
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
