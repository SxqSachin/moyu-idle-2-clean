package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	battlecore "mysecondwebgame/server/internal/battle/core"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

const (
	combatTrialDummyUnitType    = "trial_dummy"
	combatTrialProgressSchemaV1 = 1
)

func battleCombatTrialActive(state *battleRuntimeState) bool {
	return state != nil && state.CombatTrialFormat != nil &&
		strings.EqualFold(strings.TrimSpace(state.BattleKind), battleKindCombatTrial)
}

func (s *Server) battleDefenseKForState(state *battleRuntimeState) float64 {
	if battleCombatTrialActive(state) {
		if state.CombatTrialFormat.DefenseK > 0 {
			return state.CombatTrialFormat.DefenseK
		}
		if state.CombatTrialFormat.Dummy.Defense > 0 {
			return state.CombatTrialFormat.Dummy.Defense
		}
	}
	defenseK := s.deps.Config.SharedGlobalSettings.CombatBalanceModel.DefenseK
	if defenseK <= 0 {
		return 100
	}
	return defenseK
}

func combatTrialEncounterUnitForTarget(format *combatTrialFormatDefinition, target *battleUnitState) *combatTrialEncounterUnitDefinition {
	if format == nil || target == nil {
		return nil
	}
	encounter := format.Encounter
	if len(encounter.Units) == 0 && format.Dummy.Count > 0 {
		encounter = legacyCombatTrialEncounter(format.Dummy)
	}
	targetID := strings.TrimSpace(target.UnitID)
	for _, unit := range combatTrialEncounterUnitPointers(&encounter) {
		if unit != nil && combatTrialEncounterRuntimeUnitID(*unit) == targetID {
			return unit
		}
	}
	if strings.HasPrefix(strings.TrimSpace(target.UnitID), "combat_trial:dummy:") {
		index, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(target.UnitID), "combat_trial:dummy:"))
		if err == nil && index > 0 && index <= len(encounter.Units) {
			return &encounter.Units[index-1]
		}
	}
	return nil
}

func battleCombatTrialEncounterTarget(state *battleRuntimeState, target *battleUnitState) *combatTrialEncounterUnitDefinition {
	if !battleCombatTrialActive(state) || target == nil {
		return nil
	}
	return combatTrialEncounterUnitForTarget(state.CombatTrialFormat, target)
}

func battleCombatTrialTrainingTarget(state *battleRuntimeState, target *battleUnitState) bool {
	unit := battleCombatTrialEncounterTarget(state, target)
	return unit != nil && unit.Kind == "training_target"
}

func battleCombatTrialImmutableTarget(state *battleRuntimeState, target *battleUnitState) bool {
	unit := battleCombatTrialEncounterTarget(state, target)
	return unit != nil && unit.HealthMode == "immutable_full"
}

func battleCombatTrialPassiveTarget(state *battleRuntimeState, target *battleUnitState) bool {
	unit := battleCombatTrialEncounterTarget(state, target)
	return unit != nil && unit.Behavior == "passive"
}

func battleRestoreCombatTrialImmutableTarget(state *battleRuntimeState, target *battleUnitState) {
	if !battleCombatTrialImmutableTarget(state, target) {
		return
	}
	if unit := combatTrialEncounterUnitForTarget(state.CombatTrialFormat, target); unit != nil {
		target.MaxHP = unit.MaxHP
		target.Position = unit.Position
		target.BasePosition = unit.Position
		side := combatTrialEncounterUnitSide(*unit)
		target.Side = side
		target.OriginSide = side
		target.BaseSide = side
		target.PerspectiveSide = side
	} else if state.CombatTrialFormat != nil {
		target.MaxHP = state.CombatTrialFormat.Dummy.MaxHP
		target.Side = "enemy"
		target.OriginSide = "enemy"
		target.BaseSide = "enemy"
		target.PerspectiveSide = "enemy"
	}
	if target.Stats == nil {
		target.Stats = map[string]float64{}
	}
	target.Stats["hp"] = target.MaxHP
	target.HP = target.MaxHP
	target.Alive = true
}

func battleRestoreCombatTrialImmutableTargets(state *battleRuntimeState) {
	if !battleCombatTrialActive(state) {
		return
	}
	for index := range state.Players {
		battleRestoreCombatTrialImmutableTarget(state, &state.Players[index])
	}
	for index := range state.Enemies {
		battleRestoreCombatTrialImmutableTarget(state, &state.Enemies[index])
	}
}

func battleCombatTrialRegisterPlayerSource(state *battleRuntimeState, source *battleUnitState) {
	if !battleCombatTrialActive(state) || source == nil || battleUnitTeamSide(source) != "player" {
		return
	}
	sourceID := strings.TrimSpace(source.UnitID)
	if sourceID == "" {
		return
	}
	if _, excluded := state.CombatTrialSupportUnitIDs[sourceID]; excluded {
		return
	}
	if state.CombatTrialPlayerSourceIDs == nil {
		state.CombatTrialPlayerSourceIDs = map[string]struct{}{}
	}
	state.CombatTrialPlayerSourceIDs[sourceID] = struct{}{}
}

func battleCombatTrialPlayerSource(state *battleRuntimeState, sourceID string, source *battleUnitState) bool {
	if !battleCombatTrialActive(state) {
		return false
	}
	battleCombatTrialRegisterPlayerSource(state, source)
	_, ok := state.CombatTrialPlayerSourceIDs[strings.TrimSpace(sourceID)]
	return ok
}

func battleCombatTrialBuffMetricSource(state *battleRuntimeState, buff *battleBuffState, fallback *battleUnitState) (string, *battleUnitState) {
	if !battleCombatTrialActive(state) {
		return "", nil
	}
	sourceID := ""
	if buff != nil {
		sourceID = strings.TrimSpace(buff.SourceID)
	}
	if sourceID != "" {
		return sourceID, battleUnitByID(state, sourceID)
	}
	if fallback == nil {
		return "", nil
	}
	return strings.TrimSpace(fallback.UnitID), fallback
}

func battleCombatTrialSourceMatches(state *battleRuntimeState, scope, sourceID string, source *battleUnitState) bool {
	switch strings.TrimSpace(scope) {
	case "player_side":
		return battleCombatTrialPlayerSource(state, sourceID, source)
	case "enemy_side":
		return source != nil && battleUnitTeamSide(source) == "enemy"
	case "any":
		return strings.TrimSpace(sourceID) != ""
	default:
		return false
	}
}

