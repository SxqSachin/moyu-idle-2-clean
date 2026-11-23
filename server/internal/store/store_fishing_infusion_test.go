package store

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestFishingInfusionQueueArgsStayLocked(t *testing.T) {
	executor := newAlchemyExecutor(nil, "alchemy.bench")
	tests := []struct {
		name string
		raw  string
	}{
		{name: "legacy single fish", raw: `{"carrierInstanceId":601,"fishInstanceId":603}`},
		{name: "multi fish preview payload", raw: `{"carrierInstanceId":601,"fishInstanceIds":[603,605,606,607]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(test.raw)); err != ErrFishingInfusionUnavailable {
				t.Fatalf("ValidateAndNormalizeArgs() error = %v, want ErrFishingInfusionUnavailable", err)
			}
		})
	}
}

func TestFishingInfusionCarpPowerAnchors(t *testing.T) {
	species := catalog.FishingSpecies{Rarity: "uncommon", ShapeFactor: 1.55}
	tests := []struct {
		name     string
		lengthCM float64
		weightKG float64
		want     float64
	}{
		{name: "mature thirty-five centimeter carp", lengthCM: 35, weightKG: 0.665, want: 87.495},
		{name: "large one-meter carp", lengthCM: 100, weightKG: 15.5, want: 151.5},
		{name: "impossible weight is capped by body shape", lengthCM: 35, weightKG: 10, want: 87.892425},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := fishingInfusionPower(species, test.lengthCM, test.weightKG)
			if math.Abs(got-test.want) > 0.000001 {
				t.Fatalf("fishingInfusionPower() = %.3f, want %.3f", got, test.want)
			}
		})
	}
	if got := fishingInfusionPower(species, 2_000, 100); got != 200 {
		t.Fatalf("uncommon rarity cap = %.3f, want 200", got)
	}
}

func TestFishingInfusionCatalogCoversEverySpecies(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	if len(data.FishingSpecies) != 24 || len(data.FishingInfusionProfiles) != 24 {
		t.Fatalf("fishing catalog counts = species:%d profiles:%d, want 24/24", len(data.FishingSpecies), len(data.FishingInfusionProfiles))
	}
	for _, species := range data.FishingSpecies {
		profile := data.FishingInfusionProfileBySpeciesID(species.ID)
		if profile == nil || profile.AttrKey == "" || profile.UnitScale <= 0 || (profile.TraitKind != "combat" && profile.TraitKind != "life") {
			t.Fatalf("species %s has invalid infusion profile: %+v", species.ID, profile)
		}
	}
}

func TestFishingRodEnhancementTotalsAndReelLock(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	cache, err := buildEnhanceRuntimeCache(data)
	if err != nil {
		t.Fatalf("build enhance cache: %v", err)
	}
	assertTotals := func(templateID string, level int, control, sensitivity, power float64) {
		t.Helper()
		runtime, ok := cache.Template(templateID)
		if !ok {
			t.Fatalf("missing enhance runtime for %s", templateID)
		}
		got := runtime.CumulativeByLevel[level]
		if math.Abs(got["fishing_control"]-control) > 0.000001 || math.Abs(got["fishing_sensitivity"]-sensitivity) > 0.000001 || math.Abs(got["fishing_power"]-power) > 0.000001 {
			t.Fatalf("%s +%d totals = %+v, want control=%.2f sensitivity=%.2f power=%.2f", templateID, level, got, control, sensitivity, power)
		}
	}
	assertTotals("basic_fishing_rod", 15, 0.75, 0.30, 0.15)
	assertTotals("champion_current_rod", 20, 1.00, 0.40, 0.20)
	assertTotals("tournament_current_rod", 20, 1.00, 0.40, 0.20)
	for _, templateID := range []string{"basic_fishing_rod", "champion_current_rod", "tournament_current_rod"} {
		template := data.EquipmentTemplateByID(templateID)
		if template == nil || len(template.EnhanceAffixMilestonePools) != 0 {
			t.Fatalf("%s must not add milestone affixes on top of its exact enhancement totals", templateID)
		}
	}
	if reel, ok := cache.Template("basic_spinning_reel"); !ok || reel.MaxLevel != 0 {
		t.Fatalf("basic reel enhancement must stay disabled: %+v", reel)
	}
}

func TestFishingRodsDoNotExposeAffixSlots(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	for _, templateID := range []string{
		"basic_fishing_rod",
		"silverleaf_float_rod",
		"silkwood_search_rod",
		"rimebone_deep_rod",
		"moonwood_precision_rod",
		"champion_current_rod",
		"tournament_current_rod",
	} {
		template := data.EquipmentTemplateByID(templateID)
		if template == nil {
			t.Fatalf("missing fishing rod template %s", templateID)
		}
		if template.AffixSlotCount != 0 || len(template.AffixAllowedIDs) != 0 || len(template.EnhanceAffixMilestonePools) != 0 {
			t.Fatalf("%s must not expose affixes: slots=%d allowed=%v milestones=%v", templateID, template.AffixSlotCount, template.AffixAllowedIDs, template.EnhanceAffixMilestonePools)
		}
	}
}
