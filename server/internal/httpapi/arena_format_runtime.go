package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
)

const arenaFormatConsequenceLimit = 128

type arenaFormatBarrierRuntime struct {
	UnitID           string
	BuffID           string
	TriggerID        string
	TotalCharges     int
	RemainingCharges int
	ImmuneTurnSerial int64
}

type arenaPendingFormatTrigger struct {
	TriggerID      string
	SourceUnitID   string
	SummonedUnitID string
	AttachedBuffID string
	BuffPayload    map[string]any
}

func arenaFormatCurrentTurnSerial(state *battleRuntimeState) int64 {
	if state == nil {
		return 0
	}
	if state.activeBuffDurationTurnSerial > 0 {
		return state.activeBuffDurationTurnSerial
	}
	return -(state.nextBuffDurationTurnSerial + 1)
}

func decodeArenaFormatSnapshot(raw json.RawMessage) (*arenaFormatDefinition, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return nil, fmt.Errorf("arena format snapshot is missing")
	}
	var definition arenaFormatDefinition
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, fmt.Errorf("decode arena format snapshot: %w", err)
	}
	normalized, err := normalizeArenaFormatDefinition(definition, false)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func arenaFormatTargetUnitIDs(state *battleRuntimeState, includeSummons, aliveOnly bool) []string {
	if state == nil {
		return nil
	}
	out := make([]string, 0, len(state.Players)+len(state.Enemies))
	appendUnits := func(units []battleUnitState) {
		for index := range units {
			unit := &units[index]
			if strings.TrimSpace(unit.UnitID) == "" || (!includeSummons && battleUnitIsSummon(unit)) || (aliveOnly && (!unit.Alive || unit.HP <= 0)) {
				continue
			}
			out = append(out, unit.UnitID)
		}
	}
	appendUnits(state.Players)
	appendUnits(state.Enemies)
	return out
}

func arenaFormatFormalPlayerBattleLevel(units []battleUnitState) (int64, bool) {
	for index := range units {
		unit := &units[index]
		if battleUnitIsSummon(unit) || !strings.EqualFold(strings.TrimSpace(unit.UnitType), "player") {
			continue
		}
		return maxInt64(1, unit.BattleLevel), true
	}
	return 0, false
}

func arenaFormatPlayerBattleLevels(state *battleRuntimeState) (int64, int64, bool) {
	if state == nil {
		return 0, 0, false
	}
	playerLevel, playerOK := arenaFormatFormalPlayerBattleLevel(state.Players)
	enemyLevel, enemyOK := arenaFormatFormalPlayerBattleLevel(state.Enemies)
	return playerLevel, enemyLevel, playerOK && enemyOK
}

func arenaFormatWarmupEligible(rule *arenaFormatWarmup, ownLevel, opponentLevel int64) bool {
	if rule == nil {
		return false
	}
	return ownLevel > rule.AlwaysAboveLevel || ownLevel-opponentLevel > rule.HigherSideGapExceeds
}

func arenaFormatMatchupHPBonus(rule *arenaFormatMatchupHPAdjustment, ownLevel, opponentLevel int64) float64 {
	if rule == nil {
		return 0
	}
	bonus := 0.0
	if ownLevel >= rule.BothAtLeastLevel && opponentLevel >= rule.BothAtLeastLevel {
		bonus += rule.BothSidesFlatBonus
	}
	if ownLevel < opponentLevel {
		bonus += float64(opponentLevel-ownLevel) * rule.LowerSideBonusPerLevel
	}
	return roundTo4(maxFloat(0, bonus))
}

func arenaFormatUnitsBySide(state *battleRuntimeState, side string) []battleUnitState {
	if state == nil {
		return nil
	}
	if normalizeBattleSide(side) == "enemy" {
		return state.Enemies
	}
	return state.Players
}

