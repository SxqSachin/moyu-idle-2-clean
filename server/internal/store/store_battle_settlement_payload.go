package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const getBattleRunSettlementByIDSQL = `
	SELECT id,
	       room_id,
	       wave,
	       status,
	       playback_state_jsonb,
	       result_summary_jsonb,
	       started_at,
	       ended_at,
	       updated_at
	FROM battle_runs
	WHERE id = $1
`

// GetBattleRunSettlementByID reads only the hot run state and the compact
// result summary. Callers must explicitly fall back to GetBattleRunByID when
// the summary cannot prove that it contains a complete settlement payload.
func (s *Store) GetBattleRunSettlementByID(ctx context.Context, runID int64) (BattleRun, error) {
	if runID <= 0 {
		return BattleRun{}, ErrBattleRoomConfigInvalid
	}
	var (
		out        BattleRun
		rawState   []byte
		rawSummary []byte
		endedAt    *time.Time
	)
	err := s.db.QueryRow(ctx, getBattleRunSettlementByIDSQL, runID).Scan(
		&out.ID,
		&out.RoomID,
		&out.Wave,
		&out.Status,
		&rawState,
		&rawSummary,
		&out.StartedAt,
		&endedAt,
		&out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleRun{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattleRun{}, err
	}
	out.State = map[string]any{}
	if len(rawState) > 0 {
		if err := json.Unmarshal(rawState, &out.State); err != nil {
			return BattleRun{}, fmt.Errorf("decode battle run settlement state: %w", err)
		}
	}
	out.FinalResult = map[string]any{}
	if len(rawSummary) > 0 {
		// A malformed or non-object summary is not authoritative. Keep it empty so
		// the settlement owner can fall back to the complete cold run payload.
		_ = json.Unmarshal(rawSummary, &out.FinalResult)
	}
	out.EndedAt = endedAt
	return out, nil
}
