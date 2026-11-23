package httpapi

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const battleEffectPositionMapKey = "positionmap"

const battleBuffConditionIncomingDamageWouldBeLethal = "incomingdamagewouldbelethal"

type battleBuffEffectRule struct {
	ID                   string
	BuffID               string
	Trigger              string
	Priority             int
	MaxTriggersPerTurn   int
	MaxTriggersPerBattle int
	SharedTriggerGroup   string
	EventFilter          map[string]any
	Conditions           []map[string]any
	ConditionTypes       []string // pre-normalized condition types, len == len(Conditions)
	Effects              []map[string]any
	EffectKinds          []string // pre-normalized effect kinds, len == len(Effects)
}

type battleBuffRuleEntry struct {
	BuffIndex int
	BuffID    string
	Rule      battleBuffEffectRule
}

func (s *Server) battleTurnIntervalMS() int64 {
	interval := s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs
	if interval <= 0 {
		return 1000
	}
	return interval
}

func battleBuffEffectScalar(effect map[string]any, buff *battleBuffState, baseRaw any) float64 {
	value := toFloat64(baseRaw, 0)
	if effect == nil {
		return value
	}
	key := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["payloadValueKey"], effect["payload_value_key"], effect["payloadKey"], effect["payload_key"])))
	if key != "" && key != "<nil>" && buff != nil && buff.Payload != nil {
		coeff := toFloat64(firstNonNil(effect["payloadValueCoeff"], effect["payload_value_coeff"], effect["payloadCoeff"], effect["payload_coeff"]), 1)
		value += toFloat64(buff.Payload[key], 0) * coeff
	}
	if minRaw := firstNonNil(effect["valueMin"], effect["value_min"], effect["min"]); minRaw != nil {
		value = maxFloat(value, toFloat64(minRaw, value))
	}
	if maxRaw := firstNonNil(effect["valueMax"], effect["value_max"], effect["max"]); maxRaw != nil {
		if maxValue := toFloat64(maxRaw, value); value > maxValue {
			value = maxValue
		}
	}
	return value
}

func battleBuffNamedPayloadScalar(effect map[string]any, buff *battleBuffState, valueField, payloadKeyField string, fallback float64) float64 {
	if effect == nil {
		return fallback
	}
	value := toFloat64(effect[valueField], fallback)
	payloadKey := strings.TrimSpace(fmt.Sprintf("%v", effect[payloadKeyField]))
	if payloadKey != "" && payloadKey != "<nil>" && buff != nil && buff.Payload != nil {
		value = toFloat64(buff.Payload[payloadKey], value)
	}
	return value
}

func (s *Server) battleApplyOutgoingBuffModifiers(source *battleUnitState, outputKind string, amount float64) (float64, []map[string]any) {
	if s == nil || source == nil || amount <= 0 {
		return amount, nil
	}
	outputKind = strings.ToLower(strings.TrimSpace(outputKind))
	if outputKind != "damage" && outputKind != "heal" {
		return amount, nil
	}
	adjusted := amount
	sources := make([]map[string]any, 0, 1)
	for _, entry := range s.battleCollectBuffRuleEntries(source, "while_active") {
		currentBuff := findBuffByID(source, entry.BuffID)
		if currentBuff == nil {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(entry.BuffID)
		chargesRemaining := battleBuffChargesRemaining(currentBuff, maxCharges)
		if maxCharges <= 0 || !battleRuleConditionsPassed(entry.Rule, source, nil, adjusted, "", currentBuff, chargesRemaining) {
			continue
		}
		for _, effect := range entry.Rule.Effects {
			if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) != "linearoutputmultiplier" ||
				!battleStringSliceContainsFold(battleParseStringList(effect["outputs"]), outputKind) {
				continue
			}
			start := battleBuffNamedPayloadScalar(effect, currentBuff, "startMultiplier", "startMultiplierPayloadKey", 1)
			recovery := battleBuffNamedPayloadScalar(effect, currentBuff, "recoveryPerChargeConsumed", "recoveryPerChargeConsumedPayloadKey", 0)
			maximum := battleBuffNamedPayloadScalar(effect, currentBuff, "maximumMultiplier", "maximumMultiplierPayloadKey", 1)
			consumed := maxInt64(0, maxCharges-chargesRemaining)
			multiplier := clampFloat(start+float64(consumed)*recovery, 0, maxFloat(0, maximum))
			before := adjusted
			adjusted = maxFloat(0, adjusted*multiplier)
			sources = append(sources, map[string]any{
				"kind": "buff", "id": currentBuff.BuffID, "ruleId": entry.Rule.ID,
				"sourceId": source.UnitID, "output": outputKind, "multiplier": roundTo4(multiplier),
				"chargesRemaining": chargesRemaining, "before": roundTo4(before), "after": roundTo4(adjusted),
			})
		}
	}
	return adjusted, sources
}

func battleNormalizeTrigger(raw string) string {
	// Strips underscores/hyphens. JSON effect kind "skip_turn_order"
	// becomes "skipturnorder". Go constants must match normalized form.
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "_", "")
	raw = strings.ReplaceAll(raw, "-", "")
	return raw
}

func (s *Server) battleParseBuffRules() []battleBuffEffectRule {
	s.buffRulesOnce.Do(func() {
		out := make([]battleBuffEffectRule, 0, len(s.deps.GameData.BuffEffects))
		for _, raw := range s.deps.GameData.BuffEffects {
			if raw == nil {
				continue
			}
			id := strings.TrimSpace(fmt.Sprintf("%v", raw["id"]))
			buffID := strings.TrimSpace(fmt.Sprintf("%v", raw["buffId"]))
			trigger := battleNormalizeTrigger(fmt.Sprintf("%v", raw["trigger"]))
			if id == "" || buffID == "" || trigger == "" {
				continue
			}
			conditions := battleAnyMapSlice(raw["conditions"])
			effects := battleAnyMapSlice(raw["effects"])
			if len(effects) == 0 {
				continue
			}
			condTypes := make([]string, len(conditions))
			for i, cond := range conditions {
				ct := battleNormalizeTrigger(fmt.Sprintf("%v", cond["type"]))
				if ct == "" {
					ct = battleNormalizeTrigger(fmt.Sprintf("%v", cond["kind"]))
				}
				condTypes[i] = ct
			}
			effectKinds := make([]string, len(effects))
			for i, eff := range effects {
				ek := battleNormalizeTrigger(fmt.Sprintf("%v", eff["kind"]))
				if ek == "" {
					ek = battleNormalizeTrigger(fmt.Sprintf("%v", eff["type"]))
				}
				effectKinds[i] = ek
			}
			out = append(out, battleBuffEffectRule{
				ID:                   id,
				BuffID:               strings.ToLower(buffID),
				Trigger:              trigger,
				Priority:             int(toInt64Loose(raw["priority"], 0)),
				MaxTriggersPerTurn:   maxInt(0, int(toInt64Loose(firstNonNil(raw["maxTriggersPerTurn"], raw["max_triggers_per_turn"]), 0))),
				MaxTriggersPerBattle: maxInt(0, int(toInt64Loose(firstNonNil(raw["maxTriggersPerBattle"], raw["max_triggers_per_battle"]), 0))),
				SharedTriggerGroup:   strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(raw["sharedTriggerGroup"], raw["shared_trigger_group"]))),
				EventFilter:          normalizeAnyMap(firstNonNil(raw["eventFilter"], raw["event_filter"])),
				Conditions:           conditions,
				ConditionTypes:       condTypes,
				Effects:              effects,
				EffectKinds:          effectKinds,
			})
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Priority != out[j].Priority {
				return out[i].Priority > out[j].Priority
			}
			return out[i].ID < out[j].ID
		})
		idx := make(map[string][]battleBuffEffectRule, len(out))
		for _, rule := range out {
			key := rule.BuffID + "\x00" + rule.Trigger
			idx[key] = append(idx[key], rule)
		}
		s.buffRulesCache = out
		s.buffRulesIndex = idx
		maxCharges := make(map[string]int64, len(s.deps.GameData.Buff))
		tickOnBattleStart := make(map[string]bool, len(s.deps.GameData.Buff))
		onExpire := make(map[string]string, len(s.deps.GameData.Buff))
		buffKinds := make(map[string]string, len(s.deps.GameData.Buff))
		dispellable := make(map[string]bool, len(s.deps.GameData.Buff))
		durationTurns := make(map[string]int64, len(s.deps.GameData.Buff))
		stackPolicies := make(map[string]string, len(s.deps.GameData.Buff))
		maxStacks := make(map[string]int, len(s.deps.GameData.Buff))
		for _, raw := range s.deps.GameData.Buff {
			id := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", raw["id"])))
			if id == "" {
				continue
			}
			// The legacy metadata lookups returned the first matching row.
			if _, exists := durationTurns[id]; !exists {
				duration := int64(0)
				if parseBooleanLoose(raw["permanent"], false) {
					duration = -1
				} else if turns := toInt64Loose(raw["durationTurns"], toInt64Loose(raw["duration"], 0)); turns > 0 {
					duration = turns
				}
				durationTurns[id] = duration
				stackPolicies[id] = battleNormalizeBuffStackPolicy(fmt.Sprintf("%v", raw["stackPolicy"]))
				maxStacks[id] = maxInt(0, int(toInt64Loose(firstNonNil(raw["maxStacks"], raw["stackLimit"]), 0)))
			}
			if raw == nil {
				continue
			}
			if mc := toInt64Loose(raw["maxCharges"], 0); mc > 0 {
				maxCharges[id] = mc
			}
			if parseBooleanLoose(raw["tickOnBattleStart"], false) {
				tickOnBattleStart[id] = true
			}
			if v, ok := raw["onExpire"].(string); ok {
				if n := strings.TrimSpace(strings.ToLower(v)); n != "" {
					onExpire[id] = n
				}
			}
			if kind := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", raw["kind"]))); kind != "" {
				buffKinds[id] = kind
			}
			if parseBooleanLoose(raw["dispellable"], false) {
				dispellable[id] = true
			}
		}
		s.buffMaxChargesCache = maxCharges
		s.buffTickOnBattleStartCache = tickOnBattleStart
		s.buffOnExpireCache = onExpire
		s.buffKindCache = buffKinds
		s.buffDispellableCache = dispellable
		s.buffDurationTurnsCache = durationTurns
		s.buffStackPolicyCache = stackPolicies
		s.buffMaxStacksCache = maxStacks
	})
	return s.buffRulesCache
}

func (s *Server) battleBuffKindByID(buffID string) string {
	if s == nil {
		return ""
	}
	s.battleParseBuffRules()
	return s.buffKindCache[strings.TrimSpace(strings.ToLower(buffID))]
}

func (s *Server) battleBuffDispellableByID(buffID string) bool {
	if s == nil {
		return false
	}
	s.battleParseBuffRules()
	return s.buffDispellableCache[strings.TrimSpace(strings.ToLower(buffID))]
}

func (s *Server) battleFindBuffRules(buffID string, trigger string) []battleBuffEffectRule {
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	trigger = battleNormalizeTrigger(trigger)
	if buffID == "" || trigger == "" {
		return nil
	}
	// Ensure index is built (no-op after first call).
	s.battleParseBuffRules()
	if s.buffRulesIndex == nil {
		return nil
	}
	return s.buffRulesIndex[buffID+"\x00"+trigger]
}

