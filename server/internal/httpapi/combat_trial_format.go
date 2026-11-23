package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	combatTrialFormatRulesetVersionV1 = "combat-trial-format-v1"
	combatTrialFormatRulesetVersion   = "combat-trial-format-v2"
)

type combatTrialMetricDefinition struct {
	ID                 string   `json:"id"`
	SchemaVersion      int      `json:"schemaVersion"`
	NameKey            string   `json:"nameKey"`
	ShortNameKey       string   `json:"shortNameKey"`
	DescKey            string   `json:"descKey"`
	EventType          string   `json:"eventType"`
	Aggregation        string   `json:"aggregation"`
	ValueSource        string   `json:"valueSource,omitempty"`
	SourceScope        string   `json:"sourceScope"`
	TargetScope        string   `json:"targetScope"`
	Ranked             bool     `json:"ranked,omitempty"`
	RankDirection      string   `json:"rankDirection,omitempty"`
	EligibleEndReasons []string `json:"eligibleEndReasons,omitempty"`
	ValueFormat        string   `json:"valueFormat,omitempty"`
	SortOrder          int      `json:"sortOrder"`
	Enabled            bool     `json:"enabled"`
}

// V1 fields stay in the snapshot shape so queued V1 runs keep their original digest.
type combatTrialDummyDefinition struct {
	Count                int      `json:"count"`
	Positions            []string `json:"positions"`
	DefenseSource        string   `json:"defenseSource"`
	Defense              float64  `json:"defense"`
	HPMode               string   `json:"hpMode"`
	MaxHP                float64  `json:"maxHp"`
	PassiveTurns         bool     `json:"passiveTurns"`
	DamageResist         float64  `json:"damageResist"`
	CriticalDamageResist float64  `json:"criticalDamageResist"`
}

type combatTrialModifierDefinition struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"`
	SourceScope    string  `json:"sourceScope"`
	IncludeSummons bool    `json:"includeSummons"`
	DamageType     string  `json:"damageType"`
	Multiplier     float64 `json:"multiplier"`
}

type combatTrialLimitDefinition struct {
	Kind  string `json:"kind"`
	Value int    `json:"value"`
}

type combatTrialLineupRulesDefinition struct {
	PositionPolicy string `json:"positionPolicy"`
}

type combatTrialEndConditionDefinition struct {
	Kind     string `json:"kind"`
	Reason   string `json:"reason"`
	LabelKey string `json:"labelKey,omitempty"`
}

type combatTrialEncounterDefinition struct {
	Kind             string                                 `json:"kind"`
	Units            []combatTrialEncounterUnitDefinition   `json:"units"`
	Waves            []combatTrialEncounterWaveDefinition   `json:"waves,omitempty"`
	EnemyStatProfile *combatTrialEnemyStatProfileDefinition `json:"enemyStatProfile,omitempty"`
}

type combatTrialEncounterWaveDefinition struct {
	ID               string                                 `json:"id"`
	NameKey          string                                 `json:"nameKey,omitempty"`
	Units            []combatTrialEncounterUnitDefinition   `json:"units"`
	EnemyStatProfile *combatTrialEnemyStatProfileDefinition `json:"enemyStatProfile,omitempty"`
}

type combatTrialEnemyStatProfileDefinition struct {
	DifficultyLevel int                `json:"difficultyLevel"`
	Multiplier      float64            `json:"multiplier"`
	FlatBonuses     map[string]float64 `json:"flatBonuses,omitempty"`
}

type combatTrialInitialBuffDefinition struct {
	BuffID  string             `json:"buffId"`
	Payload map[string]float64 `json:"payload,omitempty"`
}

type combatTrialEncounterUnitDefinition struct {
	ID                   string                             `json:"id"`
	Kind                 string                             `json:"kind"`
	EnemyID              string                             `json:"enemyId,omitempty"`
	NameKey              string                             `json:"nameKey,omitempty"`
	Side                 string                             `json:"side,omitempty"`
	Position             string                             `json:"position"`
	Behavior             string                             `json:"behavior"`
	HealthMode           string                             `json:"healthMode"`
	ObjectiveTarget      bool                               `json:"objectiveTarget"`
	InitialHPRatio       float64                            `json:"initialHpRatio,omitempty"`
	SkipTurnOrder        bool                               `json:"skipTurnOrder,omitempty"`
	InitialBuffs         []combatTrialInitialBuffDefinition `json:"initialBuffs,omitempty"`
	DefenseSource        string                             `json:"defenseSource,omitempty"`
	Defense              float64                            `json:"defense,omitempty"`
	MaxHP                float64                            `json:"maxHp,omitempty"`
	DamageResist         float64                            `json:"damageResist,omitempty"`
	CriticalDamageResist float64                            `json:"criticalDamageResist,omitempty"`
	StatOverrides        map[string]float64                 `json:"statOverrides,omitempty"`
	ResolvedStats        map[string]float64                 `json:"resolvedStats,omitempty"`
	ResolvedUnit         *battleUnitState                   `json:"resolvedUnit,omitempty"`
}

type combatTrialRuleDefinition struct {
	ID      string                            `json:"id"`
	Event   string                            `json:"event"`
	Effects []combatTrialRuleEffectDefinition `json:"effects"`
}

type combatTrialRuleEffectDefinition struct {
	Kind           string   `json:"kind"`
	SourceScope    string   `json:"sourceScope"`
	IncludeSummons bool     `json:"includeSummons"`
	DamageType     string   `json:"damageType"`
	Multiplier     float64  `json:"multiplier"`
	TargetScope    string   `json:"targetScope,omitempty"`
	TargetUnitIDs  []string `json:"targetUnitIds,omitempty"`
	MaxHPRatio     float64  `json:"maxHpRatio,omitempty"`
	NonLethal      bool     `json:"nonLethal,omitempty"`
}

type combatTrialFormatDefinition struct {
	ID               string                              `json:"id"`
	BaseFormatID     string                              `json:"baseFormatId,omitempty"`
	SchemaVersion    int                                 `json:"schemaVersion"`
	RulesetVersion   string                              `json:"rulesetVersion"`
	BalanceRevision  int                                 `json:"balanceRevision"`
	NameKey          string                              `json:"nameKey"`
	ShortDescKey     string                              `json:"shortDescKey"`
	DescKey          string                              `json:"descKey"`
	Icon             string                              `json:"icon"`
	SortOrder        int                                 `json:"sortOrder"`
	Enabled          bool                                `json:"enabled"`
	ActionBudget     int                                 `json:"actionBudget"`
	MetricIDs        []string                            `json:"metricIds"`
	Metrics          []combatTrialMetricDefinition       `json:"metrics"`
	Dummy            combatTrialDummyDefinition          `json:"dummy"`
	Modifiers        []combatTrialModifierDefinition     `json:"modifiers"`
	DefenseK         float64                             `json:"defenseK,omitempty"`
	LineupRules      *combatTrialLineupRulesDefinition   `json:"lineupRules,omitempty"`
	Limits           []combatTrialLimitDefinition        `json:"limits,omitempty"`
	Encounter        combatTrialEncounterDefinition      `json:"encounter,omitempty"`
	EndConditions    []combatTrialEndConditionDefinition `json:"endConditions,omitempty"`
	Triggers         []combatTrialRuleDefinition         `json:"triggers,omitempty"`
	DefinitionDigest string                              `json:"definitionDigest"`
}

func decodeCombatTrialCatalogRow(row map[string]any, target any) error {
	raw, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func normalizeCombatTrialMetric(metric combatTrialMetricDefinition) combatTrialMetricDefinition {
	metric.ID = strings.TrimSpace(metric.ID)
	metric.EventType = strings.TrimSpace(metric.EventType)
	metric.Aggregation = strings.TrimSpace(metric.Aggregation)
	metric.ValueSource = strings.TrimSpace(metric.ValueSource)
	metric.SourceScope = strings.TrimSpace(metric.SourceScope)
	metric.TargetScope = strings.TrimSpace(metric.TargetScope)
	metric.RankDirection = strings.TrimSpace(metric.RankDirection)
	metric.ValueFormat = strings.TrimSpace(metric.ValueFormat)
	if metric.SchemaVersion == 1 {
		metric.Ranked = true
	}
	if metric.RankDirection == "" {
		metric.RankDirection = "desc"
	}
	for index := range metric.EligibleEndReasons {
		metric.EligibleEndReasons[index] = strings.TrimSpace(metric.EligibleEndReasons[index])
	}
	return metric
}

func combatTrialStableID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 64 && normalizeCompetitiveDefinitionID(value) == value
}

