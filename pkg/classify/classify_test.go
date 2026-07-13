package classify

import (
	"context"
	"encoding/json"
	"testing"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/records"
)

type fakeStore struct {
	records map[string]*troverpc.Record
	writes  []*troverpc.WriteRequest
	nextID  int
}

func (f *fakeStore) GetRecord(_ context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	rec, ok := f.records[req.RecordRef]
	if !ok {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (f *fakeStore) ListIncompleteRecords(_ context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	var out []*troverpc.Record
	for _, rec := range f.records {
		if rec.GetCompleteness() != records.CompletenessIncomplete {
			continue
		}
		if req.Source != "" && rec.GetSource() != req.Source {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (f *fakeStore) RecordWrite(_ context.Context, req *troverpc.WriteRequest) (*troverpc.WriteResponse, error) {
	f.writes = append(f.writes, req)
	f.nextID++
	ref := req.RecordRef
	if ref == "" {
		ref = "01RECNEW"
	}
	version := int32(1)
	if existing, ok := f.records[ref]; ok {
		version = existing.Version + 1
	}
	completeness := records.CompletenessIncomplete
	if req.Type != "" {
		completeness = records.CompletenessComplete
	}
	f.records[ref] = &troverpc.Record{
		RecordRef:    ref,
		Version:      version,
		Completeness: completeness,
		Type:         req.Type,
		Source:       req.Source,
		Body:         req.Payload,
	}
	return &troverpc.WriteResponse{
		EventId:      "01EVT" + string(rune('0'+f.nextID)),
		RecordRef:    ref,
		Version:      version,
		Completeness: completeness,
		Operation:    req.Operation,
	}, nil
}

func TestCaptureAndClassify(t *testing.T) {
	t.Parallel()
	store := &fakeStore{records: map[string]*troverpc.Record{}}

	cap, err := Capture(context.Background(), store, "telegram", []byte(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if cap.RecordRef == "" {
		t.Fatal("expected record_ref")
	}

	_, err = Classify(context.Background(), store, ClassifyRequest{
		RecordRef:  cap.RecordRef,
		TargetType: "trove://type/note/quick/1",
		Payload:    json.RawMessage(`{"title":"note"}`),
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if store.records[cap.RecordRef].GetCompleteness() != records.CompletenessComplete {
		t.Fatalf("completeness = %s", store.records[cap.RecordRef].GetCompleteness())
	}
}
