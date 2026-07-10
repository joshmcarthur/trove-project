package journal

import (
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errors.New("journal: event not found")

// Event is an immutable journal record.
type Event struct {
	ID      string
	Time    time.Time
	Type    string
	Source  string
	Payload json.RawMessage
	BlobRef *string
}

// Filter constrains journal reads. Unused until Query and Subscribe are implemented.
type Filter struct {
	TypePrefix string
	Source     string
	TimeFrom   *time.Time
	TimeTo     *time.Time
	Text       string
}
