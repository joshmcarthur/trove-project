package records_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/records"
	_ "modernc.org/sqlite"
)

const testEventsDDL = `
CREATE TABLE events (
  id          TEXT PRIMARY KEY,
  time        TEXT NOT NULL,
  operation   TEXT NOT NULL DEFAULT '',
  record_ref  TEXT NOT NULL DEFAULT '',
  type        TEXT NOT NULL DEFAULT '',
  schema_ref  TEXT NOT NULL DEFAULT '',
  source      TEXT NOT NULL,
  payload     TEXT NOT NULL,
  transforms  TEXT,
  blob_ref    TEXT
);
`

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/test.db?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(testEventsDDL); err != nil {
		t.Fatalf("create events table: %v", err)
	}
	if err := records.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	return db
}

func insertEvent(t *testing.T, db *sql.DB, e journal.Event) {
	t.Helper()

	var transforms sql.NullString
	if len(e.Transforms) > 0 {
		transforms = sql.NullString{String: string(e.Transforms), Valid: true}
	}
	var blobRef sql.NullString
	if e.BlobRef != nil {
		blobRef = sql.NullString{String: *e.BlobRef, Valid: true}
	}

	_, err := db.Exec(`
		INSERT INTO events (id, time, operation, record_ref, type, schema_ref, source, payload, transforms, blob_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.Time.UTC().Format(time.RFC3339),
		e.Operation,
		e.RecordRef,
		e.Type,
		e.SchemaRef,
		e.Source,
		string(e.Payload),
		transforms,
		blobRef,
	)
	if err != nil {
		t.Fatalf("insert event %q: %v", e.ID, err)
	}
}

func applyEvent(t *testing.T, db *sql.DB, e journal.Event) bool {
	t.Helper()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}

	mat := records.NewMaterializer(tx)
	applied, err := mat.Apply(ctx, e)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("Apply() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	return applied
}

func loadHead(t *testing.T, db *sql.DB, recordRef string) records.Head {
	t.Helper()

	var (
		head       records.Head
		body       string
		updatedAt  string
		contentRef sql.NullString
	)
	err := db.QueryRow(`
		SELECT record_ref, version, completeness, type, source, body, content_ref, updated_at
		FROM record_heads
		WHERE record_ref = ?`, recordRef).Scan(
		&head.RecordRef,
		&head.Version,
		&head.Completeness,
		&head.Type,
		&head.Source,
		&body,
		&contentRef,
		&updatedAt,
	)
	if err != nil {
		t.Fatalf("load head %q: %v", recordRef, err)
	}
	head.Body = json.RawMessage(body)
	if contentRef.Valid {
		ref := contentRef.String
		head.ContentRef = &ref
	}
	head.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		t.Fatalf("parse updated_at: %v", err)
	}
	return head
}

func ftsCount(t *testing.T, db *sql.DB, recordRef string) int {
	t.Helper()

	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM records_fts WHERE record_ref = ?`, recordRef).Scan(&n)
	if err != nil {
		t.Fatalf("fts count: %v", err)
	}
	return n
}

func TestMaterializerApplyCreatesRecord(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000001"
	e := journal.Event{
		ID:        "01JEVT00000000000000000001",
		Time:      time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC),
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"hello"}`),
	}

	if !applyEvent(t, db, e) {
		t.Fatal("Apply() skipped new event")
	}

	head := loadHead(t, db, ref)
	if head.Version != 1 {
		t.Fatalf("version = %d, want 1", head.Version)
	}
	if head.Completeness != records.CompletenessIncomplete {
		t.Fatalf("completeness = %q, want %q", head.Completeness, records.CompletenessIncomplete)
	}
	if string(head.Body) != `{"text":"hello"}` {
		t.Fatalf("body = %s", head.Body)
	}
	if ftsCount(t, db, ref) != 1 {
		t.Fatalf("fts rows = %d, want 1", ftsCount(t, db, ref))
	}
}

func TestMaterializerApplyIncrementsVersion(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000002"
	base := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)

	insertEvent(t, db, journal.Event{
		ID:        "01JEVT00000000000000000002",
		Time:      base,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"one"}`),
	})
	if !applyEvent(t, db, journal.Event{
		ID:        "01JEVT00000000000000000002",
		Time:      base,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"one"}`),
	}) {
		t.Fatal("first Apply() skipped")
	}

	second := journal.Event{
		ID:        "01JEVT00000000000000000003",
		Time:      base.Add(time.Minute),
		Operation: journal.OpApply,
		RecordRef: ref,
		Type:      "trove://type/note/quick/1",
		Source:    "test",
		Payload:   json.RawMessage(`{"title":"note"}`),
	}
	insertEvent(t, db, second)
	if !applyEvent(t, db, second) {
		t.Fatal("second Apply() skipped")
	}

	head := loadHead(t, db, ref)
	if head.Version != 2 {
		t.Fatalf("version = %d, want 2", head.Version)
	}
	if head.Completeness != records.CompletenessComplete {
		t.Fatalf("completeness = %q, want %q", head.Completeness, records.CompletenessComplete)
	}
	if head.Type != "trove://type/note/quick/1" {
		t.Fatalf("type = %q", head.Type)
	}
	if string(head.Body) != `{"text":"one","title":"note"}` {
		t.Fatalf("body = %s", head.Body)
	}
}

func TestMaterializerApplyFoldsTransforms(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000003"
	when := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)

	first := journal.Event{
		ID:        "01JEVT00000000000000000004",
		Time:      when,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"members":["a"]}`),
	}
	insertEvent(t, db, first)
	applyEvent(t, db, first)

	second := journal.Event{
		ID:         "01JEVT00000000000000000005",
		Time:       when.Add(time.Minute),
		Operation:  journal.OpApply,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		Transforms: json.RawMessage(`[{"op":"add","path":"/members/-","value":"b"}]`),
	}
	insertEvent(t, db, second)
	applyEvent(t, db, second)

	head := loadHead(t, db, ref)
	if string(head.Body) != `{"members":["a","b"]}` {
		t.Fatalf("body = %s, want merged + transformed members", head.Body)
	}
}

