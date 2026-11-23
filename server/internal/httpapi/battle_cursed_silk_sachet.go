package httpapi

import (
	"fmt"
	"math"
	"strings"
)

const cursedSilkSachetItemID = "cursed_silk_sachet"

const battleExtraEnemyPoolCurrentWaveBoss = "currentwaveboss"

func (s *Server) appendBattleEquipmentExtraBossRefreshes(
	state *battleRuntimeState,
	enemyCatalog map[string]battleEnemyConfig,
	defaultBasicSkillID string,
) {
	if s == nil || state == nil {
		return
	}
	extraCount := s.battleEquipmentExtraBossRefreshCount(state)
	if extraCount <= 0 {
		return
	}
	triggerPrefix := fmt.Sprintf("enemy:%d:t", state.Wave)
	extraPrefix := fmt.Sprintf("enemy:%d:extra-boss", state.Wave)
	candidates := make([]int, 0, len(state.Enemies))
	existingExtras := 0
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if strings.HasPrefix(strings.TrimSpace(enemy.UnitID), extraPrefix) {
			existingExtras++
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(enemy.EnemyType), "boss") ||
			strings.HasPrefix(strings.TrimSpace(enemy.UnitID), triggerPrefix) {
			continue
		}
		candidates = append(candidates, index)
	}
	remaining := extraCount - existingExtras
	if len(candidates) == 0 || remaining <= 0 {
		return
	}

	for index := 0; index < remaining; index++ {
		source := &state.Enemies[candidates[state.randInt(len(candidates))]]
		cfg, ok := enemyCatalog[source.EnemyID]
		if !ok {
			continue
		}
		extra := s.buildBattleEnemyUnit(
			cfg,
			fmt.Sprintf("%s%d:%s", extraPrefix, existingExtras+index+1, cfg.ID),
			false,
			defaultBasicSkillID,
		)
		extra.BaseStats = cloneFloatMap(source.BaseStats)
		extra.Stats = cloneFloatMap(source.Stats)
		extra.StatModifiers = cloneFloatMap(source.StatModifiers)
		extra.StatSources = cloneNestedFloatMap(source.StatSources)
		extra.Buffs = cloneBattleBuffStates(source.Buffs)
		extra.StatMode = source.StatMode
		extra.MaxHP = source.MaxHP
		extra.MaxMP = source.MaxMP
		extra.HP = source.MaxHP
		extra.MP = source.MaxMP
		state.Enemies = append(state.Enemies, extra)
	}
}

func (s *Server) battleEquipmentExtraBossRefreshCount(state *battleRuntimeState) int {
	if s == nil || state == nil {
		return 0
	}
	best := 0
	for playerIndex := range state.Players {
		player := &state.Players[playerIndex]
		if !player.Alive || player.HP <= 0 || battleUnitIsSummon(player) {
			continue
		}
		for _, hook := range player.EquipmentHooks {
			if battleNormalizeTrigger(hook.Trigger) != "onenemyrefresh" {
				continue
			}
			conditionsPassed := true
			for _, condition := range hook.Conditions {
				if !s.battleEvaluateEquipmentHookCondition(state, condition, player, nil, nil, float64(state.Wave), "", hook.SourceEnhanceLevel, nil) {
					conditionsPassed = false
					break
				}
			}
			if !conditionsPassed {
				continue
			}
			for _, effect := range hook.Effects {
				if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) != battleEffectExtraEnemyRefresh ||
					battleNormalizeTrigger(fmt.Sprintf("%v", effect["enemyPool"])) != battleExtraEnemyPoolCurrentWaveBoss ||
					battleNormalizeTrigger(fmt.Sprintf("%v", effect["partyStackMode"])) != "highest" ||
					battleNormalizeTrigger(fmt.Sprintf("%v", effect["settlementMode"])) != "normal" {
					continue
				}
				formula := parseBattleSkillAmountFormula(effect["countFormula"], nil)
				count := int(math.Floor(battleEvaluateAmountFormulaWithEnhance(player, 1, formula, hook.SourceEnhanceLevel)))
				maxCount := minInt(8, maxInt(0, int(toInt64Loose(effect["maxCount"], 0))))
				if maxCount <= 0 {
					continue
				}
				count = minInt(maxCount, maxInt(0, count))
				best = maxInt(best, count)
			}
		}
	}
	return best
}
