package records_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/records"
	"github.com/joshmcarthur/trove/internal/references"
	_ "modernc.org/sqlite"
)

const testRevisionsDDL = `
CREATE TABLE revisions (
  id          TEXT PRIMARY KEY,
  time        TEXT NOT NULL,
  operation   TEXT NOT NULL DEFAULT '',
  record_ref  TEXT NOT NULL DEFAULT '',
  type        TEXT NOT NULL DEFAULT '',
  schema_ref  TEXT NOT NULL DEFAULT '',
  source      TEXT NOT NULL,
  producer    TEXT NOT NULL DEFAULT 'unknown',
  payload     TEXT NOT NULL,
  transforms  TEXT,
  blob_ref    TEXT,
  recorded_at TEXT NOT NULL DEFAULT '',
  sequence    INTEGER NOT NULL DEFAULT 0,
  "references" TEXT
);
CREATE INDEX idx_revisions_record_sequence ON revisions(record_ref, sequence);
`

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/test.db?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(testRevisionsDDL); err != nil {
		t.Fatalf("create revisions table: %v", err)
	}
	if err := records.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	return db
}

func insertEvent(t *testing.T, db *sql.DB, e journal.Revision) {
	t.Helper()

	if e.Sequence <= 0 {
		var nextSeq int
		if err := db.QueryRow(`SELECT COALESCE(MAX(sequence), 0) + 1 FROM revisions WHERE record_ref = ?`, e.RecordRef).Scan(&nextSeq); err != nil {
			t.Fatalf("assign sequence for %q: %v", e.RecordRef, err)
		}
		e.Sequence = nextSeq
	}
	if e.RecordedAt.IsZero() {
		e.RecordedAt = e.Time
	}

	var transforms sql.NullString
	if len(e.Transforms) > 0 {
		transforms = sql.NullString{String: string(e.Transforms), Valid: true}
	}
	var blobRef sql.NullString
	if e.BlobRef != nil {
		blobRef = sql.NullString{String: *e.BlobRef, Valid: true}
	}
	var refs sql.NullString
	if e.References != nil {
		refs = sql.NullString{String: string(e.References), Valid: true}
	}

	_, err := db.Exec(`
		INSERT INTO revisions (id, time, operation, record_ref, type, schema_ref, source, producer, payload, transforms, blob_ref, recorded_at, sequence, "references")
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.Time.UTC().Format(time.RFC3339),
		e.Operation,
		e.RecordRef,
		e.Type,
		e.SchemaRef,
		e.Source,
		producerOrDefault(e.Producer),
		string(e.Payload),
		transforms,
		blobRef,
		e.RecordedAt.UTC().Format(time.RFC3339),
		e.Sequence,
		refs,
	)
	if err != nil {
		t.Fatalf("insert event %q: %v", e.ID, err)
	}
}

func producerOrDefault(producer string) string {
	if producer == "" {
		return "unknown"
	}
	return producer
}

func applyEvent(t *testing.T, db *sql.DB, e journal.Revision) bool {
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
		refsJSON   string
	)
	err := db.QueryRow(`
		SELECT record_ref, version, completeness, type, source, body, content_ref, "references", updated_at
		FROM record_heads
		WHERE record_ref = ?`, recordRef).Scan(
		&head.RecordRef,
		&head.Version,
		&head.Completeness,
		&head.Type,
		&head.Source,
		&body,
		&contentRef,
		&refsJSON,
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
	refs, err := references.Unmarshal(json.RawMessage(refsJSON))
	if err != nil {
		t.Fatalf("parse references: %v", err)
	}
	head.References = refs
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
	e := journal.Revision{
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

	insertEvent(t, db, journal.Revision{
		ID:        "01JEVT00000000000000000002",
		Time:      base,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"one"}`),
	})
	if !applyEvent(t, db, journal.Revision{
		ID:        "01JEVT00000000000000000002",
		Time:      base,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"one"}`),
	}) {
		t.Fatal("first Apply() skipped")
	}

	second := journal.Revision{
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

	first := journal.Revision{
		ID:        "01JEVT00000000000000000004",
		Time:      when,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"members":["a"]}`),
	}
	insertEvent(t, db, first)
	applyEvent(t, db, first)

	second := journal.Revision{
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

	create := journal.Revision{
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

	del := journal.Revision{
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
	e := journal.Revision{
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

	events := []journal.Revision{
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
	if _, err := db.Exec(`DELETE FROM record_revisions`); err != nil {
		t.Fatalf("clear record_revisions: %v", err)
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
	if err := db.QueryRow(`SELECT COUNT(*) FROM record_revisions WHERE record_ref = ?`, ref).Scan(&eventLinks); err != nil {
		t.Fatalf("count record_revisions: %v", err)
	}
	if eventLinks != 3 {
		t.Fatalf("record_revisions = %d, want 3", eventLinks)
	}
}

func TestMaterializerReplayBySequenceNotTime(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000007"
	later := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)

	first := journal.Revision{
		ID:        "01JEVT00000000000000000020",
		Time:      later,
		Sequence:  1,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"first-by-sequence"}`),
	}
	second := journal.Revision{
		ID:        "01JEVT00000000000000000021",
		Time:      earlier,
		Sequence:  2,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"second-by-sequence"}`),
	}

	insertEvent(t, db, first)
	insertEvent(t, db, second)
	applyEvent(t, db, first)
	applyEvent(t, db, second)

	want := loadHead(t, db, ref)
	if string(want.Body) != `{"text":"second-by-sequence"}` {
		t.Fatalf("incremental body = %s, want replay by sequence", want.Body)
	}

	if _, err := db.Exec(`DELETE FROM records_fts`); err != nil {
		t.Fatalf("clear fts: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM record_revisions`); err != nil {
		t.Fatalf("clear record_revisions: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM record_heads`); err != nil {
		t.Fatalf("clear record_heads: %v", err)
	}

	if err := records.RebuildAll(context.Background(), db); err != nil {
		t.Fatalf("RebuildAll() error = %v", err)
	}

	got := loadHead(t, db, ref)
	if string(got.Body) != string(want.Body) {
		t.Fatalf("rebuilt body = %s, want %s (sequence order, not event time)", got.Body, want.Body)
	}
}

func TestMaterializerApplyReferencesReplaceAndClear(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000008"
	when := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)

	create := journal.Revision{
		ID:         "01JEVT00000000000000000040",
		Time:       when,
		Operation:  journal.OpApply,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{"text":"hello"}`),
		References: json.RawMessage(`[{"ref":"https://example.com/a"}]`),
	}
	insertEvent(t, db, create)
	applyEvent(t, db, create)

	head := loadHead(t, db, ref)
	if len(head.References) != 1 || head.References[0].Ref != "https://example.com/a" {
		t.Fatalf("initial references = %+v", head.References)
	}

	replace := journal.Revision{
		ID:         "01JEVT00000000000000000041",
		Time:       when.Add(time.Minute),
		Operation:  journal.OpApply,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		References: json.RawMessage(`[{"ref":"https://example.com/b","rel":"source"}]`),
	}
	insertEvent(t, db, replace)
	applyEvent(t, db, replace)

	head = loadHead(t, db, ref)
	if len(head.References) != 1 || head.References[0].Ref != "https://example.com/b" {
		t.Fatalf("replaced references = %+v", head.References)
	}

	unchanged := journal.Revision{
		ID:        "01JEVT00000000000000000042",
		Time:      when.Add(2 * time.Minute),
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"more"}`),
	}
	insertEvent(t, db, unchanged)
	applyEvent(t, db, unchanged)

	head = loadHead(t, db, ref)
	if len(head.References) != 1 || head.References[0].Ref != "https://example.com/b" {
		t.Fatalf("unchanged references = %+v", head.References)
	}

	clear := journal.Revision{
		ID:         "01JEVT00000000000000000043",
		Time:       when.Add(3 * time.Minute),
		Operation:  journal.OpApply,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		References: json.RawMessage(`[]`),
	}
	insertEvent(t, db, clear)
	applyEvent(t, db, clear)

	head = loadHead(t, db, ref)
	if len(head.References) != 0 {
		t.Fatalf("cleared references = %+v, want empty", head.References)
	}
}

func TestMaterializerDeleteRetainsReferences(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000009"
	when := time.Date(2026, 7, 14, 16, 0, 0, 0, time.UTC)

	create := journal.Revision{
		ID:         "01JEVT00000000000000000044",
		Time:       when,
		Operation:  journal.OpApply,
		RecordRef:  ref,
		Type:       "trove://type/note/quick/1",
		Source:     "test",
		Payload:    json.RawMessage(`{"text":"keep"}`),
		References: json.RawMessage(`[{"ref":"https://example.com/x"}]`),
	}
	insertEvent(t, db, create)
	applyEvent(t, db, create)

	del := journal.Revision{
		ID:        "01JEVT00000000000000000045",
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
		t.Fatalf("completeness = %q", head.Completeness)
	}
	if len(head.References) != 1 || head.References[0].Ref != "https://example.com/x" {
		t.Fatalf("references = %+v, want retained", head.References)
	}
}

func TestMaterializerLinkAndUnlink(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ref := "01JREC00000000000000000010"
	when := time.Date(2026, 7, 14, 17, 0, 0, 0, time.UTC)

	create := journal.Revision{
		ID:        "01JEVT00000000000000000046",
		Time:      when,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"text":"base"}`),
	}
	insertEvent(t, db, create)
	applyEvent(t, db, create)

	link := journal.Revision{
		ID:         "01JEVT00000000000000000047",
		Time:       when.Add(time.Minute),
		Operation:  journal.OpLink,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		References: json.RawMessage(`[{"ref":"https://example.com/a"},{"ref":"https://example.com/b","rel":"source"}]`),
	}
	insertEvent(t, db, link)
	applyEvent(t, db, link)

	head := loadHead(t, db, ref)
	if len(head.References) != 2 {
		t.Fatalf("after link references = %+v", head.References)
	}
	if string(head.Body) != `{"text":"base"}` {
		t.Fatalf("body changed on link: %s", head.Body)
	}

	dupLink := journal.Revision{
		ID:         "01JEVT00000000000000000048",
		Time:       when.Add(2 * time.Minute),
		Operation:  journal.OpLink,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		References: json.RawMessage(`[{"ref":"https://example.com/a"},{"ref":"https://example.com/c"}]`),
	}
	insertEvent(t, db, dupLink)
	applyEvent(t, db, dupLink)

	head = loadHead(t, db, ref)
	if len(head.References) != 3 {
		t.Fatalf("after deduped link references = %+v", head.References)
	}

	unlink := journal.Revision{
		ID:         "01JEVT00000000000000000049",
		Time:       when.Add(3 * time.Minute),
		Operation:  journal.OpUnlink,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		References: json.RawMessage(`[{"ref":"https://example.com/a"}]`),
	}
	insertEvent(t, db, unlink)
	applyEvent(t, db, unlink)

	head = loadHead(t, db, ref)
	if len(head.References) != 2 {
		t.Fatalf("after unlink all rels for ref references = %+v", head.References)
	}

	unlinkExact := journal.Revision{
		ID:         "01JEVT00000000000000000050",
		Time:       when.Add(4 * time.Minute),
		Operation:  journal.OpUnlink,
		RecordRef:  ref,
		Source:     "test",
		Payload:    json.RawMessage(`{}`),
		References: json.RawMessage(`[{"ref":"https://example.com/b","rel":"source"}]`),
	}
	insertEvent(t, db, unlinkExact)
	applyEvent(t, db, unlinkExact)

	head = loadHead(t, db, ref)
	if len(head.References) != 1 || head.References[0].Ref != "https://example.com/c" {
		t.Fatalf("after exact unlink references = %+v", head.References)
	}
}
