package rules

import (
	"strconv"
	"strings"
	"time"
)

type Condition struct {
	Field string  `json:"field,omitempty"`
	Op    string  `json:"op,omitempty"`
	Value float64 `json:"value,omitempty"`

	CompareType string `json:"compareType,omitempty"`
	Target      string `json:"target,omitempty"`

	DataType            string `json:"dataType,omitempty"`
	ValueCompareAction  string `json:"valueCompareAction,omitempty"`
	ValueConditionValue any    `json:"valueConditionValue,omitempty"`
	EnemyCountPosition  string `json:"enemyCountPosition,omitempty"`

	StateCompareAction  string `json:"stateCompareAction,omitempty"`
	StateCompareStateID string `json:"stateCompareStateId,omitempty"`

	CooldownCompareAction string   `json:"cooldownCompareAction,omitempty"`
	CooldownSkillIDs      []string `json:"cooldownSkillIds,omitempty"`
	CooldownItemIDs       []string `json:"cooldownItemIds,omitempty"`

	UnitExistCompareAction string `json:"unitExistCompareAction,omitempty"`
	UnitID                 string `json:"unitId,omitempty"`

	SpecialAction string `json:"specialAction,omitempty"`
}

type ConditionGroup struct {
	Conditions []Condition `json:"conditions"`
}

type UnitContext struct {
	UnitID             string             `json:"unitId,omitempty"`
	TemplateUnitID     string             `json:"templateUnitId,omitempty"`
	HP                 float64            `json:"hp"`
	MP                 float64            `json:"mp"`
	MaxHP              float64            `json:"maxHp"`
	MaxMP              float64            `json:"maxMp"`
	IsBoss             bool               `json:"isBoss,omitempty"`
	SkillCooldowns     map[string]float64 `json:"skillCooldowns,omitempty"`
	ItemCooldowns      map[string]float64 `json:"itemCooldowns,omitempty"`
	StateStacks        map[string]float64 `json:"stateStacks,omitempty"`
	StateKinds         map[string]string  `json:"stateKinds,omitempty"`
	SummonSourceUnitID string             `json:"summonSourceUnitId,omitempty"`
}

type EvalContext struct {
	SelfHPPercent  float64 `json:"selfHpPercent"`
	CooldownReady  float64 `json:"cooldownReady"`
	HasBuff        float64 `json:"hasBuff"`
	TargetTagMatch float64 `json:"targetTagMatch"`
	InventoryCount float64 `json:"inventoryCount"`
	EnemyCount     float64 `json:"enemyCount"`
	Distance       float64 `json:"distance"`
	TimeWindow     float64 `json:"timeWindow"`
	PassedTurn     float64 `json:"passedTurn,omitempty"`

	SkillCooldowns map[string]float64 `json:"skillCooldowns,omitempty"`
	ItemCooldowns  map[string]float64 `json:"itemCooldowns,omitempty"`
	StateStacks    map[string]float64 `json:"stateStacks,omitempty"`
	States         map[string]float64 `json:"states,omitempty"`
	UnitAlive      map[string]float64 `json:"unitAlive,omitempty"`
	SpecialFlags   map[string]float64 `json:"specialFlags,omitempty"`
	Metrics        map[string]float64 `json:"metrics,omitempty"`

	Self              *UnitContext  `json:"self,omitempty"`
	Allies            []UnitContext `json:"allies,omitempty"`
	Enemies           []UnitContext `json:"enemies,omitempty"`
	AllUnits          []UnitContext `json:"allUnits,omitempty"`
	SelfSummonedUnits []UnitContext `json:"selfSummonedUnits,omitempty"`
}

func Evaluate(groups []ConditionGroup, ctx EvalContext) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if evaluateGroup(g, ctx) {
			return true
		}
	}
	return false
}

func evaluateGroup(group ConditionGroup, ctx EvalContext) bool {
	if len(group.Conditions) == 0 {
		return false
	}
	for _, c := range group.Conditions {
		if !evaluateCondition(c, ctx) {
			return false
		}
	}
	return true
}

