package store

import (
	"errors"
	"math"
	"math/rand"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func skillForgeTestModule() catalog.SkillAlchemyModule {
	return catalog.SkillAlchemyModule{
		ID: "skill_forge.test", ItemID: "test_material", NodeKind: "formula",
		TypedPorts: catalog.SkillAlchemyTypedPorts{
			Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "effect"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "effect"}},
		},
		CompileOp:    map[string]any{"kind": "formula_term", "effectKind": "damage", "attrKey": "strength"},
		ValuePerUnit: catalog.SkillAlchemyValueRange{Min: 0.1, Max: 0.2}, UnitBasis: 8, MaxBasisMultiplier: 4,
		MinAlchemyLevel: 1, FailBaseChance: 0.2, FailChancePerLevelAbove: 0.01, SuccessChanceModifier: -0.04, BudgetCreditPerUnit: 2,
		NameParts: map[string]string{"coreKey": "item.test_material.name", "suffixKey": "skillForge.name.mighty"}, Icon: "item:test_material",
	}
}

func skillForgeTestEffectSlotModule() catalog.SkillAlchemyModule {
	return catalog.SkillAlchemyModule{
		ID: "skill_forge.effect_slot", ItemID: "slot_material", NodeKind: "structure",
		TypedPorts: catalog.SkillAlchemyTypedPorts{
			Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "skill"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "skill"}},
		},
		CompileOp: map[string]any{"kind": "effect_slot"}, ValuePerUnit: catalog.SkillAlchemyValueRange{},
		UnitBasis: 1, MaxBasisMultiplier: 2, MinAlchemyLevel: 1, FailBaseChance: 0.1, SuccessChanceModifier: -0.10, BudgetCreditPerUnit: 2,
	}
}

func skillForgeTestGraph() SkillForgeGraphV1 {
	return SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "blank"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "material", Kind: "material", ModuleID: "skill_forge.test", Quantity: 8, EffectIndex: 0},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
			{ID: "e2", Source: "effect", SourceHandle: "out", Target: "material", TargetHandle: "in"},
			{ID: "e3", Source: "material", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		},
	}
}

func TestCompileSkillForgeGraphBlankSkill(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	settings.RulesetVersion = 7
	result, err := CompileSkillForgeGraph(skillForgeTestGraph(), nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	if !result.Valid {
		t.Fatal("expected valid preview")
	}
	if result.RulesetVersion != 7 {
		t.Fatalf("rulesetVersion=%d want 7", result.RulesetVersion)
	}
	if result.Recipe["test_material"] != 8 {
		t.Fatalf("recipe=%v", result.Recipe)
	}
	if len(result.Ranges) != 1 || result.Ranges[0].Min != 0.8 || result.Ranges[0].Max != 1.6 {
		t.Fatalf("ranges=%#v", result.Ranges)
	}
	effects, ok := result.CompiledSkill["effects"].([]any)
	if !ok || len(effects) != 1 {
		t.Fatalf("effects=%#v", result.CompiledSkill["effects"])
	}
	nameParams, ok := result.CompiledSkill["nameParams"].(map[string]any)
	if !ok || stringValue(nameParams["materialKey"]) != "item.test_material.name" || stringValue(nameParams["traitKey"]) != "skillForge.name.mighty" {
		t.Fatalf("compiled skill identity=%#v", result.CompiledSkill["nameParams"])
	}
	if stringValue(result.CompiledSkill["icon"]) != "item:test_material" {
		t.Fatalf("compiled skill icon=%#v", result.CompiledSkill["icon"])
	}
}

func TestCompileSkillForgeGraphBuildsStatusChanceAndDurationFromSeparateMaterials(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	apply := skillForgeTestModule()
	apply.ID = "skill_forge.status_apply"
	apply.ItemID = "status_material"
	apply.NodeKind = "status"
	apply.CompileOp = map[string]any{"kind": "status_apply", "buffId": "armor_break", "targetScope": "enemy", "durationBaseTurns": 1}
	apply.ValuePerUnit = catalog.SkillAlchemyValueRange{}
	apply.UnitBasis = 4
	apply.BudgetCreditPerUnit = 2
	duration := skillForgeTestModule()
	duration.ID = "skill_forge.status_duration"
	duration.ItemID = "duration_material"
	duration.NodeKind = "status"
	duration.CompileOp = map[string]any{"kind": "status_duration"}
	duration.ValuePerUnit = catalog.SkillAlchemyValueRange{Min: 0.125, Max: 0.125}
	duration.BudgetCreditPerUnit = 2
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "blank"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect", Kind: "effect", EffectKind: "status", EffectIndex: 0},
			{ID: "apply", Kind: "material", ModuleID: apply.ID, Quantity: 12, EffectIndex: 0},
			{ID: "duration", Kind: "material", ModuleID: duration.ID, Quantity: 8, EffectIndex: 0},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
			{ID: "e2", Source: "effect", SourceHandle: "out", Target: "apply", TargetHandle: "in"},
			{ID: "e3", Source: "apply", SourceHandle: "out", Target: "duration", TargetHandle: "in"},
			{ID: "e4", Source: "duration", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		},
	}

	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{apply, duration}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	effects := result.CompiledSkill["effects"].([]any)
	status := effects[0].(map[string]any)
	payload := status["statusPayload"].(map[string]any)
	if got := floatValue(payload["applyChanceBase"]); got != 0.45 {
		t.Fatalf("applyChanceBase=%v want 0.45 for three material steps", got)
	}
	if got := intValue(payload["durationBaseTurns"]); got != 2 {
		t.Fatalf("durationBaseTurns=%v want 2", got)
	}
	if got := ensureSlice(payload, "statusIds"); len(got) != 1 || stringValue(got[0]) != "armor_break" {
		t.Fatalf("statusIds=%#v", got)
	}
	if got := stringValue(ensureMap(status, "targeting")["targetScope"]); got != "enemy" {
		t.Fatalf("targetScope=%q want enemy", got)
	}
	graph.Nodes[2].Quantity = 4
	oneStep, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{apply, duration}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("one-step CompileSkillForgeGraph failed: %v", err)
	}
	oneStepPayload := oneStep.CompiledSkill["effects"].([]any)[0].(map[string]any)["statusPayload"].(map[string]any)
	if got := floatValue(oneStepPayload["applyChanceBase"]); got != 0.25 {
		t.Fatalf("one-step applyChanceBase=%v want default 0.25", got)
	}
}

func TestCompileSkillForgeGraphRejectsStatusMaterialOnDamageEffect(t *testing.T) {
	module := skillForgeTestModule()
	module.NodeKind = "status"
	module.CompileOp = map[string]any{"kind": "status_apply", "buffId": "armor_break"}
	if _, err := CompileSkillForgeGraph(skillForgeTestGraph(), nil, []catalog.SkillAlchemyModule{module}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected status material on damage effect to be rejected")
	}
}

