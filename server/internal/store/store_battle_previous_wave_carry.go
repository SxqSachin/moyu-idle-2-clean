package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const getBattlePreviousWaveCarryModernQuery = `
	SELECT previous.id,
	       previous.room_id,
	       previous.wave,
	       previous.status,
	       carry.run_id IS NOT NULL,
	       COALESCE(carry.carry_state_jsonb, 'null'::jsonb)
	FROM battle_rooms AS room
	JOIN battle_runs AS current
	  ON current.id = room.current_run_id
	 AND current.room_id = room.id
	JOIN LATERAL (
		SELECT candidate.id,
		       candidate.room_id,
		       candidate.wave,
		       candidate.status
		FROM battle_runs AS candidate
		WHERE candidate.room_id = current.room_id
		  AND candidate.id < current.id
		  AND candidate.wave = $3
		ORDER BY candidate.id DESC
		LIMIT 1
	) AS previous ON TRUE
	LEFT JOIN battle_run_carry_states AS carry ON carry.run_id = previous.id
	WHERE room.id = $1
	  AND current.id = $2
	  AND current.wave = $3 + 1
`

const getBattlePreviousWaveCarryLegacyQuery = `
	SELECT payload.run_id IS NOT NULL,
	       COALESCE(payload.final_result_raw, ''::bytea),
	       CASE
	         WHEN payload.run_id IS NOT NULL
	          AND COALESCE(octet_length(payload.final_result_raw), 0) = 0
	         THEN COALESCE(payload.final_result_jsonb -> 'carryState', 'null'::jsonb)
	         ELSE 'null'::jsonb
	       END,
	       CASE
	         WHEN payload.run_id IS NULL
	         THEN COALESCE(previous.final_result_jsonb -> 'carryState', 'null'::jsonb)
	         ELSE 'null'::jsonb
	       END
	FROM battle_runs AS previous
	LEFT JOIN battle_run_payloads AS payload ON payload.run_id = previous.id
	WHERE previous.id = $1
	  AND previous.room_id = $2
	  AND previous.wave = $3
`

type battlePreviousWaveCarrySources struct {
	payloadExists  bool
	coldFinalRaw   []byte
	coldJSONBCarry []byte
	inlineCarry    []byte
}

// GetBattlePreviousWaveCarry loads only the previous wave's carry payload. The
// current run identity and expected wave are part of the query so an unrelated
// historical run cannot seed a retried or replaced run.
func (s *Store) GetBattlePreviousWaveCarry(ctx context.Context, roomID, currentRunID int64, expectedWave int) (BattlePreviousWaveCarry, error) {
	if roomID <= 0 || currentRunID <= 0 || expectedWave <= 0 {
		return BattlePreviousWaveCarry{}, ErrBattleRoomConfigInvalid
	}

	var out BattlePreviousWaveCarry
	var dedicatedExists bool
	var dedicatedRaw []byte
	err := s.db.QueryRow(ctx, getBattlePreviousWaveCarryModernQuery, roomID, currentRunID, expectedWave).Scan(
		&out.RunID,
		&out.RoomID,
		&out.Wave,
		&out.Status,
		&dedicatedExists,
		&dedicatedRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattlePreviousWaveCarry{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattlePreviousWaveCarry{}, err
	}
	if raw, ok := resolveBattlePreviousWaveDedicatedCarryRaw(dedicatedExists, dedicatedRaw); ok {
		out.CarryStateRaw = raw
		return out, nil
	}

	var sources battlePreviousWaveCarrySources
	err = s.db.QueryRow(ctx, getBattlePreviousWaveCarryLegacyQuery, out.RunID, roomID, expectedWave).Scan(
		&sources.payloadExists,
		&sources.coldFinalRaw,
		&sources.coldJSONBCarry,
		&sources.inlineCarry,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattlePreviousWaveCarry{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattlePreviousWaveCarry{}, err
	}
	out.CarryStateRaw, err = resolveBattlePreviousWaveCarryLegacyRaw(sources)
	if err != nil {
		return BattlePreviousWaveCarry{}, err
	}
	return out, nil
}

func resolveBattlePreviousWaveDedicatedCarryRaw(exists bool, raw []byte) (json.RawMessage, bool) {
	if !exists {
		return nil, false
	}
	return normalizeBattleCarryRaw(raw), true
}

func resolveBattlePreviousWaveCarryLegacyRaw(sources battlePreviousWaveCarrySources) (json.RawMessage, error) {
	if sources.payloadExists {
		if len(sources.coldFinalRaw) > 0 {
			var envelope struct {
				CarryState json.RawMessage `json:"carryState"`
			}
			if err := json.Unmarshal(sources.coldFinalRaw, &envelope); err != nil {
				return nil, fmt.Errorf("decode battle run cold carry envelope: %w", err)
			}
			return normalizeBattleCarryRaw(envelope.CarryState), nil
		}
		return normalizeBattleCarryRaw(sources.coldJSONBCarry), nil
	}
	return normalizeBattleCarryRaw(sources.inlineCarry), nil
}

// pgx Scan into []byte already returns an owned buffer, so normalization must
// not duplicate the carry payload on this hot path.
func normalizeBattleCarryRaw(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(raw)
}
