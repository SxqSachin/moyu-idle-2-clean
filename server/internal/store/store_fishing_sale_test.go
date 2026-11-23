package store

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

func TestFishingSaleRejectsMissingIdempotencyKeyBeforeDatabaseWork(t *testing.T) {
	store := &Store{}
	now := time.Unix(1_800_000_000, 0).UTC()

	if _, err := store.SellFishingCatches(context.Background(), 42, []int64{503}, 100, "", now); !errors.Is(err, ErrFishingSaleInvalid) {
		t.Fatalf("SellFishingCatches() error = %v, want ErrFishingSaleInvalid", err)
	}
}

func TestFishingSaleReferenceSpecimenHasNeutralFactors(t *testing.T) {
	species := catalog.FishingSpecies{MinLengthCM: 20, MaxLengthCM: 100, ShapeFactor: 1.1, BaseValue: 500}
	referenceLength, referenceWeight := fishingSaleReferenceMeasurements(species)
	factors := fishingSaleSpecimenFactors(species, FishInstanceData{
		AgeDays: 930, LengthCM: referenceLength, WeightKG: referenceWeight, Condition: 1, Health: 1,
	})

	for name, value := range map[string]float64{
		"age": factors.AgeFactor, "length": factors.LengthFactor, "weight": factors.WeightFactor,
		"quality": factors.QualityFactor, "specimen": factors.SpecimenMultiplier,
	} {
		if math.Abs(value-1) > 1e-9 {
			t.Fatalf("%s factor=%v, want 1", name, value)
		}
	}
}

func TestFishingSaleSpecimenAndMarketFactorsStayWithinCaps(t *testing.T) {
	species := catalog.FishingSpecies{MinLengthCM: 20, MaxLengthCM: 100, ShapeFactor: 1.1, BaseValue: 500}
	small := fishingSaleSpecimenFactors(species, FishInstanceData{AgeDays: 1, LengthCM: 0.1, WeightKG: 0.001})
	large := fishingSaleSpecimenFactors(species, FishInstanceData{AgeDays: 999_999, LengthCM: 1_000, WeightKG: 1_000, Condition: 2, Health: 2})
	if small.SpecimenMultiplier != fishingSaleSpecimenMinimum {
		t.Fatalf("small specimen multiplier=%v, want %v", small.SpecimenMultiplier, fishingSaleSpecimenMinimum)
	}
	if large.SpecimenMultiplier != fishingSaleSpecimenMaximum {
		t.Fatalf("large specimen multiplier=%v, want %v", large.SpecimenMultiplier, fishingSaleSpecimenMaximum)
	}
	if fishingSaleMarketMultiplier(0.5, 0.5) != fishingSaleMarketMinimum || fishingSaleMarketMultiplier(2, 2) != fishingSaleMarketMaximum {
		t.Fatal("market multiplier did not honor configured caps")
	}
	if fishingSaleMarketMultiplier(0.96, 0.93) != fishingSaleMarketMinimum || fishingSaleMarketMultiplier(1.04, 1.06) != fishingSaleMarketMaximum {
		t.Fatal("configured demand and supply ranges cannot reach both market guards")
	}
}

func TestFishingSaleCultureFactorUsesWeightCapAndCeiling(t *testing.T) {
	species := catalog.FishingSpecies{BaseValue: 3_000}
	cultured := fishingSaleCultureFactor(species, FishInstanceData{Origin: "cultured", WeightKG: 0.04})
	if math.Abs(cultured-0.004) > 1e-9 {
		t.Fatalf("cultured factor=%v, want 0.004", cultured)
	}
	ceiling := fishingSaleCultureFactor(catalog.FishingSpecies{BaseValue: 500}, FishInstanceData{Origin: "cultured", WeightKG: 1})
	if math.Abs(ceiling-0.50) > 1e-9 {
		t.Fatalf("cultured factor ceiling=%v, want 0.50", ceiling)
	}
	if wild := fishingSaleCultureFactor(species, FishInstanceData{Origin: "wild", WeightKG: 0.04}); wild != 1 {
		t.Fatalf("wild factor=%v, want 1", wild)
	}
}

func TestFishingSaleFinalGoldIncludesMutationMultiplier(t *testing.T) {
	if got := fishingSaleFinalGold(1_000, 1.2, 2.5, 1.05, 0.5); got != 1_575 {
		t.Fatalf("fishingSaleFinalGold()=%d, want 1575", got)
	}
	if got := fishingSaleFinalGold(0, 1, 1, 1, 1); got != 1 {
		t.Fatalf("minimum fishingSaleFinalGold()=%d, want 1", got)
	}
}

func TestFishingSaleIneligibilityKeepsCulturedJuvenilesOutOfQuotes(t *testing.T) {
	species := &catalog.FishingSpecies{BaseValue: 370, MinLengthCM: 8, MaxLengthCM: 40, ShapeFactor: 1.2}
	profile := &catalog.FishingPondSpecies{MaturityAgeDays: 600}
	juvenile := FishInstanceData{AgeDays: 255, LengthCM: 9, WeightKG: 0.01, Origin: "cultured", LifeState: "alive"}

	reason, requiredAge := fishingSaleIneligibility(species, profile, juvenile)
	if reason != "cultured_immature" || requiredAge != 600 {
		t.Fatalf("juvenile eligibility=(%q, %d), want (cultured_immature, 600)", reason, requiredAge)
	}

	juvenile.Origin = "wild"
	if reason, _ := fishingSaleIneligibility(species, profile, juvenile); reason != "" {
		t.Fatalf("wild juvenile reason=%q, want eligible", reason)
	}

	juvenile.Origin = "cultured"
	juvenile.AgeDays = 600
	if reason, _ := fishingSaleIneligibility(species, profile, juvenile); reason != "" {
		t.Fatalf("mature cultured fish reason=%q, want eligible", reason)
	}
}
