package store

import (
	"encoding/json"
	"math"
	"testing"
)

func TestFishCuisineSoupBalanceAndFrozenItems(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, tc := range []struct {
		item, parameter string
		old, base, peak float64
	}{
		{"mossfire_dual_fish_soup", "extraMaterialQuantity", 25, 45, 327},
		{"starlight_silverfish_soup", "positiveCeilingBonusRate", .03, .05, .12},
		{"steadyfin_soup", "bonusRate", .008, .02, .05},
		{"frugal_river_soup", "rate", .04, .07, .15},
		{"harvest_river_soup", "rate", .025, .05, .1293},
		{"reedbank_soup", "bonusRate", .025, .08, .20},
	} {
		t.Run(tc.item, func(t *testing.T) {
			def, ok, err := s.resolveOperationEffectDefinitionForItem(tc.item, nil)
			if err != nil || !ok {
				t.Fatal(err)
			}
			var params map[string]float64
			if err := json.Unmarshal(def.Params, &params); err != nil {
				t.Fatal(err)
			}
			if params[tc.parameter] != tc.base || def.Lifetime.DurationSeconds != 600 {
				t.Fatal("unexpected soup base or duration", params, def.Lifetime)
			}
			recipe := s.gameData.FishFoodRecipeByOutputItemID(tc.item)
			quality := fishFoodQualitySnapshot{LengthScore: 1, WeightScore: 1, TraitAverage: 1, SpecialMultiplier: 1.815}
			raw, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, &quality)
			if err != nil {
				t.Fatal(err)
			}
			var peak map[string]float64
			if err := json.Unmarshal(raw, &peak); err != nil {
				t.Fatal(err)
			}
			if math.Abs(peak[tc.parameter]-tc.peak) > .000001 {
				t.Fatal("unexpected triple-mutation peak", peak)
			}
			params[tc.parameter] = tc.old
			if tc.item == "mossfire_dual_fish_soup" {
				params["positiveFloorReductionRate"] = .12
			}
			oldParams, _ := json.Marshal(params)
			snapshot, _ := json.Marshal(map[string]any{"operationEffect": operationEffectItemSnapshot{SchemaVersion: 1, DefinitionID: def.ID, RulesetVersion: def.RulesetVersion, Params: oldParams}})
			old, ok, err := s.resolveOperationEffectDefinitionForItem(tc.item, snapshot)
			if err != nil || !ok || !operationEffectParamsEqual(old.Params, oldParams) {
				t.Fatal("balance edit changed an existing custom item's frozen parameters", err)
			}
		})
	}
}
