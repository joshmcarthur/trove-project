package journal

import (
	"context"
	"database/sql"
	"fmt"
)

// AppendHook runs inside the append transaction after the revision row is inserted.
type AppendHook func(ctx context.Context, tx *sql.Tx, r Revision) error

// AppendTransactional persists r and runs hook in the same transaction.
func (s *Store) AppendTransactional(ctx context.Context, r Revision, hook AppendHook) error {
	if err := prepareAppend(&r); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("journal: append: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := appendRevisionInTx(ctx, tx, r); err != nil {
		return err
	}
	if hook != nil {
		if err := hook(ctx, tx, r); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("journal: append: commit: %w", err)
	}

	s.signalWatchers()
	return nil
}
