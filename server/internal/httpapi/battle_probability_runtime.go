package httpapi

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	battleProbabilityEquipmentProc = "equipment_proc"
	battleProbabilityStatusApply   = "status_apply"
)

type battleProbabilityCheck struct {
	Kind                string
	Actor               *battleUnitState
	Target              *battleUnitState
	BaseChance          float64
	SourceIDs           []string
	IgnoreHooks         bool
	ProbabilityGroup    string
	MaxSuccessesPerTurn int
}

type battleProbabilityResult struct {
	Kind          string
	ActorID       string
	TargetID      string
	SourceIDs     []string
	BaseChance    float64
	FinalChance   float64
	Roll          float64
	Rolled        bool
	Established   bool
	Succeeded     bool
	ForcedFailure bool
	IgnoreHooks   bool
}

type battleProbabilityModifierCandidate struct {
	Priority int
	ID       string
	Apply    func(*float64) (recognized bool, forcedFailure bool)
}

type battlePendingProbabilityOutcome struct {
	Check  battleProbabilityCheck
	Result battleProbabilityResult
}

func battleNormalizeProbabilityKind(raw string) string {
	switch battleNormalizeTrigger(raw) {
	case "equipmentproc":
		return battleProbabilityEquipmentProc
	case "statusapply":
		return battleProbabilityStatusApply
	default:
		return ""
	}
}

func battleIsProbabilityOutcomeTrigger(raw string) bool {
	switch battleNormalizeTrigger(raw) {
	case "onprobabilitysuccess", "onprobabilityfailure":
		return true
	default:
		return false
	}
}

func battleRunProbabilityExtraTurn(
	state *battleRuntimeState,
	run func() (map[string]any, bool),
) (payload map[string]any, ok bool) {
	if run == nil {
		return nil, false
	}
	if state == nil || state.probabilityHookDepth <= 0 {
		return run()
	}
	outerDepth := state.probabilityHookDepth
	outerHookTurnCounts := state.BattleHookTurnCounts
	outerSuccessCounts := state.ProbabilitySuccessCounts
	state.probabilityHookDepth = 0
	defer func() {
		state.probabilityHookDepth = outerDepth
		state.BattleHookTurnCounts = outerHookTurnCounts
		state.ProbabilitySuccessCounts = outerSuccessCounts
	}()
	return run()
}

func battleProbabilityStringAllowed(allowed []string, actual string) bool {
	actual = battleNormalizeProbabilityKind(actual)
	if actual == "" {
		return false
	}
	for _, candidate := range allowed {
		if battleNormalizeProbabilityKind(candidate) == actual {
			return true
		}
	}
	return false
}

func battleProbabilitySourceAllowed(allowed []string, actual []string) bool {
	for _, candidate := range allowed {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		for _, sourceID := range actual {
			if strings.EqualFold(candidate, strings.TrimSpace(sourceID)) {
				return true
			}
		}
	}
	return false
}

func battleProbabilityEventFilterAllowed(filter map[string]any, kind string, sourceIDs []string) bool {
	if len(filter) == 0 {
		return true
	}
	kinds := battleParseStringList(firstNonNil(filter["probabilityKinds"], filter["probability_kinds"], filter["probabilityKind"], filter["probability_kind"]))
	if len(kinds) > 0 && !battleProbabilityStringAllowed(kinds, kind) {
		return false
	}
	included := battleParseStringList(firstNonNil(filter["sourceIds"], filter["source_ids"], filter["probabilitySourceIds"], filter["probability_source_ids"]))
	if len(included) > 0 && !battleProbabilitySourceAllowed(included, sourceIDs) {
		return false
	}
	excluded := battleParseStringList(firstNonNil(filter["excludeSourceIds"], filter["exclude_source_ids"], filter["excludeProbabilitySourceIds"], filter["exclude_probability_source_ids"]))
	return len(excluded) == 0 || !battleProbabilitySourceAllowed(excluded, sourceIDs)
}

func battleProbabilityEffectFilterAllowed(effect map[string]any, kind string, sourceIDs []string) bool {
	return battleProbabilityEventFilterAllowed(effect, kind, sourceIDs)
}

