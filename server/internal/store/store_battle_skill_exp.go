package store

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"
)

const BattleSkillExpPerUse int64 = 2

func (s *Store) applyBattleSkillExpRewardsTx(
	ctx context.Context,
	tx pgx.Tx,
	rewards map[int64]map[string]int64,
	kittyRewards map[int64]map[string]int64,
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
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		skillRewards := rewards[userID]
		skillIDs := make([]string, 0, len(skillRewards))
		for skillID := range skillRewards {
			skillIDs = append(skillIDs, skillID)
		}
		sort.Strings(skillIDs)
		for _, skillID := range skillIDs {
			gained := skillRewards[skillID]
			if gained <= 0 {
				continue
			}
			if _, _, err := s.addSkillExpTx(ctx, tx, userID, skillID, gained); err != nil {
				return err
			}
		}
	}
	kittyIDs := make([]int64, 0, len(kittyRewards))
	for kittyID := range kittyRewards {
		if kittyID > 0 {
			kittyIDs = append(kittyIDs, kittyID)
		}
	}
	sort.Slice(kittyIDs, func(i, j int) bool { return kittyIDs[i] < kittyIDs[j] })
	for _, kittyID := range kittyIDs {
		skillRewards := kittyRewards[kittyID]
		skillIDs := make([]string, 0, len(skillRewards))
		for skillID := range skillRewards {
			skillIDs = append(skillIDs, skillID)
		}
		sort.Strings(skillIDs)
		for _, skillID := range skillIDs {
			gained := skillRewards[skillID]
			if gained <= 0 {
				continue
			}
			if _, _, _, err := s.addKittySkillExpTx(ctx, tx, kittyID, skillID, gained); err != nil {
				return err
			}
		}
	}
	return nil
}
