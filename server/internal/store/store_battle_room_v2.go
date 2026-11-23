package store

import (
	"context"
	"strings"
	"time"
)

// UpdateBattleRoomPhase updates only the phase column for a room.
// Used by v2 dispatch to transition phase without changing status.
// No-op if roomID <= 0.
func (s *Store) UpdateBattleRoomPhase(ctx context.Context, roomID int64, phase string) error {
	if roomID <= 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE battle_rooms SET phase = $2, updated_at = now() WHERE id = $1`, roomID, phase)
	return err
}

const scheduleBattleCompileRetrySQL = `
	UPDATE battle_rooms
	SET next_tick_at = $4,
	    owner_worker_id = '',
	    lease_until = $4
	WHERE id = $1
	  AND current_run_id = $2
	  AND schema_version = 2
	  AND status = 'playing'
	  AND phase = 'compiling'
	  AND (
	    owner_worker_id = $3
	    OR owner_worker_id = ''
	    OR lease_until IS NULL
	    OR lease_until < now()
	  )
`

// ScheduleBattleCompileRetry releases a failed v2 compile claim until retryAt.
// Matching both room and run prevents a stale compile task from delaying a
// newer run after the room has already advanced.
func (s *Store) ScheduleBattleCompileRetry(
	ctx context.Context,
	roomID int64,
	runID int64,
	workerID string,
	retryAt time.Time,
) (bool, error) {
	workerID = strings.TrimSpace(workerID)
	if roomID <= 0 || runID <= 0 || workerID == "" {
		return false, ErrBattleRoomConfigInvalid
	}
	if retryAt.IsZero() {
		retryAt = time.Now().Add(time.Second)
	}
	tag, err := s.db.Exec(ctx, scheduleBattleCompileRetrySQL, roomID, runID, workerID, retryAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// battleRoomTargetSchemaVersion returns the schema version a room should have
// after transitioning to newStatus. If v2 is enabled and the room is v1 and
// the new status is a "resting" state (no active battle), upgrade to v2.
func (s *Store) battleRoomTargetSchemaVersion(currentVersion int, newStatus string) int {
	if s.battleStateMachineV2 && currentVersion < 2 {
		switch newStatus {
		case "idle", "preparing", "recovering":
			return 2
		}
	}
	return currentVersion
}

// V2 claim functions partition rooms by schema_version=2 + phase.
// These are used exclusively by the v2 dispatch loops (battle_loop_v2.go).
// They NEVER overlap with v1 claim functions (which filter schema_version=1).

// BattleRoomV2Claim is the narrow row shape needed by v2 dispatch loops.
// Claim queries intentionally avoid returning the full battle_rooms row.
type BattleRoomV2Claim struct {
	ID                int64
	DungeonID         string
	IsFogMode         bool
	Status            string
	CurrentRunID      *int64
	EnteringStartedAt *time.Time
	UpdatedAt         time.Time
	SchemaVersion     int
}

// AsBattleRoom adapts a claim to existing runtime helpers. Only claim fields are populated.
func (claim BattleRoomV2Claim) AsBattleRoom() BattleRoom {
	return BattleRoom{
		ID:                claim.ID,
		DungeonID:         claim.DungeonID,
		IsFogMode:         claim.IsFogMode,
		Status:            claim.Status,
		CurrentRunID:      claim.CurrentRunID,
		EnteringStartedAt: claim.EnteringStartedAt,
		UpdatedAt:         claim.UpdatedAt,
		SchemaVersion:     claim.SchemaVersion,
	}
}

const battleRoomV2ClaimReturningSQL = `
		r.id,
		r.dungeon_id,
		r.is_fog_mode,
		r.status,
		r.current_run_id,
		r.entering_started_at,
		r.updated_at,
		r.schema_version`

const claimBattleStateAdvanceRoomsV2SQL = `
	WITH candidates AS (
		SELECT r.id
		FROM battle_rooms r
		WHERE r.schema_version = 2
		  AND r.status IN ('entering', 'recovering')
		  AND r.phase = ''
		  AND r.next_tick_at <= $1
		  AND (
			r.lease_until IS NULL
			OR r.lease_until < $1
			OR r.owner_worker_id = $2
		  )
		ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
		LIMIT $3
		FOR UPDATE OF r SKIP LOCKED
	)
	UPDATE battle_rooms AS r
	SET owner_worker_id = $2,
	    lease_until = $4
	FROM candidates c
	WHERE r.id = c.id
	RETURNING ` + battleRoomV2ClaimReturningSQL

const claimBattleReadyNextRoomsV2SQL = `
	WITH candidates AS (
		SELECT r.id
		FROM battle_rooms r
		WHERE r.schema_version = 2
		  AND r.status = 'playing'
		  AND r.phase = 'awaiting_run'
		  AND r.current_run_id IS NULL
		  AND r.next_tick_at <= $1
		  AND (
			r.lease_until IS NULL
			OR r.lease_until < $1
			OR r.owner_worker_id = $2
		  )
		ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
		LIMIT $3
		FOR UPDATE OF r SKIP LOCKED
	)
	UPDATE battle_rooms AS r
	SET owner_worker_id = $2,
	    lease_until = $4
	FROM candidates c
	WHERE r.id = c.id
	RETURNING ` + battleRoomV2ClaimReturningSQL

const claimBattleCompilingRoomsV2SQL = `
	WITH candidates AS (
		SELECT r.id
		FROM battle_rooms r
		WHERE r.schema_version = 2
		  AND r.status = 'playing'
		  AND r.phase = 'compiling'
		  AND r.next_tick_at <= $1
		  AND (
			r.lease_until IS NULL
			OR r.lease_until < $1
		  )
		ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
		LIMIT $3
		FOR UPDATE OF r SKIP LOCKED
	)
	UPDATE battle_rooms AS r
	SET owner_worker_id = $2,
	    lease_until = $4
	FROM candidates c
	WHERE r.id = c.id
	RETURNING ` + battleRoomV2ClaimReturningSQL

const claimBattlePlaybackRoomsV2SQL = `
	WITH candidates AS (
		SELECT r.id
		FROM battle_rooms r
		WHERE r.schema_version = 2
		  AND r.status = 'playing'
		  AND r.phase = 'playback'
		  AND r.next_tick_at <= $1
		  AND (
			r.lease_until IS NULL
			OR r.lease_until < $1
			OR r.owner_worker_id = $2
		  )
		ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
		LIMIT $3
		FOR UPDATE OF r SKIP LOCKED
	)
	UPDATE battle_rooms AS r
	SET owner_worker_id = $2,
	    lease_until = $4
	FROM candidates c
	WHERE r.id = c.id
	RETURNING ` + battleRoomV2ClaimReturningSQL

// All v2 claim statements share the same parameter order: now, worker, limit, lease expiry.
func (s *Store) claimBattleRoomsV2(
	ctx context.Context,
	query string,
	workerID string,
	limit int,
	defaultLimit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoomV2Claim, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return []BattleRoomV2Claim{}, nil
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	rows, err := s.db.Query(ctx, query, now, workerID, limit, now.Add(leaseDuration))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BattleRoomV2Claim, 0, limit)
	for rows.Next() {
		var claim BattleRoomV2Claim
		if scanErr := rows.Scan(
			&claim.ID,
			&claim.DungeonID,
			&claim.IsFogMode,
			&claim.Status,
			&claim.CurrentRunID,
			&claim.EnteringStartedAt,
			&claim.UpdatedAt,
			&claim.SchemaVersion,
		); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, claim)
	}
	return out, rows.Err()
}

// ClaimBattleStateAdvanceRoomsV2 claims v2 rooms in entering or recovering states.
// These are non-playing states where phase is always empty.
func (s *Store) ClaimBattleStateAdvanceRoomsV2(
	ctx context.Context,
	workerID string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoomV2Claim, error) {
	return s.claimBattleRoomsV2(ctx, claimBattleStateAdvanceRoomsV2SQL, workerID, limit, 200, leaseDuration, now)
}

// ClaimBattleReadyNextRoomsV2 claims v2 rooms waiting for the next run creation.
// These rooms have status='playing' AND phase='awaiting_run'.
func (s *Store) ClaimBattleReadyNextRoomsV2(
	ctx context.Context,
	workerID string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoomV2Claim, error) {
	return s.claimBattleRoomsV2(ctx, claimBattleReadyNextRoomsV2SQL, workerID, limit, 32, leaseDuration, now)
}

// ClaimBattleCompilingRoomsV2 claims v2 rooms that are waiting for compile kickoff.
// These rooms have status='playing' AND phase='compiling'.
func (s *Store) ClaimBattleCompilingRoomsV2(
	ctx context.Context,
	workerID string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoomV2Claim, error) {
	return s.claimBattleRoomsV2(ctx, claimBattleCompilingRoomsV2SQL, workerID, limit, 32, leaseDuration, now)
}

// ClaimBattlePlaybackRoomsV2 claims v2 rooms with active playback workers.
// These rooms have status='playing' AND phase='playback'.
func (s *Store) ClaimBattlePlaybackRoomsV2(
	ctx context.Context,
	workerID string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoomV2Claim, error) {
	return s.claimBattleRoomsV2(ctx, claimBattlePlaybackRoomsV2SQL, workerID, limit, 32, leaseDuration, now)
}
