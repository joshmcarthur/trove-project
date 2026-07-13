package query

import "errors"

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errors.New("query: event not found")

// ErrRecordNotFound is returned when a record_ref does not exist or the requested version is unavailable.
var ErrRecordNotFound = errors.New("query: record not found")

// ErrEmptyRecordRef is returned when get_record is called without a record_ref.
var ErrEmptyRecordRef = errors.New("query: record_ref is required")

// ErrEmptyQuery is returned when search_events is called without a query.
var ErrEmptyQuery = errors.New("query: search query is required")

// ErrEmptyType is returned when get_events_by_type is called without a type.
var ErrEmptyType = errors.New("query: event type is required")

// ErrInvalidTimeRange is returned when time_from is after time_to.
var ErrInvalidTimeRange = errors.New("query: time_from must not be after time_to")
