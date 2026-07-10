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
	want := Event{
		Time:    when,
		Type:    "meshtastic.message.received",
		Source:  "radio-node-1",
		Payload: json.RawMessage(`{"text":"hello"}`),
		BlobRef: &blobRef,
	}

	if err := store.Append(ctx, &want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if want.ID == "" {
		t.Fatal("Append() did not assign ULID")
	}
	if _, err := ulid.Parse(want.ID); err != nil {
		t.Fatalf("Append() assigned invalid ULID %q: %v", want.ID, err)
	}

	got, err := store.Get(ctx, want.ID)
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

	before := time.Now().UTC()
	event := Event{
		Type:    "mqtt.test.event",
		Source:  "sensor-1",
		Payload: json.RawMessage(`{"value":42}`),
	}

	if err := store.Append(ctx, &event); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if event.ID == "" {
		t.Fatal("Append() did not assign ULID")
	}
	if event.Time.Before(before) {
		t.Errorf("Time = %v, want >= %v", event.Time, before)
	}

	got, err := store.Get(ctx, event.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
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
		event *Event
	}{
		{
			name:  "nil event",
			event: nil,
		},
		{
			name: "missing type",
			event: &Event{
				Source:  "src",
				Payload: validPayload,
			},
		},
		{
			name: "missing source",
			event: &Event{
				Type:    "test.event",
				Payload: validPayload,
			},
		},
		{
			name: "missing payload",
			event: &Event{
				Type:   "test.event",
				Source: "src",
			},
		},
		{
			name: "invalid payload json",
			event: &Event{
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

	if err := store.Append(ctx, &event); err != nil {
		t.Fatalf("first Append() error = %v", err)
	}

	duplicate := Event{
		ID:      id,
		Type:    "test.event",
		Source:  "src",
		Payload: json.RawMessage(`{"n":2}`),
	}
	if err := store.Append(ctx, &duplicate); err == nil {
		t.Fatal("second Append() error = nil, want duplicate key error")
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
