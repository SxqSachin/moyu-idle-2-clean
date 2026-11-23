package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

var (
	ErrSkillForgeInvalidGraph    = errors.New("skill forge graph is invalid")
	ErrSkillForgeBudget          = errors.New("skill forge budget exceeded")
	ErrSkillForgeSourceStale     = errors.New("skill forge source definition changed")
	ErrSkillForgeLevelTooLow     = errors.New("skill forge alchemy level too low")
	ErrSkillForgePending         = errors.New("skill forge draft already has a pending result")
	ErrSkillForgeLimit           = errors.New("skill forge limit reached")
	ErrSkillForgeNotFound        = errors.New("skill forge record not found")
	ErrSkillForgeConflict        = errors.New("skill forge revision conflict")
	ErrSkillForgeBlankOnly       = errors.New("blank skill forge drafts are preview only")
	ErrSkillForgeBookCount       = errors.New("source skillbook count is outside the configured range")
	ErrSkillForgeRulesetDisabled = errors.New("skill forge ruleset is disabled")
	ErrForgedSkillbookConflict   = errors.New("forged skillbook already resolved for another target")
	ErrSkillForgeSameSource      = errors.New("only one skill from the same official source may be enabled")
)

const SkillForgeGraphVersion = 1

type SkillForgeGraphV1 struct {
	Version int                   `json:"version"`
	Source  SkillForgeGraphSource `json:"source"`
	Nodes   []SkillForgeGraphNode `json:"nodes"`
	Edges   []SkillForgeGraphEdge `json:"edges"`
}

type SkillForgeGraphSource struct {
	Kind           string `json:"kind"`
	SkillID        string `json:"skillId,omitempty"`
	DefinitionHash string `json:"definitionHash,omitempty"`
}

type SkillForgeGraphNode struct {
	ID            string                  `json:"id"`
	Kind          string                  `json:"kind"`
	ModuleID      string                  `json:"moduleId,omitempty"`
	Quantity      int64                   `json:"quantity,omitempty"`
	EffectKind    string                  `json:"effectKind,omitempty"`
	EffectIndex   int                     `json:"effectIndex,omitempty"`
	ParameterKind string                  `json:"parameterKind,omitempty"`
	DSLPath       string                  `json:"dslPath,omitempty"`
	TargetNodeID  string                  `json:"targetNodeId,omitempty"`
	Position      SkillForgeGraphPosition `json:"position"`
}

type SkillForgeGraphPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type SkillForgeGraphEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle"`
}

type SkillForgeDerived struct {
	Level     float64 `json:"level"`
	Precision float64 `json:"precision"`
	Stability float64 `json:"stability"`
}

type SkillForgeParameterRange struct {
	NodeID                string   `json:"nodeId"`
	TargetNodeID          string   `json:"targetNodeId,omitempty"`
	Kind                  string   `json:"kind"`
	Direction             string   `json:"direction,omitempty"`
	BaseMin               float64  `json:"baseMin"`
	BaseMax               float64  `json:"baseMax"`
	Min                   float64  `json:"min"`
	Max                   float64  `json:"max"`
	SuccessChanceModifier float64  `json:"successChanceModifier"`
	Rolled                *float64 `json:"rolled,omitempty"`
}

type SkillForgeGoldCostBreakdown struct {
	Base      int64 `json:"base"`
	Materials int64 `json:"materials"`
	Budget    int64 `json:"budget"`
}

type SkillForgeCompileResult struct {
	Valid                   bool                        `json:"valid"`
	BlockingReasons         []string                    `json:"blockingReasons"`
	CompiledSkill           map[string]any              `json:"compiledSkill,omitempty"`
	Ranges                  []SkillForgeParameterRange  `json:"ranges"`
	Recipe                  map[string]int64            `json:"recipe"`
	BudgetUsed              float64                     `json:"budgetUsed"`
	BudgetLimit             float64                     `json:"budgetLimit"`
	BudgetRefund            float64                     `json:"budgetRefund"`
	BudgetMultiplier        float64                     `json:"budgetMultiplier"`
	FailChance              float64                     `json:"failChance"`
	BaseSuccessChance       float64                     `json:"baseSuccessChance"`
	MaterialSuccessModifier float64                     `json:"materialSuccessModifier"`
	BaseFailChance          float64                     `json:"baseFailChance"`
	SkillbookBonus          float64                     `json:"skillbookBonus"`
	FinalFailChance         float64                     `json:"finalFailChance"`
	SourceSkillbookItemID   string                      `json:"sourceSkillbookItemId,omitempty"`
	SourceSkillbookCount    int64                       `json:"sourceSkillbookCount,omitempty"`
	GoldCost                int64                       `json:"goldCost"`
	GoldCostBreakdown       SkillForgeGoldCostBreakdown `json:"goldCostBreakdown"`
	Failed                  bool                        `json:"failed"`
	CompilerVersion         int                         `json:"compilerVersion"`
	RulesetVersion          int                         `json:"rulesetVersion"`
	GraphHash               string                      `json:"graphHash"`
	SourceDefinitionHash    string                      `json:"sourceDefinitionHash,omitempty"`
}

type skillForgeCompileMode int

const (
	skillForgePreview skillForgeCompileMode = iota
	skillForgeRoll
	skillForgeFloor
)

