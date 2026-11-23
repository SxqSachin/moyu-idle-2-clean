package store

import "mysecondwebgame/server/internal/catalog"

func BuildActionSpecMap(gameData catalog.Data) map[string]ActionSpec {
	specs := make(map[string]ActionSpec)
	for _, def := range gameData.ActionDefinitions() {
		itemReward := make(map[string]ActionItemRewardSpec, len(def.ItemReward))
		for itemID, reward := range def.ItemReward {
			var rewardRange *ActionItemRewardRange
			if reward.Range != nil {
				rewardRange = &ActionItemRewardRange{
					Min: reward.Range.Min,
					Max: reward.Range.Max,
				}
			}
			itemReward[itemID] = ActionItemRewardSpec{
				Count:                     reward.Count,
				Range:                     rewardRange,
				Percent:                   reward.Percent,
				GrantPolicy:               reward.GrantPolicy,
				DefaultAffixRarityWeights: reward.DefaultAffixRarityWeights,
			}
		}
		specs[def.ID] = ActionSpec{
			ID:                           def.ID,
			MainType:                     def.MainType,
			SubType:                      def.SubType,
			Menu:                         def.Menu,
			DurationS:                    def.DurationS,
			Costs:                        def.Costs,
			MaterialInstanceRequirements: materialInstanceRequirementsFromCatalog(def.MaterialInstanceRequirements),
			StatusRequirement:            def.StatusRequirement,
			BuildingRequirement:          def.BuildingRequirement,
			ItemReward:                   itemReward,
			MainOutput:                   def.MainOutput,
			StatusExpReward:              def.StatusExpReward,
		}
	}
	if _, ok := specs["equipment_enhance"]; !ok {
		specs["equipment_enhance"] = ActionSpec{
			ID:                  "equipment_enhance",
			MainType:            "equipment",
			Menu:                "equipment",
			DurationS:           1,
			Costs:               map[string]int64{},
			StatusRequirement:   map[string]int64{},
			BuildingRequirement: map[string]int64{},
			ItemReward:          map[string]ActionItemRewardSpec{},
			StatusExpReward:     map[string]float64{},
		}
	}
	if _, ok := specs["building_upgrade"]; !ok {
		specs["building_upgrade"] = ActionSpec{
			ID:                  "building_upgrade",
			MainType:            "building",
			SubType:             "building",
			Menu:                "building",
			DurationS:           1,
			Costs:               map[string]int64{},
			StatusRequirement:   map[string]int64{},
			BuildingRequirement: map[string]int64{},
			ItemReward:          map[string]ActionItemRewardSpec{},
			StatusExpReward:     map[string]float64{},
		}
	}
	if _, ok := specs[FishingTripActionID]; !ok {
		specs[FishingTripActionID] = ActionSpec{
			ID:                  FishingTripActionID,
			MainType:            "fishing",
			SubType:             "fishing_trip",
			Menu:                "fishing",
			DurationS:           30,
			Costs:               map[string]int64{},
			StatusRequirement:   map[string]int64{},
			BuildingRequirement: map[string]int64{},
			ItemReward:          map[string]ActionItemRewardSpec{},
			StatusExpReward:     map[string]float64{},
		}
	}
	return specs
}

func materialInstanceRequirementsFromCatalog(raw map[string]catalog.ActionMaterialInstanceRequirement) map[string]ActionMaterialInstanceRequirement {
	out := make(map[string]ActionMaterialInstanceRequirement, len(raw))
	for itemID, requirement := range raw {
		out[itemID] = ActionMaterialInstanceRequirement{
			EnhanceLevel:           requirement.EnhanceLevel,
			RequireTemplateAffixes: requirement.RequireTemplateAffixes,
			Durability:             requirement.Durability,
		}
	}
	return out
}
