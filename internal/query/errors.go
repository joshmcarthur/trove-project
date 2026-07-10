package query

import "errors"

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errors.New("query: event not found")

// ErrEmptyQuery is returned when search_events is called without a query.
var ErrEmptyQuery = errors.New("query: search query is required")