func validateCombatTrialMetric(metric combatTrialMetricDefinition) error {
	metric = normalizeCombatTrialMetric(metric)
	if !combatTrialStableID(metric.ID) || (metric.SchemaVersion != 1 && metric.SchemaVersion != 2) ||
		(metric.EventType != "damage" && metric.EventType != "heal" && metric.EventType != "battle_end") ||
		(metric.SourceScope != "player_side" && metric.SourceScope != "enemy_side" && metric.SourceScope != "any") ||
		(metric.TargetScope != "objective_units" && metric.TargetScope != "trial_dummies" && metric.TargetScope != "player_side" && metric.TargetScope != "enemy_side" && metric.TargetScope != "any") ||
		(metric.RankDirection != "asc" && metric.RankDirection != "desc") {
		return fmt.Errorf("invalid combat trial metric %q", metric.ID)
	}
	if metric.EventType == "battle_end" {
		if metric.Aggregation != "final" || (metric.ValueSource != "player_main_actions" &&
			metric.ValueSource != "waves_cleared" && metric.ValueSource != "objective_units_defeated" &&
			metric.ValueSource != "objective_units_remaining_hp") {
			return fmt.Errorf("invalid combat trial result metric %q", metric.ID)
		}
	} else if metric.Aggregation != "sum" && metric.Aggregation != "max" {
		return fmt.Errorf("invalid combat trial event aggregation %q", metric.ID)
	}
	seenReasons := map[string]struct{}{}
	for _, reason := range metric.EligibleEndReasons {
		if !combatTrialStableID(reason) {
			return fmt.Errorf("invalid combat trial metric end reason %q", metric.ID)
		}
		if _, exists := seenReasons[reason]; exists {
			return fmt.Errorf("duplicate combat trial metric end reason %q", metric.ID)
		}
		seenReasons[reason] = struct{}{}
	}
	return nil
}