func arenaFormatWarmupPayload(rule *arenaFormatWarmup, ownLevel, opponentLevel int64) map[string]any {
	if rule == nil {
		return nil
	}
	return map[string]any{
		"ownBattleLevel":          ownLevel,
		"opponentBattleLevel":     opponentLevel,
		"normalActions":           rule.NormalActions,
		"startMultiplier":         rule.StartMultiplier,
		"recoveryPerNormalAction": rule.RecoveryPerNormalAction,
		"maximumMultiplier":       rule.MaximumMultiplier,
		"outputs":                 append([]string(nil), rule.Outputs...),
	}
}

func (s *Server) battleApplyArenaWarmupBuff(state *battleRuntimeState, unit *battleUnitState, effect arenaFormatEffect, ownLevel, opponentLevel int64) bool {
	if s == nil || state == nil || state.ArenaFormat == nil || unit == nil || effect.Warmup == nil ||
		!arenaFormatWarmupEligible(effect.Warmup, ownLevel, opponentLevel) {
		return false
	}
	battleApplyOrRefreshBuff(
		unit,
		effect.BuffID,
		-1,
		arenaFormatWarmupPayload(effect.Warmup, ownLevel, opponentLevel),
		"arena_format:"+state.ArenaFormat.ID,
		"refresh_duration",
	)
	if buff := findBuffByID(unit, effect.BuffID); buff != nil {
		buff.Triggered = 0
	}
	return true
}

func (s *Server) battleApplyArenaFormatBattleStart(state *battleRuntimeState) []map[string]any {
	if s == nil || state == nil || state.ArenaFormat == nil {
		return nil
	}
	playerLevel, enemyLevel, levelsOK := arenaFormatPlayerBattleLevels(state)
	actions := make([]map[string]any, 0, 2*(len(state.Players)+len(state.Enemies)))
	for _, trigger := range state.ArenaFormat.Triggers {
		if trigger.Event != "battle_start" {
			continue
		}
		for _, effect := range trigger.Effects {
			switch effect.Kind {
			case "matchup_hp_buff":
				if !levelsOK {
					continue
				}
				for _, side := range []string{"player", "enemy"} {
					ownLevel, opponentLevel := playerLevel, enemyLevel
					if side == "enemy" {
						ownLevel, opponentLevel = enemyLevel, playerLevel
					}
					bonus := arenaFormatMatchupHPBonus(effect.MatchupHP, ownLevel, opponentLevel)
					if bonus <= 0 {
						continue
					}
					units := arenaFormatUnitsBySide(state, side)
					for index := range units {
						unitID := units[index].UnitID
						unit := battleUnitByID(state, unitID)
						if unit == nil || battleUnitIsSummon(unit) {
							continue
						}
						unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
						if unitType != "player" && unitType != "kitty" {
							continue
						}
						payload := map[string]any{
							"ownBattleLevel": ownLevel, "opponentBattleLevel": opponentLevel,
							effect.PayloadKey: bonus,
						}
						battleApplyOrRefreshBuff(unit, effect.BuffID, -1, payload, "arena_format:"+state.ArenaFormat.ID, "refresh_duration")
						s.battleRecomputeUnitStats(unit)
						actions = append(actions, map[string]any{
							"kind": "buff", "targetId": unitID, "sourceId": "arena_format:" + state.ArenaFormat.ID,
							"buffId": effect.BuffID, "buffDurationTurns": int64(-1), "buffPayload": payload,
							"reason": "arena_format_matchup_hp_buff", "delayMs": int64(0),
							"meta": arenaFormatSourceMeta(state.ArenaFormat, trigger.ID),
						})
					}
				}
			case "warmup_buff":
				if !levelsOK {
					continue
				}
				for _, side := range []string{"player", "enemy"} {
					ownLevel, opponentLevel := playerLevel, enemyLevel
					if side == "enemy" {
						ownLevel, opponentLevel = enemyLevel, playerLevel
					}
					if !arenaFormatWarmupEligible(effect.Warmup, ownLevel, opponentLevel) {
						continue
					}
					units := arenaFormatUnitsBySide(state, side)
					for index := range units {
						unit := battleUnitByID(state, units[index].UnitID)
						if unit == nil || (!effect.IncludeSummons && battleUnitIsSummon(unit)) {
							continue
						}
						if !s.battleApplyArenaWarmupBuff(state, unit, effect, ownLevel, opponentLevel) {
							continue
						}
						payload := cloneAnyMap(findBuffByID(unit, effect.BuffID).Payload)
						actions = append(actions, map[string]any{
							"kind": "buff", "targetId": unit.UnitID, "sourceId": "arena_format:" + state.ArenaFormat.ID,
							"buffId": effect.BuffID, "buffDurationTurns": int64(-1), "buffPayload": payload,
							"buffTriggered": 0, "reason": "arena_format_warmup_buff", "delayMs": int64(0),
							"meta": arenaFormatSourceMeta(state.ArenaFormat, trigger.ID),
						})
					}
				}
			case "turn_barrier":
				for _, unitID := range arenaFormatTargetUnitIDs(state, effect.IncludeSummons, false) {
					unit := battleUnitByID(state, unitID)
					if unit == nil {
						continue
					}
					payload := map[string]any{
						"arenaFormatId":    state.ArenaFormat.ID,
						"chargesRemaining": effect.Charges,
						"totalCharges":     effect.Charges,
					}
					battleApplyOrRefreshBuff(unit, effect.BuffID, -1, payload, "arena_format:"+state.ArenaFormat.ID, "refresh_duration")
					key := unitID + "\x00" + effect.BuffID
					if state.arenaFormatBarriers == nil {
						state.arenaFormatBarriers = make(map[string]*arenaFormatBarrierRuntime)
					}
					if _, exists := state.arenaFormatBarriers[key]; !exists {
						state.arenaFormatBarrierOrder = append(state.arenaFormatBarrierOrder, key)
					}
					state.arenaFormatBarriers[key] = &arenaFormatBarrierRuntime{
						UnitID: unitID, BuffID: effect.BuffID, TriggerID: trigger.ID,
						TotalCharges: effect.Charges, RemainingCharges: effect.Charges,
					}
					actions = append(actions, map[string]any{
						"kind": "buff", "targetId": unitID, "sourceId": "arena_format:" + state.ArenaFormat.ID,
						"buffId": effect.BuffID, "buffDurationTurns": int64(-1), "buffPayload": payload,
						"buffTriggered": 0, "reason": "arena_format_battle_start", "delayMs": int64(0),
						"meta": arenaFormatSourceMeta(state.ArenaFormat, trigger.ID),
					})
				}
			default:
				continue
			}
		}
	}
	return actions
}

