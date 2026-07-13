package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/internal/records"
)

const defaultRecordLimit = 100

// Record is a JSON-serializable folded record returned by the query API.
type Record struct {
	RecordRef    string          `json:"record_ref"`
	Version      int             `json:"version"`
	Completeness string          `json:"completeness"`
	Type         string          `json:"type,omitempty"`
	Source       string          `json:"source"`
	Body         json.RawMessage `json:"body"`
	ContentRef   *string         `json:"content_ref,omitempty"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// RecordSearchParams optionally narrows a record FTS search.
type RecordSearchParams struct {
	TypePrefix     string
	Source         string
	TimeFrom       *time.Time
	TimeTo         *time.Time
	IncludeDeleted bool
}

// ListRecordsParams filters list_records results.
type ListRecordsParams struct {
	TypePrefix   string
	Source       string
	Completeness string
	TimeFrom     *time.Time
	TimeTo       *time.Time
	Limit        int
}

// RecordService implements record queries over record_heads and records_fts.
type RecordService struct {
	DB *sql.DB
}

// GetRecord returns the record head for recordRef. When version is non-zero it
// must match the current head version.
func (s *RecordService) GetRecord(ctx context.Context, recordRef string, version int) (Record, error) {
	recordRef = strings.TrimSpace(recordRef)
	if recordRef == "" {
		return Record{}, ErrEmptyRecordRef
	}
	if s.DB == nil {
		return Record{}, fmt.Errorf("query: record store is not configured")
	}

	row := s.DB.QueryRowContext(ctx, `
		SELECT record_ref, version, completeness, type, source, body, content_ref, updated_at
		FROM record_heads
		WHERE record_ref = ?`, recordRef)

	rec, err := scanRecord(row)
	if err == sql.ErrNoRows {
		return Record{}, ErrRecordNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("query: get record %q: %w", recordRef, err)
	}
	if version > 0 && rec.Version != version {
		return Record{}, ErrRecordNotFound
	}
	return rec, nil
}

// SearchRecords performs FTS5 keyword search over records_fts. Deleted records
// are excluded unless IncludeDeleted is set.
func (s *RecordService) SearchRecords(ctx context.Context, text string, params RecordSearchParams) ([]Record, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmptyQuery
	}
	if s.DB == nil {
		return nil, fmt.Errorf("query: record store is not configured")
	}

	ftsQuery := formatFTSQuery(text)
	if ftsQuery == "" {
		return nil, ErrEmptyQuery
	}

	var (
		query      string
		predicates []string
		args       []any
	)

	if params.IncludeDeleted {
		query = `
			SELECT h.record_ref, h.version, h.completeness, h.type, h.source, h.body, h.content_ref, h.updated_at
			FROM record_heads h
			WHERE h.record_ref IN (
				SELECT record_ref FROM records_fts WHERE records_fts MATCH ?
			)`
		args = append(args, ftsQuery)
	} else {
		query = `
			SELECT h.record_ref, h.version, h.completeness, h.type, h.source, h.body, h.content_ref, h.updated_at
			FROM record_heads h
			INNER JOIN records_fts ON records_fts.record_ref = h.record_ref
			WHERE records_fts MATCH ?
				AND h.completeness != ?`
		args = append(args, ftsQuery, records.CompletenessDeleted)
	}

	if params.TypePrefix != "" {
		predicates = append(predicates, "h.type LIKE ?")
		args = append(args, params.TypePrefix+"%")
	}
	if params.Source != "" {
		predicates = append(predicates, "h.source = ?")
		args = append(args, params.Source)
	}
	if params.TimeFrom != nil {
		predicates = append(predicates, "h.updated_at >= ?")
		args = append(args, params.TimeFrom.UTC().Format(time.RFC3339))
	}
	if params.TimeTo != nil {
		predicates = append(predicates, "h.updated_at <= ?")
		args = append(args, params.TimeTo.UTC().Format(time.RFC3339))
	}
	if len(predicates) > 0 {
		query += " AND " + strings.Join(predicates, " AND ")
	}
	query += " ORDER BY h.updated_at ASC"

	return s.queryRecords(ctx, query, args...)
}

// ListIncompleteRecords returns records with completeness incomplete.
func (s *RecordService) ListIncompleteRecords(ctx context.Context, source string, limit int) ([]Record, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("query: record store is not configured")
	}
	if limit <= 0 {
		limit = defaultRecordLimit
	}

	query := `
		SELECT record_ref, version, completeness, type, source, body, content_ref, updated_at
		FROM record_heads
		WHERE completeness = ?`
	args := []any{records.CompletenessIncomplete}
	if source = strings.TrimSpace(source); source != "" {
		query += " AND source = ?"
		args = append(args, source)
	}
	query += " ORDER BY updated_at ASC LIMIT ?"
	args = append(args, limit)

	return s.queryRecords(ctx, query, args...)
}

// ListRecords returns records matching optional filters.
func (s *RecordService) ListRecords(ctx context.Context, params ListRecordsParams) ([]Record, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("query: record store is not configured")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultRecordLimit
	}

	var (
		predicates []string
		args       []any
	)

	if params.TypePrefix != "" {
		predicates = append(predicates, "type LIKE ?")
		args = append(args, params.TypePrefix+"%")
	}
	if params.Source != "" {
		predicates = append(predicates, "source = ?")
		args = append(args, params.Source)
	}
	if params.Completeness != "" {
		predicates = append(predicates, "completeness = ?")
		args = append(args, params.Completeness)
	}
	if params.TimeFrom != nil {
		predicates = append(predicates, "updated_at >= ?")
		args = append(args, params.TimeFrom.UTC().Format(time.RFC3339))
	}
	if params.TimeTo != nil {
		predicates = append(predicates, "updated_at <= ?")
		args = append(args, params.TimeTo.UTC().Format(time.RFC3339))
	}

	query := `
		SELECT record_ref, version, completeness, type, source, body, content_ref, updated_at
		FROM record_heads`
	if len(predicates) > 0 {
		query += " WHERE " + strings.Join(predicates, " AND ")
	}
	query += " ORDER BY updated_at ASC LIMIT ?"
	args = append(args, limit)

	return s.queryRecords(ctx, query, args...)
}

func (s *RecordService) queryRecords(ctx context.Context, query string, args ...any) ([]Record, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: list records: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("query: scan record: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query: list records: %w", err)
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (Record, error) {
	var (
		rec        Record
		body       string
		updatedAt  string
		contentRef sql.NullString
	)
	if err := row.Scan(
		&rec.RecordRef,
		&rec.Version,
		&rec.Completeness,
		&rec.Type,
		&rec.Source,
		&body,
		&contentRef,
		&updatedAt,
	); err != nil {
		return Record{}, err
	}

	var err error
	rec.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return Record{}, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	rec.Body = json.RawMessage(body)
	if contentRef.Valid {
		ref := contentRef.String
		rec.ContentRef = &ref
	}
	return rec, nil
}

func formatFTSQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	fields := strings.Fields(text)
	quoted := make([]string, len(fields))
	for i, field := range fields {
		quoted[i] = `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " ")
}