func combatTrialDefinitionDigest(format combatTrialFormatDefinition) (string, error) {
	format.DefinitionDigest = ""
	raw, err := json.Marshal(format)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func validateCombatTrialModifier(modifier combatTrialModifierDefinition) error {
	if !combatTrialStableID(modifier.ID) || modifier.Kind != "outgoing_damage_multiplier" || modifier.SourceScope != "player_side" ||
		(modifier.DamageType != "physical" && modifier.DamageType != "magical" && modifier.DamageType != "all") ||
		math.IsNaN(modifier.Multiplier) || math.IsInf(modifier.Multiplier, 0) || modifier.Multiplier <= 0 || modifier.Multiplier > 10 {
		return fmt.Errorf("invalid combat trial modifier %q", modifier.ID)
	}
	return nil
}

func validateCombatTrialRules(rules []combatTrialRuleDefinition) error {
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if !combatTrialStableID(rule.ID) || (rule.Event != "damage_calculation" && rule.Event != "player_main_action_completed") || len(rule.Effects) == 0 {
			return fmt.Errorf("invalid combat trial rule %q", rule.ID)
		}
		if _, exists := seen[rule.ID]; exists {
			return fmt.Errorf("combat trial rule duplicated: %s", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		for _, effect := range rule.Effects {
			switch rule.Event {
			case "damage_calculation":
				if effect.Kind != "outgoing_damage_multiplier" || effect.SourceScope != "player_side" ||
					(effect.DamageType != "physical" && effect.DamageType != "magical" && effect.DamageType != "all") ||
					math.IsNaN(effect.Multiplier) || math.IsInf(effect.Multiplier, 0) || effect.Multiplier <= 0 || effect.Multiplier > 10 {
					return fmt.Errorf("invalid combat trial rule effect: %s", rule.ID)
				}
			case "player_main_action_completed":
				if effect.Kind != "max_hp_percent_damage" || effect.TargetScope != "objective_units" ||
					math.IsNaN(effect.MaxHPRatio) || math.IsInf(effect.MaxHPRatio, 0) || effect.MaxHPRatio <= 0 || effect.MaxHPRatio > 1 {
					return fmt.Errorf("invalid combat trial rule effect: %s", rule.ID)
				}
				seenTargets := map[string]struct{}{}
				for _, targetID := range effect.TargetUnitIDs {
					if !combatTrialStableID(targetID) {
						return fmt.Errorf("invalid combat trial rule target: %s", rule.ID)
					}
					if _, exists := seenTargets[targetID]; exists {
						return fmt.Errorf("duplicate combat trial rule target: %s", rule.ID)
					}
					seenTargets[targetID] = struct{}{}
				}
			default:
				return fmt.Errorf("invalid combat trial rule effect: %s", rule.ID)
			}
		}
	}
	return nil
}

func combatTrialPlayerMainActionBudget(format *combatTrialFormatDefinition) int {
	if format == nil {
		return 0
	}
	for _, limit := range format.Limits {
		if limit.Kind == "player_main_actions" {
			return limit.Value
		}
	}
	return format.ActionBudget
}

func combatTrialMetricEligible(metric combatTrialMetricDefinition, endReason string) bool {
	if !metric.Enabled || !metric.Ranked {
		return false
	}
	return combatTrialMetricEndReasonEligible(metric, endReason)
}

func combatTrialMetricEndReasonEligible(metric combatTrialMetricDefinition, endReason string) bool {
	if len(metric.EligibleEndReasons) == 0 {
		return true
	}
	for _, candidate := range metric.EligibleEndReasons {
		if candidate == endReason {
			return true
		}
	}
	return false
}

func validateCombatTrialFormatMetrics(format *combatTrialFormatDefinition) error {
	if format == nil || len(format.MetricIDs) == 0 || len(format.MetricIDs) != len(format.Metrics) {
		return fmt.Errorf("invalid combat trial metric set")
	}
	endReasons := make(map[string]struct{}, len(format.EndConditions))
	for _, condition := range format.EndConditions {
		endReasons[condition.Reason] = struct{}{}
	}
	metricsByID := make(map[string]struct{}, len(format.Metrics))
	rankedCount := 0
	for index := range format.Metrics {
		format.Metrics[index] = normalizeCombatTrialMetric(format.Metrics[index])
		metric := format.Metrics[index]
		if err := validateCombatTrialMetric(metric); err != nil || !metric.Enabled {
			if err != nil {
				return err
			}
			return fmt.Errorf("disabled combat trial metric in format: %s.%s", format.ID, metric.ID)
		}
		if _, exists := metricsByID[metric.ID]; exists {
			return fmt.Errorf("duplicate combat trial metric snapshot: %s.%s", format.ID, metric.ID)
		}
		metricsByID[metric.ID] = struct{}{}
		if metric.Ranked {
			rankedCount++
		}
		eligibleForFormat := len(metric.EligibleEndReasons) == 0
		for _, reason := range metric.EligibleEndReasons {
			if _, exists := endReasons[reason]; exists {
				eligibleForFormat = true
			}
		}
		if metric.Ranked && !eligibleForFormat {
			return fmt.Errorf("combat trial metric has no eligible end reason: %s.%s", format.ID, metric.ID)
		}
	}
	if rankedCount == 0 {
		return fmt.Errorf("combat trial format has no ranked metric: %s", format.ID)
	}
	seenMetricIDs := make(map[string]struct{}, len(format.MetricIDs))
	for index, rawMetricID := range format.MetricIDs {
		metricID := strings.TrimSpace(rawMetricID)
		format.MetricIDs[index] = metricID
		if !combatTrialStableID(metricID) {
			return fmt.Errorf("invalid combat trial format metric id: %s.%s", format.ID, metricID)
		}
		if _, exists := seenMetricIDs[metricID]; exists {
			return fmt.Errorf("combat trial format metric duplicated: %s.%s", format.ID, metricID)
		}
		if _, exists := metricsByID[metricID]; !exists {
			return fmt.Errorf("combat trial format metric missing from snapshot: %s.%s", format.ID, metricID)
		}
		seenMetricIDs[metricID] = struct{}{}
	}
	return nil
}

func mergeCombatTrialFormat(base, child combatTrialFormatDefinition) combatTrialFormatDefinition {
	if len(child.MetricIDs) == 0 {
		child.MetricIDs = append([]string(nil), base.MetricIDs...)
	}
	if child.LineupRules == nil && base.LineupRules != nil {
		inherited := *base.LineupRules
		child.LineupRules = &inherited
	}
	if len(child.Limits) == 0 {
		child.Limits = append([]combatTrialLimitDefinition(nil), base.Limits...)
	}
	if child.Encounter.Kind == "" {
		child.Encounter = base.Encounter
		child.Encounter.EnemyStatProfile = cloneCombatTrialEnemyStatProfile(base.Encounter.EnemyStatProfile)
		child.Encounter.Units = append([]combatTrialEncounterUnitDefinition(nil), base.Encounter.Units...)
		for index := range child.Encounter.Units {
			child.Encounter.Units[index].InitialBuffs = cloneCombatTrialInitialBuffs(base.Encounter.Units[index].InitialBuffs)
			child.Encounter.Units[index].StatOverrides = cloneFloatMap(base.Encounter.Units[index].StatOverrides)
			child.Encounter.Units[index].ResolvedStats = cloneFloatMap(base.Encounter.Units[index].ResolvedStats)
			if base.Encounter.Units[index].ResolvedUnit == nil {
				continue
			}
			resolved := *base.Encounter.Units[index].ResolvedUnit
			child.Encounter.Units[index].ResolvedUnit = &resolved
		}
		child.Encounter.Waves = make([]combatTrialEncounterWaveDefinition, len(base.Encounter.Waves))
		for waveIndex := range base.Encounter.Waves {
			child.Encounter.Waves[waveIndex] = base.Encounter.Waves[waveIndex]
			child.Encounter.Waves[waveIndex].EnemyStatProfile = cloneCombatTrialEnemyStatProfile(base.Encounter.Waves[waveIndex].EnemyStatProfile)
			child.Encounter.Waves[waveIndex].Units = append([]combatTrialEncounterUnitDefinition(nil), base.Encounter.Waves[waveIndex].Units...)
			for unitIndex := range child.Encounter.Waves[waveIndex].Units {
				child.Encounter.Waves[waveIndex].Units[unitIndex].InitialBuffs = cloneCombatTrialInitialBuffs(base.Encounter.Waves[waveIndex].Units[unitIndex].InitialBuffs)
				child.Encounter.Waves[waveIndex].Units[unitIndex].StatOverrides = cloneFloatMap(base.Encounter.Waves[waveIndex].Units[unitIndex].StatOverrides)
				child.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedStats = cloneFloatMap(base.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedStats)
				resolvedUnit := base.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedUnit
				if resolvedUnit == nil {
					continue
				}
				resolved := *resolvedUnit
				child.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedUnit = &resolved
			}
		}
	}
	if len(child.EndConditions) == 0 {
		child.EndConditions = append([]combatTrialEndConditionDefinition(nil), base.EndConditions...)
	}
	child.Triggers = append(append([]combatTrialRuleDefinition(nil), base.Triggers...), child.Triggers...)
	child.Modifiers = append(append([]combatTrialModifierDefinition(nil), base.Modifiers...), child.Modifiers...)
	return child
}

func cloneCombatTrialEnemyStatProfile(profile *combatTrialEnemyStatProfileDefinition) *combatTrialEnemyStatProfileDefinition {
	if profile == nil {
		return nil
	}
	cloned := *profile
	cloned.FlatBonuses = cloneFloatMap(profile.FlatBonuses)
	return &cloned
}

func cloneCombatTrialInitialBuffs(initialBuffs []combatTrialInitialBuffDefinition) []combatTrialInitialBuffDefinition {
	if len(initialBuffs) == 0 {
		return nil
	}
	cloned := make([]combatTrialInitialBuffDefinition, len(initialBuffs))
	for index := range initialBuffs {
		cloned[index] = initialBuffs[index]
		cloned[index].Payload = cloneFloatMap(initialBuffs[index].Payload)
	}
	return cloned
}

func validateCombatTrialLineupRules(format *combatTrialFormatDefinition) error {
	if format == nil || format.LineupRules == nil {
		return fmt.Errorf("invalid combat trial lineup rules")
	}
	format.LineupRules.PositionPolicy = strings.TrimSpace(format.LineupRules.PositionPolicy)
	switch format.LineupRules.PositionPolicy {
	case "shared", "distinct":
		return nil
	default:
		return fmt.Errorf("invalid combat trial position policy: %s.%s", format.ID, format.LineupRules.PositionPolicy)
	}
}

func combatTrialEncounterUnitSide(unit combatTrialEncounterUnitDefinition) string {
	if side := strings.ToLower(strings.TrimSpace(unit.Side)); side != "" {
		return side
	}
	return "enemy"
}

func combatTrialEncounterRuntimeUnitID(unit combatTrialEncounterUnitDefinition) string {
	prefix := "combat_trial:opponent:"
	if combatTrialEncounterUnitSide(unit) == "player" {
		prefix = "combat_trial:ally:"
	}
	return prefix + strings.TrimSpace(unit.ID)
}

func combatTrialEncounterUnitPointers(encounter *combatTrialEncounterDefinition) []*combatTrialEncounterUnitDefinition {
	if encounter == nil {
		return nil
	}
	units := make([]*combatTrialEncounterUnitDefinition, 0, len(encounter.Units))
	for index := range encounter.Units {
		units = append(units, &encounter.Units[index])
	}
	for waveIndex := range encounter.Waves {
		for unitIndex := range encounter.Waves[waveIndex].Units {
			units = append(units, &encounter.Waves[waveIndex].Units[unitIndex])
		}
	}
	return units
}

func combatTrialEncounterUnitDefinitions(encounter combatTrialEncounterDefinition) []combatTrialEncounterUnitDefinition {
	units := append([]combatTrialEncounterUnitDefinition(nil), encounter.Units...)
	for _, wave := range encounter.Waves {
		units = append(units, wave.Units...)
	}
	return units
}

func validateCombatTrialEnemyStatProfile(profile *combatTrialEnemyStatProfileDefinition) error {
	if profile == nil {
		return nil
	}
	if profile.DifficultyLevel < 1 || profile.DifficultyLevel > 1000 ||
		math.IsNaN(profile.Multiplier) || math.IsInf(profile.Multiplier, 0) || profile.Multiplier <= 0 || profile.Multiplier > 1000 {
		return fmt.Errorf("invalid combat trial enemy stat profile")
	}
	for rawKey, value := range profile.FlatBonuses {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key != rawKey || !isCombatStat(key) || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e12 {
			return fmt.Errorf("invalid combat trial enemy stat profile")
		}
	}
	return nil
}

func applyCombatTrialEnemyStatProfile(unit *battleUnitState, profile *combatTrialEnemyStatProfileDefinition) {
	if unit == nil || profile == nil {
		return
	}
	apply := func(stats map[string]float64) {
		for key, value := range stats {
			if isCombatStat(key) {
				stats[key] = roundTo4(value * profile.Multiplier)
			}
		}
		for key, bonus := range profile.FlatBonuses {
			stats[key] = roundTo4(stats[key] + bonus)
		}
	}
	apply(unit.Stats)
	apply(unit.BaseStats)
	unit.MaxHP = maxFloat(1, unit.Stats["hp"])
	unit.MaxMP = maxFloat(0, unit.Stats["mp"])
	unit.HP = unit.MaxHP
	unit.MP = unit.MaxMP
}

func validateCombatTrialEnemyStatOverrides(overrides map[string]float64) error {
	for rawKey, value := range overrides {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key != rawKey || !isCombatStat(key) || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e12 {
			return fmt.Errorf("invalid combat trial enemy stat overrides")
		}
		if key == "hp" && value <= 0 {
			return fmt.Errorf("invalid combat trial enemy stat overrides")
		}
		if (key == "mp" || key == "hp_regen" || key == "mp_regen") && value < 0 {
			return fmt.Errorf("invalid combat trial enemy stat overrides")
		}
		if (key == "crit_damage_resist" || key == "physics_damage_resist" || key == "magical_damage_resist") && (value < 0 || value > 1) {
			return fmt.Errorf("invalid combat trial enemy stat overrides")
		}
	}
	return nil
}

func validateCombatTrialInitialBuffPayload(initialBuffs []combatTrialInitialBuffDefinition) error {
	seen := make(map[string]struct{}, len(initialBuffs))
	for index := range initialBuffs {
		initialBuffs[index].BuffID = strings.ToLower(strings.TrimSpace(initialBuffs[index].BuffID))
		buffID := initialBuffs[index].BuffID
		if !combatTrialStableID(buffID) {
			return fmt.Errorf("invalid combat trial initial buff")
		}
		if _, exists := seen[buffID]; exists {
			return fmt.Errorf("duplicate combat trial initial buff")
		}
		seen[buffID] = struct{}{}
		for key, value := range initialBuffs[index].Payload {
			if !combatTrialStableID(key) || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e12 {
				return fmt.Errorf("invalid combat trial initial buff payload")
			}
		}
	}
	return nil
}

func (s *Server) combatTrialInitialBuffDuration(buffID string) (int64, bool) {
	buffID = strings.ToLower(strings.TrimSpace(buffID))
	for _, raw := range s.deps.GameData.Buff {
		if strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw["id"]))) != buffID || parseBooleanLoose(raw["disabled"], false) {
			continue
		}
		if parseBooleanLoose(raw["permanent"], false) {
			return -1, true
		}
		duration := toInt64Loose(raw["durationTurns"], toInt64Loose(raw["duration"], 0))
		return duration, duration > 0
	}
	return 0, false
}

