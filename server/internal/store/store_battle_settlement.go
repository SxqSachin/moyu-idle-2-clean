package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	BattleSettlementJobStatusPending     = "pending"
	BattleSettlementJobStatusRunning     = "running"
	BattleSettlementJobStatusDone        = "done"
	BattleSettlementJobStatusFailed      = "failed"
	BattleSettlementJobStatusQuarantined = "quarantined"
)

func (s *Store) EnqueueBattleRunSettlement(ctx context.Context, roomID, runID int64, now time.Time, fallbackTickAt time.Time) error {
	if roomID <= 0 || runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	if fallbackTickAt.IsZero() {
		fallbackTickAt = now.Add(30 * time.Second)
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE battle_runs
			SET status = $3,
			    updated_at = now()
			WHERE id = $1
			  AND room_id = $2
			  AND status IN ($4, $3)
			  AND COALESCE(playback_state_jsonb->>'stage', '') = 'playback_finished'
			  AND COALESCE(NULLIF(playback_state_jsonb->>'nextEventIdx', '')::int, 0) >= COALESCE(NULLIF(playback_state_jsonb->>'totalEvents', '')::int, 0)
			  AND COALESCE(NULLIF(playback_state_jsonb->>'lastPushedSeq', '')::bigint, 0) >= COALESCE(NULLIF(playback_state_jsonb->>'finalSeq', '')::bigint, 0)
		`, runID, roomID, BattleRunStatusSettling, BattleRunStatusRunning)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		var schemaVersion int
		if err := tx.QueryRow(ctx, `
			SELECT schema_version
			FROM battle_rooms
			WHERE id = $1
			  AND current_run_id = $2
			FOR UPDATE
		`, roomID, runID).Scan(&schemaVersion); err != nil {
			return err
		}
		phase := ""
		if schemaVersion == 2 {
			phase = "settling"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE battle_rooms
			SET owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = $3,
			    phase = $4,
			    updated_at = now()
			WHERE id = $1
			  AND current_run_id = $2
		`, roomID, runID, fallbackTickAt, phase); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO battle_settlement_jobs(room_id, run_id, status, attempts, available_at, locked_by, locked_until, created_at, updated_at)
			VALUES($1, $2, $3, 0, $4, '', NULL, now(), now())
			ON CONFLICT (run_id) DO NOTHING
		`, roomID, runID, BattleSettlementJobStatusPending, now)
		return err
	})
}

func (s *Store) ClaimBattleSettlementJobs(ctx context.Context, workerID string, limit int, leaseDuration time.Duration, now time.Time) ([]BattleSettlementJob, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return []BattleSettlementJob{}, nil
	}
	if limit <= 0 {
		limit = 1
	}
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockedUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM battle_settlement_jobs
			WHERE (
					(status IN ($3, $4) AND available_at <= $2)
					OR (status = $6 AND locked_until IS NOT NULL AND locked_until < $2)
				)
			  AND (locked_until IS NULL OR locked_until < $2)
			ORDER BY available_at ASC, id ASC
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		)
		UPDATE battle_settlement_jobs AS j
		SET status = $6,
		    attempts = attempts + 1,
		    locked_by = $1,
		    locked_until = $7,
		    updated_at = now()
		FROM candidates c
		WHERE j.id = c.id
		RETURNING j.id, j.room_id, j.run_id, j.status, j.attempts, j.last_error, j.available_at, j.locked_by, j.locked_until, j.created_at, j.updated_at
	`, workerID, now, BattleSettlementJobStatusPending, BattleSettlementJobStatusFailed, limit, BattleSettlementJobStatusRunning, lockedUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleSettlementJob, 0, limit)
	for rows.Next() {
		job, scanErr := scanBattleSettlementJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func (s *Store) MarkBattleSettlementJobDone(ctx context.Context, jobID int64, workerID string, claimAttempt int) (bool, error) {
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || claimAttempt <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	marked := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var runID int64
		err := tx.QueryRow(ctx, `
			UPDATE battle_settlement_jobs
			SET status = $2,
			    last_error = '',
			    locked_by = '',
			    locked_until = NULL,
			    updated_at = now()
			WHERE id = $1
			  AND status = $3
			  AND locked_by = $4
			  AND attempts = $5
			RETURNING run_id
		`, jobID, BattleSettlementJobStatusDone, BattleSettlementJobStatusRunning, workerID, claimAttempt).Scan(&runID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		marked = true
		_, err = s.deleteEmptyBattleRoomAfterRewardJobsTx(ctx, tx, runID)
		return err
	})
	return marked, err
}

func (s *Store) MarkBattleSettlementJobFailed(ctx context.Context, jobID int64, workerID string, claimAttempt int, errText string, retryAt time.Time) (bool, error) {
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || claimAttempt <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	if retryAt.IsZero() {
		retryAt = time.Now().Add(time.Second)
	}
	errText = strings.TrimSpace(errText)
	if len(errText) > 1000 {
		errText = errText[:1000]
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE battle_settlement_jobs
		SET status = $2,
		    last_error = $3,
		    available_at = $4,
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND status = $5
		  AND locked_by = $6
		  AND attempts = $7
	`, jobID, BattleSettlementJobStatusFailed, errText, retryAt, BattleSettlementJobStatusRunning, workerID, claimAttempt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) MarkBattleSettlementJobQuarantined(ctx context.Context, jobID int64, workerID string, claimAttempt int, errText string) (bool, error) {
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || claimAttempt <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	errText = strings.TrimSpace(errText)
	if len(errText) > 1000 {
		errText = errText[:1000]
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE battle_settlement_jobs
		SET status = $2,
		    last_error = $3,
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND status = $4
		  AND locked_by = $5
		  AND attempts = $6
	`, jobID, BattleSettlementJobStatusQuarantined, errText, BattleSettlementJobStatusRunning, workerID, claimAttempt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func scanBattleSettlementJob(rows pgx.Rows) (BattleSettlementJob, error) {
	var job BattleSettlementJob
	if err := rows.Scan(
		&job.ID,
		&job.RoomID,
		&job.RunID,
		&job.Status,
		&job.Attempts,
		&job.LastError,
		&job.AvailableAt,
		&job.LockedBy,
		&job.LockedUntil,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return BattleSettlementJob{}, err
	}
	return job, nil
}
