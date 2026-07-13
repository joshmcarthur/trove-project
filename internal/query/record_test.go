package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/records"
)

func openTestRecordService(t *testing.T) (*journal.Store, *RecordService) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "trove.db")
	store, err := journal.Open(path)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, &RecordService{DB: store.DB()}
}

func materializeEvent(t *testing.T, store *journal.Store, e journal.Event) {
	t.Helper()

	ctx := context.Background()
	err := store.AppendTransactional(ctx, e, func(ctx context.Context, tx *sql.Tx, e journal.Event) error {
		_, err := records.NewMaterializer(tx).Apply(ctx, e)
		return err
	})
	if err != nil {
		t.Fatalf("materializeEvent() error = %v", err)
	}
}

func seedRecord(t *testing.T, store *journal.Store, eventID, ref string, when time.Time, source, body string, typ string) {
	t.Helper()

	e := journal.Event{
		ID:        eventID,
		Time:      when,
		Operation: journal.OpApply,
		RecordRef: ref,
		Source:    source,
		Payload:   json.RawMessage(body),
	}
	if typ != "" {
		e.Type = typ
	}
	materializeEvent(t, store, e)
}

func TestGetRecord(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, svc := openTestRecordService(t)
	ref := "01JREC00000000000000000001"
	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)

	seedRecord(t, store, "01JEVT00000000000000000001", ref, when, "shortcuts", `{"text":"hello"}`, "")

	got, err := svc.GetRecord(ctx, ref, 0)
	if err != nil {
		t.Fatalf("GetRecord() error = %v", err)
	}
	if got.RecordRef != ref {
		t.Errorf("RecordRef = %q, want %q", got.RecordRef, ref)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
	if got.Completeness != records.CompletenessIncomplete {
		t.Errorf("Completeness = %q, want %q", got.Completeness, records.CompletenessIncomplete)
	}
	if string(got.Body) != `{"text":"hello"}` {
		t.Errorf("Body = %s", got.Body)
	}
}

func TestGetRecordVersionMismatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, svc := openTestRecordService(t)
	ref := "01JREC00000000000000000002"
	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)

	seedRecord(t, store, "01JEVT00000000000000000001", ref, when, "shortcuts", `{"text":"hello"}`, "")

	_, err := svc.GetRecord(ctx, ref, 99)
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("GetRecord() error = %v, want %v", err, ErrRecordNotFound)
	}
}

func TestGetRecordNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, svc := openTestRecordService(t)

	_, err := svc.GetRecord(ctx, "01JREC00000000000000009999", 0)
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("GetRecord() error = %v, want %v", err, ErrRecordNotFound)
	}
}

func TestSearchRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, svc := openTestRecordService(t)

	t1 := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)

	refA := "01JREC00000000000000000010"
	refB := "01JREC00000000000000000011"
	refC := "01JREC00000000000000000012"

	seedRecord(t, store, "01JEVT00000000000000000010", refA, t1, "sensor-a", `{"reading":"balmy"}`, "trove://type/mqtt/sensor/temp/1")
	seedRecord(t, store, "01JEVT00000000000000000011", refB, t2, "sensor-a", `{"reading":"dry"}`, "trove://type/mqtt/sensor/humidity/1")
	seedRecord(t, store, "01JEVT00000000000000000012", refC, t3, "kitchen-light", `{"room":"kitchen"}`, "ha.light.on")

	materializeEvent(t, store, journal.Event{
		ID:        "01JEVT00000000000000000099",
		Time:      t3.Add(time.Minute),
		Operation: journal.OpDelete,
		RecordRef: refC,
		Source:    "kitchen-light",
		Payload:   json.RawMessage(`{}`),
	})

	tests := []struct {
		name    string
		query   string
		params  RecordSearchParams
		wantIDs []string
	}{
		{
			name:    "match keyword in body",
			query:   "balmy",
			wantIDs: []string{refA},
		},
		{
			name:    "match keyword in type",
			query:   "humidity",
			wantIDs: []string{refB},
		},
		{
			name:    "exclude deleted by default",
			query:   "kitchen",
			wantIDs: nil,
		},
		{
			name:  "type prefix filter",
			query: "balmy",
			params: RecordSearchParams{
				TypePrefix: "trove://type/mqtt/",
			},
			wantIDs: []string{refA},
		},
		{
			name:  "source filter",
			query: "reading",
			params: RecordSearchParams{
				Source: "sensor-a",
			},
			wantIDs: []string{refA, refB},
		},
		{
			name:  "updated_at range filter",
			query: "reading",
			params: RecordSearchParams{
				TimeFrom: &t2,
				TimeTo:   &t3,
			},
			wantIDs: []string{refB},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := svc.SearchRecords(ctx, tt.query, tt.params)
			if err != nil {
				t.Fatalf("SearchRecords() error = %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("SearchRecords() returned %d records, want %d", len(got), len(tt.wantIDs))
			}
			for i, rec := range got {
				if rec.RecordRef != tt.wantIDs[i] {
					t.Errorf("record[%d].RecordRef = %q, want %q", i, rec.RecordRef, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestSearchRecordsEmptyQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, svc := openTestRecordService(t)

	_, err := svc.SearchRecords(ctx, "   ", RecordSearchParams{})
	if !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("SearchRecords() error = %v, want %v", err, ErrEmptyQuery)
	}
}

func TestListIncompleteRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, svc := openTestRecordService(t)

	incompleteRef := "01JREC00000000000000000020"
	completeRef := "01JREC00000000000000000021"
	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)

	seedRecord(t, store, "01JEVT00000000000000000020", incompleteRef, when, "shortcuts", `{"text":"draft"}`, "")
	seedRecord(t, store, "01JEVT00000000000000000021", completeRef, when.Add(time.Minute), "shortcuts", `{"text":"done"}`, "trove://type/note/quick/1")

	got, err := svc.ListIncompleteRecords(ctx, "", 0)
	if err != nil {
		t.Fatalf("ListIncompleteRecords() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListIncompleteRecords() returned %d records, want 1", len(got))
	}
	if got[0].RecordRef != incompleteRef {
		t.Errorf("RecordRef = %q, want %q", got[0].RecordRef, incompleteRef)
	}

	got, err = svc.ListIncompleteRecords(ctx, "shortcuts", 0)
	if err != nil {
		t.Fatalf("ListIncompleteRecords(source) error = %v", err)
	}
	if len(got) != 1 || got[0].RecordRef != incompleteRef {
		t.Fatalf("ListIncompleteRecords(source) = %#v, want only %q", got, incompleteRef)
	}
}

func TestListRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, svc := openTestRecordService(t)

	when := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	refA := "01JREC00000000000000000030"
	refB := "01JREC00000000000000000031"

	seedRecord(t, store, "01JEVT00000000000000000030", refA, when, "sensor-a", `{"v":1}`, "trove://type/mqtt/sensor/temp/1")
	seedRecord(t, store, "01JEVT00000000000000000031", refB, when.Add(time.Minute), "sensor-b", `{"v":2}`, "trove://type/mqtt/sensor/humidity/1")

	got, err := svc.ListRecords(ctx, ListRecordsParams{
		TypePrefix:   "trove://type/mqtt/sensor/temp/",
		Completeness: records.CompletenessComplete,
	})
	if err != nil {
		t.Fatalf("ListRecords() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListRecords() returned %d records, want 1", len(got))
	}
	if got[0].RecordRef != refA {
		t.Errorf("RecordRef = %q, want %q", got[0].RecordRef, refA)
	}
}