func TestMaterializerDeleteRetainsBodyAndRemovesFTS(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000004"
	when := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	create := journal.Event{
		ID:        "01JEVT00000000000000000006",
		Time:      when,
		Operation: journal.OpApply,
		RecordRef: ref,
		Type:      "trove://type/note/quick/1",
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"keep me"}`),
	}
	insertEvent(t, db, create)
	applyEvent(t, db, create)
	if ftsCount(t, db, ref) != 1 {
		t.Fatal("expected fts row before delete")
	}

	del := journal.Event{
		ID:        "01JEVT00000000000000000007",
		Time:      when.Add(time.Minute),
		Operation: journal.OpDelete,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{}`),
	}
	insertEvent(t, db, del)
	applyEvent(t, db, del)

	head := loadHead(t, db, ref)
	if head.Completeness != records.CompletenessDeleted {
		t.Fatalf("completeness = %q, want %q", head.Completeness, records.CompletenessDeleted)
	}
	if string(head.Body) != `{"text":"keep me"}` {
		t.Fatalf("body = %s, want retained", head.Body)
	}
	if head.Type != "trove://type/note/quick/1" {
		t.Fatalf("type = %q, want retained", head.Type)
	}
	if ftsCount(t, db, ref) != 0 {
		t.Fatalf("fts rows = %d, want 0 after delete", ftsCount(t, db, ref))
	}
}

func TestMaterializerSkipsDuplicateEventID(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000005"
	e := journal.Event{
		ID:        "01JEVT00000000000000000008",
		Time:      time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC),
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"once"}`),
	}
	insertEvent(t, db, e)

	if !applyEvent(t, db, e) {
		t.Fatal("first Apply() skipped")
	}
	if applyEvent(t, db, e) {
		t.Fatal("duplicate Apply() should skip")
	}

	head := loadHead(t, db, ref)
	if head.Version != 1 {
		t.Fatalf("version = %d, want 1 after duplicate skip", head.Version)
	}
}

func TestRebuildAllReplaysEvents(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000006"
	when := time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC)

	events := []journal.Event{
		{
			ID:        "01JEVT00000000000000000009",
			Time:      when,
			Operation: journal.OpApply,
			RecordRef: ref,
			Source:    "test",
			Payload:   json.RawMessage(`{"text":"v1"}`),
		},
		{
			ID:        "01JEVT00000000000000000010",
			Time:      when.Add(time.Minute),
			Operation: journal.OpApply,
			RecordRef: ref,
			Type:      "trove://type/note/quick/1",
			Source:    "test",
			Payload:   json.RawMessage(`{"title":"v2"}`),
		},
		{
			ID:        "01JEVT00000000000000000011",
			Time:      when.Add(2 * time.Minute),
			Operation: journal.OpDelete,
			RecordRef: ref,
			Source:    "test",
			Payload:   json.RawMessage(`{}`),
		},
	}
	for _, e := range events {
		insertEvent(t, db, e)
		applyEvent(t, db, e)
	}

	want := loadHead(t, db, ref)

	if _, err := db.Exec(`DELETE FROM records_fts`); err != nil {
		t.Fatalf("clear fts: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM record_events`); err != nil {
		t.Fatalf("clear record_events: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM record_heads`); err != nil {
		t.Fatalf("clear record_heads: %v", err)
	}

	if err := records.RebuildAll(context.Background(), db); err != nil {
		t.Fatalf("RebuildAll() error = %v", err)
	}

	got := loadHead(t, db, ref)
	if got.Version != want.Version ||
		got.Completeness != want.Completeness ||
		got.Type != want.Type ||
		string(got.Body) != string(want.Body) {
		t.Fatalf("rebuilt head = %+v, want %+v", got, want)
	}
	if ftsCount(t, db, ref) != 0 {
		t.Fatalf("fts rows after rebuild = %d, want 0 for deleted record", ftsCount(t, db, ref))
	}

	var eventLinks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM record_events WHERE record_ref = ?`, ref).Scan(&eventLinks); err != nil {
		t.Fatalf("count record_events: %v", err)
	}
	if eventLinks != 3 {
		t.Fatalf("record_events = %d, want 3", eventLinks)
	}
}
