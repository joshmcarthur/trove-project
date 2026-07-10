package query

import "errors"

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errors.New("query: event not found")