func battleCombatTrialTargetMatches(state *battleRuntimeState, scope string, target *battleUnitState) bool {
	if !battleCombatTrialActive(state) || target == nil {
		return false
	}
	switch strings.TrimSpace(scope) {
	case "objective_units":
		_, ok := state.CombatTrialObjectiveTargetIDs[strings.TrimSpace(target.UnitID)]
		return ok
	case "trial_dummies":
		return battleCombatTrialTrainingTarget(state, target)
	case "player_side":
		return battleUnitTeamSide(target) == "player"
	case "enemy_side":
		return battleUnitTeamSide(target) == "enemy"
	case "any":
		return true
	default:
		return false
	}
}

func battleCombatTrialRecordMetricEvent(state *battleRuntimeState, eventType, sourceID string, source, target *battleUnitState, amount float64) {
	if !battleCombatTrialActive(state) {
		return
	}
	amount = roundTo4(maxFloat(0, amount))
	if amount <= 0 {
		return
	}
	if state.CombatTrialMetrics == nil {
		state.CombatTrialMetrics = map[string]float64{}
	}
	for _, metric := range state.CombatTrialFormat.Metrics {
		if !metric.Enabled || metric.EventType != eventType ||
			!battleCombatTrialSourceMatches(state, metric.SourceScope, sourceID, source) ||
			!battleCombatTrialTargetMatches(state, metric.TargetScope, target) {
			continue
		}
		switch metric.Aggregation {
		case "sum":
			state.CombatTrialMetrics[metric.ID] = roundTo4(state.CombatTrialMetrics[metric.ID] + amount)
		case "max":
			state.CombatTrialMetrics[metric.ID] = roundTo4(maxFloat(state.CombatTrialMetrics[metric.ID], amount))
		}
	}
}

func battleCombatTrialRecordDamage(state *battleRuntimeState, sourceID string, source, target *battleUnitState, amount float64) {
	battleCombatTrialRecordMetricEvent(state, "damage", sourceID, source, target, amount)
}

func battleCombatTrialRecordHeal(state *battleRuntimeState, sourceID string, source, target *battleUnitState, amount float64) {
	battleCombatTrialRecordMetricEvent(state, "heal", sourceID, source, target, amount)
}

func battleCombatTrialRecordBattleEndMetrics(state *battleRuntimeState) {
	if !battleCombatTrialActive(state) {
		return
	}
	if state.CombatTrialMetrics == nil {
		state.CombatTrialMetrics = map[string]float64{}
	}
	for _, metric := range state.CombatTrialFormat.Metrics {
		if !metric.Enabled || metric.EventType != "battle_end" || metric.Aggregation != "final" ||
			!combatTrialMetricEndReasonEligible(metric, state.CombatTrialEndReason) {
			continue
		}
		value := 0.0
		switch metric.ValueSource {
		case "player_main_actions":
			value = float64(state.CombatTrialActionCount)
		case "waves_cleared":
			value = float64(state.CompletedBattles)
		case "objective_units_defeated":
			value = float64(len(state.CombatTrialDefeatedObjectiveIDs))
		case "objective_units_remaining_hp":
			for index := range state.Players {
				if battleCombatTrialTargetMatches(state, "objective_units", &state.Players[index]) {
					value += maxFloat(0, state.Players[index].HP)
				}
			}
			for index := range state.Enemies {
				if battleCombatTrialTargetMatches(state, "objective_units", &state.Enemies[index]) {
					value += maxFloat(0, state.Enemies[index].HP)
				}
			}
		default:
			continue
		}
		state.CombatTrialMetrics[metric.ID] = roundTo4(value)
	}
}

func combatTrialRankDirections(format *combatTrialFormatDefinition, endReason string) map[string]string {
	out := map[string]string{}
	if format == nil {
		return out
	}
	for _, metric := range format.Metrics {
		if combatTrialMetricEligible(metric, endReason) {
			out[metric.ID] = metric.RankDirection
		}
	}
	return out
}

func battleCombatTrialDamageAmplification(state *battleRuntimeState, sourceID string, source *battleUnitState, damageType string) float64 {
	if !battleCombatTrialActive(state) || !battleCombatTrialPlayerSource(state, sourceID, source) {
		return 0
	}
	damageType = normalizeBattleDamageType(damageType)
	multiplier := 1.0
	for _, modifier := range state.CombatTrialFormat.Modifiers {
		if modifier.Kind != "outgoing_damage_multiplier" || modifier.SourceScope != "player_side" || (modifier.DamageType != "all" && modifier.DamageType != damageType) {
			continue
		}
		if !modifier.IncludeSummons && (source == nil || battleUnitIsSummon(source)) {
			continue
		}
		multiplier *= modifier.Multiplier
	}
	for _, rule := range state.CombatTrialFormat.Triggers {
		if rule.Event != "damage_calculation" {
			continue
		}
		for _, effect := range rule.Effects {
			if effect.Kind != "outgoing_damage_multiplier" || effect.SourceScope != "player_side" || (effect.DamageType != "all" && effect.DamageType != damageType) {
				continue
			}
			if !effect.IncludeSummons && (source == nil || battleUnitIsSummon(source)) {
				continue
			}
			multiplier *= effect.Multiplier
		}
	}
	return maxFloat(-1, multiplier-1)
}