func (s *Server) resolveCombatTrialInitialBuffs(formatID string, unit *combatTrialEncounterUnitDefinition) error {
	if unit == nil || len(unit.InitialBuffs) == 0 {
		return nil
	}
	if err := validateCombatTrialInitialBuffPayload(unit.InitialBuffs); err != nil {
		return fmt.Errorf("invalid combat trial initial buffs: %s.%s", formatID, unit.ID)
	}
	for _, initialBuff := range unit.InitialBuffs {
		duration, exists := s.combatTrialInitialBuffDuration(initialBuff.BuffID)
		if !exists || (unit.Kind == "scripted_ally" && duration >= 0) {
			return fmt.Errorf("invalid combat trial initial buff: %s.%s.%s", formatID, unit.ID, initialBuff.BuffID)
		}
	}
	return nil
}

func applyCombatTrialEnemyStatOverrides(unit *battleUnitState, overrides map[string]float64) {
	if unit == nil || len(overrides) == 0 {
		return
	}
	for key, value := range overrides {
		value = roundTo4(value)
		unit.Stats[key] = value
		unit.BaseStats[key] = value
	}
	unit.MaxHP = maxFloat(1, unit.Stats["hp"])
	unit.MaxMP = maxFloat(0, unit.Stats["mp"])
	unit.HP = unit.MaxHP
	unit.MP = unit.MaxMP
}

func (s *Server) resolveCombatTrialEncounterUnit(
	format *combatTrialFormatDefinition,
	unit *combatTrialEncounterUnitDefinition,
	enemyStatProfile *combatTrialEnemyStatProfileDefinition,
	enemyCatalog map[string]battleEnemyConfig,
	seenIDs map[string]struct{},
) error {
	unit.ID = strings.TrimSpace(unit.ID)
	unit.Kind = strings.TrimSpace(unit.Kind)
	unit.EnemyID = strings.TrimSpace(unit.EnemyID)
	unit.NameKey = strings.TrimSpace(unit.NameKey)
	unit.Side = strings.ToLower(strings.TrimSpace(unit.Side))
	unit.Position = normalizeCompetitivePosition(unit.Position)
	unit.Behavior = strings.TrimSpace(unit.Behavior)
	unit.HealthMode = strings.TrimSpace(unit.HealthMode)
	if err := s.resolveCombatTrialInitialBuffs(format.ID, unit); err != nil {
		return err
	}
	if !combatTrialStableID(unit.ID) || unit.Position == "" ||
		(unit.Side != "" && unit.Side != "player" && unit.Side != "enemy") ||
		(unit.Behavior != "passive" && unit.Behavior != "active") ||
		(unit.HealthMode != "normal" && unit.HealthMode != "immutable_full") ||
		math.IsNaN(unit.InitialHPRatio) || math.IsInf(unit.InitialHPRatio, 0) || unit.InitialHPRatio < 0 || unit.InitialHPRatio > 1 ||
		math.IsNaN(unit.DamageResist) || math.IsInf(unit.DamageResist, 0) || unit.DamageResist < 0 || unit.DamageResist > 1 ||
		math.IsNaN(unit.CriticalDamageResist) || math.IsInf(unit.CriticalDamageResist, 0) || unit.CriticalDamageResist < 0 || unit.CriticalDamageResist > 1 {
		return fmt.Errorf("invalid combat trial encounter unit: %s.%s", format.ID, unit.ID)
	}
	if err := validateCombatTrialEnemyStatOverrides(unit.StatOverrides); err != nil {
		return fmt.Errorf("invalid combat trial encounter unit stats: %s.%s", format.ID, unit.ID)
	}
	if _, exists := seenIDs[unit.ID]; exists {
		return fmt.Errorf("combat trial encounter unit duplicated: %s.%s", format.ID, unit.ID)
	}
	seenIDs[unit.ID] = struct{}{}
	switch unit.Kind {
	case "training_target":
		if combatTrialEncounterUnitSide(*unit) != "enemy" || unit.EnemyID != "" || unit.Behavior != "passive" ||
			unit.HealthMode != "immutable_full" || unit.MaxHP <= 0 || unit.InitialHPRatio != 0 || unit.SkipTurnOrder ||
			unit.DefenseSource != "combat_balance_defense_k" || len(unit.StatOverrides) != 0 {
			return fmt.Errorf("invalid combat trial training target: %s.%s", format.ID, unit.ID)
		}
		unit.Defense = format.DefenseK
		unit.ResolvedStats = nil
		unit.ResolvedUnit = nil
	case "scripted_ally":
		if combatTrialEncounterUnitSide(*unit) != "player" || unit.EnemyID != "" || unit.NameKey == "" || unit.Behavior != "passive" ||
			unit.HealthMode != "normal" || unit.MaxHP <= 0 || unit.InitialHPRatio <= 0 || !unit.SkipTurnOrder || len(unit.StatOverrides) != 0 {
			return fmt.Errorf("invalid combat trial scripted ally: %s.%s", format.ID, unit.ID)
		}
		if unit.DefenseSource != "" && unit.DefenseSource != "combat_balance_defense_k" {
			return fmt.Errorf("invalid combat trial scripted ally defense: %s.%s", format.ID, unit.ID)
		}
		if unit.DefenseSource == "combat_balance_defense_k" {
			unit.Defense = format.DefenseK
		}
		unit.ResolvedStats = nil
		unit.ResolvedUnit = nil
	case "authored_enemy":
		if combatTrialEncounterUnitSide(*unit) != "enemy" || unit.EnemyID == "" || unit.HealthMode != "normal" ||
			unit.InitialHPRatio != 0 || unit.SkipTurnOrder {
			return fmt.Errorf("invalid combat trial authored enemy: %s.%s", format.ID, unit.ID)
		}
		cfg, ok := enemyCatalog[unit.EnemyID]
		if !ok {
			return fmt.Errorf("combat trial enemy missing: %s.%s", format.ID, unit.EnemyID)
		}
		resolved := s.buildBattleEnemyUnit(cfg, combatTrialEncounterRuntimeUnitID(*unit), false, s.battleDefaultBasicSkillID())
		if enemyStatProfile != nil {
			s.applyEnemyHpRegenRatio(&resolved)
			applyCombatTrialEnemyStatProfile(&resolved, enemyStatProfile)
		}
		applyCombatTrialEnemyStatOverrides(&resolved, unit.StatOverrides)
		resolved.Position = unit.Position
		resolved.BasePosition = unit.Position
		resolved.RewardEligible = false
		unit.NameKey = resolved.NameKey
		unit.MaxHP = resolved.MaxHP
		unit.ResolvedStats = cloneFloatMap(resolved.Stats)
		unit.ResolvedUnit = &resolved
	default:
		return fmt.Errorf("unsupported combat trial encounter unit kind: %s.%s", format.ID, unit.Kind)
	}
	return nil
}

