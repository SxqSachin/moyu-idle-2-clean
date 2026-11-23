package store

import (
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestSampleLuckSkewedBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 1000; i++ {
		got := sampleLuckSkewed(1.5, 3.5, 2, rng)
		if got < 1.5 || got > 3.5 {
			t.Fatalf("sampleLuckSkewed out of bounds: got %v", got)
		}
	}
}

func TestSampleLuckSkewedBiasesTowardHighValues(t *testing.T) {
	lowLuck := averageAlchemySample(0)
	highLuck := averageAlchemySample(3)
	if highLuck <= lowLuck {
		t.Fatalf("expected high luck average > low luck average, low=%v high=%v", lowLuck, highLuck)
	}
}

func TestAlchemyDerivedRangeSeparatesPrecisionAndStability(t *testing.T) {
	minValue, maxValue := applyAlchemyDerivedRange(0.1, 1, 1, 0)
	if !nearlyEqual(minValue, 0.1) || !nearlyEqual(maxValue, 1.5) {
		t.Fatalf("precision 100%% range=(%v,%v), want (0.1,1.5)", minValue, maxValue)
	}
	minValue, maxValue = applyAlchemyDerivedRange(0.1, 1, 2, 0)
	if !nearlyEqual(minValue, 0.1) || !nearlyEqual(maxValue, 1+(2.0/3.0)) {
		t.Fatalf("precision 200%% range=(%v,%v), want (0.1,%v)", minValue, maxValue, 1+(2.0/3.0))
	}
	minValue, maxValue = applyAlchemyDerivedRange(0.1, 1, -1, 0)
	if !nearlyEqual(minValue, 0.1) || !nearlyEqual(maxValue, 0.5) {
		t.Fatalf("precision -100%% range=(%v,%v), want (0.1,0.5)", minValue, maxValue)
	}
	minValue, maxValue = applyAlchemyDerivedRange(0.1, 1, 0, 1)
	if !nearlyEqual(minValue, 0.5) || !nearlyEqual(maxValue, 1.125) {
		t.Fatalf("stability 100%% range=(%v,%v), want (0.5,1.125)", minValue, maxValue)
	}
	minValue, maxValue = applyAlchemyDerivedRange(0.1, 1, 1, 1)
	if !nearlyEqual(minValue, 0.5) || !nearlyEqual(maxValue, 1.625) {
		t.Fatalf("precision+stability 100%% range=(%v,%v), want (0.5,1.625)", minValue, maxValue)
	}
	minValue, maxValue = applyAlchemyDerivedRange(0.1, 1, 0, -1)
	if !nearlyEqual(minValue, -0.5) || !nearlyEqual(maxValue, 0.875) {
		t.Fatalf("stability -100%% range=(%v,%v), want (-0.5,0.875)", minValue, maxValue)
	}
	minValue, maxValue = applyAlchemyDerivedRange(-10, -4, 1, 1)
	if !nearlyEqual(minValue, -10) || !nearlyEqual(maxValue, -4) {
		t.Fatalf("derived range should not amplify penalties, range=(%v,%v)", minValue, maxValue)
	}
}

func TestCapAlchemyInfusionPositiveRange(t *testing.T) {
	minValue, maxValue := capAlchemyInfusionPositiveRange(10, 100)
	if !nearlyEqual(minValue, 10) || !nearlyEqual(maxValue, 85) {
		t.Fatalf("positive range=(%v,%v), want (10,85)", minValue, maxValue)
	}

	minValue, maxValue = capAlchemyInfusionPositiveRange(-10, -4)
	if !nearlyEqual(minValue, -10) || !nearlyEqual(maxValue, -4) {
		t.Fatalf("negative range should remain full, got (%v,%v)", minValue, maxValue)
	}

	minValue, maxValue = capAlchemyInfusionPositiveRange(-2, 10)
	if !nearlyEqual(minValue, -2) || !nearlyEqual(maxValue, 8.5) {
		t.Fatalf("mixed-sign range should cap positive max, got (%v,%v)", minValue, maxValue)
	}

	minValue, maxValue = capAlchemyInfusionPositiveRange(80, 100)
	if !nearlyEqual(minValue, 80) || !nearlyEqual(maxValue, 85) {
		t.Fatalf("positive range=(%v,%v), want (80,85)", minValue, maxValue)
	}

	minValue, maxValue = capAlchemyInfusionPositiveRange(90, 100)
	if !nearlyEqual(minValue, 90) || !nearlyEqual(maxValue, 90) {
		t.Fatalf("capped max should not fall below min, got (%v,%v)", minValue, maxValue)
	}
}

func TestAlchemyInfusionOperationRangeAppliesFoodTradeoffs(t *testing.T) {
	modifiers := alchemyeffect.Modifiers{
		PositiveFloorReductionRate: 0.2,
		PositiveCeilingBonusRate:   0.1,
	}
	minValue, maxValue := applyAlchemyInfusionOperationRange(10, 100, modifiers)
	if !nearlyEqual(minValue, 8) || !nearlyEqual(maxValue, 93.5) {
		t.Fatalf("buffed positive range=(%v,%v), want (8,93.5)", minValue, maxValue)
	}

	minValue, maxValue = applyAlchemyInfusionOperationRange(-2, 10, modifiers)
	if !nearlyEqual(minValue, -2) || !nearlyEqual(maxValue, 9.35) {
		t.Fatalf("mixed-sign range should only lift positive ceiling, got (%v,%v)", minValue, maxValue)
	}

	minValue, maxValue = applyAlchemyInfusionOperationRange(80, 100, modifiers)
	if !nearlyEqual(minValue, 64) || !nearlyEqual(maxValue, 93.5) {
		t.Fatalf("floor tradeoff should open a valid range after ceiling cap, got (%v,%v)", minValue, maxValue)
	}
	minValue, maxValue = applyAlchemyInfusionOperationRange(100, 100, modifiers)
	if !nearlyEqual(minValue, 80) || !nearlyEqual(maxValue, 93.5) {
		t.Fatalf("floor reduction must precede range validation, got (%v,%v)", minValue, maxValue)
	}
}

func nearlyEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.0000001
}

func TestEncodeCustomAffixIDStableAcrossMapOrder(t *testing.T) {
	left := encodeCustomAffixID("add_stat", map[string]any{
		"attrMods": map[string]float64{
			"speed":          1.25,
			"physics_damage": 4.5,
		},
	}, 1, 3)
	right := encodeCustomAffixID("add_stat", map[string]any{
		"attrMods": map[string]float64{
			"physics_damage": 4.5,
			"speed":          1.25,
		},
	}, 1, 3)
	if left == "" {
		t.Fatal("expected non-empty custom affix id")
	}
	if left != right {
		t.Fatalf("expected stable custom affix id, left=%q right=%q", left, right)
	}
}

func TestResolveAlchemyComboParamTemplatesNestedProcChanceFormula(t *testing.T) {
	params := map[string]any{
		"battleHooks": []any{
			map[string]any{
				"id":      "alchemy_test_start",
				"trigger": "on_battle_start",
				"effects": []any{
					map[string]any{
						"kind":           "add_buff",
						"buffId":         "test_buff",
						"procChanceBase": 0.5,
						"procChanceMin":  0,
						"procChanceMax":  1,
						"procChanceFormula": map[string]any{
							"baseAdd": map[string]any{
								"alchemyValueCoeff": 0.01,
							},
							"terms": []any{},
						},
					},
				},
			},
		},
	}

	resolved, ok := resolveAlchemyComboParamTemplates(params, 26.18, 1).(map[string]any)
	if !ok {
		t.Fatalf("resolved params type=%T, want map[string]any", resolved)
	}
	hooks := resolved["battleHooks"].([]any)
	effects := hooks[0].(map[string]any)["effects"].([]any)
	effect := effects[0].(map[string]any)
	formula := effect["procChanceFormula"].(map[string]any)
	if got := formula["baseAdd"]; got != 0.262 {
		t.Fatalf("procChanceFormula.baseAdd=%#v, want 0.262", got)
	}
}

