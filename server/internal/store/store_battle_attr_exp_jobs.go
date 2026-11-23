package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	BattleAttrExpJobStatusPending     = "pending"
	BattleAttrExpJobStatusRunning     = "running"
	BattleAttrExpJobStatusDone        = "done"
	BattleAttrExpJobStatusFailed      = "failed"
	BattleAttrExpJobStatusQuarantined = "quarantined"
)

type BattleAttrExpJobApplyMetrics struct {
	TransactionElapsed    time.Duration
	JobLockElapsed        time.Duration
	RewardApplyElapsed    time.Duration
	UserLockElapsed       time.Duration
	PlayerAttrElapsed     time.Duration
	KittyAttrElapsed      time.Duration
	DoneMarkElapsed       time.Duration
	Users                 int
	Kitties               int
	PlayerAttrKeys        int
	KittyAttrKeys         int
	PlayerProgressChanges int
	KittyProgressChanges  int
	PlayerLevelChanges    int
	KittyLevelChanges     int
}

func (s *Store) EnqueueBattleAttrExpJob(ctx context.Context, runID int64, payload []byte, now time.Time) error {
	if runID <= 0 || len(payload) == 0 {
		return ErrBattleRoomConfigInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := validatePostgresJSONBRaw("battle attribute experience job payload", payload); err != nil {
		return err
	}
	return s.enqueueBattleAttrExpJobTx(ctx, s.db, runID, payload, now)
}

func (s *Store) enqueueBattleAttrExpJobTx(
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
	if err := validatePostgresJSONBRaw("battle attribute experience job payload", payload); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO battle_attr_exp_jobs(run_id, status, payload_jsonb, attempts, available_at, locked_by, locked_until, created_at, updated_at)
		VALUES($1, $2, $3::jsonb, 0, $4, '', NULL, now(), now())
		ON CONFLICT (run_id) DO NOTHING
	`, runID, BattleAttrExpJobStatusPending, string(payload), now)
	return err
}

func (s *Store) ClaimBattleAttrExpJobs(ctx context.Context, workerID string, limit int, leaseDuration time.Duration, now time.Time) ([]BattleAttrExpJob, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return []BattleAttrExpJob{}, nil
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
			FROM battle_attr_exp_jobs
			WHERE applied_at IS NULL
			  AND (
					(status IN ('pending', 'failed') AND available_at <= $2)
					OR (status = 'running' AND locked_until IS NOT NULL AND locked_until < $2)
				)
			  AND (locked_until IS NULL OR locked_until < $2)
			ORDER BY available_at ASC, id ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		UPDATE battle_attr_exp_jobs AS j
		SET status = 'running',
		    attempts = attempts + 1,
		    locked_by = $1,
		    locked_until = $4,
		    updated_at = now()
		FROM candidates c
		WHERE j.id = c.id
		RETURNING j.id, j.run_id, j.status, j.payload_jsonb, j.attempts, j.last_error, j.available_at, j.locked_by, j.locked_until, j.applied_at, j.created_at, j.updated_at
	`, workerID, now, limit, lockedUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleAttrExpJob, 0, limit)
	for rows.Next() {
		var job BattleAttrExpJob
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
			&job.AppliedAt,
			&job.CreatedAt,
			&job.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// ApplyBattleAttrExpJob commits the attribute rewards and the done marker in one
// transaction. A retry after an ambiguous commit observes applied_at and cannot
// grant the same run twice.
func (s *Store) ApplyBattleAttrExpJob(
	ctx context.Context,
	jobID int64,
	workerID string,
	claimAttempt int,
	rewards map[int64]BattleWaveReward,
) (BattleAttrExpJobApplyMetrics, error) {
	metrics := BattleAttrExpJobApplyMetrics{}
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || claimAttempt <= 0 {
		return metrics, ErrBattleRoomConfigInvalid
	}
	seeds := s.attributeSeedsFromGameData()
	startedAt := time.Now()
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var (
			runID     int64
			status    string
			appliedAt *time.Time
			lockedBy  string
			attempts  int
		)
		stageStartedAt := time.Now()
		err := tx.QueryRow(ctx, `
			SELECT run_id, status, applied_at, locked_by, attempts
			FROM battle_attr_exp_jobs
			WHERE id = $1
			FOR UPDATE
		`, jobID).Scan(&runID, &status, &appliedAt, &lockedBy, &attempts)
		metrics.JobLockElapsed += time.Since(stageStartedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleRoomConfigInvalid
			}
			return err
		}
		if status == BattleAttrExpJobStatusDone || appliedAt != nil {
			return nil
		}
		if status != BattleAttrExpJobStatusRunning || lockedBy != workerID || attempts != claimAttempt {
			return ErrBattleAttrExpJobLeaseLost
		}
		// Attribute rewards are background work. Never occupy a database
		// connection waiting behind a player-facing transaction, and never hold
		// one party member while waiting for another. Lock every user in stable
		// order first; a busy user rolls the whole job back for a fenced retry.
		stageStartedAt = time.Now()
		for _, userID := range sortedBattleRewardUserIDs(rewards) {
			locked, lockErr := s.tryLockUserTx(ctx, tx, userID)
			if lockErr != nil {
				metrics.UserLockElapsed += time.Since(stageStartedAt)
				return lockErr
			}
			if !locked {
				metrics.UserLockElapsed += time.Since(stageStartedAt)
				return ErrBattleAttrExpUserBusy
			}
		}
		metrics.UserLockElapsed += time.Since(stageStartedAt)
		stageStartedAt = time.Now()
		applyErr := s.applyBattleWaveAttrRewardsWithMetricsTx(ctx, tx, rewards, seeds, &metrics)
		metrics.RewardApplyElapsed += time.Since(stageStartedAt)
		if applyErr != nil {
			return applyErr
		}
		stageStartedAt = time.Now()
		commandTag, err := tx.Exec(ctx, `
			UPDATE battle_attr_exp_jobs
			SET status = $2,
			    applied_at = now(),
			    last_error = '',
			    locked_by = '',
			    locked_until = NULL,
			    updated_at = now()
			WHERE id = $1
			  AND status = $3
			  AND applied_at IS NULL
			  AND locked_by = $4
			  AND attempts = $5
		`, jobID, BattleAttrExpJobStatusDone, BattleAttrExpJobStatusRunning, workerID, claimAttempt)
		metrics.DoneMarkElapsed += time.Since(stageStartedAt)
		if err != nil {
			return err
		}
		if commandTag.RowsAffected() != 1 {
			return ErrBattleRoomConfigInvalid
		}
		if _, err := s.deleteEmptyBattleRoomAfterRewardJobsTx(ctx, tx, runID); err != nil {
			return err
		}
		return nil
	})
	metrics.TransactionElapsed = time.Since(startedAt)
	return metrics, err
}

func (s *Store) MarkBattleAttrExpJobFailed(
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
		UPDATE battle_attr_exp_jobs
		SET status = $2,
		    last_error = $3,
		    available_at = $4,
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND applied_at IS NULL
		  AND status = $5
		  AND locked_by = $6
		  AND attempts = $7
	`, jobID, BattleAttrExpJobStatusFailed, errText, retryAt, BattleAttrExpJobStatusRunning, workerID, claimAttempt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) MarkBattleAttrExpJobQuarantined(
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
		UPDATE battle_attr_exp_jobs
		SET status = $2,
		    last_error = $3,
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND applied_at IS NULL
		  AND status = $4
		  AND locked_by = $5
		  AND attempts = $6
	`, jobID, BattleAttrExpJobStatusQuarantined, errText, BattleAttrExpJobStatusRunning, workerID, claimAttempt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
