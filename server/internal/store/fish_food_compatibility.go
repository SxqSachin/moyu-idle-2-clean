package store

import (
	"encoding/json"
	"math"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
	"reflect"
)

// Retired recipe grants cannot attach to new operations. Already
// prepared operations continue using their own immutable snapshots.
func (s *Store) isRetiredFishFoodEffect(def operationeffect.Definition) bool {
	for _, recipe := range s.gameData.FishFoodRecipes {
		if def.Kind == alchemyeffect.KindInfusionCapacityFloorTradeoff {
			for _, id := range recipe.LegacyEffectIDs {
				if id == def.ID {
					return true
				}
			}
		}
		for _, id := range recipe.ReplacedEffectIDs {
			if id == def.ID {
				item := s.gameData.ItemByID(recipe.OutputItemID)
				if item != nil {
					current, ok := s.operationEffectDefinitions[item.UseOperationEffectID]
					if !ok {
						return false
					}
					if current.Kind != def.Kind {
						return true
					}
					if lifeaction.IsWorkMeal(def.Kind) {
						// Serving-only revisions do not revoke already-eaten meals.
						var before, after map[string]any
						return json.Unmarshal(def.Params, &before) != nil || json.Unmarshal(current.Params, &after) != nil || !reflect.DeepEqual(before, after)
					}
					return false
				}
			}
		}
	}
	return false
}

// Convert only explicitly replaced recipes, using the persisted ingredient
// quality rather than looking up mutable fish or modifying inventory rows.
func (s *Store) replaceFishFoodEffect(recipe catalog.FishFoodRecipe, current operationeffect.Definition, raw json.RawMessage) (operationeffect.Definition, error) {
	var food fishFoodItemSnapshot
	if json.Unmarshal(raw, &food) != nil || food.SchemaVersion != 1 || food.RecipeID != recipe.ID || food.QualityRulesetVersion != fishFoodQualityRulesetV2 || len(food.Ingredients) == 0 {
		return current, ErrOperationEffectItemInvalid
	}
	q := food.Quality
	for _, bound := range []struct{ value, min, max float64 }{{q.LengthScore, 0, 1}, {q.WeightScore, 0, 1}, {q.TraitAverage, -1, 1}, {q.SpecialMultiplier, 1, 1.815}} {
		if math.IsNaN(bound.value) || math.IsInf(bound.value, 0) || bound.value < bound.min || bound.value > bound.max {
			return current, ErrOperationEffectItemInvalid
		}
	}
	params, err := scaleFishFoodParamsV2(current.Params, recipe.ParameterScaling, &q)
	if err != nil {
		return current, ErrOperationEffectItemInvalid
	}
	mutations := map[string]bool{}
	for _, row := range food.Ingredients {
		for _, id := range row.Mutations {
			mutations[id] = true
		}
	}
	charges, seconds := fishFoodLifetimeBonus(recipe, mutations)
	current.Params = params
	q.ExtraCharges, q.ExtraDurationSeconds = charges, seconds
	if current.Lifetime.Kind == operationeffect.LifetimeCharges {
		current.Lifetime.Charges = fishFoodScaledCharges(recipe, current.Lifetime.Charges, &q)
	} else {
		current.Lifetime.DurationSeconds = fishFoodScaledDuration(recipe, current.Lifetime.DurationSeconds, &q)
	}
	if err := s.operationEffectRegistry.ValidateDefinition(current); err != nil {
		return current, ErrOperationEffectItemInvalid
	}
	return current, nil
}

// Historical item definitions remain readable. Their old grants must still
// participate in the new family's exclusivity, in either activation order.
func (s *Store) operationEffectExclusiveFamily(def operationeffect.Definition) (string, []string, bool) {
	key, exclusive := def.Stacking.Key, def.Stacking.Exclusive
	for _, recipe := range s.gameData.FishFoodRecipes {
		for _, legacyID := range s.fishFoodCompatibleEffectIDs(recipe) {
			if legacyID != def.ID {
				continue
			}
			if item := s.gameData.ItemByID(recipe.OutputItemID); item != nil {
				if current, ok := s.operationEffectDefinitions[item.UseOperationEffectID]; ok {
					key, exclusive = current.Stacking.Key, current.Stacking.Exclusive
				}
			}
		}
	}
	ids := []string{}
	for _, recipe := range s.gameData.FishFoodRecipes {
		item := s.gameData.ItemByID(recipe.OutputItemID)
		if item == nil {
			continue
		}
		current, ok := s.operationEffectDefinitions[item.UseOperationEffectID]
		if ok && current.Stacking.Key == key {
			ids = append(ids, current.ID)
			ids = append(ids, s.fishFoodCompatibleEffectIDs(recipe)...)
		}
	}
	return key, ids, exclusive
}

func (s *Store) fishFoodCompatibleEffectIDs(recipe catalog.FishFoodRecipe) []string {
	ids := append([]string(nil), recipe.LegacyEffectIDs...)
	for _, id := range recipe.ReplacedEffectIDs {
		if def, ok := s.operationEffectDefinitions[id]; ok && !s.isRetiredFishFoodEffect(def) {
			ids = append(ids, id)
		}
	}
	return ids
}