func evaluateCondition(c Condition, ctx EvalContext) bool {
	compareType := strings.TrimSpace(c.CompareType)
	if compareType != "" {
		switch strings.ToLower(compareType) {
		case "value":
			return evaluateValueCondition(c, ctx)
		case "state":
			return evaluateStateCondition(c, ctx)
		case "statestack":
			return evaluateStateStackCondition(c, ctx)
		case "unitexiststate":
			return evaluateUnitExistStateCondition(c, ctx)
		case "cooldown":
			return evaluateUnitCooldownCondition(c, ctx, false)
		case "itemcooldown":
			return evaluateUnitCooldownCondition(c, ctx, true)
		case "special":
			return evaluateSpecialCondition(c, ctx)
		default:
			return false
		}
	}

	left, ok := lookupField(strings.TrimSpace(c.Field), ctx)
	if !ok {
		return false
	}
	return compareNumber(left, c.Value, c.Op)
}

func evaluateValueCondition(c Condition, ctx EvalContext) bool {
	dataType := strings.TrimSpace(c.DataType)
	if dataType == "" {
		dataType = strings.TrimSpace(c.Field)
	}
	if dataType == "" {
		dataType = "self_hp_percent"
	}
	normalizedDataType := normalizeLogicToken(dataType)
	if normalizedDataType == "enemycount" {
		left, ok := lookupEnemyCountValue(c, ctx)
		if !ok {
			return false
		}
		threshold, ok := parseConditionNumber(c.ValueConditionValue, c.Value)
		if !ok {
			return false
		}
		return compareNumber(left, threshold, conditionOp(c, "<="))
	}

	if isUnitValueDataType(normalizedDataType) {
		return evaluateUnitValueCondition(c, ctx, normalizedDataType)
	}

	left, ok := lookupField(dataType, ctx)
	if !ok {
		return false
	}
	threshold, ok := parseConditionNumber(c.ValueConditionValue, c.Value)
	if !ok {
		return false
	}
	return compareNumber(left, threshold, conditionOp(c, "<="))
}

func lookupEnemyCountValue(c Condition, ctx EvalContext) (float64, bool) {
	target := normalizeEnemyCountTarget(c.Target)
	position := normalizeEnemyCountPosition(c.EnemyCountPosition)
	if ctx.Metrics != nil {
		key := "enemy_count." + target + "." + position
		if value, ok := ctx.Metrics[key]; ok {
			return value, true
		}
		if position == "all" {
			if value, ok := ctx.Metrics["enemy_count."+target]; ok {
				return value, true
			}
		}
		if value, ok := ctx.Metrics["enemy_count"]; ok {
			return value, true
		}
	}
	if target == "enemies" && position == "all" {
		return ctx.EnemyCount, true
	}
	return 0, false
}

func normalizeEnemyCountTarget(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "allies", "eachallies", "all", "self", "none", "selfsummonedunit":
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "eachallies" {
			return "allies"
		}
		if value == "selfsummonedunit" {
			return "self"
		}
		return value
	case "eachenemies":
		return "enemies"
	case "enemies":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "enemies"
	}
}

func normalizeEnemyCountPosition(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "vanguard", "midguard", "rearguard":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "all"
	}
}

func evaluateStateCondition(c Condition, ctx EvalContext) bool {
	stateID := strings.ToLower(strings.TrimSpace(c.StateCompareStateID))
	action := strings.TrimSpace(c.StateCompareAction)
	if action == "" {
		action = "someoneHasState"
	}
	units, hasUnits := targetUnits(c.Target, ctx)
	if !hasUnits {
		hasState := false
		if stateID == "" {
			hasState = ctx.HasBuff > 0
		} else {
			hasState = lookupStateFlag(stateID, ctx)
		}
		switch action {
		case "someoneHasState", "AllHasState":
			return hasState
		case "someoneNotHasState", "allNotHasState":
			return !hasState
		default:
			return false
		}
	}
	hasState := func(unit UnitContext) bool {
		if stateID == "" {
			return len(unit.StateStacks) > 0
		}
		return unit.StateStacks[stateID] > 0
	}
	switch action {
	case "someoneHasState":
		return anyUnit(units, hasState)
	case "AllHasState":
		return allUnits(units, hasState)
	case "someoneNotHasState":
		return anyUnit(units, func(unit UnitContext) bool { return !hasState(unit) })
	case "allNotHasState":
		return allUnits(units, func(unit UnitContext) bool { return !hasState(unit) })
	default:
		return false
	}
}

