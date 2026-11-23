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
	arenaFormatStandardID     = "standard"
	arenaFormatRulesetVersion = "arena-format-v1"
)

type arenaFormatDefinition struct {
	ID               string               `json:"id"`
	BaseFormatID     string               `json:"baseFormatId,omitempty"`
	SchemaVersion    int                  `json:"schemaVersion"`
	RulesetVersion   string               `json:"rulesetVersion"`
	BalanceRevision  int                  `json:"balanceRevision"`
	NameKey          string               `json:"nameKey"`
	ShortDescKey     string               `json:"shortDescKey"`
	DescKey          string               `json:"descKey"`
	Icon             string               `json:"icon"`
	SortOrder        int                  `json:"sortOrder"`
	Enabled          bool                 `json:"enabled"`
	Triggers         []arenaFormatTrigger `json:"triggers"`
	DefinitionDigest string               `json:"definitionDigest,omitempty"`
}

type arenaFormatTrigger struct {
	ID      string              `json:"id"`
	Event   string              `json:"event"`
	Effects []arenaFormatEffect `json:"effects"`
}

type arenaFormatEffect struct {
	Kind                     string                          `json:"kind"`
	TargetScope              string                          `json:"targetScope"`
	IncludeSummons           bool                            `json:"includeSummons"`
	BuffID                   string                          `json:"buffId,omitempty"`
	PayloadKey               string                          `json:"payloadKey,omitempty"`
	Warmup                   *arenaFormatWarmup              `json:"warmup,omitempty"`
	MatchupHP                *arenaFormatMatchupHPAdjustment `json:"matchupHp,omitempty"`
	SpecializationEnabled    *bool                           `json:"specializationEnabled,omitempty"`
	Charges                  int                             `json:"charges,omitempty"`
	DamageType               string                          `json:"damageType,omitempty"`
	SkipFinalDamageModifiers bool                            `json:"skipFinalDamageModifiers,omitempty"`
	DispatchHooks            bool                            `json:"dispatchHooks,omitempty"`
	Simultaneous             bool                            `json:"simultaneous,omitempty"`
	Amount                   arenaFormatAmount               `json:"amount,omitempty"`
}

type arenaFormatWarmup struct {
	LevelSource             string   `json:"levelSource"`
	HigherSideGapExceeds    int64    `json:"higherSideGapExceeds"`
	AlwaysAboveLevel        int64    `json:"alwaysAboveLevel"`
	NormalActions           int      `json:"normalActions"`
	StartMultiplier         float64  `json:"startMultiplier"`
	RecoveryPerNormalAction float64  `json:"recoveryPerNormalAction"`
	MaximumMultiplier       float64  `json:"maximumMultiplier"`
	Outputs                 []string `json:"outputs"`
}

type arenaFormatMatchupHPAdjustment struct {
	LevelSource            string  `json:"levelSource"`
	BothAtLeastLevel       int64   `json:"bothAtLeastLevel"`
	BothSidesFlatBonus     float64 `json:"bothSidesFlatBonus"`
	LowerSideBonusPerLevel float64 `json:"lowerSideBonusPerLevel"`
}

type arenaFormatAmount struct {
	Basis      string  `json:"basis"`
	Multiplier float64 `json:"multiplier"`
	Flat       float64 `json:"flat"`
	Minimum    float64 `json:"minimum"`
	Maximum    float64 `json:"maximum,omitempty"`
}

type arenaFormatPublic struct {
	ID               string                  `json:"id"`
	BaseFormatID     string                  `json:"baseFormatId,omitempty"`
	NameKey          string                  `json:"nameKey"`
	ShortDescKey     string                  `json:"shortDescKey"`
	DescKey          string                  `json:"descKey"`
	Icon             string                  `json:"icon"`
	Standard         bool                    `json:"standard"`
	SchemaVersion    int                     `json:"schemaVersion,omitempty"`
	RulesetVersion   string                  `json:"rulesetVersion,omitempty"`
	BalanceRevision  int                     `json:"balanceRevision,omitempty"`
	DefinitionDigest string                  `json:"definitionDigest,omitempty"`
	Rules            []arenaFormatRulePublic `json:"rules"`
}

