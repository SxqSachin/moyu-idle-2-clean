package store

import (
	"encoding/json"
	"math"

	"mysecondwebgame/server/internal/catalog"
)

const fishFoodQualityRulesetV2 = "fish-food-quality-v2"

// Normalize size within the species, but weight by condition at the actual
// length. Growing longer must not count a second time as growing heavier.
func (e *fishFoodExecutor) resolveQualityV2(selected []selectedFishFoodIngredient) (fishFoodQualitySnapshot, []fishFoodIngredientSnapshot, error) {
	if len(selected) == 0 {
		return fishFoodQualitySnapshot{}, nil, ErrFishFoodIngredientsInvalid
	}
	q := fishFoodQualitySnapshot{SpecialMultiplier: 1}
	rows := make([]fishFoodIngredientSnapshot, 0, len(selected))
	mutations := map[string]bool{}
	for _, selectedFish := range selected {
		fish := selectedFish.Fish
		species := e.store.gameData.FishingSpeciesByID(fish.SpeciesID)
		if species == nil || fish.LengthCM <= 0 || fish.WeightKG <= 0 || species.ShapeFactor <= 0 || math.IsNaN(fish.LengthCM) || math.IsNaN(fish.WeightKG) || math.IsInf(fish.LengthCM, 0) || math.IsInf(fish.WeightKG, 0) {
			return q, nil, ErrFishFoodIngredientsInvalid
		}
		row := fishFoodIngredientQualityV1(*species, fish)
		condition := fish.WeightKG / (species.ShapeFactor * math.Pow(fish.LengthCM, 3) / 100_000)
		row.WeightScore = roundFloat(clampFloat((condition-0.75)/0.5, 0, 1), 4)
		row.Multiplier = 1
		rows = append(rows, row)
		q.LengthScore += row.LengthScore
		q.WeightScore += row.WeightScore
		q.TraitAverage += row.TraitAverage
		for _, id := range row.Mutations {
			mutations[id] = true
		}
	}
	n := float64(len(rows))
	q.LengthScore = roundFloat(q.LengthScore/n, 4)
	q.WeightScore = roundFloat(q.WeightScore/n, 4)
	q.TraitAverage = roundFloat(q.TraitAverage/n, 4)
	q.MutationCount = len(mutations)
	for _, special := range []struct {
		id         string
		multiplier float64
	}{{"shimmering", 1.50}, {"prismatic", 1.10}, {"rainbow", 1.10}} {
		if mutations[special.id] {
			q.SpecialMultiplier *= special.multiplier
			q.SpecialMutations = append(q.SpecialMutations, special.id)
		}
	}
	q.SpecialMultiplier = roundFloat(q.SpecialMultiplier, 4)
	q.Multiplier = q.SpecialMultiplier
	q.ExtraCharges, q.ExtraDurationSeconds = fishFoodLifetimeBonus(e.recipe, mutations)
	return q, rows, nil
}

func fishFoodLifetimeBonus(recipe catalog.FishFoodRecipe, mutations map[string]bool) (charges, seconds int64) {
	bonuses := recipe.MutationLifetimeBonuses
	if bonuses == nil {
		bonuses = map[string]catalog.FishFoodMutationLifetimeBonus{"prismatic": {Charges: 1}, "rainbow": {DurationSeconds: 900}}
	}
	for id, bonus := range bonuses {
		if mutations != nil && !mutations[id] {
			continue
		}
		if recipe.Kind == "grilled_fish" {
			charges += bonus.Charges
		} else {
			seconds += bonus.DurationSeconds
		}
	}
	return
}

