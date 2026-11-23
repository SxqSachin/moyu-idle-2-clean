package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const fishingTripRuntimeInvalidStopReason = "runtime_invalid"

type fishingTripStopState struct {
	ID          string
	Phase       string
	LoadoutRaw  []byte
	SuppliesRaw []byte
	SummaryRaw  []byte
}

// The queue association, not an untrusted or stale payload tripId, owns cleanup.
// Callers hold the user lock and delete the queue only after this succeeds.
func (s *Store) stopFishingTripForQueueTx(ctx context.Context, tx pgx.Tx, userID, queueID int64, reason string, at time.Time) (string, error) {
	var trip fishingTripStopState
	err := tx.QueryRow(ctx, `
		SELECT id, phase, loadout_snapshot_jsonb, supplies_jsonb, summary_jsonb
		FROM fishing_trips
		WHERE user_id = $1 AND queue_id = $2 AND status = 'running'
		FOR UPDATE
	`, userID, queueID).Scan(&trip.ID, &trip.Phase, &trip.LoadoutRaw, &trip.SuppliesRaw, &trip.SummaryRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if err := s.finishStoppedFishingTripTx(ctx, tx, userID, trip, reason, at); err != nil {
		return "", err
	}
	return trip.ID, s.pruneFishingTripEventsTx(ctx, tx, userID)
}

func (s *Store) finishStoppedFishingTripTx(ctx context.Context, tx pgx.Tx, userID int64, trip fishingTripStopState, reason string, at time.Time) error {
	var loadout FishingLoadoutSnapshot
	var supplies FishingTripSupplies
	var summary FishingTripSummary
	if err := json.Unmarshal(trip.LoadoutRaw, &loadout); err != nil {
		return err
	}
	if err := json.Unmarshal(trip.SuppliesRaw, &supplies); err != nil {
		return err
	}
	if len(trip.SummaryRaw) > 0 {
		if err := json.Unmarshal(trip.SummaryRaw, &summary); err != nil {
			return err
		}
	}
	if !supplies.Returned && (trip.Phase == fishingPhaseBite || trip.Phase == fishingPhaseFighting) {
		consumeFishingTerminalSupplies(&supplies, loadout, false)
	}
	if err := newFishingExecutor(s).returnFishingSuppliesTx(ctx, tx, userID, loadout, &supplies); err != nil {
		return err
	}
	summary.StopReason = reason
	var err error
	summary, err = s.appendFishingTripFinishedEventTx(ctx, tx, trip.ID, reason, at, summary)
	if err != nil {
		return err
	}
	summaryRaw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	suppliesRaw, err := json.Marshal(supplies)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE fishing_trips
		SET status = 'stopped', phase = 'finished', current_fish_jsonb = '{}'::jsonb,
		    summary_jsonb = $3::jsonb, supplies_jsonb = $4::jsonb,
		    stop_reason = $5, ended_at = $6, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'running'
	`, trip.ID, userID, string(summaryRaw), string(suppliesRaw), reason, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("fishing stop checkpoint rejected user_id=%d trip_id=%s", userID, trip.ID)
	}
	return nil
}