func battleHookCounterGroup(sharedGroup, fallback string) string {
	group := strings.TrimSpace(sharedGroup)
	if group == "" || group == "<nil>" {
		group = strings.TrimSpace(fallback)
	}
	return group
}

// Counter keys are persisted in battle JSONB snapshots. PostgreSQL JSONB
// rejects U+0000 even though encoding/json can represent it as \u0000, so the
// old NUL-delimited composite keys could make an otherwise valid run
// impossible to save. Base64url keeps the pair collision-free and JSONB-safe.
func battleRuntimeCounterKey(unitID, group string) string {
	unitPart := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(unitID)))
	groupPart := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(group)))
	return unitPart + "." + groupPart
}

// Reject non-canonical keys so carry pruning can preserve unknown legacy
// formats instead of guessing their ownership.
func battleRuntimeCounterKeyUnitID(key string) (string, bool) {
	unitPart, groupPart, ok := strings.Cut(key, ".")
	if !ok || strings.Contains(groupPart, ".") {
		return "", false
	}
	unitID, err := base64.RawURLEncoding.DecodeString(unitPart)
	if err != nil || base64.RawURLEncoding.EncodeToString(unitID) != unitPart {
		return "", false
	}
	group, err := base64.RawURLEncoding.DecodeString(groupPart)
	if err != nil || base64.RawURLEncoding.EncodeToString(group) != groupPart {
		return "", false
	}
	return string(unitID), true
}

func battleHookCounterKeys(state *battleRuntimeState, unit *battleUnitState, group string) (string, string) {
	unitID := "unknown"
	if unit != nil && strings.TrimSpace(unit.UnitID) != "" {
		unitID = strings.TrimSpace(unit.UnitID)
	}
	turnKey := battleRuntimeCounterKey(unitID, group)
	battleKey := turnKey
	return turnKey, battleKey
}

func battleHookLimitAllowed(state *battleRuntimeState, unit *battleUnitState, group string, maxPerTurn, maxPerBattle int) bool {
	if state == nil || (maxPerTurn <= 0 && maxPerBattle <= 0) {
		return true
	}
	turnKey, battleKey := battleHookCounterKeys(state, unit, group)
	if maxPerTurn > 0 && state.BattleHookTurnCounts[turnKey] >= maxPerTurn {
		return false
	}
	if maxPerBattle > 0 && state.BattleHookBattleCounts[battleKey] >= maxPerBattle {
		return false
	}
	return true
}

func battleMarkHookTriggered(state *battleRuntimeState, unit *battleUnitState, group string) {
	if state == nil {
		return
	}
	if state.BattleHookTurnCounts == nil {
		state.BattleHookTurnCounts = map[string]int{}
	}
	if state.BattleHookBattleCounts == nil {
		state.BattleHookBattleCounts = map[string]int{}
	}
	turnKey, battleKey := battleHookCounterKeys(state, unit, group)
	state.BattleHookTurnCounts[turnKey]++
	state.BattleHookBattleCounts[battleKey]++
}

func battleEquipmentHookCounterGroup(hook battleEquipmentHookRule) string {
	fallback := strings.TrimSpace(hook.ID)
	if fallback == "" {
		fallback = strings.TrimSpace(hook.SourceItemID)
	}
	if fallback == "" {
		fallback = "anonymous_equipment_hook"
	}
	return battleHookCounterGroup(hook.SharedTriggerGroup, "equipment:"+fallback)
}

func parseBattleHookActivationCost(raw any) battleHookActivationCost {
	row := normalizeAnyMap(raw)
	if len(row) == 0 {
		return battleHookActivationCost{}
	}
	return battleHookActivationCost{
		MaxHPRatio: clampFloat(toFloat64(firstNonNil(row["maxHpRatio"], row["max_hp_ratio"]), 0), 0, 1),
		MaxMPRatio: clampFloat(toFloat64(firstNonNil(row["maxMpRatio"], row["max_mp_ratio"]), 0), 0, 1),
	}
}

func battleEquipmentHookCooldownKey(unit *battleUnitState, hook battleEquipmentHookRule) string {
	unitID := "unknown"
	if unit != nil && strings.TrimSpace(unit.UnitID) != "" {
		unitID = strings.TrimSpace(unit.UnitID)
	}
	return battleRuntimeCounterKey(unitID, battleEquipmentHookCounterGroup(hook))
}

