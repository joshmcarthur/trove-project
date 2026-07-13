package query

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

const maxNotableRevisions = 5

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
func (s *Service) GetRevision(ctx context.Context, id string) (Revision, error) {
	e, err := s.Journal.Get(ctx, id)
	if err != nil {
		if errors.Is(err, journal.ErrNotFound) {
			return Revision{}, ErrNotFound
		}
		return Revision{}, err
	}
	return revisionFromJournal(e), nil
}

// SearchEvents performs FTS5 keyword search over the journal.
func (s *Service) SearchRevisions(ctx context.Context, query string, params SearchParams) ([]Revision, error) {
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

	out := make([]Revision, len(events))
	for i, e := range events {
		out[i] = revisionFromJournal(e)
	}
	return out, nil
}

// GetEventsByType returns events with the exact type, optionally narrowed by time range.
func (s *Service) GetRevisionsByType(ctx context.Context, eventType string, timeFrom, timeTo *time.Time) ([]Revision, error) {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return nil, ErrEmptyType
	}
	if timeFrom != nil && timeTo != nil && timeFrom.After(*timeTo) {
		return nil, ErrInvalidTimeRange
	}

	events, err := s.Journal.Query(ctx, journal.Filter{
		Type:     eventType,
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Revision, len(events))
	for i, e := range events {
		out[i] = revisionFromJournal(e)
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

	notable := selectNotableRevisions(events, maxNotableRevisions)

	return Summary{
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
		Total:    len(events),
		ByType:   byType,
		Notable:  notable,
	}, nil
}

func selectNotableRevisions(events []journal.Revision, limit int) []Revision {
	if len(events) == 0 || limit <= 0 {
		return nil
	}

	sorted := make([]journal.Revision, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Time.After(sorted[j].Time)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	out := make([]Revision, len(sorted))
	for i, e := range sorted {
		out[i] = revisionFromJournal(e)
	}
	return out
}
