package store

import (
	"math"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func TestRollWildFishingMutationsAllowsIndependentStacking(t *testing.T) {
	store := &Store{config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{
		{ID: "prismatic", AppearanceChance: 1},
		{ID: "rainbow", AppearanceChance: 1},
		{ID: "shimmering", AppearanceChance: 0},
	}}}}
	got := store.rollWildFishingMutations("catch:42")
	want := []string{"prismatic", "rainbow"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollWildFishingMutations()=%v, want %v", got, want)
	}
}

func TestRollFishingMutationsIsStableWhenCatalogExpands(t *testing.T) {
	base := []config.FishingMutationSettings{
		{ID: "prismatic", AppearanceChance: 0.5},
		{ID: "rainbow", AppearanceChance: 0.5},
	}
	extended := append([]config.FishingMutationSettings{{ID: "new_trait", AppearanceChance: 0.5}}, base...)
	baseResult := fishMutationSet(rollFishingMutations(base, "stable-catch", 1))
	extendedResult := fishMutationSet(rollFishingMutations(extended, "stable-catch", 1))
	for _, id := range []string{"prismatic", "rainbow"} {
		_, baseHas := baseResult[id]
		_, extendedHas := extendedResult[id]
		if baseHas != extendedHas {
			t.Fatalf("%s outcome changed after catalog expansion: base=%v extended=%v", id, baseHas, extendedHas)
		}
	}
}

func TestRollFishingMutationsAppliesFrozenChanceMultiplier(t *testing.T) {
	seed := "baited-catch"
	roll := pondUnit(seed + ":mutation:special:appearance")
	if roll == 0 {
		t.Fatal("test seed unexpectedly produced a zero mutation roll")
	}
	mutations := []config.FishingMutationSettings{{ID: "special", AppearanceChance: roll / 2}}
	if got := rollFishingMutations(mutations, seed, 1); len(got) != 0 {
		t.Fatalf("base roll=%v, want no mutation", got)
	}
	if got := rollFishingMutations(mutations, seed, 4); !reflect.DeepEqual(got, []string{"special"}) {
		t.Fatalf("boosted roll=%v, want special mutation", got)
	}
	if got, want := rollFishingMutations(mutations, seed, 0), rollFishingMutations(mutations, seed, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy zero multiplier=%v, want base behavior %v", got, want)
	}
}

func TestInheritFishingMutationsOnlyRollsTraitsCarriedByAParent(t *testing.T) {
	store := &Store{config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{
		{ID: "inherited", AppearanceChance: 0, InheritanceChance: 1},
		{ID: "spontaneous", AppearanceChance: 1, InheritanceChance: 0},
		{ID: "missed", AppearanceChance: 0, InheritanceChance: 0},
	}}}}
	female := FishInstanceData{Genes: FishGenes{Mutations: []string{"inherited", "missed"}}}
	got := store.inheritFishingMutations(female, FishInstanceData{}, "fry", 1)
	want := []string{"inherited"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inheritFishingMutations()=%v, want %v", got, want)
	}
}

func TestInheritFishingMutationsReducesScatterChance(t *testing.T) {
	seed := "scatter-fry"
	roll := pondUnit(seed + ":mutation:inherited:inherit")
	if roll <= 0.1 || roll >= 0.9 {
		t.Fatalf("test seed produced unsuitable mutation roll %v", roll)
	}
	inheritanceChance := roll + 0.05
	store := &Store{config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{{
		ID: "inherited", InheritanceChance: inheritanceChance,
	}}}}}
	parent := FishInstanceData{Genes: FishGenes{Mutations: []string{"inherited"}}}
	if got := store.inheritFishingMutations(parent, FishInstanceData{}, seed, 1); !reflect.DeepEqual(got, []string{"inherited"}) {
		t.Fatalf("designated-pair inheritance=%v, want inherited trait", got)
	}
	if got := store.inheritFishingMutations(parent, FishInstanceData{}, seed, pondScatterMutationInheritance); len(got) != 0 {
		t.Fatalf("scatter inheritance=%v, want reduced chance to miss trait", got)
	}
}

