package journal

import (
	"encoding/json"
	"errors"
	"time"
)

// Operation names persisted on journal events.
const (
	OpApply  = "apply"
	OpDelete = "delete"
)

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errors.New("journal: event not found")

// ErrConflictingFilter is returned when a Filter sets mutually exclusive fields.
var ErrConflictingFilter = errors.New("journal: type and type_prefix are mutually exclusive")

// Event is an immutable journal record.
type Event struct {
	ID         string
	Time       time.Time
	Operation  string
	RecordRef  string
	Type       string // optional; empty means unset
	SchemaRef  string
	Source     string
	Payload    json.RawMessage
	BlobRef    *string
	Transforms json.RawMessage
}

// Filter constrains journal reads. Text performs FTS5 keyword search when set.
type Filter struct {
	Type       string
	TypePrefix string
	Source     string
	TimeFrom   *time.Time
	TimeTo     *time.Time
	Text       string
}