func (s *Server) battleCollectBuffRuleEntries(unit *battleUnitState, trigger string) []battleBuffRuleEntry {
	if s == nil || unit == nil {
		return nil
	}
	buffIDs := snapshotBuffIDs(unit)
	if len(buffIDs) == 0 {
		return nil
	}
	out := make([]battleBuffRuleEntry, 0, 8)
	for buffIndex, buffID := range buffIDs {
		rules := s.battleFindBuffRules(buffID, trigger)
		for _, rule := range rules {
			out = append(out, battleBuffRuleEntry{
				BuffIndex: buffIndex,
				BuffID:    buffID,
				Rule:      rule,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Rule.Priority != out[j].Rule.Priority {
			return out[i].Rule.Priority > out[j].Rule.Priority
		}
		if out[i].BuffIndex != out[j].BuffIndex {
			return out[i].BuffIndex < out[j].BuffIndex
		}
		if out[i].BuffID != out[j].BuffID {
			return out[i].BuffID < out[j].BuffID
		}
		return out[i].Rule.ID < out[j].Rule.ID
	})
	return out
}

func (s *Server) battleBuffDurationByIDTurns(buffID string) int64 {
	buffID = strings.TrimSpace(strings.ToLower(buffID))
	if buffID == "" {
		return 0
	}
	s.battleParseBuffRules()
	return s.buffDurationTurnsCache[buffID]
}

func battleNormalizeBuffStackPolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "stack", "stackcount", "stack_count":
		return "stack"
	default:
		return "refresh_duration"
	}
}

func (s *Server) battleBuffStackPolicyByID(buffID string) string {
	buffID = strings.TrimSpace(strings.ToLower(buffID))
	if buffID == "" {
		return "refresh_duration"
	}
	s.battleParseBuffRules()
	if policy := s.buffStackPolicyCache[buffID]; policy != "" {
		return policy
	}
	return "refresh_duration"
}

func (s *Server) battleBuffMaxStacksByID(buffID string) int {
	buffID = strings.TrimSpace(strings.ToLower(buffID))
	if buffID == "" {
		return 0
	}
	s.battleParseBuffRules()
	return s.buffMaxStacksCache[buffID]
}

func (s *Server) battleBuffDurationByIDMS(buffID string) int64 {
	turns := s.battleBuffDurationByIDTurns(buffID)
	if turns < 0 {
		return -1
	}
	return turns * s.battleTurnIntervalMS()
}

func parseBooleanLoose(raw any, fallback bool) bool {
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		if normalized == "true" || normalized == "1" {
			return true
		}
		if normalized == "false" || normalized == "0" {
			return false
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	}
	return fallback
}

func battleHasBuff(unit *battleUnitState, buffID string) bool {
	if unit == nil {
		return false
	}
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	if buffID == "" {
		return false
	}
	for _, row := range unit.Buffs {
		if strings.ToLower(strings.TrimSpace(row.BuffID)) == buffID {
			return true
		}
	}
	return false
}

func battleUnitHasAllStatuses(unit *battleUnitState, buffIDs []string) bool {
	for _, buffID := range buffIDs {
		if strings.TrimSpace(buffID) == "" {
			continue
		}
		if !battleHasBuff(unit, buffID) {
			return false
		}
	}
	return true
}

func battleUnitHasAnyStatus(unit *battleUnitState, buffIDs []string) bool {
	for _, buffID := range buffIDs {
		if strings.TrimSpace(buffID) != "" && battleHasBuff(unit, buffID) {
			return true
		}
	}
	return false
}

func battleFilterEffectStatusConditions(units []*battleUnitState, required, forbidden []string) []*battleUnitState {
	if len(required) == 0 && len(forbidden) == 0 {
		return units
	}
	out := make([]*battleUnitState, 0, len(units))
	for _, unit := range units {
		if unit == nil || !battleUnitHasAllStatuses(unit, required) || battleUnitHasAnyStatus(unit, forbidden) {
			continue
		}
		out = append(out, unit)
	}
	return out
}

func battleCountBuff(unit *battleUnitState, buffID string) int {
	if unit == nil {
		return 0
	}
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	if buffID == "" {
		return 0
	}
	count := 0
	for _, row := range unit.Buffs {
		if strings.ToLower(strings.TrimSpace(row.BuffID)) == buffID {
			count++
		}
	}
	return count
}

func battleRemoveBuffOneStack(unit *battleUnitState, buffID string) bool {
	if unit == nil {
		return false
	}
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	if buffID == "" {
		return false
	}
	for i, row := range unit.Buffs {
		if strings.ToLower(strings.TrimSpace(row.BuffID)) == buffID {
			unit.Buffs = append(unit.Buffs[:i], unit.Buffs[i+1:]...)
			return true
		}
	}
	return false
}

func battleRemoveBuff(unit *battleUnitState, buffID string) bool {
	if unit == nil {
		return false
	}
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	if buffID == "" {
		return false
	}
	next := make([]battleBuffState, 0, len(unit.Buffs))
	removed := false
	for _, row := range unit.Buffs {
		if strings.ToLower(strings.TrimSpace(row.BuffID)) == buffID {
			removed = true
			continue
		}
		next = append(next, row)
	}
	unit.Buffs = next
	if removed && buffID == battleSummonBindingBuffID && battleSummonBindingOnDispel(unit) {
		battleDespawnSummonedUnit(unit)
	}
	return removed
}

type battleBuffDispelResult struct {
	Changed          bool
	Removed          bool
	RemainTurns      int64
	Payload          map[string]any
	Triggered        int64
	ChargesRemaining int64
}

// battleDispelBuff peels one charge from a charged buff before removing it.
// Multi-charge buffs represent visible layers, while ordinary buffs keep the
// existing all-at-once dispel behavior.
func (s *Server) battleDispelBuff(unit *battleUnitState, buffID string) battleBuffDispelResult {
	if s == nil || unit == nil {
		return battleBuffDispelResult{}
	}
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	if buffID == "" {
		return battleBuffDispelResult{}
	}
	current := findBuffByID(unit, buffID)
	if current == nil {
		return battleBuffDispelResult{}
	}
	result := battleBuffDispelResult{
		RemainTurns: current.RemainTurns,
		Payload:     cloneAnyMap(current.Payload),
		Triggered:   current.Triggered,
	}
	maxCharges := s.battleBuffMaxCharges(buffID)
	chargesRemaining := battleBuffChargesRemaining(current, maxCharges)
	if maxCharges > 1 && chargesRemaining > 1 {
		current.Triggered++
		result.Changed = true
		result.Triggered = current.Triggered
		result.ChargesRemaining = chargesRemaining - 1
		return result
	}
	if !battleRemoveBuff(unit, buffID) {
		return battleBuffDispelResult{}
	}
	result.Changed = true
	result.Removed = true
	return result
}

func buildBattleBuffDispelAction(sourceID, targetID, buffID, reason string, result battleBuffDispelResult, meta map[string]any) map[string]any {
	actionMeta := cloneAnyMap(meta)
	if actionMeta == nil {
		actionMeta = map[string]any{}
	}
	actionMeta["chargesRemaining"] = result.ChargesRemaining
	if result.Removed {
		return map[string]any{
			"kind":     "state_detach",
			"targetId": targetID,
			"sourceId": sourceID,
			"buffId":   buffID,
			"reason":   reason,
			"meta":     actionMeta,
		}
	}
	return map[string]any{
		"kind":              "buff_charge",
		"targetId":          targetID,
		"sourceId":          sourceID,
		"buffId":            buffID,
		"buffDurationTurns": result.RemainTurns,
		"buffPayload":       cloneAnyMap(result.Payload),
		"buffTriggered":     result.Triggered,
		"reason":            reason,
		"meta":              actionMeta,
	}
}

// findBuffByID returns a pointer into unit.Buffs for the given buffID, or nil.
func findBuffByID(unit *battleUnitState, buffID string) *battleBuffState {
	if unit == nil || buffID == "" {
		return nil
	}
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	for i := range unit.Buffs {
		if strings.ToLower(strings.TrimSpace(unit.Buffs[i].BuffID)) == buffID {
			return &unit.Buffs[i]
		}
	}
	return nil
}

// snapshotBuffIDs returns the IDs of all current buffs on a unit (for safe iteration).
func snapshotBuffIDs(unit *battleUnitState) []string {
	if unit == nil || len(unit.Buffs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(unit.Buffs))
	for _, b := range unit.Buffs {
		if id := strings.ToLower(strings.TrimSpace(b.BuffID)); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// battleBuffMaxCharges reads the maxCharges field from the buff definition table.
func (s *Server) battleBuffMaxCharges(buffID string) int64 {
	buffID = strings.TrimSpace(strings.ToLower(buffID))
	if buffID == "" {
		return 0
	}
	s.battleParseBuffRules()
	return s.buffMaxChargesCache[buffID]
}

// battleBuffChargesRemaining computes how many charges are left.
func battleBuffChargesRemaining(buff *battleBuffState, maxCharges int64) int64 {
	if buff == nil || maxCharges <= 0 {
		return 0
	}
	if remaining := maxCharges - buff.Triggered; remaining > 0 {
		return remaining
	}
	return 0
}

func battleSelfHPPercent(unit *battleUnitState) float64 {
	if unit == nil || unit.MaxHP <= 0 {
		return 0
	}
	return clampFloat((unit.HP/unit.MaxHP)*100, 0, 100)
}

func battleConditionDamageTypeAllowed(condition map[string]any, incomingDamageType string) bool {
	damageType := normalizeBattleDamageType(incomingDamageType)
	if damageType == "" {
		return true
	}
	raw := firstNonNil(condition["damageTypes"], condition["damage_types"], condition["damageType"], condition["damage_type"])
	if raw == nil {
		return true
	}
	allowed := map[string]struct{}{}
	appendAllowed := func(value any) {
		key := normalizeBattleDamageType(fmt.Sprintf("%v", value))
		if key == "" {
			return
		}
		allowed[key] = struct{}{}
	}
	switch typed := raw.(type) {
	case []any:
		for _, entry := range typed {
			appendAllowed(entry)
		}
	case []string:
		for _, entry := range typed {
			appendAllowed(entry)
		}
	default:
		appendAllowed(raw)
	}
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[damageType]
	return ok
}

func battleEvaluateBuffConditionTyped(condType string, condition map[string]any, self *battleUnitState, source *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	return battleEvaluateBuffConditionTypedWithTarget(condType, condition, self, source, nil, incomingDamage, incomingDamageType, currentBuff, chargesRemaining)
}

func battleEvaluateBuffConditionTypedWithTarget(condType string, condition map[string]any, self *battleUnitState, source *battleUnitState, target *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	if len(condition) == 0 {
		return true
	}
	if !battleConditionDamageTypeAllowed(condition, incomingDamageType) {
		return false
	}
	if condType == "" {
		return true
	}
	return battleEvalConditionByTypeWithTarget(condType, condition, self, source, target, incomingDamage, incomingDamageType, currentBuff, chargesRemaining)
}

func battleEvaluateBuffCondition(condition map[string]any, self *battleUnitState, source *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	return battleEvaluateBuffConditionWithTarget(condition, self, source, nil, incomingDamage, incomingDamageType, currentBuff, chargesRemaining)
}

func battleEvaluateBuffConditionWithTarget(condition map[string]any, self *battleUnitState, source *battleUnitState, target *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	if len(condition) == 0 {
		return true
	}
	if !battleConditionDamageTypeAllowed(condition, incomingDamageType) {
		return false
	}
	condType := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(condition["type"], condition["kind"])))
	if condType == "" {
		return true
	}
	return battleEvalConditionByTypeWithTarget(condType, condition, self, source, target, incomingDamage, incomingDamageType, currentBuff, chargesRemaining)
}

func battleUnitHasAnyWeaponClass(unit *battleUnitState, raw any) bool {
	if unit == nil {
		return false
	}
	wanted := battleParseStringList(raw)
	if len(wanted) == 0 {
		return false
	}
	owned := make(map[string]struct{}, len(unit.WeaponClasses))
	for _, weaponClass := range unit.WeaponClasses {
		if normalized := normalizeWeaponClassID(weaponClass); normalized != "" {
			owned[normalized] = struct{}{}
		}
	}
	for _, weaponClass := range wanted {
		if _, ok := owned[normalizeWeaponClassID(weaponClass)]; ok {
			return true
		}
	}
	return false
}

type battleSkillCooldownSelector struct {
	IncludeBasic    bool
	SkillIDs        []string
	ExcludeSkillIDs []string
	SkillCategories []string
	SelectionMode   string
	MaxSkills       int
}