func (s *Server) battleApplyArenaFormatDamageGuard(state *battleRuntimeState, target *battleUnitState, amount float64) (bool, []map[string]any) {
	if s == nil || state == nil || state.ArenaFormat == nil || target == nil || amount <= 0 || len(state.arenaFormatBarrierOrder) == 0 {
		return false, nil
	}
	turnSerial := arenaFormatCurrentTurnSerial(state)
	for _, key := range state.arenaFormatBarrierOrder {
		barrier := state.arenaFormatBarriers[key]
		if barrier == nil || barrier.UnitID != target.UnitID {
			continue
		}
		if barrier.ImmuneTurnSerial == turnSerial {
			return true, nil
		}
		if barrier.RemainingCharges <= 0 {
			continue
		}
		barrier.RemainingCharges--
		barrier.ImmuneTurnSerial = turnSerial
		consumed := barrier.TotalCharges - barrier.RemainingCharges
		payload := map[string]any{
			"arenaFormatId":    state.ArenaFormat.ID,
			"chargesRemaining": barrier.RemainingCharges,
			"totalCharges":     barrier.TotalCharges,
		}
		if buff := findBuffByID(target, barrier.BuffID); buff != nil {
			buff.Payload = cloneAnyMap(payload)
			buff.Triggered = int64(consumed)
		}
		return true, []map[string]any{{
			"kind": "buff_charge", "targetId": target.UnitID, "sourceId": "arena_format:" + state.ArenaFormat.ID,
			"buffId": barrier.BuffID, "buffDurationTurns": int64(-1), "buffPayload": payload,
			"buffTriggered": consumed, "reason": "arena_format_turn_barrier", "delayMs": int64(0),
			"meta": arenaFormatSourceMeta(state.ArenaFormat, barrier.TriggerID),
		}}
	}
	return false, nil
}

