package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// The caller holds the user transaction lock. Paused lanes are included because
// they can still carry historical starts when they resume.
func (s *Store) ensureOperationEffectChargeReplenishmentCaughtUpTx(ctx context.Context, tx pgx.Tx, userID int64, now time.Time) error {
	var due bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM player_action_queue
			WHERE user_id = $1 AND status = 'running'
			  AND (current_run_ends_at IS NULL OR current_run_ends_at <= $2)
		)
	`, userID, now).Scan(&due); err != nil {
		return err
	}
	if due {
		return ErrOperationEffectRecoveryPending
	}
	return nil
}
