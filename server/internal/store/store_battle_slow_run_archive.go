package store

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const battleSlowRunArchiveSaveSlowLogThreshold = 50 * time.Millisecond

func clampBattleSlowRunArchiveListLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (s *Store) UpsertBattleSlowRunArchive(ctx context.Context, input BattleSlowRunArchiveInput) error {
	if input.RoomID <= 0 || input.RunID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	totalStartedAt := time.Now()
	marshalStartedAt := time.Now()
	triggerFlagsRaw, err := json.Marshal(input.TriggerFlags)
	triggerFlagsMarshalElapsed := time.Since(marshalStartedAt)
	if err != nil {
		return err
	}
	marshalStartedAt = time.Now()
	turnProfileRaw, err := json.Marshal(input.TurnProfile)
	turnProfileMarshalElapsed := time.Since(marshalStartedAt)
	if err != nil {
		return err
	}
	marshalStartedAt = time.Now()
	compileInputRaw, err := json.Marshal(input.CompileInput)
	compileInputMarshalElapsed := time.Since(marshalStartedAt)
	if err != nil {
		return err
	}
	marshalStartedAt = time.Now()
	checkpointRaw, err := json.Marshal(input.Checkpoint)
	checkpointMarshalElapsed := time.Since(marshalStartedAt)
	if err != nil {
		return err
	}
	marshalStartedAt = time.Now()
	timelineRaw, err := json.Marshal(input.Timeline)
	timelineMarshalElapsed := time.Since(marshalStartedAt)
	if err != nil {
		return err
	}
	marshalStartedAt = time.Now()
	finalResultRaw, err := json.Marshal(input.FinalResult)
	finalResultMarshalElapsed := time.Since(marshalStartedAt)
	if err != nil {
		return err
	}
	execStartedAt := time.Now()
	_, err = s.db.Exec(ctx, `
		INSERT INTO battle_slow_run_archives(
			room_id,
			run_id,
			dungeon_id,
			worker_id,
			source,
			wave,
			outcome,
			queue_since_start_ms,
			compile_elapsed_ms,
			total_ms,
			total_steps,
			total_turns,
			timeline_events,
			final_seq,
			player_units,
			guard_timed_out,
			guard_step_limited,
			trigger_flags_jsonb,
			turn_profile_jsonb,
			compile_input_jsonb,
			checkpoint_jsonb,
			timeline_jsonb,
			final_result_jsonb,
			run_started_at,
			captured_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18::jsonb, $19::jsonb, $20::jsonb,
			$21::jsonb, $22::jsonb, $23::jsonb, $24, now(), now()
		)
		ON CONFLICT (room_id, run_id) DO UPDATE
		SET dungeon_id = EXCLUDED.dungeon_id,
		    worker_id = EXCLUDED.worker_id,
		    source = EXCLUDED.source,
		    wave = EXCLUDED.wave,
		    outcome = EXCLUDED.outcome,
		    queue_since_start_ms = EXCLUDED.queue_since_start_ms,
		    compile_elapsed_ms = EXCLUDED.compile_elapsed_ms,
		    total_ms = EXCLUDED.total_ms,
		    total_steps = EXCLUDED.total_steps,
		    total_turns = EXCLUDED.total_turns,
		    timeline_events = EXCLUDED.timeline_events,
		    final_seq = EXCLUDED.final_seq,
		    player_units = EXCLUDED.player_units,
		    guard_timed_out = EXCLUDED.guard_timed_out,
		    guard_step_limited = EXCLUDED.guard_step_limited,
		    trigger_flags_jsonb = EXCLUDED.trigger_flags_jsonb,
		    turn_profile_jsonb = EXCLUDED.turn_profile_jsonb,
		    compile_input_jsonb = EXCLUDED.compile_input_jsonb,
		    checkpoint_jsonb = EXCLUDED.checkpoint_jsonb,
		    timeline_jsonb = EXCLUDED.timeline_jsonb,
		    final_result_jsonb = EXCLUDED.final_result_jsonb,
		    run_started_at = EXCLUDED.run_started_at,
		    updated_at = now()
	`, input.RoomID, input.RunID, strings.TrimSpace(input.DungeonID), strings.TrimSpace(input.WorkerID), strings.TrimSpace(input.Source), input.Wave, strings.TrimSpace(input.Outcome), input.QueueSinceStartMs, input.CompileElapsedMs, input.TotalMs, input.TotalSteps, input.TotalTurns, input.TimelineEvents, input.FinalSeq, input.PlayerUnits, input.GuardTimedOut, input.GuardStepLimited, string(triggerFlagsRaw), string(turnProfileRaw), string(compileInputRaw), string(checkpointRaw), string(timelineRaw), string(finalResultRaw), input.RunStartedAt)
	dbExecElapsed := time.Since(execStartedAt)
	totalElapsed := time.Since(totalStartedAt)
	if totalElapsed >= battleSlowRunArchiveSaveSlowLogThreshold ||
		dbExecElapsed >= battleSlowRunArchiveSaveSlowLogThreshold ||
		finalResultMarshalElapsed >= battleSlowRunArchiveSaveSlowLogThreshold ||
		len(timelineRaw) >= 64*1024 ||
		len(finalResultRaw) >= 64*1024 {
		log.Printf(
			"battle slow run archive save slow room_id=%d run_id=%d marshal_trigger_flags_ms=%d marshal_turn_profile_ms=%d marshal_compile_input_ms=%d marshal_checkpoint_ms=%d marshal_timeline_ms=%d marshal_final_result_ms=%d marshal_total_ms=%d db_exec_ms=%d total_ms=%d trigger_flags_bytes=%d turn_profile_bytes=%d compile_input_bytes=%d checkpoint_bytes=%d timeline_bytes=%d final_result_bytes=%d",
			input.RoomID,
			input.RunID,
			triggerFlagsMarshalElapsed.Milliseconds(),
			turnProfileMarshalElapsed.Milliseconds(),
			compileInputMarshalElapsed.Milliseconds(),
			checkpointMarshalElapsed.Milliseconds(),
			timelineMarshalElapsed.Milliseconds(),
			finalResultMarshalElapsed.Milliseconds(),
			(triggerFlagsMarshalElapsed + turnProfileMarshalElapsed + compileInputMarshalElapsed + checkpointMarshalElapsed + timelineMarshalElapsed + finalResultMarshalElapsed).Milliseconds(),
			dbExecElapsed.Milliseconds(),
			totalElapsed.Milliseconds(),
			len(triggerFlagsRaw),
			len(turnProfileRaw),
			len(compileInputRaw),
			len(checkpointRaw),
			len(timelineRaw),
			len(finalResultRaw),
		)
	}
	return err
}

