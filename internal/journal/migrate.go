package journal

import (
	"database/sql"
	"fmt"
)

// migrateSchema upgrades the events table to the records-layer journal shape.
// Pre-records databases without an operation column are wiped destructively
// (events_fts rows and events dropped, then recreated). This is intentional
// during active development — dev journals must be recreated after upgrade.
func migrateSchema(db *sql.DB) error {
	hasOperation, err := eventsTableHasColumn(db, "operation")
	if err != nil {
		return err
	}
	if hasOperation {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("journal: migrate schema: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DROP TABLE IF EXISTS events_fts`); err != nil {
		return fmt.Errorf("journal: migrate schema: drop events_fts: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS events`); err != nil {
		return fmt.Errorf("journal: migrate schema: drop events: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("journal: migrate schema: commit: %w", err)
	}

	return nil
}

func eventsTableHasColumn(db *sql.DB, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(events)`)
	if err != nil {
		return false, fmt.Errorf("journal: migrate schema: table_info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			typ       string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("journal: migrate schema: scan table_info: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("journal: migrate schema: table_info: %w", err)
	}

	return false, nil
}