func battleNormalizeSkillCooldownSelectionMode(raw string) string {
	switch battleNormalizeTrigger(raw) {
	case "maxremaining":
		return "max_remaining"
	case "contextskill":
		return "context_skill"
	default:
		return "all"
	}
}

func battleParseSkillCooldownSelector(raw map[string]any) battleSkillCooldownSelector {
	return battleSkillCooldownSelector{
		IncludeBasic:    parseBooleanLoose(firstNonNil(raw["includeBasic"], raw["includeBasicAttack"], raw["include_basic"], raw["include_basic_attack"]), false),
		SkillIDs:        battleParseStringList(firstNonNil(raw["skillIds"], raw["skillIDs"], raw["skills"], raw["skill_ids"])),
		ExcludeSkillIDs: battleParseStringList(firstNonNil(raw["excludeSkillIds"], raw["excludeSkillIDs"], raw["excludeSkills"], raw["exclude_skill_ids"])),
		SkillCategories: battleParseStringList(firstNonNil(raw["skillCategories"], raw["skillTypes"], raw["skill_categories"], raw["skill_types"])),
		SelectionMode:   battleNormalizeSkillCooldownSelectionMode(fmt.Sprintf("%v", firstNonNil(raw["selectionMode"], raw["selection_mode"]))),
		MaxSkills:       maxInt(0, int(toInt64Loose(firstNonNil(raw["maxSkills"], raw["max_skills"]), 0))),
	}
}

func battleSkillCooldownCandidateIndices(unit *battleUnitState, selector battleSkillCooldownSelector, basicSkillID string, requireCooling bool) []int {
	if unit == nil || len(unit.Skills) == 0 {
		return nil
	}
	basicSkillID = strings.ToLower(strings.TrimSpace(basicSkillID))
	targetSkillSet := map[string]struct{}{}
	for _, id := range selector.SkillIDs {
		if key := strings.ToLower(strings.TrimSpace(id)); key != "" {
			targetSkillSet[key] = struct{}{}
		}
	}
	excludeSkillSet := map[string]struct{}{}
	for _, id := range selector.ExcludeSkillIDs {
		if key := strings.ToLower(strings.TrimSpace(id)); key != "" {
			excludeSkillSet[key] = struct{}{}
		}
	}
	categorySet := map[string]struct{}{}
	for _, category := range selector.SkillCategories {
		if key := strings.ToLower(strings.TrimSpace(category)); key != "" {
			categorySet[key] = struct{}{}
		}
	}
	candidates := make([]int, 0, len(unit.Skills))
	for idx := range unit.Skills {
		skillID := strings.ToLower(strings.TrimSpace(unit.Skills[idx].ID))
		if skillID == "" {
			continue
		}
		if !selector.IncludeBasic && (skillID == "basic_attack" || (basicSkillID != "" && skillID == basicSkillID)) {
			continue
		}
		if _, blocked := excludeSkillSet[skillID]; blocked {
			continue
		}
		if len(targetSkillSet) > 0 {
			if _, ok := targetSkillSet[skillID]; !ok {
				continue
			}
		}
		if len(categorySet) > 0 {
			category := strings.ToLower(strings.TrimSpace(unit.Skills[idx].Category))
			if _, ok := categorySet[category]; !ok {
				continue
			}
		}
		if requireCooling && unit.Skills[idx].CooldownRemain <= 0 {
			continue
		}
		candidates = append(candidates, idx)
	}
	if battleNormalizeSkillCooldownSelectionMode(selector.SelectionMode) == "max_remaining" {
		sort.SliceStable(candidates, func(i, j int) bool {
			return unit.Skills[candidates[i]].CooldownRemain > unit.Skills[candidates[j]].CooldownRemain
		})
	}
	if selector.MaxSkills > 0 && len(candidates) > selector.MaxSkills {
		candidates = candidates[:selector.MaxSkills]
	}
	return candidates
}

func battleCountCoolingSkills(unit *battleUnitState, selector battleSkillCooldownSelector) int {
	selector.SelectionMode = "all"
	selector.MaxSkills = 0
	return len(battleSkillCooldownCandidateIndices(unit, selector, "basic_attack", true))
}

func battleEvalConditionByType(condType string, condition map[string]any, self *battleUnitState, source *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	return battleEvalConditionByTypeWithTarget(condType, condition, self, source, nil, incomingDamage, incomingDamageType, currentBuff, chargesRemaining)
}

func battleEvalConditionByTypeWithTarget(condType string, condition map[string]any, self *battleUnitState, source *battleUnitState, target *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	value := toFloat64(condition["value"], 0)
	if target == nil {
		target = source
	}
	switch condType {
	case "selfweaponclass", "selfhasweaponclass":
		return battleUnitHasAnyWeaponClass(self, firstNonNil(condition["weaponClasses"], condition["weaponClass"], condition["weapon_classes"], condition["weapon_class"], condition["value"]))
	case "sourceweaponclass", "sourcehasweaponclass":
		return battleUnitHasAnyWeaponClass(source, firstNonNil(condition["weaponClasses"], condition["weaponClass"], condition["weapon_classes"], condition["weapon_class"], condition["value"]))
	case "selfhppercentlte":
		return battleSelfHPPercent(self) <= value
	case "selfhppercentgte":
		return battleSelfHPPercent(self) >= value
	case "sourcehppercentlte":
		return battleSelfHPPercent(source) <= value
	case "sourcehppercentgte":
		return battleSelfHPPercent(source) >= value
	case "targethppercentlte":
		return battleSelfHPPercent(target) <= value
	case "targethppercentgte":
		return battleSelfHPPercent(target) >= value
	case "selfmplt":
		if self == nil {
			return false
		}
		return self.MP < value
	case "selfmpgte":
		if self == nil {
			return false
		}
		return self.MP >= value
	case "selfskillcooldowncountgte":
		return battleCountCoolingSkills(self, battleParseSkillCooldownSelector(condition)) >= int(value)
	case "incomingdamagegte":
		return incomingDamage >= value
	case "incomingdamagelte":
		return incomingDamage <= value
	case "incomingdamagewouldbelethal":
		return self != nil && incomingDamage >= self.HP
	case "incominghealgte":
		return incomingDamage >= value
	case "incomingheallte":
		return incomingDamage <= value
	case "hasbuff":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return battleHasBuff(self, buffID)
	case "sourcehasbuff":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return battleHasBuff(source, buffID)
	case "targethasbuff":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return battleHasBuff(target, buffID)
	case "selfchargesremaininggt":
		return chargesRemaining > int64(value)
	case "selfchargesremaininggte":
		return chargesRemaining >= int64(value)
	case "selfchargesremaininglt":
		return chargesRemaining < int64(value)
	case "selfchargesremainingstle", "selfchargesremaininglte":
		return chargesRemaining <= int64(value)
	case "selfbuffcountlte":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return int64(battleCountBuff(self, buffID)) <= int64(value)
	case "selfbuffcountlt":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return int64(battleCountBuff(self, buffID)) < int64(value)
	case "selfbuffcountgte":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return int64(battleCountBuff(self, buffID)) >= int64(value)
	case "selfbuffcountgt":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return int64(battleCountBuff(self, buffID)) > int64(value)
	case "selfbuffcounteq":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return int64(battleCountBuff(self, buffID)) == int64(value)
	case "targetbuffcountgte":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"]))
		return int64(battleCountBuff(target, buffID)) >= int64(value)
	case "waveintervaleq":
		divisor := int64(toFloat64(condition["divisor"], 1))
		if divisor <= 0 {
			divisor = 1
		}
		return int64(incomingDamage)%divisor == int64(value)
	default:
		return true
	}
}

func battleRuleConditionsPassed(rule battleBuffEffectRule, self *battleUnitState, source *battleUnitState, incomingDamage float64, incomingDamageType string, currentBuff *battleBuffState, chargesRemaining int64) bool {
	if len(rule.Conditions) == 0 {
		return true
	}
	for i, condition := range rule.Conditions {
		condType := ""
		if i < len(rule.ConditionTypes) {
			condType = rule.ConditionTypes[i]
		}
		if !battleEvaluateBuffConditionTyped(condType, condition, self, source, incomingDamage, incomingDamageType, currentBuff, chargesRemaining) {
			return false
		}
	}
	return true
}

func battleBuffRuleHasConditionType(rule battleBuffEffectRule, conditionType string) bool {
	wanted := battleNormalizeTrigger(conditionType)
	if wanted == "" {
		return false
	}
	for index, condition := range rule.Conditions {
		actual := ""
		if index < len(rule.ConditionTypes) {
			actual = rule.ConditionTypes[index]
		}
		if actual == "" {
			actual = battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(condition["type"], condition["kind"])))
		}
		if actual == wanted {
			return true
		}
	}
	return false
}

func (s *Server) battleUnitHasBuffEffect(unit *battleUnitState, trigger string, effectKind string, source *battleUnitState) bool {
	if s == nil || unit == nil {
		return false
	}
	effectKind = battleNormalizeTrigger(effectKind)
	if effectKind == "" {
		return false
	}
	for i := range unit.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(unit.Buffs[i].BuffID))
		if buffID == "" {
			continue
		}
		rules := s.battleFindBuffRules(buffID, trigger)
		if len(rules) == 0 {
			continue
		}
		chargesRemaining := battleBuffChargesRemaining(&unit.Buffs[i], s.buffMaxChargesCache[buffID])
		for _, rule := range rules {
			if !battleRuleConditionsPassed(rule, unit, source, 0, "", &unit.Buffs[i], chargesRemaining) {
				continue
			}
			for ei, effect := range rule.Effects {
				var kind string
				if ei < len(rule.EffectKinds) {
					kind = rule.EffectKinds[ei]
				} else {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
					if kind == "" {
						kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
					}
				}
				if kind == effectKind {
					return true
				}
			}
		}
	}
	return false
}

func (s *Server) battleUnitHasWhileActiveEffect(unit *battleUnitState, effectKind string, source *battleUnitState) bool {
	return s.battleUnitHasBuffEffect(unit, "while_active", effectKind, source)
}

// battleUnitCheckTurnSuppressions checks three turn-action suppression effects in one buff pass.
func (s *Server) battleUnitCheckTurnSuppressions(unit *battleUnitState) (actionSuppressed, skillSuppressed, basicSuppressed bool) {
	if s == nil || unit == nil || len(unit.Buffs) == 0 {
		return
	}
	s.battleParseBuffRules()
	const trigger = "whileactive"
	for i := range unit.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(unit.Buffs[i].BuffID))
		if buffID == "" {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(buffID)
		chargesRemaining := battleBuffChargesRemaining(&unit.Buffs[i], maxCharges)
		for _, rule := range s.buffRulesIndex[buffID+"\x00"+trigger] {
			if !battleRuleConditionsPassed(rule, unit, nil, 0, "", &unit.Buffs[i], chargesRemaining) {
				continue
			}
			for _, effect := range rule.Effects {
				kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
				if kind == "" {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
				}
				switch kind {
				case battleNormalizeTrigger(battleEffectTurnActionSuppressed):
					actionSuppressed = true
				case battleNormalizeTrigger(battleEffectSkillActionSuppressed):
					skillSuppressed = true
				case battleNormalizeTrigger(battleEffectBasicAttackSuppressed):
					basicSuppressed = true
				}
			}
			if actionSuppressed && skillSuppressed && basicSuppressed {
				return
			}
		}
	}
	return
}