func (s *Store) TrimBattleSlowRunArchivesByRoom(ctx context.Context, roomID int64, keepCount int) error {
	if roomID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	if keepCount <= 0 {
		keepCount = 5
	}
	_, err := s.db.Exec(ctx, `
		WITH kept AS (
			SELECT id
			FROM battle_slow_run_archives
			WHERE room_id = $1
			ORDER BY id DESC
			LIMIT $2
		),
		doomed AS (
			SELECT id
			FROM battle_slow_run_archives
			WHERE room_id = $1
			  AND id NOT IN (SELECT id FROM kept)
		)
		DELETE FROM battle_slow_run_archives b
		USING doomed d
		WHERE b.id = d.id
	`, roomID, keepCount)
	return err
}

func (s *Store) PurgeBattleSlowRunArchives(ctx context.Context, keepDays, batchSize int) (int64, error) {
	if keepDays <= 0 {
		keepDays = 30
	}
	if batchSize <= 0 {
		batchSize = 1000
	}
	tag, err := s.db.Exec(ctx, `
		WITH doomed AS (
			SELECT id
			FROM battle_slow_run_archives
			WHERE captured_at < now() - ($1 * INTERVAL '1 day')
			ORDER BY id ASC
			LIMIT $2
		)
		DELETE FROM battle_slow_run_archives b
		USING doomed d
		WHERE b.id = d.id
	`, keepDays, batchSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) ListBattleSlowRunArchives(ctx context.Context, filter BattleSlowRunArchiveFilter) ([]BattleSlowRunArchiveSummary, error) {
	limit := clampBattleSlowRunArchiveListLimit(filter.Limit)
	rows, err := s.db.Query(ctx, `
		SELECT id, room_id, run_id, dungeon_id, worker_id, source, wave, outcome,
		       queue_since_start_ms, compile_elapsed_ms, total_ms, total_steps, total_turns,
		       timeline_events, final_seq, player_units, guard_timed_out, guard_step_limited,
		       trigger_flags_jsonb, turn_profile_jsonb, run_started_at, captured_at, updated_at
		FROM battle_slow_run_archives
		WHERE ($1::bigint <= 0 OR room_id = $1)
		  AND ($2::bigint <= 0 OR run_id = $2)
		  AND ($3::bigint <= 0 OR id < $3)
		ORDER BY id DESC
		LIMIT $4
	`, filter.RoomID, filter.RunID, filter.BeforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BattleSlowRunArchiveSummary, 0, limit)
	for rows.Next() {
		var item BattleSlowRunArchiveSummary
		var triggerFlagsRaw []byte
		var turnProfileRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.RoomID,
			&item.RunID,
			&item.DungeonID,
			&item.WorkerID,
			&item.Source,
			&item.Wave,
			&item.Outcome,
			&item.QueueSinceStartMs,
			&item.CompileElapsedMs,
			&item.TotalMs,
			&item.TotalSteps,
			&item.TotalTurns,
			&item.TimelineEvents,
			&item.FinalSeq,
			&item.PlayerUnits,
			&item.GuardTimedOut,
			&item.GuardStepLimited,
			&triggerFlagsRaw,
			&turnProfileRaw,
			&item.RunStartedAt,
			&item.CapturedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.TriggerFlags = map[string]any{}
		if len(triggerFlagsRaw) > 0 {
			_ = json.Unmarshal(triggerFlagsRaw, &item.TriggerFlags)
		}
		item.TurnProfile = map[string]any{}
		if len(turnProfileRaw) > 0 {
			_ = json.Unmarshal(turnProfileRaw, &item.TurnProfile)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetBattleSlowRunArchiveByID(ctx context.Context, id int64) (BattleSlowRunArchive, error) {
	if id <= 0 {
		return BattleSlowRunArchive{}, ErrBattleRoomConfigInvalid
	}
	var item BattleSlowRunArchive
	var (
		triggerFlagsRaw []byte
		turnProfileRaw  []byte
		compileInputRaw []byte
		checkpointRaw   []byte
		timelineRaw     []byte
		finalResultRaw  []byte
	)
	err := s.db.QueryRow(ctx, `
		SELECT id, room_id, run_id, dungeon_id, worker_id, source, wave, outcome,
		       queue_since_start_ms, compile_elapsed_ms, total_ms, total_steps, total_turns,
		       timeline_events, final_seq, player_units, guard_timed_out, guard_step_limited,
		       trigger_flags_jsonb, turn_profile_jsonb, compile_input_jsonb, checkpoint_jsonb,
		       timeline_jsonb, final_result_jsonb, run_started_at, captured_at, updated_at
		FROM battle_slow_run_archives
		WHERE id = $1
	`, id).Scan(
		&item.ID,
		&item.RoomID,
		&item.RunID,
		&item.DungeonID,
		&item.WorkerID,
		&item.Source,
		&item.Wave,
		&item.Outcome,
		&item.QueueSinceStartMs,
		&item.CompileElapsedMs,
		&item.TotalMs,
		&item.TotalSteps,
		&item.TotalTurns,
		&item.TimelineEvents,
		&item.FinalSeq,
		&item.PlayerUnits,
		&item.GuardTimedOut,
		&item.GuardStepLimited,
		&triggerFlagsRaw,
		&turnProfileRaw,
		&compileInputRaw,
		&checkpointRaw,
		&timelineRaw,
		&finalResultRaw,
		&item.RunStartedAt,
		&item.CapturedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleSlowRunArchive{}, ErrBattleSlowRunArchiveNotFound
	}
	if err != nil {
		return BattleSlowRunArchive{}, err
	}
	item.TriggerFlags = map[string]any{}
	if len(triggerFlagsRaw) > 0 {
		_ = json.Unmarshal(triggerFlagsRaw, &item.TriggerFlags)
	}
	item.TurnProfile = map[string]any{}
	if len(turnProfileRaw) > 0 {
		_ = json.Unmarshal(turnProfileRaw, &item.TurnProfile)
	}
	item.CompileInput = map[string]any{}
	if len(compileInputRaw) > 0 {
		_ = json.Unmarshal(compileInputRaw, &item.CompileInput)
	}
	item.Checkpoint = map[string]any{}
	if len(checkpointRaw) > 0 {
		_ = json.Unmarshal(checkpointRaw, &item.Checkpoint)
	}
	item.Timeline = []map[string]any{}
	if len(timelineRaw) > 0 {
		_ = json.Unmarshal(timelineRaw, &item.Timeline)
	}
	item.FinalResult = map[string]any{}
	if len(finalResultRaw) > 0 {
		_ = json.Unmarshal(finalResultRaw, &item.FinalResult)
	}
	if item.RunID > 0 && len(item.CompileInput) == 0 && len(item.Timeline) == 0 && len(item.FinalResult) == 0 {
		if payload, err := s.GetBattleRunColdPayloadByID(ctx, item.RunID); err == nil {
			item.CompileInput = payload.CompileInput
			item.Timeline = payload.Timeline
			item.FinalResult = payload.FinalResult
			if carryState, carryErr := s.GetBattleRunCarryStateByID(ctx, item.RunID); carryErr == nil && len(carryState) > 0 {
				item.FinalResult = mergeBattleRunResult(nil, item.FinalResult, carryState)
			}
		}
		if run, err := s.GetBattleRunByID(ctx, item.RunID); err == nil && len(item.Checkpoint) == 0 {
			item.Checkpoint = run.State
		}
	}
	return item, nil
}
