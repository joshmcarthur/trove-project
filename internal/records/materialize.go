package records

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS record_heads (
  record_ref   TEXT PRIMARY KEY,
  version      INTEGER NOT NULL,
  completeness TEXT NOT NULL,
  type         TEXT NOT NULL DEFAULT '',
  source       TEXT NOT NULL,
  body         TEXT NOT NULL,
  content_ref  TEXT,
  updated_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS record_events (
  event_id    TEXT PRIMARY KEY,
  record_ref  TEXT NOT NULL,
  version     INTEGER NOT NULL,
  UNIQUE(record_ref, version)
);
CREATE INDEX IF NOT EXISTS idx_record_events_ref ON record_events(record_ref);

CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
  record_ref UNINDEXED,
  type,
  source,
  body,
  tokenize = 'porter'
);
`

// Materializer projects journal record events into record_heads and records_fts.
type Materializer struct {
	tx *sql.Tx
}

// NewMaterializer returns a materializer bound to tx.
func NewMaterializer(tx *sql.Tx) *Materializer {
	return &Materializer{tx: tx}
}

// EnsureSchema creates record projection tables when missing.
func EnsureSchema(db *sql.DB) error {
	if _, err := db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("records: ensure schema: %w", err)
	}
	return nil
}

// Apply materializes e when it is an apply or delete record event.
// Returns true when the event was applied, false when skipped.
func (m *Materializer) Apply(ctx context.Context, e journal.Event) (bool, error) {
	switch e.Operation {
	case journal.OpApply, journal.OpDelete:
	default:
		return false, nil
	}
	if e.RecordRef == "" {
		return false, fmt.Errorf("records: materialize %q: record_ref is required", e.ID)
	}

	applied, err := m.eventApplied(ctx, e.ID)
	if err != nil {
		return false, err
	}
	if applied {
		return false, nil
	}

	prev, found, err := m.loadHead(ctx, e.RecordRef)
	if err != nil {
		return false, err
	}

	head, err := foldHead(e, prev, found)
	if err != nil {
		return false, fmt.Errorf("records: materialize %q: %w", e.ID, err)
	}

	if err := m.writeHead(ctx, head); err != nil {
		return false, err
	}
	if err := m.linkEvent(ctx, e.ID, head.RecordRef, head.Version); err != nil {
		return false, err
	}
	if err := m.syncFTS(ctx, head); err != nil {
		return false, err
	}

	return true, nil
}

// RebuildAll wipes projection tables and replays record events from the journal.
func RebuildAll(ctx context.Context, db *sql.DB) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("records: rebuild: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		`DELETE FROM records_fts`,
		`DELETE FROM record_events`,
		`DELETE FROM record_heads`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("records: rebuild: %s: %w", stmt, err)
		}
	}

	events, err := listRecordEvents(ctx, tx)
	if err != nil {
		return err
	}

	mat := NewMaterializer(tx)
	for _, e := range events {
		if _, err := mat.Apply(ctx, e); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("records: rebuild: commit: %w", err)
	}
	return nil
}

func foldHead(e journal.Event, prev Head, found bool) (Head, error) {
	if e.Operation == journal.OpDelete {
		if !found {
			return Head{}, fmt.Errorf("records: delete: record %q not found", e.RecordRef)
		}
		prev.Version++
		prev.Completeness = CompletenessDeleted
		prev.UpdatedAt = e.Time.UTC()
		return prev, nil
	}

	var previousBody json.RawMessage
	if found {
		previousBody = prev.Body
	} else {
		previousBody = json.RawMessage(`{}`)
	}

	body, err := FoldApply(ApplyInput{
		PreviousBody: previousBody,
		Payload:      e.Payload,
		Transforms:   e.Transforms,
	})
	if err != nil {
		return Head{}, err
	}

	head := Head{
		RecordRef: e.RecordRef,
		Version:   1,
		Source:    e.Source,
		Body:      body,
		UpdatedAt: e.Time.UTC(),
	}
	if found {
		head.Version = prev.Version + 1
		head.Type = prev.Type
		head.ContentRef = prev.ContentRef
	}
	if e.Type != "" {
		head.Type = e.Type
	}
	if e.BlobRef != nil {
		head.ContentRef = e.BlobRef
	}
	head.Completeness = completenessFor(head.Type)
	return head, nil
}

func completenessFor(recordType string) string {
	if recordType != "" {
		return CompletenessComplete
	}
	return CompletenessIncomplete
}

func (m *Materializer) Head(ctx context.Context, recordRef string) (Head, bool, error) {
	return m.loadHead(ctx, recordRef)
}

func (m *Materializer) eventApplied(ctx context.Context, eventID string) (bool, error) {
	var n int
	err := m.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM record_events WHERE event_id = ?`, eventID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("records: check event applied: %w", err)
	}
	return n > 0, nil
}

