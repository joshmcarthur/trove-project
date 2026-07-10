package query

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/oklog/ulid"
)

func openTestJournal(t *testing.T) journal.Journal {
	t.Helper()

	path := filepath.Join(t.TempDir(), "trove.db")
	store, err := journal.Open(path)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestGetEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	when := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	blobRef := "sha256:abc123"
	id := ulid.MustNew(ulid.Now(), rand.Reader).String()
	want := journal.Event{
		ID:      id,
		Time:    when,
		Type:    "http.ingest.received",
		Source:  "shortcuts",
		Payload: json.RawMessage(`{"text":"hello"}`),
		BlobRef: &blobRef,
	}
	if err := store.Append(ctx, want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got, err := svc.GetEvent(ctx, id)
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
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

func TestGetEventNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}

	_, err := svc.GetEvent(ctx, "01J0000000000000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetEvent() error = %v, want %v", err, ErrNotFound)
	}
}
