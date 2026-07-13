package modules

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

func rpcEventToJournal(e *troverpc.Event) (journal.Event, error) {
	if e == nil {
		return journal.Event{}, fmt.Errorf("modules: event is nil")
	}
	if e.Type == "" {
		return journal.Event{}, fmt.Errorf("modules: type is required")
	}
	if e.Source == "" {
		return journal.Event{}, fmt.Errorf("modules: source is required")
	}
	if len(e.Payload) == 0 {
		return journal.Event{}, fmt.Errorf("modules: payload is required")
	}
	if !json.Valid(e.Payload) {
		return journal.Event{}, fmt.Errorf("modules: payload must be valid JSON")
	}

	eventTime, err := parseProtoTime(e.Time)
	if err != nil {
		return journal.Event{}, fmt.Errorf("modules: time: %w", err)
	}

	var blobRef *string
	if e.BlobRef != "" {
		ref := e.BlobRef
		blobRef = &ref
	}

	operation := e.Operation
	if operation == "" {
		operation = journal.OpApply
	}

	return journal.Event{
		ID:         e.Id,
		Time:       eventTime,
		Operation:  operation,
		RecordRef:  e.RecordRef,
		Type:       e.Type,
		SchemaRef:  e.SchemaRef,
		Source:     e.Source,
		Payload:    json.RawMessage(e.Payload),
		BlobRef:    blobRef,
		Transforms: json.RawMessage(e.Transforms),
	}, nil
}

func rpcEmitRecordRequestToJournal(req *troverpc.EmitRecordRequest) (journal.Event, error) {
	if req == nil {
		return journal.Event{}, fmt.Errorf("modules: emit record request is nil")
	}

	eventTime, err := parseProtoTime(req.Time)
	if err != nil {
		return journal.Event{}, fmt.Errorf("modules: time: %w", err)
	}

	var blobRef *string
	if req.BlobRef != "" {
		ref := req.BlobRef
		blobRef = &ref
	}

	operation := req.Operation
	if operation == "" {
		operation = journal.OpApply
	}

	return journal.Event{
		Time:       eventTime,
		Operation:  operation,
		RecordRef:  req.RecordRef,
		Type:       req.Type,
		Source:     req.Source,
		Payload:    json.RawMessage(req.Payload),
		BlobRef:    blobRef,
		Transforms: json.RawMessage(req.Transforms),
	}, nil
}

func parseProtoTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func journalEventToRPC(e journal.Event) *troverpc.Event {
	out := &troverpc.Event{
		Id:         e.ID,
		Type:       e.Type,
		SchemaRef:  e.SchemaRef,
		Source:     e.Source,
		Payload:    e.Payload,
		Operation:  e.Operation,
		RecordRef:  e.RecordRef,
		Transforms: e.Transforms,
	}
	if !e.Time.IsZero() {
		out.Time = e.Time.UTC().Format(time.RFC3339)
	}
	if e.BlobRef != nil {
		out.BlobRef = *e.BlobRef
	}
	return out
}
