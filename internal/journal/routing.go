package journal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const routerWatermarkKey = "last_dispatched_id"

const routingSchemaDDL = `
CREATE TABLE IF NOT EXISTS router_state (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS event_dispatch (
  event_id TEXT PRIMARY KEY,
  root_id  TEXT NOT NULL,
  seen     TEXT NOT NULL
);
`

// QueryAfter returns up to limit events with id strictly greater than afterID,
// ordered by id ascending. An empty afterID returns from the beginning.
func (s *Store) QueryAfter(ctx context.Context, afterID string, limit int) ([]Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("journal: query after: store is closed")
	}
	if limit <= 0 {
		limit = 1
	}

	query := `
		SELECT id, time, type, source, payload, blob_ref
		FROM events`
	args := []any{}
	if afterID != "" {
		query += ` WHERE id > ?`
		args = append(args, afterID)
	}
	query += ` ORDER BY id ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("journal: query after: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("journal: query after: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("journal: query after: %w", err)
	}

	return events, nil
}

// LoadRouterWatermark returns the last successfully dispatched event id, or "" if unset.
func (s *Store) LoadRouterWatermark(ctx context.Context) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("journal: load router watermark: store is closed")
	}

	var value string
	err := s.db.QueryRowContext(ctx, `
		SELECT value FROM router_state WHERE key = ?`, routerWatermarkKey).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("journal: load router watermark: %w", err)
	}
	return value, nil
}

// SaveRouterWatermark persists the last successfully dispatched event id.
func (s *Store) SaveRouterWatermark(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("journal: save router watermark: store is closed")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO router_state (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		routerWatermarkKey, id)
	if err != nil {
		return fmt.Errorf("journal: save router watermark: %w", err)
	}
	return nil
}

// SaveEventDispatch stores routing metadata for a derived event awaiting dispatch.
func (s *Store) SaveEventDispatch(ctx context.Context, eventID, rootID string, seen []string) error {
	if s.db == nil {
		return fmt.Errorf("journal: save event dispatch: store is closed")
	}
	if eventID == "" {
		return fmt.Errorf("journal: save event dispatch: event id is required")
	}
	if rootID == "" {
		return fmt.Errorf("journal: save event dispatch: root id is required")
	}

	seenJSON, err := json.Marshal(seen)
	if err != nil {
		return fmt.Errorf("journal: save event dispatch: marshal seen: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO event_dispatch (event_id, root_id, seen) VALUES (?, ?, ?)
		ON CONFLICT(event_id) DO UPDATE SET root_id = excluded.root_id, seen = excluded.seen`,
		eventID, rootID, string(seenJSON))
	if err != nil {
		return fmt.Errorf("journal: save event dispatch: %w", err)
	}
	return nil
}

// LoadEventDispatch returns persisted routing metadata for a derived event.
func (s *Store) LoadEventDispatch(ctx context.Context, eventID string) (rootID string, seen []string, ok bool, err error) {
	if s.db == nil {
		return "", nil, false, fmt.Errorf("journal: load event dispatch: store is closed")
	}

	var seenJSON string
	err = s.db.QueryRowContext(ctx, `
		SELECT root_id, seen FROM event_dispatch WHERE event_id = ?`, eventID).Scan(&rootID, &seenJSON)
	if err == sql.ErrNoRows {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, fmt.Errorf("journal: load event dispatch: %w", err)
	}
	if err := json.Unmarshal([]byte(seenJSON), &seen); err != nil {
		return "", nil, false, fmt.Errorf("journal: load event dispatch: unmarshal seen: %w", err)
	}
	return rootID, seen, true, nil
}

// DeleteEventDispatch removes persisted routing metadata after successful dispatch.
func (s *Store) DeleteEventDispatch(ctx context.Context, eventID string) error {
	if s.db == nil {
		return fmt.Errorf("journal: delete event dispatch: store is closed")
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM event_dispatch WHERE event_id = ?`, eventID)
	if err != nil {
		return fmt.Errorf("journal: delete event dispatch: %w", err)
	}
	return nil
}