func TestAlchemyArgsRejectDerivedFields(t *testing.T) {
	executor := newAlchemyExecutor(&Store{}, "alchemy.convert")
	_, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"sourceItemId": "wood",
		"groupId": "group.life_primary_common",
		"sourceCount": 1,
		"durationSec": 1,
		"alchemyExp": 999
	}`))
	if err == nil {
		t.Fatal("expected derived submit fields to be rejected")
	}
}

func TestAlchemyArgsDoNotOverrideQueueCount(t *testing.T) {
	convert := newAlchemyExecutor(&Store{}, "alchemy.convert")
	convertArgs, err := convert.ValidateAndNormalizeArgs(json.RawMessage(`{
		"sourceItemId": "wood",
		"groupId": "group.life_primary_common",
		"sourceCount": 100
	}`))
	if err != nil {
		t.Fatalf("convert args invalid: %v", err)
	}
	if _, ok := convertArgs.(queueTotalCountProvider); ok {
		t.Fatal("convert args must not override queue total count")
	}
	if got := convertArgs.(alchemyConvertArgs).SourceCount; got != 1 {
		t.Fatalf("convert SourceCount=%d, want sanitized single-run count 1", got)
	}

	transmute := newAlchemyExecutor(&Store{}, "alchemy.transmute")
	transmuteArgs, err := transmute.ValidateAndNormalizeArgs(json.RawMessage(`{
		"itemId": "wood",
		"totalCount": 100
	}`))
	if err != nil {
		t.Fatalf("transmute args invalid: %v", err)
	}
	if _, ok := transmuteArgs.(queueTotalCountProvider); ok {
		t.Fatal("transmute args must not override queue total count")
	}
	if got := transmuteArgs.(alchemyTransmuteArgs).TotalCount; got != 1 {
		t.Fatalf("transmute TotalCount=%d, want sanitized single-run count 1", got)
	}
}

func TestAlchemyInheritSelectableGenesKeepParentsSeparate(t *testing.T) {
	father := inheritParentAffix{ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_strength", "strength", 5),
		inheritTestComponent("stone", "t_hp", "hp", 30),
	)}
	mother := inheritParentAffix{ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_strength", "strength", 2),
	)}
	genes := (&Store{}).buildInheritSelectableGenes(father, mother)
	if len(genes) != 3 {
		t.Fatalf("genes=%d, want 3: %#v", len(genes), genes)
	}
	keys := map[string]bool{}
	for _, gene := range genes {
		keys[inheritChoiceKey(gene.Parent, gene.ComponentIndex, gene.Component.SourceKey)] = true
	}
	for _, key := range []string{"father:0:trait:wood:t_strength:strength", "father:1:trait:stone:t_hp:hp", "mother:0:trait:wood:t_strength:strength"} {
		if !keys[key] {
			t.Fatalf("missing gene key %s in %#v", key, keys)
		}
	}
}

func TestAlchemyInheritOrdinaryAffixProjectsAttrAndEffectSources(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		AffixDefinitions: []catalog.AffixDefinition{{
			ID:         "affix.test",
			NameKey:    "affix.test.name",
			EffectType: "battle_hook",
			Type:       "battle",
			Active:     true,
			EffectParams: map[string]any{
				"attrMods": map[string]any{
					"common": map[string]any{
						"strength": map[string]any{"min": 1.0, "max": 3.0},
					},
				},
				"battleHooks": map[string]any{
					"common": []any{map[string]any{"id": "hook.test", "trigger": "turn_start", "effects": []any{map[string]any{"kind": "heal"}}}},
				},
			},
			MaxEffectivePerEquipment: 1,
			MaxEffectivePerBody:      2,
		}},
		AffixCarrierRules: []catalog.AffixCarrierRule{{
			CarrierItemID:    "carrier",
			AllowedRarityIDs: []string{"common"},
			MaxCustomAffixes: 1,
			AllowedTraitKind: "combat",
			Active:           true,
		}},
	}}
	slot := EquipmentAffixSlot{
		AffixID:    "affix.test",
		RarityID:   "common",
		AttrMods:   map[string]float64{"strength": 2},
		EffectType: "battle_hook",
	}
	if got := store.alchemyInheritSlotKind(slot); got != "combat" {
		t.Fatalf("kind=%q, want combat", got)
	}
	if !store.alchemyInheritParentSlotEligible(slot, "carrier") {
		t.Fatal("ordinary carrier affix should be eligible")
	}
	if store.alchemyInheritParentSlotEligible(slot, "not_carrier") {
		t.Fatal("ordinary affix on non-carrier item should be rejected")
	}
	components := store.projectOrdinaryAffixToInheritComponents(slot, "father")
	if len(components) != 2 {
		t.Fatalf("components=%d, want attr+effect: %#v", len(components), components)
	}
	if components[0].SourceKey != "affix:affix.test:strength" || components[0].Attr != "strength" {
		t.Fatalf("attr component mismatch: %#v", components[0])
	}
	if components[0].ValueRange.Min != 1 || components[0].ValueRange.Max != 3 {
		t.Fatalf("range=%#v, want 1..3", components[0].ValueRange)
	}
	if components[1].SourceKey != "affix:affix.test:effect" || components[1].ComponentKind != "effect" {
		t.Fatalf("effect component mismatch: %#v", components[1])
	}
	if _, ok := components[1].EffectParams["battleHooks"]; !ok {
		t.Fatalf("effect params missing battleHooks: %#v", components[1].EffectParams)
	}
}

func TestAlchemyInheritRejectsOrphanCustomParent(t *testing.T) {
	store := &Store{}
	slot := inheritTestSlot("combat", inheritTestComponent("wood", "t_strength", "strength", 5))
	slot.SourceKind = "inherit_orphan"
	slot.EffectParams["inheritOrphan"] = true
	if store.alchemyInheritParentSlotEligible(slot, "carrier") {
		t.Fatal("orphan inherit output must not be eligible as parent")
	}
}

func TestAlchemyInheritEffectSourcesDedupeWithoutNumericFusion(t *testing.T) {
	component := alchemyInheritComponent{
		SourceKind:    "ordinary_affix",
		SourceKey:     "affix:affix.test:effect",
		SourceType:    "affix",
		SourceID:      "affix.test",
		AffixID:       "affix.test",
		ComponentKind: "effect",
		EffectParams:  map[string]any{"effectMutations": []any{map[string]any{"scope": "both"}}},
	}
	retained := []inheritRetainedGene{
		{Gene: inheritSelectableGene{Parent: "father", ComponentIndex: 0, Component: component}, Component: component, RetentionReason: inheritDecisionInherit},
		{Gene: inheritSelectableGene{Parent: "mother", ComponentIndex: 0, Component: component}, Component: component, RetentionReason: inheritDecisionInherit},
	}
	components, traces := dedupeRetainedInheritEffects(retained, rand.New(rand.NewSource(1)))
	if len(components) != 1 {
		t.Fatalf("components=%d, want 1", len(components))
	}
	if components[0].FusionKind != "effect_dedupe" || components[0].CurrentValue != 0 || components[0].Attr != "" {
		t.Fatalf("effect dedupe should not numeric-fuse: %#v", components[0])
	}
	if len(traces) != 1 || traces[0].FusionKind != "effect_dedupe" {
		t.Fatalf("trace mismatch: %#v", traces)
	}
}

func TestAlchemyInheritArgsRejectLegacyIncludeSuppress(t *testing.T) {
	store := &Store{gameData: catalog.Data{AlchemyInheritMediums: []catalog.AlchemyInheritMedium{{
		ItemID: "inherit_medium",
		Active: true,
		Modifiers: catalog.AlchemyInheritModifiers{
			InheritSlots:  1,
			SuppressSlots: 1,
		},
	}}}}
	executor := newAlchemyExecutor(store, "alchemy.inherit")
	_, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"fatherInstanceId":1,
		"fatherAffixIndex":0,
		"motherInstanceId":2,
		"motherAffixIndex":0,
		"carrierInstanceId":3,
		"mediumItemId":"inherit_medium",
		"includeGeneKeys":["trait:wood:t_strength:strength"],
		"suppressGeneKeys":["trait:wood:t_strength:strength"]
	}`))
	if err != ErrAlchemyArgsInvalid {
		t.Fatalf("legacy args err=%v, want %v", err, ErrAlchemyArgsInvalid)
	}
}