func TestCompileSkillForgeGraphRejectsMixedStatusTargetScopes(t *testing.T) {
	module := func(id, itemID, buffID, scope string) catalog.SkillAlchemyModule {
		out := skillForgeTestModule()
		out.ID = id
		out.ItemID = itemID
		out.NodeKind = "status"
		out.CompileOp = map[string]any{"kind": "status_apply", "buffId": buffID, "targetScope": scope, "durationBaseTurns": 1}
		out.ValuePerUnit = catalog.SkillAlchemyValueRange{Min: 0.05, Max: 0.05}
		return out
	}
	enemy := module("skill_forge.enemy_status", "enemy_status_material", "armor_break", "enemy")
	ally := module("skill_forge.ally_status", "ally_status_material", "blessing", "ally")
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "blank"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect", Kind: "effect", EffectKind: "status", EffectIndex: 0},
			{ID: "enemy", Kind: "material", ModuleID: enemy.ID, Quantity: 8, EffectIndex: 0},
			{ID: "ally", Kind: "material", ModuleID: ally.ID, Quantity: 8, EffectIndex: 0},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
			{ID: "e2", Source: "effect", SourceHandle: "out", Target: "enemy", TargetHandle: "in"},
			{ID: "e3", Source: "enemy", SourceHandle: "out", Target: "ally", TargetHandle: "in"},
			{ID: "e4", Source: "ally", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		},
	}
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{enemy, ally}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected mixed ally/enemy status materials on one effect to be rejected")
	}
}

func TestCompileSkillForgeGraphTargetGroupMaterialSetsWholeBranchTargeting(t *testing.T) {
	formula := skillForgeTestModule()
	group := skillForgeTestModule()
	group.ID = "skill_forge.all_allies"
	group.ItemID = "all_allies_material"
	group.NodeKind = "target"
	group.CompileOp = map[string]any{"kind": "target_group", "scope": "ally", "excludeSelf": true}
	group.ValuePerUnit = catalog.SkillAlchemyValueRange{}
	group.UnitBasis = 4
	group.MaxBasisMultiplier = 6
	group.BudgetCreditPerUnit = 0.3
	graph := skillForgeTestGraph()
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "group", Kind: "material", ModuleID: group.ID, Quantity: 4, EffectIndex: 0})
	graph.Edges = []SkillForgeGraphEdge{
		{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
		{ID: "e2", Source: "effect", SourceHandle: "out", Target: "material", TargetHandle: "in"},
		{ID: "e3", Source: "material", SourceHandle: "out", Target: "group", TargetHandle: "in"},
		{ID: "e4", Source: "group", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
	}
	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{formula, group}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	effect := result.CompiledSkill["effects"].([]any)[0].(map[string]any)
	targeting := ensureMap(effect, "targeting")
	if stringValue(targeting["targetScope"]) != "ally" || targeting["excludeSelf"] != true {
		t.Fatalf("target group not compiled: %#v", targeting)
	}
	count := ensureMap(targeting, "targetCount")
	if intValue(count["base"]) != config.DefaultGlobalSettings().SkillForge.MaxTargetCount {
		t.Fatalf("target count=%#v", count)
	}
}

func TestCompileSkillForgeGraphCompilesTargetDirectionMaximumAndGoldCost(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	module := func(id, itemID string, op map[string]any, min, max float64) catalog.SkillAlchemyModule {
		return catalog.SkillAlchemyModule{
			ID: id, ItemID: itemID, NodeKind: "target",
			TypedPorts: catalog.SkillAlchemyTypedPorts{
				Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "effect"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "effect"}},
			},
			CompileOp: op, ValuePerUnit: catalog.SkillAlchemyValueRange{Min: min, Max: max}, UnitBasis: 100, MaxBasisMultiplier: 2,
			MinAlchemyLevel: 1, FailBaseChance: 0.1, BudgetCreditPerUnit: 1,
		}
	}
	direction := module("skill_forge.direction", "direction_material", map[string]any{"kind": "target_direction", "seek": "rear_to_front"}, 0, 0)
	maximum := module("skill_forge.maximum", "maximum_material", map[string]any{"kind": "target_count_max"}, 0.01, 0.01)
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "official", SkillID: "test_skill"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "direction", Kind: "parameter", ParameterKind: "target_direction", DSLPath: "effects.0.targeting.secondarySeek", EffectKind: "damage", EffectIndex: 0},
			{ID: "maximum", Kind: "parameter", ParameterKind: "target_count_max", DSLPath: "effects.0.targeting.targetCount.max", EffectKind: "damage", EffectIndex: 0},
			{ID: "direction-material", Kind: "material", ModuleID: direction.ID, Quantity: 100, EffectIndex: 0, TargetNodeID: "direction"},
			{ID: "maximum-material", Kind: "material", ModuleID: maximum.ID, Quantity: 100, EffectIndex: 0, TargetNodeID: "maximum"},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
			{ID: "e2", Source: "effect", SourceHandle: "out", Target: "direction", TargetHandle: "in"},
			{ID: "e3", Source: "direction", SourceHandle: "out", Target: "maximum", TargetHandle: "in"},
			{ID: "e4", Source: "maximum", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e5", Source: "direction", SourceHandle: "modifier", Target: "direction-material", TargetHandle: "in"},
			{ID: "e6", Source: "maximum", SourceHandle: "modifier", Target: "maximum-material", TargetHandle: "in"},
		},
	}
	base := map[string]any{
		"id": "test_skill", "cooldown": 2,
		"effects": []any{map[string]any{"kind": "damage", "targeting": map[string]any{
			"targetScope": "enemy", "secondarySeek": "front_to_rear", "selectRule": "random", "targetPositions": []any{"vanguard"},
			"targetCount": map[string]any{"base": 1, "max": 2, "formula": []any{}},
		}}},
	}
	result, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{direction, maximum}, settings, SkillForgeDerived{Level: 50}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	targeting := ensureMap(result.CompiledSkill["effects"].([]any)[0].(map[string]any), "targeting")
	if got := stringValue(targeting["secondarySeek"]); got != "rear_to_front" {
		t.Fatalf("secondarySeek=%q", got)
	}
	if got := intValue(ensureMap(targeting, "targetCount")["max"]); got != 3 {
		t.Fatalf("targetCount.max=%d", got)
	}
	if result.GoldCost != 20750 || result.Recipe[settings.GoldCostItemID] != 20750 {
		t.Fatalf("gold cost=%d breakdown=%#v recipe=%#v", result.GoldCost, result.GoldCostBreakdown, result.Recipe)
	}
	if result.Recipe[settings.CustomizedNodeSealItemID] != 2*settings.CustomizedNodeSealCount || result.Recipe[settings.ForgeCatalystItemID] != settings.ForgeCatalystCount {
		t.Fatalf("mandatory recipe=%#v", result.Recipe)
	}
}