func evaluateStateStackCondition(c Condition, ctx EvalContext) bool {
	stateID := strings.ToLower(strings.TrimSpace(c.StateCompareStateID))
	threshold, ok := parseConditionNumber(c.ValueConditionValue, c.Value)
	if !ok {
		return false
	}
	units, hasUnits := targetUnits(c.Target, ctx)
	if !hasUnits {
		left := 0.0
		if stateID != "" {
			left = ctx.StateStacks[stateID]
		}
		return compareNumber(left, threshold, conditionOp(c, ">="))
	}
	predicate := func(unit UnitContext) bool {
		if stateID == "" {
			return false
		}
		return compareNumber(unit.StateStacks[stateID], threshold, conditionOp(c, ">="))
	}
	return evaluateTargetIteration(c.Target, units, predicate)
}

func evaluateUnitExistStateCondition(c Condition, ctx EvalContext) bool {
	unitID := strings.ToLower(strings.TrimSpace(c.UnitID))
	if unitID == "" {
		return false
	}
	exists := ctx.UnitAlive[unitID] > 0
	if units, hasUnits := targetUnits(c.Target, ctx); hasUnits {
		exists = anyUnit(units, func(unit UnitContext) bool {
			return strings.EqualFold(strings.TrimSpace(unit.UnitID), unitID) ||
				strings.EqualFold(strings.TrimSpace(unit.TemplateUnitID), unitID)
		})
	}
	action := strings.TrimSpace(c.UnitExistCompareAction)
	if action == "" {
		action = "exist"
	}
	switch action {
	case "notExist":
		return !exists
	case "exist":
		return exists
	default:
		return false
	}
}

func evaluateCooldownCondition(ids []string, action string, source map[string]float64) bool {
	normalizedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.ToLower(strings.TrimSpace(id))
		if trimmed == "" {
			continue
		}
		normalizedIDs = append(normalizedIDs, trimmed)
	}
	if len(normalizedIDs) == 0 {
		return false
	}
	flags := make([]bool, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		flags = append(flags, source[id] > 0)
	}
	switch strings.TrimSpace(action) {
	case "allInCooldown":
		for _, inCooldown := range flags {
			if !inCooldown {
				return false
			}
		}
		return true
	case "allNotInCooldown":
		for _, inCooldown := range flags {
			if inCooldown {
				return false
			}
		}
		return true
	case "someoneInCooldown":
		for _, inCooldown := range flags {
			if inCooldown {
				return true
			}
		}
		return false
	case "someoneNotInCooldown":
		for _, inCooldown := range flags {
			if !inCooldown {
				return true
			}
		}
		return false
	case "notInCooldown":
		return !flags[0]
	case "inCooldown":
		fallthrough
	default:
		return flags[0]
	}
}

func evaluateUnitCooldownCondition(c Condition, ctx EvalContext, item bool) bool {
	ids := c.CooldownSkillIDs
	if item {
		ids = c.CooldownItemIDs
	}
	units, hasUnits := targetUnits(c.Target, ctx)
	if !hasUnits {
		if item {
			return evaluateCooldownCondition(ids, c.CooldownCompareAction, ctx.ItemCooldowns)
		}
		return evaluateCooldownCondition(ids, c.CooldownCompareAction, ctx.SkillCooldowns)
	}
	predicate := func(unit UnitContext) bool {
		source := unit.SkillCooldowns
		if item {
			source = unit.ItemCooldowns
		}
		return evaluateCooldownCondition(ids, c.CooldownCompareAction, source)
	}
	return evaluateTargetIteration(c.Target, units, predicate)
}