func CompileSkillForgeGraph(
	graph SkillForgeGraphV1,
	baseSkill map[string]any,
	modules []catalog.SkillAlchemyModule,
	settings config.SkillForgeSettings,
	_ SkillForgeDerived,
	rng *rand.Rand,
	mode skillForgeCompileMode,
) (SkillForgeCompileResult, error) {
	result := SkillForgeCompileResult{
		BlockingReasons:   []string{},
		Ranges:            []SkillForgeParameterRange{},
		Recipe:            map[string]int64{},
		CompilerVersion:   settings.CompilerVersion,
		RulesetVersion:    settings.RulesetVersion,
		BudgetMultiplier:  1,
		BaseSuccessChance: settings.BaseSuccessChance,
	}
	rawGraph, err := json.Marshal(graph)
	if err != nil {
		return result, fmt.Errorf("marshal skill forge graph: %w", err)
	}
	sum := sha256.Sum256(rawGraph)
	result.GraphHash = hex.EncodeToString(sum[:])
	if len(rawGraph) > settings.MaxGraphBytes {
		return result, fmt.Errorf("%w: graph payload exceeds %d bytes", ErrSkillForgeInvalidGraph, settings.MaxGraphBytes)
	}
	ordered, moduleByNode, err := validateSkillForgeGraph(graph, modules, settings)
	if err != nil {
		return result, err
	}

	compiled, err := skillForgeBaseDefinition(graph, baseSkill)
	if err != nil {
		return result, err
	}
	upperBoundCompiled, err := skillForgeBaseDefinition(graph, baseSkill)
	if err != nil {
		return result, err
	}
	if baseSkill != nil {
		hash, hashErr := SkillForgeDefinitionHash(baseSkill)
		if hashErr != nil {
			return result, hashErr
		}
		result.SourceDefinitionHash = hash
		if graph.Source.DefinitionHash != "" && graph.Source.DefinitionHash != hash {
			return result, ErrSkillForgeSourceStale
		}
	}

	effects := skillForgeEffects(compiled)
	upperBoundEffects := skillForgeEffects(upperBoundCompiled)
	var customizationCount int64
	hasMaterialModification := false
	var totalMaterialQuantity int64
	baseEffectCapacity := len(effects)
	if baseEffectCapacity < 1 {
		baseEffectCapacity = 1
	}
	extraEffectSlots := 0
	for _, node := range graph.Nodes {
		if module, ok := moduleByNode[node.ID]; ok && stringValue(module.CompileOp["kind"]) == "effect_slot" {
			extraEffectSlots += int(node.Quantity / module.UnitBasis)
		}
	}
	allowedEffectCount := baseEffectCapacity + extraEffectSlots
	if allowedEffectCount > settings.MaxEffects {
		return result, fmt.Errorf("%w: effect-slot material exceeds the global maximum of %d effects", ErrSkillForgeInvalidGraph, settings.MaxEffects)
	}
	effectNodes := make([]SkillForgeGraphNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.Kind == "effect" {
			effectNodes = append(effectNodes, node)
		}
	}
	sort.Slice(effectNodes, func(i, j int) bool {
		if effectNodes[i].EffectIndex != effectNodes[j].EffectIndex {
			return effectNodes[i].EffectIndex < effectNodes[j].EffectIndex
		}
		return effectNodes[i].ID < effectNodes[j].ID
	})
	if len(effectNodes) > allowedEffectCount {
		return result, fmt.Errorf("%w: %d effects require %d additional effect-slot material", ErrSkillForgeInvalidGraph, len(effectNodes), len(effectNodes)-baseEffectCapacity)
	}
	for _, node := range effectNodes {
		if node.EffectIndex < 0 || node.EffectIndex >= settings.MaxEffects {
			return result, fmt.Errorf("%w: effect index outside limits", ErrSkillForgeInvalidGraph)
		}
		if node.EffectIndex > len(effects) {
			return result, fmt.Errorf("%w: effect index has a gap", ErrSkillForgeInvalidGraph)
		}
		if node.EffectIndex == len(effects) {
			if len(effects) >= settings.MaxEffects {
				return result, fmt.Errorf("%w: too many effects", ErrSkillForgeInvalidGraph)
			}
			effects = append(effects, newSkillForgeEffect(node.EffectKind))
			upperBoundEffects = append(upperBoundEffects, newSkillForgeEffect(node.EffectKind))
		}
	}
	for _, nodeID := range ordered {
		node := graphNodeByID(graph.Nodes, nodeID)
		if node == nil {
			continue
		}
		if node.Kind == "effect" {
			continue
		}
		module, ok := moduleByNode[node.ID]
		if !ok {
			continue
		}
		hasMaterialModification = true
		totalMaterialQuantity += node.Quantity
		result.Recipe[module.ItemID] += node.Quantity
		customizationKey := strings.TrimSpace(node.TargetNodeID)
		if customizationKey == "" {
			// Older drafts can encode modifiers directly in an effect or skill chain.
			// Derive the authoritative parameter so omitting targetNodeId cannot skip
			// the per-customized-node seal cost.
			customizationKey = skillForgeImplicitCustomizationKey(*node, module)
		}
		if customizationKey != "" {
			customizationCount++
		}
		baseMin := module.ValuePerUnit.Min * float64(node.Quantity)
		baseMax := module.ValuePerUnit.Max * float64(node.Quantity)
		minValue, maxValue := math.Min(baseMin, baseMax), math.Max(baseMin, baseMax)
		moduleKind := stringValue(module.CompileOp["kind"])
		if moduleKind == "status_apply" {
			steps := node.Quantity / module.UnitBasis
			chance := settings.StatusBaseApplyChance
			if steps > 1 {
				chance += float64(steps-1) * settings.StatusApplyChancePerStep
			}
			baseMin = clampSkillForge(chance, 0, 1)
			baseMax = baseMin
			minValue = baseMin
			maxValue = baseMin
		}
		value := (minValue + maxValue) / 2
		basisCount := float64(node.Quantity) / float64(max(int64(1), module.UnitBasis))
		nodeSuccessModifier := module.SuccessChanceModifier * basisCount
		if mode == skillForgeRoll {
			if rng == nil {
				return result, fmt.Errorf("skill forge roll requires rng")
			}
			value = minValue + (maxValue-minValue)*rng.Float64()
		} else if mode == skillForgeFloor {
			value = minValue
		}
		rangeRow := SkillForgeParameterRange{
			NodeID: node.ID, TargetNodeID: node.TargetNodeID, Kind: moduleKind,
			Direction: strings.TrimSpace(stringValue(module.CompileOp["direction"])),
			BaseMin:   roundSkillForge(baseMin), BaseMax: roundSkillForge(baseMax),
			Min: roundSkillForge(minValue), Max: roundSkillForge(maxValue),
			SuccessChanceModifier: roundSkillForge(nodeSuccessModifier),
		}
		if mode == skillForgeRoll || mode == skillForgeFloor {
			rolled := roundSkillForge(value)
			rangeRow.Rolled = &rolled
		}
		result.Ranges = append(result.Ranges, rangeRow)
		_, _, applyErr := applySkillForgeModule(compiled, &effects, *node, module, value, settings)
		if applyErr != nil {
			return result, applyErr
		}
		if _, _, applyErr := applySkillForgeModule(upperBoundCompiled, &upperBoundEffects, *node, module, maxValue, settings); applyErr != nil {
			return result, applyErr
		}
		upperUsed, upperRefund := skillForgeModuleBudget(module, maxValue)
		result.BudgetUsed += upperUsed
		result.BudgetRefund += upperRefund
		result.BudgetLimit += module.BudgetCreditPerUnit * float64(node.Quantity)
		result.MaterialSuccessModifier += nodeSuccessModifier
	}
	if customizationCount > 0 {
		result.Recipe[settings.CustomizedNodeSealItemID] += customizationCount * settings.CustomizedNodeSealCount
	}
	if graph.Source.Kind == "official" && hasMaterialModification {
		result.Recipe[settings.ForgeCatalystItemID] += settings.ForgeCatalystCount
	}
	if err := applySkillForgeConditionBranches(graph, moduleByNode, effects); err != nil {
		return result, err
	}
	if err := applySkillForgeConditionBranches(graph, moduleByNode, upperBoundEffects); err != nil {
		return result, err
	}
	if err := validateSkillForgeStatusScopes(effects); err != nil {
		return result, err
	}
	if err := validateSkillForgeStatusScopes(upperBoundEffects); err != nil {
		return result, err
	}
	clearSkillForgeCompilerMarkers(effects)
	clearSkillForgeCompilerMarkers(upperBoundEffects)

	if graph.Source.Kind == "blank" {
		effects = filterSkillForgeEmptyEffects(effects)
	}
	if len(effects) == 0 {
		return result, fmt.Errorf("%w: skill requires at least one effect", ErrSkillForgeInvalidGraph)
	}
	if len(effects) > settings.MaxEffects || len(upperBoundEffects) > settings.MaxEffects {
		return result, fmt.Errorf("%w: effect count exceeds %d", ErrSkillForgeInvalidGraph, settings.MaxEffects)
	}
	compiled["effects"] = effects
	compiled["rarity"] = skillForgeRarityForEffectCount(len(effects))
	normalizeSkillForgeCooldown(compiled, settings.MinCooldown)
	normalizeSkillForgeCooldown(upperBoundCompiled, settings.MinCooldown)
	result.BudgetMultiplier = skillForgeCombinationMultiplier(upperBoundEffects)
	gross := math.Max(0, result.BudgetUsed*result.BudgetMultiplier)
	refundCap := gross * settings.MaxTradeoffRefundRatio
	result.BudgetRefund = math.Min(result.BudgetRefund, refundCap)
	result.BudgetUsed = math.Max(0, gross-result.BudgetRefund)
	result.BudgetUsed = roundSkillForge(result.BudgetUsed)
	result.BudgetLimit = roundSkillForge(result.BudgetLimit)
	result.BudgetRefund = roundSkillForge(result.BudgetRefund)
	result.MaterialSuccessModifier = roundSkillForge(result.MaterialSuccessModifier)
	result.FailChance = roundSkillForge(clampSkillForge(1-result.BaseSuccessChance-result.MaterialSuccessModifier, 0, 1))
	if graph.Source.Kind == "official" && hasMaterialModification {
		result.GoldCostBreakdown = SkillForgeGoldCostBreakdown{
			Base:      settings.GoldCostBase,
			Materials: totalMaterialQuantity * settings.GoldCostPerMaterial,
			Budget:    int64(math.Ceil(result.BudgetUsed * float64(settings.GoldCostPerBudget))),
		}
		result.GoldCost = result.GoldCostBreakdown.Base + result.GoldCostBreakdown.Materials + result.GoldCostBreakdown.Budget
		if result.GoldCost > 0 {
			result.Recipe[settings.GoldCostItemID] += result.GoldCost
		}
	}
	if result.BudgetUsed > result.BudgetLimit+0.000001 {
		return result, fmt.Errorf("%w: %.3f > %.3f", ErrSkillForgeBudget, result.BudgetUsed, result.BudgetLimit)
	}
	if cooldown := intValue(compiled["cooldown"]); cooldown > settings.MaxCooldown {
		return result, fmt.Errorf("%w: cooldown exceeds %d", ErrSkillForgeInvalidGraph, settings.MaxCooldown)
	}
	if cooldown := intValue(upperBoundCompiled["cooldown"]); cooldown > settings.MaxCooldown {
		return result, fmt.Errorf("%w: cooldown upper bound exceeds %d", ErrSkillForgeInvalidGraph, settings.MaxCooldown)
	}
	applySkillForgeIdentity(compiled, graph, modules)
	result.CompiledSkill = compiled
	if !hasMaterialModification {
		result.BlockingReasons = append(result.BlockingReasons, "graph requires at least one material modification")
		result.Valid = false
		return result, nil
	}
	result.Valid = true
	return result, nil
}