func TestCompileSkillForgeGraphBuildsTrueAndFalseStatusBranches(t *testing.T) {
	formula := skillForgeTestModule()
	condition := skillForgeTestModule()
	condition.ID = "skill_forge.target_condition"
	condition.ItemID = "condition_material"
	condition.NodeKind = "condition"
	condition.TypedPorts = catalog.SkillAlchemyTypedPorts{
		Inputs:  []catalog.SkillAlchemyPort{{ID: "in", Type: "condition"}},
		Outputs: []catalog.SkillAlchemyPort{{ID: "true", Type: "effect"}, {ID: "false", Type: "effect"}},
	}
	condition.CompileOp = map[string]any{"kind": "target_status_condition", "buffId": "armor_break"}
	condition.ValuePerUnit = catalog.SkillAlchemyValueRange{}
	slot := skillForgeTestEffectSlotModule()
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "blank"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "condition", Kind: "material", ModuleID: condition.ID, Quantity: 8},
			{ID: "slot", Kind: "material", ModuleID: slot.ID, Quantity: 1},
			{ID: "effect-true", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "effect-false", Kind: "effect", EffectKind: "damage", EffectIndex: 1},
			{ID: "formula-true", Kind: "material", ModuleID: formula.ID, Quantity: 8, EffectIndex: 0},
			{ID: "formula-false", Kind: "material", ModuleID: formula.ID, Quantity: 8, EffectIndex: 1},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "condition", Target: "condition", TargetHandle: "in"},
			{ID: "e2", Source: "condition", SourceHandle: "true", Target: "effect-true", TargetHandle: "in"},
			{ID: "e3", Source: "condition", SourceHandle: "false", Target: "effect-false", TargetHandle: "in"},
			{ID: "e4", Source: "effect-true", SourceHandle: "out", Target: "formula-true", TargetHandle: "in"},
			{ID: "e5", Source: "formula-true", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e6", Source: "effect-false", SourceHandle: "out", Target: "formula-false", TargetHandle: "in"},
			{ID: "e7", Source: "formula-false", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e8", Source: "source", SourceHandle: "skill", Target: "slot", TargetHandle: "in"},
			{ID: "e9", Source: "slot", SourceHandle: "out", Target: "output", TargetHandle: "skill"},
		},
	}
	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{formula, condition, slot}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	effects := result.CompiledSkill["effects"].([]any)
	trueEffect := effects[0].(map[string]any)
	falseEffect := effects[1].(map[string]any)
	if got := ensureSlice(trueEffect, "requiredTargetStatuses"); len(got) != 1 || stringValue(got[0]) != "armor_break" {
		t.Fatalf("true branch conditions=%#v", got)
	}
	if got := ensureSlice(falseEffect, "forbiddenTargetStatuses"); len(got) != 1 || stringValue(got[0]) != "armor_break" {
		t.Fatalf("false branch conditions=%#v", got)
	}
	truePredicates := ensureSlice(ensureMap(trueEffect, "targeting"), "targetPredicates")
	falsePredicates := ensureSlice(ensureMap(falseEffect, "targeting"), "targetPredicates")
	if len(truePredicates) != 1 || stringValue(truePredicates[0].(map[string]any)["kind"]) != "require_target_has_status" {
		t.Fatalf("true branch target predicates=%#v", truePredicates)
	}
	if len(falsePredicates) != 1 || stringValue(falsePredicates[0].(map[string]any)["kind"]) != "exclude_target_has_status" {
		t.Fatalf("false branch target predicates=%#v", falsePredicates)
	}
}

