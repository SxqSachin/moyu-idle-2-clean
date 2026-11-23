package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type battleRunPayloadDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (s *Store) UpsertBattleRunColdPayload(ctx context.Context, runID int64, compileInput map[string]any, timeline []map[string]any, finalResult map[string]any) error {
	return s.upsertBattleRunColdPayload(ctx, s.db, runID, compileInput, timeline, finalResult)
}

func (s *Store) upsertBattleRunColdPayload(ctx context.Context, db battleRunPayloadDB, runID int64, compileInput map[string]any, timeline []map[string]any, finalResult map[string]any) error {
	if runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	compileInputRaw, err := json.Marshal(compileInput)
	if err != nil {
		return err
	}
	timelineRaw, err := json.Marshal(timeline)
	if err != nil {
		return err
	}
	finalResultRaw, err := json.Marshal(finalResult)
	if err != nil {
		return err
	}
	return s.upsertBattleRunColdPayloadRaw(ctx, db, runID, compileInputRaw, timelineRaw, finalResultRaw)
}

// upsertBattleRunColdPayloadRaw accepts pre-marshaled JSON bytes to avoid
// holding the DB transaction lock during JSON marshaling.
func (s *Store) upsertBattleRunColdPayloadRaw(ctx context.Context, db battleRunPayloadDB, runID int64, compileInputRaw, timelineRaw, finalResultRaw []byte) error {
	if runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	_, err := db.Exec(ctx, `
		INSERT INTO battle_run_payloads(
			run_id,
			compile_input_jsonb,
			timeline_jsonb,
			final_result_jsonb,
			compile_input_raw,
			timeline_raw,
			final_result_raw,
			created_at,
			updated_at
		)
		VALUES($1, '{}'::jsonb, '[]'::jsonb, '{}'::jsonb, $2, $3, $4, now(), now())
		ON CONFLICT (run_id) DO UPDATE
		SET compile_input_jsonb = '{}'::jsonb,
		    timeline_jsonb = '[]'::jsonb,
		    final_result_jsonb = '{}'::jsonb,
		    compile_input_raw = EXCLUDED.compile_input_raw,
		    timeline_raw = EXCLUDED.timeline_raw,
		    final_result_raw = EXCLUDED.final_result_raw,
		    updated_at = now()
	`, runID, compileInputRaw, timelineRaw, finalResultRaw)
	return err
}

func (s *Store) UpsertBattleRunCarryState(ctx context.Context, runID int64, carryState map[string]any) error {
	return s.upsertBattleRunCarryState(ctx, s.db, runID, carryState)
}

func (s *Store) upsertBattleRunCarryState(ctx context.Context, db battleRunPayloadDB, runID int64, carryState map[string]any) error {
	if runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	raw, err := json.Marshal(carryState)
	if err != nil {
		return err
	}
	return s.upsertBattleRunCarryStateRaw(ctx, db, runID, raw)
}

// upsertBattleRunCarryStateRaw accepts pre-marshaled JSON bytes to avoid
// holding the DB transaction lock during JSON marshaling.
func (s *Store) upsertBattleRunCarryStateRaw(ctx context.Context, db battleRunPayloadDB, runID int64, carryStateRaw []byte) error {
	if runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	if err := validatePostgresJSONBRaw("battle run carry state", carryStateRaw); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO battle_run_carry_states(run_id, carry_state_jsonb, created_at, updated_at)
		VALUES($1, $2::jsonb, now(), now())
		ON CONFLICT (run_id) DO UPDATE
		SET carry_state_jsonb = EXCLUDED.carry_state_jsonb,
		    updated_at = now()
	`, runID, string(carryStateRaw))
	return err
}

func (s *Store) GetBattleRunColdPayloadByID(ctx context.Context, runID int64) (BattleRunColdPayload, error) {
	if runID <= 0 {
		return BattleRunColdPayload{}, ErrBattleRoomConfigInvalid
	}
	var item BattleRunColdPayload
	var compileInputRawBytes []byte
	var timelineRawBytes []byte
	var finalResultRawBytes []byte
	var compileInputRaw []byte
	var timelineRaw []byte
	var finalResultRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT run_id,
		       COALESCE(compile_input_raw, ''::bytea),
		       COALESCE(timeline_raw, ''::bytea),
		       COALESCE(final_result_raw, ''::bytea),
		       compile_input_jsonb,
		       timeline_jsonb,
		       final_result_jsonb
		FROM battle_run_payloads
		WHERE run_id = $1
	`, runID).Scan(&item.RunID, &compileInputRawBytes, &timelineRawBytes, &finalResultRawBytes, &compileInputRaw, &timelineRaw, &finalResultRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleRunColdPayload{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattleRunColdPayload{}, err
	}
	if len(compileInputRawBytes) > 0 {
		compileInputRaw = compileInputRawBytes
	}
	if len(timelineRawBytes) > 0 {
		timelineRaw = timelineRawBytes
	}
	if len(finalResultRawBytes) > 0 {
		finalResultRaw = finalResultRawBytes
	}
	item.CompileInput = map[string]any{}
	if len(compileInputRaw) > 0 {
		_ = json.Unmarshal(compileInputRaw, &item.CompileInput)
	}
	item.Timeline = []map[string]any{}
	if len(timelineRaw) > 0 {
		_ = json.Unmarshal(timelineRaw, &item.Timeline)
	}
	item.FinalResult = map[string]any{}
	if len(finalResultRaw) > 0 {
		_ = json.Unmarshal(finalResultRaw, &item.FinalResult)
	}
	return item, nil
}

func (s *Store) GetBattleRunCarryStateByID(ctx context.Context, runID int64) (map[string]any, error) {
	if runID <= 0 {
		return nil, ErrBattleRoomConfigInvalid
	}
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT carry_state_jsonb
		FROM battle_run_carry_states
		WHERE run_id = $1
	`, runID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBattleRoomNotFound
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out, nil
}
