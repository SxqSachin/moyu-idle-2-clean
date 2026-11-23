package store

import (
	"strings"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

type fishingMutationAlchemyBonus struct {
	Attribute   string
	MutationIDs []string
	Range       catalog.AlchemyValueRange
}

func (s *Store) FishingMutationCatalog() []config.FishingMutationSettings {
	if s == nil || len(s.config.Fishing.Mutations) == 0 {
		return []config.FishingMutationSettings{}
	}
	out := make([]config.FishingMutationSettings, len(s.config.Fishing.Mutations))
	for index, mutation := range s.config.Fishing.Mutations {
		out[index] = mutation
		out[index].Alchemy.BonusAttributes = append([]config.FishingMutationAlchemyBonusAttribute{}, mutation.Alchemy.BonusAttributes...)
	}
	return out
}

func (s *Store) rollWildFishingMutations(seed string) []string {
	if s == nil {
		return nil
	}
	return rollFishingMutations(s.config.Fishing.Mutations, seed, 1)
}

func rollFishingMutations(mutations []config.FishingMutationSettings, seed string, chanceMultiplier float64) []string {
	if strings.TrimSpace(seed) == "" {
		return nil
	}
	if chanceMultiplier <= 0 {
		chanceMultiplier = 1
	}
	chanceMultiplier = clampFloat(chanceMultiplier, 0.01, 100)
	out := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		// Key every roll by mutation ID so adding or reordering definitions neither
		// perturbs established mutation outcomes nor consumes the trip's combat RNG.
		chance := clampFloat(mutation.AppearanceChance*chanceMultiplier, 0, 1)
		if chance > 0 && pondUnit(seed+":mutation:"+mutation.ID+":appearance") < chance {
			out = append(out, mutation.ID)
		}
	}
	return out
}

func (s *Store) inheritFishingMutations(female, male FishInstanceData, seed string, chanceMultiplier float64) []string {
	if s == nil {
		return nil
	}
	chanceMultiplier = clampFloat(chanceMultiplier, 0, 1)
	femaleMutations := fishMutationSet(female.Genes.Mutations)
	maleMutations := fishMutationSet(male.Genes.Mutations)
	out := make([]string, 0, len(s.config.Fishing.Mutations))
	for _, mutation := range s.config.Fishing.Mutations {
		_, inheritedFromFemale := femaleMutations[mutation.ID]
		_, inheritedFromMale := maleMutations[mutation.ID]
		if !inheritedFromFemale && !inheritedFromMale {
			continue
		}
		chance := mutation.InheritanceChance * chanceMultiplier
		if chance > 0 && pondUnit(seed+":mutation:"+mutation.ID+":inherit") < chance {
			out = append(out, mutation.ID)
		}
	}
	return out
}

func (s *Store) fishingMutationLifespanMultiplier(ids []string) float64 {
	multiplier := 1.0
	for _, mutation := range s.matchFishingMutations(ids) {
		multiplier += mutation.LifespanMultiplier - 1
	}
	return multiplier
}

func (s *Store) fishingMutationPrimaryValueMultiplier(ids []string) float64 {
	multiplier := 1.0
	for _, mutation := range s.matchFishingMutations(ids) {
		multiplier += mutation.Alchemy.PrimaryValueMultiplier - 1
	}
	return multiplier
}

func (s *Store) fishingMutationSaleValueMultiplier(ids []string) float64 {
	multiplier := 1.0
	for _, mutation := range s.matchFishingMutations(ids) {
		value := mutation.SaleValueMultiplier
		if value <= 0 {
			value = 1
		}
		multiplier += value - 1
	}
	return multiplier
}

func (s *Store) fishingMutationAlchemyBonuses(ids []string) []fishingMutationAlchemyBonus {
	matched := s.matchFishingMutations(ids)
	if len(matched) == 0 {
		return nil
	}
	out := make([]fishingMutationAlchemyBonus, 0, len(matched))
	byAttribute := make(map[string]int)
	for _, mutation := range matched {
		for _, bonus := range mutation.Alchemy.BonusAttributes {
			index, exists := byAttribute[bonus.AttrKey]
			if !exists {
				byAttribute[bonus.AttrKey] = len(out)
				out = append(out, fishingMutationAlchemyBonus{Attribute: bonus.AttrKey})
				index = len(out) - 1
			}
			out[index].MutationIDs = append(out[index].MutationIDs, mutation.ID)
			out[index].Range.Min += bonus.Min
			out[index].Range.Max += bonus.Max
		}
	}
	return out
}

func (s *Store) configuredFishMutationIDs(ids []string) []string {
	matched := s.matchFishingMutations(ids)
	out := make([]string, 0, len(matched))
	for _, mutation := range matched {
		out = append(out, mutation.ID)
	}
	return out
}

func (s *Store) matchFishingMutations(ids []string) []config.FishingMutationSettings {
	if s == nil || len(ids) == 0 || len(s.config.Fishing.Mutations) == 0 {
		return nil
	}
	wanted := fishMutationSet(ids)
	out := make([]config.FishingMutationSettings, 0, len(wanted))
	for _, mutation := range s.config.Fishing.Mutations {
		if _, ok := wanted[mutation.ID]; ok {
			out = append(out, mutation)
		}
	}
	return out
}

func fishMutationSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}