func TestCompileSkillForgeGraphBuildsThreeEffectExampleFromMaterials(t *testing.T) {
	module := func(id, itemID, nodeKind string, op map[string]any, min, max float64) catalog.SkillAlchemyModule {
		return catalog.SkillAlchemyModule{
			ID: id, ItemID: itemID, NodeKind: nodeKind,
			TypedPorts: catalog.SkillAlchemyTypedPorts{
				Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "effect"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "effect"}},
			},
			CompileOp: op, ValuePerUnit: catalog.SkillAlchemyValueRange{Min: min, Max: max}, UnitBasis: 1, MaxBasisMultiplier: 8,
			MinAlchemyLevel: 1, FailBaseChance: 0.1, BudgetCreditPerUnit: 20,
		}
	}
	slot := skillForgeTestEffectSlotModule()
	freeze := module("skill_forge.freeze", "freeze_material", "status", map[string]any{"kind": "status_apply", "buffId": "frozen_action_suppressed", "targetScope": "enemy", "durationBaseTurns": 1}, 1, 1)
	allEnemies := module("skill_forge.all_enemies", "all_enemy_material", "target", map[string]any{"kind": "target_group", "scope": "enemy", "excludeSelf": false}, 0, 0)
	healLevel := module("skill_forge.heal_level", "heal_level_material", "formula", map[string]any{"kind": "skill_level_term", "effectKind": "heal"}, 0.1, 0.1)
	healInt := module("skill_forge.heal_int", "heal_int_material", "formula", map[string]any{"kind": "formula_term", "effectKind": "heal", "attrKey": "intelligence"}, 0.01, 0.01)
	allies := module("skill_forge.allies", "ally_material", "target", map[string]any{"kind": "target_group", "scope": "ally", "excludeSelf": false}, 0, 0)
	twoTargets := module("skill_forge.two_targets", "target_count_material", "target", map[string]any{"kind": "target_count"}, 1, 1)
	selfLoss := module("skill_forge.self_loss", "self_loss_material", "formula", map[string]any{"kind": "self_current_hp_loss", "effectKind": "resource"}, 0.3, 0.3)
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "blank"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect-1", Kind: "effect", EffectKind: "empty", EffectIndex: 0},
			{ID: "freeze", Kind: "material", ModuleID: freeze.ID, Quantity: 1, EffectIndex: 0},
			{ID: "all-enemies", Kind: "material", ModuleID: allEnemies.ID, Quantity: 1, EffectIndex: 0},
			{ID: "effect-2", Kind: "effect", EffectKind: "empty", EffectIndex: 1},
			{ID: "heal-level", Kind: "material", ModuleID: healLevel.ID, Quantity: 1, EffectIndex: 1},
			{ID: "heal-int", Kind: "material", ModuleID: healInt.ID, Quantity: 1, EffectIndex: 1},
			{ID: "allies", Kind: "material", ModuleID: allies.ID, Quantity: 1, EffectIndex: 1},
			{ID: "two-targets", Kind: "material", ModuleID: twoTargets.ID, Quantity: 1, EffectIndex: 1},
			{ID: "effect-3", Kind: "effect", EffectKind: "empty", EffectIndex: 2},
			{ID: "self-loss", Kind: "material", ModuleID: selfLoss.ID, Quantity: 1, EffectIndex: 2},
			{ID: "slot", Kind: "material", ModuleID: slot.ID, Quantity: 2},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect-1", TargetHandle: "in"},
			{ID: "e2", Source: "effect-1", SourceHandle: "out", Target: "freeze", TargetHandle: "in"},
			{ID: "e3", Source: "freeze", SourceHandle: "out", Target: "all-enemies", TargetHandle: "in"},
			{ID: "e4", Source: "all-enemies", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e5", Source: "source", SourceHandle: "effect", Target: "effect-2", TargetHandle: "in"},
			{ID: "e6", Source: "effect-2", SourceHandle: "out", Target: "heal-level", TargetHandle: "in"},
			{ID: "e7", Source: "heal-level", SourceHandle: "out", Target: "heal-int", TargetHandle: "in"},
			{ID: "e8", Source: "heal-int", SourceHandle: "out", Target: "allies", TargetHandle: "in"},
			{ID: "e9", Source: "allies", SourceHandle: "out", Target: "two-targets", TargetHandle: "in"},
			{ID: "e10", Source: "two-targets", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e11", Source: "source", SourceHandle: "effect", Target: "effect-3", TargetHandle: "in"},
			{ID: "e12", Source: "effect-3", SourceHandle: "out", Target: "self-loss", TargetHandle: "in"},
			{ID: "e13", Source: "self-loss", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e14", Source: "source", SourceHandle: "skill", Target: "slot", TargetHandle: "in"},
			{ID: "e15", Source: "slot", SourceHandle: "out", Target: "output", TargetHandle: "skill"},
		},
	}
	modules := []catalog.SkillAlchemyModule{slot, freeze, allEnemies, healLevel, healInt, allies, twoTargets, selfLoss}
	result, err := CompileSkillForgeGraph(graph, nil, modules, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 50}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	effects := result.CompiledSkill["effects"].([]any)
	if len(effects) != 3 || stringValue(result.CompiledSkill["rarity"]) != "legendary" {
		t.Fatalf("effects=%d rarity=%v", len(effects), result.CompiledSkill["rarity"])
	}
	status := effects[0].(map[string]any)
	if stringValue(status["kind"]) != "status" || stringValue(ensureMap(status, "targeting")["targetScope"]) != "enemy" || intValue(ensureMap(ensureMap(status, "targeting"), "targetCount")["base"]) != config.DefaultGlobalSettings().SkillForge.MaxTargetCount {
		t.Fatalf("freeze effect=%#v", status)
	}
	heal := effects[1].(map[string]any)
	terms := ensureSlice(ensureMap(heal, "amountFormula"), "terms")
	if stringValue(heal["kind"]) != "heal" || len(terms) != 2 || stringValue(terms[0].(map[string]any)["attrKey"]) != "skill_level" || floatValue(terms[0].(map[string]any)["coefficient"]) != 0.1 || stringValue(terms[1].(map[string]any)["attrKey"]) != "intelligence" || floatValue(terms[1].(map[string]any)["coefficient"]) != 0.01 {
		t.Fatalf("heal effect=%#v", heal)
	}
	if intValue(ensureMap(ensureMap(heal, "targeting"), "targetCount")["base"]) != 2 {
		t.Fatalf("heal target count=%#v", ensureMap(heal, "targeting"))
	}
	if _, leaked := heal["_skillForgeTargetCountExplicit"]; leaked {
		t.Fatalf("compiler marker leaked into player skill DSL: %#v", heal)
	}
	loss := effects[2].(map[string]any)
	lossTerms := ensureSlice(ensureMap(loss, "amountFormula"), "terms")
	if stringValue(loss["kind"]) != "resource" || stringValue(loss["resource"]) != "hp" || stringValue(ensureMap(loss, "targeting")["targetScope"]) != "self" || len(lossTerms) != 1 || floatValue(lossTerms[0].(map[string]any)["coefficient"]) != -0.3 {
		t.Fatalf("self loss effect=%#v", loss)
	}
}

func TestCompileSkillForgeGraphAllowsLifeLevelHealingAttribute(t *testing.T) {
	module := skillForgeTestModule()
	module.CompileOp = map[string]any{"kind": "formula_term", "effectKind": "heal", "attrKey": "collecting"}
	graph := skillForgeTestGraph()
	graph.Nodes[1].EffectKind = "empty"
	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{module}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("expected collecting-level healing formula to compile: %v", err)
	}
	terms := ensureSlice(ensureMap(result.CompiledSkill["effects"].([]any)[0].(map[string]any), "amountFormula"), "terms")
	if len(terms) != 1 || stringValue(terms[0].(map[string]any)["attrKey"]) != "collecting" {
		t.Fatalf("unexpected life-level healing formula: %#v", terms)
	}
}

func TestCompileSkillForgeGraphRejectsUnsupportedHealingAttribute(t *testing.T) {
	module := skillForgeTestModule()
	module.CompileOp = map[string]any{"kind": "formula_term", "effectKind": "heal", "attrKey": "strength"}
	graph := skillForgeTestGraph()
	graph.Nodes[1].EffectKind = "empty"
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{module}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected strength-based healing formula to be rejected")
	}
}

func TestCompileSkillForgeGraphRejectsCycleAndPortMismatch(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	graph := skillForgeTestGraph()
	graph.Edges = append(graph.Edges, SkillForgeGraphEdge{ID: "cycle", Source: "material", SourceHandle: "out", Target: "effect", TargetHandle: "in"})
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected cycle rejection")
	}
	graph = skillForgeTestGraph()
	graph.Edges[1].SourceHandle = "skill"
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected port mismatch rejection")
	}
}

func TestCompileSkillForgeGraphRejectsDisconnectedAndExcessQuantity(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	graph := skillForgeTestGraph()
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "disconnected", Kind: "material", ModuleID: "skill_forge.test", Quantity: 1})
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected disconnected node rejection")
	}

	graph = skillForgeTestGraph()
	graph.Nodes[2].Quantity = skillForgeTestModule().UnitBasis*int64(skillForgeTestModule().MaxBasisMultiplier) + 1
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected module quantity limit rejection")
	}

	graph = skillForgeTestGraph()
	graph.Nodes[2].Quantity = skillForgeTestModule().UnitBasis - 1
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected quantity below one material batch to be rejected")
	}
}

