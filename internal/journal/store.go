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

const revisionSelectColumns = `id, time, operation, record_ref, type, schema_ref, source, producer, payload, transforms, blob_ref, recorded_at, sequence, "references"`

const schemaDDL = `
CREATE TABLE IF NOT EXISTS revisions (
  id          TEXT PRIMARY KEY,
  time        TEXT NOT NULL,
  operation   TEXT NOT NULL,
  record_ref  TEXT NOT NULL,
  type        TEXT,
  schema_ref  TEXT NOT NULL,
  source      TEXT NOT NULL,
  producer    TEXT NOT NULL DEFAULT 'unknown',
  payload     TEXT NOT NULL,
  transforms  TEXT NOT NULL DEFAULT '[]',
  blob_ref    TEXT,
  recorded_at TEXT NOT NULL DEFAULT '',
  sequence    INTEGER NOT NULL DEFAULT 0,
  "references" TEXT
);
CREATE INDEX IF NOT EXISTS idx_revisions_time ON revisions(time);
CREATE INDEX IF NOT EXISTS idx_revisions_type ON revisions(type);
CREATE INDEX IF NOT EXISTS idx_revisions_source ON revisions(source);
CREATE INDEX IF NOT EXISTS idx_revisions_operation ON revisions(operation);
CREATE INDEX IF NOT EXISTS idx_revisions_record_ref ON revisions(record_ref);
CREATE INDEX IF NOT EXISTS idx_revisions_record_sequence ON revisions(record_ref, sequence);
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

	if err := migrateSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if _, err := db.Exec(schemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("journal: init schema: %w", err)
	}

	if _, err := db.Exec(recordsSchemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("journal: init records schema: %w", err)
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

// DB returns the underlying SQLite handle for record projection queries.
func (s *Store) DB() *sql.DB {
	return s.db
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

// PruneBefore deletes revisions with time strictly before cutoff and their FTS rows.
// Returns the number of revisions deleted.
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
		DELETE FROM records_fts
		WHERE record_ref IN (
			SELECT DISTINCT record_ref FROM record_revisions
			WHERE revision_id IN (SELECT id FROM revisions WHERE time < ?)
		)`, cutoffStr); err != nil {
		return 0, fmt.Errorf("journal: prune records fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM record_revisions
		WHERE revision_id IN (SELECT id FROM revisions WHERE time < ?)`, cutoffStr); err != nil {
		return 0, fmt.Errorf("journal: prune record_revisions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM record_heads
		WHERE record_ref NOT IN (SELECT DISTINCT record_ref FROM record_revisions)`); err != nil {
		return 0, fmt.Errorf("journal: prune record_heads: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM revisions_fts
		WHERE revision_id IN (SELECT id FROM revisions WHERE time < ?)`, cutoffStr); err != nil {
		return 0, fmt.Errorf("journal: prune fts: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM revisions WHERE time < ?`, cutoffStr)
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
func (s *Store) Append(ctx context.Context, e Revision) error {
	return s.AppendTransactional(ctx, e, nil)
}

func prepareAppend(e *Revision) error {
	if e.Operation == "" {
		e.Operation = OpApply
	}
	if e.Operation != OpApply && e.Operation != OpDelete {
		return fmt.Errorf("journal: append: operation must be %q or %q", OpApply, OpDelete)
	}
	if e.Source == "" {
		return fmt.Errorf("journal: append: source is required")
	}
	if e.Operation == OpDelete && e.RecordRef == "" {
		return fmt.Errorf("journal: append: record_ref is required for delete")
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("journal: append: payload is required")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("journal: append: payload must be valid JSON")
	}
	if len(e.Transforms) == 0 {
		e.Transforms = json.RawMessage(`[]`)
	} else if !json.Valid(e.Transforms) {
		return fmt.Errorf("journal: append: transforms must be valid JSON")
	}
	if e.References != nil && !json.Valid(e.References) {
		return fmt.Errorf("journal: append: references must be valid JSON")
	}

	if e.ID == "" {
		e.ID = ulid.MustNew(ulid.Now(), rand.Reader).String()
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if e.RecordRef == "" {
		e.RecordRef = ulid.MustNew(ulid.Now(), rand.Reader).String()
	}
	if e.RecordedAt.IsZero() {
		e.RecordedAt = time.Now().UTC()
	}
	return nil
}

func appendRevisionInTx(ctx context.Context, tx *sql.Tx, e Revision) error {
	if e.Sequence <= 0 {
		var nextSeq int
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(sequence), 0) + 1 FROM revisions WHERE record_ref = ?`,
			e.RecordRef,
		).Scan(&nextSeq); err != nil {
			return fmt.Errorf("journal: append: assign sequence: %w", err)
		}
		e.Sequence = nextSeq
	}
	var (
		blobRef    sql.NullString
		typ        sql.NullString
		references sql.NullString
	)
	if e.BlobRef != nil {
		blobRef = sql.NullString{String: *e.BlobRef, Valid: true}
	}
	if e.Type != "" {
		typ = sql.NullString{String: e.Type, Valid: true}
	}
	if e.References != nil {
		references = sql.NullString{String: string(e.References), Valid: true}
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO revisions (id, time, operation, record_ref, type, schema_ref, source, producer, payload, transforms, blob_ref, recorded_at, sequence, "references")
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.Time.UTC().Format(time.RFC3339),
		e.Operation,
		e.RecordRef,
		typ,
		e.SchemaRef,
		e.Source,
		e.Producer,
		string(e.Payload),
		string(e.Transforms),
		blobRef,
		e.RecordedAt.UTC().Format(time.RFC3339),
		e.Sequence,
		references,
	)
	if err != nil {
		return fmt.Errorf("journal: append: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO revisions_fts (revision_id, type, source, payload)
		VALUES (?, ?, ?, ?)`,
		e.ID,
		e.Type,
		e.Source,
		string(e.Payload),
	)
	if err != nil {
		return fmt.Errorf("journal: append fts: %w", err)
	}
	return nil
}

// Query returns revisions matching f, ordered by time ascending.
func (s *Store) Query(ctx context.Context, f Filter) ([]Revision, error) {
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
			SELECT e.id, e.time, e.operation, e.record_ref, e.type, e.schema_ref, e.source, e.producer, e.payload, e.transforms, e.blob_ref, e.recorded_at, e.sequence, e."references"
			FROM revisions e
			INNER JOIN revisions_fts ON revisions_fts.revision_id = e.id`
		predicates = append(predicates, "revisions_fts MATCH ?")
		args = append(args, ftsQuery)
	} else {
		query = `
			SELECT ` + revisionSelectColumns + `
			FROM revisions`
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

	var revisions []Revision
	for rows.Next() {
		e, err := scanRevision(rows)
		if err != nil {
			return nil, fmt.Errorf("journal: query: %w", err)
		}
		revisions = append(revisions, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("journal: query: %w", err)
	}

	return revisions, nil
}

// Get returns the revision with id.
func (s *Store) Get(ctx context.Context, id string) (Revision, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+revisionSelectColumns+`
		FROM revisions
		WHERE id = ?`, id)

	e, err := scanRevision(row)
	if err == sql.ErrNoRows {
		return Revision{}, ErrNotFound
	}
	if err != nil {
		return Revision{}, fmt.Errorf("journal: get %q: %w", id, err)
	}

	return e, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRevision(row rowScanner) (Revision, error) {
	var (
		e          Revision
		timeStr    string
		recordedAt string
		payload    string
		transforms string
		typ        sql.NullString
		blobRef    sql.NullString
		references sql.NullString
	)

	if err := row.Scan(
		&e.ID,
		&timeStr,
		&e.Operation,
		&e.RecordRef,
		&typ,
		&e.SchemaRef,
		&e.Source,
		&e.Producer,
		&payload,
		&transforms,
		&blobRef,
		&recordedAt,
		&e.Sequence,
		&references,
	); err != nil {
		return Revision{}, err
	}

	var err error
	e.Time, err = time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return Revision{}, fmt.Errorf("parse time %q: %w", timeStr, err)
	}
	if recordedAt != "" {
		e.RecordedAt, err = time.Parse(time.RFC3339, recordedAt)
		if err != nil {
			return Revision{}, fmt.Errorf("parse recorded_at %q: %w", recordedAt, err)
		}
	}

	if typ.Valid {
		e.Type = typ.String
	}
	e.Payload = json.RawMessage(payload)
	e.Transforms = json.RawMessage(transforms)
	if blobRef.Valid {
		ref := blobRef.String
		e.BlobRef = &ref
	}
	if references.Valid {
		e.References = json.RawMessage(references.String)
	}

	return e, nil
}
