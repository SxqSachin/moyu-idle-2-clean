package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	BattleSkillExpJobStatusPending     = "pending"
	BattleSkillExpJobStatusRunning     = "running"
	BattleSkillExpJobStatusDone        = "done"
	BattleSkillExpJobStatusFailed      = "failed"
	BattleSkillExpJobStatusQuarantined = "quarantined"
)

func (s *Store) EnqueueBattleSkillExpJob(ctx context.Context, runID int64, payload []byte, now time.Time) error {
	if runID <= 0 || len(payload) == 0 {
		return ErrBattleRoomConfigInvalid
	}
	return s.enqueueBattleSkillExpJobTx(ctx, s.db, runID, payload, now)
}

func (s *Store) enqueueBattleSkillExpJobTx(
	ctx context.Context,
	db battleRunPayloadDB,
	runID int64,
	payload []byte,
	now time.Time,
) error {
	if len(payload) == 0 {
		return nil
	}
	if runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := validatePostgresJSONBRaw("battle skill experience job payload", payload); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO battle_skill_exp_jobs(run_id, status, payload_jsonb, attempts, available_at, locked_by, locked_until, created_at, updated_at)
		VALUES($1, $2, $3::jsonb, 0, $4, '', NULL, now(), now())
		ON CONFLICT (run_id) DO NOTHING
	`, runID, BattleSkillExpJobStatusPending, string(payload), now)
	return err
}

func (s *Store) ClaimBattleSkillExpJobs(ctx context.Context, workerID string, limit int, leaseDuration time.Duration, now time.Time) ([]BattleSkillExpJob, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return []BattleSkillExpJob{}, nil
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
			FROM battle_skill_exp_jobs
			WHERE (
					(status IN ($3, $4) AND available_at <= $2)
					OR (status = $6 AND locked_until IS NOT NULL AND locked_until < $2)
				)
			  AND (locked_until IS NULL OR locked_until < $2)
			ORDER BY available_at ASC, id ASC
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		)
		UPDATE battle_skill_exp_jobs AS j
		SET status = $6,
		    attempts = attempts + 1,
		    locked_by = $1,
		    locked_until = $7,
		    updated_at = now()
		FROM candidates c
		WHERE j.id = c.id
		RETURNING j.id, j.run_id, j.status, j.payload_jsonb, j.attempts, j.last_error, j.available_at, j.locked_by, j.locked_until, j.created_at, j.updated_at
	`, workerID, now, BattleSkillExpJobStatusPending, BattleSkillExpJobStatusFailed, limit, BattleSkillExpJobStatusRunning, lockedUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleSkillExpJob, 0, limit)
	for rows.Next() {
		var job BattleSkillExpJob
		if scanErr := rows.Scan(
			&job.ID,
			&job.RunID,
			&job.Status,
			&job.Payload,
			&job.Attempts,
			&job.LastError,
			&job.AvailableAt,
			&job.LockedBy,
			&job.LockedUntil,
			&job.CreatedAt,
			&job.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// ApplyBattleSkillExpJob commits the skill rewards and the done marker in one
// transaction so a retry after an ambiguous commit cannot grant them twice.
func (s *Store) ApplyBattleSkillExpJob(
	ctx context.Context,
	jobID int64,
	workerID string,
	claimAttempt int,
	rewards map[int64]map[string]int64,
	kittyRewards map[int64]map[string]int64,
) error {
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || claimAttempt <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var (
			runID    int64
			status   string
			lockedBy string
			attempts int
		)
		err := tx.QueryRow(ctx, `
			SELECT run_id, status, locked_by, attempts
			FROM battle_skill_exp_jobs
			WHERE id = $1
			FOR UPDATE
		`, jobID).Scan(&runID, &status, &lockedBy, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomConfigInvalid
		}
		if err != nil {
			return err
		}
		if status == BattleSkillExpJobStatusDone {
			return nil
		}
		if status != BattleSkillExpJobStatusRunning || lockedBy != workerID || attempts != claimAttempt {
			return ErrBattleSkillExpJobLeaseLost
		}
		if err := s.applyBattleSkillExpRewardsTx(ctx, tx, rewards, kittyRewards); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE battle_skill_exp_jobs
			SET status = $2,
			    last_error = '',
			    locked_by = '',
			    locked_until = NULL,
			    updated_at = now()
			WHERE id = $1
			  AND status = $3
			  AND locked_by = $4
			  AND attempts = $5
		`, jobID, BattleSkillExpJobStatusDone, BattleSkillExpJobStatusRunning, workerID, claimAttempt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrBattleSkillExpJobLeaseLost
		}
		if _, err := s.deleteEmptyBattleRoomAfterRewardJobsTx(ctx, tx, runID); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) MarkBattleSkillExpJobFailed(
	ctx context.Context,
	jobID int64,
	workerID string,
	claimAttempt int,
	errText string,
	retryAt time.Time,
) (bool, error) {
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
		UPDATE battle_skill_exp_jobs
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
	`, jobID, BattleSkillExpJobStatusFailed, errText, retryAt, BattleSkillExpJobStatusRunning, workerID, claimAttempt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) MarkBattleSkillExpJobQuarantined(
	ctx context.Context,
	jobID int64,
	workerID string,
	claimAttempt int,
	errText string,
) (bool, error) {
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || claimAttempt <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	errText = strings.TrimSpace(errText)
	if len(errText) > 1000 {
		errText = errText[:1000]
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE battle_skill_exp_jobs
		SET status = $2,
		    last_error = $3,
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND status = $4
		  AND locked_by = $5
		  AND attempts = $6
	`, jobID, BattleSkillExpJobStatusQuarantined, errText, BattleSkillExpJobStatusRunning, workerID, claimAttempt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
