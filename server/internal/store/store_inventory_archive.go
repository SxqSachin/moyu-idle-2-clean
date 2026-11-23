package store

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const InventoryArchiveBatchLimit = 1000

type InventoryArchiveResult struct {
	UpdatedCount int64
}

func normalizeArchiveInstanceIDs(instanceIDs []int64) ([]int64, error) {
	if len(instanceIDs) == 0 || len(instanceIDs) > InventoryArchiveBatchLimit {
		return nil, ErrBadQuantity
	}
	seen := make(map[int64]struct{}, len(instanceIDs))
	out := make([]int64, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		if id <= 0 {
			return nil, ErrBadQuantity
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, ErrBadQuantity
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (s *Store) SetItemInstancesArchived(ctx context.Context, userID int64, instanceIDs []int64, archived bool) (InventoryArchiveResult, error) {
	ids, err := normalizeArchiveInstanceIDs(instanceIDs)
	if err != nil {
		return InventoryArchiveResult{}, err
	}
	var out InventoryArchiveResult
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT instance_id, user_id, state
			FROM player_item_instances
			WHERE instance_id = ANY($1)
			ORDER BY instance_id
			FOR UPDATE
		`, ids)
		if err != nil {
			return err
		}
		locked := make(map[int64]struct{}, len(ids))
		for rows.Next() {
			var instanceID int64
			var ownerID int64
			var state string
			if err := rows.Scan(&instanceID, &ownerID, &state); err != nil {
				rows.Close()
				return err
			}
			locked[instanceID] = struct{}{}
			if ownerID != userID {
				rows.Close()
				return ErrInstanceNotOwned
			}
			if state != "inventory" {
				rows.Close()
				return ErrInvalidInstanceState
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(locked) != len(ids) {
			return ErrInstanceNotOwned
		}
		var tag pgconn.CommandTag
		if archived {
			tag, err = execArchiveUpdate(ctx, tx, userID, ids)
		} else {
			tag, err = execUnarchiveUpdate(ctx, tx, userID, ids)
		}
		if err != nil {
			return err
		}
		out.UpdatedCount = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return InventoryArchiveResult{}, err
	}
	return out, nil
}

func execArchiveUpdate(ctx context.Context, tx pgx.Tx, userID int64, ids []int64) (pgconn.CommandTag, error) {
	return tx.Exec(ctx, `
		UPDATE player_item_instances
		SET archived_at = now(), updated_at = now()
		WHERE user_id = $1
		  AND instance_id = ANY($2)
		  AND state = 'inventory'
		  AND archived_at IS NULL
	`, userID, ids)
}

func execUnarchiveUpdate(ctx context.Context, tx pgx.Tx, userID int64, ids []int64) (pgconn.CommandTag, error) {
	return tx.Exec(ctx, `
		UPDATE player_item_instances
		SET archived_at = NULL, updated_at = now()
		WHERE user_id = $1
		  AND instance_id = ANY($2)
		  AND state = 'inventory'
		  AND archived_at IS NOT NULL
	`, userID, ids)
}
