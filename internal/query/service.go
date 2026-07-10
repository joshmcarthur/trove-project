package query

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

const maxNotableEvents = 5

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

// SummarizeRange returns aggregated counts and a sample of notable events for a time window.
func (s *Service) SummarizeRange(ctx context.Context, timeFrom, timeTo time.Time) (Summary, error) {
	if timeFrom.After(timeTo) {
		return Summary{}, ErrInvalidTimeRange
	}

	events, err := s.Journal.Query(ctx, journal.Filter{
		TimeFrom: &timeFrom,
		TimeTo:   &timeTo,
	})
	if err != nil {
		return Summary{}, err
	}

	byType := make(map[string]int)
	for _, e := range events {
		byType[e.Type]++
	}

	notable := selectNotableEvents(events, maxNotableEvents)

	return Summary{
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
		Total:    len(events),
		ByType:   byType,
		Notable:  notable,
	}, nil
}

func selectNotableEvents(events []journal.Event, limit int) []Event {
	if len(events) == 0 || limit <= 0 {
		return nil
	}

	sorted := make([]journal.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Time.After(sorted[j].Time)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	out := make([]Event, len(sorted))
	for i, e := range sorted {
		out[i] = eventFromJournal(e)
	}
	return out
}