func TestCompileSkillForgeGraphRollsAndSoftFailureFloor(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	derived := SkillForgeDerived{Level: 20, Precision: 1, Stability: 1}
	rolled, err := CompileSkillForgeGraph(skillForgeTestGraph(), nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, derived, rand.New(rand.NewSource(9)), skillForgeRoll)
	if err != nil {
		t.Fatalf("roll compile failed: %v", err)
	}
	floor, err := CompileSkillForgeGraph(skillForgeTestGraph(), nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, derived, rand.New(rand.NewSource(9)), skillForgeFloor)
	if err != nil {
		t.Fatalf("floor compile failed: %v", err)
	}
	if floor.Ranges[0].Rolled == nil || *floor.Ranges[0].Rolled != floor.Ranges[0].Min {
		t.Fatalf("floor roll=%v min=%v", floor.Ranges[0].Rolled, floor.Ranges[0].Min)
	}
	if rolled.Ranges[0].Rolled == nil || *rolled.Ranges[0].Rolled < rolled.Ranges[0].Min || *rolled.Ranges[0].Rolled > rolled.Ranges[0].Max {
		t.Fatalf("rolled range=%#v", rolled.Ranges[0])
	}
}

func TestCompileSkillForgeGraphRejectsStaleOfficialSource(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	graph := skillForgeTestGraph()
	graph.Source = SkillForgeGraphSource{Kind: "official", SkillID: "power_strike", DefinitionHash: "stale"}
	base := map[string]any{"id": "power_strike", "effects": []any{newSkillForgeEffect("damage")}}
	if _, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err != ErrSkillForgeSourceStale {
		t.Fatalf("error=%v want ErrSkillForgeSourceStale", err)
	}
}

func TestCompileSkillForgeGraphOfficialParameterSlotChainsMultipleMaterials(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	module := skillForgeTestModule()
	module.CompileOp = map[string]any{"kind": "target_count"}
	module.ValuePerUnit = catalog.SkillAlchemyValueRange{Min: 1, Max: 1}
	module.UnitBasis = 1
	module.MaxBasisMultiplier = 2
	module.BudgetCreditPerUnit = 10
	baseEffect := newSkillForgeEffect("damage")
	ensureMap(ensureMap(baseEffect, "targeting"), "targetCount")["base"] = 4
	base := map[string]any{"id": "arrow_rain", "effects": []any{baseEffect}}
	graph := SkillForgeGraphV1{
		Version: 1,
		Source:  SkillForgeGraphSource{Kind: "official", SkillID: "arrow_rain"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect-1", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "target-count", Kind: "parameter", ParameterKind: "target_count", DSLPath: "effects.0.targeting.targetCount.base", EffectKind: "damage", EffectIndex: 0},
			{ID: "material-1", Kind: "material", ModuleID: module.ID, Quantity: 1, EffectIndex: 0, TargetNodeID: "target-count"},
			{ID: "material-2", Kind: "material", ModuleID: module.ID, Quantity: 1, EffectIndex: 0, TargetNodeID: "target-count"},
			{ID: "material-3", Kind: "material", ModuleID: module.ID, Quantity: 1, EffectIndex: 0, TargetNodeID: "target-count"},
			{ID: "material-4", Kind: "material", ModuleID: module.ID, Quantity: 1, EffectIndex: 0, TargetNodeID: "target-count"},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect-1", TargetHandle: "in"},
			{ID: "e2", Source: "effect-1", SourceHandle: "out", Target: "target-count", TargetHandle: "in"},
			{ID: "e3", Source: "target-count", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e4", Source: "target-count", SourceHandle: "modifier", Target: "material-1", TargetHandle: "in"},
			{ID: "e5", Source: "material-1", SourceHandle: "out", Target: "material-2", TargetHandle: "in"},
			{ID: "e6", Source: "material-2", SourceHandle: "out", Target: "material-3", TargetHandle: "in"},
			{ID: "e7", Source: "material-3", SourceHandle: "out", Target: "material-4", TargetHandle: "in"},
		},
	}
	result, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{module}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	count := ensureMap(ensureMap(result.CompiledSkill["effects"].([]any)[0].(map[string]any), "targeting"), "targetCount")
	if got := intValue(count["base"]); got != 8 {
		t.Fatalf("target count=%d want original 4 + four chained materials", got)
	}
	if len(result.Ranges) != 4 || result.Ranges[0].TargetNodeID != "target-count" || result.Ranges[3].TargetNodeID != "target-count" {
		t.Fatalf("range target=%#v", result.Ranges)
	}
	if got := result.Recipe[settings.CustomizedNodeSealItemID]; got != 4*settings.CustomizedNodeSealCount {
		t.Fatalf("customization seal cost=%d want %d for four modifications on one parameter", got, 4*settings.CustomizedNodeSealCount)
	}
	if got := result.Recipe[settings.ForgeCatalystItemID]; got != settings.ForgeCatalystCount {
		t.Fatalf("forge catalyst cost=%d want %d", got, settings.ForgeCatalystCount)
	}
}

func TestCompileSkillForgeGraphCooldownCannotDropBelowConfiguredMinimum(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	module := catalog.SkillAlchemyModule{
		ID: "skill_forge.cooldown_reduce", ItemID: "cooldown_material", NodeKind: "cost",
		TypedPorts: catalog.SkillAlchemyTypedPorts{
			Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "skill"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "skill"}},
		},
		CompileOp: map[string]any{"kind": "cooldown", "direction": "reduce"}, ValuePerUnit: catalog.SkillAlchemyValueRange{Min: 10, Max: 10},
		UnitBasis: 1, MaxBasisMultiplier: 1, MinAlchemyLevel: 1, FailBaseChance: 0.1, BudgetCreditPerUnit: 100,
	}
	base := map[string]any{"id": "instant_skill", "cooldown": 2, "effects": []any{newSkillForgeEffect("damage")}}
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "official", SkillID: "instant_skill"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "cooldown", Kind: "parameter", ParameterKind: "cooldown", DSLPath: "skill.cooldown"},
			{ID: "material", Kind: "material", ModuleID: module.ID, Quantity: 1, TargetNodeID: "cooldown"},
			{ID: "effect", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "skill", Target: "cooldown", TargetHandle: "in"},
			{ID: "e2", Source: "cooldown", SourceHandle: "out", Target: "output", TargetHandle: "skill"},
			{ID: "e3", Source: "cooldown", SourceHandle: "modifier", Target: "material", TargetHandle: "in"},
			{ID: "e4", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
			{ID: "e5", Source: "effect", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		},
	}
	result, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{module}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	if got := intValue(result.CompiledSkill["cooldown"]); got != settings.MinCooldown {
		t.Fatalf("cooldown=%d want configured minimum %d", got, settings.MinCooldown)
	}
	if len(result.Ranges) != 1 || result.Ranges[0].Direction != "reduce" {
		t.Fatalf("cooldown range direction=%#v", result.Ranges)
	}
}

