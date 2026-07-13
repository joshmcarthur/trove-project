package records

import (
	"encoding/json"
	"time"
)

// Completeness values for record_heads.
const (
	CompletenessIncomplete = "incomplete"
	CompletenessComplete   = "complete"
	CompletenessDeleted    = "deleted"
)

// Head is the current folded state for a record_ref.
type Head struct {
	RecordRef    string
	Version      int
	Completeness string
	Type         string
	Source       string
	Body         json.RawMessage
	ContentRef   *string
	UpdatedAt    time.Time
}