func (s *Server) battleFinalizeArenaFormatTurn(state *battleRuntimeState) []map[string]any {
	if s == nil || state == nil || state.ArenaFormat == nil || len(state.arenaFormatBarrierOrder) == 0 {
		return nil
	}
	turnSerial := arenaFormatCurrentTurnSerial(state)
	actions := make([]map[string]any, 0, 2)
	remainingOrder := state.arenaFormatBarrierOrder[:0]
	for _, key := range state.arenaFormatBarrierOrder {
		barrier := state.arenaFormatBarriers[key]
		if barrier == nil {
			continue
		}
		if barrier.RemainingCharges > 0 || barrier.ImmuneTurnSerial != turnSerial {
			remainingOrder = append(remainingOrder, key)
			continue
		}
		unit := battleUnitByID(state, barrier.UnitID)
		if unit != nil {
			battleRemoveBuff(unit, barrier.BuffID)
		}
		delete(state.arenaFormatBarriers, key)
		actions = append(actions, map[string]any{
			"kind": "state_detach", "targetId": barrier.UnitID, "sourceId": "arena_format:" + state.ArenaFormat.ID,
			"buffId": barrier.BuffID, "reason": "arena_format_barrier_exhausted", "delayMs": int64(0),
			"meta": arenaFormatSourceMeta(state.ArenaFormat, barrier.TriggerID),
		})
	}
	state.arenaFormatBarrierOrder = remainingOrder
	return actions
}

func (s *Server) battleQueueArenaFormatAfterSummon(state *battleRuntimeState, sourceUnit, summonedUnit *battleUnitState) {
	if s == nil || state == nil || state.ArenaFormat == nil || sourceUnit == nil || summonedUnit == nil {
		return
	}
	playerLevel, enemyLevel, levelsOK := arenaFormatPlayerBattleLevels(state)
	if levelsOK {
		ownLevel, opponentLevel := playerLevel, enemyLevel
		if battleUnitPerspectiveSide(summonedUnit) == "enemy" {
			ownLevel, opponentLevel = enemyLevel, playerLevel
		}
		for _, trigger := range state.ArenaFormat.Triggers {
			if trigger.Event != "battle_start" {
				continue
			}
			for _, effect := range trigger.Effects {
				if effect.Kind == "warmup_buff" && effect.IncludeSummons {
					if s.battleApplyArenaWarmupBuff(state, summonedUnit, effect, ownLevel, opponentLevel) {
						state.pendingArenaFormatTriggers = append(state.pendingArenaFormatTriggers, arenaPendingFormatTrigger{
							TriggerID: trigger.ID, SourceUnitID: sourceUnit.UnitID, SummonedUnitID: summonedUnit.UnitID,
							AttachedBuffID: effect.BuffID, BuffPayload: cloneAnyMap(findBuffByID(summonedUnit, effect.BuffID).Payload),
						})
					}
				}
			}
		}
	}
	for _, trigger := range state.ArenaFormat.Triggers {
		if trigger.Event != "after_summon" {
			continue
		}
		state.pendingArenaFormatTriggers = append(state.pendingArenaFormatTriggers, arenaPendingFormatTrigger{
			TriggerID: trigger.ID, SourceUnitID: sourceUnit.UnitID, SummonedUnitID: summonedUnit.UnitID,
		})
	}
}

func arenaFormatDamageAmount(effect arenaFormatEffect, target *battleUnitState) float64 {
	if target == nil {
		return 0
	}
	amount := effect.Amount.Flat
	if effect.Amount.Basis == "target_max_hp" {
		amount += target.MaxHP * effect.Amount.Multiplier
	}
	amount = maxFloat(effect.Amount.Minimum, amount)
	if effect.Amount.Maximum > 0 && amount > effect.Amount.Maximum {
		amount = effect.Amount.Maximum
	}
	return roundTo4(maxFloat(0, amount))
}