func evaluateSpecialCondition(c Condition, ctx EvalContext) bool {
	action := strings.TrimSpace(c.SpecialAction)
	if action == "" {
		action = strings.TrimSpace(c.Field)
	}
	if action == "" {
		return false
	}
	switch action {
	case "enemyHasBoss":
		if ctx.Enemies != nil {
			return anyUnit(ctx.Enemies, func(unit UnitContext) bool { return unit.IsBoss })
		}
	case "enemyNotHasBoss":
		if ctx.Enemies != nil {
			return !anyUnit(ctx.Enemies, func(unit UnitContext) bool { return unit.IsBoss })
		}
	}
	return ctx.SpecialFlags[action] > 0
}

func evaluateUnitValueCondition(c Condition, ctx EvalContext, dataType string) bool {
	if dataType == "passedturn" {
		threshold, ok := parseConditionNumber(c.ValueConditionValue, c.Value)
		return ok && compareNumber(ctx.PassedTurn, threshold, conditionOp(c, ">="))
	}
	units, hasUnits := targetUnits(c.Target, ctx)
	if !hasUnits {
		if dataType == "hp" || dataType == "selfhppercent" {
			left, ok := lookupField("self_hp_percent", ctx)
			if !ok {
				return false
			}
			threshold, ok := parseConditionNumber(c.ValueConditionValue, c.Value)
			return ok && compareNumber(left, threshold, conditionOp(c, "<="))
		}
		return false
	}
	if dataType == "count" {
		threshold, ok := parseConditionNumber(c.ValueConditionValue, c.Value)
		return ok && compareNumber(float64(len(units)), threshold, conditionOp(c, ">="))
	}
	if dataType == "totalhp" {
		left := 0.0
		maxValue := 0.0
		for _, unit := range units {
			left += unit.HP
			maxValue += unit.MaxHP
		}
		threshold, ok := parseConditionThreshold(c.ValueConditionValue, c.Value, maxValue)
		return ok && compareNumber(left, threshold, conditionOp(c, "<="))
	}
	predicate := func(unit UnitContext) bool {
		left, maxValue, ok := unitValue(unit, dataType)
		if !ok {
			return false
		}
		threshold, ok := parseConditionThreshold(c.ValueConditionValue, c.Value, maxValue)
		if !ok {
			return false
		}
		return compareNumber(left, threshold, conditionOp(c, "<="))
	}
	return evaluateTargetIteration(c.Target, units, predicate)
}

func unitValue(unit UnitContext, dataType string) (float64, float64, bool) {
	switch dataType {
	case "hp":
		return unit.HP, unit.MaxHP, true
	case "mp":
		return unit.MP, unit.MaxMP, true
	case "losthp":
		return unit.MaxHP - unit.HP, unit.MaxHP, true
	case "lostmp":
		return unit.MaxMP - unit.MP, unit.MaxMP, true
	case "buffcount":
		return float64(countStatesByKind(unit, "buff")), 0, true
	case "debuffcount":
		return float64(countStatesByKind(unit, "debuff")), 0, true
	default:
		return 0, 0, false
	}
}

func countStatesByKind(unit UnitContext, kind string) int {
	count := 0
	for stateID, stacks := range unit.StateStacks {
		if stacks <= 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(unit.StateKinds[stateID]), kind) {
			count++
		}
	}
	return count
}

func isUnitValueDataType(dataType string) bool {
	switch dataType {
	case "hp", "mp", "losthp", "lostmp", "totalhp", "count", "buffcount", "debuffcount", "passedturn":
		return true
	default:
		return false
	}
}