func (m *Materializer) loadHead(ctx context.Context, recordRef string) (Head, bool, error) {
	row := m.tx.QueryRowContext(ctx, `
		SELECT record_ref, version, completeness, type, source, body, content_ref, updated_at
		FROM record_heads
		WHERE record_ref = ?`, recordRef)

	head, err := scanHead(row)
	if err == sql.ErrNoRows {
		return Head{}, false, nil
	}
	if err != nil {
		return Head{}, false, fmt.Errorf("records: load head %q: %w", recordRef, err)
	}
	return head, true, nil
}

func (m *Materializer) writeHead(ctx context.Context, head Head) error {
	var contentRef sql.NullString
	if head.ContentRef != nil {
		contentRef = sql.NullString{String: *head.ContentRef, Valid: true}
	}

	_, err := m.tx.ExecContext(ctx, `
		INSERT INTO record_heads (record_ref, version, completeness, type, source, body, content_ref, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(record_ref) DO UPDATE SET
			version = excluded.version,
			completeness = excluded.completeness,
			type = excluded.type,
			source = excluded.source,
			body = excluded.body,
			content_ref = excluded.content_ref,
			updated_at = excluded.updated_at`,
		head.RecordRef,
		head.Version,
		head.Completeness,
		head.Type,
		head.Source,
		string(head.Body),
		contentRef,
		head.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("records: write head %q: %w", head.RecordRef, err)
	}
	return nil
}

func (m *Materializer) linkEvent(ctx context.Context, eventID, recordRef string, version int) error {
	_, err := m.tx.ExecContext(ctx, `
		INSERT INTO record_events (event_id, record_ref, version)
		VALUES (?, ?, ?)`,
		eventID, recordRef, version,
	)
	if err != nil {
		return fmt.Errorf("records: link event %q: %w", eventID, err)
	}
	return nil
}

func (m *Materializer) syncFTS(ctx context.Context, head Head) error {
	if _, err := m.tx.ExecContext(ctx, `DELETE FROM records_fts WHERE record_ref = ?`, head.RecordRef); err != nil {
		return fmt.Errorf("records: clear fts %q: %w", head.RecordRef, err)
	}
	if head.Completeness == CompletenessDeleted {
		return nil
	}
	_, err := m.tx.ExecContext(ctx, `
		INSERT INTO records_fts (record_ref, type, source, body)
		VALUES (?, ?, ?, ?)`,
		head.RecordRef,
		head.Type,
		head.Source,
		string(head.Body),
	)
	if err != nil {
		return fmt.Errorf("records: write fts %q: %w", head.RecordRef, err)
	}
	return nil
}

func listRecordEvents(ctx context.Context, q queryer) ([]journal.Event, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, time, operation, record_ref, type, schema_ref, source, payload, transforms, blob_ref
		FROM events
		WHERE operation IN (?, ?)
		ORDER BY time ASC`,
		journal.OpApply,
		journal.OpDelete,
	)
	if err != nil {
		return nil, fmt.Errorf("records: list record events: %w", err)
	}
	defer rows.Close()

	var events []journal.Event
	for rows.Next() {
		e, err := scanRecordEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("records: scan record event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("records: list record events: %w", err)
	}
	return events, nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHead(row rowScanner) (Head, error) {
	var (
		head       Head
		body       string
		updatedAt  string
		contentRef sql.NullString
	)
	if err := row.Scan(
		&head.RecordRef,
		&head.Version,
		&head.Completeness,
		&head.Type,
		&head.Source,
		&body,
		&contentRef,
		&updatedAt,
	); err != nil {
		return Head{}, err
	}

	var err error
	head.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return Head{}, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	head.Body = json.RawMessage(body)
	if contentRef.Valid {
		ref := contentRef.String
		head.ContentRef = &ref
	}
	return head, nil
}

func scanRecordEvent(row rowScanner) (journal.Event, error) {
	var (
		e          journal.Event
		timeStr    string
		operation  string
		payload    string
		transforms sql.NullString
		blobRef    sql.NullString
	)
	if err := row.Scan(
		&e.ID,
		&timeStr,
		&operation,
		&e.RecordRef,
		&e.Type,
		&e.SchemaRef,
		&e.Source,
		&payload,
		&transforms,
		&blobRef,
	); err != nil {
		return journal.Event{}, err
	}

	var err error
	e.Time, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return journal.Event{}, fmt.Errorf("parse time %q: %w", timeStr, err)
	}
	e.Operation = operation
	e.Payload = json.RawMessage(payload)
	if transforms.Valid {
		e.Transforms = json.RawMessage(transforms.String)
	}
	if blobRef.Valid {
		ref := blobRef.String
		e.BlobRef = &ref
	}
	return e, nil
}