func TestAlchemyInheritArgsRequireMedium(t *testing.T) {
	store := &Store{gameData: catalog.Data{AlchemyInheritMediums: []catalog.AlchemyInheritMedium{{
		ItemID: "inherit_medium",
		Active: true,
	}}}}
	executor := newAlchemyExecutor(store, "alchemy.inherit")
	_, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"fatherInstanceId":1,
		"fatherAffixIndex":0,
		"motherInstanceId":2,
		"motherAffixIndex":0,
		"carrierInstanceId":3
	}`))
	if err != ErrAlchemyMediumUnknown {
		t.Fatalf("missing medium err=%v, want %v", err, ErrAlchemyMediumUnknown)
	}
}

func TestAlchemyInheritMediumLevelRequirement(t *testing.T) {
	medium := &catalog.AlchemyInheritMedium{ItemID: "inherit_medium", MinAlchemyLevel: 40}
	if err := validateAlchemyInheritMediumLevel(nil, alchemyDerived{Level: 40}); err != ErrAlchemyMediumUnknown {
		t.Fatalf("nil medium err=%v, want %v", err, ErrAlchemyMediumUnknown)
	}
	if err := validateAlchemyInheritMediumLevel(medium, alchemyDerived{Level: 39}); err != ErrAlchemyLevelInsufficient {
		t.Fatalf("low level err=%v, want %v", err, ErrAlchemyLevelInsufficient)
	}
	if err := validateAlchemyInheritMediumLevel(medium, alchemyDerived{Level: 40}); err != nil {
		t.Fatalf("level met err=%v, want nil", err)
	}
}

func TestAlchemyInheritMediumCostIncludesConsumePerCraft(t *testing.T) {
	store := &Store{}
	costs := store.alchemyInheritPerRunCosts(alchemyInheritArgs{}, &catalog.AlchemyInheritMedium{
		ItemID:          "harmonic_crystal_lens",
		ConsumePerCraft: 1,
	})
	if costs["harmonic_crystal_lens"] != 1 {
		t.Fatalf("medium cost=%v, want 1", costs["harmonic_crystal_lens"])
	}
}

func TestAlchemyInheritChoicesEnforceSlots(t *testing.T) {
	selectable := []inheritSelectableGene{
		{Parent: "father", ComponentIndex: 0, Component: alchemyInheritComponent{SourceKey: "a"}},
		{Parent: "mother", ComponentIndex: 0, Component: alchemyInheritComponent{SourceKey: "b"}},
	}
	choices := []alchemyInheritGeneChoice{
		{Parent: "father", ComponentIndex: 0, SourceKey: "a", Decision: inheritDecisionInherit},
		{Parent: "mother", ComponentIndex: 0, SourceKey: "b", Decision: inheritDecisionInherit},
	}
	_, err := applyInheritChoices(selectable, choices, 1, 0)
	if err != ErrAlchemyArgsInvalid {
		t.Fatalf("slot err=%v, want %v", err, ErrAlchemyArgsInvalid)
	}
}

func TestAlchemyInheritIgnoreUsesNaturalPositiveRetention(t *testing.T) {
	store := &Store{}
	father := inheritParentAffix{Kind: "combat", ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_strength", "strength", 5),
	)}
	mother := inheritParentAffix{Kind: "combat", ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponent("stone", "t_hp", "hp", 30),
	)}
	result, err := store.resolveAlchemyInherit(father, mother, inheritTestMedium(0, 0, 0), alchemyInheritArgs{}, alchemyDerived{Level: 1}, rand.New(rand.NewSource(2)))
	if err != nil {
		t.Fatalf("resolveAlchemyInherit: %v", err)
	}
	if len(result.Components) == 0 {
		t.Fatal("positive ignored sources should have natural retention chance")
	}
}

func TestAlchemyInheritEffectIgnoreUsesPositiveBaseRetention(t *testing.T) {
	store := &Store{}
	rules := store.effectiveAlchemyInheritRules(nil)
	got := inheritAdjustedIgnoreKeepChanceForComponent(&rules, alchemyDerived{}, alchemyInheritComponent{ComponentKind: "effect"})
	if !nearlyEqual(got, 0.4) {
		t.Fatalf("effect ignore chance=%v, want positive base 0.4", got)
	}
	rules.Modifiers.IgnoreKeepChance = 0.2
	got = inheritAdjustedIgnoreKeepChanceForComponent(&rules, alchemyDerived{}, alchemyInheritComponent{ComponentKind: "effect"})
	if !nearlyEqual(got, 0.6) {
		t.Fatalf("effect ignore chance with medium bonus=%v, want 0.6", got)
	}
}

func TestAlchemyInheritUnrelatedAttrsPreserveCurrentValues(t *testing.T) {
	store := &Store{}
	father := inheritParentAffix{Kind: "combat", ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponentRange("wood", "t_strength", "strength", 5, 1, 10),
	)}
	mother := inheritParentAffix{Kind: "combat", ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponentRange("stone", "t_hp", "hp", 30, 10, 50),
	)}
	args := alchemyInheritArgs{GeneChoices: []alchemyInheritGeneChoice{
		{Parent: "father", ComponentIndex: 0, SourceKey: "trait:wood:t_strength:strength", Decision: inheritDecisionInherit},
		{Parent: "mother", ComponentIndex: 0, SourceKey: "trait:stone:t_hp:hp", Decision: inheritDecisionInherit},
	}}
	result, err := store.resolveAlchemyInherit(father, mother, inheritTestMedium(2, 0, 0), args, alchemyDerived{Level: 1, LuckyChance: 1}, rand.New(rand.NewSource(2)))
	if err != nil {
		t.Fatalf("resolveAlchemyInherit: %v", err)
	}
	if got := result.AttrMods["strength"]; !nearlyEqual(got, 5) {
		t.Fatalf("strength=%v, want retained current value 5", got)
	}
	if got := result.AttrMods["hp"]; !nearlyEqual(got, 30) {
		t.Fatalf("hp=%v, want retained current value 30", got)
	}
	if len(result.EffectParams["alchemyInheritSummary"].(alchemyInheritSummary).Fusions) != 0 {
		t.Fatalf("unrelated attrs should not create fusions: %#v", result.EffectParams["alchemyInheritSummary"])
	}
}

func TestAlchemyInheritSameParentSameAttrDoesNotHybridFuse(t *testing.T) {
	store := &Store{}
	father := inheritParentAffix{Kind: "combat", ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_attack_a", "attack", 1),
		inheritTestComponent("stone", "t_attack_b", "attack", 2),
	)}
	mother := inheritParentAffix{Kind: "combat", ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponent("herb", "t_speed", "speed", 3),
	)}
	args := alchemyInheritArgs{GeneChoices: []alchemyInheritGeneChoice{
		{Parent: "father", ComponentIndex: 0, SourceKey: "trait:wood:t_attack_a:attack", Decision: inheritDecisionInherit},
		{Parent: "father", ComponentIndex: 1, SourceKey: "trait:stone:t_attack_b:attack", Decision: inheritDecisionInherit},
		{Parent: "mother", ComponentIndex: 0, SourceKey: "trait:herb:t_speed:speed", Decision: inheritDecisionSuppress},
	}}
	result, err := store.resolveAlchemyInherit(father, mother, inheritTestMedium(2, 1, 0), args, alchemyDerived{Level: 1}, rand.New(rand.NewSource(2)))
	if err != nil {
		t.Fatalf("resolveAlchemyInherit: %v", err)
	}
	if got := result.AttrMods["attack"]; !nearlyEqual(got, 3) {
		t.Fatalf("attack=%v, want same-parent components to merge by sum 3", got)
	}
	if len(result.Components) != 2 {
		t.Fatalf("components=%d, want two unfused source components: %#v", len(result.Components), result.Components)
	}
	for _, component := range result.Components {
		if component.FusionKind == "hybrid" || strings.HasPrefix(component.FusionKind, "inbreed") {
			t.Fatalf("same-parent components must not fuse: %#v", result.Components)
		}
	}
}

func TestAlchemyInheritHybridFusionUsesDiminishingDivisor(t *testing.T) {
	store := &Store{}
	father := inheritParentAffix{Kind: "combat", ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_strength", "strength", 10),
	)}
	mother := inheritParentAffix{Kind: "combat", ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponent("stone", "t_power", "strength", 6),
	)}
	args := alchemyInheritArgs{GeneChoices: []alchemyInheritGeneChoice{
		{Parent: "father", ComponentIndex: 0, SourceKey: "trait:wood:t_strength:strength", Decision: inheritDecisionInherit},
		{Parent: "mother", ComponentIndex: 0, SourceKey: "trait:stone:t_power:strength", Decision: inheritDecisionInherit},
	}}
	result, err := store.resolveAlchemyInherit(father, mother, inheritTestMedium(2, 0, 0), args, alchemyDerived{Level: 1}, rand.New(rand.NewSource(2)))
	if err != nil {
		t.Fatalf("resolveAlchemyInherit: %v", err)
	}
	if len(result.Components) != 1 || result.Components[0].FusionKind != "hybrid" {
		t.Fatalf("components=%#v, want one hybrid", result.Components)
	}
	if got := result.AttrMods["strength"]; !nearlyEqual(got, 10) {
		t.Fatalf("strength=%v, want (10+6)/1.6=10", got)
	}
}

func TestAlchemyInheritInbreedBadMutationMakesNegativeWorse(t *testing.T) {
	store := &Store{}
	father := inheritParentAffix{Kind: "combat", ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_penalty", "speed", -0.1),
	)}
	mother := inheritParentAffix{Kind: "combat", ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_penalty", "speed", -0.1),
	)}
	args := alchemyInheritArgs{GeneChoices: []alchemyInheritGeneChoice{
		{Parent: "father", ComponentIndex: 0, SourceKey: "trait:wood:t_penalty:speed", Decision: inheritDecisionInherit},
		{Parent: "mother", ComponentIndex: 0, SourceKey: "trait:wood:t_penalty:speed", Decision: inheritDecisionInherit},
	}}
	result, err := store.resolveAlchemyInherit(father, mother, &catalog.AlchemyInheritMedium{
		Modifiers: catalog.AlchemyInheritModifiers{
			InheritSlots:               2,
			HybridMergeDivisor:         1,
			InbreedMutationChance:      1,
			InbreedGoodChance:          0,
			InbreedBadMultiplierRange:  catalog.AlchemyInheritMultiplierRange{Min: 0.5, Max: 0.5},
			InbreedGoodMultiplierRange: catalog.AlchemyInheritMultiplierRange{Min: 1.1, Max: 1.1},
		},
	}, args, alchemyDerived{Level: 1}, rand.New(rand.NewSource(2)))
	if err != nil {
		t.Fatalf("resolveAlchemyInherit: %v", err)
	}
	if len(result.Components) != 1 || result.Components[0].FusionKind != "inbreed_bad" {
		t.Fatalf("components=%#v, want one inbreed_bad", result.Components)
	}
	if got := result.AttrMods["speed"]; got >= -0.2 {
		t.Fatalf("speed=%v, want worse than stable -0.2", got)
	}
}

func TestAlchemyInheritNumericValueCapsAtTripleSourceUpperBound(t *testing.T) {
	store := &Store{}
	father := inheritParentAffix{Kind: "combat", ParentTag: "father", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_power", "attack", 0.1),
	)}
	mother := inheritParentAffix{Kind: "combat", ParentTag: "mother", Slot: inheritTestSlot("combat",
		inheritTestComponent("wood", "t_power", "attack", 0.1),
	)}
	args := alchemyInheritArgs{GeneChoices: []alchemyInheritGeneChoice{
		{Parent: "father", ComponentIndex: 0, SourceKey: "trait:wood:t_power:attack", Decision: inheritDecisionInherit},
		{Parent: "mother", ComponentIndex: 0, SourceKey: "trait:wood:t_power:attack", Decision: inheritDecisionInherit},
	}}
	result, err := store.resolveAlchemyInherit(father, mother, &catalog.AlchemyInheritMedium{
		Modifiers: catalog.AlchemyInheritModifiers{
			InheritSlots:               2,
			HybridMergeDivisor:         1,
			InbreedMutationChance:      1,
			InbreedGoodChance:          1,
			InbreedGoodMultiplierRange: catalog.AlchemyInheritMultiplierRange{Min: 10, Max: 10},
			InbreedBadMultiplierRange:  catalog.AlchemyInheritMultiplierRange{Min: 0.5, Max: 0.5},
		},
	}, args, alchemyDerived{Level: 1}, rand.New(rand.NewSource(2)))
	if err != nil {
		t.Fatalf("resolveAlchemyInherit: %v", err)
	}
	if got := result.AttrMods["attack"]; !nearlyEqual(got, 0.3) {
		t.Fatalf("attack=%v, want capped source upper 0.1*3=0.3", got)
	}
	if got := result.Components[0].CurrentValue; !nearlyEqual(got, 0.3) {
		t.Fatalf("component current=%v, want capped source upper 0.1*3=0.3", got)
	}
}

func TestAlchemyFastForwardBudgetYieldsOnlyAfterProgress(t *testing.T) {
	started := time.Now().Add(-time.Second)
	budget := FastForwardBudget{MaxExec: time.Millisecond}

	if shouldYieldAlchemyFastForwardBudget(started, budget, 0) {
		t.Fatal("budget should not yield before at least one alchemy run settles")
	}
	if !shouldYieldAlchemyFastForwardBudget(started, budget, 1) {
		t.Fatal("budget should yield after progress when max exec is exceeded")
	}
	if shouldYieldAlchemyFastForwardBudget(started, FastForwardBudget{}, 1) {
		t.Fatal("empty budget should not yield")
	}
}

func TestAlchemyQueueRNGDeterministic(t *testing.T) {
	left := alchemyQueueRNG(10, 20, 3, "alchemy.transmute")
	right := alchemyQueueRNG(10, 20, 3, "alchemy.transmute")
	for i := 0; i < 8; i++ {
		if a, b := left.Int63(), right.Int63(); a != b {
			t.Fatalf("alchemyQueueRNG mismatch at %d: %d != %d", i, a, b)
		}
	}
	other := alchemyQueueRNG(10, 20, 4, "alchemy.transmute")
	if left.Int63() == other.Int63() {
		t.Fatal("different doneBefore should produce a different alchemy queue sequence")
	}
}

func TestAlchemyReservedRunCountRequiresFullRunCosts(t *testing.T) {
	perRun := map[string]int64{"wood": 1, GoldCoinItemID: 45}
	if got := alchemyReservedRunCount(map[string]int64{"wood": 1, GoldCoinItemID: 45}, perRun); got != 1 {
		t.Fatalf("reserved run count=%d, want 1", got)
	}
	if got := alchemyReservedRunCount(map[string]int64{"wood": 1}, perRun); got != 0 {
		t.Fatalf("partial reserved run count=%d, want 0", got)
	}
	if got := alchemyReservedRunCount(nil, perRun); got != 0 {
		t.Fatalf("empty reserved run count=%d, want 0", got)
	}
}

func TestCheckedScaleInt64MapRejectsOverflow(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	if _, err := checkedScaleInt64Map(map[string]int64{"gold_coin": maxInt64}, 2); err != ErrInventoryQuantityOverflow {
		t.Fatalf("checkedScaleInt64Map overflow err=%v, want %v", err, ErrInventoryQuantityOverflow)
	}
	got, err := checkedScaleInt64Map(map[string]int64{"wood": 3}, 4)
	if err != nil {
		t.Fatalf("checkedScaleInt64Map normal err=%v", err)
	}
	if got["wood"] != 12 {
		t.Fatalf("checkedScaleInt64Map wood=%d, want 12", got["wood"])
	}
}

func TestAlchemyBatchLedgerMetaGroupsByFirstRunAndKeepsRange(t *testing.T) {
	meta := alchemyBatchLedgerMeta(QueueRuntimeItem{
		QueueID:    99,
		ActionID:   "alchemy.convert",
		DoneCount:  4,
		AuditPhase: "offline_fastforward",
	}, "alchemy_convert", "inventory", InventoryAuditDirectionGain, "alchemy_convert_reward", 25, nil)
	if got := meta.Details["runId"]; got != "99:5" {
		t.Fatalf("runId=%v, want 99:5", got)
	}
	if got := meta.Details["runNoEnd"]; got != 29 {
		t.Fatalf("runNoEnd=%v, want 29", got)
	}
	if got := meta.Details["count"]; got != 25 {
		t.Fatalf("count=%v, want 25", got)
	}
}

func TestAlchemyBatchBenchAuditItemsDoNotDoubleCount(t *testing.T) {
	event := normalizeInventoryAuditEvent(InventoryAuditEvent{
		Category:      InventoryAuditCategoryAlchemy,
		Direction:     InventoryAuditDirectionGain,
		SubjectType:   "alchemy",
		SubjectID:     "9",
		Items:         map[string]int64{"carrier_gem": 3},
		InstanceIDs:   []int64{101, 102, 103},
		SourceSurface: "alchemy_bench",
		TargetSurface: "inventory",
		Details:       map[string]any{"queueId": int64(9), "actionId": "alchemy.bench"},
	})
	if got := event.Items["carrier_gem"]; got != 3 {
		t.Fatalf("bench audit item quantity=%d, want 3", got)
	}
	if event.Quantity != 3 {
		t.Fatalf("bench audit quantity=%d, want 3", event.Quantity)
	}
}

func TestAlchemySmeltDoesNotFallBackToGenericQueueExecutor(t *testing.T) {
	executor := (&Store{}).executorForAction("alchemy.smelt")
	if _, ok := executor.(*alchemyExecutor); !ok {
		t.Fatalf("alchemy.smelt executor=%T, want *alchemyExecutor", executor)
	}
	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{}`)); err != ErrAlchemyArgsInvalid {
		t.Fatalf("alchemy.smelt args err=%v, want %v", err, ErrAlchemyArgsInvalid)
	}
}

