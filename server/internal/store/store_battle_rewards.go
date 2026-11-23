package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
)

type BattleWaveReward struct {
	Contributions operationeffect.Contributions
	ItemRewards   map[string]int64
	AttrExp       map[string]float64
	KittyAttrExp  map[int64]map[string]float64
}

func (s *Store) ApplyBattleWaveRewards(ctx context.Context, rewards map[int64]BattleWaveReward) error {
	if len(rewards) == 0 {
		return nil
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		userIDs := make([]int64, 0, len(rewards))
		for userID := range rewards {
			userIDs = append(userIDs, userID)
		}
		sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
		for _, userID := range userIDs {
			reward := rewards[userID]
			if userID <= 0 {
				continue
			}
			if err := s.lockUserTx(ctx, tx, userID); err != nil {
				return err
			}
			for itemID, qty := range reward.ItemRewards {
				if qty <= 0 {
					continue
				}
				if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
					SourceSurface: "battle_reward",
					TargetSurface: "inventory",
					Details: map[string]any{
						"type": "wave_reward",
					},
				}); err != nil {
					return err
				}
			}
			if err := s.applyAttrExpTx(ctx, tx, userID, reward.AttrExp); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ApplyBattleWaveAttrRewards(ctx context.Context, rewards map[int64]BattleWaveReward) error {
	if len(rewards) == 0 {
		return nil
	}
	seeds := s.attributeSeedsFromGameData()
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.applyBattleWaveAttrRewardsTx(ctx, tx, rewards, seeds)
	})
}

func (s *Store) applyBattleWaveAttrRewardsTx(ctx context.Context, tx pgx.Tx, rewards map[int64]BattleWaveReward, seeds []PlayerAttributeSeed) error {
	return s.applyBattleWaveAttrRewardsWithMetricsTx(ctx, tx, rewards, seeds, nil)
}

func (s *Store) applyBattleWaveAttrRewardsWithMetricsTx(
	ctx context.Context,
	tx pgx.Tx,
	rewards map[int64]BattleWaveReward,
	seeds []PlayerAttributeSeed,
	metrics *BattleAttrExpJobApplyMetrics,
) error {
	userIDs := make([]int64, 0, len(rewards))
	for userID := range rewards {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		reward := rewards[userID]
		filtered := map[string]float64{}
		for key, value := range reward.AttrExp {
			if strings.TrimSpace(key) == "" || value == 0 {
				continue
			}
			filtered[key] = value
		}
		filteredKitty := make(map[int64]map[string]float64, len(reward.KittyAttrExp))
		kittyIDs := make([]int64, 0, len(reward.KittyAttrExp))
		for kittyID, kittyAttrs := range reward.KittyAttrExp {
			if kittyID <= 0 {
				continue
			}
			attrs := make(map[string]float64, len(kittyAttrs))
			for key, value := range kittyAttrs {
				if strings.TrimSpace(key) == "" || value == 0 {
					continue
				}
				attrs[key] += value
			}
			if len(attrs) == 0 {
				continue
			}
			filteredKitty[kittyID] = attrs
			kittyIDs = append(kittyIDs, kittyID)
		}
		if len(filtered) == 0 && len(kittyIDs) == 0 {
			continue
		}
		sort.Slice(kittyIDs, func(i, j int) bool { return kittyIDs[i] < kittyIDs[j] })
		if metrics != nil {
			metrics.Users++
			metrics.PlayerAttrKeys += len(filtered)
			metrics.Kitties += len(kittyIDs)
			for _, kittyID := range kittyIDs {
				metrics.KittyAttrKeys += len(filteredKitty[kittyID])
			}
		}
		stageStartedAt := time.Now()
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			if metrics != nil {
				metrics.UserLockElapsed += time.Since(stageStartedAt)
			}
			return err
		}
		if metrics != nil {
			metrics.UserLockElapsed += time.Since(stageStartedAt)
		}
		if len(filtered) > 0 {
			stageStartedAt = time.Now()
			result, err := s.applyAttrExpLockedResultTx(ctx, tx, userID, filtered)
			if metrics != nil {
				metrics.PlayerAttrElapsed += time.Since(stageStartedAt)
				if result.Changed {
					metrics.PlayerProgressChanges++
				}
				if result.LevelChanged {
					metrics.PlayerLevelChanges++
				}
			}
			if err != nil {
				return err
			}
		}
		for _, kittyID := range kittyIDs {
			stageStartedAt = time.Now()
			result, err := s.applyKittyAttrExpLockedResultTx(ctx, tx, userID, kittyID, filteredKitty[kittyID], seeds)
			if metrics != nil {
				metrics.KittyAttrElapsed += time.Since(stageStartedAt)
				if result.Changed {
					metrics.KittyProgressChanges++
				}
				if result.LevelChanged {
					metrics.KittyLevelChanges++
				}
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
