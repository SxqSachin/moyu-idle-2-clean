package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func TestPublicFishingPondFishViewOnlyExposesPresentationData(t *testing.T) {
	configured := &Store{
		config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{{ID: "prismatic"}}}},
		gameData: catalog.Data{
			FishingSpecies: []catalog.FishingSpecies{{ID: "mirror_carp", MaturityAgeDays: 20}},
			FishingPondSpecies: []catalog.FishingPondSpecies{{
				SpeciesID: "mirror_carp",
				Predation: &catalog.FishingPondPredation{MaxPreyLengthRatio: 0.4},
			}},
		},
	}
	privateTime := time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)
	view := configured.publicFishingPondFishView(81, ItemCustomization{Name: "Silver", LabelColor: "cyan"}, FishInstanceData{
		SpeciesID: "mirror_carp", AgeDays: 10, BioAgeFraction: 0.5, Sex: "female",
		LengthCM: 25.5, WeightKG: 1.2, Condition: 0.9, Health: 0.8,
		WaterID: "hidden_water", SpotID: "hidden_spot", CaughtAt: privateTime,
		RulesetVersion: "private-ruleset", RecordFlags: []string{"private_record"},
		ParentInstanceIDs: []int64{12, 13}, LongevityLimitDays: 9000,
		PreferredFeedHours: 42, StarvationHours: 7, PondEnteredAt: &privateTime,
		BornAt: &privateTime, LastBredAt: &privateTime, DeathAt: &privateTime,
		DeathCause: "private_cause", Genes: FishGenes{GrowthRate: 0.7, Mutations: []string{"prismatic", "unconfigured"}},
		GenomeRevealed: true, PondZone: fishingPondZoneNursery, LifeState: "alive", Satiety: 55,
	})

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal public fish: %v", err)
	}
	payload := string(raw)
	for _, forbidden := range []string{
		"waterId", "spotId", "caughtAt", "rulesetVersion", "recordFlags",
		"parentInstanceIds", "longevityLimitDays", "preferredFeedHours",
		"starvationHours", "pondEnteredAt", "bornAt", "lastBredAt",
		"deathAt", "deathCause", "genes", "genomeRevealed", "condition", "health", "satiety",
	} {
		if strings.Contains(payload, `"`+forbidden+`"`) {
			t.Fatalf("public fish leaked %q: %s", forbidden, payload)
		}
	}
	if view.Mature || view.MaturityProgress != 0.525 {
		t.Fatalf("unexpected public maturity: mature=%v progress=%v", view.Mature, view.MaturityProgress)
	}
	if view.Fish.Sex != "female" {
		t.Fatalf("visible fish traits were lost: %+v", view.Fish)
	}
	if view.Fish.PondZone != fishingPondZoneNursery {
		t.Fatalf("public pond zone=%q, want nursery", view.Fish.PondZone)
	}
	if view.Predation != nil {
		t.Fatalf("nursery fish exposed an inactive predation rule: %+v", view.Predation)
	}
	if view.CustomData.Name != "Silver" || view.CustomData.LabelColor != "cyan" {
		t.Fatalf("public fish rename was lost: %+v", view.CustomData)
	}
	if len(view.Fish.Mutations) != 1 || view.Fish.Mutations[0] != "prismatic" {
		t.Fatalf("visible mutation traits were lost: %+v", view.Fish)
	}
}

func TestPublicFishingPondCapabilitiesExcludeMutations(t *testing.T) {
	allowed := map[string]bool{
		FishingPondCapabilityFishInspect:     true,
		FishingPondCapabilityFishTrack:       true,
		FishingPondCapabilityWaterThrowStone: true,
		FishingPondCapabilityWaterSkipStone:  true,
	}
	for _, capability := range publicFishingPondCapabilities {
		if !allowed[capability] {
			t.Fatalf("unexpected public capability %q", capability)
		}
		if strings.Contains(capability, "feed") || strings.Contains(capability, "remove") || strings.Contains(capability, "write") {
			t.Fatalf("mutation capability exposed publicly: %q", capability)
		}
	}
	if len(publicFishingPondCapabilities) != len(allowed) {
		t.Fatalf("public capability list=%v, want exactly %v", publicFishingPondCapabilities, allowed)
	}
}

func TestPublicFishingPondFishViewClampsSpeciesSizeCaps(t *testing.T) {
	species := catalog.FishingSpecies{ID: "mirror_carp", MaxLengthCM: 40, ShapeFactor: 1.1}
	configured := &Store{gameData: catalog.Data{FishingSpecies: []catalog.FishingSpecies{species}}}
	view := configured.publicFishingPondFishView(81, ItemCustomization{}, FishInstanceData{
		SpeciesID: species.ID, LengthCM: 400, WeightKG: 400, LifeState: "alive",
	})
	if view.Fish.LengthCM != species.MaxLengthCM || view.Fish.WeightKG != fishingPondMaximumWeight(&species) {
		t.Fatalf("public pond leaked an oversize fish: %+v", view.Fish)
	}
}
