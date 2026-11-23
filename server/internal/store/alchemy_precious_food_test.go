package store

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestFoodSuccessPenaltyAffectsTraitAndComboCrafts(t *testing.T) {
	s := arenaAlchemyGateTestStore()
	for _, combo := range []string{"", "test_combo"} {
		if combo == "" {
			s.gameData.AlchemyComboRecipes = nil
		} else {
			s.gameData.AlchemyComboRecipes = arenaAlchemyGateTestStore().gameData.AlchemyComboRecipes
		}
		for _, medium := range []*catalog.AlchemyMedium{nil, {Modifiers: catalog.AlchemyMediumModifiers{FailChanceDelta: -.045, SuccessFloor: .16}}} {
			base, err := s.resolveAlchemyCraftWithModifiers(map[string]int64{"test_material": 1}, combo, medium, alchemyDerived{Level: 50, Stability: 1_000_000}, alchemyeffect.Modifiers{}, rand.New(rand.NewSource(3)))
			if err != nil {
				t.Fatal(combo, err)
			}
			boosted, err := s.resolveAlchemyCraftWithModifiers(map[string]int64{"test_material": 1}, combo, medium, alchemyDerived{Level: 50, Stability: 1_000_000}, alchemyeffect.Modifiers{SuccessRatePenalty: .06}, rand.New(rand.NewSource(3)))
			if err != nil || math.Abs(boosted.FailChance-base.FailChance-.06) > 1e-9 {
				t.Fatal("food penalty lost or multiplied", combo, base.FailChance, boosted.FailChance, err)
			}
		}
	}
}

func TestFrostmoonFoodQualityAndFrozenCapacity(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, id := range []string{"frostmoon_dual_fish_grill", "frostmoon_dual_fish_soup"} {
		recipe := *s.gameData.FishFoodRecipeByOutputItemID(id)
		def, ok, err := s.resolveOperationEffectDefinitionForItem(id, nil)
		if err != nil || !ok {
			t.Fatal(id, err)
		}
		for _, tc := range []struct{ score, mutation, want float64 }{{0, 1, 25}, {.5, 1, 31}, {1, 1, 50}, {1, 1.815, 91}} {
			quality := fishFoodQualitySnapshot{LengthScore: tc.score, WeightScore: tc.score, SpecialMultiplier: tc.mutation}
			raw, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, &quality)
			if err != nil {
				t.Fatal(err)
			}
			var params map[string]float64
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatal(err)
			}
			if len(params) != 3 || params["extraMaterialQuantity"] != tc.want || params["quantityLimitBonusRate"] < .07 || params["quantityLimitBonusRate"] > .12 || params["successRatePenalty"] < .015 || params["successRatePenalty"] > .045 {
				t.Fatal(id, tc, params)
			}
			saved, _ := json.Marshal(map[string]any{"operationEffect": operationEffectItemSnapshot{SchemaVersion: 1, DefinitionID: def.ID, RulesetVersion: def.RulesetVersion, Params: raw}})
			frozen, ok, err := s.resolveOperationEffectDefinitionForItem(id, saved)
			if err != nil || !ok || !operationEffectParamsEqual(frozen.Params, raw) {
				t.Fatal("frozen food changed", err)
			}
		}
	}
}

