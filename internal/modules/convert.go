package modules

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

func rpcRevisionToJournal(e *troverpc.Revision) (journal.Revision, error) {
	if e == nil {
		return journal.Revision{}, fmt.Errorf("modules: event is nil")
	}
	if e.Type == "" {
		return journal.Revision{}, fmt.Errorf("modules: type is required")
	}
	if e.Source == "" {
		return journal.Revision{}, fmt.Errorf("modules: source is required")
	}
	if len(e.Payload) == 0 {
		return journal.Revision{}, fmt.Errorf("modules: payload is required")
	}
	if !json.Valid(e.Payload) {
		return journal.Revision{}, fmt.Errorf("modules: payload must be valid JSON")
	}

	eventTime, err := parseProtoTime(e.Time)
	if err != nil {
		return journal.Revision{}, fmt.Errorf("modules: time: %w", err)
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

	return journal.Revision{
		ID:         e.Id,
		Time:       eventTime,
		Operation:  operation,
		RecordRef:  e.RecordRef,
		Type:       e.Type,
		SchemaRef:  e.SchemaRef,
		Source:     e.Source,
		Producer:   e.Producer,
		Payload:    json.RawMessage(e.Payload),
		BlobRef:    blobRef,
		Transforms: json.RawMessage(e.Transforms),
	}, nil
}

func rpcAppendRevisionRequestToJournal(req *troverpc.AppendRevisionRequest) (journal.Revision, error) {
	if req == nil {
		return journal.Revision{}, fmt.Errorf("modules: emit record request is nil")
	}

	eventTime, err := parseProtoTime(req.Time)
	if err != nil {
		return journal.Revision{}, fmt.Errorf("modules: time: %w", err)
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

	return journal.Revision{
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

func journalRevisionToRPC(e journal.Revision) *troverpc.Revision {
	out := &troverpc.Revision{
		Id:         e.ID,
		Type:       e.Type,
		SchemaRef:  e.SchemaRef,
		Source:     e.Source,
		Producer:   e.Producer,
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