func TestCompileSkillForgeGraphRejectsMaterialOnIncompatibleParameterSlot(t *testing.T) {
	graph := skillForgeTestGraph()
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "target-count", Kind: "parameter", ParameterKind: "target_count", DSLPath: "effects.0.targeting.targetCount.base", EffectKind: "damage", EffectIndex: 0})
	graph.Nodes[2].TargetNodeID = "target-count"
	graph.Edges = []SkillForgeGraphEdge{
		{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
		{ID: "e2", Source: "effect", SourceHandle: "out", Target: "target-count", TargetHandle: "in"},
		{ID: "e3", Source: "target-count", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		{ID: "e4", Source: "target-count", SourceHandle: "modifier", Target: "material", TargetHandle: "in"},
		{ID: "e5", Source: "material", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
	}
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected formula material on target-count slot to be rejected")
	}
}

func TestCompileSkillForgeGraphOfficialPreviewWithoutMaterialsStillShowsBaseDSL(t *testing.T) {
	base := map[string]any{"id": "arrow_rain", "effects": []any{newSkillForgeEffect("damage")}}
	graph := SkillForgeGraphV1{
		Version: 1,
		Source:  SkillForgeGraphSource{Kind: "official", SkillID: "arrow_rain"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect-1", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect-1", TargetHandle: "in"},
			{ID: "e2", Source: "effect-1", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		},
	}
	result, err := CompileSkillForgeGraph(graph, base, nil, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if result.Valid || len(result.BlockingReasons) != 1 || len(result.CompiledSkill["effects"].([]any)) != 1 {
		t.Fatalf("expected blocked base preview, got %+v", result)
	}
}

func TestCompileSkillForgeGraphReducesOfficialAttributeRequirements(t *testing.T) {
	base := map[string]any{
		"id": "test_skill", "statusRequirement": map[string]any{"intelligence": 40.0},
		"weaponRequirement": []any{"staff"}, "effects": []any{newSkillForgeEffect("damage")},
	}
	hash, err := SkillForgeDefinitionHash(base)
	if err != nil {
		t.Fatal(err)
	}
	module := catalog.SkillAlchemyModule{
		ID: "skill_forge.requirement", ItemID: "requirement_material", NodeKind: "cost",
		TypedPorts: catalog.SkillAlchemyTypedPorts{
			Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "skill"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "skill"}},
		},
		CompileOp: map[string]any{"kind": "requirement_reduction"}, ValuePerUnit: catalog.SkillAlchemyValueRange{Min: 0.25, Max: 0.25},
		UnitBasis: 1, MaxBasisMultiplier: 1, MinAlchemyLevel: 1, FailBaseChance: 0.1, BudgetCreditPerUnit: 2,
	}
	graph := SkillForgeGraphV1{
		Version: 1, Source: SkillForgeGraphSource{Kind: "official", SkillID: "test_skill", DefinitionHash: hash},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "effect", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "requirements", Kind: "parameter", ParameterKind: "requirements", DSLPath: "skill.requirements"},
			{ID: "material", Kind: "material", ModuleID: module.ID, Quantity: 1, TargetNodeID: "requirements"},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "skill", Target: "requirements", TargetHandle: "in"},
			{ID: "e2", Source: "requirements", SourceHandle: "out", Target: "output", TargetHandle: "skill"},
			{ID: "e3", Source: "requirements", SourceHandle: "modifier", Target: "material", TargetHandle: "in"},
			{ID: "e4", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
			{ID: "e5", Source: "effect", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		},
	}
	result, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{module}, config.DefaultGlobalSettings().SkillForge, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	requirements := result.CompiledSkill["statusRequirement"].(map[string]any)
	if got := intValue(requirements["intelligence"]); got != 30 {
		t.Fatalf("intelligence requirement=%d want 30", got)
	}
	if weapons := ensureSlice(result.CompiledSkill, "weaponRequirement"); len(weapons) != 1 || stringValue(weapons[0]) != "staff" {
		t.Fatalf("weaponRequirement=%#v", weapons)
	}
}