func TestFrostmoonActualMaterialsChargesAndPersistedStamp(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	var high string
	for _, item := range s.gameData.Items {
		if item.Rarity == "mythic" && s.gameData.ItemTraitByItemID(item.ID) != nil {
			high = item.ID
			break
		}
	}
	if high == "" {
		t.Fatal("missing mythic material fixture")
	}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	op := func(seq int, start time.Time, materials map[string]int64) operationeffect.Operation {
		args, _ := json.Marshal(alchemyBenchArgs{Materials: materials})
		raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: args})
		item := QueueRuntimeItem{QueueID: 7, ActionID: "alchemy.bench", ActorType: ActionActorTypeKitty, ActorID: 3}
		return enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, start, seq, raw), raw)
	}
	grill := s.operationEffectDefinitions["food_frostmoon_dual_fish_grill_v3"]
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: grill, Target: operationeffect.Target{Kind: grill.AllowedTargets[0]}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1}})
	if err != nil {
		t.Fatal(err)
	}
	for i, materials := range []map[string]int64{{"coal": 1}, {high: 0}, {high: 1, "coal": 1}, {high: 1}} {
		operation := op(i+1, at.Add(time.Duration(i)*time.Second), materials)
		if i == 2 && (!operation.Capabilities[alchemyeffect.CapabilityBulkMaterial] || !operation.Capabilities[alchemyeffect.CapabilityPreciousMaterial]) {
			t.Fatal("mixed selection lost a rarity capability")
		}
		snapshot, err := session.Commit(session.Prepare(operation))
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if i == 2 {
			want = 1
		}
		if len(snapshot.Rules) != want {
			t.Fatal("incorrect precious charge consumption", i, snapshot)
		}
		if want == 0 {
			continue
		}
		craft := &resolvedAlchemyCraft{Components: []alchemyCraftComponent{{}}}
		if err := stampAlchemyCraftOperationEffects(craft, snapshot); err != nil {
			t.Fatal(err)
		}
		m, ok := alchemyOperationEffectModifiersFromComponent(alchemyInheritComponent{OperationEffects: craft.Components[0].OperationEffects})
		if !ok || m.PreciousMaterialQuantity != 25 || m.QuantityBonus("epic", 50) != 0 || m.PositiveFloorReductionRate != 0 || m.SuccessRatePenalty != .06 {
			t.Fatal("persisted pure-capacity stamp lost", m)
		}
	}
	if len(session.ChargeReceipts()) != 1 {
		t.Fatal("wrong charge ledger")
	}
	soup := s.operationEffectDefinitions["food_frostmoon_dual_fish_soup_v3"]
	until := at.Add(10 * time.Minute)
	session, err = operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 2, Definition: soup, Target: operationeffect.Target{Kind: soup.AllowedTargets[0]}, ValidFrom: at, ValidUntil: &until}})
	if err != nil {
		t.Fatal(err)
	}
	for i, start := range []time.Time{at.Add(-time.Second), until.Add(-time.Millisecond), until, until.Add(time.Hour)} {
		snapshot, err := session.Commit(session.Prepare(op(i+1, start, map[string]int64{high: 1})))
		if err != nil {
			t.Fatal(err)
		}
		m, err := alchemyeffect.Resolve(snapshot)
		want := int64(0)
		wantPenalty := 0.0
		if i == 1 {
			want = 25
			wantPenalty = .05
		}
		if err != nil || m.PreciousMaterialQuantity != want || m.SuccessRatePenalty != wantPenalty {
			t.Fatal("historical start eligibility changed", start, m, err)
		}
	}
}

func TestFrostmoonServerQuantityValidation(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, itemID := range []string{"moonwood", "base4_cursed_wing_membrane"} {
		trait := s.gameData.ItemTraitByItemID(itemID).Traits[0]
		basis := alchemyTraitUnitBasis(trait)
		base := alchemyTraitMaxQuantity(trait, 0)
		m := alchemyeffect.Modifiers{PreciousMaterialQuantity: 25}
		maximum := s.alchemyMaterialMaxQuantity(itemID, trait, 0, m) / basis * basis
		if err := s.validateAlchemyMaterialQuantities(map[string]int64{itemID: maximum}, alchemyDerived{}, m); err != nil {
			t.Fatal(itemID, err)
		}
		if err := s.validateAlchemyMaterialQuantities(map[string]int64{itemID: maximum + basis}, alchemyDerived{}, m); !errors.Is(err, ErrAlchemyMaterialOverflow) {
			t.Fatal("accepted excessive quantity", itemID, err)
		}
		if itemID == "moonwood" {
			if maximum <= base {
				t.Fatal("legendary capacity did not grow")
			}
			if err := s.validateAlchemyMaterialQuantities(map[string]int64{itemID: maximum}, alchemyDerived{}, alchemyeffect.Modifiers{}); !errors.Is(err, ErrAlchemyMaterialOverflow) {
				t.Fatal("missing snapshot still accepted boosted quantity", err)
			}
		} else if maximum != base {
			t.Fatal("epic material received precious bonus")
		}
	}
}