type arenaFormatRulePublic struct {
	Event                    string                          `json:"event"`
	EffectKind               string                          `json:"effectKind"`
	TargetScope              string                          `json:"targetScope"`
	IncludeSummons           bool                            `json:"includeSummons"`
	BuffID                   string                          `json:"buffId,omitempty"`
	PayloadKey               string                          `json:"payloadKey,omitempty"`
	Warmup                   *arenaFormatWarmup              `json:"warmup,omitempty"`
	MatchupHP                *arenaFormatMatchupHPAdjustment `json:"matchupHp,omitempty"`
	SpecializationEnabled    *bool                           `json:"specializationEnabled,omitempty"`
	Charges                  int                             `json:"charges,omitempty"`
	DamageType               string                          `json:"damageType,omitempty"`
	SkipFinalDamageModifiers bool                            `json:"skipFinalDamageModifiers"`
	DispatchHooks            bool                            `json:"dispatchHooks"`
	Simultaneous             bool                            `json:"simultaneous"`
	Amount                   *arenaFormatAmount              `json:"amount,omitempty"`
}

func normalizeArenaFormatID(raw string) string {
	return normalizeCompetitiveDefinitionID(raw)
}

func arenaFormatFiniteRange(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func arenaFormatOutputKindsValid(outputs []string) bool {
	if len(outputs) != 2 {
		return false
	}
	seen := make(map[string]struct{}, len(outputs))
	for _, output := range outputs {
		if output != "damage" && output != "heal" {
			return false
		}
		seen[output] = struct{}{}
	}
	return len(seen) == 2
}

func arenaPublicFormat(definition *arenaFormatDefinition) arenaFormatPublic {
	if definition == nil {
		return arenaFormatPublic{Rules: []arenaFormatRulePublic{}}
	}
	public := arenaFormatPublic{
		ID:               definition.ID,
		BaseFormatID:     definition.BaseFormatID,
		NameKey:          definition.NameKey,
		ShortDescKey:     definition.ShortDescKey,
		DescKey:          definition.DescKey,
		Icon:             definition.Icon,
		SchemaVersion:    definition.SchemaVersion,
		RulesetVersion:   definition.RulesetVersion,
		BalanceRevision:  definition.BalanceRevision,
		DefinitionDigest: definition.DefinitionDigest,
		Standard:         definition.ID == arenaFormatStandardID,
		Rules:            make([]arenaFormatRulePublic, 0, len(definition.Triggers)),
	}
	for _, trigger := range definition.Triggers {
		for _, effect := range trigger.Effects {
			rule := arenaFormatRulePublic{
				Event: trigger.Event, EffectKind: effect.Kind, TargetScope: effect.TargetScope,
				IncludeSummons: effect.IncludeSummons, BuffID: effect.BuffID, PayloadKey: effect.PayloadKey,
				SpecializationEnabled: effect.SpecializationEnabled, Charges: effect.Charges, DamageType: effect.DamageType,
				SkipFinalDamageModifiers: effect.SkipFinalDamageModifiers, DispatchHooks: effect.DispatchHooks,
				Simultaneous: effect.Simultaneous,
			}
			if effect.Warmup != nil {
				warmup := *effect.Warmup
				warmup.Outputs = append([]string(nil), effect.Warmup.Outputs...)
				rule.Warmup = &warmup
			}
			if effect.MatchupHP != nil {
				matchupHP := *effect.MatchupHP
				rule.MatchupHP = &matchupHP
			}
			if effect.Kind == "damage" {
				amount := effect.Amount
				rule.Amount = &amount
			}
			public.Rules = append(public.Rules, rule)
		}
	}
	return public
}

func arenaFormatDigest(definition arenaFormatDefinition) (string, error) {
	definition.DefinitionDigest = ""
	raw, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func normalizeArenaFormatDefinition(definition arenaFormatDefinition, requireEnabled bool) (arenaFormatDefinition, error) {
	definition.ID = normalizeArenaFormatID(definition.ID)
	definition.BaseFormatID = normalizeArenaFormatID(definition.BaseFormatID)
	definition.RulesetVersion = strings.TrimSpace(definition.RulesetVersion)
	definition.NameKey = strings.TrimSpace(definition.NameKey)
	definition.ShortDescKey = strings.TrimSpace(definition.ShortDescKey)
	definition.DescKey = strings.TrimSpace(definition.DescKey)
	definition.Icon = strings.TrimSpace(definition.Icon)
	definition.DefinitionDigest = strings.TrimSpace(definition.DefinitionDigest)
	if definition.ID == "" || definition.BaseFormatID == definition.ID || definition.SchemaVersion != 1 || definition.RulesetVersion != arenaFormatRulesetVersion || definition.BalanceRevision <= 0 || definition.SortOrder < 0 {
		return arenaFormatDefinition{}, fmt.Errorf("invalid arena format identity")
	}
	if definition.ID == arenaFormatStandardID && definition.BaseFormatID != "" {
		return arenaFormatDefinition{}, fmt.Errorf("standard arena format cannot inherit another format")
	}
	if definition.ID != arenaFormatStandardID && definition.BaseFormatID == "" {
		return arenaFormatDefinition{}, fmt.Errorf("arena format %q must inherit the standard format", definition.ID)
	}
	if requireEnabled && !definition.Enabled {
		return arenaFormatDefinition{}, fmt.Errorf("arena format %q is not enabled", definition.ID)
	}
	if definition.NameKey != "game.arena.formats."+definition.ID+".name" ||
		definition.ShortDescKey != "game.arena.formats."+definition.ID+".shortDesc" ||
		definition.DescKey != "game.arena.formats."+definition.ID+".desc" || definition.Icon == "" || len(definition.Triggers) == 0 {
		return arenaFormatDefinition{}, fmt.Errorf("invalid arena format presentation: %s", definition.ID)
	}
	triggerIDs := make(map[string]struct{}, len(definition.Triggers))
	for triggerIndex := range definition.Triggers {
		trigger := &definition.Triggers[triggerIndex]
		trigger.ID = normalizeArenaFormatID(trigger.ID)
		trigger.Event = strings.ToLower(strings.TrimSpace(trigger.Event))
		if trigger.ID == "" || (trigger.Event != "lineup_snapshot" && trigger.Event != "battle_start" && trigger.Event != "after_summon") || len(trigger.Effects) == 0 {
			return arenaFormatDefinition{}, fmt.Errorf("invalid arena format trigger: %s", definition.ID)
		}
		if _, exists := triggerIDs[trigger.ID]; exists {
			return arenaFormatDefinition{}, fmt.Errorf("duplicate arena format trigger: %s.%s", definition.ID, trigger.ID)
		}
		triggerIDs[trigger.ID] = struct{}{}
		for effectIndex := range trigger.Effects {
			effect := &trigger.Effects[effectIndex]
			effect.Kind = strings.ToLower(strings.TrimSpace(effect.Kind))
			effect.TargetScope = strings.ToLower(strings.TrimSpace(effect.TargetScope))
			effect.BuffID = strings.ToLower(strings.TrimSpace(effect.BuffID))
			effect.PayloadKey = strings.TrimSpace(effect.PayloadKey)
			effect.DamageType = strings.ToLower(strings.TrimSpace(effect.DamageType))
			switch effect.Kind {
			case "warmup_buff":
				warmup := effect.Warmup
				if trigger.Event != "battle_start" || effect.TargetScope != "eligible_side_units" || !effect.IncludeSummons ||
					effect.BuffID != arenaOutputWarmupBuffID || warmup == nil {
					return arenaFormatDefinition{}, fmt.Errorf("invalid warmup buff: %s.%s", definition.ID, trigger.ID)
				}
				warmup.LevelSource = strings.ToLower(strings.TrimSpace(warmup.LevelSource))
				for index := range warmup.Outputs {
					warmup.Outputs[index] = strings.ToLower(strings.TrimSpace(warmup.Outputs[index]))
				}
				if warmup.LevelSource != "player_battle_level" || warmup.HigherSideGapExceeds <= 0 || warmup.AlwaysAboveLevel <= 0 ||
					warmup.NormalActions <= 0 || warmup.NormalActions > 10 || !arenaFormatFiniteRange(warmup.StartMultiplier, 0, 1) ||
					!arenaFormatFiniteRange(warmup.RecoveryPerNormalAction, math.SmallestNonzeroFloat64, 1) ||
					!arenaFormatFiniteRange(warmup.MaximumMultiplier, math.SmallestNonzeroFloat64, 1) ||
					math.Abs(warmup.StartMultiplier+float64(warmup.NormalActions)*warmup.RecoveryPerNormalAction-warmup.MaximumMultiplier) > 0.000001 ||
					!arenaFormatOutputKindsValid(warmup.Outputs) {
					return arenaFormatDefinition{}, fmt.Errorf("invalid warmup rule: %s.%s", definition.ID, trigger.ID)
				}
			case "matchup_hp_buff":
				adjustment := effect.MatchupHP
				if trigger.Event != "battle_start" || effect.TargetScope != "formal_units_by_side" || effect.IncludeSummons ||
					effect.BuffID != arenaMatchupHPBuffID || effect.PayloadKey != "hpBonus" || adjustment == nil {
					return arenaFormatDefinition{}, fmt.Errorf("invalid matchup hp buff: %s.%s", definition.ID, trigger.ID)
				}
				adjustment.LevelSource = strings.ToLower(strings.TrimSpace(adjustment.LevelSource))
				if adjustment.LevelSource != "player_battle_level" || adjustment.BothAtLeastLevel <= 0 ||
					!arenaFormatFiniteRange(adjustment.BothSidesFlatBonus, math.SmallestNonzeroFloat64, math.MaxFloat64) ||
					!arenaFormatFiniteRange(adjustment.LowerSideBonusPerLevel, math.SmallestNonzeroFloat64, math.MaxFloat64) {
					return arenaFormatDefinition{}, fmt.Errorf("invalid matchup hp adjustment: %s.%s", definition.ID, trigger.ID)
				}
			case "specialization_policy":
				if trigger.Event != "lineup_snapshot" || effect.TargetScope != "player_units" || effect.IncludeSummons || effect.SpecializationEnabled == nil {
					return arenaFormatDefinition{}, fmt.Errorf("invalid specialization policy: %s.%s", definition.ID, trigger.ID)
				}
			case "turn_barrier":
				if trigger.Event != "battle_start" || effect.TargetScope != "all_units" || effect.BuffID == "" || effect.Charges <= 0 || effect.Charges > 12 {
					return arenaFormatDefinition{}, fmt.Errorf("invalid turn barrier: %s.%s", definition.ID, trigger.ID)
				}
			case "damage":
				if trigger.Event != "after_summon" || effect.TargetScope != "all_alive_units" ||
					(effect.DamageType != "physical" && effect.DamageType != "magical") || effect.Amount.Basis != "target_max_hp" ||
					!effect.Simultaneous ||
					effect.Amount.Multiplier < 0 || effect.Amount.Multiplier > 1 || effect.Amount.Flat < 0 || effect.Amount.Minimum < 0 ||
					(effect.Amount.Maximum > 0 && effect.Amount.Maximum < effect.Amount.Minimum) ||
					(effect.Amount.Multiplier == 0 && effect.Amount.Flat == 0) {
					return arenaFormatDefinition{}, fmt.Errorf("invalid arena format damage: %s.%s", definition.ID, trigger.ID)
				}
			default:
				return arenaFormatDefinition{}, fmt.Errorf("unsupported arena format effect %q", effect.Kind)
			}
		}
	}
	digest, err := arenaFormatDigest(definition)
	if err != nil {
		return arenaFormatDefinition{}, err
	}
	if definition.DefinitionDigest != "" && !strings.EqualFold(definition.DefinitionDigest, digest) {
		return arenaFormatDefinition{}, fmt.Errorf("arena format digest mismatch: %s", definition.ID)
	}
	definition.DefinitionDigest = digest
	return definition, nil
}

func (s *Server) arenaFormatCatalog() ([]arenaFormatDefinition, map[string]arenaFormatDefinition, error) {
	rows := s.deps.GameData.ArenaFormats
	authoredByID := make(map[string]arenaFormatDefinition, len(rows))
	for _, row := range rows {
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, nil, err
		}
		var definition arenaFormatDefinition
		if err := json.Unmarshal(raw, &definition); err != nil {
			return nil, nil, err
		}
		definition, err = normalizeArenaFormatDefinition(definition, false)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := authoredByID[definition.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate arena format %q", definition.ID)
		}
		authoredByID[definition.ID] = definition
	}
	standard, exists := authoredByID[arenaFormatStandardID]
	if !exists {
		return nil, nil, fmt.Errorf("standard arena format is missing")
	}
	if !standard.Enabled {
		return nil, nil, fmt.Errorf("standard arena format is disabled")
	}

	resolvedByID := make(map[string]arenaFormatDefinition, len(authoredByID))
	visiting := make(map[string]bool, len(authoredByID))
	var resolve func(string) (arenaFormatDefinition, error)
	resolve = func(id string) (arenaFormatDefinition, error) {
		if resolved, exists := resolvedByID[id]; exists {
			return resolved, nil
		}
		definition, exists := authoredByID[id]
		if !exists {
			return arenaFormatDefinition{}, fmt.Errorf("arena format base %q is missing", id)
		}
		if visiting[id] {
			return arenaFormatDefinition{}, fmt.Errorf("arena format inheritance cycle at %q", id)
		}
		visiting[id] = true
		if definition.BaseFormatID != "" {
			base, err := resolve(definition.BaseFormatID)
			if err != nil {
				return arenaFormatDefinition{}, err
			}
			if definition.Enabled && !base.Enabled {
				return arenaFormatDefinition{}, fmt.Errorf("arena format %q inherits disabled base %q", id, definition.BaseFormatID)
			}
			merged := make([]arenaFormatTrigger, 0, len(base.Triggers)+len(definition.Triggers))
			merged = append(merged, base.Triggers...)
			merged = append(merged, definition.Triggers...)
			definition.Triggers = merged
		}
		definition.DefinitionDigest = ""
		resolved, err := normalizeArenaFormatDefinition(definition, false)
		if err != nil {
			return arenaFormatDefinition{}, err
		}
		visiting[id] = false
		resolvedByID[id] = resolved
		return resolved, nil
	}

	definitions := make([]arenaFormatDefinition, 0, len(authoredByID))
	for id := range authoredByID {
		definition, err := resolve(id)
		if err != nil {
			return nil, nil, err
		}
		if definition.Enabled {
			definitions = append(definitions, definition)
		}
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].SortOrder != definitions[j].SortOrder {
			return definitions[i].SortOrder < definitions[j].SortOrder
		}
		return definitions[i].ID < definitions[j].ID
	})
	return definitions, resolvedByID, nil
}

