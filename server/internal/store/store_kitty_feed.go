package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type KittyFeedResult struct {
	KittyID            int64 `json:"kittyId"`
	BattleLevelCapGain int64 `json:"battleLevelCapGain"`
	BattleLevelCap     int64 `json:"battleLevelCap"`
}

func (s *Store) FeedKittyBattleLevelCap(ctx context.Context, userID, kittyID, instanceID, battleLevelCapGain int64) (KittyFeedResult, error) {
	if kittyID <= 0 || instanceID <= 0 || battleLevelCapGain <= 0 {
		return KittyFeedResult{}, ErrKittyFeedItemInvalid
	}
	var nextCap int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, true); err != nil {
			return err
		}
		instance, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if instance.UserID != userID {
			return ErrInstanceNotOwned
		}
		if instance.State != "inventory" {
			return ErrInvalidInstanceState
		}
		if err := s.DecrementOrDeleteTx(ctx, tx, instanceID, userID, 1); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			UPDATE player_kitties
			SET battle_level_cap = GREATEST(1, battle_level_cap + $3),
			    updated_at = now()
			WHERE user_id = $1
			  AND id = $2
			RETURNING battle_level_cap
		`, userID, kittyID, battleLevelCapGain).Scan(&nextCap); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return KittyFeedResult{}, err
	}
	return KittyFeedResult{
		KittyID:            kittyID,
		BattleLevelCapGain: battleLevelCapGain,
		BattleLevelCap:     nextCap,
	}, nil
}
