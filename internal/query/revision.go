package query

import (
	"encoding/json"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

// Revision is a JSON-serializable journal revision returned by the query API.
type Revision struct {
	ID         string          `json:"id"`
	Time       time.Time       `json:"time"`
	Operation  string          `json:"operation,omitempty"`
	RecordRef  string          `json:"record_ref,omitempty"`
	Type       string          `json:"type"`
	SchemaRef  string          `json:"schema_ref,omitempty"`
	Source     string          `json:"source"`
	Producer   string          `json:"producer,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	BlobRef    *string         `json:"blob_ref,omitempty"`
	Transforms json.RawMessage `json:"transforms,omitempty"`
}

func revisionFromJournal(r journal.Revision) Revision {
	return Revision{
		ID:         r.ID,
		Time:       r.Time,
		Operation:  r.Operation,
		RecordRef:  r.RecordRef,
		Type:       r.Type,
		SchemaRef:  r.SchemaRef,
		Source:     r.Source,
		Producer:   r.Producer,
		Payload:    r.Payload,
		BlobRef:    r.BlobRef,
		Transforms: r.Transforms,
	}
}
