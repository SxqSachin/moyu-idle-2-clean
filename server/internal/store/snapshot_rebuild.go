package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *Store) RebuildAllEquipmentSnapshots(ctx context.Context, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	var (
		lastID int64
		total  int64
	)
	for {
		tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return total, err
		}
		batchCount := 0
		rows, err := tx.Query(ctx, `
			SELECT instance_id, user_id
			FROM player_item_instances
			WHERE instance_id > $1
			ORDER BY instance_id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		`, lastID, batchSize)
		if err != nil {
			_ = tx.Rollback(ctx)
			return total, err
		}
		for rows.Next() {
			var (
				instanceID int64
				userID     int64
			)
			if err = rows.Scan(&instanceID, &userID); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return total, err
			}
			if err = s.refreshInstanceBaseSnapshotTx(ctx, tx, instanceID, userID); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return total, err
			}
			lastID = instanceID
			batchCount++
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return total, err
		}
		rows.Close()
		if err = tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return total, err
		}
		total += int64(batchCount)
		if batchCount == 0 {
			break
		}
	}
	return total, nil
}
