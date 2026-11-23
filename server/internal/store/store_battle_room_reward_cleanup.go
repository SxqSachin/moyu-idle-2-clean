package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) battleRoomHasPendingRewardWorkTx(ctx context.Context, tx pgx.Tx, roomID int64) (bool, error) {
	var pending bool
	err := tx.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM battle_runs run
				JOIN battle_settlement_jobs job ON job.run_id = run.id
				WHERE run.room_id = $1 AND job.status <> 'done'
			)
			OR EXISTS (
				SELECT 1
				FROM battle_runs run
				JOIN battle_attr_exp_jobs job ON job.run_id = run.id
				WHERE run.room_id = $1 AND job.applied_at IS NULL
			)
			OR EXISTS (
				SELECT 1
				FROM battle_runs run
				JOIN battle_skill_exp_jobs job ON job.run_id = run.id
				WHERE run.room_id = $1 AND job.status <> 'done'
			)
	`, roomID).Scan(&pending)
	return pending, err
}

// deleteEmptyBattleRoomAfterRewardJobsTx removes a room retained by its last
// member leaving in the same transaction that finishes the last reward job.
func (s *Store) deleteEmptyBattleRoomAfterRewardJobsTx(ctx context.Context, tx pgx.Tx, runID int64) (bool, error) {
	if runID <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	var roomID int64
	err := tx.QueryRow(ctx, `
			SELECT run.room_id
			FROM battle_runs run
			JOIN battle_rooms room ON room.id = run.room_id
			WHERE run.id = $1
			  AND room.phase = 'reward_cleanup'
			FOR UPDATE OF room
	`, runID).Scan(&roomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var hasMembers bool
	if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM battle_room_members WHERE room_id = $1
			)
	`, roomID).Scan(&hasMembers); err != nil {
		return false, err
	}
	if hasMembers {
		return false, nil
	}
	pending, err := s.battleRoomHasPendingRewardWorkTx(ctx, tx, roomID)
	if err != nil || pending {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM battle_rooms WHERE id = $1`, roomID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