func TestAlchemyBenchArgsRequireAllMaterialSlots(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		AlchemyMediums: []catalog.AlchemyMedium{{
			ID:     "medium.wide",
			ItemID: "wide_medium",
			Modifiers: catalog.AlchemyMediumModifiers{
				MaxTraitSlots: 4,
			},
			Active: true,
		}},
	}}
	executor := newAlchemyExecutor(store, "alchemy.bench")
	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"carrierInstanceId": 1,
		"materials": { "wood": 1, "stone": 1 }
	}`)); err != ErrAlchemyTraitSlotOverflow {
		t.Fatalf("default slot fill err=%v, want %v", err, ErrAlchemyTraitSlotOverflow)
	}
	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"carrierInstanceId": 1,
		"mediumItemId": "wide_medium",
		"materials": { "wood": 1, "stone": 1, "milk": 1 }
	}`)); err != ErrAlchemyTraitSlotOverflow {
		t.Fatalf("medium slot fill err=%v, want %v", err, ErrAlchemyTraitSlotOverflow)
	}
	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"carrierInstanceId": 1,
		"mediumItemId": "wide_medium",
		"materials": { "wood": 1, "stone": 1, "milk": 1, "egg": 1 }
	}`)); err != nil {
		t.Fatalf("full medium slots should pass, got %v", err)
	}
}

func TestAlchemyBenchComboIDMustMatchMaterials(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		AlchemyComboRecipes: []catalog.AlchemyComboRecipe{{
			ID:     "recipe.strict",
			Kind:   "combat",
			Inputs: map[string]int64{"wood": 2, "stone": 3, "milk": 1},
			OutputAffix: catalog.AlchemyComboOutputAffix{
				EffectType: "add_stat",
				ValueRange: catalog.AlchemyValueRange{Min: 1, Max: 2},
			},
		}},
	}}
	executor := newAlchemyExecutor(store, "alchemy.bench")
	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"carrierInstanceId": 1,
		"comboRecipeId": "recipe.strict",
		"materials": { "wood": 2, "stone": 3, "milk": 1 }
	}`)); err != nil {
		t.Fatalf("matching combo args should pass, got %v", err)
	}
	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{
		"carrierInstanceId": 1,
		"comboRecipeId": "recipe.strict",
		"materials": { "wood": 4, "stone": 3, "milk": 1 }
	}`)); err != ErrAlchemyArgsInvalid {
		t.Fatalf("mismatched combo args err=%v, want %v", err, ErrAlchemyArgsInvalid)
	}
	if _, err := store.resolveAlchemyCraft(map[string]int64{"wood": 2, "stone": 3, "milk": 1}, "recipe.missing", nil, alchemyDerived{Level: 99}, rand.New(rand.NewSource(1))); err != ErrAlchemyArgsInvalid {
		t.Fatalf("stale combo id craft err=%v, want %v", err, ErrAlchemyArgsInvalid)
	}
}

func TestAlchemyBenchRejectsMaterialLevelDeficit(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		ItemTraits: []catalog.ItemTraitDefinition{{
			ItemID: "wood",
			Traits: []catalog.AlchemyTraitEntry{{
				TraitID:            "wood",
				Kind:               "life",
				EffectType:         "add_stat",
				UnitBasis:          10,
				MaxBasisMultiplier: 1,
				MinAlchemyLevel:    15,
				Effects: []catalog.AlchemyTraitEffect{{
					Attr:         "collecting_speed",
					ValuePerUnit: catalog.AlchemyValueRange{Min: 0.001, Max: 0.002},
				}},
			}},
		}},
	}}
	_, err := store.resolveAlchemyCraft(map[string]int64{"wood": 10}, "", nil, alchemyDerived{Level: 14}, rand.New(rand.NewSource(1)))
	if err != ErrAlchemyLevelInsufficient {
		t.Fatalf("material level err=%v, want %v", err, ErrAlchemyLevelInsufficient)
	}
}

func TestAlchemyBenchRejectsMediumLevelDeficit(t *testing.T) {
	medium := &catalog.AlchemyMedium{ItemID: "moon_resin", MinAlchemyLevel: 20}
	store := &Store{gameData: catalog.Data{
		ItemTraits: []catalog.ItemTraitDefinition{{
			ItemID: "wood",
			Traits: []catalog.AlchemyTraitEntry{{
				TraitID:            "wood",
				Kind:               "life",
				EffectType:         "add_stat",
				UnitBasis:          10,
				MaxBasisMultiplier: 1,
				MinAlchemyLevel:    1,
				Effects: []catalog.AlchemyTraitEffect{{
					Attr:         "collecting_speed",
					ValuePerUnit: catalog.AlchemyValueRange{Min: 0.001, Max: 0.002},
				}},
			}},
		}},
	}}
	_, err := store.resolveAlchemyCraft(map[string]int64{"wood": 10}, "", medium, alchemyDerived{Level: 19}, rand.New(rand.NewSource(1)))
	if err != ErrAlchemyLevelInsufficient {
		t.Fatalf("medium level err=%v, want %v", err, ErrAlchemyLevelInsufficient)
	}
}

func TestAlchemyClearReservedPayloadPreservesDuration(t *testing.T) {
	args := alchemyConvertArgs{
		SourceItemID: "wood",
		GroupID:      "group.life_primary_common",
		SourceCount:  1,
	}
	runCtxRaw, err := marshalAlchemyRunContext(args, map[string]int64{"wood": 1})
	if err != nil {
		t.Fatalf("marshalAlchemyRunContext: %v", err)
	}
	runtimeRaw, err := json.Marshal(alchemyRuntime{
		BaseDurationMs: int64(16 * time.Second / time.Millisecond),
		DurationMs:     int64(32 * time.Second / time.Millisecond),
	})
	if err != nil {
		t.Fatalf("marshal runtime: %v", err)
	}
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "alchemy.convert",
		ExecutorID: "alchemy.convert",
		Runtime:    runtimeRaw,
		RunContext: runCtxRaw,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	next := newAlchemyExecutor(&Store{}, "alchemy.convert").clearReservedPayload(payloadRaw)
	if got := parseQueueRuntimeDuration(next); got != 32*time.Second {
		t.Fatalf("duration after clearReservedPayload=%v, want 32s", got)
	}
	if reserved := parseAlchemyReservedCosts(next); len(reserved) != 0 {
		t.Fatalf("reserved costs after clearReservedPayload=%v, want empty", reserved)
	}
}

func TestParseAlchemyRunContextArgsRejectsMissingArgs(t *testing.T) {
	runCtxRaw, err := json.Marshal(alchemyRunContextEnvelope{ReservedTemplateCosts: map[string]int64{"wood": 1}})
	if err != nil {
		t.Fatalf("marshal run context: %v", err)
	}
	runtimeRaw, err := json.Marshal(alchemyRuntime{DurationMs: int64(32 * time.Second / time.Millisecond)})
	if err != nil {
		t.Fatalf("marshal runtime: %v", err)
	}
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "alchemy.convert",
		ExecutorID: "alchemy.convert",
		Runtime:    runtimeRaw,
		RunContext: runCtxRaw,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := parseAlchemyRunContextArgs("alchemy.convert", payloadRaw); err == nil {
		t.Fatal("expected missing alchemy args to be rejected")
	}
}

func TestAlchemyLedgerMetaIncludesStableRunID(t *testing.T) {
	meta := alchemyLedgerMeta(QueueRuntimeItem{
		QueueID:    123,
		ActionID:   "alchemy.convert",
		DoneCount:  4,
		AuditPhase: "online_settle",
	}, "alchemy_convert", "inventory", InventoryAuditDirectionGain, "alchemy_convert_reward", nil)

	if got := meta.Details["runNo"]; got != 5 {
		t.Fatalf("runNo=%v, want 5", got)
	}
	if got := meta.Details["runId"]; got != "123:5" {
		t.Fatalf("runId=%v, want 123:5", got)
	}
}

func TestAlchemyConvertUpgradeChanceHonorsFixedDSLProbability(t *testing.T) {
	group := &catalog.AlchemyGroup{UpgradeChanceBase: 0.05}
	weights := map[string]float64{
		"rare_a": 1,
		"rare_b": 1,
		"rare_c": 1,
	}
	if got := alchemyConvertUpgradeChance(group, weights); math.Abs(got-0.05) > 0.000001 {
		t.Fatalf("upgrade chance=%v, want 0.05", got)
	}

	weights["rare_d"] = 1
	weights["rare_e"] = 1
	if got := alchemyConvertUpgradeChance(group, weights); math.Abs(got-0.05) > 0.000001 {
		t.Fatalf("upgrade chance grew with reward count: got=%v want=0.05", got)
	}
	if got := alchemyConvertUpgradeChance(group, nil); got != 0 {
		t.Fatalf("upgrade chance without rewards=%v, want 0", got)
	}
}

func TestRoundAlchemyAttributeValuePreservesSmallPercentTraits(t *testing.T) {
	store := &Store{gameData: catalog.Data{Attributes: []catalog.AttributeDefinition{
		{ID: "physics_damage_amplification", Unit: "percent"},
		{ID: "physics_defence", Unit: "number"},
	}, ItemTraits: []catalog.ItemTraitDefinition{{
		ItemID: "wayward_carousel_spring",
		Traits: []catalog.AlchemyTraitEntry{{
			TraitID: "wayward_carousel_spring",
			Effects: []catalog.AlchemyTraitEffect{{
				Attr:         "physics_damage_amplification",
				ValuePerUnit: catalog.AlchemyValueRange{Min: 0.00003, Max: 0.000085},
			}},
		}},
	}}}}

	if got, want := store.roundAlchemyAttributeValue("physics_damage_amplification", 0.00024), 0.00024; got != want {
		t.Fatalf("small percent trait rounded to %v, want %v", got, want)
	}
	if got, want := store.roundAlchemyAttributeValue("physics_defence", 1.23456), 1.235; got != want {
		t.Fatalf("number trait rounded to %v, want %v", got, want)
	}
	if got, want := store.roundAlchemyAttributeValue("unknown", 0.00024), 0.0; got != want {
		t.Fatalf("unknown attribute fallback rounded to %v, want %v", got, want)
	}
	if got, want := store.computeFloorAttrMods(map[string]int64{"wayward_carousel_spring": 8}, "", nil, alchemyDerived{})["physics_damage_amplification"], 0.00024; got != want {
		t.Fatalf("percent trait floor=%v, want %v", got, want)
	}
}

func TestAlchemyConvertFailChanceUsesGroupBaseAndStability(t *testing.T) {
	group := &catalog.AlchemyGroup{FailChanceBase: 0.22}
	if got, want := alchemyConvertFailChance(group, alchemyDerived{Stability: 0.07}), 0.22/1.07; math.Abs(got-want) > 0.000001 {
		t.Fatalf("fail chance=%v, want %v", got, want)
	}
	group.FailChanceBase = 0.15
	group.MinFailChance = 0.15
	if got := alchemyConvertFailChance(group, alchemyDerived{Stability: 1.0}); math.Abs(got-0.15) > 0.000001 {
		t.Fatalf("100%% stability fail chance=%v, want 0.15", got)
	}
}

func TestRollAlchemyConvertBatchRewardsDeterministicAndConservesRuns(t *testing.T) {
	weights := map[string]float64{
		"common_a": 3,
		"common_b": 1,
	}
	upgradeWeights := map[string]float64{
		"rare_a": 1,
		"rare_b": 1,
	}
	left, err := rollAlchemyConvertBatchRewards(100_000, 0.21, 0.08, weights, upgradeWeights, rand.New(rand.NewSource(42)))
	if err != nil {
		t.Fatalf("roll left: %v", err)
	}
	right, err := rollAlchemyConvertBatchRewards(100_000, 0.21, 0.08, weights, upgradeWeights, rand.New(rand.NewSource(42)))
	if err != nil {
		t.Fatalf("roll right: %v", err)
	}
	if left.FailedCount != right.FailedCount || left.UpgradeCount != right.UpgradeCount || !sameInt64Map(left.Rewards, right.Rewards) {
		t.Fatalf("batch convert roll is not deterministic: left=%+v right=%+v", left, right)
	}
	rewardTotal := sumInt64Map(left.Rewards)
	if got, want := int64(left.FailedCount)+rewardTotal, int64(100_000); got != want {
		t.Fatalf("failed+rewarded=%d, want %d; roll=%+v", got, want, left)
	}
	if left.UpgradeCount < 0 || left.UpgradeCount > int(rewardTotal) {
		t.Fatalf("upgrade count=%d outside rewarded total=%d", left.UpgradeCount, rewardTotal)
	}
	if left.FailedCount < 18_000 || left.FailedCount > 24_000 {
		t.Fatalf("failed count=%d outside expected range", left.FailedCount)
	}
	upgradeRewarded := left.Rewards["rare_a"] + left.Rewards["rare_b"]
	if int64(left.UpgradeCount) != upgradeRewarded {
		t.Fatalf("upgrade rewards=%d, want upgradeCount=%d", upgradeRewarded, left.UpgradeCount)
	}
}

func TestRollAlchemyConvertBatchRewardsHandlesAllFailAndMissingWeights(t *testing.T) {
	allFail, err := rollAlchemyConvertBatchRewards(1000, 1, 0.5, nil, map[string]float64{"rare": 1}, rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatalf("all fail roll: %v", err)
	}
	if allFail.FailedCount != 1000 || allFail.UpgradeCount != 0 || len(allFail.Rewards) != 0 {
		t.Fatalf("all fail roll=%+v, want all failed and no rewards", allFail)
	}
	if _, err := rollAlchemyConvertBatchRewards(10, 0, 0, nil, nil, rand.New(rand.NewSource(7))); err != ErrAlchemyGroupNotFound {
		t.Fatalf("missing normal weights err=%v, want %v", err, ErrAlchemyGroupNotFound)
	}
}

func TestAlchemyTransmuteKeyFilteringUsesPrefixAndTypedMetadata(t *testing.T) {
	s := &Store{}
	finalSigil := &catalog.Item{
		ID:               "final_volume_key_sigil",
		Type:             []string{"loot", "material"},
		Category:         "loot",
		EconomyBaseValue: 520,
	}
	if !s.itemAllowedForAlchemyTransmute(finalSigil) {
		t.Fatal("final_volume_key_sigil should be transmutable; key_ is not an id prefix")
	}
	if s.itemAllowedForAlchemyTransmute(&catalog.Item{ID: "key_archive", Type: []string{"material"}, Category: "material", EconomyBaseValue: 10}) {
		t.Fatal("key_ prefix item should remain blocked")
	}
	if s.itemAllowedForAlchemyTransmute(&catalog.Item{ID: "archive_sigil", Type: []string{"key"}, Category: "material", EconomyBaseValue: 10}) {
		t.Fatal("typed key item should remain blocked")
	}
}

func TestRollAlchemyTransmuteBatchRewardsExactMatchesSequential(t *testing.T) {
	ratio := catalog.AlchemyReturnRatio{MinRatio: 0.75, MaxRatio: 0.95}
	tiers := []catalog.AlchemyTransmuteTier{
		{ID: "small", Weight: 75, Kind: "gold_multiplier", Value: 1.15},
		{ID: "good", Weight: 20, Kind: "gold_multiplier", Value: 1.35},
		{ID: "great", Weight: 5, Kind: "gold_multiplier", Value: 1.8},
	}
	got, err := rollAlchemyTransmuteBatchRewards(127, 0.18, 37, ratio, 0.2, 0.07, 0.03, tiers, rand.New(rand.NewSource(77)))
	if err != nil {
		t.Fatalf("batch transmute roll: %v", err)
	}
	want, err := rollAlchemyTransmuteSequentialForTest(127, 0.18, 37, ratio, 0.2, 0.07, 0.03, tiers, rand.New(rand.NewSource(77)))
	if err != nil {
		t.Fatalf("sequential transmute roll: %v", err)
	}
	if got.FailedCount != want.FailedCount || got.LuckyCount != want.LuckyCount || !sameInt64Map(got.Rewards, want.Rewards) {
		t.Fatalf("exact batch transmute mismatch:\ngot=%+v\nwant=%+v", got, want)
	}
}

func TestRollAlchemyTransmuteBatchRewardsFastDeterministic(t *testing.T) {
	ratio := catalog.AlchemyReturnRatio{MinRatio: 0.75, MaxRatio: 0.95}
	tiers := []catalog.AlchemyTransmuteTier{
		{ID: "small", Weight: 75, Kind: "gold_multiplier", Value: 1.15},
		{ID: "good", Weight: 20, Kind: "gold_multiplier", Value: 1.35},
		{ID: "great", Weight: 5, Kind: "gold_multiplier", Value: 1.8},
	}
	left, err := rollAlchemyTransmuteBatchRewards(100_000, 0.18, 40, ratio, 0.35, 0.09, 0.05, tiers, rand.New(rand.NewSource(123)))
	if err != nil {
		t.Fatalf("left transmute roll: %v", err)
	}
	right, err := rollAlchemyTransmuteBatchRewards(100_000, 0.18, 40, ratio, 0.35, 0.09, 0.05, tiers, rand.New(rand.NewSource(123)))
	if err != nil {
		t.Fatalf("right transmute roll: %v", err)
	}
	if left.FailedCount != right.FailedCount || left.LuckyCount != right.LuckyCount || !sameInt64Map(left.Rewards, right.Rewards) {
		t.Fatalf("fast batch transmute roll is not deterministic: left=%+v right=%+v", left, right)
	}
	if left.FailedCount < 15_000 || left.FailedCount > 21_000 {
		t.Fatalf("failed count=%d outside expected range", left.FailedCount)
	}
	success := int64(100_000 - left.FailedCount)
	if left.LuckyCount < 0 || int64(left.LuckyCount) > success {
		t.Fatalf("lucky count=%d outside success count=%d", left.LuckyCount, success)
	}
	minGold := success * int64(math.Floor(40*ratio.MinRatio))
	if got := left.Rewards[GoldCoinItemID]; got < minGold {
		t.Fatalf("gold=%d below conservative min=%d; roll=%+v", got, minGold, left)
	}
}

func TestRollAlchemyTransmuteBatchRewardsHandlesAllFail(t *testing.T) {
	roll, err := rollAlchemyTransmuteBatchRewards(10_000, 1, 40, catalog.AlchemyReturnRatio{MinRatio: 0.75, MaxRatio: 0.95}, 0.2, 1, 0.5, []catalog.AlchemyTransmuteTier{{ID: "great", Weight: 1, Kind: "gold_multiplier", Value: 1.8}}, rand.New(rand.NewSource(5)))
	if err != nil {
		t.Fatalf("all fail transmute roll: %v", err)
	}
	if roll.FailedCount != 10_000 || roll.LuckyCount != 0 || len(roll.Rewards) != 0 {
		t.Fatalf("all fail transmute roll=%+v, want all failed and no rewards", roll)
	}
}

func TestAlchemyMinimumLossFailChanceAppliesActionFloor(t *testing.T) {
	if got := applyAlchemyMinimumLossFailChance(0.03, 0.10); math.Abs(got-0.10) > 0.000001 {
		t.Fatalf("minimum loss fail chance=%v, want 0.10", got)
	}
	if got := applyAlchemyMinimumLossFailChance(0.03, 0.15); math.Abs(got-0.15) > 0.000001 {
		t.Fatalf("convert minimum loss fail chance=%v, want 0.15", got)
	}
	if got := applyAlchemyMinimumLossFailChance(0.35, 0.15); math.Abs(got-0.35) > 0.000001 {
		t.Fatalf("existing fail chance=%v, want 0.35", got)
	}
	if got := applyAlchemyMinimumLossFailChance(0.03, 0); math.Abs(got-0.03) > 0.000001 {
		t.Fatalf("zero floor fail chance=%v, want 0.03", got)
	}
}

func sumInt64Map(values map[string]int64) int64 {
	total := int64(0)
	for _, value := range values {
		total += value
	}
	return total
}

func sameInt64Map(left, right map[string]int64) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func rollAlchemyTransmuteSequentialForTest(actual int, failChance float64, economyBaseValue float64, returnRatio catalog.AlchemyReturnRatio, stability float64, luckyTriggerChance float64, luckyBonusChance float64, luckyTiers []catalog.AlchemyTransmuteTier, rng *rand.Rand) (alchemyTransmuteBatchRoll, error) {
	rewards := map[string]int64{}
	failedCount := 0
	luckyCount := 0
	for i := 0; i < actual; i++ {
		if rng.Float64() < failChance {
			failedCount++
			continue
		}
		ratio := sampleLuckSkewed(returnRatio.MinRatio, returnRatio.MaxRatio, stability, rng)
		gold, err := alchemyGoldFromRatio(economyBaseValue, ratio)
		if err != nil {
			return alchemyTransmuteBatchRoll{}, err
		}
		if rng.Float64() < clampAlchemyFloat(luckyTriggerChance, 0, 1) {
			tier := rollAlchemyTier(luckyTiers, luckyBonusChance, rng)
			if tier.Kind == "gold_multiplier" {
				multiplier := sampleLuckSkewed(1, math.Max(1, tier.Value), luckyBonusChance*2, rng)
				next, err := alchemyGoldAfterMultiplier(gold, multiplier)
				if err != nil {
					return alchemyTransmuteBatchRoll{}, err
				}
				if next > gold {
					gold = next
					luckyCount++
				}
			}
		}
		if err := addAlchemyRewardQuantity(rewards, GoldCoinItemID, gold); err != nil {
			return alchemyTransmuteBatchRoll{}, err
		}
	}
	return alchemyTransmuteBatchRoll{
		Rewards:     rewards,
		FailedCount: failedCount,
		LuckyCount:  luckyCount,
	}, nil
}

func TestAlchemyLevelAdjustedFailChanceAllowsBelowLevelWithPenalty(t *testing.T) {
	s := &Store{config: RuntimeConfig{Alchemy: AlchemyRuntimeConfig{
		LevelDeficitFailChancePerLevel: 0.06,
		LevelDeficitFailChanceCap:      0.85,
	}}}
	if got, want := s.alchemyLevelAdjustedFailChance(0.15, 10, 15, 0.002), 0.45; math.Abs(got-want) > 0.000001 {
		t.Fatalf("below-level fail chance=%v, want %v", got, want)
	}
	if got, want := s.alchemyLevelAdjustedFailChance(0.15, 1, 35, 0.002), 1.0; math.Abs(got-want) > 0.000001 {
		t.Fatalf("capped below-level fail chance=%v, want %v", got, want)
	}
}

func TestAlchemyLevelAdjustedFailChanceReducesAboveLevel(t *testing.T) {
	s := &Store{config: RuntimeConfig{Alchemy: AlchemyRuntimeConfig{
		LevelDeficitFailChancePerLevel: 0.06,
		LevelDeficitFailChanceCap:      0.85,
	}}}
	if got, want := s.alchemyLevelAdjustedFailChance(0.15, 25, 20, 0.002), 0.14; math.Abs(got-want) > 0.000001 {
		t.Fatalf("above-level fail chance=%v, want %v", got, want)
	}
	if got := s.alchemyLevelAdjustedFailChance(0.01, 100, 20, 0.002); got != 0 {
		t.Fatalf("above-level fail chance should clamp at 0, got %v", got)
	}
}

func TestAlchemyTraitQuantityFailDeltaUsesExtraBasisAndCap(t *testing.T) {
	trait := catalog.AlchemyTraitEntry{
		UnitBasis:               8,
		FailChancePerExtraBasis: 0.002,
		FailChanceQuantityCap:   0.01,
	}
	if got := alchemyTraitQuantityFailDelta(trait, 8); got != 0 {
		t.Fatalf("first basis quantity delta=%v, want 0", got)
	}
	if got, want := alchemyTraitQuantityFailDelta(trait, 56), 0.01; math.Abs(got-want) > 0.000001 {
		t.Fatalf("capped quantity delta=%v, want %v", got, want)
	}
}

func TestAlchemyTraitMaxQuantityUsesBoundedPursuitStabilityCurve(t *testing.T) {
	trait := catalog.AlchemyTraitEntry{UnitBasis: 20, MaxBasisMultiplier: 10}
	for _, tc := range []struct {
		stability float64
		want      int64
	}{
		{stability: 0, want: 200},
		{stability: 1, want: 400},
		{stability: 5, want: 1200},
		{stability: 10, want: 1600},
		{stability: 15, want: 1960},
		{stability: 20, want: 2400},
		{stability: 25, want: 2780},
		{stability: 30, want: 3000},
		{stability: 40, want: 3200},
		{stability: 60, want: 3600},
		{stability: 100, want: 4400},
		{stability: 200, want: 5400},
	} {
		if got := alchemyTraitMaxQuantity(trait, tc.stability); got != tc.want {
			t.Fatalf("stability %.0f max=%d, want %d", tc.stability, got, tc.want)
		}
	}
	for _, tc := range []struct {
		stability float64
		want      float64
	}{
		{stability: 10, want: 8},
		{stability: 15, want: 9.84375},
		{stability: 20, want: 12},
		{stability: 25, want: 13.90625},
		{stability: 30, want: 15},
		{stability: 40, want: 16},
		{stability: 60, want: 18},
		{stability: 100, want: 22},
		{stability: 120, want: 23.6666667},
		{stability: 150, want: 25.3333333},
		{stability: 200, want: 27},
		{stability: 300, want: 28.6666667},
		{stability: 500, want: 30},
		{stability: 1000, want: 31},
	} {
		if got := alchemyStabilityQuantityMultiplier(tc.stability); math.Abs(got-tc.want) > 0.001 {
			t.Fatalf("stability %.0f multiplier=%.6f, want approximately %.3f", tc.stability, got, tc.want)
		}
	}
	if got := alchemyTraitMaxQuantity(trait, 0.25); got != 240 {
		t.Fatalf("25%% stability max=%d, want 240", got)
	}
	if got := alchemyTraitMaxQuantity(catalog.AlchemyTraitEntry{UnitBasis: 12, MaxBasisMultiplier: 3}, 0.5); got != 48 {
		t.Fatalf("fractional stability should align max to basis, got %d, want 48", got)
	}
	if got := alchemyTraitMaxQuantity(trait, -1); got != 200 {
		t.Fatalf("negative stability should not reduce max, got %d", got)
	}
	if got := alchemyTraitMaxQuantity(catalog.AlchemyTraitEntry{UnitBasis: 0, MaxBasisMultiplier: 3}, 0); got != 3 {
		t.Fatalf("invalid basis should fall back to one, got %d, want 3", got)
	}
	previous := 0.0
	for _, stability := range []float64{0, 1, 5, 10, 15, 20, 25, 30, 40, 60, 100, 120, 150, 200, 300, 500, 1000, 1_000_000} {
		got := alchemyStabilityQuantityMultiplier(stability)
		if got < previous {
			t.Fatalf("curve must be monotonic: stability %.0f multiplier=%.6f, previous=%.6f", stability, got, previous)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) || got > alchemyQuantitySoftCap {
			t.Fatalf("curve must stay finite and at or below %.0fx: stability %.0f multiplier=%.6f", alchemyQuantitySoftCap, stability, got)
		}
		previous = got
	}
	if got := alchemyStabilityQuantityMultiplier(math.MaxFloat64); math.IsNaN(got) || math.IsInf(got, 0) || got > alchemyQuantitySoftCap {
		t.Fatalf("extreme finite stability multiplier=%v, want finite value at or below %.0f", got, alchemyQuantitySoftCap)
	}
	if got := alchemyTraitMaxQuantity(trait, math.MaxFloat64); got > 6400 {
		t.Fatalf("soft-capped material max=%d, want at most 6400", got)
	}
}

func TestAlchemyTraitMaxQuantityAddsFixedFoodCapacityWithoutChangingBasis(t *testing.T) {
	trait := catalog.AlchemyTraitEntry{UnitBasis: 8, MaxBasisMultiplier: 3}
	modifiers := alchemyeffect.Modifiers{ExtraMaterialQuantity: 10}
	if got := alchemyTraitMaxQuantityWithModifiers(trait, 0, modifiers); got != 34 {
		t.Fatalf("buffed max quantity=%d, want 34", got)
	}
	if got := alchemyTraitMaxQuantity(trait, 0); got != 24 {
		t.Fatalf("base max quantity changed to %d, want 24", got)
	}
}

func averageAlchemySample(luck float64) float64 {
	rng := rand.New(rand.NewSource(11))
	total := 0.0
	const n = 5000
	for i := 0; i < n; i++ {
		total += sampleLuckSkewed(0, 1, luck, rng)
	}
	return total / n
}

func inheritTestSlot(kind string, components ...alchemyInheritComponent) EquipmentAffixSlot {
	return EquipmentAffixSlot{
		AffixID:    "custom:test",
		RarityID:   "rare",
		SourceKind: "craft",
		EffectType: "add_stat",
		EffectParams: map[string]any{
			"alchemyKind":       kind,
			"alchemyComponents": components,
		},
	}
}

func inheritTestComponent(itemID, traitID, attr string, value float64) alchemyInheritComponent {
	return alchemyInheritComponent{
		SourceKind:               "material",
		ItemID:                   itemID,
		TraitID:                  traitID,
		Family:                   "alchemy",
		Attr:                     attr,
		ValueRange:               catalog.AlchemyValueRange{Min: value, Max: value},
		CurrentValue:             value,
		MaxEffectivePerEquipment: 2,
		MaxEffectivePerBody:      5,
	}
}

func inheritTestComponentRange(itemID, traitID, attr string, value, minValue, maxValue float64) alchemyInheritComponent {
	component := inheritTestComponent(itemID, traitID, attr, value)
	component.ValueRange = catalog.AlchemyValueRange{Min: minValue, Max: maxValue}
	return component
}

func inheritTestMedium(inheritSlots, suppressSlots int, ignoreKeepChance float64) *catalog.AlchemyInheritMedium {
	return &catalog.AlchemyInheritMedium{
		Modifiers: catalog.AlchemyInheritModifiers{
			InheritSlots:               inheritSlots,
			SuppressSlots:              suppressSlots,
			IgnoreKeepChance:           ignoreKeepChance,
			HybridMergeDivisor:         1.6,
			InbreedMutationChance:      0,
			InbreedGoodChance:          0,
			InbreedGoodMultiplierRange: catalog.AlchemyInheritMultiplierRange{Min: 1.1, Max: 1.1},
			InbreedBadMultiplierRange:  catalog.AlchemyInheritMultiplierRange{Min: 0.5, Max: 0.5},
		},
	}
}