func battleRecordCombatTrialMainAction(state *battleRuntimeState, actor *battleUnitState, isPreparationTurn bool, payload map[string]any) {
	if !battleCombatTrialActive(state) || actor == nil || isPreparationTurn || battleUnitTeamSide(actor) != "player" {
		return
	}
	battleCombatTrialRegisterPlayerSource(state, actor)
	state.CombatTrialActionCount++
	actionLog := []map[string]any(nil)
	if payload != nil {
		actionLog = battleAnyMapSlice(payload["actionLog"])
	}
	// Queued V2 snapshots may still carry the original post-action deterioration rule.
	for _, rule := range state.CombatTrialFormat.Triggers {
		if rule.Event != "player_main_action_completed" {
			continue
		}
		for _, effect := range rule.Effects {
			if effect.Kind != "max_hp_percent_damage" || effect.TargetScope != "objective_units" {
				continue
			}
			allowedTargets := make(map[string]struct{}, len(effect.TargetUnitIDs))
			for _, targetID := range effect.TargetUnitIDs {
				allowedTargets[strings.TrimSpace(targetID)] = struct{}{}
			}
			apply := func(target *battleUnitState) {
				if target == nil || !target.Alive || target.HP <= 0 || !battleCombatTrialTargetMatches(state, "objective_units", target) {
					return
				}
				definition := battleCombatTrialEncounterTarget(state, target)
				if definition == nil {
					return
				}
				if len(allowedTargets) > 0 {
					if _, exists := allowedTargets[definition.ID]; !exists {
						return
					}
				}
				beforeHP := target.HP
				damage := maxFloat(0, target.MaxHP*effect.MaxHPRatio)
				minimumHP := 0.0
				if effect.NonLethal {
					minimumHP = 1
				}
				target.HP = roundTo4(maxFloat(minimumHP, target.HP-damage))
				if target.HP <= 0 {
					target.HP = 0
					target.Alive = false
				}
				applied := maxFloat(0, beforeHP-target.HP)
				if applied <= 0 {
					return
				}
				actionLog = append(actionLog, map[string]any{
					"kind":        "resource",
					"targetId":    target.UnitID,
					"resource":    "hp",
					"amount":      roundTo4(-applied),
					"reason":      "combat_trial_rule:" + rule.ID,
					"beforeHp":    roundTo4(beforeHP),
					"afterHp":     roundTo4(target.HP),
					"beforeAlive": true,
					"afterAlive":  target.Alive,
				})
			}
			for index := range state.Players {
				apply(&state.Players[index])
			}
			for index := range state.Enemies {
				apply(&state.Enemies[index])
			}
		}
	}
	if payload != nil {
		payload["actionLog"] = actionLog
		payload["combatTrialActionCount"] = state.CombatTrialActionCount
		if state.CombatTrialFormat != nil {
			payload["combatTrialActionBudget"] = combatTrialPlayerMainActionBudget(state.CombatTrialFormat)
		}
	}
}

func combatTrialTrainingTargetUnit(definition combatTrialEncounterUnitDefinition, index int) battleUnitState {
	position := normalizeCompetitivePosition(definition.Position)
	dummyHP := definition.MaxHP
	nameKey := strings.TrimSpace(definition.NameKey)
	if nameKey == "" {
		nameKey = fmt.Sprintf("game.combatTrial.dummies.%s", position)
	}
	stats := map[string]float64{
		"hp":                    dummyHP,
		"mp":                    0,
		"speed":                 1,
		"physics_defence":       definition.Defense,
		"magical_defence":       definition.Defense,
		"physics_damage_resist": definition.DamageResist,
		"magical_damage_resist": definition.DamageResist,
		"crit_damage_resist":    definition.CriticalDamageResist,
	}
	unit := battleUnitState{
		UnitID:          combatTrialEncounterRuntimeUnitID(definition),
		Side:            "enemy",
		OriginSide:      "enemy",
		BaseSide:        "enemy",
		PerspectiveSide: "enemy",
		UnitType:        combatTrialDummyUnitType,
		EnemyID:         "combat_trial_dummy",
		EnemyType:       "trial_dummy",
		NameKey:         nameKey,
		Name:            fmt.Sprintf("Training Dummy %d", index+1),
		Position:        position,
		BasePosition:    position,
		Alive:           true,
		RewardEligible:  false,
		HP:              dummyHP,
		MP:              0,
		MaxHP:           dummyHP,
		MaxMP:           0,
		BaseStats:       cloneFloatMap(stats),
		Stats:           stats,
		StatModifiers:   map[string]float64{},
		StatSources:     map[string]map[string]float64{},
		SkillSlots:      []string{},
		SkillIDs:        []string{},
		SkillRules:      map[string]battleSkillRuleState{},
		ItemRules:       map[string]battleSkillRuleState{},
		Skills:          []battleSkillState{},
		BattleItems:     []battleItemState{},
		EquipmentDigest: []string{},
		EquipmentIDs:    []int64{},
		Buffs:           []battleBuffState{},
	}
	normalizeBattleUnitState(&unit)
	return unit
}

func combatTrialScriptedAllyUnit(definition combatTrialEncounterUnitDefinition) battleUnitState {
	position := normalizeCompetitivePosition(definition.Position)
	maxHP := maxFloat(1, definition.MaxHP)
	initialHP := roundTo4(maxFloat(1, maxHP*definition.InitialHPRatio))
	stats := map[string]float64{
		"hp":                    maxHP,
		"mp":                    0,
		"speed":                 1,
		"physics_defence":       definition.Defense,
		"magical_defence":       definition.Defense,
		"physics_damage_resist": definition.DamageResist,
		"magical_damage_resist": definition.DamageResist,
		"crit_damage_resist":    definition.CriticalDamageResist,
	}
	unit := battleUnitState{
		UnitID:          combatTrialEncounterRuntimeUnitID(definition),
		Side:            "player",
		OriginSide:      "player",
		BaseSide:        "player",
		PerspectiveSide: "player",
		UnitType:        "trial_support",
		NameKey:         definition.NameKey,
		Name:            definition.ID,
		Position:        position,
		BasePosition:    position,
		Alive:           true,
		RewardEligible:  false,
		HP:              initialHP,
		MP:              0,
		MaxHP:           maxHP,
		MaxMP:           0,
		BaseStats:       cloneFloatMap(stats),
		Stats:           stats,
		StatModifiers:   map[string]float64{},
		StatSources:     map[string]map[string]float64{},
		SkillSlots:      []string{},
		SkillIDs:        []string{},
		SkillRules:      map[string]battleSkillRuleState{},
		ItemRules:       map[string]battleSkillRuleState{},
		Skills:          []battleSkillState{},
		BattleItems:     []battleItemState{},
		EquipmentDigest: []string{},
		EquipmentIDs:    []int64{},
		Buffs:           []battleBuffState{},
		SkipTurnOrder:   true,
	}
	normalizeBattleUnitState(&unit)
	unit.HP = initialHP
	unit.SkipTurnOrder = true
	return unit
}

func combatTrialRuntimeUnitByID(state *battleRuntimeState, unitID string) *battleUnitState {
	if state == nil || strings.TrimSpace(unitID) == "" {
		return nil
	}
	for index := range state.Players {
		if state.Players[index].UnitID == unitID {
			return &state.Players[index]
		}
	}
	for index := range state.Enemies {
		if state.Enemies[index].UnitID == unitID {
			return &state.Enemies[index]
		}
	}
	return nil
}

func combatTrialInitialBuffPayload(payload map[string]float64) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	resolved := make(map[string]any, len(payload))
	for key, value := range payload {
		resolved[key] = value
	}
	return resolved
}

