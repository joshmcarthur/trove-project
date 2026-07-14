package journal

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/oklog/ulid"
)

// migrateSchema upgrades legacy journal databases.
func migrateSchema(db *sql.DB) error {
	if err := migratePreRecordsShape(db); err != nil {
		return err
	}
	if err := migrateEventsToRevisions(db); err != nil {
		return err
	}
	return migrateReplayOrdering(db)
}

// migratePreRecordsShape wipes pre-records databases without an operation column.
func migratePreRecordsShape(db *sql.DB) error {
	hasOperation, err := tableHasColumn(db, "events", "operation")
	if err != nil {
		return err
	}
	if !tableExists(db, "events") {
		return nil
	}
	if hasOperation {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("journal: migrate pre-records: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DROP TABLE IF EXISTS events_fts`); err != nil {
		return fmt.Errorf("journal: migrate pre-records: drop events_fts: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS events`); err != nil {
		return fmt.Errorf("journal: migrate pre-records: drop events: %w", err)
	}
	return tx.Commit()
}

// migrateEventsToRevisions renames legacy events-schema tables to revisions naming.
func migrateEventsToRevisions(db *sql.DB) error {
	if !tableExists(db, "events") || tableExists(db, "revisions") {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("journal: migrate events to revisions: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`ALTER TABLE events RENAME TO revisions`,
		`DROP INDEX IF EXISTS idx_events_time`,
		`DROP INDEX IF EXISTS idx_events_type`,
		`DROP INDEX IF EXISTS idx_events_source`,
		`DROP INDEX IF EXISTS idx_events_operation`,
		`DROP INDEX IF EXISTS idx_events_record_ref`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_time ON revisions(time)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_type ON revisions(type)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_source ON revisions(source)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_operation ON revisions(operation)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_record_ref ON revisions(record_ref)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("journal: migrate events to revisions: %s: %w", stmt, err)
		}
	}

	if tableExistsTx(tx, "record_events") {
		if _, err := tx.Exec(`ALTER TABLE record_events RENAME TO record_revisions`); err != nil {
			return fmt.Errorf("journal: migrate record_events: %w", err)
		}
		if _, err := tx.Exec(`ALTER TABLE record_revisions RENAME COLUMN event_id TO revision_id`); err != nil {
			return fmt.Errorf("journal: migrate record_revisions column: %w", err)
		}
		_, _ = tx.Exec(`DROP INDEX IF EXISTS idx_record_events_event_id`)
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_record_revisions_revision_id ON record_revisions(revision_id)`); err != nil {
			return fmt.Errorf("journal: migrate record_revisions index: %w", err)
		}
	}

	if tableExistsTx(tx, "event_dispatch") {
		if _, err := tx.Exec(`ALTER TABLE event_dispatch RENAME TO revision_dispatch`); err != nil {
			return fmt.Errorf("journal: migrate event_dispatch: %w", err)
		}
		if _, err := tx.Exec(`ALTER TABLE revision_dispatch RENAME COLUMN event_id TO revision_id`); err != nil {
			return fmt.Errorf("journal: migrate revision_dispatch column: %w", err)
		}
	}

	if tableExistsTx(tx, "events_fts") {
		if _, err := tx.Exec(`DROP TABLE events_fts`); err != nil {
			return fmt.Errorf("journal: migrate drop events_fts: %w", err)
		}
		if _, err := tx.Exec(ftsSchemaDDL); err != nil {
			return fmt.Errorf("journal: migrate create revisions_fts: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO revisions_fts (revision_id, type, source, payload)
			SELECT id, type, source, payload FROM revisions`); err != nil {
			return fmt.Errorf("journal: migrate backfill revisions_fts: %w", err)
		}
	}

	return tx.Commit()
}

func tableExists(db *sql.DB, name string) bool {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return err == nil && n > 0
}

func tableExistsTx(tx *sql.Tx, name string) bool {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return err == nil && n > 0
}

func tableHasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`) //nolint:gosec // G201 table from const
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

// migrateReplayOrdering adds recorded_at and sequence columns and backfills existing rows.
func migrateReplayOrdering(db *sql.DB) error {
	if !tableExists(db, "revisions") {
		return nil
	}

	hasSequence, err := tableHasColumn(db, "revisions", "sequence")
	if err != nil {
		return err
	}
	if hasSequence {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("journal: migrate replay ordering: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`ALTER TABLE revisions ADD COLUMN recorded_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE revisions ADD COLUMN sequence INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_record_sequence ON revisions(record_ref, sequence)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("journal: migrate replay ordering: %s: %w", stmt, err)
		}
	}

	rows, err := tx.Query(`
		SELECT id, time, record_ref
		FROM revisions
		ORDER BY record_ref ASC, time ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("journal: migrate replay ordering: list revisions: %w", err)
	}
	defer rows.Close()

	type row struct {
		id        string
		timeStr   string
		recordRef string
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.timeStr, &r.recordRef); err != nil {
			return fmt.Errorf("journal: migrate replay ordering: scan: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("journal: migrate replay ordering: list revisions: %w", err)
	}

	seqByRecord := make(map[string]int)
	for _, r := range all {
		seqByRecord[r.recordRef]++
		seq := seqByRecord[r.recordRef]

		eventTime, err := time.Parse(time.RFC3339, r.timeStr)
		if err != nil {
			return fmt.Errorf("journal: migrate replay ordering: parse time %q: %w", r.timeStr, err)
		}
		recordedAt := recordedAtFromID(r.id, eventTime)

		if _, err := tx.Exec(`
			UPDATE revisions
			SET sequence = ?, recorded_at = ?
			WHERE id = ?`, seq, recordedAt.UTC().Format(time.RFC3339), r.id); err != nil {
			return fmt.Errorf("journal: migrate replay ordering: update %q: %w", r.id, err)
		}
	}

	return tx.Commit()
}

func recordedAtFromID(id string, fallback time.Time) time.Time {
	parsed, err := ulid.Parse(id)
	if err != nil {
		return fallback.UTC()
	}
	return ulid.Time(parsed.Time()).UTC()
}