func battleEquipmentHookActivationAmounts(unit *battleUnitState, hook battleEquipmentHookRule) (float64, float64) {
	if unit == nil {
		return 0, 0
	}
	hpCost := math.Floor(maxFloat(0, unit.MaxHP) * clampFloat(hook.ActivationCost.MaxHPRatio, 0, 1))
	mpCost := math.Floor(maxFloat(0, unit.MaxMP) * clampFloat(hook.ActivationCost.MaxMPRatio, 0, 1))
	return maxFloat(0, hpCost), maxFloat(0, mpCost)
}

func battleEquipmentHookActivationAllowed(state *battleRuntimeState, unit *battleUnitState, hook battleEquipmentHookRule) bool {
	if !battleEquipmentHookLimitAllowed(state, unit, hook) {
		return false
	}
	if unit == nil || !unit.Alive || unit.HP <= 0 {
		return false
	}
	if state != nil && hook.CooldownCompletedTurns > 0 {
		until := state.BattleHookCooldownUntilTurns[battleEquipmentHookCooldownKey(unit, hook)]
		if state.CompletedTurnCounts[unit.UnitID] < until {
			return false
		}
	}
	hpCost, mpCost := battleEquipmentHookActivationAmounts(unit, hook)
	return unit.HP > hpCost && unit.MP >= mpCost
}

func battleActivateEquipmentHook(state *battleRuntimeState, unit *battleUnitState, hook battleEquipmentHookRule, trigger string) ([]map[string]any, bool) {
	if !battleEquipmentHookActivationAllowed(state, unit, hook) {
		return nil, false
	}
	hpCost, mpCost := battleEquipmentHookActivationAmounts(unit, hook)
	actions := make([]map[string]any, 0, 2)
	appendResourceAction := func(resource string, before, after float64) {
		actions = append(actions, map[string]any{
			"kind":     "resource",
			"resource": resource,
			"targetId": unit.UnitID,
			"sourceId": unit.UnitID,
			"amount":   roundTo4(after - before),
			"reason":   trigger,
			"delayMs":  int64(0),
			"meta": map[string]any{
				"sourceItemId":       hook.SourceItemID,
				"sourceEnhanceLevel": hook.SourceEnhanceLevel,
				"hookId":             hook.ID,
				"activationCost":     true,
			},
		})
	}
	if hpCost > 0 {
		before := unit.HP
		unit.HP -= hpCost
		appendResourceAction("hp", before, unit.HP)
	}
	if mpCost > 0 {
		before := unit.MP
		unit.MP -= mpCost
		appendResourceAction("mp", before, unit.MP)
	}
	if state != nil && hook.CooldownCompletedTurns > 0 {
		if state.BattleHookCooldownUntilTurns == nil {
			state.BattleHookCooldownUntilTurns = map[string]int{}
		}
		state.BattleHookCooldownUntilTurns[battleEquipmentHookCooldownKey(unit, hook)] = state.CompletedTurnCounts[unit.UnitID] + hook.CooldownCompletedTurns
	}
	battleMarkEquipmentHookTriggered(state, unit, hook)
	return actions, true
}

func battleEquipmentHookLimitAllowed(state *battleRuntimeState, unit *battleUnitState, hook battleEquipmentHookRule) bool {
	group := battleEquipmentHookCounterGroup(hook)
	if !battleHookLimitAllowed(state, unit, group, hook.MaxTriggersPerTurn, hook.MaxTriggersPerBattle) {
		return false
	}
	if state == nil || hook.MaxTriggersPerWave <= 0 {
		return true
	}
	if state.SpecializationHookWaveCounts == nil {
		return true
	}
	unitID := "unknown"
	if unit != nil && strings.TrimSpace(unit.UnitID) != "" {
		unitID = strings.TrimSpace(unit.UnitID)
	}
	return state.SpecializationHookWaveCounts[battleRuntimeCounterKey(unitID, group)] < hook.MaxTriggersPerWave
}

