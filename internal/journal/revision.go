package journal

import (
	"encoding/json"
	"errors"
	"time"
)

// Operation names persisted on journal revisions.
const (
	OpApply  = "apply"
	OpDelete = "delete"
)

// ErrNotFound is returned when a revision id does not exist.
var ErrNotFound = errors.New("journal: revision not found")

// ErrConflictingFilter is returned when a Filter sets mutually exclusive fields.
var ErrConflictingFilter = errors.New("journal: type and type_prefix are mutually exclusive")

// Revision is an immutable append-only journal row.
type Revision struct {
	ID         string
	Time       time.Time
	RecordedAt time.Time // host commit time; distinct from source event time
	Sequence   int       // monotonic per record_ref for deterministic replay
	Operation  string
	RecordRef  string
	Type       string // optional; empty means unset
	SchemaRef  string
	Source     string
	Producer   string // host-stamped module identity; modules must not set
	Payload    json.RawMessage
	BlobRef    *string
	Transforms json.RawMessage
	References json.RawMessage // nil = unchanged on apply; [] = clear; list = replace
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
