package journal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid"
	_ "modernc.org/sqlite"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS events (
  id        TEXT PRIMARY KEY,
  time      TEXT NOT NULL,
  type      TEXT NOT NULL,
  source    TEXT NOT NULL,
  payload   TEXT NOT NULL,
  blob_ref  TEXT
);
CREATE INDEX IF NOT EXISTS idx_events_time ON events(time);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
`

// Store is a SQLite-backed journal.
type Store struct {
	db *sql.DB
}

// Open opens or creates the journal database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("journal: open %q: %w", path, err)
	}

	if _, err := db.Exec(schemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("journal: init schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Append persists e, generating ID and Time when unset.
func (s *Store) Append(ctx context.Context, e *Event) error {
	if e == nil {
		return fmt.Errorf("journal: append: event is nil")
	}
	if e.Type == "" {
		return fmt.Errorf("journal: append: type is required")
	}
	if e.Source == "" {
		return fmt.Errorf("journal: append: source is required")
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("journal: append: payload is required")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("journal: append: payload must be valid JSON")
	}

	if e.ID == "" {
		e.ID = ulid.MustNew(ulid.Now(), rand.Reader).String()
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}

	var blobRef sql.NullString
	if e.BlobRef != nil {
		blobRef = sql.NullString{String: *e.BlobRef, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO events (id, time, type, source, payload, blob_ref)
		VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.Time.UTC().Format(time.RFC3339),
		e.Type,
		e.Source,
		string(e.Payload),
		blobRef,
	)
	if err != nil {
		return fmt.Errorf("journal: append: %w", err)
	}

	return nil
}

// Get returns the event with id.
func (s *Store) Get(ctx context.Context, id string) (Event, error) {
	var (
		e       Event
		timeStr string
		payload string
		blobRef sql.NullString
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, time, type, source, payload, blob_ref
		FROM events
		WHERE id = ?`, id).Scan(
		&e.ID,
		&timeStr,
		&e.Type,
		&e.Source,
		&payload,
		&blobRef,
	)
	if err == sql.ErrNoRows {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("journal: get %q: %w", id, err)
	}

	e.Time, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return Event{}, fmt.Errorf("journal: get %q: parse time: %w", id, err)
	}

	e.Payload = json.RawMessage(payload)
	if blobRef.Valid {
		ref := blobRef.String
		e.BlobRef = &ref
	}

	return e, nil
}