func battleMarkEquipmentHookTriggered(state *battleRuntimeState, unit *battleUnitState, hook battleEquipmentHookRule) {
	if hook.MaxTriggersPerTurn <= 0 && hook.MaxTriggersPerBattle <= 0 && hook.MaxTriggersPerWave <= 0 {
		return
	}
	group := battleEquipmentHookCounterGroup(hook)
	battleMarkHookTriggered(state, unit, group)
	if state == nil || hook.MaxTriggersPerWave <= 0 {
		return
	}
	if state.SpecializationHookWaveCounts == nil {
		state.SpecializationHookWaveCounts = map[string]int{}
	}
	unitID := "unknown"
	if unit != nil && strings.TrimSpace(unit.UnitID) != "" {
		unitID = strings.TrimSpace(unit.UnitID)
	}
	state.SpecializationHookWaveCounts[battleRuntimeCounterKey(unitID, group)]++
}

func battleBuffRuleCounterGroup(rule battleBuffEffectRule) string {
	fallback := strings.TrimSpace(rule.ID)
	if fallback == "" {
		fallback = strings.TrimSpace(rule.BuffID)
	}
	if fallback == "" {
		fallback = "anonymous_buff_hook"
	}
	return battleHookCounterGroup(rule.SharedTriggerGroup, "buff:"+fallback)
}

func battleBuffRuleLimitAllowed(state *battleRuntimeState, unit *battleUnitState, rule battleBuffEffectRule) bool {
	return battleHookLimitAllowed(state, unit, battleBuffRuleCounterGroup(rule), rule.MaxTriggersPerTurn, rule.MaxTriggersPerBattle)
}

func battleMarkBuffRuleTriggered(state *battleRuntimeState, unit *battleUnitState, rule battleBuffEffectRule) {
	if rule.MaxTriggersPerTurn <= 0 && rule.MaxTriggersPerBattle <= 0 {
		return
	}
	battleMarkHookTriggered(state, unit, battleBuffRuleCounterGroup(rule))
}

func (state *battleRuntimeState) beginBattleHookTurn() {
	if state == nil {
		return
	}
	state.BattleHookTurnCounts = map[string]int{}
	state.ProbabilitySuccessCounts = map[string]int{}
}

func battleProbabilitySuccessCounterKey(actor *battleUnitState, group string) string {
	unitID := "unknown"
	if actor != nil && strings.TrimSpace(actor.UnitID) != "" {
		unitID = strings.TrimSpace(actor.UnitID)
	}
	return battleRuntimeCounterKey(unitID, group)
}

func battleProbabilitySuccessCapReached(state *battleRuntimeState, check battleProbabilityCheck) bool {
	if state == nil || check.MaxSuccessesPerTurn <= 0 || strings.TrimSpace(check.ProbabilityGroup) == "" {
		return false
	}
	return state.ProbabilitySuccessCounts[battleProbabilitySuccessCounterKey(check.Actor, check.ProbabilityGroup)] >= check.MaxSuccessesPerTurn
}

func battleMarkProbabilitySuccess(state *battleRuntimeState, check battleProbabilityCheck) {
	if state == nil || check.MaxSuccessesPerTurn <= 0 || strings.TrimSpace(check.ProbabilityGroup) == "" {
		return
	}
	if state.ProbabilitySuccessCounts == nil {
		state.ProbabilitySuccessCounts = map[string]int{}
	}
	state.ProbabilitySuccessCounts[battleProbabilitySuccessCounterKey(check.Actor, check.ProbabilityGroup)]++
}

func battleProbabilityRollSucceeds(kind string, chance, roll float64) bool {
	if battleNormalizeProbabilityKind(kind) == battleProbabilityStatusApply {
		return roll < chance
	}
	return roll <= chance
}

func battleProbabilityModifierValueForEquipment(actor *battleUnitState, hook battleEquipmentHookRule, effect map[string]any) float64 {
	value := toFloat64(firstNonNil(effect["value"], effect["percent"], effect["amount"]), 0)
	if formulaRaw := firstNonNil(effect["valueFormula"], effect["value_formula"], effect["amountFormula"], effect["amount_formula"]); formulaRaw != nil {
		value += battleEvaluateEquipmentHookProcChanceFormula(actor, hook, parseBattleSkillAmountFormula(formulaRaw, nil))
	}
	return value
}

