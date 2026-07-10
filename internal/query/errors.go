package query

import "errors"

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errors.New("query: event not found")

// ErrEmptyQuery is returned when search_events is called without a query.
var ErrEmptyQuery = errors.New("query: search query is required")

// ErrEmptyType is returned when get_events_by_type is called without a type.
var ErrEmptyType = errors.New("query: event type is required")

// ErrInvalidTimeRange is returned when time_from is after time_to.
var ErrInvalidTimeRange = errors.New("query: time_from must not be after time_to")