func (s *Server) battleRecomputeUnitPosition(unit *battleUnitState) bool {
	if s == nil || unit == nil {
		return false
	}
	baseSide := battleResolveUnitBaseSide(unit)
	teamSide := battleUnitTeamSide(unit)
	if teamSide == "" {
		teamSide = baseSide
	}
	basePosition := battleResolveUnitBasePosition(unit)
	if mappedPosition, found := s.battleResolvePositionMap(unit, basePosition); found {
		return s.battleRecomputeMappedUnitPosition(unit, baseSide, teamSide, basePosition, mappedPosition)
	}
	nextSide := baseSide
	invertPerspective := false
	nextPosition := basePosition
	overrideFound := false
	for i := range unit.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(unit.Buffs[i].BuffID))
		if buffID == "" {
			continue
		}
		chargesRemaining := battleBuffChargesRemaining(&unit.Buffs[i], s.buffMaxChargesCache[buffID])
		for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
			if !battleRuleConditionsPassed(rule, unit, nil, 0, "", &unit.Buffs[i], chargesRemaining) {
				continue
			}
			for ei, effect := range rule.Effects {
				var kind string
				if ei < len(rule.EffectKinds) {
					kind = rule.EffectKinds[ei]
				} else {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
					if kind == "" {
						kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
					}
				}
				switch kind {
				case battleEffectPositionOverrideKey:
					position := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["position"], effect["targetPosition"]))))
					if position != "" {
						nextPosition = position
						overrideFound = true
					}
					continue
				case battleEffectSideOverrideKey:
					side := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["side"], effect["targetSide"]))))
					if side != "" {
						nextSide = side
					}
					continue
				case battleEffectInvertRelationViewKey:
					invertPerspective = true
					continue
				}
				if overrideFound || kind != battleEffectAdvancePositionKey {
					continue
				}
				steps := toInt64Loose(effect["steps"], 1)
				if steps <= 0 {
					continue
				}
				for step := int64(0); step < steps; step++ {
					nextPosition = battleAdvancePositionTowardsVanguard(nextPosition)
				}
			}
		}
	}
	before := normalizeBattleTargetPosition(unit.Position)
	beforeSide := normalizeBattleSide(unit.Side)
	beforePerspective := normalizeBattleSide(unit.PerspectiveSide)
	unit.BasePosition = basePosition
	unit.BaseSide = baseSide
	unit.OriginSide = teamSide
	unit.Position = nextPosition
	unit.Side = nextSide
	unit.PerspectiveSide = battleResolvePerspectiveSide(teamSide, nextSide, invertPerspective)
	return before != nextPosition || beforeSide != unit.Side || beforePerspective != unit.PerspectiveSide
}

func (s *Server) battleResolvePositionMap(unit *battleUnitState, basePosition string) (string, bool) {
	if s == nil || unit == nil {
		return "", false
	}
	found := false
	selectedPriority := 0
	selectedRuleID := ""
	selectedPosition := ""
	for buffIndex := range unit.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(unit.Buffs[buffIndex].BuffID))
		if buffID == "" {
			continue
		}
		chargesRemaining := battleBuffChargesRemaining(&unit.Buffs[buffIndex], s.buffMaxChargesCache[buffID])
		for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
			if !battleRuleConditionsPassed(rule, unit, nil, 0, "", &unit.Buffs[buffIndex], chargesRemaining) {
				continue
			}
			for effectIndex, effect := range rule.Effects {
				kind := ""
				if effectIndex < len(rule.EffectKinds) {
					kind = rule.EffectKinds[effectIndex]
				}
				if kind != battleEffectPositionMapKey {
					continue
				}
				mapping := normalizeAnyMap(effect["mapping"])
				mapped := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", mapping[basePosition])))
				if mapped == "" {
					continue
				}
				priority := int(toInt64Loose(effect["priority"], int64(rule.Priority)))
				if found && (priority < selectedPriority || priority == selectedPriority && rule.ID >= selectedRuleID) {
					continue
				}
				found = true
				selectedPriority = priority
				selectedRuleID = rule.ID
				selectedPosition = mapped
			}
		}
	}
	return selectedPosition, found
}

func (s *Server) battleRecomputeMappedUnitPosition(
	unit *battleUnitState,
	baseSide string,
	teamSide string,
	basePosition string,
	mappedPosition string,
) bool {
	nextSide := baseSide
	invertPerspective := false
	advanceSteps := int64(0)
	overridePosition := ""
	for buffIndex := range unit.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(unit.Buffs[buffIndex].BuffID))
		if buffID == "" {
			continue
		}
		chargesRemaining := battleBuffChargesRemaining(&unit.Buffs[buffIndex], s.buffMaxChargesCache[buffID])
		for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
			if !battleRuleConditionsPassed(rule, unit, nil, 0, "", &unit.Buffs[buffIndex], chargesRemaining) {
				continue
			}
			for effectIndex, effect := range rule.Effects {
				kind := ""
				if effectIndex < len(rule.EffectKinds) {
					kind = rule.EffectKinds[effectIndex]
				} else {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
				}
				switch kind {
				case battleEffectPositionOverrideKey:
					if position := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["position"], effect["targetPosition"])))); position != "" {
						overridePosition = position
					}
				case battleEffectAdvancePositionKey:
					advanceSteps += maxInt64(0, toInt64Loose(effect["steps"], 1))
				case battleEffectSideOverrideKey:
					if side := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["side"], effect["targetSide"])))); side != "" {
						nextSide = side
					}
				case battleEffectInvertRelationViewKey:
					invertPerspective = true
				}
			}
		}
	}
	nextPosition := mappedPosition
	for step := int64(0); step < advanceSteps; step++ {
		nextPosition = battleAdvancePositionTowardsVanguard(nextPosition)
	}
	if overridePosition != "" {
		nextPosition = overridePosition
	}
	before := normalizeBattleTargetPosition(unit.Position)
	beforeSide := normalizeBattleSide(unit.Side)
	beforePerspective := normalizeBattleSide(unit.PerspectiveSide)
	unit.BasePosition = basePosition
	unit.BaseSide = baseSide
	unit.OriginSide = teamSide
	unit.Position = nextPosition
	unit.Side = nextSide
	unit.PerspectiveSide = battleResolvePerspectiveSide(teamSide, nextSide, invertPerspective)
	return before != nextPosition || beforeSide != unit.Side || beforePerspective != unit.PerspectiveSide
}

func battleResolveUnitBaseSide(unit *battleUnitState) string {
	if unit == nil {
		return "player"
	}
	for _, candidate := range []string{unit.BaseSide, unit.Side, battleUnitTeamSide(unit)} {
		if side := normalizeBattleSide(candidate); side != "" {
			return side
		}
	}
	return "player"
}

func battleResolveUnitBasePosition(unit *battleUnitState) string {
	if unit == nil {
		return "vanguard"
	}
	for _, candidate := range []string{unit.BasePosition, unit.Position} {
		if position := normalizeBattleTargetPosition(candidate); position != "" {
			return position
		}
	}
	return "vanguard"
}

func battleResolvePerspectiveSide(teamSide, currentSide string, invert bool) string {
	if !invert {
		return normalizeBattleSide(teamSide)
	}
	return battleOppositeSide(currentSide)
}

func battleAdvancePositionTowardsVanguard(position string) string {
	switch normalizeBattleTargetPosition(position) {
	case "rearguard":
		return "midguard"
	case "midguard":
		return "vanguard"
	default:
		return "vanguard"
	}
}

func (s *Server) battleFindDamageRedirectToSource(state *battleRuntimeState, target *battleUnitState, incomingDamage float64) (*battleUnitState, float64, string, string) {
	if s == nil || state == nil || target == nil || incomingDamage <= 0 {
		return nil, 0, "", ""
	}
	for i := range target.Buffs {
		buff := &target.Buffs[i]
		buffID := strings.TrimSpace(strings.ToLower(buff.BuffID))
		if buffID == "" || strings.TrimSpace(buff.SourceID) == "" || strings.TrimSpace(buff.SourceID) == strings.TrimSpace(target.UnitID) {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(buffID)
		chargesRemaining := battleBuffChargesRemaining(buff, maxCharges)
		for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
			if !battleRuleConditionsPassed(rule, target, nil, incomingDamage, "", buff, chargesRemaining) {
				continue
			}
			for _, effect := range rule.Effects {
				kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
				if kind == "" {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
				}
				if kind != battleNormalizeTrigger(battleEffectDamageRedirectSource) {
					continue
				}
				source := battleUnitByID(state, buff.SourceID)
				if source == nil || !source.Alive || source.HP <= 0 {
					continue
				}
				percent := toFloat64(effect["percent"], toFloat64(effect["value"], 1))
				percent = clampFloat(percent, 0, 1)
				if percent <= 0 {
					continue
				}
				return source, maxFloat(0, incomingDamage*percent), buff.BuffID, rule.ID
			}
		}
	}
	return nil, 0, "", ""
}

func cloneNestedFloatMap(source map[string]map[string]float64) map[string]map[string]float64 {
	out := make(map[string]map[string]float64, len(source))
	for key, child := range source {
		nextChild := make(map[string]float64, len(child))
		for childKey, value := range child {
			nextChild[childKey] = value
		}
		out[key] = nextChild
	}
	return out
}

func (s *Server) battleRecomputeUnitStats(unit *battleUnitState) {
	if unit == nil {
		return
	}
	if unit.BaseStats == nil {
		baseStats := cloneFloatMap(unit.Stats)
		// Recover a clean baseline when state is resumed with previously applied
		// modifiers already folded into Stats.
		for attr, delta := range unit.StatModifiers {
			normalizedAttr := strings.TrimSpace(strings.ToLower(attr))
			if normalizedAttr == "" || delta == 0 {
				continue
			}
			baseStats[normalizedAttr] = roundTo4(baseStats[normalizedAttr] - delta)
			if baseStats[normalizedAttr] == 0 {
				delete(baseStats, normalizedAttr)
			}
		}
		unit.BaseStats = baseStats
	}
	unit.SkipTurnOrder = false
	unit.TurnIneligible = false
	unit.HasSyncedSummonTrigger = false
	unit.SyncedSummonTemplateIDs = nil
	baseFinal := cloneFloatMap(unit.BaseStats)
	rawDelta := map[string]float64{}
	sourceByAttr := map[string]map[string]float64{}
	for i := range unit.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(unit.Buffs[i].BuffID))
		if buffID == "" {
			continue
		}
		chargesRemaining := battleBuffChargesRemaining(&unit.Buffs[i], s.buffMaxChargesCache[buffID])
		for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
			if !battleRuleConditionsPassed(rule, unit, nil, 0, "", &unit.Buffs[i], chargesRemaining) {
				continue
			}
			for ei, effect := range rule.Effects {
				var kind string
				if ei < len(rule.EffectKinds) {
					kind = rule.EffectKinds[ei]
				} else {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
					if kind == "" {
						kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
					}
				}
				switch kind {
				case battleEffectSkipTurnOrder:
					unit.SkipTurnOrder = true
					continue
				case battleEffectTurnIneligible:
					unit.TurnIneligible = true
					unit.SkipTurnOrder = true
					continue
				case battleEffectTriggerSyncedSummonTurn:
					templateIDs := battleParseStringList(firstNonNil(effect["includeSummonTemplateIds"], effect["include_summon_template_ids"]))
					hadTrigger := unit.HasSyncedSummonTrigger
					unit.HasSyncedSummonTrigger = true
					if len(templateIDs) == 0 {
						unit.SyncedSummonTemplateIDs = nil
						continue
					}
					if !hadTrigger {
						unit.SyncedSummonTemplateIDs = map[string]struct{}{}
					}
					if unit.SyncedSummonTemplateIDs != nil {
						for _, templateID := range templateIDs {
							normalized := strings.ToLower(strings.TrimSpace(templateID))
							if normalized != "" {
								unit.SyncedSummonTemplateIDs[normalized] = struct{}{}
							}
						}
					}
					continue
				}
				attr := strings.TrimSpace(fmt.Sprintf("%v", effect["attr"]))
				if attr == "" {
					attr = strings.TrimSpace(fmt.Sprintf("%v", effect["stat"]))
				}
				if attr == "" {
					continue
				}
				attr = strings.ToLower(attr)
				delta := 0.0
				switch kind {
				case "addstat":
					delta = battleBuffEffectScalar(effect, &unit.Buffs[i], effect["value"])
				case "addstatpercent":
					percent := battleBuffEffectScalar(effect, &unit.Buffs[i], firstNonNil(effect["percent"], effect["value"]))
					delta = baseFinal[attr] * percent
				default:
					continue
				}
				if delta == 0 {
					continue
				}
				rawDelta[attr] = roundTo4(rawDelta[attr] + delta)
				if sourceByAttr[attr] == nil {
					sourceByAttr[attr] = map[string]float64{}
				}
				sourceByAttr[attr][rule.ID] = roundTo4(sourceByAttr[attr][rule.ID] + delta)
			}
		}
	}

	// Reuse rawDelta as deltaFinal (rawDelta not referenced after this point).
	deltaFinal := rawDelta
	if strings.TrimSpace(unit.StatMode) != battleStatModeDirect {
		for _, formula := range s.deps.Config.GlobalSettings.DerivedStats.Formulas {
			targetAttr := strings.TrimSpace(strings.ToLower(formula.TargetAttr))
			if targetAttr == "" {
				continue
			}
			if !formula.IncludeBase {
				deltaFinal[targetAttr] = 0
			}
			total := 0.0
			for _, term := range formula.Terms {
				attrKey := strings.TrimSpace(strings.ToLower(term.AttrKey))
				if attrKey == "" {
					continue
				}
				total += deltaFinal[attrKey] * term.Coefficient
			}
			if total != 0 {
				if sourceByAttr[targetAttr] == nil {
					sourceByAttr[targetAttr] = map[string]float64{}
				}
				sourceByAttr[targetAttr]["__derived__"] = roundTo4(sourceByAttr[targetAttr]["__derived__"] + total)
			}
			deltaFinal[targetAttr] = roundTo4(deltaFinal[targetAttr] + total)
		}
	}

	// Apply deltas into baseFinal in-place to avoid a separate finalStats clone.
	for attr, delta := range deltaFinal {
		baseFinal[attr] = roundTo4(baseFinal[attr] + delta)
	}

	unit.StatModifiers = deltaFinal
	unit.StatSources = sourceByAttr
	unit.Stats = baseFinal
	prevMaxHP := maxFloat(0, unit.MaxHP)
	prevMaxMP := maxFloat(0, unit.MaxMP)
	prevHP := maxFloat(0, unit.HP)
	prevMP := maxFloat(0, unit.MP)
	unit.MaxHP = battleMaxHPFromStat(baseFinal["hp"])
	unit.MaxMP = maxFloat(0, baseFinal["mp"])
	unit.HP = clampFloat(scaleResourceByMaxChange(prevHP, prevMaxHP, unit.MaxHP), 0, unit.MaxHP)
	unit.MP = clampFloat(scaleResourceByMaxChange(prevMP, prevMaxMP, unit.MaxMP), 0, unit.MaxMP)
	unit.Alive = unit.HP > 0
	s.battleRecomputeUnitPosition(unit)
}