func TestCapacityFoodBreakthroughSurvivesServerValidationAndStamp(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, id := range []string{"moonwood", "base4_cursed_wing_membrane", "wolf_king_fang"} {
		trait := s.gameData.ItemTraitByItemID(id).Traits[0]
		basis := alchemyTraitUnitBasis(trait)
		derived := alchemyDerived{Stability: 1_000_000}
		base := alchemyTraitMaxQuantity(trait, derived.Stability)
		absolute := arenaAlchemyTraitAbsoluteMaxQuantity(trait)
		kind := alchemyeffect.KindInfusionPreciousCapacityBonus
		params := json.RawMessage(`{"extraMaterialQuantity":91,"quantityLimitBonusRate":0.17}`)
		if id == "base4_cursed_wing_membrane" {
			kind = alchemyeffect.KindInfusionBulkCapacityFloorTradeoff
			params = json.RawMessage(`{"extraMaterialQuantity":363,"quantityLimitBonusRate":0.17,"positiveFloorReductionRate":0.1}`)
		}
		m, err := resolveAlchemyOperationEffectStamp(&alchemyOperationEffectStamp{Version: alchemyOperationEffectStampVersion, Rules: []alchemyOperationEffectStampRule{{DefinitionID: "capacity", RulesetVersion: "breakthrough-v1", Kind: kind, Params: params}}})
		if err != nil {
			t.Fatal(err)
		}
		quantity := s.alchemyMaterialMaxQuantity(id, trait, derived.Stability, m) / basis * basis
		if quantity <= absolute || float64(quantity)/float64(base) > 1.170001 {
			t.Fatal("endgame capacity not bounded above old ceiling", id, base, absolute, quantity)
		}
		if err := s.validateAlchemyMaterialQuantities(map[string]int64{id: quantity}, derived, m); err != nil {
			t.Fatal(err)
		}
		if err := s.validateAlchemyMaterialQuantities(map[string]int64{id: quantity}, derived, alchemyeffect.Modifiers{}); !errors.Is(err, ErrAlchemyMaterialOverflow) {
			t.Fatal("unbuffed operation accepted breakthrough", err)
		}
	}
}

