package journal

import (
	"database/sql"
	"fmt"
)

func migrateSchema(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(events)`)
	if err != nil {
		return fmt.Errorf("journal: migrate schema: table_info: %w", err)
	}
	defer rows.Close()

	var hasSchemaRef bool
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
			return fmt.Errorf("journal: migrate schema: scan table_info: %w", err)
		}
		if name == "schema_ref" {
			hasSchemaRef = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("journal: migrate schema: table_info: %w", err)
	}

	if hasSchemaRef {
		return nil
	}

	if _, err := db.Exec(`ALTER TABLE events ADD COLUMN schema_ref TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("journal: migrate schema: add schema_ref: %w", err)
	}

	return nil
}
