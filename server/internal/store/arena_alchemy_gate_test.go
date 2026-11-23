package store

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func arenaAlchemyGateTestStore() *Store {
	return &Store{gameData: catalog.Data{
		Attributes: []catalog.AttributeDefinition{
			{ID: "speed", Category: "battle", Unit: "number"},
			{ID: "guard", Category: "battle", Unit: "number"},
			{ID: "retired_power", Category: "battle", Unit: "number"},
			{ID: "battle_drop_rate_bonus", Category: "battle", Unit: "percent"},
			{ID: "alchemy_craft_success", Category: "work", Unit: "percent"},
		},
		ItemTraits: []catalog.ItemTraitDefinition{{
			ItemID: "test_material",
			Traits: []catalog.AlchemyTraitEntry{{
				TraitID:            "test_trait",
				Family:             "speed",
				Kind:               "combat",
				EffectType:         "add_stat",
				UnitBasis:          1,
				MaxBasisMultiplier: 1,
				Effects: []catalog.AlchemyTraitEffect{{
					Attr:         "speed",
					ValuePerUnit: catalog.AlchemyValueRange{Min: 0.005, Max: 0.01},
				}},
			}},
		}},
		AlchemyComboRecipes: []catalog.AlchemyComboRecipe{{
			ID:     "test_combo",
			Kind:   "combat",
			Inputs: map[string]int64{"test_material": 1},
			OutputAffix: catalog.AlchemyComboOutputAffix{
				EffectType: "battle_hook",
				Params: map[string]any{"attrMods": map[string]any{
					"guard": map[string]any{"base": 10.0, "alchemyValueCoeff": -1.0},
				}},
				ValueRange: catalog.AlchemyValueRange{Min: 0.5, Max: 1},
			},
		}},
	}}
}

func arenaAlchemyGateTestAffix(sourceKind string, value float64, quantity int64) EquipmentAffixSlot {
	component := alchemyCraftComponent{
		SourceKind:   "trait_effect",
		ItemID:       "test_material",
		TraitID:      "test_trait",
		Family:       "speed",
		Attr:         "speed",
		Quantity:     quantity,
		CurrentValue: value,
	}
	return EquipmentAffixSlot{
		SlotIndex:    2,
		AffixID:      "custom:test",
		SourceKind:   sourceKind,
		AttrMods:     map[string]float64{"speed": value, "battle_drop_rate_bonus": 999, "alchemy_craft_success": 999},
		EffectParams: map[string]any{"alchemyComponents": []alchemyCraftComponent{component}},
	}
}

func TestAlchemyCraftSlotStampsCurrentRulesetVersion(t *testing.T) {
	slot := alchemyCraftSlot(resolvedAlchemyCraft{EffectType: "add_stat", EffectParams: map[string]any{}, AttrMods: map[string]float64{"speed": 1}})
	if slot.AlchemyRulesetVersion != AlchemyCurrentRulesetVersion {
		t.Fatalf("alchemy ruleset version=%q want %q", slot.AlchemyRulesetVersion, AlchemyCurrentRulesetVersion)
	}
}

func TestAlchemyCraftComponentStampsFrozenOperationEffects(t *testing.T) {
	capacity, _ := json.Marshal(alchemyeffect.CapacityFloorTradeoffParams{
		ExtraMaterialQuantity:      10,
		PositiveFloorReductionRate: 0.2,
	})
	precision, _ := json.Marshal(alchemyeffect.PrecisionCeilingBonusParams{PositiveCeilingBonusRate: 0.1})
	craft := resolvedAlchemyCraft{
		EffectParams: map[string]any{},
		Components:   []alchemyCraftComponent{{SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait"}},
	}
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{
		{GrantID: 123, DefinitionID: "capacity-food", RulesetVersion: "food-v1", Domain: operationeffect.DomainAlchemy, Kind: alchemyeffect.KindInfusionCapacityFloorTradeoff, Params: capacity},
		{GrantID: 456, DefinitionID: "precision-food", RulesetVersion: "food-v1", Domain: operationeffect.DomainAlchemy, Kind: alchemyeffect.KindInfusionPrecisionCeilingBonus, Params: precision},
	}}

	if err := stampAlchemyCraftOperationEffects(&craft, snapshot); err != nil {
		t.Fatal(err)
	}
	if len(craft.Components) != 1 || craft.Components[0].OperationEffects == nil {
		t.Fatalf("operation effect proof was not stamped: %#v", craft.Components)
	}
	encoded, err := json.Marshal(craft.EffectParams)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("grantId")) || bytes.Contains(encoded, []byte("123")) || bytes.Contains(encoded, []byte("456")) {
		t.Fatalf("private grant identity leaked into affix provenance: %s", encoded)
	}
}

