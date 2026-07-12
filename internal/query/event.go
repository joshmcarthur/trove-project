package query

import (
	"encoding/json"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

// Event is a JSON-serializable journal event returned by the query API.
type Event struct {
	ID        string          `json:"id"`
	Time      time.Time       `json:"time"`
	Type      string          `json:"type"`
	SchemaRef string          `json:"schema_ref,omitempty"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
	BlobRef   *string         `json:"blob_ref,omitempty"`
}

func eventFromJournal(e journal.Event) Event {
	return Event{
		ID:        e.ID,
		Time:      e.Time,
		Type:      e.Type,
		SchemaRef: e.SchemaRef,
		Source:    e.Source,
		Payload:   e.Payload,
		BlobRef:   e.BlobRef,
	}
}