func (s *Server) arenaPublicFormats() ([]arenaFormatPublic, error) {
	definitions, _, err := s.arenaFormatCatalog()
	if err != nil {
		return nil, err
	}
	out := make([]arenaFormatPublic, 0, len(definitions))
	for index := range definitions {
		out = append(out, arenaPublicFormat(&definitions[index]))
	}
	return out, nil
}

func (s *Server) resolveArenaFormatForPublish(formatID string) (*arenaFormatDefinition, error) {
	formatID = normalizeArenaFormatID(formatID)
	if formatID == "" {
		return nil, fmt.Errorf("arena format is required")
	}
	_, byID, err := s.arenaFormatCatalog()
	if err != nil {
		return nil, err
	}
	definition, exists := byID[formatID]
	if !exists || !definition.Enabled {
		return nil, fmt.Errorf("arena format %q is unavailable", formatID)
	}
	return &definition, nil
}

func arenaFormatFromSelection(selection arenaLineupSelection) (*arenaFormatDefinition, error) {
	formatID := normalizeArenaFormatID(selection.FormatID)
	if formatID == "" {
		return nil, fmt.Errorf("arena format id is missing")
	}
	if selection.FormatSnapshot == nil {
		return nil, fmt.Errorf("arena format snapshot is missing")
	}
	definition, err := normalizeArenaFormatDefinition(*selection.FormatSnapshot, false)
	if err != nil {
		return nil, err
	}
	if definition.ID != formatID {
		return nil, fmt.Errorf("arena format snapshot id mismatch")
	}
	return &definition, nil
}

func arenaFormatSpecializationEnabled(definition *arenaFormatDefinition) bool {
	enabled := true
	if definition == nil {
		return enabled
	}
	for _, trigger := range definition.Triggers {
		if trigger.Event != "lineup_snapshot" {
			continue
		}
		for _, effect := range trigger.Effects {
			if effect.Kind == "specialization_policy" && effect.TargetScope == "player_units" && effect.SpecializationEnabled != nil {
				enabled = *effect.SpecializationEnabled
			}
		}
	}
	return enabled
}

func arenaFormatSourceMeta(definition *arenaFormatDefinition, triggerID string) map[string]any {
	if definition == nil {
		return nil
	}
	return map[string]any{
		"sourceKind":    "arena_format",
		"sourceId":      definition.ID,
		"sourceNameKey": definition.NameKey,
		"hookId":        strings.TrimSpace(triggerID),
	}
}
