package query

import (
	"context"
	"errors"

	"github.com/joshmcarthur/trove/internal/journal"
)

// Service implements the internal query RPC API over a journal.
type Service struct {
	Journal journal.Journal
}

// GetEvent returns the event with id.
func (s *Service) GetEvent(ctx context.Context, id string) (Event, error) {
	e, err := s.Journal.Get(ctx, id)
	if err != nil {
		if errors.Is(err, journal.ErrNotFound) {
			return Event{}, ErrNotFound
		}
		return Event{}, err
	}
	return eventFromJournal(e), nil
}
