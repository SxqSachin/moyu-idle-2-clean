package store

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestFishFoodExecutorValidatesAutomaticAndSpecificArguments(t *testing.T) {
	executor := &fishFoodExecutor{recipe: fishFoodTestRecipe()}

	automatic, err := executor.ValidateAndNormalizeArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	autoArgs, ok := automatic.(fishFoodExecutorArgs)
	if !ok || autoArgs.IngredientMode != fishFoodIngredientModeAuto || autoArgs.RequiresSingleQueueRun() {
		t.Fatalf("unexpected automatic args: %#v", automatic)
	}

	raw := json.RawMessage(`{
		"ingredientMode":"specific",
		"fishIngredientInstances":{"crucian_carp":[23],"bluegill":[11]}
	}`)
	specific, err := executor.ValidateAndNormalizeArgs(raw)
	if err != nil {
		t.Fatal(err)
	}
	specificArgs, ok := specific.(fishFoodExecutorArgs)
	if !ok || !specificArgs.RequiresSingleQueueRun() {
		t.Fatalf("specific ingredients must require one queue run: %#v", specific)
	}
	if !reflect.DeepEqual(specificArgs.FishIngredientInstances, map[string][]int64{
		"crucian_carp": {23},
		"bluegill":     {11},
	}) {
		t.Fatalf("specific bindings were not normalized: %#v", specificArgs.FishIngredientInstances)
	}
}

func TestFishFoodExecutorRejectsIncompleteOrDuplicateSpecificIngredients(t *testing.T) {
	executor := &fishFoodExecutor{recipe: fishFoodTestRecipe()}
	tests := []json.RawMessage{
		json.RawMessage(`{"ingredientMode":"specific","fishIngredientInstances":{"crucian_carp":[7]}}`),
		json.RawMessage(`{"ingredientMode":"specific","fishIngredientInstances":{"crucian_carp":[7],"bluegill":[7]}}`),
		json.RawMessage(`{"ingredientMode":"automatic","fishIngredientInstances":{"crucian_carp":[7]}}`),
		json.RawMessage(`{"ingredientMode":"unknown"}`),
	}
	for _, raw := range tests {
		if _, err := executor.ValidateAndNormalizeArgs(raw); !errors.Is(err, ErrFishFoodArgsInvalid) {
			t.Fatalf("raw=%s: err=%v, want ErrFishFoodArgsInvalid", raw, err)
		}
	}
}

func TestFishFoodQualityRulesetUsesSizeWeightTraitsAndMutations(t *testing.T) {
	species := catalog.FishingSpecies{ID: "test", MinLengthCM: 10, MaxLengthCM: 50, ShapeFactor: 1.25}
	midLength := 30.0
	midWeight := species.ShapeFactor * math.Pow(midLength, 3) / 100_000
	mid := fishFoodIngredientQualityV1(species, FishInstanceData{
		SpeciesID: "test",
		LengthCM:  midLength,
		WeightKG:  midWeight,
	})
	if !almostEqual(mid.Multiplier, 1, 0.0001) {
		t.Fatalf("midpoint multiplier=%v, want 1", mid.Multiplier)
	}

	high := fishFoodIngredientQualityV1(species, FishInstanceData{
		SpeciesID: "test",
		LengthCM:  species.MaxLengthCM,
		WeightKG:  species.ShapeFactor * math.Pow(species.MaxLengthCM, 3) / 100_000,
		Genes: FishGenes{
			SizePotential: 1, GrowthRate: 1, FeedEfficiency: 1,
			Vitality: 1, Lifespan: 1, Fertility: 1,
			Mutations: []string{"m3", "m1", "m2", "m1"},
		},
	})
	if high.Multiplier != 1.35 || !reflect.DeepEqual(high.Mutations, []string{"m1", "m2", "m3"}) {
		t.Fatalf("high quality was not capped/deduplicated deterministically: %#v", high)
	}

	low := fishFoodIngredientQualityV1(species, FishInstanceData{
		SpeciesID: "test",
		LengthCM:  species.MinLengthCM,
		WeightKG:  species.ShapeFactor * math.Pow(species.MinLengthCM, 3) / 100_000,
		Genes: FishGenes{
			SizePotential: -1, GrowthRate: -1, FeedEfficiency: -1,
			Vitality: -1, Lifespan: -1, Fertility: -1,
		},
	})
	if low.Multiplier != 0.75 {
		t.Fatalf("low quality multiplier=%v, want lower cap 0.75", low.Multiplier)
	}
}

func TestScaledFishFoodEffectParamsImprovesBenefitAndReducesTradeoff(t *testing.T) {
	capacityRaw, _ := json.Marshal(alchemyeffect.CapacityFloorTradeoffParams{
		ExtraMaterialQuantity:      40,
		PositiveFloorReductionRate: 0.12,
	})
	capacityRaw, err := scaledFishFoodEffectParams(operationeffect.Definition{
		Kind:   alchemyeffect.KindInfusionCapacityFloorTradeoff,
		Params: capacityRaw,
	}, 1.25)
	if err != nil {
		t.Fatal(err)
	}
	var capacity alchemyeffect.CapacityFloorTradeoffParams
	if err := json.Unmarshal(capacityRaw, &capacity); err != nil {
		t.Fatal(err)
	}
	if capacity.ExtraMaterialQuantity != 50 || !almostEqual(capacity.PositiveFloorReductionRate, 0.096, 0.0001) {
		t.Fatalf("unexpected capacity params: %#v", capacity)
	}

	precisionRaw, _ := json.Marshal(alchemyeffect.PrecisionCeilingBonusParams{PositiveCeilingBonusRate: 0.06})
	precisionRaw, err = scaledFishFoodEffectParams(operationeffect.Definition{
		Kind:   alchemyeffect.KindInfusionPrecisionCeilingBonus,
		Params: precisionRaw,
	}, 1.25)
	if err != nil {
		t.Fatal(err)
	}
	var precision alchemyeffect.PrecisionCeilingBonusParams
	if err := json.Unmarshal(precisionRaw, &precision); err != nil {
		t.Fatal(err)
	}
	if !almostEqual(precision.PositiveCeilingBonusRate, 0.075, 0.0001) {
		t.Fatalf("unexpected precision params: %#v", precision)
	}
}

