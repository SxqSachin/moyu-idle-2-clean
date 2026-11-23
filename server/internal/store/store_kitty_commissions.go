package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type KittyCommissionStartInput struct {
	UserID          int64
	KittyID         int64
	CommissionID    string
	MinKittyLevel   int
	DurationSeconds int64
	Costs           map[string]int64
	DailyLimit      int
	DayStart        time.Time
}

type KittyCommissionClaimInput struct {
	UserID        int64
	RunID         int64
	Rewards       []KittyRewardGrantSpec
	RewardPayload map[string]any
	KittyExp      int64
	BondExp       int64
	MoodDelta     float64
}

func (s *Store) CountKittyCommissionsSince(ctx context.Context, userID int64, kittyID int64, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM player_kitty_commissions
		WHERE user_id = $1 AND kitty_id = $2 AND started_at >= $3 AND status <> 'cancelled'
	`, userID, kittyID, since).Scan(&count)
	return count, err
}

func (s *Store) ListPlayerKittyCommissionRuns(ctx context.Context, userID int64, kittyID int64, limit int) ([]PlayerKittyCommissionRun, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, kitty_id, commission_id, status, costs_jsonb, reward_jsonb, started_at, ready_at, claimed_at
		FROM player_kitty_commissions
		WHERE user_id = $1 AND kitty_id = $2
		ORDER BY started_at DESC, id DESC
		LIMIT $3
	`, userID, kittyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlayerKittyCommissionRun, 0, limit)
	for rows.Next() {
		var row PlayerKittyCommissionRun
		var costsRaw, rewardRaw []byte
		if err := rows.Scan(&row.ID, &row.UserID, &row.KittyID, &row.CommissionID, &row.Status, &costsRaw, &rewardRaw, &row.StartedAt, &row.ReadyAt, &row.ClaimedAt); err != nil {
			return nil, err
		}
		row.Costs = decodeJSONMap(costsRaw)
		row.Reward = decodeJSONMap(rewardRaw)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) StartKittyCommission(ctx context.Context, input KittyCommissionStartInput) (PlayerKittyCommissionRun, error) {
	if input.UserID <= 0 || input.KittyID <= 0 || strings.TrimSpace(input.CommissionID) == "" || input.DurationSeconds <= 0 {
		return PlayerKittyCommissionRun{}, ErrKittyCommissionNotFound
	}
	if input.DailyLimit <= 0 {
		input.DailyLimit = 1
	}
	var out PlayerKittyCommissionRun
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, input.UserID); err != nil {
			return err
		}
		var level int
		var kittyStatus string
		if err := tx.QueryRow(ctx, `
			SELECT level, status
			FROM player_kitties
			WHERE user_id = $1 AND id = $2
			FOR UPDATE
		`, input.UserID, input.KittyID).Scan(&level, &kittyStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrKittyNotFound
			}
			return err
		}
		if normalizeKittyStatus(kittyStatus) == KittyStatusInCombat {
			return ErrKittyModeConflictLifeQueue
		}
		if input.MinKittyLevel > 0 && level < input.MinKittyLevel {
			return ErrKittyLevelTooLow
		}
		var count int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM player_kitty_commissions
			WHERE user_id = $1 AND kitty_id = $2 AND started_at >= $3 AND status <> 'cancelled'
		`, input.UserID, input.KittyID, input.DayStart).Scan(&count); err != nil {
			return err
		}
		if count >= int64(input.DailyLimit) {
			return ErrKittyCommissionLimitReached
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_kitties
			SET status = $3,
			    last_commission_at = now(),
			    updated_at = now()
			WHERE user_id = $1 AND id = $2
		`, input.UserID, input.KittyID, KittyStatusInLifeQueue); err != nil {
			return err
		}
		for itemID, qty := range input.Costs {
			itemID = strings.TrimSpace(itemID)
			if itemID == "" || qty <= 0 {
				return ErrBadQuantity
			}
			if err := s.LedgerConsumeTemplateTx(ctx, tx, input.UserID, itemID, qty, InventoryLedgerMeta{
				SourceSurface: "inventory",
				TargetSurface: "kitty_commission",
				Details: map[string]any{
					"type":         "kitty_commission_cost",
					"kittyId":      input.KittyID,
					"commissionId": strings.TrimSpace(input.CommissionID),
				},
			}); err != nil {
				return err
			}
		}
		costRaw, err := json.Marshal(input.Costs)
		if err != nil {
			return err
		}
		var insertedCostsRaw, insertedRewardRaw []byte
		return tx.QueryRow(ctx, `
			INSERT INTO player_kitty_commissions(user_id, kitty_id, commission_id, status, costs_jsonb, reward_jsonb, started_at, ready_at)
			VALUES($1, $2, $3, 'running', $4::jsonb, '{}'::jsonb, now(), now() + ($5::bigint * interval '1 second'))
			RETURNING id, user_id, kitty_id, commission_id, status, costs_jsonb, reward_jsonb, started_at, ready_at, claimed_at
		`, input.UserID, input.KittyID, strings.TrimSpace(input.CommissionID), string(costRaw), input.DurationSeconds).Scan(&out.ID, &out.UserID, &out.KittyID, &out.CommissionID, &out.Status, &insertedCostsRaw, &insertedRewardRaw, &out.StartedAt, &out.ReadyAt, &out.ClaimedAt)
	})
	if err != nil {
		return PlayerKittyCommissionRun{}, err
	}
	return s.GetKittyCommissionRun(ctx, input.UserID, out.ID)
}