func TestArenaAlchemyEquipmentViolationsAcceptsStampedInfusionCeiling(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	capacity, _ := json.Marshal(alchemyeffect.CapacityFloorTradeoffParams{
		ExtraMaterialQuantity:      10,
		PositiveFloorReductionRate: 0.2,
	})
	precision, _ := json.Marshal(alchemyeffect.PrecisionCeilingBonusParams{PositiveCeilingBonusRate: 0.25})
	craft := resolvedAlchemyCraft{
		EffectType:   "add_stat",
		EffectParams: map[string]any{"alchemyKind": "combat"},
		AttrMods:     map[string]float64{"speed": 0.8},
		Components: []alchemyCraftComponent{{
			SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait", Attr: "speed", Quantity: 42, CurrentValue: 0.8,
		}},
	}
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{
		{DefinitionID: "capacity-food", RulesetVersion: "food-v1", Domain: operationeffect.DomainAlchemy, Kind: alchemyeffect.KindInfusionCapacityFloorTradeoff, Params: capacity},
		{DefinitionID: "precision-food", RulesetVersion: "food-v1", Domain: operationeffect.DomainAlchemy, Kind: alchemyeffect.KindInfusionPrecisionCeilingBonus, Params: precision},
	}}
	if err := stampAlchemyCraftOperationEffects(&craft, snapshot); err != nil {
		t.Fatal(err)
	}
	persisted, err := json.Marshal(craft.EffectParams)
	if err != nil {
		t.Fatal(err)
	}
	craft.EffectParams = map[string]any{}
	if err := json.Unmarshal(persisted, &craft.EffectParams); err != nil {
		t.Fatal(err)
	}
	stamped := alchemyCraftSlot(craft)
	item := EquippedItem{InstanceID: 85, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{stamped}}
	if violations := store.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(violations) != 0 {
		t.Fatalf("valid stamped infusion was rejected: %#v", violations)
	}

	unstamped := stamped
	unstamped.AlchemyRulesetVersion = ""
	if violations := store.ArenaAlchemyEquipmentViolations([]EquippedItem{{InstanceID: 86, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{unstamped}}}); len(violations) != 1 {
		t.Fatalf("unstamped value must keep the legacy ceiling, got %#v", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsRejectsInvalidOperationEffectStamp(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	invalidParams := json.RawMessage(`{"positiveCeilingBonusRate":0.9}`)
	component := alchemyCraftComponent{
		SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait", Attr: "speed", Quantity: 42, CurrentValue: 0.8,
		OperationEffects: &alchemyOperationEffectStamp{
			Version: alchemyOperationEffectStampVersion,
			Rules: []alchemyOperationEffectStampRule{{
				DefinitionID: "forged", RulesetVersion: "food-v1", Kind: alchemyeffect.KindInfusionPrecisionCeilingBonus, Params: invalidParams,
			}},
		},
	}
	affix := EquipmentAffixSlot{
		AffixID:               "custom:invalid-stamp",
		SourceKind:            "craft",
		AlchemyRulesetVersion: AlchemyCurrentRulesetVersion,
		AttrMods:              map[string]float64{"speed": 0.8},
		EffectParams:          map[string]any{"alchemyComponents": []alchemyCraftComponent{component}},
	}
	violations := store.ArenaAlchemyEquipmentViolations([]EquippedItem{{InstanceID: 87, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}})
	if len(violations) != 1 || violations[0].AllowedValue >= 0.8 {
		t.Fatalf("invalid proof widened arena ceiling: %#v", violations)
	}
}

func TestAlchemyInheritSlotStampsCurrentRulesetVersion(t *testing.T) {
	slot := alchemyInheritSlot(resolvedAlchemyInherit{EffectType: "add_stat", EffectParams: map[string]any{}, AttrMods: map[string]float64{"speed": 1}})
	if slot.AlchemyRulesetVersion != AlchemyCurrentRulesetVersion {
		t.Fatalf("alchemy inherit ruleset version=%q want %q", slot.AlchemyRulesetVersion, AlchemyCurrentRulesetVersion)
	}
}

func TestArenaAlchemyEquipmentViolationsTargetsOnlyExcessiveCombatValues(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	legal := arenaAlchemyGateTestAffix("craft", 0.015, 1)
	legacyExcessive := arenaAlchemyGateTestAffix("craft", 0.7, 100000)
	items := []EquippedItem{{
		InstanceID:     81,
		TemplateItemID: "test_blade",
		CustomData:     ItemCustomization{Name: "Old Edge"},
		EquipType:      "battle",
		EquippedSlot:   "right_hand",
		Affixes:        []EquipmentAffixSlot{legal, legacyExcessive},
	}}

	violations := store.ArenaAlchemyEquipmentViolations(items)
	if len(violations) != 1 {
		t.Fatalf("violations=%#v want one excessive speed result", violations)
	}
	got := violations[0]
	if got.Attribute != "speed" || got.EquipmentInstanceID != 81 || got.TemplateItemID != "test_blade" || got.EquipmentName != "Old Edge" {
		t.Fatalf("unexpected violation identity: %#v", got)
	}
	if got.ActualValue != 0.7 || got.AllowedValue <= 0 || got.AllowedValue >= got.ActualValue {
		t.Fatalf("unexpected violation values: %#v", got)
	}
	if got.Reason != "legacy_unversioned_value_exceeds_current_ruleset" {
		t.Fatalf("reason=%q", got.Reason)
	}
}

func TestArenaAlchemyEquipmentViolationsUsesFrozenFishInfusionRange(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	fishAffix := func(value float64, stamp map[string]any) EquipmentAffixSlot {
		return EquipmentAffixSlot{
			SlotIndex:             1,
			AffixID:               "custom:fish-carp",
			SourceKind:            "fish",
			AlchemyRulesetVersion: FishingInfusionRulesetVersion,
			AttrMods:              map[string]float64{"speed": value},
			EffectParams: map[string]any{
				"attr":         "speed",
				"alchemyKind":  "combat",
				"fishInfusion": stamp,
			},
		}
	}
	validStamp := map[string]any{
		"version":    FishingInfusionRulesetVersion,
		"valueRange": map[string]any{"min": 0.01, "max": 0.02},
	}
	legal := fishAffix(0.02, validStamp)
	if violations := store.ArenaAlchemyEquipmentViolations([]EquippedItem{{InstanceID: 82, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{legal}}}); len(violations) != 0 {
		t.Fatalf("legal frozen fish infusion rejected: %#v", violations)
	}

	excessive := fishAffix(0.03, validStamp)
	violations := store.ArenaAlchemyEquipmentViolations([]EquippedItem{{InstanceID: 83, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{excessive}}})
	if len(violations) != 1 || violations[0].AllowedValue != 0.02 || violations[0].Reason != "value_exceeds_fish_infusion_profile" {
		t.Fatalf("excessive fish infusion violations=%#v", violations)
	}

	unstamped := fishAffix(0.001, map[string]any{})
	violations = store.ArenaAlchemyEquipmentViolations([]EquippedItem{{InstanceID: 84, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{unstamped}}})
	if len(violations) != 1 || violations[0].AllowedValue != 0 {
		t.Fatalf("unstamped fish infusion must fail closed: %#v", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsChecksInheritedLegacyValues(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := arenaAlchemyGateTestAffix("inherit", 0.06, 1)
	affix.AlchemyRulesetVersion = AlchemyCurrentRulesetVersion
	items := []EquippedItem{{InstanceID: 91, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}}

	violations := store.ArenaAlchemyEquipmentViolations(items)
	if len(violations) != 1 || violations[0].Attribute != "speed" {
		t.Fatalf("inherited violations=%#v want speed", violations)
	}
	if violations[0].Reason != "value_exceeds_current_ruleset" {
		t.Fatalf("reason=%q", violations[0].Reason)
	}
}

func TestArenaAlchemyEquipmentViolationsRecognizesLegacyCustomAffixAndInheritedParams(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	legacy := arenaAlchemyGateTestAffix("template", 0.7, 100000)
	legacy.EffectParams = nil
	inherited := arenaAlchemyGateTestAffix("template", 0.04, 1)
	inherited.EffectParams["inheritSourceKind"] = "inherit"
	items := []EquippedItem{{InstanceID: 95, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{legacy, inherited}}}

	violations := store.ArenaAlchemyEquipmentViolations(items)
	if len(violations) != 1 || violations[0].ActualValue != 0.7 {
		t.Fatalf("legacy custom violations=%#v want only the excessive unversioned affix", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsKeepsUnresolvedComponentFallback(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := arenaAlchemyGateTestAffix("craft", 0.2, 1)
	affix.EffectParams["alchemyComponents"] = []alchemyCraftComponent{
		{SourceKind: "trait_effect", ItemID: "missing_material", TraitID: "missing_trait", Attr: "speed", Quantity: 1},
		{SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait", Attr: "speed", Quantity: 1},
	}
	items := []EquippedItem{{InstanceID: 96, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}}

	if violations := store.ArenaAlchemyEquipmentViolations(items); len(violations) != 0 {
		t.Fatalf("partially unresolved legacy components must use the conservative catalog ceiling, got %#v", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsRejectsUnsupportedPositiveBattleAttribute(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := arenaAlchemyGateTestAffix("craft", 0, 1)
	affix.AttrMods = map[string]float64{"retired_power": 0.001}
	items := []EquippedItem{{InstanceID: 98, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}}

	violations := store.ArenaAlchemyEquipmentViolations(items)
	if len(violations) != 1 || violations[0].Attribute != "retired_power" || violations[0].AllowedValue != 0 {
		t.Fatalf("unsupported battle attribute violations=%#v", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsRejectsExcessiveComboPower(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := EquipmentAffixSlot{
		AffixID:    "custom:combo",
		SourceKind: "craft",
		EffectParams: map[string]any{
			"alchemyKind": "combat",
			"comboPower":  1_000.0,
			"alchemyComponents": []alchemyCraftComponent{
				{SourceKind: "combo_output", ComboRecipeID: "test_combo", CurrentValue: 1_000},
				{SourceKind: "combo_material", ItemID: "test_material", Quantity: 100_000},
			},
		},
	}
	items := []EquippedItem{{InstanceID: 99, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}}

	violations := store.ArenaAlchemyEquipmentViolations(items)
	if len(violations) != 1 || violations[0].Attribute != arenaAlchemyComboPowerAttribute || violations[0].AllowedValue <= 0 || violations[0].AllowedValue >= violations[0].ActualValue {
		t.Fatalf("combo power violations=%#v", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsAllowsCurrentComboPowerCeiling(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	limit, ok := store.arenaAlchemyComboPowerLimit("test_combo", nil, false)
	if !ok || limit <= 0 {
		t.Fatalf("combo power limit=%v ok=%v", limit, ok)
	}
	affix := EquipmentAffixSlot{
		AffixID:               "custom:combo-current",
		SourceKind:            "craft",
		AlchemyRulesetVersion: AlchemyCurrentRulesetVersion,
		EffectParams: map[string]any{
			"alchemyKind": "combat",
			"comboPower":  limit,
			"alchemyComponents": []alchemyCraftComponent{
				{SourceKind: "combo_output", ComboRecipeID: "test_combo", CurrentValue: limit},
			},
		},
	}
	items := []EquippedItem{{InstanceID: 100, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}}
	if violations := store.ArenaAlchemyEquipmentViolations(items); len(violations) != 0 {
		t.Fatalf("current combo ceiling must remain eligible, got %#v", violations)
	}
}

func TestArenaAlchemyComboLimitsCoverNegativePowerCoefficient(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	limits, ok := store.arenaAlchemyComboLimits("test_combo", nil, false)
	if !ok || limits["guard"] < 10 {
		t.Fatalf("combo limits=%#v ok=%v want guard ceiling from the low-power endpoint", limits, ok)
	}
}

func TestArenaAlchemyEquipmentViolationsSanitizesInvalidNumbers(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := arenaAlchemyGateTestAffix("craft", math.Inf(1), 1)
	items := []EquippedItem{{InstanceID: 97, TemplateItemID: "test_armor", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}}

	violations := store.ArenaAlchemyEquipmentViolations(items)
	if len(violations) != 1 || violations[0].Reason != "invalid_numeric_value" || violations[0].ActualValue != 0 {
		t.Fatalf("invalid numeric violations=%#v", violations)
	}
	if _, err := json.Marshal(violations); err != nil {
		t.Fatalf("violation payload must stay JSON encodable: %v", err)
	}
}

func TestArenaAlchemyTraitAbsoluteQuantityCoversCurrentCurve(t *testing.T) {
	trait := catalog.AlchemyTraitEntry{UnitBasis: 8, MaxBasisMultiplier: 25}
	absolute := arenaAlchemyTraitAbsoluteMaxQuantity(trait)
	for _, stability := range []float64{0, 1, 5, 10, 30, 100, 1_000, 1_000_000, math.MaxFloat64} {
		if current := alchemyTraitMaxQuantity(trait, stability); current > absolute {
			t.Fatalf("stability %.0f current quantity=%d exceeds arena ceiling=%d", stability, current, absolute)
		}
	}
}

func TestArenaAlchemyEquipmentViolationsIgnoresWorkEquipment(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := arenaAlchemyGateTestAffix("craft", 999, 100000)
	items := []EquippedItem{{InstanceID: 101, TemplateItemID: "test_tool", EquipType: "work", Affixes: []EquipmentAffixSlot{affix}}}
	if violations := store.ArenaAlchemyEquipmentViolations(items); len(violations) != 0 {
		t.Fatalf("work equipment violations=%#v want none", violations)
	}
}

func TestArenaAlchemyEquipmentViolationsDeduplicatesMultiSlotEquipment(t *testing.T) {
	store := arenaAlchemyGateTestStore()
	affix := arenaAlchemyGateTestAffix("craft", 0.7, 100000)
	item := EquippedItem{InstanceID: 102, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{affix}}
	items := []EquippedItem{item, item}
	if violations := store.ArenaAlchemyEquipmentViolations(items); len(violations) != 1 {
		t.Fatalf("multi-slot equipment violations=%#v want one", violations)
	}
}
