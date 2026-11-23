package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type offlineAchievementBatchKey struct{}

// Prepared event identities and payloads are retained verbatim. The worker
// cannot observe them before COMMIT, so batching writes changes no event input.
type offlineAchievementBatch struct {
	store    *Store
	tx       pgx.Tx
	userID   int64
	rows     []achievementEventInsertRow
	inserted bool
}

func (b *offlineAchievementBatch) append(ctx context.Context, rows []achievementEventInsertRow) error {
	for len(rows) > 0 {
		count := min(achievementEventInsertBatchSize-len(b.rows), len(rows))
		b.rows = append(b.rows, rows[:count]...)
		rows = rows[count:]
		if len(b.rows) == achievementEventInsertBatchSize {
			if err := b.flush(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *offlineAchievementBatch) flush(ctx context.Context) error {
	if b == nil || len(b.rows) == 0 {
		return nil
	}
	inserted, err := b.store.insertAchievementEventRowsTx(ctx, b.tx, b.rows)
	if err != nil {
		return err
	}
	b.inserted = b.inserted || inserted
	clear(b.rows)
	b.rows = b.rows[:0]
	return nil
}
