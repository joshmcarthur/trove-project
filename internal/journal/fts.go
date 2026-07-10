package journal

import (
	"database/sql"
	"fmt"
	"strings"
)

const ftsSchemaDDL = `
CREATE VIRTUAL TABLE IF NOT EXISTS events_fts USING fts5(
  event_id UNINDEXED,
  type,
  source,
  payload,
  tokenize = 'porter'
);
`

func migrateFTS(db *sql.DB) error {
	var ftsCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events_fts`).Scan(&ftsCount); err != nil {
		return fmt.Errorf("journal: count fts rows: %w", err)
	}
	if ftsCount > 0 {
		return nil
	}

	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&eventCount); err != nil {
		return fmt.Errorf("journal: count events: %w", err)
	}
	if eventCount == 0 {
		return nil
	}

	if _, err := db.Exec(`
		INSERT INTO events_fts (event_id, type, source, payload)
		SELECT id, type, source, payload FROM events`); err != nil {
		return fmt.Errorf("journal: backfill fts: %w", err)
	}

	return nil
}

// formatFTSQuery turns user text into a safe FTS5 MATCH expression. Each
// whitespace-separated token is quoted so FTS operators are not interpreted.
func formatFTSQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	fields := strings.Fields(text)
	quoted := make([]string, len(fields))
	for i, field := range fields {
		quoted[i] = `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " ")
}