func battleApplyProbabilityModifierEffect(effect map[string]any, value float64, chance *float64) (bool, bool) {
	if chance == nil {
		return false, false
	}
	switch battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) {
	case "addprobability":
		*chance += value
		return true, false
	case "setprobability":
		*chance = value
		return true, false
	case "forceprobabilityfailure":
		return true, true
	default:
		return false, false
	}
}

func (s *Server) battleCollectProbabilityModifiers(state *battleRuntimeState, check battleProbabilityCheck) []battleProbabilityModifierCandidate {
	if s == nil || check.Actor == nil {
		return nil
	}
	context := &battleEquipmentHookContext{ProbabilityKind: check.Kind, ProbabilitySourceIDs: append([]string(nil), check.SourceIDs...)}
	candidates := make([]battleProbabilityModifierCandidate, 0, 4)
	for index := 0; index < check.Actor.equipmentHookCount(); index++ {
		hook, ok := check.Actor.equipmentHookAt(index)
		if !ok || battleNormalizeTrigger(hook.Trigger) != "onbeforeprobabilitycheck" {
			continue
		}
		if !battleEquipmentHookSubjectAllowed(hook, check.Target) || !battleEquipmentHookEventAllowed(hook, check.Actor, check.Target, context) || !battleEquipmentHookLimitAllowed(state, check.Actor, hook) {
			continue
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, check.Actor, check.Target, check.Target, 0, "", hook.SourceEnhanceLevel, context) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		hookCopy := hook
		candidates = append(candidates, battleProbabilityModifierCandidate{
			Priority: hook.Priority,
			ID:       hook.ID,
			Apply: func(chance *float64) (bool, bool) {
				if !battleEquipmentHookLimitAllowed(state, check.Actor, hookCopy) {
					return false, false
				}
				recognized := false
				for _, effect := range hookCopy.Effects {
					if !battleProbabilityEffectFilterAllowed(effect, check.Kind, check.SourceIDs) {
						continue
					}
					matched, forced := battleApplyProbabilityModifierEffect(effect, battleProbabilityModifierValueForEquipment(check.Actor, hookCopy, effect), chance)
					recognized = recognized || matched
					if forced {
						if recognized {
							battleMarkEquipmentHookTriggered(state, check.Actor, hookCopy)
						}
						return recognized, true
					}
				}
				if recognized {
					battleMarkEquipmentHookTriggered(state, check.Actor, hookCopy)
				}
				return recognized, false
			},
		})
	}
	for _, entry := range s.battleCollectBuffRuleEntries(check.Actor, "on_before_probability_check") {
		rule := entry.Rule
		if !battleProbabilityEventFilterAllowed(rule.EventFilter, check.Kind, check.SourceIDs) || !battleBuffRuleLimitAllowed(state, check.Actor, rule) {
			continue
		}
		currentBuff := findBuffByID(check.Actor, entry.BuffID)
		if currentBuff == nil {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(entry.BuffID)
		if !battleRuleConditionsPassed(rule, check.Actor, check.Target, 0, "", currentBuff, battleBuffChargesRemaining(currentBuff, maxCharges)) {
			continue
		}
		buffCopy := *currentBuff
		ruleCopy := rule
		candidates = append(candidates, battleProbabilityModifierCandidate{
			Priority: rule.Priority,
			ID:       rule.ID,
			Apply: func(chance *float64) (bool, bool) {
				if !battleBuffRuleLimitAllowed(state, check.Actor, ruleCopy) {
					return false, false
				}
				recognized := false
				for _, effect := range ruleCopy.Effects {
					if !battleProbabilityEffectFilterAllowed(effect, check.Kind, check.SourceIDs) {
						continue
					}
					value := battleBuffEffectScalar(effect, &buffCopy, firstNonNil(effect["value"], effect["percent"], effect["amount"]))
					matched, forced := battleApplyProbabilityModifierEffect(effect, value, chance)
					recognized = recognized || matched
					if forced {
						if recognized {
							battleMarkBuffRuleTriggered(state, check.Actor, ruleCopy)
						}
						return recognized, true
					}
				}
				if recognized {
					battleMarkBuffRuleTriggered(state, check.Actor, ruleCopy)
				}
				return recognized, false
			},
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

func (s *Server) resolveBattleProbability(state *battleRuntimeState, check battleProbabilityCheck) battleProbabilityResult {
	check.Kind = battleNormalizeProbabilityKind(check.Kind)
	result := battleProbabilityResult{
		Kind:        check.Kind,
		SourceIDs:   append([]string(nil), check.SourceIDs...),
		BaseChance:  clampFloat(check.BaseChance, 0, 1),
		FinalChance: clampFloat(check.BaseChance, 0, 1),
		IgnoreHooks: check.IgnoreHooks,
	}
	if check.Actor != nil {
		result.ActorID = strings.TrimSpace(check.Actor.UnitID)
	}
	if check.Target != nil {
		result.TargetID = strings.TrimSpace(check.Target.UnitID)
	}
	if check.Kind == "" || battleProbabilitySuccessCapReached(state, check) {
		return result
	}
	result.Established = true
	if !check.IgnoreHooks && (state == nil || state.probabilityHookDepth == 0) {
		for _, modifier := range s.battleCollectProbabilityModifiers(state, check) {
			_, forced := modifier.Apply(&result.FinalChance)
			if forced {
				result.ForcedFailure = true
				result.Succeeded = false
				result.FinalChance = clampFloat(result.FinalChance, 0, 1)
				return result
			}
		}
	}
	result.FinalChance = clampFloat(result.FinalChance, 0, 1)
	if result.FinalChance <= 0 {
		return result
	}
	if result.FinalChance >= 1 {
		result.Succeeded = true
		battleMarkProbabilitySuccess(state, check)
		return result
	}
	if state == nil {
		return result
	}
	result.Roll = state.randFloat64()
	result.Rolled = true
	result.Succeeded = battleProbabilityRollSucceeds(result.Kind, result.FinalChance, result.Roll)
	if result.Succeeded {
		battleMarkProbabilitySuccess(state, check)
	}
	return result
}

func battleProbabilityResultMeta(result battleProbabilityResult) map[string]any {
	meta := map[string]any{
		"probabilityKind":          result.Kind,
		"probabilitySourceIds":     append([]string(nil), result.SourceIDs...),
		"baseProbability":          roundTo4(result.BaseChance),
		"finalProbability":         roundTo4(result.FinalChance),
		"probabilitySucceeded":     result.Succeeded,
		"probabilityForcedFailure": result.ForcedFailure,
	}
	if result.Rolled {
		meta["probabilityRoll"] = roundTo4(result.Roll)
	}
	return meta
}

func battleAppendProbabilityMeta(actions []map[string]any, result battleProbabilityResult) {
	probabilityMeta := battleProbabilityResultMeta(result)
	for _, action := range actions {
		if action == nil {
			continue
		}
		action["meta"] = mergeActionMeta(normalizeAnyMap(action["meta"]), probabilityMeta)
	}
}

func (s *Server) battleFinalizeProbabilityResult(state *battleRuntimeState, now time.Time, check battleProbabilityCheck, result battleProbabilityResult) []map[string]any {
	if s == nil || check.Actor == nil || !result.Established || result.IgnoreHooks || (state != nil && state.probabilityHookDepth > 0) {
		return nil
	}
	trigger := "on_probability_failure"
	if result.Succeeded {
		trigger = "on_probability_success"
	}
	context := &battleEquipmentHookContext{
		ProbabilityKind:      result.Kind,
		ProbabilitySourceIDs: append([]string(nil), result.SourceIDs...),
	}
	if state != nil {
		state.probabilityHookDepth++
		defer func() { state.probabilityHookDepth-- }()
	}
	buffOutcomeIDs := make([]string, 0, 2)
	seenBuffOutcomeIDs := map[string]struct{}{}
	for _, entry := range s.battleCollectBuffRuleEntries(check.Actor, trigger) {
		if !battleProbabilityEventFilterAllowed(entry.Rule.EventFilter, result.Kind, result.SourceIDs) {
			continue
		}
		if _, exists := seenBuffOutcomeIDs[entry.BuffID]; exists {
			continue
		}
		seenBuffOutcomeIDs[entry.BuffID] = struct{}{}
		buffOutcomeIDs = append(buffOutcomeIDs, entry.BuffID)
	}
	actions := s.battleTriggerUnitEquipmentHooksWithContext(state, now, check.Actor, check.Target, check.Target, trigger, 0, "", context)
	if len(buffOutcomeIDs) > 0 {
		buffActions := s.battleTriggerUnitBuffHooksWithSourceAndFilter(state, now, check.Actor, check.Target, trigger, func(rule battleBuffEffectRule) bool {
			return battleProbabilityEventFilterAllowed(rule.EventFilter, result.Kind, result.SourceIDs)
		}, buffOutcomeIDs...)
		if len(buffActions) > 0 {
			actions = append(actions, buffActions...)
		}
	}
	battleAppendProbabilityMeta(actions, result)
	return actions
}

func (s *Server) battleFinalizePendingProbabilityOutcomes(state *battleRuntimeState, now time.Time, pending []battlePendingProbabilityOutcome) []map[string]any {
	if len(pending) == 0 {
		return nil
	}
	actions := make([]map[string]any, 0, len(pending))
	for _, outcome := range pending {
		check := outcome.Check
		if state != nil {
			if actor := battleUnitByID(state, outcome.Result.ActorID); actor != nil {
				check.Actor = actor
			}
			if target := battleUnitByID(state, outcome.Result.TargetID); target != nil {
				check.Target = target
			}
		}
		actions = append(actions, s.battleFinalizeProbabilityResult(state, now, check, outcome.Result)...)
	}
	if len(actions) == 0 {
		return nil
	}
	return actions
}

func battleEquipmentProbabilityCheck(actor, target *battleUnitState, hook battleEquipmentHookRule, effect map[string]any, chance float64) battleProbabilityCheck {
	sourceIDs := make([]string, 0, 3)
	for _, sourceID := range []string{hook.SourceID, hook.SourceItemID, hook.ID} {
		if sourceID = strings.TrimSpace(sourceID); sourceID != "" && !battleProbabilitySourceAllowed(sourceIDs, []string{sourceID}) {
			sourceIDs = append(sourceIDs, sourceID)
		}
	}
	probabilityGroup := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["probabilityGroup"], effect["probability_group"])))
	if probabilityGroup == "<nil>" {
		probabilityGroup = ""
	}
	return battleProbabilityCheck{
		Kind:                battleProbabilityEquipmentProc,
		Actor:               actor,
		Target:              target,
		BaseChance:          chance,
		SourceIDs:           sourceIDs,
		IgnoreHooks:         parseBooleanLoose(firstNonNil(effect["ignoreProbabilityHooks"], effect["ignore_probability_hooks"]), false),
		ProbabilityGroup:    probabilityGroup,
		MaxSuccessesPerTurn: maxInt(0, int(toInt64Loose(firstNonNil(effect["maxSuccessesPerTurn"], effect["max_successes_per_turn"]), 0))),
	}
}

func (s *Server) battleResolveEquipmentEffectProbability(state *battleRuntimeState, actor, target *battleUnitState, hook battleEquipmentHookRule, effect map[string]any) (battleProbabilityCheck, battleProbabilityResult, bool) {
	chance, configured := battleResolveEquipmentHookProcChance(actor, hook, effect)
	if !configured {
		return battleProbabilityCheck{}, battleProbabilityResult{}, false
	}
	check := battleEquipmentProbabilityCheck(actor, target, hook, effect, chance)
	return check, s.resolveBattleProbability(state, check), true
}

func (s *Server) battleResolveStatusApplyProbability(state *battleRuntimeState, actor, target *battleUnitState, baseChance float64, sourceIDs ...string) (battleProbabilityCheck, battleProbabilityResult) {
	filteredSourceIDs := make([]string, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if sourceID = strings.TrimSpace(sourceID); sourceID != "" {
			filteredSourceIDs = append(filteredSourceIDs, sourceID)
		}
	}
	check := battleProbabilityCheck{
		Kind:       battleProbabilityStatusApply,
		Actor:      actor,
		Target:     target,
		BaseChance: baseChance,
		SourceIDs:  filteredSourceIDs,
	}
	return check, s.resolveBattleProbability(state, check)
}