func scaleResourceByMaxChange(current, prevMax, nextMax float64) float64 {
	if nextMax <= 0 {
		return 0
	}
	if current <= 0 {
		return 0
	}
	if prevMax <= 0 {
		return current
	}
	return nextMax * (current / prevMax)
}

func (s *Server) battleApplyBeforeTakeDamageBuffHooks(
	state *battleRuntimeState,
	now time.Time,
	damageSource *battleUnitState,
	target *battleUnitState,
	incomingDamage float64,
	incomingDamageType string,
) (float64, []map[string]any) {
	damage, logs := s.battleApplyBeforeTakeDamageBuffHooksByPhase(
		state, now, damageSource, target, incomingDamage, incomingDamage, incomingDamageType, false,
	)
	projectedDamage := s.battleProjectedDamageAfterRedirect(state, target, damage)
	damage, lethalLogs := s.battleApplyBeforeTakeDamageBuffHooksByPhase(
		state, now, damageSource, target, damage, projectedDamage, incomingDamageType, true,
	)
	return damage, append(logs, lethalLogs...)
}

func (s *Server) battleApplyBeforeTakeDamageBuffHooksByPhase(
	state *battleRuntimeState,
	now time.Time,
	damageSource *battleUnitState,
	target *battleUnitState,
	incomingDamage float64,
	conditionDamage float64,
	incomingDamageType string,
	lethalPhase bool,
) (float64, []map[string]any) {
	if target == nil {
		return incomingDamage, nil
	}
	damage := incomingDamage
	logs := make([]map[string]any, 0, 2)
	entries := append(
		s.battleCollectBuffRuleEntries(target, "on_before_take_damage"),
		s.battleCollectBuffRuleEntries(target, "while_active")...,
	)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Rule.Priority != entries[j].Rule.Priority {
			return entries[i].Rule.Priority > entries[j].Rule.Priority
		}
		return entries[i].Rule.ID < entries[j].Rule.ID
	})
	for _, entry := range entries {
		isLethalRule := battleBuffRuleHasConditionType(entry.Rule, battleBuffConditionIncomingDamageWouldBeLethal)
		if isLethalRule != lethalPhase {
			continue
		}
		// Re-look up currentBuff each rule in case an earlier consume_charge removed it.
		currentBuff := findBuffByID(target, entry.BuffID)
		if currentBuff == nil {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(entry.BuffID)
		chargesRemaining := battleBuffChargesRemaining(currentBuff, maxCharges)
		damageForConditions := damage
		if lethalPhase {
			damageForConditions = conditionDamage
		}
		if !battleRuleConditionsPassed(entry.Rule, target, damageSource, damageForConditions, incomingDamageType, currentBuff, chargesRemaining) {
			continue
		}
		for _, effect := range entry.Rule.Effects {
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
			if kind == "" {
				kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
			}
			if entry.Rule.Trigger == "whileactive" && kind != "reducedamagepercent" {
				continue
			}
			before := damage
			switch kind {
			case "reducedamagepercent":
				percent := clampFloat(battleBuffEffectScalar(effect, currentBuff, firstNonNil(effect["value"], effect["percent"])), 0, 1)
				damage = maxFloat(0, damage*(1-percent))
				if lethalPhase {
					conditionDamage = maxFloat(0, conditionDamage*(1-percent))
				}
				logs = append(logs, map[string]any{
					"trigger":  "on_before_take_damage",
					"ruleId":   entry.Rule.ID,
					"buffId":   currentBuff.BuffID,
					"sourceId": target.UnitID,
					"targetId": target.UnitID,
					"before":   roundTo4(before),
					"after":    roundTo4(damage),
					"ts":       now.UnixMilli(),
				})
			case "setdamagemultiplier":
				mul := clampFloat(battleBuffEffectScalar(effect, currentBuff, effect["value"]), 0, 10)
				damage = maxFloat(0, damage*mul)
				if lethalPhase {
					conditionDamage = maxFloat(0, conditionDamage*mul)
				}
				logs = append(logs, map[string]any{
					"trigger":  "on_before_take_damage",
					"ruleId":   entry.Rule.ID,
					"buffId":   currentBuff.BuffID,
					"sourceId": target.UnitID,
					"targetId": target.UnitID,
					"before":   roundTo4(before),
					"after":    roundTo4(damage),
					"ts":       now.UnixMilli(),
				})
			case "setdamagevalue":
				damage = maxFloat(0, toFloat64(effect["value"], damage))
				if lethalPhase {
					conditionDamage = math.Min(conditionDamage, damage)
				}
				logs = append(logs, map[string]any{
					"trigger":  "on_before_take_damage",
					"ruleId":   entry.Rule.ID,
					"buffId":   currentBuff.BuffID,
					"sourceId": target.UnitID,
					"targetId": target.UnitID,
					"before":   roundTo4(before),
					"after":    roundTo4(damage),
					"ts":       now.UnixMilli(),
				})
			case "heal":
				value := battleBuffEffectScalar(effect, currentBuff, effect["value"])
				if formulaRaw := firstNonNil(effect["amountFormula"], effect["amount_formula"], effect["healFormula"], effect["heal_formula"]); formulaRaw != nil {
					value += battleEvaluateAmountFormula(target, 1, parseBattleSkillAmountFormula(formulaRaw, nil))
				}
				value = maxFloat(0, value)
				value, _ = s.battleApplyOutgoingBuffModifiers(target, "heal", value)
				beforeHP := target.HP
				target.HP = clampFloat(target.HP+value, 0, target.MaxHP)
				if target.HP > 0 {
					target.Alive = true
				}
				metricSourceID, metricSource := battleCombatTrialBuffMetricSource(state, currentBuff, target)
				battleCombatTrialRecordHeal(state, metricSourceID, metricSource, target, target.HP-beforeHP)
				actionSourceID := target.UnitID
				if battleCombatTrialActive(state) && metricSourceID != "" {
					actionSourceID = metricSourceID
				}
				logs = append(logs, map[string]any{
					"trigger":  "on_before_take_damage",
					"kind":     "heal",
					"ruleId":   entry.Rule.ID,
					"buffId":   currentBuff.BuffID,
					"sourceId": actionSourceID,
					"targetId": target.UnitID,
					"amount":   roundTo4(target.HP - beforeHP),
					"beforeHp": roundTo4(beforeHP),
					"afterHp":  roundTo4(target.HP),
					"ts":       now.UnixMilli(),
				})
			case "addbuff":
				newBuffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
				if newBuffID == "" {
					continue
				}
				durationTurns := toInt64Loose(effect["durationTurns"], 0)
				if durationTurns == 0 {
					if durationMS := toInt64Loose(effect["durationMs"], 0); durationMS != 0 {
						durationTurns = battleDurationTurnsFromLegacyMS(durationMS, s.battleTurnIntervalMS())
					}
				}
				if durationTurns == 0 {
					durationTurns = s.battleBuffDurationByIDTurns(newBuffID)
				}
				buffTarget := target
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target"])))
				if targetMode == "source" {
					if damageSource == nil || !damageSource.Alive {
						continue
					}
					buffTarget = damageSource
				} else {
					targetMode = "self"
				}
				payload := normalizeAnyMap(effect["payload"])
				isNewAttach := battleApplyOrRefreshBuffForTurn(
					buffTarget,
					newBuffID,
					durationTurns,
					payload,
					target.UnitID,
					s.battleBuffStackPolicyByID(newBuffID),
					s.battleBuffMaxStacksByID(newBuffID),
					battleBuffApplicationTurnSerial(state, "on_before_take_damage"),
				)
				s.battleRecomputeUnitStats(buffTarget)
				logs = append(logs, map[string]any{
					"trigger":           "on_before_take_damage",
					"kind":              "buff",
					"ruleId":            entry.Rule.ID,
					"buffId":            newBuffID,
					"sourceId":          target.UnitID,
					"targetId":          buffTarget.UnitID,
					"buffDurationTurns": durationTurns,
					"newAttach":         isNewAttach,
					"targetMode":        targetMode,
					"ts":                now.UnixMilli(),
				})
			case "consumecharge":
				targetBuffID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"])))
				if targetBuffID == "" || targetBuffID == "<nil>" {
					targetBuffID = entry.BuffID
				}
				targetBuff := findBuffByID(target, targetBuffID)
				if targetBuff == nil {
					continue
				}
				targetMaxCharges := s.battleBuffMaxCharges(targetBuffID)
				if targetMaxCharges <= 0 {
					continue
				}
				targetBuff.Triggered++
				if targetBuff.Triggered >= targetMaxCharges {
					battleRemoveBuff(target, targetBuffID)
					logs = append(logs, map[string]any{
						"trigger":  "on_before_take_damage",
						"kind":     "state_detach",
						"buffId":   targetBuffID,
						"targetId": target.UnitID,
						"reason":   "charges_exhausted",
						"ts":       now.UnixMilli(),
					})
				}
			}
		}
	}
	return damage, logs
}