func (s *Server) resolveCombatTrialEncounter(format *combatTrialFormatDefinition) error {
	if format == nil {
		return fmt.Errorf("invalid combat trial encounter")
	}
	if err := validateCombatTrialEnemyStatProfile(format.Encounter.EnemyStatProfile); err != nil {
		return fmt.Errorf("invalid combat trial encounter profile: %s", format.ID)
	}
	enemyCatalog := s.battleEnemyCatalog()
	seenIDs := map[string]struct{}{}
	objectiveCount := 0
	validateRoster := func(units []combatTrialEncounterUnitDefinition, label string, enemyStatProfile *combatTrialEnemyStatProfileDefinition) error {
		if len(units) == 0 || len(units) > 12 {
			return fmt.Errorf("invalid combat trial encounter roster: %s.%s", format.ID, label)
		}
		rosterObjectives := 0
		enemyCount := 0
		authoredEnemyCount := 0
		for index := range units {
			if err := s.resolveCombatTrialEncounterUnit(format, &units[index], enemyStatProfile, enemyCatalog, seenIDs); err != nil {
				return err
			}
			if units[index].ObjectiveTarget {
				objectiveCount++
				rosterObjectives++
			}
			if combatTrialEncounterUnitSide(units[index]) == "enemy" {
				enemyCount++
			}
			if units[index].Kind == "authored_enemy" {
				authoredEnemyCount++
			}
		}
		if rosterObjectives == 0 || enemyCount == 0 {
			return fmt.Errorf("combat trial encounter roster has no objective or enemy: %s.%s", format.ID, label)
		}
		if enemyStatProfile != nil && authoredEnemyCount == 0 {
			return fmt.Errorf("combat trial encounter profile has no authored enemy: %s.%s", format.ID, label)
		}
		return nil
	}
	switch format.Encounter.Kind {
	case "unit_roster":
		if len(format.Encounter.Waves) != 0 {
			return fmt.Errorf("invalid combat trial encounter: %s", format.ID)
		}
		if err := validateRoster(format.Encounter.Units, "units", format.Encounter.EnemyStatProfile); err != nil {
			return err
		}
	case "wave_roster":
		if len(format.Encounter.Units) != 0 || len(format.Encounter.Waves) == 0 || len(format.Encounter.Waves) > 20 {
			return fmt.Errorf("invalid combat trial wave encounter: %s", format.ID)
		}
		seenWaves := map[string]struct{}{}
		totalUnits := 0
		for waveIndex := range format.Encounter.Waves {
			wave := &format.Encounter.Waves[waveIndex]
			wave.ID = strings.TrimSpace(wave.ID)
			wave.NameKey = strings.TrimSpace(wave.NameKey)
			if !combatTrialStableID(wave.ID) {
				return fmt.Errorf("invalid combat trial encounter wave: %s", format.ID)
			}
			if _, exists := seenWaves[wave.ID]; exists {
				return fmt.Errorf("combat trial encounter wave duplicated: %s.%s", format.ID, wave.ID)
			}
			seenWaves[wave.ID] = struct{}{}
			if err := validateCombatTrialEnemyStatProfile(wave.EnemyStatProfile); err != nil {
				return fmt.Errorf("invalid combat trial encounter wave profile: %s.%s", format.ID, wave.ID)
			}
			enemyStatProfile := wave.EnemyStatProfile
			if enemyStatProfile == nil {
				enemyStatProfile = format.Encounter.EnemyStatProfile
			}
			if err := validateRoster(wave.Units, wave.ID, enemyStatProfile); err != nil {
				return err
			}
			totalUnits += len(wave.Units)
		}
		if totalUnits > 60 {
			return fmt.Errorf("combat trial wave encounter is too large: %s", format.ID)
		}
	default:
		return fmt.Errorf("invalid combat trial encounter: %s", format.ID)
	}
	if objectiveCount == 0 {
		return fmt.Errorf("combat trial encounter has no objective targets: %s", format.ID)
	}
	return nil
}