func normalizeSkillForgeCooldown(skill map[string]any, minimum int) {
	if minimum < 1 {
		minimum = 1
	}
	skill["cooldown"] = max(minimum, intValue(skill["cooldown"]))
}

func skillForgeCombinationMultiplier(effects []any) float64 {
	multiplier := 1 + math.Max(0, float64(len(effects)-1))*0.1
	for _, raw := range effects {
		effect, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		targeting := ensureMap(effect, "targeting")
		targetCount := math.Max(1, float64(intValue(ensureMap(targeting, "targetCount")["base"])))
		hitCount := 1.0
		if damageSpec, ok := effect["damageSpec"].(map[string]any); ok {
			hitCount = math.Max(1, float64(intValue(ensureMap(damageSpec, "hitCount")["base"])))
		}
		multiplier += math.Max(0, targetCount*hitCount-1) * 0.025
		if intValue(effect["reuseTargetsFromEffect"]) > 0 {
			multiplier += 0.12
		}
	}
	return roundSkillForge(math.Min(3, multiplier))
}

func skillForgeRarityForEffectCount(count int) string {
	switch {
	case count >= 3:
		return "legendary"
	case count == 2:
		return "epic"
	default:
		return "rare"
	}
}

func filterSkillForgeEmptyEffects(effects []any) []any {
	out := make([]any, 0, len(effects))
	for _, raw := range effects {
		effect, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind := stringValue(effect["kind"])
		if kind == "status" {
			payload := ensureMap(effect, "statusPayload")
			if len(ensureSlice(payload, "statusIds")) > 0 {
				out = append(out, effect)
			}
			continue
		}
		if kind == "damage" || kind == "heal" || kind == "resource" {
			formula := ensureMap(effect, "amountFormula")
			if math.Abs(floatValue(formula["baseAdd"])) > 0 || len(ensureSlice(formula, "terms")) > 0 {
				out = append(out, effect)
			}
		}
	}
	return out
}

func skillForgeModuleBudget(module catalog.SkillAlchemyModule, upperValue float64) (float64, float64) {
	kind := stringValue(module.CompileOp["kind"])
	value := math.Abs(upperValue)
	switch kind {
	case "requirement_reduction":
		return value * 2, 0
	case "formula_term", "skill_level_term":
		return value * 4, 0
	case "flat_amount":
		return value * 0.12, 0
	case "target_count", "target_count_max":
		return math.Ceil(value) * 0.8, 0
	case "hit_count":
		return math.Ceil(value), 0
	case "target_scope", "target_position":
		return 0.4, 0
	case "target_direction":
		return 0.55, 0
	case "target_group":
		return 1.1, 0
	case "select_rule":
		return 0.35, 0
	case "status_apply":
		return 1.2 + value*0.3, 0
	case "status_duration":
		return value * 0.3, 0
	case "effect_chain":
		return value * 3, 0
	case "caster_status_condition", "target_status_condition":
		return 0, 0
	case "consume_caster_status", "consume_target_status":
		return value, 0
	case "hp_cost", "mp_cost", "cooldown":
		if stringValue(module.CompileOp["direction"]) == "increase" {
			return 0, value * 0.5
		}
		return value * 0.8, 0
	case "self_current_hp_loss":
		return value * 3, 0
	case "effect_slot":
		return 0, 0
	default:
		return 0, 0
	}
}