func scaleFishFoodParamsV2(raw json.RawMessage, scales map[string]catalog.FishFoodParameterScale, q *fishFoodQualitySnapshot) (json.RawMessage, error) {
	var params map[string]float64
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	q.ParameterMultipliers = map[string]float64{}
	for name, scale := range scales {
		base, exists := params[name]
		if !exists || scale.Length < 0 || scale.Weight < 0 || scale.Traits < 0 || (scale.Curve == "" && scale.Length+scale.Weight+scale.Traits > 0.8) || scale.Min < 0 || scale.Max < scale.Min {
			return nil, ErrFishFoodIngredientsInvalid
		}
		multiplier := roundFloat((1+(q.LengthScore-0.5)*scale.Length+(q.WeightScore-0.5)*scale.Weight+q.TraitAverage*scale.Traits)*q.SpecialMultiplier, 4)
		if scale.Curve == "size_growth" {
			if scale.Inverse || (name != "extraMaterialQuantity" && name != "quantityLimitBonusRate") || scale.Integer != (name == "extraMaterialQuantity") || math.Abs(scale.Length+scale.Weight-1) > 1e-9 || scale.Traits > 0.2 || scale.MaxBeforeMutation < base || base <= 0 {
				return nil, ErrFishFoodIngredientsInvalid
			}
			score := clampFloat(q.LengthScore*scale.Length+q.WeightScore*scale.Weight, 0, 1)
			growth := scale.MaxBeforeMutation/base - 1
			grown := clampFloat(base*(1+growth*score*score)*(1+q.TraitAverage*scale.Traits), scale.Min, scale.MaxBeforeMutation)
			multiplier = grown / base * q.SpecialMultiplier
		} else if scale.Curve == "bounded_growth" {
			if base <= 0 || scale.Inverse || scale.MaxBeforeMutation < base || math.Abs(scale.Length+scale.Weight-1) > 1e-9 || scale.Traits > .2 {
				return nil, ErrFishFoodIngredientsInvalid
			}
			score := clampFloat(q.LengthScore*scale.Length+q.WeightScore*scale.Weight+q.TraitAverage*scale.Traits, 0, 1)
			multiplier = (base + (scale.MaxBeforeMutation-base)*score*score) / base * q.SpecialMultiplier
		} else if scale.Curve != "" {
			return nil, ErrFishFoodIngredientsInvalid
		}
		if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
			return nil, ErrFishFoodIngredientsInvalid
		}
		q.ParameterMultipliers[name] = roundFloat(multiplier, 4)
		value := base * multiplier
		if scale.Inverse {
			value = base / multiplier
		}
		value = clampFloat(value, scale.Min, scale.Max)
		if scale.Integer {
			value = math.Round(value)
		} else {
			value = roundFloat(value, 4)
		}
		params[name] = value
	}
	return json.Marshal(params)
}

func fishFoodScaledCharges(recipe catalog.FishFoodRecipe, base int64, q *fishFoodQualitySnapshot) int64 {
	var scale *catalog.FishFoodParameterScale
	if recipe.LifetimeScaling != nil {
		scale = recipe.LifetimeScaling.Charges
	}
	value := fishFoodScaledLifetime(base, q.ExtraCharges, scale, q)
	q.ExtraCharges = value - base
	return value
}

func fishFoodScaledDuration(recipe catalog.FishFoodRecipe, base int64, q *fishFoodQualitySnapshot) int64 {
	var scale *catalog.FishFoodParameterScale
	if recipe.LifetimeScaling != nil {
		scale = recipe.LifetimeScaling.DurationSeconds
	}
	value := fishFoodScaledLifetime(base, q.ExtraDurationSeconds, scale, q)
	q.ExtraDurationSeconds = value - base
	return value
}

func fishFoodScaledLifetime(base, extra int64, scale *catalog.FishFoodParameterScale, q *fishFoodQualitySnapshot) int64 {
	if scale == nil {
		return base + extra
	}
	score := clampFloat(q.LengthScore*scale.Length+q.WeightScore*scale.Weight+q.TraitAverage*scale.Traits, 0, 1)
	grown := float64(base) + (scale.MaxBeforeMutation-float64(base))*score*score
	return int64(clampFloat(math.Floor(grown*q.SpecialMultiplier)+float64(extra), scale.Min, scale.Max))
}