func validateCombatTrialEncounterUnitSnapshot(format *combatTrialFormatDefinition, unit *combatTrialEncounterUnitDefinition, seenIDs map[string]struct{}) error {
	if err := validateCombatTrialInitialBuffPayload(unit.InitialBuffs); err != nil {
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	if !combatTrialStableID(unit.ID) || normalizeCompetitivePosition(unit.Position) != unit.Position ||
		(unit.Side != "" && unit.Side != "player" && unit.Side != "enemy") ||
		(unit.Behavior != "passive" && unit.Behavior != "active") ||
		(unit.HealthMode != "normal" && unit.HealthMode != "immutable_full") ||
		math.IsNaN(unit.InitialHPRatio) || math.IsInf(unit.InitialHPRatio, 0) || unit.InitialHPRatio < 0 || unit.InitialHPRatio > 1 ||
		math.IsNaN(unit.DamageResist) || math.IsInf(unit.DamageResist, 0) || unit.DamageResist < 0 || unit.DamageResist > 1 ||
		math.IsNaN(unit.CriticalDamageResist) || math.IsInf(unit.CriticalDamageResist, 0) || unit.CriticalDamageResist < 0 || unit.CriticalDamageResist > 1 {
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	if err := validateCombatTrialEnemyStatOverrides(unit.StatOverrides); err != nil {
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	if _, exists := seenIDs[unit.ID]; exists {
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	seenIDs[unit.ID] = struct{}{}
	switch unit.Kind {
	case "training_target":
		if combatTrialEncounterUnitSide(*unit) != "enemy" || unit.EnemyID != "" || unit.Behavior != "passive" || unit.HealthMode != "immutable_full" ||
			unit.InitialHPRatio != 0 || unit.SkipTurnOrder || unit.DefenseSource != "combat_balance_defense_k" || unit.Defense != format.DefenseK ||
			math.IsNaN(unit.MaxHP) || math.IsInf(unit.MaxHP, 0) || unit.MaxHP <= 0 || len(unit.StatOverrides) != 0 || len(unit.ResolvedStats) != 0 || unit.ResolvedUnit != nil {
			return fmt.Errorf("invalid combat trial training target snapshot")
		}
	case "scripted_ally":
		if combatTrialEncounterUnitSide(*unit) != "player" || unit.EnemyID != "" || unit.NameKey == "" || unit.Behavior != "passive" ||
			unit.HealthMode != "normal" || unit.InitialHPRatio <= 0 || !unit.SkipTurnOrder ||
			math.IsNaN(unit.MaxHP) || math.IsInf(unit.MaxHP, 0) || unit.MaxHP <= 0 || len(unit.StatOverrides) != 0 || len(unit.ResolvedStats) != 0 || unit.ResolvedUnit != nil ||
			(unit.DefenseSource != "" && (unit.DefenseSource != "combat_balance_defense_k" || unit.Defense != format.DefenseK)) {
			return fmt.Errorf("invalid combat trial scripted ally snapshot")
		}
	case "authored_enemy":
		resolved := unit.ResolvedUnit
		if combatTrialEncounterUnitSide(*unit) != "enemy" || unit.EnemyID == "" || unit.HealthMode != "normal" ||
			unit.InitialHPRatio != 0 || unit.SkipTurnOrder || resolved == nil ||
			resolved.UnitID != combatTrialEncounterRuntimeUnitID(*unit) || resolved.EnemyID != unit.EnemyID ||
			resolved.Side != "enemy" || resolved.Position != unit.Position || resolved.BasePosition != unit.Position ||
			resolved.RewardEligible || resolved.MaxHP <= 0 || unit.MaxHP != resolved.MaxHP ||
			(len(unit.ResolvedStats) > 0 && !combatTrialStatMapsEqual(unit.ResolvedStats, resolved.Stats)) {
			return fmt.Errorf("invalid combat trial authored enemy snapshot")
		}
	default:
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	return nil
}

func combatTrialStatMapsEqual(left, right map[string]float64) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if rightValue, ok := right[key]; !ok || value != rightValue {
			return false
		}
	}
	return true
}

func validateCombatTrialEncounterSnapshot(format *combatTrialFormatDefinition) error {
	if format == nil {
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	if err := validateCombatTrialEnemyStatProfile(format.Encounter.EnemyStatProfile); err != nil {
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	seenIDs := map[string]struct{}{}
	objectiveCount := 0
	validateRoster := func(units []combatTrialEncounterUnitDefinition, enemyStatProfile *combatTrialEnemyStatProfileDefinition) error {
		if len(units) == 0 || len(units) > 12 {
			return fmt.Errorf("invalid combat trial encounter snapshot")
		}
		rosterObjectives := 0
		enemyCount := 0
		authoredEnemyCount := 0
		for index := range units {
			if err := validateCombatTrialEncounterUnitSnapshot(format, &units[index], seenIDs); err != nil {
				return err
			}
			if units[index].ObjectiveTarget {
				objectiveCount++
				rosterObjectives++
			}
			if combatTrialEncounterUnitSide(units[index]) == "enemy" {
				enemyCount++
			}
			if units[index].Kind == "authored_enemy" {
				authoredEnemyCount++
			}
		}
		if rosterObjectives == 0 || enemyCount == 0 || (enemyStatProfile != nil && authoredEnemyCount == 0) {
			return fmt.Errorf("invalid combat trial encounter snapshot")
		}
		return nil
	}
	switch format.Encounter.Kind {
	case "unit_roster":
		if len(format.Encounter.Waves) != 0 || validateRoster(format.Encounter.Units, format.Encounter.EnemyStatProfile) != nil {
			return fmt.Errorf("invalid combat trial encounter snapshot")
		}
	case "wave_roster":
		if len(format.Encounter.Units) != 0 || len(format.Encounter.Waves) == 0 || len(format.Encounter.Waves) > 20 {
			return fmt.Errorf("invalid combat trial encounter snapshot")
		}
		seenWaves := map[string]struct{}{}
		totalUnits := 0
		for _, wave := range format.Encounter.Waves {
			if !combatTrialStableID(wave.ID) {
				return fmt.Errorf("invalid combat trial encounter snapshot")
			}
			if err := validateCombatTrialEnemyStatProfile(wave.EnemyStatProfile); err != nil {
				return fmt.Errorf("invalid combat trial encounter snapshot")
			}
			if _, exists := seenWaves[wave.ID]; exists {
				return fmt.Errorf("invalid combat trial encounter snapshot")
			}
			seenWaves[wave.ID] = struct{}{}
			enemyStatProfile := wave.EnemyStatProfile
			if enemyStatProfile == nil {
				enemyStatProfile = format.Encounter.EnemyStatProfile
			}
			if err := validateRoster(wave.Units, enemyStatProfile); err != nil {
				return err
			}
			totalUnits += len(wave.Units)
		}
		if totalUnits > 60 {
			return fmt.Errorf("invalid combat trial encounter snapshot")
		}
	default:
		return fmt.Errorf("invalid combat trial encounter snapshot")
	}
	if objectiveCount == 0 {
		return fmt.Errorf("combat trial encounter snapshot has no objective targets")
	}
	return nil
}

func validateCombatTrialLimitsAndEndConditions(format *combatTrialFormatDefinition) error {
	seenLimits := map[string]struct{}{}
	for _, limit := range format.Limits {
		if limit.Kind != "player_main_actions" || limit.Value <= 0 || limit.Value > 1000 {
			return fmt.Errorf("invalid combat trial limit: %s.%s", format.ID, limit.Kind)
		}
		if _, exists := seenLimits[limit.Kind]; exists {
			return fmt.Errorf("combat trial limit duplicated: %s.%s", format.ID, limit.Kind)
		}
		seenLimits[limit.Kind] = struct{}{}
	}
	format.ActionBudget = combatTrialPlayerMainActionBudget(format)
	if format.ActionBudget <= 0 {
		return fmt.Errorf("combat trial player action limit missing: %s", format.ID)
	}
	seenConditions := map[string]struct{}{}
	seenReasons := map[string]struct{}{}
	for index := range format.EndConditions {
		condition := &format.EndConditions[index]
		condition.Kind = strings.TrimSpace(condition.Kind)
		condition.Reason = strings.TrimSpace(condition.Reason)
		condition.LabelKey = strings.TrimSpace(condition.LabelKey)
		if condition.LabelKey == "" {
			switch condition.Reason {
			case "team_defeated":
				condition.LabelKey = "game.combatTrial.endReason.teamDefeated"
			case "enemy_team_defeated":
				condition.LabelKey = "game.combatTrial.endReason.enemyTeamDefeated"
			case "action_budget_reached":
				condition.LabelKey = "game.combatTrial.endReason.actionBudgetReached"
			case "all_waves_cleared":
				condition.LabelKey = "game.combatTrial.endReason.allWavesCleared"
			}
		}
		if (condition.Kind != "player_team_defeated" && condition.Kind != "enemy_team_defeated" &&
			condition.Kind != "player_main_actions_reached" && condition.Kind != "encounter_completed") || !combatTrialStableID(condition.Reason) {
			return fmt.Errorf("invalid combat trial end condition: %s.%s", format.ID, condition.Kind)
		}
		if condition.LabelKey == "" {
			return fmt.Errorf("combat trial end condition label missing: %s.%s", format.ID, condition.Kind)
		}
		if _, exists := seenConditions[condition.Kind]; exists {
			return fmt.Errorf("combat trial end condition duplicated: %s.%s", format.ID, condition.Kind)
		}
		if _, exists := seenReasons[condition.Reason]; exists {
			return fmt.Errorf("combat trial end reason duplicated: %s.%s", format.ID, condition.Reason)
		}
		seenConditions[condition.Kind] = struct{}{}
		seenReasons[condition.Reason] = struct{}{}
	}
	if _, ok := seenConditions["player_main_actions_reached"]; !ok {
		return fmt.Errorf("combat trial action limit end condition missing: %s", format.ID)
	}
	if _, ok := seenConditions["player_team_defeated"]; !ok {
		return fmt.Errorf("combat trial player defeat end condition missing: %s", format.ID)
	}
	if format.Encounter.Kind == "wave_roster" {
		if _, ok := seenConditions["encounter_completed"]; !ok {
			return fmt.Errorf("combat trial encounter completion condition missing: %s", format.ID)
		}
		return nil
	}
	allEnemiesDefeatable := false
	for _, unit := range combatTrialEncounterUnitDefinitions(format.Encounter) {
		if combatTrialEncounterUnitSide(unit) != "enemy" {
			continue
		}
		if unit.HealthMode != "normal" {
			allEnemiesDefeatable = false
			break
		}
		allEnemiesDefeatable = true
	}
	if allEnemiesDefeatable {
		if _, ok := seenConditions["enemy_team_defeated"]; !ok {
			return fmt.Errorf("combat trial enemy defeat end condition missing: %s", format.ID)
		}
	}
	return nil
}

func validateCombatTrialRuleTargets(format *combatTrialFormatDefinition) error {
	if format == nil {
		return fmt.Errorf("invalid combat trial rule targets")
	}
	objectives := map[string]struct{}{}
	for _, unit := range combatTrialEncounterUnitDefinitions(format.Encounter) {
		if unit.ObjectiveTarget {
			objectives[unit.ID] = struct{}{}
		}
	}
	for _, rule := range format.Triggers {
		for _, effect := range rule.Effects {
			for _, targetID := range effect.TargetUnitIDs {
				if _, exists := objectives[targetID]; !exists {
					return fmt.Errorf("combat trial rule target is not an objective: %s.%s.%s", format.ID, rule.ID, targetID)
				}
			}
		}
	}
	return nil
}

func (s *Server) finalizeCombatTrialFormat(format combatTrialFormatDefinition, metricsByID map[string]combatTrialMetricDefinition) (combatTrialFormatDefinition, error) {
	if !combatTrialStableID(format.ID) || format.SchemaVersion != 2 || format.RulesetVersion != combatTrialFormatRulesetVersion || format.BalanceRevision <= 0 {
		return combatTrialFormatDefinition{}, fmt.Errorf("combat trial format unavailable: %s", format.ID)
	}
	defenseK := s.deps.Config.SharedGlobalSettings.CombatBalanceModel.DefenseK
	if defenseK <= 0 {
		defenseK = 100
	}
	format.DefenseK = roundTo4(defenseK)
	if err := validateCombatTrialLineupRules(&format); err != nil {
		return combatTrialFormatDefinition{}, err
	}
	if err := validateCombatTrialLimitsAndEndConditions(&format); err != nil {
		return combatTrialFormatDefinition{}, err
	}
	if err := s.resolveCombatTrialEncounter(&format); err != nil {
		return combatTrialFormatDefinition{}, err
	}
	if err := validateCombatTrialRules(format.Triggers); err != nil {
		return combatTrialFormatDefinition{}, err
	}
	if err := validateCombatTrialRuleTargets(&format); err != nil {
		return combatTrialFormatDefinition{}, err
	}
	seenModifiers := make(map[string]struct{}, len(format.Modifiers))
	for _, modifier := range format.Modifiers {
		if err := validateCombatTrialModifier(modifier); err != nil {
			return combatTrialFormatDefinition{}, err
		}
		if _, exists := seenModifiers[modifier.ID]; exists {
			return combatTrialFormatDefinition{}, fmt.Errorf("combat trial modifier duplicated: %s.%s", format.ID, modifier.ID)
		}
		seenModifiers[modifier.ID] = struct{}{}
	}
	format.Metrics = make([]combatTrialMetricDefinition, 0, len(format.MetricIDs))
	seenMetrics := map[string]struct{}{}
	for index, rawMetricID := range format.MetricIDs {
		metricID := strings.TrimSpace(rawMetricID)
		format.MetricIDs[index] = metricID
		metric, ok := metricsByID[metricID]
		if !ok || !metric.Enabled {
			return combatTrialFormatDefinition{}, fmt.Errorf("combat trial format metric missing: %s.%s", format.ID, metricID)
		}
		if _, exists := seenMetrics[metricID]; exists {
			return combatTrialFormatDefinition{}, fmt.Errorf("combat trial format metric duplicated: %s.%s", format.ID, metricID)
		}
		seenMetrics[metricID] = struct{}{}
		format.Metrics = append(format.Metrics, metric)
	}
	if len(format.Metrics) == 0 {
		return combatTrialFormatDefinition{}, fmt.Errorf("combat trial format metrics missing: %s", format.ID)
	}
	if err := validateCombatTrialFormatMetrics(&format); err != nil {
		return combatTrialFormatDefinition{}, err
	}
	if format.ID == combatTrialDefaultFormatID {
		if format.ActionBudget != 20 || len(format.Encounter.Units) != 3 {
			return combatTrialFormatDefinition{}, fmt.Errorf("invalid standard combat trial contract")
		}
		for _, metricID := range []string{"damage_total", "damage_max_segment"} {
			if _, ok := seenMetrics[metricID]; !ok {
				return combatTrialFormatDefinition{}, fmt.Errorf("standard combat trial metric missing: %s", metricID)
			}
		}
		positions := []string{"vanguard", "midguard", "rearguard"}
		for index, unit := range format.Encounter.Units {
			if unit.Kind != "training_target" || unit.Position != positions[index] || !unit.ObjectiveTarget {
				return combatTrialFormatDefinition{}, fmt.Errorf("invalid standard combat trial targets")
			}
		}
		format.Dummy = combatTrialDummyDefinition{
			Count: 3, Positions: positions, DefenseSource: "combat_balance_defense_k", Defense: format.DefenseK,
			HPMode: "immutable_full", MaxHP: format.Encounter.Units[0].MaxHP, PassiveTurns: true,
			DamageResist: format.Encounter.Units[0].DamageResist, CriticalDamageResist: format.Encounter.Units[0].CriticalDamageResist,
		}
	}
	format.DefinitionDigest = ""
	digest, err := combatTrialDefinitionDigest(format)
	if err != nil {
		return combatTrialFormatDefinition{}, err
	}
	format.DefinitionDigest = digest
	return format, nil
}

func (s *Server) combatTrialFormatCatalog() ([]combatTrialFormatDefinition, map[string]combatTrialFormatDefinition, error) {
	metricsByID := make(map[string]combatTrialMetricDefinition, len(s.deps.GameData.CombatTrialMetrics))
	for _, row := range s.deps.GameData.CombatTrialMetrics {
		var metric combatTrialMetricDefinition
		if err := decodeCombatTrialCatalogRow(row, &metric); err != nil {
			return nil, nil, fmt.Errorf("decode combat trial metric: %w", err)
		}
		metric = normalizeCombatTrialMetric(metric)
		if err := validateCombatTrialMetric(metric); err != nil {
			return nil, nil, err
		}
		if _, exists := metricsByID[metric.ID]; exists {
			return nil, nil, fmt.Errorf("combat trial metric duplicated: %s", metric.ID)
		}
		metricsByID[metric.ID] = metric
	}
	authored := map[string]combatTrialFormatDefinition{}
	for _, row := range s.deps.GameData.CombatTrialFormats {
		var format combatTrialFormatDefinition
		if err := decodeCombatTrialCatalogRow(row, &format); err != nil {
			return nil, nil, fmt.Errorf("decode combat trial format: %w", err)
		}
		format.ID = strings.ToLower(strings.TrimSpace(format.ID))
		format.BaseFormatID = strings.ToLower(strings.TrimSpace(format.BaseFormatID))
		if !combatTrialStableID(format.ID) || format.SchemaVersion != 2 || format.RulesetVersion != combatTrialFormatRulesetVersion || format.BalanceRevision <= 0 {
			return nil, nil, fmt.Errorf("invalid combat trial format: %s", format.ID)
		}
		if _, exists := authored[format.ID]; exists {
			return nil, nil, fmt.Errorf("combat trial format duplicated: %s", format.ID)
		}
		authored[format.ID] = format
	}
	if standard, ok := authored[combatTrialDefaultFormatID]; !ok || !standard.Enabled || standard.BaseFormatID != "" {
		return nil, nil, fmt.Errorf("standard combat trial format is missing")
	}
	resolved := map[string]combatTrialFormatDefinition{}
	visiting := map[string]bool{}
	var resolve func(string) (combatTrialFormatDefinition, error)
	resolve = func(id string) (combatTrialFormatDefinition, error) {
		if format, ok := resolved[id]; ok {
			return format, nil
		}
		format, ok := authored[id]
		if !ok {
			return combatTrialFormatDefinition{}, fmt.Errorf("combat trial base format missing: %s", id)
		}
		if visiting[id] {
			return combatTrialFormatDefinition{}, fmt.Errorf("combat trial format inheritance cycle: %s", id)
		}
		visiting[id] = true
		if format.BaseFormatID != "" {
			base, err := resolve(format.BaseFormatID)
			if err != nil {
				return combatTrialFormatDefinition{}, err
			}
			if format.Enabled && !base.Enabled {
				return combatTrialFormatDefinition{}, fmt.Errorf("combat trial format %s inherits disabled base", id)
			}
			format = mergeCombatTrialFormat(base, format)
		}
		finalized, err := s.finalizeCombatTrialFormat(format, metricsByID)
		if err != nil {
			return combatTrialFormatDefinition{}, err
		}
		visiting[id] = false
		resolved[id] = finalized
		return finalized, nil
	}
	definitions := make([]combatTrialFormatDefinition, 0, len(authored))
	for id := range authored {
		format, err := resolve(id)
		if err != nil {
			return nil, nil, err
		}
		if format.Enabled {
			definitions = append(definitions, format)
		}
	}
	sort.SliceStable(definitions, func(i, j int) bool {
		if definitions[i].SortOrder != definitions[j].SortOrder {
			return definitions[i].SortOrder < definitions[j].SortOrder
		}
		return definitions[i].ID < definitions[j].ID
	})
	return definitions, resolved, nil
}

func (s *Server) resolveCombatTrialFormat(formatID string) (*combatTrialFormatDefinition, error) {
	formatID = strings.ToLower(strings.TrimSpace(formatID))
	if formatID == "" {
		formatID = combatTrialDefaultFormatID
	}
	_, byID, err := s.combatTrialFormatCatalog()
	if err != nil {
		return nil, err
	}
	format, ok := byID[formatID]
	if !ok || !format.Enabled {
		return nil, fmt.Errorf("combat trial format not found: %s", formatID)
	}
	return &format, nil
}

func (s *Server) combatTrialPublicFormats() ([]combatTrialFormatDefinition, error) {
	formats, _, err := s.combatTrialFormatCatalog()
	if err != nil {
		return nil, err
	}
	if len(formats) == 0 {
		return nil, fmt.Errorf("no enabled combat trial formats")
	}
	for index := range formats {
		formats[index] = combatTrialPublicFormat(formats[index])
	}
	return formats, nil
}

func combatTrialPublicFormat(format combatTrialFormatDefinition) combatTrialFormatDefinition {
	format.Encounter.EnemyStatProfile = cloneCombatTrialEnemyStatProfile(format.Encounter.EnemyStatProfile)
	format.Encounter.Units = append([]combatTrialEncounterUnitDefinition(nil), format.Encounter.Units...)
	for index := range format.Encounter.Units {
		format.Encounter.Units[index].InitialBuffs = cloneCombatTrialInitialBuffs(format.Encounter.Units[index].InitialBuffs)
		format.Encounter.Units[index].StatOverrides = cloneFloatMap(format.Encounter.Units[index].StatOverrides)
		format.Encounter.Units[index].ResolvedStats = cloneFloatMap(format.Encounter.Units[index].ResolvedStats)
		format.Encounter.Units[index].ResolvedUnit = nil
	}
	originalWaves := format.Encounter.Waves
	format.Encounter.Waves = make([]combatTrialEncounterWaveDefinition, len(originalWaves))
	for waveIndex := range originalWaves {
		format.Encounter.Waves[waveIndex] = originalWaves[waveIndex]
		format.Encounter.Waves[waveIndex].EnemyStatProfile = cloneCombatTrialEnemyStatProfile(originalWaves[waveIndex].EnemyStatProfile)
		format.Encounter.Waves[waveIndex].Units = append([]combatTrialEncounterUnitDefinition(nil), originalWaves[waveIndex].Units...)
		for unitIndex := range format.Encounter.Waves[waveIndex].Units {
			format.Encounter.Waves[waveIndex].Units[unitIndex].InitialBuffs = cloneCombatTrialInitialBuffs(originalWaves[waveIndex].Units[unitIndex].InitialBuffs)
			format.Encounter.Waves[waveIndex].Units[unitIndex].StatOverrides = cloneFloatMap(format.Encounter.Waves[waveIndex].Units[unitIndex].StatOverrides)
			format.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedStats = cloneFloatMap(format.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedStats)
			format.Encounter.Waves[waveIndex].Units[unitIndex].ResolvedUnit = nil
		}
	}
	return format
}

func decodeCombatTrialFormatSnapshot(raw json.RawMessage) (*combatTrialFormatDefinition, error) {
	var format combatTrialFormatDefinition
	if err := json.Unmarshal(raw, &format); err != nil {
		return nil, fmt.Errorf("decode combat trial format snapshot: %w", err)
	}
	expectedDigest := format.DefinitionDigest
	actualDigest, err := combatTrialDefinitionDigest(format)
	if err != nil || expectedDigest == "" || actualDigest != expectedDigest {
		return nil, fmt.Errorf("combat trial format digest mismatch")
	}
	for _, modifier := range format.Modifiers {
		if err := validateCombatTrialModifier(modifier); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
	}
	switch {
	case format.SchemaVersion == 1 && format.RulesetVersion == combatTrialFormatRulesetVersionV1:
		if !format.Enabled || format.ActionBudget <= 0 || format.Dummy.Count != 3 || strings.Join(format.Dummy.Positions, ",") != "vanguard,midguard,rearguard" ||
			format.Dummy.DefenseSource != "combat_balance_defense_k" || format.Dummy.Defense <= 0 || format.Dummy.HPMode != "immutable_full" || format.Dummy.MaxHP <= 0 || !format.Dummy.PassiveTurns {
			return nil, fmt.Errorf("invalid legacy combat trial format snapshot")
		}
		for index := range format.Metrics {
			format.Metrics[index] = normalizeCombatTrialMetric(format.Metrics[index])
			if err := validateCombatTrialMetric(format.Metrics[index]); err != nil {
				return nil, err
			}
		}
		format.DefenseK = format.Dummy.Defense
		format.LineupRules = &combatTrialLineupRulesDefinition{PositionPolicy: "distinct"}
		format.Limits = []combatTrialLimitDefinition{{Kind: "player_main_actions", Value: format.ActionBudget}}
		format.EndConditions = []combatTrialEndConditionDefinition{
			{Kind: "player_team_defeated", Reason: "team_defeated", LabelKey: "game.combatTrial.endReason.teamDefeated"},
			{Kind: "player_main_actions_reached", Reason: "action_budget_reached", LabelKey: "game.combatTrial.endReason.actionBudgetReached"},
		}
		format.Encounter = legacyCombatTrialEncounter(format.Dummy)
	case format.SchemaVersion == 2 && format.RulesetVersion == combatTrialFormatRulesetVersion:
		if !combatTrialStableID(format.ID) || !format.Enabled || format.BalanceRevision <= 0 || format.DefenseK <= 0 || combatTrialPlayerMainActionBudget(&format) != format.ActionBudget {
			return nil, fmt.Errorf("invalid combat trial format snapshot")
		}
		// V2 snapshots created before lineup policies existed used distinct rows.
		if format.LineupRules == nil {
			format.LineupRules = &combatTrialLineupRulesDefinition{PositionPolicy: "distinct"}
		}
		if err := validateCombatTrialLineupRules(&format); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
		if err := validateCombatTrialEncounterSnapshot(&format); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
		if err := validateCombatTrialLimitsAndEndConditions(&format); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
		if err := validateCombatTrialRules(format.Triggers); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
		if err := validateCombatTrialRuleTargets(&format); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
		if err := validateCombatTrialFormatMetrics(&format); err != nil {
			return nil, fmt.Errorf("invalid combat trial format snapshot: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported combat trial format snapshot")
	}
	return &format, nil
}

func legacyCombatTrialEncounter(dummy combatTrialDummyDefinition) combatTrialEncounterDefinition {
	units := make([]combatTrialEncounterUnitDefinition, 0, dummy.Count)
	for index := 0; index < dummy.Count && index < len(dummy.Positions); index++ {
		position := dummy.Positions[index]
		units = append(units, combatTrialEncounterUnitDefinition{
			ID: fmt.Sprintf("dummy_%d", index+1), Kind: "training_target", Position: position,
			NameKey: fmt.Sprintf("game.combatTrial.dummies.%s", position), Behavior: "passive", HealthMode: "immutable_full", ObjectiveTarget: true,
			DefenseSource: dummy.DefenseSource, Defense: dummy.Defense, MaxHP: dummy.MaxHP,
			DamageResist: dummy.DamageResist, CriticalDamageResist: dummy.CriticalDamageResist,
		})
	}
	return combatTrialEncounterDefinition{Kind: "unit_roster", Units: units}
}
