package query

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

// Service implements the internal query RPC API over a journal.
type Service struct {
	Journal journal.Journal
}

// SearchParams optionally narrows an FTS search.
type SearchParams struct {
	TypePrefix string
	Source     string
	TimeFrom   *time.Time
	TimeTo     *time.Time
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

// SearchEvents performs FTS5 keyword search over the journal.
func (s *Service) SearchEvents(ctx context.Context, query string, params SearchParams) ([]Event, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}

	events, err := s.Journal.Query(ctx, journal.Filter{
		Text:       query,
		TypePrefix: params.TypePrefix,
		Source:     params.Source,
		TimeFrom:   params.TimeFrom,
		TimeTo:     params.TimeTo,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Event, len(events))
	for i, e := range events {
		out[i] = eventFromJournal(e)
	}
	return out, nil
}
