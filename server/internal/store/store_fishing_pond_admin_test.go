package store

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func newAdminFishingPondValidationStore() *Store {
	return &Store{
		config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{
			{ID: "prismatic"}, {ID: "rainbow"}, {ID: "shimmering"},
		}}},
		gameData: catalog.Data{
			FishingSpecies: []catalog.FishingSpecies{{
				ID: "crucian_carp", WaterID: "willow_creek", MinLengthCM: 8, MaxLengthCM: 42,
				MaturityLengthCM: 21, MaturityAgeDays: 730, LifespanDays: 3650, ShapeFactor: 1.08,
			}},
			FishingPondSpecies: []catalog.FishingPondSpecies{{
				ID: "pond_crucian_carp", SpeciesID: "crucian_carp", MaturityAgeDays: 730, LifespanDays: 3650,
			}},
		},
	}
}

func TestValidateAdminFishingPondFishInputDefaultsAndOrdersMutations(t *testing.T) {
	s := newAdminFishingPondValidationStore()
	species, _, mutations, customization, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{
		UserID: 7, Mutations: []string{"shimmering", "prismatic", "shimmering"}, Name: "Starlight",
	})
	if err != nil {
		t.Fatalf("validate input failed: %v", err)
	}
	if species.ID != "crucian_carp" {
		t.Fatalf("default species=%s, want crucian_carp", species.ID)
	}
	if !reflect.DeepEqual(mutations, []string{"prismatic", "shimmering"}) {
		t.Fatalf("mutations=%v, want config order", mutations)
	}
	if customization.Name != "Starlight" || customization.LabelColor != "gold" {
		t.Fatalf("customization=%+v, want default gold label", customization)
	}
}

func TestValidateAdminFishingPondFishInputRejectsUnknownMutationAndOutOfRangeGene(t *testing.T) {
	s := newAdminFishingPondValidationStore()
	if _, _, _, _, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{UserID: 7, Mutations: []string{"unknown"}}); !errors.Is(err, ErrAdminFishingPondFishInvalid) {
		t.Fatalf("unknown mutation error=%v", err)
	}
	badGene := 1.1
	if _, _, _, _, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{UserID: 7, Genes: &AdminFishingPondGenesInput{Vitality: &badGene}}); !errors.Is(err, ErrAdminFishingPondFishInvalid) {
		t.Fatalf("out-of-range gene error=%v", err)
	}
}

func TestValidateAdminFishingPondFishInputRejectsIncoherentLivingState(t *testing.T) {
	s := newAdminFishingPondValidationStore()
	zeroHealth := 0.0
	if _, _, _, _, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{UserID: 7, Health: &zeroHealth}); !errors.Is(err, ErrAdminFishingPondFishInvalid) {
		t.Fatalf("zero health error=%v", err)
	}
	ageDays := 730
	longevityLimitDays := 730.5
	bioAgeFraction := 0.5
	if _, _, _, _, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{
		UserID: 7, AgeDays: &ageDays, BioAgeFraction: &bioAgeFraction, LongevityLimitDays: &longevityLimitDays,
	}); !errors.Is(err, ErrAdminFishingPondFishInvalid) {
		t.Fatalf("expired longevity error=%v", err)
	}
}

func TestValidateAdminFishingPondFishInputRejectsSpeciesSizeOverflow(t *testing.T) {
	s := newAdminFishingPondValidationStore()
	species := s.gameData.FishingSpeciesByID("crucian_carp")
	overLength := species.MaxLengthCM + 0.1
	if _, _, _, _, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{UserID: 7, LengthCM: &overLength}); !errors.Is(err, ErrAdminFishingPondFishInvalid) {
		t.Fatalf("oversize length error=%v", err)
	}
	overWeight := fishingPondMaximumWeight(species) + 0.001
	if _, _, _, _, err := s.validateAdminFishingPondFishInput(AdminFishingPondFishInput{UserID: 7, WeightKG: &overWeight}); !errors.Is(err, ErrAdminFishingPondFishInvalid) {
		t.Fatalf("oversize weight error=%v", err)
	}
}

func TestBuildAdminFishingPondFishUsesMatureHealthyDefaults(t *testing.T) {
	species := &catalog.FishingSpecies{
		ID: "crucian_carp", WaterID: "willow_creek", MaturityLengthCM: 21, MaturityAgeDays: 730, ShapeFactor: 1.08,
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	fish := buildAdminFishingPondFish(AdminFishingPondFishInput{}, species, []string{"prismatic"}, now)
	wantWeight := roundFloat(species.ShapeFactor*math.Pow(species.MaturityLengthCM, 3)/100_000, 3)
	if fish.AgeDays != species.MaturityAgeDays || fish.LengthCM != species.MaturityLengthCM || fish.WeightKG != wantWeight {
		t.Fatalf("default fish measurements=%+v, want mature length and calculated weight=%v", fish, wantWeight)
	}
	if fish.Health != 1 || fish.Condition != 1 || fish.Satiety != 100 || fish.Origin != "cultured" || fish.LifeState != "alive" {
		t.Fatalf("default fish state=%+v", fish)
	}
	if !reflect.DeepEqual(fish.Genes.Mutations, []string{"prismatic"}) || fish.PondEnteredAt == nil || !fish.PondEnteredAt.Equal(now) {
		t.Fatalf("default fish mutation or pond time=%+v", fish)
	}
}
