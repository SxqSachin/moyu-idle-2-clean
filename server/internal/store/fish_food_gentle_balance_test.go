package store

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestCapacityFoodHalvesRiskWithoutChangingOtherParameters(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, series := range []struct{ name, oldVersion string }{{"mossfire", "7"}, {"lotusflame", "1"}, {"frostmoon", "4"}} {
		for _, form := range []string{"grill", "soup"} {
			id := series.name + "_dual_fish_" + form
			old := s.operationEffectDefinitions["food_"+id+"_v"+series.oldVersion]
			current, ok, err := s.resolveOperationEffectDefinitionForItem(id, nil)
			if err != nil || !ok || current.ID == old.ID {
				t.Fatal(id, current.ID, err)
			}
			var before, after map[string]float64
			if err := json.Unmarshal(old.Params, &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(current.Params, &after); err != nil {
				t.Fatal(err)
			}
			after["successRatePenalty"] *= 2
			if !reflect.DeepEqual(before, after) {
				t.Fatal("changed capacity or floor", id, before, after)
			}
			recipe := s.gameData.FishFoodRecipeByOutputItemID(id)
			oldScales := make(map[string]catalog.FishFoodParameterScale, len(recipe.ParameterScaling))
			for name, scale := range recipe.ParameterScaling {
				if name == "successRatePenalty" {
					scale.Min *= 2
					scale.Max *= 2
				}
				oldScales[name] = scale
			}
			for _, factor := range []float64{1, 1.815} {
				q1 := fishFoodQualitySnapshot{LengthScore: .8, WeightScore: .9, TraitAverage: .7, SpecialMultiplier: factor}
				q2 := q1
				x, err := scaleFishFoodParamsV2(old.Params, oldScales, &q1)
				if err != nil {
					t.Fatal(err)
				}
				y, err := scaleFishFoodParamsV2(current.Params, recipe.ParameterScaling, &q2)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(x, &before); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(y, &after); err != nil {
					t.Fatal(err)
				}
				if math.Abs(before["successRatePenalty"]/2-after["successRatePenalty"]) > .0001 {
					t.Fatal(id, before, after)
				}
			}
		}
	}
}