func (s *Server) applyCombatTrialEncounterInitialBuffs(state *battleRuntimeState, waveIndex int) error {
	if !battleCombatTrialActive(state) {
		return nil
	}
	definitions, err := combatTrialEncounterRoster(state.CombatTrialFormat, waveIndex)
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		unit := combatTrialRuntimeUnitByID(state, combatTrialEncounterRuntimeUnitID(definition))
		if unit == nil {
			return fmt.Errorf("combat trial initial buff target is missing: %s", definition.ID)
		}
		for _, initialBuff := range definition.InitialBuffs {
			duration, exists := s.combatTrialInitialBuffDuration(initialBuff.BuffID)
			if !exists {
				return fmt.Errorf("combat trial initial buff is unavailable: %s", initialBuff.BuffID)
			}
			battleApplyOrRefreshBuff(
				unit,
				initialBuff.BuffID,
				duration,
				combatTrialInitialBuffPayload(initialBuff.Payload),
				unit.UnitID,
				s.battleBuffStackPolicyByID(initialBuff.BuffID),
				s.battleBuffMaxStacksByID(initialBuff.BuffID),
			)
		}
	}
	return nil
}

func (s *Server) battleTriggerCombatTrialSupportTurnStartHooks(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	isPreparationTurn bool,
) []map[string]any {
	if !battleCombatTrialActive(state) || actor == nil || isPreparationTurn || battleUnitTeamSide(actor) != "player" {
		return nil
	}
	if _, supportActor := state.CombatTrialSupportUnitIDs[actor.UnitID]; supportActor {
		return nil
	}
	turnKind := "ordered"
	if battleUnitHasActiveExtraTurn(state, actor) {
		turnKind = "extra"
	}
	context := &battleEquipmentHookContext{TurnKind: turnKind, IsPreparation: false}
	actions := make([]map[string]any, 0, len(state.CombatTrialSupportUnitIDs))
	for index := range state.Players {
		support := &state.Players[index]
		if _, exists := state.CombatTrialSupportUnitIDs[support.UnitID]; !exists || !support.Alive || support.HP <= 0 {
			continue
		}
		definition := battleCombatTrialEncounterTarget(state, support)
		if definition == nil || len(definition.InitialBuffs) == 0 {
			continue
		}
		initialBuffIDs := make([]string, 0, len(definition.InitialBuffs))
		for _, initialBuff := range definition.InitialBuffs {
			initialBuffIDs = append(initialBuffIDs, initialBuff.BuffID)
		}
		actions = append(actions, s.battleTriggerUnitBuffHooksWithContext(
			state,
			now,
			support,
			actor,
			"on_turn_start",
			context,
			initialBuffIDs...,
		)...)
	}
	return actions
}

func combatTrialEncounterRoster(format *combatTrialFormatDefinition, waveIndex int) ([]combatTrialEncounterUnitDefinition, error) {
	if format == nil {
		return nil, fmt.Errorf("combat trial format is missing")
	}
	encounter := format.Encounter
	if len(encounter.Units) == 0 && format.Dummy.Count > 0 {
		encounter = legacyCombatTrialEncounter(format.Dummy)
	}
	switch encounter.Kind {
	case "unit_roster":
		return encounter.Units, nil
	case "wave_roster":
		if waveIndex < 0 || waveIndex >= len(encounter.Waves) {
			return nil, fmt.Errorf("combat trial encounter wave is unavailable: %d", waveIndex)
		}
		return encounter.Waves[waveIndex].Units, nil
	default:
		return nil, fmt.Errorf("unsupported combat trial encounter: %s", encounter.Kind)
	}
}

func combatTrialEncounterParticipants(format *combatTrialFormatDefinition, waveIndex int) ([]battleUnitState, []battleUnitState, error) {
	definitions, err := combatTrialEncounterRoster(format, waveIndex)
	if err != nil {
		return nil, nil, err
	}
	players := make([]battleUnitState, 0, len(definitions))
	enemies := make([]battleUnitState, 0, len(definitions))
	for index, definition := range definitions {
		var unit battleUnitState
		switch definition.Kind {
		case "training_target":
			unit = combatTrialTrainingTargetUnit(definition, index)
		case "scripted_ally":
			unit = combatTrialScriptedAllyUnit(definition)
		case "authored_enemy":
			if definition.ResolvedUnit == nil {
				return nil, nil, fmt.Errorf("combat trial resolved enemy is missing: %s", definition.ID)
			}
			raw, err := json.Marshal(definition.ResolvedUnit)
			if err != nil {
				return nil, nil, err
			}
			if err := json.Unmarshal(raw, &unit); err != nil {
				return nil, nil, err
			}
			unit.UnitID = combatTrialEncounterRuntimeUnitID(definition)
			unit.Position = definition.Position
			unit.BasePosition = definition.Position
			unit.RewardEligible = false
			normalizeBattleUnitState(&unit)
			unit.HP = unit.MaxHP
			unit.MP = unit.MaxMP
			unit.Alive = true
		default:
			return nil, nil, fmt.Errorf("unsupported combat trial encounter unit: %s", definition.Kind)
		}
		if combatTrialEncounterUnitSide(definition) == "player" {
			players = append(players, unit)
		} else {
			enemies = append(enemies, unit)
		}
	}
	return players, enemies, nil
}

func combatTrialEncounterUnits(format *combatTrialFormatDefinition) ([]battleUnitState, error) {
	_, enemies, err := combatTrialEncounterParticipants(format, 0)
	return enemies, err
}

func combatTrialDummyUnits(format *combatTrialFormatDefinition) []battleUnitState {
	units, _ := combatTrialEncounterUnits(format)
	return units
}

func combatTrialMetricsSnapshot(state *battleRuntimeState) map[string]float64 {
	out := map[string]float64{}
	if !battleCombatTrialActive(state) {
		return out
	}
	for _, metric := range state.CombatTrialFormat.Metrics {
		out[metric.ID] = roundTo4(maxFloat(0, state.CombatTrialMetrics[metric.ID]))
	}
	return out
}