func (s *Store) GetKittyCommissionRun(ctx context.Context, userID int64, runID int64) (PlayerKittyCommissionRun, error) {
	var row PlayerKittyCommissionRun
	var costsRaw, rewardRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, kitty_id, commission_id, status, costs_jsonb, reward_jsonb, started_at, ready_at, claimed_at
		FROM player_kitty_commissions
		WHERE user_id = $1 AND id = $2
	`, userID, runID).Scan(&row.ID, &row.UserID, &row.KittyID, &row.CommissionID, &row.Status, &costsRaw, &rewardRaw, &row.StartedAt, &row.ReadyAt, &row.ClaimedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerKittyCommissionRun{}, ErrKittyCommissionNotFound
	}
	if err != nil {
		return PlayerKittyCommissionRun{}, err
	}
	row.Costs = decodeJSONMap(costsRaw)
	row.Reward = decodeJSONMap(rewardRaw)
	return row, nil
}

func (s *Store) ClaimKittyCommission(ctx context.Context, input KittyCommissionClaimInput) (PlayerKittyCommissionRun, error) {
	var out PlayerKittyCommissionRun
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, input.UserID); err != nil {
			return err
		}
		var costsRaw, rewardRaw []byte
		if err := tx.QueryRow(ctx, `
			SELECT id, user_id, kitty_id, commission_id, status, costs_jsonb, reward_jsonb, started_at, ready_at, claimed_at
			FROM player_kitty_commissions
			WHERE user_id = $1 AND id = $2
			FOR UPDATE
		`, input.UserID, input.RunID).Scan(&out.ID, &out.UserID, &out.KittyID, &out.CommissionID, &out.Status, &costsRaw, &rewardRaw, &out.StartedAt, &out.ReadyAt, &out.ClaimedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrKittyCommissionNotFound
			}
			return err
		}
		if out.Status != "running" {
			return ErrKittyCommissionNotRunning
		}
		if time.Now().Before(out.ReadyAt) {
			return ErrKittyCommissionNotReady
		}
		for _, reward := range input.Rewards {
			itemID := strings.TrimSpace(reward.ItemID)
			if itemID == "" || reward.Quantity <= 0 {
				continue
			}
			if err := s.LedgerGrantTemplateTx(ctx, tx, input.UserID, itemID, reward.Quantity, InventoryLedgerMeta{
				SourceSurface: "kitty_commission",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":         "kitty_commission_reward",
					"kittyId":      out.KittyID,
					"commissionId": out.CommissionID,
					"runId":        out.ID,
				},
			}); err != nil {
				return err
			}
		}
		if err := s.applyKittyGrowthTx(ctx, tx, input.UserID, out.KittyID, input.KittyExp, input.BondExp, input.MoodDelta); err != nil {
			return err
		}
		rewardRaw, err := encodeJSONMap(input.RewardPayload)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			UPDATE player_kitty_commissions
			SET status = 'claimed', reward_jsonb = $3::jsonb, claimed_at = now()
			WHERE user_id = $1 AND id = $2
			RETURNING id, user_id, kitty_id, commission_id, status, costs_jsonb, reward_jsonb, started_at, ready_at, claimed_at
		`, input.UserID, input.RunID, string(rewardRaw)).Scan(&out.ID, &out.UserID, &out.KittyID, &out.CommissionID, &out.Status, &costsRaw, &rewardRaw, &out.StartedAt, &out.ReadyAt, &out.ClaimedAt); err != nil {
			return err
		}
		var runningCount int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM player_kitty_commissions
			WHERE user_id = $1
			  AND kitty_id = $2
			  AND status = 'running'
		`, input.UserID, out.KittyID).Scan(&runningCount); err != nil {
			return err
		}
		if runningCount <= 0 {
			if err := s.setOwnedKittyStatusTx(ctx, tx, input.UserID, out.KittyID, KittyStatusIdle); err != nil {
				return err
			}
		}
		out.Costs = decodeJSONMap(costsRaw)
		out.Reward = decodeJSONMap(rewardRaw)
		return nil
	})
	if err != nil {
		return PlayerKittyCommissionRun{}, err
	}
	return out, nil
}