func (s *Server) battleProjectedDamageAfterRedirect(state *battleRuntimeState, target *battleUnitState, incomingDamage float64) float64 {
	damage := maxFloat(0, incomingDamage)
	if damage <= 0 {
		return 0
	}
	_, redirected, _, _ := s.battleFindDamageRedirectToSource(state, target, damage)
	return maxFloat(0, damage-math.Min(damage, redirected))
}

func (s *Server) battleApplyBeforeDealDamageBuffHooks(
	state *battleRuntimeState,
	now time.Time,
	damageSource *battleUnitState,
	target *battleUnitState,
	incomingDamage float64,
	incomingDamageType string,
) (float64, []map[string]any, []map[string]any) {
	if damageSource == nil {
		return incomingDamage, nil, nil
	}
	damage := incomingDamage
	logs := make([]map[string]any, 0, 2)
	sources := make([]map[string]any, 0, 2)
	for _, entry := range s.battleCollectBuffRuleEntries(damageSource, "on_before_deal_damage") {
		currentBuff := findBuffByID(damageSource, entry.BuffID)
		if currentBuff == nil {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(entry.BuffID)
		chargesRemaining := battleBuffChargesRemaining(currentBuff, maxCharges)
		if !battleRuleConditionsPassed(entry.Rule, damageSource, target, damage, incomingDamageType, currentBuff, chargesRemaining) {
			continue
		}
		for _, effect := range entry.Rule.Effects {
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
			if kind == "" {
				kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
			}
			before := damage
			switch kind {
			case "adddamagepercent":
				percent := battleBuffEffectScalar(effect, currentBuff, firstNonNil(effect["value"], effect["percent"]))
				damage = damage * (1 + percent)
				sources = append(sources, map[string]any{
					"kind":     "buff",
					"id":       currentBuff.BuffID,
					"ruleId":   entry.Rule.ID,
					"sourceId": damageSource.UnitID,
					"effect":   kind,
				})
			case "adddamageflat", "adddamage":
				damage = damage + battleBuffEffectScalar(effect, currentBuff, effect["value"])
				sources = append(sources, map[string]any{
					"kind":     "buff",
					"id":       currentBuff.BuffID,
					"ruleId":   entry.Rule.ID,
					"sourceId": damageSource.UnitID,
					"effect":   kind,
				})
			case "setdamagemultiplier":
				mul := clampFloat(battleBuffEffectScalar(effect, currentBuff, effect["value"]), 0, 10)
				damage = damage * mul
				sources = append(sources, map[string]any{
					"kind":     "buff",
					"id":       currentBuff.BuffID,
					"ruleId":   entry.Rule.ID,
					"sourceId": damageSource.UnitID,
					"effect":   kind,
				})
			case "setdamagevalue":
				damage = maxFloat(0, toFloat64(effect["value"], damage))
				sources = append(sources, map[string]any{
					"kind":     "buff",
					"id":       currentBuff.BuffID,
					"ruleId":   entry.Rule.ID,
					"sourceId": damageSource.UnitID,
					"effect":   kind,
				})
			case "consumecharge":
				targetBuffID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"])))
				if targetBuffID == "" || targetBuffID == "<nil>" {
					targetBuffID = entry.BuffID
				}
				targetBuff := findBuffByID(damageSource, targetBuffID)
				if targetBuff == nil {
					continue
				}
				targetMaxCharges := s.battleBuffMaxCharges(targetBuffID)
				if targetMaxCharges <= 0 {
					continue
				}
				targetBuff.Triggered++
				if targetBuff.Triggered >= targetMaxCharges {
					battleRemoveBuff(damageSource, targetBuffID)
					logs = append(logs, map[string]any{
						"trigger":  "on_before_deal_damage",
						"kind":     "state_detach",
						"buffId":   targetBuffID,
						"sourceId": damageSource.UnitID,
						"reason":   "charges_exhausted",
						"ts":       now.UnixMilli(),
					})
				}
				continue
			default:
				continue
			}
			damage = maxFloat(0, damage)
			logs = append(logs, map[string]any{
				"trigger":  "on_before_deal_damage",
				"ruleId":   entry.Rule.ID,
				"buffId":   currentBuff.BuffID,
				"sourceId": damageSource.UnitID,
				"targetId": damageSource.UnitID,
				"before":   roundTo4(before),
				"after":    roundTo4(damage),
				"ts":       now.UnixMilli(),
			})
		}
	}
	return damage, logs, sources
}

func (s *Server) battleApplyBeforeHealBuffHooks(
	state *battleRuntimeState,
	now time.Time,
	healSource *battleUnitState,
	target *battleUnitState,
	incomingHeal float64,
) (float64, []map[string]any) {
	if target == nil {
		return incomingHeal, nil
	}
	heal := incomingHeal
	logs := make([]map[string]any, 0, 2)
	for _, entry := range s.battleCollectBuffRuleEntries(target, "on_before_heal") {
		currentBuff := findBuffByID(target, entry.BuffID)
		if currentBuff == nil {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(entry.BuffID)
		chargesRemaining := battleBuffChargesRemaining(currentBuff, maxCharges)
		if !battleRuleConditionsPassed(entry.Rule, target, healSource, heal, "", currentBuff, chargesRemaining) {
			continue
		}
		for _, effect := range entry.Rule.Effects {
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
			if kind == "" {
				kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
			}
			before := heal
			switch kind {
			case "sethealvalue":
				heal = maxFloat(0, toFloat64(effect["value"], heal))
				logs = append(logs, map[string]any{
					"trigger":  "on_before_heal",
					"ruleId":   entry.Rule.ID,
					"buffId":   currentBuff.BuffID,
					"sourceId": target.UnitID,
					"targetId": target.UnitID,
					"before":   roundTo4(before),
					"after":    roundTo4(heal),
					"ts":       now.UnixMilli(),
				})
			case "consumecharge":
				targetBuffID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"])))
				if targetBuffID == "" || targetBuffID == "<nil>" {
					targetBuffID = entry.BuffID
				}
				targetBuff := findBuffByID(target, targetBuffID)
				if targetBuff == nil {
					continue
				}
				targetMaxCharges := s.battleBuffMaxCharges(targetBuffID)
				if targetMaxCharges <= 0 {
					continue
				}
				targetBuff.Triggered++
				if targetBuff.Triggered >= targetMaxCharges {
					battleRemoveBuff(target, targetBuffID)
					logs = append(logs, map[string]any{
						"trigger":  "on_before_heal",
						"kind":     "state_detach",
						"buffId":   targetBuffID,
						"targetId": target.UnitID,
						"reason":   "charges_exhausted",
						"ts":       now.UnixMilli(),
					})
				}
			}
		}
	}
	return heal, logs
}

func (s *Server) battleTriggerUnitBuffHooks(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	trigger string,
	incomingBuffIDs ...string,
) []map[string]any {
	return s.battleTriggerUnitBuffHooksWithSource(state, now, self, nil, trigger, incomingBuffIDs...)
}

func (s *Server) battleConfiguredDefaultBasicSkillID() string {
	for _, raw := range s.deps.GameData.CombatConfigRules {
		value, _ := stringFromAny(raw["default_basic_skill_id"])
		if skillID := strings.TrimSpace(value); skillID != "" {
			return skillID
		}
	}
	return ""
}

func (s *Server) battleApplySkillCooldownBuffEffect(self *battleUnitState, currentBuff *battleBuffState, effect map[string]any, ruleID string, trigger string, delayMS int64) (map[string]any, bool) {
	if self == nil {
		return nil, false
	}
	amount := int(toInt64Loose(firstNonNil(effect["turns"], effect["value"], effect["amount"]), 0))
	if amount == 0 || len(self.Skills) == 0 {
		return nil, false
	}
	kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
	if kind != "adjustskillcooldown" && kind != "adjustskillcooldownturns" && amount < 0 {
		return nil, false
	}
	sourceID := self.UnitID
	buffID := ""
	if currentBuff != nil {
		sourceID = firstNonEmptyString(strings.TrimSpace(currentBuff.SourceID), self.UnitID)
		buffID = currentBuff.BuffID
	}
	return s.battleAdjustUnitSkillCooldownsWithSelector(self, amount, battleParseSkillCooldownSelector(effect), sourceID, trigger, delayMS, map[string]any{
		"buffId": buffID,
		"ruleId": ruleID,
	})
}

func (s *Server) battleAdjustUnitSkillCooldowns(
	unit *battleUnitState,
	amount int,
	includeBasic bool,
	targetSkillIDs []string,
	excludeSkillIDs []string,
	sourceID string,
	reason string,
	delayMS int64,
	meta map[string]any,
) (map[string]any, bool) {
	return s.battleAdjustUnitSkillCooldownsWithSelector(unit, amount, battleSkillCooldownSelector{
		IncludeBasic:    includeBasic,
		SkillIDs:        targetSkillIDs,
		ExcludeSkillIDs: excludeSkillIDs,
	}, sourceID, reason, delayMS, meta)
}

func (s *Server) battleAdjustUnitSkillCooldownsWithSelector(
	unit *battleUnitState,
	amount int,
	selector battleSkillCooldownSelector,
	sourceID string,
	reason string,
	delayMS int64,
	meta map[string]any,
) (map[string]any, bool) {
	if unit == nil || amount == 0 || len(unit.Skills) == 0 {
		return nil, false
	}
	basicSkillID := "basic_attack"
	if s != nil {
		basicSkillID = s.battleConfiguredDefaultBasicSkillID()
	}
	candidates := battleSkillCooldownCandidateIndices(unit, selector, basicSkillID, amount < 0)
	affected := make([]map[string]any, 0, len(unit.Skills))
	for _, idx := range candidates {
		skillID := strings.ToLower(strings.TrimSpace(unit.Skills[idx].ID))
		before := unit.Skills[idx].CooldownRemain
		unit.Skills[idx].CooldownRemain = maxInt(0, before+amount)
		if unit.Skills[idx].CooldownRemain == before {
			continue
		}
		affected = append(affected, map[string]any{
			"skillId": skillID,
			"before":  before,
			"after":   unit.Skills[idx].CooldownRemain,
		})
	}
	if len(affected) == 0 {
		return nil, false
	}
	if strings.TrimSpace(sourceID) == "" {
		sourceID = unit.UnitID
	}
	return map[string]any{
		"kind":     "skill_cooldown",
		"targetId": unit.UnitID,
		"sourceId": sourceID,
		"amount":   amount,
		"reason":   reason,
		"delayMs":  delayMS,
		"meta": mergeActionMeta(meta, map[string]any{
			"affectedSkills":  affected,
			"includeBasic":    selector.IncludeBasic,
			"skillCategories": append([]string(nil), selector.SkillCategories...),
			"selectionMode":   battleNormalizeSkillCooldownSelectionMode(selector.SelectionMode),
			"maxSkills":       selector.MaxSkills,
		}),
	}, true
}

func battleSkillIndexByID(unit *battleUnitState, skillID string) int {
	if unit == nil {
		return -1
	}
	skillID = strings.TrimSpace(strings.ToLower(skillID))
	if skillID == "" {
		return -1
	}
	for idx := range unit.Skills {
		if strings.ToLower(strings.TrimSpace(unit.Skills[idx].ID)) == skillID {
			return idx
		}
	}
	return -1
}

func battleSkillIndexAllowedByCooldownSelector(unit *battleUnitState, skillIndex int, selector battleSkillCooldownSelector, basicSkillID string) bool {
	if unit == nil || skillIndex < 0 || skillIndex >= len(unit.Skills) {
		return false
	}
	selector.SelectionMode = "all"
	selector.MaxSkills = 0
	for _, candidateIndex := range battleSkillCooldownCandidateIndices(unit, selector, basicSkillID, false) {
		if candidateIndex == skillIndex {
			return true
		}
	}
	return false
}

func (s *Server) battleTransferSkillCooldown(
	unit *battleUnitState,
	contextSkillID string,
	effect map[string]any,
	sourceID string,
	reason string,
	meta map[string]any,
) (map[string]any, string, bool) {
	if unit == nil || len(unit.Skills) == 0 || effect == nil {
		return nil, "", false
	}
	amount := int(toInt64Loose(firstNonNil(effect["amount"], effect["turns"], effect["value"]), 0))
	if amount <= 0 {
		return nil, "", false
	}
	contextSkillIndex := battleSkillIndexByID(unit, contextSkillID)
	if contextSkillIndex < 0 {
		return nil, "", false
	}
	basicSkillID := "basic_attack"
	if s != nil {
		basicSkillID = s.battleConfiguredDefaultBasicSkillID()
	}
	toRaw := normalizeAnyMap(effect["to"])
	if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(toRaw["selectionMode"], toRaw["selection_mode"]))) != "contextskill" {
		return nil, "", false
	}
	toSelector := battleParseSkillCooldownSelector(toRaw)
	if !battleSkillIndexAllowedByCooldownSelector(unit, contextSkillIndex, toSelector, basicSkillID) {
		return nil, "", false
	}
	fromRaw := normalizeAnyMap(effect["from"])
	fromSelector := battleParseSkillCooldownSelector(fromRaw)
	if parseBooleanLoose(firstNonNil(fromRaw["excludeContextSkill"], fromRaw["exclude_context_skill"]), true) {
		fromSelector.ExcludeSkillIDs = append(fromSelector.ExcludeSkillIDs, unit.Skills[contextSkillIndex].ID)
	}
	fromSelector.MaxSkills = 1
	candidateIndices := battleSkillCooldownCandidateIndices(unit, fromSelector, basicSkillID, true)
	if len(candidateIndices) == 0 {
		return nil, "", false
	}
	fromSkillIndex := candidateIndices[0]
	if fromSkillIndex == contextSkillIndex {
		return nil, "", false
	}

	waiverBuffID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["waiveIncreaseBuffId"], effect["waive_increase_buff_id"])))
	if waiverBuffID == "<nil>" {
		waiverBuffID = ""
	}
	increaseWaived := waiverBuffID != "" && findBuffByID(unit, waiverBuffID) != nil
	fromBefore := unit.Skills[fromSkillIndex].CooldownRemain
	fromAfter := maxInt(0, fromBefore-amount)
	if fromAfter == fromBefore {
		return nil, "", false
	}
	toBefore := unit.Skills[contextSkillIndex].CooldownRemain
	toAfter := toBefore
	if !increaseWaived {
		toAfter = toBefore + amount
	}
	unit.Skills[fromSkillIndex].CooldownRemain = fromAfter
	unit.Skills[contextSkillIndex].CooldownRemain = toAfter

	consumedWaiverBuffID := ""
	if increaseWaived && parseBooleanLoose(firstNonNil(effect["consumeWaiverBuff"], effect["consume_waiver_buff"]), true) {
		if battleRemoveBuff(unit, waiverBuffID) {
			consumedWaiverBuffID = waiverBuffID
		}
	}
	affectedSkills := []map[string]any{{
		"skillId": strings.ToLower(strings.TrimSpace(unit.Skills[fromSkillIndex].ID)),
		"before":  fromBefore,
		"after":   fromAfter,
		"role":    "from",
	}}
	if toAfter != toBefore {
		affectedSkills = append(affectedSkills, map[string]any{
			"skillId": strings.ToLower(strings.TrimSpace(unit.Skills[contextSkillIndex].ID)),
			"before":  toBefore,
			"after":   toAfter,
			"role":    "to",
		})
	}
	if strings.TrimSpace(sourceID) == "" {
		sourceID = unit.UnitID
	}
	return map[string]any{
		"kind":     "skill_cooldown",
		"targetId": unit.UnitID,
		"sourceId": sourceID,
		"amount":   amount,
		"reason":   reason,
		"delayMs":  int64(0),
		"meta": mergeActionMeta(meta, map[string]any{
			"effectKind":           "transfer_skill_cooldown",
			"affectedSkills":       affectedSkills,
			"fromSkillId":          strings.ToLower(strings.TrimSpace(unit.Skills[fromSkillIndex].ID)),
			"toSkillId":            strings.ToLower(strings.TrimSpace(unit.Skills[contextSkillIndex].ID)),
			"transferredAmount":    fromBefore - fromAfter,
			"increaseWaived":       increaseWaived,
			"consumedWaiverBuffId": consumedWaiverBuffID,
		}),
	}, consumedWaiverBuffID, true
}