func combatTrialProgressPayload(state *battleRuntimeState) map[string]any {
	payload := map[string]any{
		"progressVersion": combatTrialProgressSchemaV1,
		"actionCount":     0,
		"actionBudget":    0,
		"metrics":         map[string]float64{},
	}
	if !battleCombatTrialActive(state) {
		return payload
	}
	payload["actionCount"] = state.CombatTrialActionCount
	payload["actionBudget"] = combatTrialPlayerMainActionBudget(state.CombatTrialFormat)
	payload["metrics"] = combatTrialMetricsSnapshot(state)
	payload["encounterKind"] = state.CombatTrialFormat.Encounter.Kind
	payload["currentWave"] = state.CombatTrialWaveIndex + 1
	payload["waveCount"] = maxInt(1, len(state.CombatTrialFormat.Encounter.Waves))
	payload["wavesCleared"] = state.CompletedBattles
	payload["objectiveUnitsDefeated"] = len(state.CombatTrialDefeatedObjectiveIDs)
	if reason := strings.TrimSpace(state.CombatTrialEndReason); reason != "" {
		payload["endReason"] = reason
	}
	return payload
}

func combatTrialCaptureObjectiveDefeats(state *battleRuntimeState) {
	if !battleCombatTrialActive(state) {
		return
	}
	if state.CombatTrialDefeatedObjectiveIDs == nil {
		state.CombatTrialDefeatedObjectiveIDs = map[string]struct{}{}
	}
	for index := range state.Enemies {
		unit := &state.Enemies[index]
		if unit.Alive || unit.HP > 0 || !battleCombatTrialTargetMatches(state, "objective_units", unit) {
			continue
		}
		state.CombatTrialDefeatedObjectiveIDs[unit.UnitID] = struct{}{}
	}
}

func combatTrialInvalidateUnitIndexes(state *battleRuntimeState) {
	if state == nil {
		return
	}
	state.unitIndexByID = nil
	state.unitIndexCount = 0
	state.otherUnitTurnEndHookUnitIDs = nil
	state.otherUnitTurnEndHookIndexBuilt = false
	state.otherUnitTurnEndHookUnitCount = 0
}

func combatTrialRestoreSupportTurnExclusions(state *battleRuntimeState) {
	if !battleCombatTrialActive(state) || len(state.CombatTrialSupportUnitIDs) == 0 {
		return
	}
	for index := range state.Players {
		if _, scriptedSupport := state.CombatTrialSupportUnitIDs[state.Players[index].UnitID]; !scriptedSupport {
			continue
		}
		state.Players[index].SkipTurnOrder = true
		state.Players[index].TurnIneligible = true
	}
	state.Order = nil
	state.FullOrder = nil
	state.reseedOrder()
}

func (s *Server) advanceCombatTrialWave(state *battleRuntimeState, now time.Time) (bool, error) {
	if !battleCombatTrialActive(state) || state.CombatTrialFormat.Encounter.Kind != "wave_roster" || battleTeamAlive(state, "enemy") || state.CombatTrialEncounterCompleted {
		return false, nil
	}
	state.CompletedBattles++
	state.emit("battle.wave.win", now, map[string]any{
		"wave":             state.Wave,
		"completedBattles": state.CompletedBattles,
		"combatTrial":      combatTrialProgressPayload(state),
	})
	if state.CombatTrialWaveIndex+1 >= len(state.CombatTrialFormat.Encounter.Waves) {
		state.CombatTrialEncounterCompleted = true
		return false, nil
	}
	if !battlePlayerMembersAlive(state) || state.CombatTrialActionCount >= combatTrialPlayerMainActionBudget(state.CombatTrialFormat) {
		return false, nil
	}
	nextWaveIndex := state.CombatTrialWaveIndex + 1
	scriptedPlayers, enemies, err := combatTrialEncounterParticipants(state.CombatTrialFormat, nextWaveIndex)
	if err != nil {
		return false, err
	}
	keptPlayers := state.Players[:0]
	for index := range state.Players {
		if _, scriptedSupport := state.CombatTrialSupportUnitIDs[state.Players[index].UnitID]; scriptedSupport {
			continue
		}
		keptPlayers = append(keptPlayers, state.Players[index])
	}
	state.Players = append(keptPlayers, scriptedPlayers...)
	state.Enemies = enemies
	state.CombatTrialWaveIndex = nextWaveIndex
	state.Wave = nextWaveIndex + 1
	state.Order = nil
	state.FullOrder = nil
	state.PreparationOrder = nil
	state.PreparationSeedOrder = nil
	state.ActedUnits = nil
	state.ExtraTurnQueue = nil
	state.TurnCursor = 0
	state.RuleTurnCounts = map[string]int{}
	state.pendingUnitDeaths = nil
	state.activeExtraTurnRequest = nil
	state.activeExtraTurnChildCount = 0
	combatTrialInvalidateUnitIndexes(state)
	if err := s.applyCombatTrialEncounterInitialBuffs(state, nextWaveIndex); err != nil {
		return false, err
	}
	ensureSliceSpareCap(&state.Players, 40)
	ensureSliceSpareCap(&state.Enemies, 40)
	waveStartActions := s.battleTriggerCompetitiveWaveStartHooks(state, now)
	waveStartActions = append(waveStartActions, s.battleFlushPendingUnitDeathEquipmentHooks(state, now)...)
	for index := range state.Players {
		s.battleRecomputeUnitStats(&state.Players[index])
	}
	for index := range state.Enemies {
		s.battleRecomputeUnitStats(&state.Enemies[index])
	}
	s.rebuildBattleOrder(state)
	combatTrialRestoreSupportTurnExclusions(state)
	state.emit("battle.wave.spawn", now, map[string]any{
		"wave":               state.Wave,
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   []string{},
		"players":            competitivePublicUnitSlice(state.Players),
		"enemies":            competitivePublicUnitSlice(state.Enemies),
		"battlefieldEffects": []map[string]any{},
		"battleStartActions": waveStartActions,
		"combatTrialFormat":  combatTrialPublicFormat(*state.CombatTrialFormat),
		"combatTrial":        combatTrialProgressPayload(state),
	})
	return true, nil
}