func TestCompileSkillForgeGraphOfficialConditionCanSkipEffectOnFalseBranch(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	condition := skillForgeTestModule()
	condition.ID = "skill_forge.official_condition"
	condition.ItemID = "official_condition_material"
	condition.NodeKind = "condition"
	condition.TypedPorts = catalog.SkillAlchemyTypedPorts{
		Inputs:  []catalog.SkillAlchemyPort{{ID: "in", Type: "condition"}},
		Outputs: []catalog.SkillAlchemyPort{{ID: "true", Type: "effect"}, {ID: "false", Type: "effect"}},
	}
	condition.CompileOp = map[string]any{"kind": "target_status_condition", "buffId": "armor_break"}
	condition.ValuePerUnit = catalog.SkillAlchemyValueRange{}
	condition.UnitBasis = 1
	condition.MaxBasisMultiplier = 1
	condition.BudgetCreditPerUnit = 2
	base := map[string]any{"id": "power_strike", "effects": []any{newSkillForgeEffect("damage")}}
	graph := SkillForgeGraphV1{
		Version: 1,
		Source:  SkillForgeGraphSource{Kind: "official", SkillID: "power_strike"},
		Nodes: []SkillForgeGraphNode{
			{ID: "source", Kind: "source"},
			{ID: "conditions", Kind: "parameter", ParameterKind: "conditions", DSLPath: "effects.0.conditions", EffectKind: "damage", EffectIndex: 0},
			{ID: "effect-1", Kind: "effect", EffectKind: "damage", EffectIndex: 0},
			{ID: "condition", Kind: "material", ModuleID: condition.ID, Quantity: 1, EffectIndex: 0, TargetNodeID: "conditions"},
			{ID: "output", Kind: "output"},
		},
		Edges: []SkillForgeGraphEdge{
			{ID: "e1", Source: "source", SourceHandle: "effect", Target: "conditions", TargetHandle: "in"},
			{ID: "e2", Source: "conditions", SourceHandle: "out", Target: "effect-1", TargetHandle: "in"},
			{ID: "e3", Source: "effect-1", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
			{ID: "e4", Source: "conditions", SourceHandle: "modifier", Target: "condition", TargetHandle: "in"},
			{ID: "e5", Source: "condition", SourceHandle: "true", Target: "effect-1", TargetHandle: "in"},
			{ID: "e6", Source: "condition", SourceHandle: "false", Target: "output", TargetHandle: "effect"},
		},
	}
	result, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{condition}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	effect := result.CompiledSkill["effects"].([]any)[0].(map[string]any)
	if got := ensureSlice(effect, "requiredTargetStatuses"); len(got) != 1 || stringValue(got[0]) != "armor_break" {
		t.Fatalf("required target statuses=%#v", got)
	}
}

func TestCompileSkillForgeGraphAppendsEffectToOfficialSource(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	graph := skillForgeTestGraph()
	graph.Source = SkillForgeGraphSource{Kind: "official", SkillID: "power_strike"}
	graph.Nodes[1].ID = "effect-0"
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "effect-1", Kind: "effect", EffectKind: "damage", EffectIndex: 1})
	graph.Nodes[2].EffectIndex = 1
	slot := skillForgeTestEffectSlotModule()
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "slot", Kind: "material", ModuleID: slot.ID, Quantity: 1})
	graph.Edges = []SkillForgeGraphEdge{
		{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect-0", TargetHandle: "in"},
		{ID: "e2", Source: "effect-0", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		{ID: "e3", Source: "source", SourceHandle: "effect", Target: "effect-1", TargetHandle: "in"},
		{ID: "e4", Source: "effect-1", SourceHandle: "out", Target: "material", TargetHandle: "in"},
		{ID: "e5", Source: "material", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
		{ID: "e6", Source: "source", SourceHandle: "skill", Target: "slot", TargetHandle: "in"},
		{ID: "e7", Source: "slot", SourceHandle: "out", Target: "output", TargetHandle: "skill"},
	}
	base := map[string]any{"id": "power_strike", "effects": []any{newSkillForgeEffect("damage")}}
	result, err := CompileSkillForgeGraph(graph, base, []catalog.SkillAlchemyModule{skillForgeTestModule(), slot}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	effects, ok := result.CompiledSkill["effects"].([]any)
	if !ok || len(effects) != 2 {
		t.Fatalf("effects=%#v", result.CompiledSkill["effects"])
	}
}

func TestCompileSkillForgeGraphValidatesUpperBoundLimits(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	module := skillForgeTestModule()
	module.CompileOp = map[string]any{"kind": "target_count"}
	module.ValuePerUnit = catalog.SkillAlchemyValueRange{Min: 0, Max: 8}
	module.UnitBasis = 1
	module.MaxBasisMultiplier = 1
	module.BudgetCreditPerUnit = 20
	graph := skillForgeTestGraph()
	graph.Nodes[2].Quantity = 1
	if _, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{module}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview); err == nil {
		t.Fatal("expected upper-bound target limit rejection")
	}
}

func TestCompileSkillForgeGraphRollsParametersIndependently(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	graph := skillForgeTestGraph()
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "material-2", Kind: "material", ModuleID: "skill_forge.test", Quantity: 8, EffectIndex: 0})
	graph.Edges = []SkillForgeGraphEdge{
		{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
		{ID: "e2", Source: "effect", SourceHandle: "out", Target: "material", TargetHandle: "in"},
		{ID: "e3", Source: "material", SourceHandle: "out", Target: "material-2", TargetHandle: "in"},
		{ID: "e4", Source: "material-2", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
	}
	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{Level: 20}, rand.New(rand.NewSource(4)), skillForgeRoll)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	if len(result.Ranges) != 2 || result.Ranges[0].Rolled == nil || result.Ranges[1].Rolled == nil || *result.Ranges[0].Rolled == *result.Ranges[1].Rolled {
		t.Fatalf("expected independent rolls, got %#v", result.Ranges)
	}
}

func TestSkillForgeIgnoresAlchemyDerivedModifiers(t *testing.T) {
	module := skillForgeTestModule()
	module.FailBaseChance = 0.4
	module.FailChancePerLevelAbove = 0.02
	settings := config.DefaultGlobalSettings().SkillForge
	graph := skillForgeTestGraph()
	low, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{module}, settings, SkillForgeDerived{Level: 1, Precision: -1, Stability: -1}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("low-derived compile failed: %v", err)
	}
	high, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{module}, settings, SkillForgeDerived{Level: 99, Precision: 9, Stability: 9}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("high-derived compile failed: %v", err)
	}
	if low.FailChance != 0.74 || high.FailChance != low.FailChance {
		t.Fatalf("skill forge failure chance must ignore alchemy stats and legacy module failure: low=%v high=%v", low.FailChance, high.FailChance)
	}
	if len(low.Ranges) != 1 || len(high.Ranges) != 1 || low.Ranges[0] != high.Ranges[0] {
		t.Fatalf("alchemy-derived stats must not change skill forge ranges: low=%#v high=%#v", low.Ranges, high.Ranges)
	}
}

func TestSkillForgeTradeoffRefundIsBounded(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	formula := skillForgeTestModule()
	cost := skillForgeTestModule()
	cost.ID = "skill_forge.cost"
	cost.ItemID = "cost_material"
	cost.NodeKind = "cost"
	cost.TypedPorts = catalog.SkillAlchemyTypedPorts{
		Inputs: []catalog.SkillAlchemyPort{{ID: "in", Type: "skill"}}, Outputs: []catalog.SkillAlchemyPort{{ID: "out", Type: "skill"}},
	}
	cost.CompileOp = map[string]any{"kind": "hp_cost", "direction": "increase"}
	cost.ValuePerUnit = catalog.SkillAlchemyValueRange{Min: 100, Max: 100}
	cost.UnitBasis = 1
	cost.MaxBasisMultiplier = 1
	cost.SuccessChanceModifier = 0.04
	cost.BudgetCreditPerUnit = 20
	graph := skillForgeTestGraph()
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "cost", Kind: "material", ModuleID: cost.ID, Quantity: 1})
	graph.Edges = append(graph.Edges,
		SkillForgeGraphEdge{ID: "cost-1", Source: "source", SourceHandle: "skill", Target: "cost", TargetHandle: "in"},
		SkillForgeGraphEdge{ID: "cost-2", Source: "cost", SourceHandle: "out", Target: "output", TargetHandle: "skill"},
	)
	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{formula, cost}, settings, SkillForgeDerived{Level: 20}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	gross := result.BudgetUsed + result.BudgetRefund
	if result.BudgetRefund > gross*settings.MaxTradeoffRefundRatio+0.000001 {
		t.Fatalf("refund=%v gross=%v cap=%v", result.BudgetRefund, gross, settings.MaxTradeoffRefundRatio)
	}
	if result.MaterialSuccessModifier != 0 || result.FailChance != 0.70 {
		t.Fatalf("cost tradeoff must offset the formula-node success penalty: %+v", result)
	}
}