func (s *Server) battleDrainArenaFormatActions(state *battleRuntimeState, now time.Time) []map[string]any {
	if s == nil || state == nil || state.ArenaFormat == nil || len(state.pendingArenaFormatTriggers) == 0 {
		return nil
	}
	actions := make([]map[string]any, 0, 8)
	processed := 0
	for len(state.pendingArenaFormatTriggers) > 0 && processed < arenaFormatConsequenceLimit {
		pending := state.pendingArenaFormatTriggers[0]
		state.pendingArenaFormatTriggers = state.pendingArenaFormatTriggers[1:]
		processed++
		if pending.AttachedBuffID != "" {
			meta := arenaFormatSourceMeta(state.ArenaFormat, pending.TriggerID)
			meta["summonedUnitId"] = pending.SummonedUnitID
			actions = append(actions, map[string]any{
				"kind": "buff", "targetId": pending.SummonedUnitID, "sourceId": "arena_format:" + state.ArenaFormat.ID,
				"buffId": pending.AttachedBuffID, "buffDurationTurns": int64(-1), "buffPayload": cloneAnyMap(pending.BuffPayload),
				"buffTriggered": 0, "reason": "arena_format_summon_warmup", "delayMs": int64(0), "meta": meta,
			})
			continue
		}
		var trigger *arenaFormatTrigger
		for index := range state.ArenaFormat.Triggers {
			if state.ArenaFormat.Triggers[index].ID == pending.TriggerID && state.ArenaFormat.Triggers[index].Event == "after_summon" {
				trigger = &state.ArenaFormat.Triggers[index]
				break
			}
		}
		if trigger == nil {
			continue
		}
		sourceUnit := battleUnitByID(state, pending.SourceUnitID)
		for _, effect := range trigger.Effects {
			if effect.Kind != "damage" {
				continue
			}
			targetIDs := arenaFormatTargetUnitIDs(state, effect.IncludeSummons, true)
			amounts := make(map[string]float64, len(targetIDs))
			for _, targetID := range targetIDs {
				amounts[targetID] = arenaFormatDamageAmount(effect, battleUnitByID(state, targetID))
			}
			runtime := newBattleSkillActionRuntime(state, sourceUnit, now, s, targetIDs)
			for _, targetID := range targetIDs {
				amount := amounts[targetID]
				if amount <= 0 {
					continue
				}
				meta := arenaFormatSourceMeta(state.ArenaFormat, trigger.ID)
				meta["summonedUnitId"] = pending.SummonedUnitID
				meta["skipFinalDamageModifiers"] = effect.SkipFinalDamageModifiers
				meta["suppressSourceHooks"] = !effect.DispatchHooks
				meta["suppressBuffHooks"] = !effect.DispatchHooks
				meta["suppressDamageRedirect"] = !effect.DispatchHooks
				runtime.ApplyDamage(battleskills.DamageAction{
					SourceID: pending.SourceUnitID, TargetID: targetID, Amount: amount,
					DamageType: effect.DamageType, SuppressEquipmentHooks: !effect.DispatchHooks,
					Reason: "arena_format_" + state.ArenaFormat.ID, Meta: meta,
				})
			}
			actions = append(actions, runtime.ActionLog()...)
		}
	}
	if len(state.pendingArenaFormatTriggers) > 0 {
		state.pendingArenaFormatTriggers = nil
	}
	return actions
}

func (s *Server) battleDrainArenaFormatConsequences(state *battleRuntimeState, now time.Time) []map[string]any {
	if s == nil || state == nil {
		return nil
	}
	actions := make([]map[string]any, 0, 8)
	for iteration := 0; iteration < arenaFormatConsequenceLimit; iteration++ {
		formatActions := s.battleDrainArenaFormatActions(state, now)
		deathActions := s.battleFlushPendingUnitDeathEquipmentHooks(state, now)
		if len(formatActions) == 0 && len(deathActions) == 0 {
			break
		}
		actions = append(actions, formatActions...)
		actions = append(actions, deathActions...)
	}
	return actions
}