func (s *Server) initCombatTrialRuntimeState(run store.CombatTrialRun, now time.Time) (battleRuntimeState, error) {
	players, err := competitiveRuntimeLineup(run.Lineup, "player", "combat_trial:player")
	if err != nil {
		return battleRuntimeState{}, fmt.Errorf("decode combat trial lineup: %w", err)
	}
	format, err := decodeCombatTrialFormatSnapshot(run.FormatSnapshot)
	if err != nil {
		return battleRuntimeState{}, err
	}
	if format.DefinitionDigest != run.FormatDefinitionDigest || combatTrialPlayerMainActionBudget(format) != run.ActionBudget {
		return battleRuntimeState{}, fmt.Errorf("combat trial format snapshot mismatch")
	}
	scriptedPlayers, enemies, err := combatTrialEncounterParticipants(format, 0)
	if err != nil {
		return battleRuntimeState{}, err
	}
	players = append(players, scriptedPlayers...)
	metrics := make(map[string]float64, len(format.Metrics))
	for _, metric := range format.Metrics {
		metrics[metric.ID] = 0
	}
	state := battleRuntimeState{
		BattleKind:                      battleKindCombatTrial,
		Seed:                            run.Seed,
		StartedAt:                       now.UnixMilli(),
		LastActionAt:                    now.UnixMilli(),
		Wave:                            1,
		Players:                         players,
		Enemies:                         enemies,
		Order:                           []string{},
		FullOrder:                       []string{},
		PreparationOrder:                []string{},
		PreparationSeedOrder:            []string{},
		ActedUnits:                      []string{},
		Events:                          []battleEvent{},
		RetainCompleteEventHistory:      true,
		CombatTrialFormat:               format,
		CombatTrialMetrics:              metrics,
		CombatTrialPlayerSourceIDs:      map[string]struct{}{},
		CombatTrialObjectiveTargetIDs:   map[string]struct{}{},
		CombatTrialSupportUnitIDs:       map[string]struct{}{},
		CombatTrialDefeatedObjectiveIDs: map[string]struct{}{},
		CombatTrialWaveIndex:            0,
		CombatTrialActionCount:          0,
	}
	for _, definition := range combatTrialEncounterUnitDefinitions(format.Encounter) {
		unitID := combatTrialEncounterRuntimeUnitID(definition)
		if definition.ObjectiveTarget {
			state.CombatTrialObjectiveTargetIDs[unitID] = struct{}{}
		}
		if definition.Kind == "scripted_ally" {
			state.CombatTrialSupportUnitIDs[unitID] = struct{}{}
		}
	}
	if len(state.CombatTrialObjectiveTargetIDs) == 0 {
		for index := range state.Enemies {
			if battleCombatTrialTrainingTarget(&state, &state.Enemies[index]) {
				state.CombatTrialObjectiveTargetIDs[state.Enemies[index].UnitID] = struct{}{}
			}
		}
	}
	for index := range state.Players {
		battleCombatTrialRegisterPlayerSource(&state, &state.Players[index])
	}
	if err := s.applyCombatTrialEncounterInitialBuffs(&state, 0); err != nil {
		return battleRuntimeState{}, err
	}
	ensureSliceSpareCap(&state.Players, 40)
	ensureSliceSpareCap(&state.Enemies, 40)
	battleStartActions := s.battleTriggerBattleStartEquipmentHooks(&state, now)
	battleStartActions = append(battleStartActions, s.battleTriggerCompetitiveWaveStartHooks(&state, now)...)
	battleStartActions = append(battleStartActions, s.battleFlushPendingUnitDeathEquipmentHooks(&state, now)...)
	for index := range state.Players {
		s.battleRecomputeUnitStats(&state.Players[index])
	}
	for index := range state.Enemies {
		s.battleRecomputeUnitStats(&state.Enemies[index])
	}
	s.rebuildBattleOrder(&state)
	combatTrialRestoreSupportTurnExclusions(&state)
	state.emit("battle.wave.spawn", now, map[string]any{
		"wave":               state.Wave,
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   []string{},
		"players":            competitivePublicUnitSlice(state.Players),
		"enemies":            competitivePublicUnitSlice(state.Enemies),
		"battlefieldEffects": []map[string]any{},
		"battleStartActions": battleStartActions,
		"combatTrialFormat":  combatTrialPublicFormat(*format),
		"combatTrial":        combatTrialProgressPayload(&state),
	})
	return state, nil
}

func combatTrialPublicState(state battleRuntimeState) map[string]any {
	return map[string]any{
		"battleKind":         battleKindCombatTrial,
		"seed":               state.Seed,
		"wave":               state.Wave,
		"completedBattles":   state.CompletedBattles,
		"seq":                state.Seq,
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   []string{},
		"players":            competitivePublicUnitSlice(state.Players),
		"enemies":            competitivePublicUnitSlice(state.Enemies),
		"battlefieldEffects": []map[string]any{},
		"combatTrialFormat":  combatTrialPublicFormat(*state.CombatTrialFormat),
		"combatTrial":        combatTrialProgressPayload(&state),
		"events":             []battleEvent{},
	}
}

func (s *Server) finishCombatTrial(state *battleRuntimeState, now time.Time, reason string, triggerBattleEnd bool) {
	if !battleCombatTrialActive(state) || state.CombatTrialEndReason != "" {
		return
	}
	state.CombatTrialEndReason = strings.TrimSpace(reason)
	actions := []map[string]any{}
	if triggerBattleEnd {
		actions = append(actions, s.battleTriggerBattleEndEquipmentHooks(state, now)...)
		actions = append(actions, s.battleFlushPendingUnitDeathEquipmentHooks(state, now)...)
	}
	battleCombatTrialRecordBattleEndMetrics(state)
	battleRestoreCombatTrialImmutableTargets(state)
	state.emit("battle.ended", now, map[string]any{
		"reason":       state.CombatTrialEndReason,
		"actions":      actions,
		"actionCount":  state.CombatTrialActionCount,
		"actionBudget": combatTrialPlayerMainActionBudget(state.CombatTrialFormat),
		"metrics":      combatTrialMetricsSnapshot(state),
	})
}

func combatTrialReachedEndCondition(state *battleRuntimeState) (string, bool) {
	if !battleCombatTrialActive(state) {
		return "", false
	}
	for _, condition := range state.CombatTrialFormat.EndConditions {
		reached := false
		switch condition.Kind {
		case "player_team_defeated":
			reached = !battlePlayerMembersAlive(state)
		case "enemy_team_defeated":
			reached = !battleTeamAlive(state, "enemy")
		case "player_main_actions_reached":
			reached = state.CombatTrialActionCount >= combatTrialPlayerMainActionBudget(state.CombatTrialFormat)
		case "encounter_completed":
			reached = state.CombatTrialEncounterCompleted
		}
		if reached {
			return condition.Reason, true
		}
	}
	return "", false
}

