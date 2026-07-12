package journal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid"
	_ "modernc.org/sqlite"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS events (
  id         TEXT PRIMARY KEY,
  time       TEXT NOT NULL,
  type       TEXT NOT NULL,
  schema_ref TEXT NOT NULL,
  source     TEXT NOT NULL,
  payload    TEXT NOT NULL,
  blob_ref   TEXT
);
CREATE INDEX IF NOT EXISTS idx_events_time ON events(time);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
`

var _ Journal = (*Store)(nil)

// Store is a SQLite-backed journal.
type Store struct {
	db             *sql.DB
	mu             sync.Mutex
	appendWatchers []chan struct{}
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

	if err := migrateSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if _, err := db.Exec(routingSchemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("journal: init routing schema: %w", err)
	}

	if _, err := db.Exec(ftsSchemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("journal: init fts schema: %w", err)
	}

	if err := migrateFTS(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := configureDB(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func configureDB(db *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("journal: configure db: %w", err)
		}
	}
	return nil
}

// Close closes active subscriptions and the underlying database.
func (s *Store) Close() error {
	s.mu.Lock()
	for _, ch := range s.appendWatchers {
		close(ch)
	}
	s.appendWatchers = nil
	s.mu.Unlock()

	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// PruneBefore deletes events with time strictly before cutoff and their FTS rows.
// Returns the number of events deleted.
func (s *Store) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("journal: prune: store is closed")
	}
	cutoff = cutoff.UTC()
	cutoffStr := cutoff.Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("journal: prune: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM events_fts
		WHERE event_id IN (SELECT id FROM events WHERE time < ?)`, cutoffStr); err != nil {
		return 0, fmt.Errorf("journal: prune fts: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM events WHERE time < ?`, cutoffStr)
	if err != nil {
		return 0, fmt.Errorf("journal: prune events: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("journal: prune rows affected: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("journal: prune: commit: %w", err)
	}
	return n, nil
}

// Append persists e, generating ID and Time when unset.
func (s *Store) Append(ctx context.Context, e Event) error {
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("journal: append: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events (id, time, type, schema_ref, source, payload, blob_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.Time.UTC().Format(time.RFC3339),
		e.Type,
		e.SchemaRef,
		e.Source,
		string(e.Payload),
		blobRef,
	)
	if err != nil {
		return fmt.Errorf("journal: append: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events_fts (event_id, type, source, payload)
		VALUES (?, ?, ?, ?)`,
		e.ID,
		e.Type,
		e.Source,
		string(e.Payload),
	)
	if err != nil {
		return fmt.Errorf("journal: append fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("journal: append: commit: %w", err)
	}

	s.signalWatchers()
	return nil
}

// Query returns events matching f, ordered by time ascending.
func (s *Store) Query(ctx context.Context, f Filter) ([]Event, error) {
	if f.Type != "" && f.TypePrefix != "" {
		return nil, ErrConflictingFilter
	}

	var (
		query      string
		predicates []string
		args       []any
	)

	ftsQuery := formatFTSQuery(f.Text)
	if ftsQuery != "" {
		query = `
			SELECT e.id, e.time, e.type, e.schema_ref, e.source, e.payload, e.blob_ref
			FROM events e
			INNER JOIN events_fts ON events_fts.event_id = e.id`
		predicates = append(predicates, "events_fts MATCH ?")
		args = append(args, ftsQuery)
	} else {
		query = `
			SELECT id, time, type, schema_ref, source, payload, blob_ref
			FROM events`
	}

	tablePrefix := ""
	if ftsQuery != "" {
		tablePrefix = "e."
	}

	if f.Type != "" {
		predicates = append(predicates, tablePrefix+"type = ?")
		args = append(args, f.Type)
	}
	if f.TypePrefix != "" {
		predicates = append(predicates, tablePrefix+"type LIKE ?")
		args = append(args, f.TypePrefix+"%")
	}
	if f.Source != "" {
		predicates = append(predicates, tablePrefix+"source = ?")
		args = append(args, f.Source)
	}
	if f.TimeFrom != nil {
		predicates = append(predicates, tablePrefix+"time >= ?")
		args = append(args, f.TimeFrom.UTC().Format(time.RFC3339))
	}
	if f.TimeTo != nil {
		predicates = append(predicates, tablePrefix+"time <= ?")
		args = append(args, f.TimeTo.UTC().Format(time.RFC3339))
	}

	if len(predicates) > 0 {
		// Predicates are fixed fragments; user input is passed only via args.
		query += " WHERE " + strings.Join(predicates, " AND ") //nolint:gosec // G202
	}
	if ftsQuery != "" {
		query += " ORDER BY e.time ASC"
	} else {
		query += " ORDER BY time ASC"
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("journal: query: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("journal: query: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("journal: query: %w", err)
	}

	return events, nil
}

// Get returns the event with id.
func (s *Store) Get(ctx context.Context, id string) (Event, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, time, type, schema_ref, source, payload, blob_ref
		FROM events
		WHERE id = ?`, id)

	e, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("journal: get %q: %w", id, err)
	}

	return e, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (Event, error) {
	var (
		e       Event
		timeStr string
		payload string
		blobRef sql.NullString
	)

	if err := row.Scan(
		&e.ID,
		&timeStr,
		&e.Type,
		&e.SchemaRef,
		&e.Source,
		&payload,
		&blobRef,
	); err != nil {
		return Event{}, err
	}

	var err error
	e.Time, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return Event{}, fmt.Errorf("parse time %q: %w", timeStr, err)
	}

	e.Payload = json.RawMessage(payload)
	if blobRef.Valid {
		ref := blobRef.String
		e.BlobRef = &ref
	}

	return e, nil
}