func targetUnits(raw string, ctx EvalContext) ([]UnitContext, bool) {
	target := normalizeLogicToken(raw)
	switch target {
	case "self", "none":
		if ctx.Self != nil {
			return []UnitContext{*ctx.Self}, true
		}
	case "allies", "eachallies":
		if ctx.Allies != nil {
			return ctx.Allies, true
		}
	case "enemies", "eachenemies":
		if ctx.Enemies != nil {
			return ctx.Enemies, true
		}
	case "all":
		if ctx.AllUnits != nil {
			return ctx.AllUnits, true
		}
	case "selfsummonedunit":
		if ctx.SelfSummonedUnits != nil {
			return ctx.SelfSummonedUnits, true
		}
	}
	return nil, false
}

func evaluateTargetIteration(target string, units []UnitContext, predicate func(UnitContext) bool) bool {
	if normalizeLogicToken(target) == "eachallies" || normalizeLogicToken(target) == "eachenemies" {
		return allUnits(units, predicate)
	}
	return anyUnit(units, predicate)
}

func anyUnit(units []UnitContext, predicate func(UnitContext) bool) bool {
	for _, unit := range units {
		if predicate(unit) {
			return true
		}
	}
	return false
}

func allUnits(units []UnitContext, predicate func(UnitContext) bool) bool {
	if len(units) == 0 {
		return false
	}
	for _, unit := range units {
		if !predicate(unit) {
			return false
		}
	}
	return true
}

func conditionOp(c Condition, fallback string) string {
	op := strings.TrimSpace(c.ValueCompareAction)
	if op == "" {
		op = strings.TrimSpace(c.Op)
	}
	if op == "" {
		return fallback
	}
	return op
}

func normalizeLogicToken(raw string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func compareNumber(left, right float64, op string) bool {
	switch strings.TrimSpace(op) {
	case ">", "gt":
		return left > right
	case ">=", "gte":
		return left >= right
	case "<", "lt":
		return left < right
	case "<=", "lte":
		return left <= right
	case "=", "==", "eq":
		return left == right
	case "!=", "ne":
		return left != right
	default:
		return false
	}
}

func lookupField(field string, ctx EvalContext) (float64, bool) {
	normalized := strings.ToLower(strings.TrimSpace(field))
	if normalized == "" {
		return 0, false
	}
	if value, ok := ctx.Metrics[normalized]; ok {
		return value, true
	}
	if value, ok := ctx.SkillCooldowns[normalized]; ok {
		return value, true
	}
	switch normalized {
	case "self_hp_percent":
		return ctx.SelfHPPercent, true
	case "hp", "hp_percent":
		return ctx.SelfHPPercent, true
	case "cooldown_ready":
		return ctx.CooldownReady, true
	case "has_buff":
		return ctx.HasBuff, true
	case "target_tag_match":
		return ctx.TargetTagMatch, true
	case "inventory_count":
		return ctx.InventoryCount, true
	case "enemy_count":
		return ctx.EnemyCount, true
	case "distance":
		return ctx.Distance, true
	case "time_window":
		return ctx.TimeWindow, true
	case "hour_of_day":
		now := time.Now()
		return float64(now.Hour()), true
	default:
		return 0, false
	}
}

func lookupStateFlag(stateID string, ctx EvalContext) bool {
	if stateID == "" {
		return false
	}
	return ctx.States[stateID] > 0 || ctx.StateStacks[stateID] > 0
}

func parseConditionNumber(raw any, fallback float64) (float64, bool) {
	if raw == nil {
		return fallback, true
	}
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return fallback, true
		}
		text = strings.TrimSuffix(text, "%")
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	default:
		return 0, false
	}
}

func parseConditionThreshold(raw any, fallback float64, maxValue float64) (float64, bool) {
	if text, ok := raw.(string); ok {
		trimmed := strings.TrimSpace(text)
		if strings.HasSuffix(trimmed, "%") {
			number, ok := parseConditionNumber(trimmed, fallback)
			if !ok {
				return 0, false
			}
			return maxValue * number / 100, true
		}
	}
	return parseConditionNumber(raw, fallback)
}