func TestSkillForgeAddsMaterialSuccessModifiersPerInvestedBasis(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	settings.BaseSuccessChance = 0.70
	high := skillForgeTestModule()
	high.SuccessChanceModifier = -0.08
	low := skillForgeTestModule()
	low.ID = "skill_forge.low_failure"
	low.ItemID = "low_failure_material"
	low.SuccessChanceModifier = -0.03
	graph := skillForgeTestGraph()
	graph.Nodes[2].Quantity = 32
	graph.Nodes = append(graph.Nodes, SkillForgeGraphNode{ID: "low", Kind: "material", ModuleID: low.ID, Quantity: 8, EffectIndex: 0})
	graph.Edges = []SkillForgeGraphEdge{
		{ID: "e1", Source: "source", SourceHandle: "effect", Target: "effect", TargetHandle: "in"},
		{ID: "e2", Source: "effect", SourceHandle: "out", Target: "material", TargetHandle: "in"},
		{ID: "e3", Source: "material", SourceHandle: "out", Target: "low", TargetHandle: "in"},
		{ID: "e4", Source: "low", SourceHandle: "out", Target: "output", TargetHandle: "effect"},
	}
	result, err := CompileSkillForgeGraph(graph, nil, []catalog.SkillAlchemyModule{high, low}, settings, SkillForgeDerived{Level: 1}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	want := 0.65
	if result.FailChance != want {
		t.Fatalf("failChance=%v want base 30%% plus four high-risk shares and one low-risk share %v", result.FailChance, want)
	}
	if result.MaterialSuccessModifier != -0.35 || len(result.Ranges) != 2 || result.Ranges[0].SuccessChanceModifier != -0.32 || result.Ranges[1].SuccessChanceModifier != -0.03 {
		t.Fatalf("material modifiers must be explicit and auditable: %+v", result)
	}
}

func TestSkillForgeBookCountAndChanceBoundaries(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	for _, invalid := range []int64{0, 199, 250, 1001} {
		if validSkillForgeBookCount(invalid, settings) {
			t.Fatalf("count %d must be rejected", invalid)
		}
	}
	wants := map[int64]float64{200: 0, 300: 0.10, 400: 0.20, 500: 0.30, 600: 0.40, 700: 0.50, 800: 0.60, 900: 0.70, 1000: 0.80}
	for count, want := range wants {
		if !validSkillForgeBookCount(count, settings) {
			t.Fatalf("count %d must be accepted", count)
		}
		if got := skillForgeBookBonus(count, settings); math.Abs(got-want) > 0.000001 {
			t.Fatalf("count %d bonus=%v want=%v", count, got, want)
		}
	}
	testSettings := settings
	testSettings.SourceSkillbookMinCount = 1
	testSettings.SourceSkillbookMaxCount = 1
	testSettings.SourceSkillbookCountStep = 1
	if !validSkillForgeBookCount(1, testSettings) || validSkillForgeBookCount(2, testSettings) {
		t.Fatal("test environment must accept exactly one source skillbook")
	}
	testSettings.SourceSkillbookCountStep = 0
	if validSkillForgeBookCount(1, testSettings) {
		t.Fatal("invalid server configuration must reject counts instead of panicking")
	}
}

func TestSkillForgeDirectChainCannotBypassCustomizedNodeSealCost(t *testing.T) {
	settings := config.DefaultGlobalSettings().SkillForge
	result, err := CompileSkillForgeGraph(skillForgeTestGraph(), nil, []catalog.SkillAlchemyModule{skillForgeTestModule()}, settings, SkillForgeDerived{}, nil, skillForgePreview)
	if err != nil {
		t.Fatalf("CompileSkillForgeGraph failed: %v", err)
	}
	if got := result.Recipe[settings.CustomizedNodeSealItemID]; got != settings.CustomizedNodeSealCount {
		t.Fatalf("direct-chain seal cost=%d want %d", got, settings.CustomizedNodeSealCount)
	}
}

func TestSkillForgeRequestedRecipeRequiresFullInputsBeforeFailureRefund(t *testing.T) {
	requested, itemIDs, err := skillForgeRequestedRecipe(map[string]int64{
		"gold_coin":  1200,
		"skill_book": 25,
	}, "skill_book", 200)
	if err != nil {
		t.Fatalf("skillForgeRequestedRecipe failed: %v", err)
	}
	if got, want := requested["skill_book"], int64(225); got != want {
		t.Fatalf("combined source book quantity=%d, want %d", got, want)
	}
	if len(itemIDs) != 2 || itemIDs[0] != "gold_coin" || itemIDs[1] != "skill_book" {
		t.Fatalf("unexpected sorted recipe ids: %#v", itemIDs)
	}
	if _, _, err := skillForgeRequestedRecipe(map[string]int64{"gold_coin": 0}, "skill_book", 200); !errors.Is(err, ErrSkillForgeInvalidGraph) {
		t.Fatalf("invalid recipe error=%v, want ErrSkillForgeInvalidGraph", err)
	}
}

func TestSkillForgeFailureReceiptKeepsRetryResultWithoutHeavyDSL(t *testing.T) {
	now := time.Now()
	row := SkillForgeRevision{
		ID:              "revision:failed",
		CustomSkillID:   "custom:failed",
		DraftID:         "draft:1",
		Graph:           skillForgeTestGraph(),
		CompiledSkill:   map[string]any{"effects": []any{newSkillForgeEffect("damage")}},
		RequestedRecipe: map[string]int64{"book": 200},
		ConsumedRecipe:  map[string]int64{"book": 100},
		RefundedRecipe:  map[string]int64{"book": 100},
		Failed:          true,
		ResultStatus:    "failed",
		CompilerVersion: 10,
		RulesetVersion:  1,
		CreatedAt:       now,
	}
	receipt := skillForgeFailureReceipt(row)
	if len(receipt.Graph.Nodes) != 0 || receipt.CompiledSkill != nil || len(receipt.Rolls) != 0 {
		t.Fatalf("failure receipt retained heavy DSL payload: %#v", receipt)
	}
	if receipt.ConsumedRecipe["book"] != 100 || receipt.RefundedRecipe["book"] != 100 || !receipt.Failed || receipt.ResultStatus != "failed" {
		t.Fatalf("failure receipt lost retry outcome: %#v", receipt)
	}
}

func TestApplySkillbookChanceDoesNotChangeBudgetOrRanges(t *testing.T) {
	rolled := 0.42
	result := SkillForgeCompileResult{
		BaseSuccessChance: 0.30, MaterialSuccessModifier: -0.08, BudgetUsed: 7, BudgetLimit: 12, BudgetRefund: 2,
		Ranges: []SkillForgeParameterRange{{NodeID: "n", Min: 1, Max: 2, Rolled: &rolled}},
	}
	draft := SkillForgeDraft{SourceKind: "official", SourceSkillbookItemID: "skillbook_power_strike", SourceSkillbookCount: 1000}
	applySkillbookChance(&result, draft, config.DefaultGlobalSettings().SkillForge)
	if result.BaseFailChance != 0.78 || result.SkillbookBonus != 0.80 || result.FinalFailChance != 0 || result.FailChance != 0 {
		t.Fatalf("unexpected chance projection: %+v", result)
	}
	if result.BudgetUsed != 7 || result.BudgetLimit != 12 || result.BudgetRefund != 2 || len(result.Ranges) != 1 || result.Ranges[0].Rolled == nil || *result.Ranges[0].Rolled != rolled {
		t.Fatalf("skillbook count changed compiler values: %+v", result)
	}
}