func SkillForgeDefinitionHash(definition map[string]any) (string, error) {
	raw, err := json.Marshal(definition)
	if err != nil {
		return "", fmt.Errorf("marshal skill definition: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func validateSkillForgeGraph(graph SkillForgeGraphV1, modules []catalog.SkillAlchemyModule, settings config.SkillForgeSettings) ([]string, map[string]catalog.SkillAlchemyModule, error) {
	if graph.Version != SkillForgeGraphVersion {
		return nil, nil, fmt.Errorf("%w: unsupported version %d", ErrSkillForgeInvalidGraph, graph.Version)
	}
	if graph.Source.Kind != "blank" && graph.Source.Kind != "official" {
		return nil, nil, fmt.Errorf("%w: invalid source kind", ErrSkillForgeInvalidGraph)
	}
	if graph.Source.Kind != "blank" && strings.TrimSpace(graph.Source.SkillID) == "" {
		return nil, nil, fmt.Errorf("%w: source skill id missing", ErrSkillForgeInvalidGraph)
	}
	if len(graph.Nodes) == 0 || len(graph.Nodes) > settings.MaxNodes || len(graph.Edges) > settings.MaxEdges {
		return nil, nil, fmt.Errorf("%w: graph size outside limits", ErrSkillForgeInvalidGraph)
	}
	moduleByID := make(map[string]catalog.SkillAlchemyModule, len(modules))
	for _, module := range modules {
		if !module.Disabled {
			moduleByID[module.ID] = module
		}
	}
	nodes := make(map[string]SkillForgeGraphNode, len(graph.Nodes))
	moduleByNode := make(map[string]catalog.SkillAlchemyModule)
	effectIndexes := make(map[int]string)
	sourceCount, outputCount, materialCount := 0, 0, 0
	for _, node := range graph.Nodes {
		if !validSkillForgeGraphID(node.ID) {
			return nil, nil, fmt.Errorf("%w: invalid node id", ErrSkillForgeInvalidGraph)
		}
		if _, exists := nodes[node.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate node %s", ErrSkillForgeInvalidGraph, node.ID)
		}
		if math.IsNaN(node.Position.X) || math.IsNaN(node.Position.Y) || math.IsInf(node.Position.X, 0) || math.IsInf(node.Position.Y, 0) {
			return nil, nil, fmt.Errorf("%w: invalid node position", ErrSkillForgeInvalidGraph)
		}
		switch node.Kind {
		case "source":
			sourceCount++
		case "output":
			outputCount++
		case "effect":
			if node.EffectKind != "empty" && node.EffectKind != "damage" && node.EffectKind != "heal" && node.EffectKind != "status" && node.EffectKind != "resource" {
				return nil, nil, fmt.Errorf("%w: invalid effect kind", ErrSkillForgeInvalidGraph)
			}
			if prior, exists := effectIndexes[node.EffectIndex]; exists {
				return nil, nil, fmt.Errorf("%w: duplicate effect index on %s and %s", ErrSkillForgeInvalidGraph, prior, node.ID)
			}
			effectIndexes[node.EffectIndex] = node.ID
		case "parameter":
			if skillForgeParameterPortType(node.ParameterKind) == "" || !skillForgeValidParameterPath(node) {
				return nil, nil, fmt.Errorf("%w: invalid DSL parameter node", ErrSkillForgeInvalidGraph)
			}
		case "material":
			materialCount++
			module, exists := moduleByID[node.ModuleID]
			if !exists {
				return nil, nil, fmt.Errorf("%w: unknown module %s", ErrSkillForgeInvalidGraph, node.ModuleID)
			}
			maxQuantity := module.UnitBasis * module.MaxBasisMultiplier
			if node.Quantity < module.UnitBasis || node.Quantity > maxQuantity || node.Quantity%module.UnitBasis != 0 {
				return nil, nil, fmt.Errorf("%w: module quantity must be %d..%d in steps of %d", ErrSkillForgeInvalidGraph, module.UnitBasis, maxQuantity, module.UnitBasis)
			}
			moduleByNode[node.ID] = module
		default:
			return nil, nil, fmt.Errorf("%w: unknown node kind %s", ErrSkillForgeInvalidGraph, node.Kind)
		}
		nodes[node.ID] = node
	}
	if sourceCount != 1 || outputCount != 1 {
		return nil, nil, fmt.Errorf("%w: graph requires one source and one output", ErrSkillForgeInvalidGraph)
	}
	_ = materialCount
	for _, node := range graph.Nodes {
		if node.Kind != "material" || strings.TrimSpace(node.TargetNodeID) == "" {
			continue
		}
		target, exists := nodes[node.TargetNodeID]
		if !exists || target.Kind != "parameter" {
			return nil, nil, fmt.Errorf("%w: material target parameter missing", ErrSkillForgeInvalidGraph)
		}
		module := moduleByNode[node.ID]
		if !skillForgeModuleCanModifyParameter(module, target) {
			return nil, nil, fmt.Errorf("%w: material %s cannot modify parameter %s", ErrSkillForgeInvalidGraph, module.ID, target.ParameterKind)
		}
		if skillForgeParameterUsesEffect(target.ParameterKind) && node.EffectIndex != target.EffectIndex {
			return nil, nil, fmt.Errorf("%w: material effect does not match target parameter", ErrSkillForgeInvalidGraph)
		}
	}
	adj := make(map[string][]string, len(nodes))
	indegree := make(map[string]int, len(nodes))
	reverse := make(map[string][]string, len(nodes))
	edgeIDs := make(map[string]struct{}, len(graph.Edges))
	for _, edge := range graph.Edges {
		if !validSkillForgeGraphID(edge.ID) {
			return nil, nil, fmt.Errorf("%w: edge id missing", ErrSkillForgeInvalidGraph)
		}
		if _, exists := edgeIDs[edge.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate edge %s", ErrSkillForgeInvalidGraph, edge.ID)
		}
		edgeIDs[edge.ID] = struct{}{}
		source, sourceOK := nodes[edge.Source]
		target, targetOK := nodes[edge.Target]
		if !sourceOK || !targetOK || source.ID == target.ID {
			return nil, nil, fmt.Errorf("%w: edge endpoint invalid", ErrSkillForgeInvalidGraph)
		}
		sourceType, ok := skillForgePortType(source, edge.SourceHandle, false, moduleByNode)
		if !ok {
			return nil, nil, fmt.Errorf("%w: source port invalid %s.%s", ErrSkillForgeInvalidGraph, source.ID, edge.SourceHandle)
		}
		targetType, ok := skillForgePortType(target, edge.TargetHandle, true, moduleByNode)
		if !ok || sourceType != targetType {
			return nil, nil, fmt.Errorf("%w: port type mismatch %s -> %s", ErrSkillForgeInvalidGraph, sourceType, targetType)
		}
		adj[source.ID] = append(adj[source.ID], target.ID)
		reverse[target.ID] = append(reverse[target.ID], source.ID)
		indegree[target.ID]++
	}
	queue := make([]string, 0, len(nodes))
	for id := range nodes {
		if indegree[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	ordered := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, id)
		nextIDs := append([]string(nil), adj[id]...)
		sort.Strings(nextIDs)
		for _, next := range nextIDs {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}
	if len(ordered) != len(nodes) {
		return nil, nil, fmt.Errorf("%w: graph contains cycle", ErrSkillForgeInvalidGraph)
	}
	sourceID, outputID := "", ""
	for id, node := range nodes {
		if node.Kind == "source" {
			sourceID = id
		}
		if node.Kind == "output" {
			outputID = id
		}
	}
	reachable := skillForgeReachable(sourceID, adj)
	canReachOutput := skillForgeReachable(outputID, reverse)
	for id, node := range nodes {
		if !reachable[id] {
			return nil, nil, fmt.Errorf("%w: disconnected node %s", ErrSkillForgeInvalidGraph, id)
		}
		if node.Kind == "material" && strings.TrimSpace(node.TargetNodeID) != "" {
			kind := stringValue(moduleByNode[id].CompileOp["kind"])
			if kind != "target_status_condition" && kind != "caster_status_condition" {
				continue
			}
		}
		if !canReachOutput[id] {
			return nil, nil, fmt.Errorf("%w: disconnected node %s", ErrSkillForgeInvalidGraph, id)
		}
	}
	return ordered, moduleByNode, nil
}

func skillForgeValidParameterPath(node SkillForgeGraphNode) bool {
	path := strings.TrimSpace(node.DSLPath)
	switch node.ParameterKind {
	case "requirements":
		return path == "skill.requirements"
	case "hp_cost":
		return path == "skill.hpCost"
	case "mp_cost":
		return path == "skill.mpCost"
	case "cooldown":
		return path == "skill.cooldown"
	}
	if node.EffectIndex < 0 {
		return false
	}
	prefix := fmt.Sprintf("effects.%d.", node.EffectIndex)
	wants := map[string]string{
		"conditions":       "conditions",
		"target_scope":     "targeting.targetScope",
		"target_count":     "targeting.targetCount.base",
		"target_count_max": "targeting.targetCount.max",
		"target_direction": "targeting.secondarySeek",
		"select_rule":      "targeting.selectRule",
		"target_positions": "targeting.targetPositions",
		"formula":          "amountFormula",
		"hit_count":        "damageSpec.hitCount.base",
		"status_apply":     "statusPayload.statusIds",
		"status_duration":  "statusPayload.durationBaseTurns",
		"effect_chain":     "reuseTargetsFromEffect",
	}
	want, ok := wants[node.ParameterKind]
	return ok && path == prefix+want
}

func validSkillForgeGraphID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == ':' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func skillForgeReachable(start string, adjacency map[string][]string) map[string]bool {
	out := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[id] {
			continue
		}
		out[id] = true
		stack = append(stack, adjacency[id]...)
	}
	return out
}

func skillForgePortType(node SkillForgeGraphNode, handle string, input bool, modules map[string]catalog.SkillAlchemyModule) (string, bool) {
	if handle == "" {
		return "", false
	}
	switch node.Kind {
	case "source":
		if input {
			return "", false
		}
		if handle == "effect" || handle == "skill" || handle == "condition" {
			return handle, true
		}
	case "output":
		if !input {
			return "", false
		}
		if handle == "effect" || handle == "skill" || handle == "condition" {
			return handle, true
		}
	case "effect":
		if handle == "in" || handle == "out" {
			return "effect", (input && handle == "in") || (!input && handle == "out")
		}
	case "parameter":
		portType := skillForgeParameterPortType(node.ParameterKind)
		if portType == "" {
			return "", false
		}
		if input && handle == "in" {
			return portType, true
		}
		if !input && handle == "out" {
			return portType, true
		}
		if !input && handle == "modifier" {
			if node.ParameterKind == "conditions" {
				return "condition", true
			}
			return portType, true
		}
	case "material":
		module, ok := modules[node.ID]
		if !ok {
			return "", false
		}
		kind := stringValue(module.CompileOp["kind"])
		if !input && handle == "out" && (kind == "caster_status_condition" || kind == "target_status_condition") {
			return "effect", true
		}
		ports := module.TypedPorts.Outputs
		if input {
			ports = module.TypedPorts.Inputs
		}
		for _, port := range ports {
			if port.ID == handle {
				return port.Type, true
			}
		}
	}
	return "", false
}

func skillForgeParameterPortType(kind string) string {
	switch kind {
	case "requirements", "hp_cost", "mp_cost", "cooldown":
		return "skill"
	case "conditions", "target_scope", "target_count", "target_count_max", "target_direction", "select_rule", "target_positions", "formula", "hit_count", "status_apply", "status_duration", "effect_chain":
		return "effect"
	default:
		return ""
	}
}

func skillForgeParameterUsesEffect(kind string) bool {
	return skillForgeParameterPortType(kind) == "effect"
}

func skillForgeImplicitCustomizationKey(node SkillForgeGraphNode, module catalog.SkillAlchemyModule) string {
	parameterKind := map[string]string{
		"requirement_reduction":   "requirements",
		"formula_term":            "formula",
		"skill_level_term":        "formula",
		"flat_amount":             "formula",
		"target_count":            "target_count",
		"target_count_max":        "target_count_max",
		"target_direction":        "target_direction",
		"hit_count":               "hit_count",
		"target_scope":            "target_scope",
		"target_group":            "target_scope",
		"target_position":         "target_positions",
		"select_rule":             "select_rule",
		"status_apply":            "status_apply",
		"status_duration":         "status_duration",
		"effect_chain":            "effect_chain",
		"caster_status_condition": "conditions",
		"target_status_condition": "conditions",
		"consume_caster_status":   "conditions",
		"consume_target_status":   "conditions",
		"self_current_hp_loss":    "formula",
		"hp_cost":                 "hp_cost",
		"mp_cost":                 "mp_cost",
		"cooldown":                "cooldown",
		"effect_slot":             "effects",
	}[stringValue(module.CompileOp["kind"])]
	if parameterKind == "" {
		return ""
	}
	if skillForgeParameterUsesEffect(parameterKind) {
		return fmt.Sprintf("effects.%d.%s", node.EffectIndex, parameterKind)
	}
	return "skill." + parameterKind
}

func skillForgeModuleCanModifyParameter(module catalog.SkillAlchemyModule, parameter SkillForgeGraphNode) bool {
	kind := stringValue(module.CompileOp["kind"])
	allowed := map[string][]string{
		"requirement_reduction":   {"requirements"},
		"formula_term":            {"formula"},
		"skill_level_term":        {"formula"},
		"flat_amount":             {"formula"},
		"target_count":            {"target_count"},
		"target_count_max":        {"target_count_max"},
		"target_direction":        {"target_direction"},
		"hit_count":               {"hit_count"},
		"target_scope":            {"target_scope"},
		"target_group":            {"target_scope"},
		"target_position":         {"target_positions"},
		"select_rule":             {"select_rule"},
		"status_apply":            {"status_apply"},
		"status_duration":         {"status_duration"},
		"effect_chain":            {"effect_chain"},
		"caster_status_condition": {"conditions"},
		"target_status_condition": {"conditions"},
		"consume_caster_status":   {"conditions"},
		"consume_target_status":   {"conditions"},
		"self_current_hp_loss":    {"formula"},
		"hp_cost":                 {"hp_cost"},
		"mp_cost":                 {"mp_cost"},
		"cooldown":                {"cooldown"},
	}
	matched := false
	for _, value := range allowed[kind] {
		if value == parameter.ParameterKind {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	expected := stringValue(module.CompileOp["effectKind"])
	return expected == "" || parameter.EffectKind == "" || expected == parameter.EffectKind
}

func skillForgeBaseDefinition(graph SkillForgeGraphV1, base map[string]any) (map[string]any, error) {
	if graph.Source.Kind == "blank" {
		return map[string]any{
			"id": "custom:pending", "nameKey": "skillForge.customName", "descKey": "skillForge.customDesc",
			"category": "alchemy", "icon": "game-icons:chemical-arrow", "hpCost": 0, "mpCost": 0, "cooldown": 0,
			"statusRequirement": map[string]any{}, "weaponRequirement": []any{}, "effects": []any{}, "disabled": false,
		}, nil
	}
	if base == nil {
		return nil, fmt.Errorf("%w: source definition missing", ErrSkillForgeInvalidGraph)
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("marshal base skill: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("clone base skill: %w", err)
	}
	out["id"] = "custom:pending"
	out["nameKey"] = "skillForge.customName"
	out["descKey"] = "skillForge.customDesc"
	return out, nil
}

func skillForgeEffects(skill map[string]any) []any {
	raw, ok := skill["effects"].([]any)
	if !ok {
		return []any{}
	}
	out := make([]any, len(raw))
	copy(out, raw)
	return out
}

func newSkillForgeEffect(kind string) map[string]any {
	scope := "enemy"
	if kind == "heal" || kind == "resource" {
		scope = "ally"
	}
	effect := map[string]any{
		"kind": kind,
		"targeting": map[string]any{
			"targetPositions": []any{"vanguard"}, "secondarySeek": "front_to_rear", "targetScope": scope,
			"selectRule": "random", "selectAttrKey": "", "fallbackSelfIfEmpty": false,
			"targetCount": map[string]any{"base": 1, "formula": []any{}},
		},
		"damageType":    map[bool]string{true: "magical", false: "physical"}[kind == "heal"],
		"amountFormula": map[string]any{"baseAdd": 0.0, "terms": []any{}},
	}
	if kind == "status" {
		delete(effect, "damageType")
		delete(effect, "amountFormula")
		effect["statusPayload"] = map[string]any{
			"statusIds":                  []any{},
			"durationBaseTurns":          1,
			"durationPerSkillLevelTurns": 0.0,
			"applyChanceBase":            0.0,
			"applyChancePerSkillLevel":   0.0,
			"applyChanceMin":             0.0,
			"applyChanceMax":             1.0,
		}
	}
	if kind == "empty" {
		delete(effect, "damageType")
		delete(effect, "amountFormula")
	}
	if kind == "resource" {
		delete(effect, "damageType")
		effect["resource"] = "hp"
	}
	return effect
}

func applySkillForgeModule(skill map[string]any, effects *[]any, node SkillForgeGraphNode, module catalog.SkillAlchemyModule, value float64, settings config.SkillForgeSettings) (float64, float64, error) {
	kind := stringValue(module.CompileOp["kind"])
	effectIndex := node.EffectIndex
	needEffect := kind != "effect_slot" && kind != "requirement_reduction" && kind != "hp_cost" && kind != "mp_cost" && kind != "cooldown" && kind != "caster_status_condition" && kind != "target_status_condition"
	if needEffect && (effectIndex < 0 || effectIndex >= len(*effects)) {
		return 0, 0, fmt.Errorf("%w: module %s effect index %d invalid", ErrSkillForgeInvalidGraph, module.ID, effectIndex)
	}
	var effect map[string]any
	if needEffect {
		var ok bool
		effect, ok = (*effects)[effectIndex].(map[string]any)
		if !ok {
			return 0, 0, fmt.Errorf("%w: effect %d malformed", ErrSkillForgeInvalidGraph, effectIndex)
		}
	}
	switch kind {
	case "requirement_reduction":
		ratio := clampSkillForge(math.Max(0, value), 0, 0.75)
		requirements := ensureMap(skill, "statusRequirement")
		for attributeID, raw := range requirements {
			current := intValue(raw)
			if current <= 0 {
				continue
			}
			next := int(math.Round(float64(current) * (1 - ratio)))
			if next < 1 {
				next = 1
			}
			requirements[attributeID] = next
		}
		return ratio * 2, 0, nil
	case "formula_term":
		expected := stringValue(module.CompileOp["effectKind"])
		attrKey := stringValue(module.CompileOp["attrKey"])
		if expected == "heal" && !skillForgeHealingFormulaAttribute(attrKey) {
			return 0, 0, fmt.Errorf("%w: healing formula attribute %s is not supported", ErrSkillForgeInvalidGraph, attrKey)
		}
		if err := ensureSkillForgeEffectKind(effect, expected); err != nil {
			return 0, 0, err
		}
		formula := ensureMap(effect, "amountFormula")
		terms := ensureSlice(formula, "terms")
		updated := false
		if node.TargetNodeID != "" {
			for _, raw := range terms {
				term, ok := raw.(map[string]any)
				if !ok || stringValue(term["attrKey"]) != attrKey {
					continue
				}
				term["coefficient"] = roundSkillForge(floatValue(term["coefficient"]) + value)
				updated = true
				break
			}
		}
		if !updated {
			if len(terms) >= settings.MaxFormulaTermsPerEffect {
				return 0, 0, fmt.Errorf("%w: formula term limit", ErrSkillForgeInvalidGraph)
			}
			terms = append(terms, map[string]any{"attrKey": attrKey, "coefficient": roundSkillForge(value), "perSkillLevel": 0.0})
		}
		formula["terms"] = terms
		return math.Abs(value) * 4, 0, nil
	case "skill_level_term":
		expected := stringValue(module.CompileOp["effectKind"])
		if err := ensureSkillForgeEffectKind(effect, expected); err != nil {
			return 0, 0, err
		}
		formula := ensureMap(effect, "amountFormula")
		terms := ensureSlice(formula, "terms")
		updated := false
		if node.TargetNodeID != "" {
			for _, raw := range terms {
				term, ok := raw.(map[string]any)
				if !ok || stringValue(term["attrKey"]) != "skill_level" {
					continue
				}
				term["coefficient"] = roundSkillForge(floatValue(term["coefficient"]) + value)
				updated = true
				break
			}
		}
		if !updated {
			if len(terms) >= settings.MaxFormulaTermsPerEffect {
				return 0, 0, fmt.Errorf("%w: formula term limit", ErrSkillForgeInvalidGraph)
			}
			terms = append(terms, map[string]any{"attrKey": "skill_level", "coefficient": roundSkillForge(value), "perSkillLevel": 0.0})
		}
		formula["terms"] = terms
		return math.Abs(value) * 4, 0, nil
	case "flat_amount":
		if err := ensureSkillForgeEffectKind(effect, stringValue(module.CompileOp["effectKind"])); err != nil {
			return 0, 0, err
		}
		formula := ensureMap(effect, "amountFormula")
		formula["baseAdd"] = roundSkillForge(floatValue(formula["baseAdd"]) + value)
		return math.Abs(value) * 0.12, 0, nil
	case "target_count":
		targeting := ensureMap(effect, "targeting")
		count := ensureMap(targeting, "targetCount")
		base := 1.0
		if node.TargetNodeID != "" {
			base = math.Max(1, float64(intValue(count["base"])))
		}
		next := int(math.Round(base + value))
		if next < 1 {
			next = 1
		}
		if next > settings.MaxTargetCount {
			return 0, 0, fmt.Errorf("%w: target count limit", ErrSkillForgeInvalidGraph)
		}
		count["base"] = next
		if currentMax := intValue(count["max"]); currentMax > 0 && currentMax < next {
			count["max"] = next
		}
		effect["_skillForgeTargetCountExplicit"] = true
		return math.Max(0, float64(next-1)) * 0.8, 0, nil
	case "target_count_max":
		targeting := ensureMap(effect, "targeting")
		count := ensureMap(targeting, "targetCount")
		base := intValue(count["max"])
		if base < intValue(count["base"]) {
			base = intValue(count["base"])
		}
		if base < 1 {
			base = 1
		}
		next := int(math.Round(float64(base) + value))
		if next < base {
			next = base
		}
		if next > settings.MaxTargetCount {
			return 0, 0, fmt.Errorf("%w: maximum target count limit", ErrSkillForgeInvalidGraph)
		}
		count["max"] = next
		return math.Max(0, float64(next-base)) * 0.9, 0, nil
	case "hit_count":
		if err := ensureSkillForgeEffectKind(effect, "damage"); err != nil {
			return 0, 0, err
		}
		spec := ensureMap(effect, "damageSpec")
		count := ensureMap(spec, "hitCount")
		base := intValue(count["base"])
		if base < 1 {
			base = 1
		}
		next := int(math.Round(float64(base) + value))
		if next > settings.MaxHitCount {
			return 0, 0, fmt.Errorf("%w: hit count limit", ErrSkillForgeInvalidGraph)
		}
		count["base"] = next
		return math.Max(0, float64(next-1)), 0, nil
	case "target_scope":
		targeting := ensureMap(effect, "targeting")
		targeting["targetScope"] = stringValue(module.CompileOp["scope"])
		if excludeSelf, ok := module.CompileOp["excludeSelf"].(bool); ok {
			targeting["excludeSelf"] = excludeSelf
		}
		return 0.4, 0, nil
	case "target_position":
		ensureMap(effect, "targeting")["targetPositions"] = module.CompileOp["positions"]
		return 0.4, 0, nil
	case "target_direction":
		seek := stringValue(module.CompileOp["seek"])
		if seek != "front_to_rear" && seek != "rear_to_front" && seek != "random_alive" && seek != "none" {
			return 0, 0, fmt.Errorf("%w: target direction invalid", ErrSkillForgeInvalidGraph)
		}
		ensureMap(effect, "targeting")["secondarySeek"] = seek
		return 0.55, 0, nil
	case "select_rule":
		targeting := ensureMap(effect, "targeting")
		rule := stringValue(module.CompileOp["rule"])
		targeting["selectRule"] = rule
		if rule == "attr_max" || rule == "attr_min" {
			targeting["selectAttrKey"] = stringValue(module.CompileOp["attrKey"])
		} else {
			delete(targeting, "selectAttrKey")
		}
		return 0.35, 0, nil
	case "target_group":
		targeting := ensureMap(effect, "targeting")
		scope := stringValue(module.CompileOp["scope"])
		if scope != "enemy" && scope != "ally" && scope != "all" && scope != "self" {
			return 0, 0, fmt.Errorf("%w: target group scope invalid", ErrSkillForgeInvalidGraph)
		}
		targeting["targetScope"] = scope
		targeting["targetPositions"] = []any{"vanguard", "midguard", "rearguard"}
		targeting["secondarySeek"] = "random_alive"
		targeting["selectRule"] = "random"
		targeting["excludeSelf"] = module.CompileOp["excludeSelf"] == true
		if effect["_skillForgeTargetCountExplicit"] != true {
			count := settings.MaxTargetCount
			if scope == "self" {
				count = 1
			}
			targeting["targetCount"] = map[string]any{"base": count, "max": count, "formula": []any{}}
		}
		return 1.1, 0, nil
	case "status_apply":
		if err := ensureSkillForgeEffectKind(effect, "status"); err != nil {
			return 0, 0, err
		}
		scope := stringValue(module.CompileOp["targetScope"])
		if scope != "enemy" && scope != "ally" && scope != "self" {
			return 0, 0, fmt.Errorf("%w: status target scope invalid", ErrSkillForgeInvalidGraph)
		}
		payload := ensureMap(effect, "statusPayload")
		statusIDs := ensureSlice(payload, "statusIds")
		currentScope := stringValue(ensureMap(effect, "targeting")["targetScope"])
		if expectedScope := stringValue(effect["_skillForgeStatusScope"]); expectedScope != "" && expectedScope != scope {
			return 0, 0, fmt.Errorf("%w: one status effect cannot mix %s and %s materials", ErrSkillForgeInvalidGraph, expectedScope, scope)
		}
		if len(statusIDs) > 0 && currentScope != "" && currentScope != scope {
			return 0, 0, fmt.Errorf("%w: status material scope %s conflicts with source scope %s", ErrSkillForgeInvalidGraph, scope, currentScope)
		}
		effect["_skillForgeStatusScope"] = scope
		payload["statusIds"] = appendUniqueString(statusIDs, stringValue(module.CompileOp["buffId"]))
		if intValue(payload["durationBaseTurns"]) < 1 {
			payload["durationBaseTurns"] = maxIntValue(1, intValue(module.CompileOp["durationBaseTurns"]))
		}
		chance := clampSkillForge(math.Max(floatValue(payload["applyChanceBase"]), value), 0, 1)
		payload["applyChanceBase"] = roundSkillForge(chance)
		payload["applyChancePerSkillLevel"] = 0.0
		payload["applyChanceMin"] = 0.0
		payload["applyChanceMax"] = 1.0
		ensureMap(effect, "targeting")["targetScope"] = scope
		return 1.2 + math.Abs(value)*0.3, 0, nil
	case "status_duration":
		if err := ensureSkillForgeEffectKind(effect, "status"); err != nil {
			return 0, 0, err
		}
		payload := ensureMap(effect, "statusPayload")
		payload["durationBaseTurns"] = math.Max(1, math.Round(floatValue(payload["durationBaseTurns"])+value))
		return math.Abs(value) * 0.3, 0, nil
	case "effect_chain":
		if effectIndex <= 0 {
			return 0, 0, fmt.Errorf("%w: effect chain requires a later effect", ErrSkillForgeInvalidGraph)
		}
		effect["reuseTargetsFromEffect"] = effectIndex
		return math.Abs(value) * 3, 0, nil
	case "caster_status_condition":
		return 0, 0.5, nil
	case "target_status_condition":
		return 0, 0.5, nil
	case "consume_caster_status":
		effect["consumeCasterStatusesOnHit"] = appendUniqueString(ensureSlice(effect, "consumeCasterStatusesOnHit"), stringValue(module.CompileOp["buffId"]))
		return math.Abs(value), math.Abs(value) * 0.5, nil
	case "consume_target_status":
		effect["consumeStatusesOnHit"] = appendUniqueString(ensureSlice(effect, "consumeStatusesOnHit"), stringValue(module.CompileOp["buffId"]))
		return math.Abs(value), math.Abs(value) * 0.5, nil
	case "self_current_hp_loss":
		if err := ensureSkillForgeEffectKind(effect, "resource"); err != nil {
			return 0, 0, err
		}
		effect["resource"] = "hp"
		effect["targeting"] = map[string]any{
			"targetPositions": []any{"vanguard", "midguard", "rearguard"}, "secondarySeek": "front_to_rear",
			"targetScope": "self", "selectRule": "self", "excludeSelf": false,
			"targetCount": map[string]any{"base": 1, "max": 1, "formula": []any{}},
		}
		formula := ensureMap(effect, "amountFormula")
		formula["baseAdd"] = 0.0
		formula["terms"] = []any{map[string]any{"attrKey": "current_hp", "coefficient": roundSkillForge(-math.Abs(value)), "perSkillLevel": 0.0}}
		return math.Abs(value) * 3, 0, nil
	case "effect_slot":
		return 0, 0, nil
	case "hp_cost", "mp_cost", "cooldown":
		field := map[string]string{"hp_cost": "hpCost", "mp_cost": "mpCost", "cooldown": "cooldown"}[kind]
		current := float64(intValue(skill[field]))
		direction := stringValue(module.CompileOp["direction"])
		delta := math.Max(0, value)
		next := current + delta
		if direction == "reduce" {
			minimum := 0.0
			if kind == "cooldown" {
				minimum = float64(max(1, settings.MinCooldown))
			}
			next = math.Max(minimum, current-delta)
		}
		skill[field] = int(math.Round(next))
		if direction == "increase" {
			return 0, delta * 0.5, nil
		}
		return delta * 0.8, 0, nil
	default:
		return 0, 0, fmt.Errorf("%w: unsupported compile op %s", ErrSkillForgeInvalidGraph, kind)
	}
}

func skillForgeHealingFormulaAttribute(attrKey string) bool {
	switch attrKey {
	case "intelligence", "perception", "mining", "collecting", "farming_animal", "cooking", "crafting", "sewing", "forging", "enhance", "alchemy_level":
		return true
	default:
		return false
	}
}

func clearSkillForgeCompilerMarkers(effects []any) {
	for _, raw := range effects {
		if effect, ok := raw.(map[string]any); ok {
			delete(effect, "_skillForgeTargetCountExplicit")
			delete(effect, "_skillForgeStatusScope")
		}
	}
}

func validateSkillForgeStatusScopes(effects []any) error {
	for _, raw := range effects {
		effect, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		expected := stringValue(effect["_skillForgeStatusScope"])
		if expected == "" {
			continue
		}
		actual := stringValue(ensureMap(effect, "targeting")["targetScope"])
		if actual != expected {
			return fmt.Errorf("%w: status target scope %s conflicts with applied material scope %s", ErrSkillForgeInvalidGraph, actual, expected)
		}
	}
	return nil
}

func ensureSkillForgeEffectKind(effect map[string]any, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	current := stringValue(effect["kind"])
	if current != "" && current != "empty" {
		if current != expected {
			return fmt.Errorf("%w: %s material cannot modify %s effect", ErrSkillForgeInvalidGraph, expected, current)
		}
		return nil
	}
	targeting := effect["targeting"]
	for key := range effect {
		delete(effect, key)
	}
	for key, value := range newSkillForgeEffect(expected) {
		effect[key] = value
	}
	if targeting != nil {
		effect["targeting"] = targeting
	}
	return nil
}

func applySkillForgeConditionBranches(graph SkillForgeGraphV1, modules map[string]catalog.SkillAlchemyModule, effects []any) error {
	nodes := make(map[string]SkillForgeGraphNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	adjacency := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
	}
	for _, node := range graph.Nodes {
		module, ok := modules[node.ID]
		if !ok {
			continue
		}
		kind := stringValue(module.CompileOp["kind"])
		if kind != "caster_status_condition" && kind != "target_status_condition" {
			continue
		}
		buffID := stringValue(module.CompileOp["buffId"])
		if buffID == "" {
			return fmt.Errorf("%w: condition module %s has no status", ErrSkillForgeInvalidGraph, module.ID)
		}
		branchCount := 0
		for _, edge := range graph.Edges {
			if edge.Source != node.ID {
				continue
			}
			isFalse := edge.SourceHandle == "false"
			if !isFalse && edge.SourceHandle != "true" && edge.SourceHandle != "out" {
				continue
			}
			indexes := skillForgeDownstreamEffectIndexes(edge.Target, nodes, adjacency)
			if len(indexes) == 0 {
				if target, ok := nodes[edge.Target]; isFalse && ok && target.Kind == "output" {
					branchCount++
					continue
				}
				return fmt.Errorf("%w: condition branch %s.%s has no effect", ErrSkillForgeInvalidGraph, node.ID, edge.SourceHandle)
			}
			branchCount++
			for _, effectIndex := range indexes {
				if effectIndex < 0 || effectIndex >= len(effects) {
					return fmt.Errorf("%w: condition branch effect index invalid", ErrSkillForgeInvalidGraph)
				}
				effect, ok := effects[effectIndex].(map[string]any)
				if !ok {
					return fmt.Errorf("%w: condition branch effect malformed", ErrSkillForgeInvalidGraph)
				}
				field := "requiredTargetStatuses"
				if kind == "caster_status_condition" {
					field = "requiredCasterStatuses"
				}
				if isFalse {
					field = map[string]string{
						"requiredTargetStatuses": "forbiddenTargetStatuses",
						"requiredCasterStatuses": "forbiddenCasterStatuses",
					}[field]
				}
				effect[field] = appendUniqueString(ensureSlice(effect, field), buffID)
				if kind == "target_status_condition" {
					targeting := ensureMap(effect, "targeting")
					predicateKind := "require_target_has_status"
					if isFalse {
						predicateKind = "exclude_target_has_status"
					}
					targeting["targetPredicates"] = append(ensureSlice(targeting, "targetPredicates"), map[string]any{
						"kind": predicateKind, "statusId": buffID,
					})
				}
			}
		}
		if branchCount == 0 {
			return fmt.Errorf("%w: condition module %s has no true or false branch", ErrSkillForgeInvalidGraph, node.ID)
		}
	}
	return nil
}

func skillForgeDownstreamEffectIndexes(start string, nodes map[string]SkillForgeGraphNode, adjacency map[string][]string) []int {
	seen := map[string]bool{}
	indexes := map[int]struct{}{}
	queue := []string{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		node, ok := nodes[id]
		if !ok {
			continue
		}
		if node.Kind == "effect" {
			indexes[node.EffectIndex] = struct{}{}
			continue
		}
		queue = append(queue, adjacency[id]...)
	}
	out := make([]int, 0, len(indexes))
	for index := range indexes {
		out = append(out, index)
	}
	sort.Ints(out)
	return out
}

func graphNodeByID(nodes []SkillForgeGraphNode, id string) *SkillForgeGraphNode {
	for idx := range nodes {
		if nodes[idx].ID == id {
			return &nodes[idx]
		}
	}
	return nil
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if value, ok := parent[key].(map[string]any); ok {
		return value
	}
	value := map[string]any{}
	parent[key] = value
	return value
}

func ensureSlice(parent map[string]any, key string) []any {
	if value, ok := parent[key].([]any); ok {
		return value
	}
	value := []any{}
	parent[key] = value
	return value
}

func appendUniqueString(values []any, value string) []any {
	for _, current := range values {
		if stringValue(current) == value {
			return values
		}
	}
	return append(values, value)
}

func cloneSkillForgeMap(source map[string]any) map[string]any {
	raw, err := json.Marshal(source)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}
func intValue(value any) int { return int(math.Round(floatValue(value))) }
func floatValue(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		return 0
	}
}
func roundSkillForge(value float64) float64 { return math.Round(value*1_000_000) / 1_000_000 }
func clampSkillForge(value, minValue, maxValue float64) float64 {
	return math.Max(minValue, math.Min(maxValue, value))
}
func maxIntValue(left, right int) int {
	if left > right {
		return left
	}
	return right
}