func (s *Server) compileCombatTrialRun(ctx context.Context, run store.CombatTrialRun, now time.Time) (json.RawMessage, json.RawMessage, json.RawMessage, []wsbus.BattleTimelineEvent, map[string]float64, int, string, error) {
	state, err := s.initCombatTrialRuntimeState(run, now)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, "", err
	}
	initialState := combatTrialPublicState(state)
	turnInterval := time.Duration(s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs) * time.Millisecond
	if turnInterval <= 0 {
		turnInterval = time.Second
	}
	maxSteps := maxInt(200, run.ActionBudget*(len(state.Players)+len(state.Enemies)+4)*4)
	rewardPlan := map[int64]store.BattleWaveReward{}
	orchestrated, err := battlecore.RunCompileOrchestration(battlecore.CompileOrchestratorInput{
		Now:             now,
		TurnInterval:    turnInterval,
		MaxSteps:        maxSteps,
		MaxExecDuration: time.Duration(s.deps.Config.GlobalSettings.Battle.CompileMaxExecMs) * time.Millisecond,
		ProcessTurnStep: func(simNow time.Time) (string, bool, error) {
			status, advanced, processErr := s.processBattleTurn(ctx, &state, battleWorldConfig{}, nil, simNow, rewardPlan)
			if processErr != nil {
				return status, advanced, processErr
			}
			combatTrialCaptureObjectiveDefeats(&state)
			if state.CombatTrialFormat.Encounter.Kind == "wave_roster" && !battleTeamAlive(&state, "enemy") {
				waveAdvanced, waveErr := s.advanceCombatTrialWave(&state, simNow)
				if waveErr != nil {
					return status, advanced, waveErr
				}
				if waveAdvanced {
					return "", true, nil
				}
			}
			if reason, reached := combatTrialReachedEndCondition(&state); reached {
				s.finishCombatTrial(&state, simNow, reason, true)
				return store.BattleRunStatusEnded, true, nil
			}
			if status == store.BattleRunStatusFailed {
				return status, advanced, fmt.Errorf("combat trial failed without a matching authored end condition")
			}
			return status, advanced, nil
		},
	})
	if err != nil {
		return nil, nil, nil, nil, nil, 0, "", err
	}
	if orchestrated.TimedOut || orchestrated.StepLimitReached || orchestrated.Outcome != store.BattleRunStatusEnded || state.CombatTrialEndReason == "" {
		return nil, nil, nil, nil, nil, 0, "", fmt.Errorf("combat trial compile incomplete outcome=%q timed_out=%t step_limited=%t", orchestrated.Outcome, orchestrated.TimedOut, orchestrated.StepLimitReached)
	}
	timeline, playbackTimeline := encodeBattleTimelineWithPlayback(state.Events, state.StartedAt, turnInterval.Milliseconds())
	metrics := combatTrialMetricsSnapshot(&state)
	finalResult := map[string]any{
		"battleKind":   battleKindCombatTrial,
		"outcome":      state.CombatTrialEndReason,
		"endReason":    state.CombatTrialEndReason,
		"seed":         state.Seed,
		"metrics":      metrics,
		"actionCount":  state.CombatTrialActionCount,
		"actionBudget": run.ActionBudget,
		"totalEvents":  len(timeline),
		"totalTurns":   countBattleTimelineTurns(state.Events),
		"totalSteps":   orchestrated.Steps,
		"finalState":   combatTrialPublicState(state),
		"battleStats":  battleAchievementStatsFromState(state),
		"compiledAtMs": time.Now().UnixMilli(),
	}
	initialRaw, err := json.Marshal(initialState)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, "", err
	}
	timelineRaw, err := json.Marshal(timeline)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, "", err
	}
	finalRaw, err := json.Marshal(finalResult)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, "", err
	}
	return initialRaw, timelineRaw, finalRaw, playbackTimeline, metrics, state.CombatTrialActionCount, state.CombatTrialEndReason, nil
}

func (s *Server) startCombatTrialLoop(workerRuntimeID string) {
	cfg := s.deps.Config.GlobalSettings.CombatTrial
	if !cfg.Enabled {
		return
	}
	for index := 0; index < cfg.WorkerCount; index++ {
		workerID := fmt.Sprintf("%s:combat-trial:%d", workerRuntimeID, index+1)
		s.launchBattleTask(func() { s.runCombatTrialWorker(workerID) })
	}
	log.Printf("combat trial workers started count=%d history_keep=%d", cfg.WorkerCount, cfg.HistoryKeepCount)
}

func (s *Server) runCombatTrialWorker(workerID string) {
	cfg := s.deps.Config.GlobalSettings.CombatTrial
	poll := time.Duration(cfg.WorkerPollIntervalMs) * time.Millisecond
	lease := time.Duration(cfg.WorkerLeaseSeconds) * time.Second
	for {
		select {
		case <-s.battleStopCh:
			return
		default:
		}
		claimCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		run, err := s.deps.Store.ClaimCombatTrialRun(claimCtx, workerID, lease, time.Now())
		cancel()
		if errors.Is(err, pgx.ErrNoRows) {
			select {
			case <-time.After(poll):
				continue
			case <-s.battleStopCh:
				return
			}
		}
		if err != nil {
			log.Printf("combat trial claim failed worker=%s err=%v", workerID, err)
			select {
			case <-time.After(poll):
				continue
			case <-s.battleStopCh:
				return
			}
		}
		s.processClaimedCombatTrialRun(workerID, run)
	}
}