func (s *Server) battleTriggerUnitBuffHooksWithSource(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	incomingBuffIDs ...string,
) []map[string]any {
	return s.battleTriggerUnitBuffHooksWithSourceAndFilter(state, now, self, source, trigger, nil, incomingBuffIDs...)
}

func (s *Server) battleTriggerUnitBuffHooksWithContext(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	context *battleEquipmentHookContext,
	incomingBuffIDs ...string,
) []map[string]any {
	return s.battleTriggerUnitBuffHooksWithSourceAndFilter(state, now, self, source, trigger, func(rule battleBuffEffectRule) bool {
		return battleEquipmentHookEventAllowed(
			battleEquipmentHookRule{EventFilter: rule.EventFilter},
			self,
			source,
			context,
		)
	}, incomingBuffIDs...)
}

func (s *Server) battleTriggerUnitBuffHooksWithSourceAndFilter(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	ruleFilter func(battleBuffEffectRule) bool,
	incomingBuffIDs ...string,
) []map[string]any {
	if self == nil {
		return nil
	}
	triggerKey := battleNormalizeTrigger(trigger)
	if triggerKey == "" {
		return nil
	}
	attachFilter := make(map[string]struct{}, len(incomingBuffIDs))
	for _, id := range incomingBuffIDs {
		if key := strings.ToLower(strings.TrimSpace(id)); key != "" {
			attachFilter[key] = struct{}{}
		}
	}
	actions := make([]map[string]any, 0, 4)
	affectedUnits := make(map[*battleUnitState]struct{})
	// Snapshot buff IDs to allow safe mutation (e.g., consume_charge / remove_buff mid-loop).
	buffIDs := snapshotBuffIDs(self)
	for _, buffID := range buffIDs {
		if len(attachFilter) > 0 {
			if _, ok := attachFilter[buffID]; !ok {
				continue
			}
		}
		maxCharges := s.battleBuffMaxCharges(buffID)
		for _, rule := range s.battleFindBuffRules(buffID, triggerKey) {
			if ruleFilter != nil && !ruleFilter(rule) {
				continue
			}
			if !battleBuffRuleLimitAllowed(state, self, rule) {
				continue
			}
			// Re-look up currentBuff each rule in case an earlier effect removed it.
			currentBuff := findBuffByID(self, buffID)
			if currentBuff == nil {
				break // buff removed; skip remaining rules
			}
			chargesRemaining := battleBuffChargesRemaining(currentBuff, maxCharges)
			if !battleRuleConditionsPassed(rule, self, source, 0, "", currentBuff, chargesRemaining) {
				continue
			}
			battleMarkBuffRuleTriggered(state, self, rule)
			for ei, effect := range rule.Effects {
				kind := ""
				if ei < len(rule.EffectKinds) {
					kind = rule.EffectKinds[ei]
				} else {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
					if kind == "" {
						kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
					}
				}
				delayMS := maxInt64(0, toInt64Loose(effect["delayMs"], 0))
				switch kind {
				case "damage":
					value := maxFloat(0, toFloat64(firstNonNil(effect["value"], effect["amount"]), 0))
					sourceID := strings.TrimSpace(currentBuff.SourceID)
					var sourceUnit *battleUnitState
					if sourceID != "" {
						sourceUnit = battleUnitByID(state, sourceID)
					}
					if amountFormulaRaw, ok := firstNonNil(effect["amountFormula"], effect["damageFormula"]).(map[string]any); ok && len(amountFormulaRaw) > 0 {
						value += battleEvaluateAmountFormula(sourceUnit, 0, parseBattleSkillAmountFormula(amountFormulaRaw, nil))
					}
					if value <= 0 {
						continue
					}
					value, outputSources := s.battleApplyOutgoingBuffModifiers(sourceUnit, "damage", value)
					if value <= 0 {
						continue
					}
					damageType := normalizeBattleDamageType(fmt.Sprintf("%v", firstNonNil(effect["damageType"], effect["damage_type"])))
					if trialAmp := battleCombatTrialDamageAmplification(state, sourceID, sourceUnit, damageType); trialAmp != 0 {
						value *= maxFloat(0, 1+trialAmp)
					}
					beforeHP := self.HP
					beforeAlive := self.Alive
					if sourceID == "" {
						sourceID = self.UnitID
					}
					battleCombatTrialRecordDamage(state, sourceID, sourceUnit, self, value)
					if battleCombatTrialImmutableTarget(state, self) {
						battleRestoreCombatTrialImmutableTarget(state, self)
					} else {
						self.HP = maxFloat(0, self.HP-value)
						self.Alive = self.HP > 0
					}
					meta := map[string]any{
						"buffId": currentBuff.BuffID,
						"ruleId": rule.ID,
					}
					if len(outputSources) > 0 {
						meta["outgoingBuffAdjusted"] = true
						meta["outgoingBuffSources"] = outputSources
					}
					actions = append(actions, map[string]any{
						"kind":        "damage",
						"targetId":    self.UnitID,
						"sourceId":    sourceID,
						"amount":      roundTo4(value),
						"damageType":  damageType,
						"reason":      trigger,
						"beforeHp":    roundTo4(beforeHP),
						"afterHp":     roundTo4(self.HP),
						"beforeAlive": beforeAlive,
						"afterAlive":  self.Alive,
						"delayMs":     delayMS,
						"meta":        meta,
					})
					if beforeAlive && !self.Alive {
						battleQueueUnitDeath(state, self, sourceUnit)
					}
				case "heal":
					value := battleBuffEffectScalar(effect, currentBuff, effect["value"])
					if formulaRaw := firstNonNil(effect["amountFormula"], effect["amount_formula"], effect["healFormula"], effect["heal_formula"]); formulaRaw != nil {
						value += battleEvaluateAmountFormula(self, 1, parseBattleSkillAmountFormula(formulaRaw, nil))
					}
					value = maxFloat(0, value)
					value, _ = s.battleApplyOutgoingBuffModifiers(self, "heal", value)
					beforeHP := self.HP
					beforeAlive := self.Alive
					self.HP = clampFloat(self.HP+value, 0, self.MaxHP)
					if self.HP > 0 {
						self.Alive = true
					}
					metricSourceID, metricSource := battleCombatTrialBuffMetricSource(state, currentBuff, self)
					battleCombatTrialRecordHeal(state, metricSourceID, metricSource, self, self.HP-beforeHP)
					actionSourceID := self.UnitID
					if battleCombatTrialActive(state) && metricSourceID != "" {
						actionSourceID = metricSourceID
					}
					actions = append(actions, map[string]any{
						"kind":        "heal",
						"targetId":    self.UnitID,
						"sourceId":    actionSourceID,
						"amount":      roundTo4(value),
						"reason":      trigger,
						"beforeHp":    roundTo4(beforeHP),
						"afterHp":     roundTo4(self.HP),
						"beforeAlive": beforeAlive,
						"afterAlive":  self.Alive,
						"delayMs":     delayMS,
						"meta": map[string]any{
							"buffId": currentBuff.BuffID,
							"ruleId": rule.ID,
						},
					})
				case "resource":
					resource := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", effect["resource"])))
					value := battleBuffEffectScalar(effect, currentBuff, effect["value"])
					if formulaRaw := firstNonNil(effect["amountFormula"], effect["amount_formula"], effect["resourceFormula"], effect["resource_formula"]); formulaRaw != nil {
						value += battleEvaluateAmountFormula(self, 1, parseBattleSkillAmountFormula(formulaRaw, nil))
					}
					if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["valueScale"], effect["value_scale"]))) == "maxresource" {
						maximum := self.MaxMP
						if resource == "hp" {
							maximum = self.MaxHP
						}
						value *= maximum
					}
					beforeHP := self.HP
					beforeMP := self.MP
					beforeAlive := self.Alive
					var outputSources []map[string]any
					switch resource {
					case "hp":
						if value > 0 {
							value, outputSources = s.battleApplyOutgoingBuffModifiers(self, "heal", value)
						}
						minimumHP := maxFloat(0, toFloat64(firstNonNil(effect["minimumHp"], effect["minimum_hp"]), 0))
						self.HP = clampFloat(maxFloat(minimumHP, self.HP+value), 0, self.MaxHP)
						if self.HP <= 0 {
							self.Alive = false
						} else {
							self.Alive = true
						}
					default:
						resource = "mp"
						self.MP = clampFloat(self.MP+value, 0, self.MaxMP)
					}
					metricSourceID, metricSource := battleCombatTrialBuffMetricSource(state, currentBuff, self)
					battleRestoreCombatTrialImmutableTarget(state, self)
					if resource == "hp" && value > 0 {
						battleCombatTrialRecordHeal(state, metricSourceID, metricSource, self, maxFloat(0, self.HP-beforeHP))
					}
					actionSourceID := self.UnitID
					if battleCombatTrialActive(state) && metricSourceID != "" {
						actionSourceID = metricSourceID
					}
					meta := map[string]any{
						"buffId": currentBuff.BuffID,
						"ruleId": rule.ID,
					}
					if len(outputSources) > 0 {
						meta["outgoingBuffAdjusted"] = true
						meta["outgoingBuffSources"] = outputSources
					}
					actions = append(actions, map[string]any{
						"kind":        "resource",
						"targetId":    self.UnitID,
						"sourceId":    actionSourceID,
						"resource":    resource,
						"amount":      roundTo4(value),
						"reason":      trigger,
						"beforeHp":    roundTo4(beforeHP),
						"afterHp":     roundTo4(self.HP),
						"beforeMp":    roundTo4(beforeMP),
						"afterMp":     roundTo4(self.MP),
						"beforeAlive": beforeAlive,
						"afterAlive":  self.Alive,
						"delayMs":     delayMS,
						"meta":        meta,
					})
					if beforeAlive && !self.Alive {
						battleQueueUnitDeath(state, self, self)
					}
				case "addskillcooldown", "addskillcooldownturns", "adjustskillcooldown", "adjustskillcooldownturns":
					if action, ok := s.battleApplySkillCooldownBuffEffect(self, currentBuff, effect, rule.ID, trigger, delayMS); ok {
						actions = append(actions, action)
					}
				case "addbuff":
					newBuffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
					if newBuffID == "" {
						continue
					}
					durationTurns := toInt64Loose(effect["durationTurns"], 0)
					if durationTurns == 0 {
						if durationMS := toInt64Loose(effect["durationMs"], 0); durationMS != 0 {
							durationTurns = battleDurationTurnsFromLegacyMS(durationMS, s.battleTurnIntervalMS())
						}
					}
					if durationTurns == 0 {
						durationTurns = s.battleBuffDurationByIDTurns(newBuffID)
					}
					payload := normalizeAnyMap(effect["payload"])
					target := self
					targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target"])))
					if targetMode == "source" {
						if source == nil || !source.Alive {
							continue
						}
						target = source
					} else {
						targetMode = "self"
					}
					isNewAttach := battleApplyOrRefreshBuffForTurn(
						target,
						newBuffID,
						durationTurns,
						payload,
						self.UnitID,
						s.battleBuffStackPolicyByID(newBuffID),
						s.battleBuffMaxStacksByID(newBuffID),
						battleBuffApplicationTurnSerial(state, trigger),
					)
					affectedUnits[target] = struct{}{}
					actions = append(actions, map[string]any{
						"kind":              "buff",
						"targetId":          target.UnitID,
						"sourceId":          self.UnitID,
						"buffId":            newBuffID,
						"buffDurationTurns": durationTurns,
						"buffPayload":       payload,
						"reason":            trigger,
						"delayMs":           delayMS,
						"meta": map[string]any{
							"buffId":     currentBuff.BuffID,
							"ruleId":     rule.ID,
							"targetMode": targetMode,
						},
					})
					afterApplyActions := s.battleTriggerAfterApplyBuffEquipmentHooks(state, now, self, target, newBuffID, isNewAttach)
					if len(afterApplyActions) > 0 {
						actions = append(actions, afterApplyActions...)
					}
				case "removebuff":
					removeBuffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
					if removeBuffID == "" {
						removeBuffID = currentBuff.BuffID
					}
					if !battleRemoveBuff(self, removeBuffID) {
						continue
					}
					actions = append(actions, map[string]any{
						"kind":     "state_detach",
						"targetId": self.UnitID,
						"sourceId": self.UnitID,
						"buffId":   removeBuffID,
						"reason":   trigger,
						"delayMs":  delayMS,
						"meta": map[string]any{
							"buffId": currentBuff.BuffID,
							"ruleId": rule.ID,
						},
					})
				case "consumecharge":
					targetBuffID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"])))
					if targetBuffID == "" || targetBuffID == "<nil>" {
						targetBuffID = buffID
					}
					targetBuff := findBuffByID(self, targetBuffID)
					if targetBuff == nil {
						continue
					}
					targetMaxCharges := s.battleBuffMaxCharges(targetBuffID)
					if targetMaxCharges <= 0 {
						continue
					}
					targetBuff.Triggered++
					if targetBuff.Triggered >= targetMaxCharges {
						battleRemoveBuff(self, targetBuffID)
						actions = append(actions, map[string]any{
							"kind":     "state_detach",
							"targetId": self.UnitID,
							"sourceId": self.UnitID,
							"buffId":   targetBuffID,
							"reason":   "charges_exhausted",
							"delayMs":  delayMS,
							"meta": map[string]any{
								"buffId": currentBuff.BuffID,
								"ruleId": rule.ID,
							},
						})
					} else {
						actions = append(actions, map[string]any{
							"kind":             "buff_charge",
							"targetId":         self.UnitID,
							"sourceId":         self.UnitID,
							"buffId":           targetBuffID,
							"buffTriggered":    targetBuff.Triggered,
							"chargesRemaining": targetMaxCharges - targetBuff.Triggered,
							"buffPayload":      cloneAnyMap(targetBuff.Payload),
							"reason":           trigger,
							"delayMs":          delayMS,
							"meta": map[string]any{
								"buffId": currentBuff.BuffID,
								"ruleId": rule.ID,
							},
						})
					}
				}
			}
		}
	}
	appendStatRecalcIfChanged := func(unit *battleUnitState, sourceID string) {
		if unit == nil {
			return
		}
		beforeStats := cloneFloatMap(unit.Stats)
		beforeMaxHP := unit.MaxHP
		beforeMaxMP := unit.MaxMP
		beforePosition := unit.Position
		beforeSide := normalizeBattleSide(unit.Side)
		beforePerspective := battleUnitPerspectiveSide(unit)
		s.battleRecomputeUnitStats(unit)
		afterSide := normalizeBattleSide(unit.Side)
		afterPerspective := battleUnitPerspectiveSide(unit)
		if battleFloatMapEqual(beforeStats, unit.Stats) && beforeMaxHP == unit.MaxHP && beforeMaxMP == unit.MaxMP && beforePosition == unit.Position && beforeSide == afterSide && beforePerspective == afterPerspective {
			return
		}
		actions = append(actions, map[string]any{
			"kind":            "stat_recalc",
			"targetId":        unit.UnitID,
			"sourceId":        sourceID,
			"reason":          trigger,
			"stats":           cloneFloatMap(unit.Stats),
			"baseStats":       cloneFloatMap(unit.BaseStats),
			"deltaStats":      cloneFloatMap(unit.StatModifiers),
			"statModifiers":   cloneFloatMap(unit.StatModifiers),
			"statSources":     cloneNestedFloatMap(unit.StatSources),
			"buffs":           battlePublicBuffSlice(unit.Buffs),
			"maxHp":           roundTo4(unit.MaxHP),
			"maxMp":           roundTo4(unit.MaxMP),
			"fromPosition":    beforePosition,
			"position":        unit.Position,
			"basePosition":    unit.BasePosition,
			"side":            unit.Side,
			"baseSide":        unit.BaseSide,
			"perspectiveSide": unit.PerspectiveSide,
			"delayMs":         int64(0),
			"meta": fiber.Map{
				"trigger": trigger,
				"ts":      now.UnixMilli(),
			},
		})
	}
	appendStatRecalcIfChanged(self, self.UnitID)
	for unit := range affectedUnits {
		if unit == self {
			continue
		}
		appendStatRecalcIfChanged(unit, self.UnitID)
	}
	return actions
}

