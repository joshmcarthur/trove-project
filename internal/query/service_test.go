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

func TestSearchEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	t1 := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)

	seed := []journal.Event{
		{ID: "01JEVT00000000000000000001", Time: t1, Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"reading":"balmy"}`)},
		{ID: "01JEVT00000000000000000002", Time: t2, Type: "mqtt.sensor.humidity", Source: "sensor-a", Payload: json.RawMessage(`{"reading":"dry"}`)},
		{ID: "01JEVT00000000000000000003", Time: t3, Type: "ha.light.on", Source: "kitchen-light", Payload: json.RawMessage(`{"room":"kitchen"}`)},
	}
	for _, e := range seed {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	tests := []struct {
		name    string
		query   string
		params  SearchParams
		wantIDs []string
	}{
		{
			name:    "match keyword in payload",
			query:   "balmy",
			wantIDs: []string{seed[0].ID},
		},
		{
			name:    "match keyword in type",
			query:   "humidity",
			wantIDs: []string{seed[1].ID},
		},
		{
			name:    "match keyword in source",
			query:   "kitchen-light",
			wantIDs: []string{seed[2].ID},
		},
		{
			name:  "type prefix filter",
			query: "balmy",
			params: SearchParams{
				TypePrefix: "mqtt.",
			},
			wantIDs: []string{seed[0].ID},
		},
		{
			name:  "source filter",
			query: "reading",
			params: SearchParams{
				Source: "sensor-a",
			},
			wantIDs: []string{seed[0].ID, seed[1].ID},
		},
		{
			name:  "time range filter",
			query: "reading",
			params: SearchParams{
				TimeFrom: &t2,
				TimeTo:   &t3,
			},
			wantIDs: []string{seed[1].ID},
		},
		{
			name:    "no match returns empty slice",
			query:   "missing-keyword",
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := svc.SearchEvents(ctx, tt.query, tt.params)
			if err != nil {
				t.Fatalf("SearchEvents() error = %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("SearchEvents() returned %d events, want %d", len(got), len(tt.wantIDs))
			}
			for i, e := range got {
				if e.ID != tt.wantIDs[i] {
					t.Errorf("event[%d].ID = %q, want %q", i, e.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestSearchEventsEmptyQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}

	_, err := svc.SearchEvents(ctx, "   ", SearchParams{})
	if !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("SearchEvents() error = %v, want %v", err, ErrEmptyQuery)
	}
}

func TestGetEventsByType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	t1 := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)

	seed := []journal.Event{
		{ID: "01JEVT00000000000000000001", Time: t1, Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"v":1}`)},
		{ID: "01JEVT00000000000000000002", Time: t2, Type: "mqtt.sensor.humidity", Source: "sensor-a", Payload: json.RawMessage(`{"v":2}`)},
		{ID: "01JEVT00000000000000000003", Time: t3, Type: "mqtt.sensor.temp", Source: "sensor-b", Payload: json.RawMessage(`{"v":3}`)},
	}
	for _, e := range seed {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	tests := []struct {
		name      string
		eventType string
		timeFrom  *time.Time
		timeTo    *time.Time
		wantIDs   []string
	}{
		{
			name:      "exact type excludes other types",
			eventType: "mqtt.sensor.temp",
			wantIDs:   []string{seed[0].ID, seed[2].ID},
		},
		{
			name:      "time window excludes outside events",
			eventType: "mqtt.sensor.temp",
			timeFrom:  &t1,
			timeTo:    &t2,
			wantIDs:   []string{seed[0].ID},
		},
		{
			name:      "no matches returns empty slice",
			eventType: "ha.light.on",
			wantIDs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := svc.GetEventsByType(ctx, tt.eventType, tt.timeFrom, tt.timeTo)
			if err != nil {
				t.Fatalf("GetEventsByType() error = %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("GetEventsByType() returned %d events, want %d", len(got), len(tt.wantIDs))
			}
			for i, e := range got {
				if e.ID != tt.wantIDs[i] {
					t.Errorf("event[%d].ID = %q, want %q", i, e.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestGetEventsByTypeEmptyType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}

	_, err := svc.GetEventsByType(ctx, "   ", nil, nil)
	if !errors.Is(err, ErrEmptyType) {
		t.Fatalf("GetEventsByType() error = %v, want %v", err, ErrEmptyType)
	}
}

func TestGetEventsByTypeInvalidRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}

	timeFrom := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)

	_, err := svc.GetEventsByType(ctx, "mqtt.sensor.temp", &timeFrom, &timeTo)
	if !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("GetEventsByType() error = %v, want %v", err, ErrInvalidTimeRange)
	}
}

func TestSummarizeRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	timeFrom := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	seed := []journal.Event{
		{ID: "01JEVT00000000000000000001", Time: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC), Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"v":1}`)},
		{ID: "01JEVT00000000000000000002", Time: time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{"v":2}`)},
		{ID: "01JEVT00000000000000000003", Time: time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC), Type: "http.ingest.received", Source: "shortcuts", Payload: json.RawMessage(`{"v":3}`)},
	}
	for _, e := range seed {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append(%q) error = %v", e.ID, err)
		}
	}

	got, err := svc.SummarizeRange(ctx, timeFrom, timeTo)
	if err != nil {
		t.Fatalf("SummarizeRange() error = %v", err)
	}
	if got.Total != 3 {
		t.Errorf("Total = %d, want 3", got.Total)
	}
	if got.ByType["mqtt.sensor.temp"] != 2 {
		t.Errorf("ByType[mqtt.sensor.temp] = %d, want 2", got.ByType["mqtt.sensor.temp"])
	}
	if got.ByType["http.ingest.received"] != 1 {
		t.Errorf("ByType[http.ingest.received] = %d, want 1", got.ByType["http.ingest.received"])
	}
	if len(got.Notable) != 3 {
		t.Fatalf("len(Notable) = %d, want 3", len(got.Notable))
	}
	if got.Notable[0].ID != seed[2].ID {
		t.Errorf("Notable[0].ID = %q, want most recent %q", got.Notable[0].ID, seed[2].ID)
	}
}

func TestSummarizeRangeEmptyWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	timeFrom := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	got, err := svc.SummarizeRange(ctx, timeFrom, timeTo)
	if err != nil {
		t.Fatalf("SummarizeRange() error = %v", err)
	}
	if got.Total != 0 {
		t.Errorf("Total = %d, want 0", got.Total)
	}
	if len(got.ByType) != 0 {
		t.Errorf("ByType = %v, want empty", got.ByType)
	}
	if len(got.Notable) != 0 {
		t.Errorf("Notable = %v, want empty", got.Notable)
	}
}

func TestSummarizeRangeExcludesOutside(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	inside := journal.Event{
		ID: "01JEVT00000000000000000001", Time: time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC),
		Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{}`),
	}
	outside := journal.Event{
		ID: "01JEVT00000000000000000002", Time: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
		Type: "mqtt.sensor.temp", Source: "sensor-a", Payload: json.RawMessage(`{}`),
	}
	for _, e := range []journal.Event{inside, outside} {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	timeFrom := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 7, 10, 23, 59, 59, 0, time.UTC)

	got, err := svc.SummarizeRange(ctx, timeFrom, timeTo)
	if err != nil {
		t.Fatalf("SummarizeRange() error = %v", err)
	}
	if got.Total != 1 {
		t.Errorf("Total = %d, want 1", got.Total)
	}
	if len(got.Notable) != 1 || got.Notable[0].ID != inside.ID {
		t.Errorf("Notable = %#v, want only inside event", got.Notable)
	}
}

func TestSummarizeRangeNotableCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestJournal(t)
	svc := &Service{Journal: store}

	timeFrom := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 7, 10, 23, 59, 59, 0, time.UTC)

	for i := range 7 {
		e := journal.Event{
			ID:      ulid.MustNew(ulid.Now(), rand.Reader).String(),
			Time:    time.Date(2026, 7, 10, i+1, 0, 0, 0, time.UTC),
			Type:    "mqtt.sensor.temp",
			Source:  "sensor-a",
			Payload: json.RawMessage(`{}`),
		}
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	got, err := svc.SummarizeRange(ctx, timeFrom, timeTo)
	if err != nil {
		t.Fatalf("SummarizeRange() error = %v", err)
	}
	if got.Total != 7 {
		t.Errorf("Total = %d, want 7", got.Total)
	}
	if len(got.Notable) != maxNotableEvents {
		t.Errorf("len(Notable) = %d, want %d", len(got.Notable), maxNotableEvents)
	}
}

func TestSummarizeRangeInvalidRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := &Service{Journal: openTestJournal(t)}

	timeFrom := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)

	_, err := svc.SummarizeRange(ctx, timeFrom, timeTo)
	if !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("SummarizeRange() error = %v, want %v", err, ErrInvalidTimeRange)
	}
}