func TestFishingMutationMultipliersStackAdditively(t *testing.T) {
	store := &Store{config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{
		{ID: "prismatic", LifespanMultiplier: 1.8, SaleValueMultiplier: 2, Alchemy: config.FishingMutationAlchemySettings{PrimaryValueMultiplier: 1.5}},
		{ID: "rainbow", LifespanMultiplier: 1.6, SaleValueMultiplier: 2.5, Alchemy: config.FishingMutationAlchemySettings{PrimaryValueMultiplier: 1.35}},
	}}}}
	ids := []string{"rainbow", "prismatic", "rainbow", "unknown"}
	if got := store.fishingMutationLifespanMultiplier(ids); math.Abs(got-2.4) > 0.000001 {
		t.Fatalf("lifespan multiplier=%v, want 2.4", got)
	}
	if got := store.fishingMutationPrimaryValueMultiplier(ids); math.Abs(got-1.85) > 0.000001 {
		t.Fatalf("alchemy multiplier=%v, want 1.85", got)
	}
	if got := store.fishingMutationSaleValueMultiplier(ids); math.Abs(got-3.5) > 0.000001 {
		t.Fatalf("sale multiplier=%v, want 3.5", got)
	}
}

func TestFishingMutationAlchemyBonusesAggregateByAttribute(t *testing.T) {
	store := &Store{config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{
		{ID: "prismatic", Alchemy: config.FishingMutationAlchemySettings{BonusAttributes: []config.FishingMutationAlchemyBonusAttribute{{AttrKey: "alchemy_precision", Min: 0.01, Max: 0.02}}}},
		{ID: "rainbow", Alchemy: config.FishingMutationAlchemySettings{BonusAttributes: []config.FishingMutationAlchemyBonusAttribute{{AttrKey: "alchemy_precision", Min: 0.03, Max: 0.05}, {AttrKey: "luck", Min: 1, Max: 2}}}},
	}}}}
	bonuses := store.fishingMutationAlchemyBonuses([]string{"rainbow", "prismatic"})
	if len(bonuses) != 2 {
		t.Fatalf("bonus attributes=%d, want 2: %+v", len(bonuses), bonuses)
	}
	if bonuses[0].Attribute != "alchemy_precision" || math.Abs(bonuses[0].Range.Min-0.04) > 0.000001 || math.Abs(bonuses[0].Range.Max-0.07) > 0.000001 || !reflect.DeepEqual(bonuses[0].MutationIDs, []string{"prismatic", "rainbow"}) {
		t.Fatalf("unexpected aggregated precision bonus: %+v", bonuses[0])
	}
	if bonuses[1].Attribute != "luck" || bonuses[1].Range.Min != 1 || bonuses[1].Range.Max != 2 {
		t.Fatalf("unexpected luck bonus: %+v", bonuses[1])
	}
}

func TestInitializeFishingPondFishPreservesMutationGenesAndExtendsLife(t *testing.T) {
	profile := &catalog.FishingPondSpecies{SpeciesID: "common_carp", MaturityAgeDays: 365, LifespanDays: 7300}
	baseStore := &Store{}
	mutatedStore := &Store{config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{{ID: "prismatic", LifespanMultiplier: 2}}}}}
	now := time.Unix(1_800_000_000, 0).UTC()
	base := FishInstanceData{SpeciesID: "common_carp", Health: 1, Condition: 1}
	mutated := FishInstanceData{SpeciesID: "common_carp", Health: 1, Condition: 1, Genes: FishGenes{Mutations: []string{"prismatic"}}}
	baseStore.initializeFishingPondFish(&base, 42, 88, now, profile)
	mutatedStore.initializeFishingPondFish(&mutated, 42, 88, now, profile)
	if !reflect.DeepEqual(mutated.Genes.Mutations, []string{"prismatic"}) {
		t.Fatalf("mutation genes were lost during initialization: %+v", mutated.Genes)
	}
	if math.Abs(mutated.LongevityLimitDays-base.LongevityLimitDays*2) > 0.000001 {
		t.Fatalf("mutated longevity=%v, want %v", mutated.LongevityLimitDays, base.LongevityLimitDays*2)
	}
}
