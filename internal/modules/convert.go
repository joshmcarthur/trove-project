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

	var eventTime time.Time
	if e.Time != "" {
		parsed, err := time.Parse(time.RFC3339, e.Time)
		if err != nil {
			return journal.Event{}, fmt.Errorf("modules: time: %w", err)
		}
		eventTime = parsed.UTC()
	}

	var blobRef *string
	if e.BlobRef != "" {
		ref := e.BlobRef
		blobRef = &ref
	}

	return journal.Event{
		ID:        e.Id,
		Time:      eventTime,
		Type:      e.Type,
		SchemaRef: e.SchemaRef,
		Source:    e.Source,
		Payload:   json.RawMessage(e.Payload),
		BlobRef:   blobRef,
	}, nil
}

func journalEventToRPC(e journal.Event) *troverpc.Event {
	out := &troverpc.Event{
		Id:        e.ID,
		Type:      e.Type,
		SchemaRef: e.SchemaRef,
		Source:    e.Source,
		Payload:   e.Payload,
	}
	if !e.Time.IsZero() {
		out.Time = e.Time.UTC().Format(time.RFC3339)
	}
	if e.BlobRef != nil {
		out.BlobRef = *e.BlobRef
	}
	return out
}