func (s *Server) processClaimedCombatTrialRun(workerID string, run store.CombatTrialRun) {
	compileCtx, cancelCompile := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCompile()
	s.pushBattleRoomEventToUsers([]int64{run.UserID}, "battle.run.started", fiber.Map{
		"battleKind": battleKindCombatTrial, "roomId": run.ID, "runId": run.ID,
	})
	initialRaw, timelineRaw, finalRaw, playbackTimeline, metrics, actionCount, endReason, err := s.compileCombatTrialRun(compileCtx, run, time.Now())
	if err != nil {
		log.Printf("combat trial compile failed run=%d worker=%s attempt=%d err=%v", run.ID, workerID, run.Attempts, err)
		failCtx, cancelFail := context.WithTimeout(context.Background(), 5*time.Second)
		terminal, failErr := s.deps.Store.FailCombatTrialRun(failCtx, run.ID, workerID, "compile_failed", s.deps.Config.GlobalSettings.CombatTrial.MaxAttempts)
		cancelFail()
		if failErr != nil && !errors.Is(failErr, store.ErrCombatTrialRunLeaseLost) {
			log.Printf("combat trial failure save failed run=%d worker=%s err=%v", run.ID, workerID, failErr)
		}
		if terminal && failErr == nil {
			s.pruneCombatTrialRuns(run.UserID, run.ID)
			s.pushBattleRoomEventToUsers([]int64{run.UserID}, "battle.run.finished", fiber.Map{"battleKind": battleKindCombatTrial, "roomId": run.ID, "runId": run.ID, "outcome": store.CombatTrialStatusFailed})
		}
		return
	}
	format, err := decodeCombatTrialFormatSnapshot(run.FormatSnapshot)
	if err != nil {
		log.Printf("combat trial result format decode failed run=%d worker=%s err=%v", run.ID, workerID, err)
		failCtx, cancelFail := context.WithTimeout(context.Background(), 5*time.Second)
		terminal, failErr := s.deps.Store.FailCombatTrialRun(failCtx, run.ID, workerID, "format_snapshot_invalid", s.deps.Config.GlobalSettings.CombatTrial.MaxAttempts)
		cancelFail()
		if failErr != nil && !errors.Is(failErr, store.ErrCombatTrialRunLeaseLost) {
			log.Printf("combat trial failure save failed run=%d worker=%s err=%v", run.ID, workerID, failErr)
		}
		if terminal && failErr == nil {
			s.pruneCombatTrialRuns(run.UserID, run.ID)
			s.pushBattleRoomEventToUsers([]int64{run.UserID}, "battle.run.finished", fiber.Map{"battleKind": battleKindCombatTrial, "roomId": run.ID, "runId": run.ID, "outcome": store.CombatTrialStatusFailed})
		}
		return
	}
	completeCtx, cancelComplete := context.WithTimeout(context.Background(), 5*time.Second)
	completed, err := s.deps.Store.CompleteCombatTrialRun(completeCtx, store.CompleteCombatTrialRunInput{
		RunID: run.ID, WorkerID: workerID, Metrics: metrics, ActionCount: actionCount, EndReason: endReason,
		MetricRankDirections: combatTrialRankDirections(format, endReason),
		InitialState:         initialRaw, Timeline: timelineRaw, FinalResult: finalRaw,
	})
	cancelComplete()
	if err != nil {
		log.Printf("combat trial completion failed run=%d worker=%s err=%v", run.ID, workerID, err)
		failCtx, cancelFail := context.WithTimeout(context.Background(), 5*time.Second)
		terminal, failErr := s.deps.Store.FailCombatTrialRun(failCtx, run.ID, workerID, "completion_failed", s.deps.Config.GlobalSettings.CombatTrial.MaxAttempts)
		cancelFail()
		if terminal && failErr == nil {
			s.pruneCombatTrialRuns(run.UserID, run.ID)
			s.pushBattleRoomEventToUsers([]int64{run.UserID}, "battle.run.finished", fiber.Map{"battleKind": battleKindCombatTrial, "roomId": run.ID, "runId": run.ID, "outcome": store.CombatTrialStatusFailed})
		}
		return
	}
	s.pruneCombatTrialRuns(run.UserID, run.ID)
	s.pushCombatTrialPlayback(completed, initialRaw, playbackTimeline)
	log.Printf("combat trial completed run=%d user=%d actions=%d reason=%s", run.ID, run.UserID, actionCount, endReason)
}

func (s *Server) pushCombatTrialPlayback(run store.CombatTrialRun, initialRaw json.RawMessage, timeline []wsbus.BattleTimelineEvent) {
	var snapshot map[string]any
	if run.ID <= 0 || run.UserID <= 0 || json.Unmarshal(initialRaw, &snapshot) != nil {
		return
	}
	users := []int64{run.UserID}
	startedAtMs := time.Now().UnixMilli()
	s.pushBattleRoomEventToUsers(users, "battle.run.compiled", fiber.Map{"battleKind": battleKindCombatTrial, "roomId": run.ID, "runId": run.ID})
	if !s.pushBattleRoomEventToUsers(users, "battle.run.batch", toCombatTrialBattleEventBatchWirePayload(run.ID, wsbus.BattleEventBatchPayload{
		RunID: run.ID, WindowStartMS: startedAtMs, WindowEndMS: startedAtMs, Events: []wsbus.BattleTimelineEvent{}, Snapshot: snapshot,
	})) {
		return
	}
	timelineCopy := append([]wsbus.BattleTimelineEvent(nil), timeline...)
	s.launchBattleTask(func() { s.streamCombatTrialTimeline(run, startedAtMs, timelineCopy) })
}

func (s *Server) streamCombatTrialTimeline(run store.CombatTrialRun, startedAtMs int64, timeline []wsbus.BattleTimelineEvent) {
	users := []int64{run.UserID}
	if len(timeline) > 0 && !s.waitBattlePlaybackDelay(battlePlaybackFirstEventDelay) {
		return
	}
	maxEvents := s.deps.Config.GlobalSettings.Battle.PlaybackMaxEventsBatch
	intervalMs := s.deps.Config.GlobalSettings.Battle.PlaybackFixedIntervalMs
	for start := 0; start < len(timeline); {
		events, next, _ := collectPlaybackEventsAtCurrentOffset(timeline, start, maxEvents)
		if len(events) == 0 {
			break
		}
		if !s.pushBattleRoomEventToUsers(users, "battle.run.batch", toCombatTrialBattleEventBatchWirePayload(run.ID, wsbus.BattleEventBatchPayload{
			RunID: run.ID, FromSeq: events[0].Seq, ToSeq: events[len(events)-1].Seq,
			WindowStartMS: startedAtMs + maxInt64(0, events[0].OffsetMS), WindowEndMS: startedAtMs + maxInt64(0, events[len(events)-1].OffsetMS), Events: events,
		})) {
			return
		}
		start = next
		if start < len(timeline) && !s.waitBattlePlaybackDelay(time.Duration(intervalMs)*time.Millisecond) {
			return
		}
	}
	s.pushBattleRoomEventToUsers(users, "battle.run.finished", fiber.Map{
		"battleKind": battleKindCombatTrial, "roomId": run.ID, "runId": run.ID, "outcome": run.EndReason,
	})
}

func (s *Server) pruneCombatTrialRuns(userID, runID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.deps.Store.PruneCombatTrialRuns(ctx, userID, s.deps.Config.GlobalSettings.CombatTrial.HistoryKeepCount); err != nil {
		log.Printf("combat trial history prune failed run=%d user=%d err=%v", runID, userID, err)
	}
}