func TestOperationEffectDefinitionForItemAllowsOnlyValidatedParamOverrides(t *testing.T) {
	data := catalog.Data{
		Items: []catalog.Item{{
			ID:                   "test_food",
			Type:                 []string{"food"},
			UseOperationEffectID: "test_food_effect",
		}},
		OperationEffects: []catalog.OperationEffectDefinition{{
			ID:             "test_food_effect",
			RulesetVersion: "food-v1",
			Domain:         "alchemy",
			Kind:           alchemyeffect.KindInfusionPrecisionCeilingBonus,
			NameKey:        "test.name",
			DescKey:        "test.desc",
			AllowedTargets: []string{"player"},
			Lifetime:       catalog.OperationEffectLifetime{Kind: "duration", DurationSeconds: 60},
			Stacking:       catalog.OperationEffectStacking{Key: "test", Mode: "extend_duration", MaxDurationSeconds: 180},
			Selector:       catalog.OperationEffectSelector{ActionIDs: []string{"alchemy.bench"}},
			Params:         json.RawMessage(`{"positiveCeilingBonusRate":0.06}`),
		}},
	}
	registry, definitions, err := buildOperationEffectRuntime(data)
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{gameData: data, operationEffectRegistry: registry, operationEffectDefinitions: definitions}

	view, ok, err := store.OperationEffectDefinitionForItem("test_food", map[string]any{
		"operationEffect": map[string]any{
			"schemaVersion":  1,
			"definitionId":   "test_food_effect",
			"rulesetVersion": "food-v1",
			"params": map[string]any{
				"positiveCeilingBonusRate": 0.075,
			},
		},
	})
	if err != nil || !ok {
		t.Fatalf("valid override failed: ok=%v err=%v", ok, err)
	}
	var params alchemyeffect.PrecisionCeilingBonusParams
	if err := json.Unmarshal(view.Params, &params); err != nil || !almostEqual(params.PositiveCeilingBonusRate, 0.075, 0.0001) {
		t.Fatalf("unexpected overridden params: raw=%s parsed=%#v err=%v", view.Params, params, err)
	}

	_, _, err = store.OperationEffectDefinitionForItem("test_food", map[string]any{
		"operationEffect": map[string]any{
			"schemaVersion":  1,
			"definitionId":   "test_food_effect",
			"rulesetVersion": "food-v1",
			"params": map[string]any{
				"positiveCeilingBonusRate": 0.075,
				"unexpected":               true,
			},
		},
	})
	if !errors.Is(err, ErrOperationEffectItemInvalid) {
		t.Fatalf("invalid params err=%v, want ErrOperationEffectItemInvalid", err)
	}
}

func TestFishFoodMarketPathsKeepCustomSnapshotsOutOfTemplateListings(t *testing.T) {
	marketStore := &Store{gameData: catalog.Data{Items: []catalog.Item{{
		ID:             "test_food",
		Type:           []string{"food"},
		MarketListable: true,
	}}}}
	standard := LockedItemInstance{
		TemplateItem:  "test_food",
		Quantity:      1,
		BaseSnapshot:  []byte(`{}`),
		AffixesRaw:    []byte(`[]`),
		Durability:    100,
		MaxDurability: 100,
		State:         "inventory",
	}
	if !marketStore.lockedInstanceIsTemplateCommodity(standard) {
		t.Fatal("standard fish food must remain eligible for template-level listings")
	}
	custom := standard
	custom.BaseSnapshot = []byte(`{"operationEffect":{"schemaVersion":1}}`)
	if marketStore.lockedInstanceIsTemplateCommodity(custom) {
		t.Fatal("custom fish food snapshot must never be flattened into a template listing")
	}
	if !marketStore.auctionItemListable(custom.TemplateItem) {
		t.Fatal("custom fish food must remain eligible for instance auction")
	}
}

func fishFoodTestRecipe() catalog.FishFoodRecipe {
	return catalog.FishFoodRecipe{
		ID:                       "test_recipe",
		RulesetVersion:           "fish-food-v1",
		ActionID:                 "test_action",
		OutputItemID:             "test_food",
		Kind:                     "grilled_fish",
		QualityRulesetVersion:    fishFoodQualityRulesetV1,
		AllowSpecificIngredients: true,
		FishIngredients: []catalog.FishFoodIngredient{
			{SpeciesID: "crucian_carp", Quantity: 1},
			{SpeciesID: "bluegill", Quantity: 1},
		},
	}
}

func almostEqual(left, right, tolerance float64) bool {
	return math.Abs(left-right) <= tolerance
}