func TestFrostmoonArenaAcceptsFrozenCapacityOnlyForPreciousMaterials(t *testing.T) {
	s := arenaAlchemyGateTestStore()
	s.gameData.Items = []catalog.Item{{ID: "test_material", Rarity: "legendary"}}
	craft := resolvedAlchemyCraft{EffectType: "add_stat", EffectParams: map[string]any{"alchemyKind": "combat"}, AttrMods: map[string]float64{"speed": .65}, Components: []alchemyCraftComponent{{SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait", Attr: "speed", Quantity: 42, CurrentValue: .65}}}
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{DefinitionID: "food_frostmoon_dual_fish_grill_v1", RulesetVersion: "food-frostmoon-v1", Domain: operationeffect.DomainAlchemy, Kind: alchemyeffect.KindInfusionPreciousCapacityBonus, Params: json.RawMessage(`{"extraMaterialQuantity":25}`)}}}
	if err := stampAlchemyCraftOperationEffects(&craft, snapshot); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(craft.EffectParams)
	if err != nil {
		t.Fatal(err)
	}
	craft.EffectParams = map[string]any{}
	if err := json.Unmarshal(encoded, &craft.EffectParams); err != nil {
		t.Fatal(err)
	}
	item := EquippedItem{InstanceID: 123, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{alchemyCraftSlot(craft)}}
	if violations := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(violations) != 0 {
		t.Fatal("valid precious stamp rejected", violations)
	}
	s.gameData.Items[0].Rarity = "epic"
	if violations := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(violations) != 1 {
		t.Fatal("precious stamp incorrectly covered epic material", violations)
	}
}

func TestCapacityBreakthroughKeepsHistoricalFoodParams(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, tc := range []struct{ item, oldID string }{
		{"mossfire_dual_fish_grill", "food_mossfire_dual_fish_grill_v4"},
		{"mossfire_dual_fish_soup", "food_mossfire_dual_fish_soup_v4"},
		{"frostmoon_dual_fish_grill", "food_frostmoon_dual_fish_grill_v1"},
		{"frostmoon_dual_fish_soup", "food_frostmoon_dual_fish_soup_v1"},
		{"mossfire_dual_fish_grill", "food_mossfire_dual_fish_grill_v5"},
		{"mossfire_dual_fish_soup", "food_mossfire_dual_fish_soup_v5"},
		{"frostmoon_dual_fish_grill", "food_frostmoon_dual_fish_grill_v2"},
		{"frostmoon_dual_fish_soup", "food_frostmoon_dual_fish_soup_v2"},
	} {
		old := s.operationEffectDefinitions[tc.oldID]
		raw, err := json.Marshal(map[string]any{"operationEffect": operationEffectItemSnapshot{SchemaVersion: 1, DefinitionID: old.ID, RulesetVersion: old.RulesetVersion, Params: old.Params}})
		if err != nil {
			t.Fatal(err)
		}
		resolved, ok, err := s.resolveOperationEffectDefinitionForItem(tc.item, raw)
		if err != nil || !ok || !operationEffectParamsEqual(resolved.Params, old.Params) {
			t.Fatal("historical food parameters were changed", tc.item, resolved, err)
		}
		var params map[string]float64
		if err := json.Unmarshal(resolved.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params["successRatePenalty"] != 0 {
			t.Fatal("historical food unexpectedly received success penalty", tc.item)
		}
	}
}

func TestArenaAcceptsProportionalCapacityStampAfterSerialization(t *testing.T) {
	s := arenaAlchemyGateTestStore()
	s.gameData.Items = []catalog.Item{{ID: "test_material", Rarity: "legendary"}}
	craft := resolvedAlchemyCraft{
		EffectType: "add_stat", EffectParams: map[string]any{"alchemyKind": "combat"}, AttrMods: map[string]float64{"speed": .60},
		Components: []alchemyCraftComponent{{SourceKind: "trait_effect", ItemID: "test_material", TraitID: "test_trait", Attr: "speed", Quantity: 36, CurrentValue: .60}},
	}
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{
		DefinitionID: "food_frostmoon_dual_fish_grill_v2", RulesetVersion: "food-capacity-breakthrough-v1", Domain: operationeffect.DomainAlchemy,
		Kind: alchemyeffect.KindInfusionPreciousCapacityBonus, Params: json.RawMessage(`{"extraMaterialQuantity":1,"quantityLimitBonusRate":0.17}`),
	}}}
	if err := stampAlchemyCraftOperationEffects(&craft, snapshot); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(craft.EffectParams)
	if err != nil {
		t.Fatal(err)
	}
	craft.EffectParams = map[string]any{}
	if err := json.Unmarshal(encoded, &craft.EffectParams); err != nil {
		t.Fatal(err)
	}
	item := EquippedItem{InstanceID: 124, TemplateItemID: "test_blade", EquipType: "battle", Affixes: []EquipmentAffixSlot{alchemyCraftSlot(craft)}}
	if violations := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(violations) != 0 {
		t.Fatal("proportional capacity lost after serialization", violations)
	}
	s.gameData.Items[0].Rarity = "epic"
	if violations := s.ArenaAlchemyEquipmentViolations([]EquippedItem{item}); len(violations) != 1 {
		t.Fatal("precious proportional capacity incorrectly covered epic material", violations)
	}
}