func battleFloatMapEqual(left map[string]float64, right map[string]float64) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if roundTo4(value) != roundTo4(right[key]) {
			return false
		}
	}
	return true
}

func battlePublicBuffSlice(buffs []battleBuffState) []map[string]any {
	if len(buffs) == 0 {
		return []map[string]any{}
	}
	rows := make([]map[string]any, 0, len(buffs))
	for _, buff := range buffs {
		rows = append(rows, map[string]any{
			"buffId":      strings.TrimSpace(buff.BuffID),
			"remainTurns": buff.RemainTurns,
			"payload":     cloneAnyMap(buff.Payload),
			"sourceId":    strings.TrimSpace(buff.SourceID),
			"triggered":   buff.Triggered,
		})
	}
	return rows
}

func battleFormationBuffID(position string) string {
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "vanguard":
		return "formation_frontline_guard"
	case "midguard":
		return "formation_midline_speed"
	case "rearguard":
		return "formation_rearline_amplify"
	default:
		return ""
	}
}

func (s *Server) battleApplyFormationBuff(unit *battleUnitState) {
	if s == nil || unit == nil {
		return
	}
	buffID := battleFormationBuffID(unit.Position)
	if buffID == "" {
		return
	}
	for _, row := range unit.Buffs {
		if strings.EqualFold(strings.TrimSpace(row.BuffID), buffID) {
			return
		}
	}
	unit.Buffs = append(unit.Buffs, battleBuffState{
		BuffID:      buffID,
		RemainTurns: -1,
		Payload: map[string]any{
			"origin": "formation",
		},
		SourceID: "system:formation",
	})
}

// battleApplyBuffExpiry handles the consequences of buff(s) expiring on a unit.
// It reads the buff DSL's onExpire field from cache and executes the appropriate action.
// Returns (shouldRemoveUnit, actionsToAppend).
// Callers must append the returned actions to the current event's actionLog.
func (s *Server) battleApplyBuffExpiry(unit *battleUnitState, expiredBuffIDs []string) (bool, []map[string]any) {
	if s == nil || unit == nil || len(expiredBuffIDs) == 0 {
		return false, nil
	}
	s.battleParseBuffRules()
	shouldRemove := false
	var actions []map[string]any
	for _, rawID := range expiredBuffIDs {
		buffID := strings.ToLower(strings.TrimSpace(rawID))
		if s.buffOnExpireCache[buffID] != "remove_unit" {
			continue
		}
		shouldRemove = true
		if !unit.Alive || unit.HP <= 0 {
			continue
		}
		beforeHP := unit.HP
		unit.HP = 0
		unit.MP = 0
		unit.Alive = false
		unit.SummonRemainTurns = 0
		sourceID := strings.TrimSpace(unit.SummonSourceUnitID)
		if sourceID == "" {
			sourceID = unit.UnitID
		}
		actions = append(actions, map[string]any{
			"kind":       "summon_expire",
			"targetId":   unit.UnitID,
			"sourceId":   sourceID,
			"reason":     "buff_on_expire_remove_unit",
			"beforeHp":   roundTo4(beforeHP),
			"afterHp":    0.0,
			"afterMp":    0.0,
			"afterAlive": false,
			"delayMs":    int64(0),
		})
	}
	return shouldRemove, actions
}
