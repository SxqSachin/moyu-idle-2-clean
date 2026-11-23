package httpapi

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/store"
	"sort"
	"strconv"
	"sync"
)

var battlePassiveRNGPool = sync.Pool{New: func() any { return rand.New(rand.NewSource(1)) }}

func battlePassiveSeed(state battleRuntimeState, owner, kitty int64, source, stage string) int64 {
	var buffer [512]byte
	key := append(buffer[:0], "passive-battle-v1:"...)
	for _, n := range []int64{state.PassiveRunID, state.Seed, int64(state.Wave), owner, kitty} {
		key = strconv.AppendInt(key, n, 10)
		key = append(key, ':')
	}
	key = append(key, source...)
	key = append(key, ':')
	key = append(key, stage...)
	sum := sha256.Sum256(key)
	return int64(binary.LittleEndian.Uint64(sum[:8]))
}

func battlePassiveRandom(state battleRuntimeState, owner, kitty int64, source, stage string) *rand.Rand {
	return rand.New(rand.NewSource(battlePassiveSeed(state, owner, kitty, source, stage)))
}

// This adapter runs after every original reward roll and value normalization.
// It never advances the combat RNG or feeds extras back into the base bucket.
func (s *Server) appendBattlePassiveRewards(state battleRuntimeState, world battleWorldConfig, defeated []battleUnitState, players map[int64]battleUnitState, rewards map[int64]store.BattleWaveReward, summaries map[int64]map[string]any) error {
	if len(state.PassiveSources) == 0 || state.IsFogMode || battleCombatTrialActive(&state) {
		return nil
	}
	enemies := s.battleEnemyCatalog()
	rng := battlePassiveRNGPool.Get().(*rand.Rand)
	defer battlePassiveRNGPool.Put(rng)
	difficulty := 0.0
	for _, d := range s.deps.Config.SharedGlobalSettings.CombatDifficulty.Levels {
		if d.Level == state.DifficultyLevel {
			difficulty = d.BattleDropRateBonus
			break
		}
	}
	validated := map[int64]bool{}
	seen := map[int64]bool{}
	for _, cat := range state.Players {
		if cat.UnitType != "kitty" || cat.KittyID <= 0 || !cat.RewardEligible || cat.SummonSourceUnitID != "" || seen[cat.KittyID] {
			continue
		}
		seen[cat.KittyID] = true
		owner, ok := players[cat.UserID]
		if !ok || !owner.RewardEligible {
			continue
		}
		snapshot, ok := state.PassiveSources[cat.UserID]
		if !ok {
			continue
		}
		if !validated[cat.UserID] {
			if err := snapshot.Validate(); err != nil {
				return err
			}
			validated[cat.UserID] = true
		}
		rate, source := snapshot.Best(operationeffect.KindExtraLoot)
		if rate == 0 {
			continue
		}
		rng.Seed(battlePassiveSeed(state, cat.UserID, cat.KittyID, source, "participation"))
		triggered := rng.Int63n(10000) < rate
		extra := map[string]int64{}
		if triggered {
			for _, enemy := range defeated {
				cfg, ok := enemies[enemy.EnemyID]
				if !ok {
					continue
				}
				ids := cfg.DropIDs
				if len(ids) != len(cfg.Drops) {
					ids = make([]string, 0, len(cfg.Drops))
					for id := range cfg.Drops {
						ids = append(ids, id)
					}
					sort.Strings(ids)
				}
				for _, id := range ids {
					rule := cfg.Drops[id]
					rng.Seed(battlePassiveSeed(state, cat.UserID, cat.KittyID, source, "enemy:"+enemy.UnitID+":item:"+id))
					chance := battleSettlementDropPercent(rule.Percent, owner.Stats["battle_drop_rate_bonus"], difficulty, owner.Stats["luck"], s.deps.Config.GlobalSettings.Battle.DropLuckFactor)
					if rng.Float64() >= chance {
						continue
					}
					qty := rule.Min
					if rule.Max > rule.Min {
						qty += rng.Int63n(rule.Max - rule.Min + 1)
					}
					if qty > 0 {
						if qty > 1e12-extra[id] {
							return fmt.Errorf("passive loot quantity overflow")
						}
						extra[id] += qty
					}
				}
			}
		}
		reward := rewards[cat.UserID]
		contribution := operationeffect.Contribution{Items: extra, Opportunities: 1}
		if triggered {
			contribution.Triggered = 1
			if len(extra) == 0 {
				contribution.Empty = 1
			}
		}
		var err error
		reward.Contributions, err = operationeffect.MergeContributions(reward.Contributions, operationeffect.Contributions{source: contribution})
		if err != nil {
			return err
		}
		for id, n := range extra {
			if reward.ItemRewards[id] > math.MaxInt64-n {
				return fmt.Errorf("passive reward total overflow")
			}
			reward.ItemRewards[id] += n
		}
		rewards[cat.UserID] = reward
		summary := summaries[cat.UserID]
		rows, _ := summary["kittyExtraLoot"].([]map[string]any)
		summary["kittyExtraLoot"] = append(rows, map[string]any{"kittyId": cat.KittyID, "name": cat.Name, "source": source, "triggered": triggered, "items": extra})
	}
	return nil
}
