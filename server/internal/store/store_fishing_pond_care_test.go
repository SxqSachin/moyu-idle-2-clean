package store

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

func pondCareTestFeed() *catalog.PondFeedConfig {
	return &catalog.PondFeedConfig{Delivery: "targeted", Role: "functional", RulesetVersion: pondFeedRulesetVersion, Effects: []catalog.PondFeedEffect{
		{Kind: "stat_delta", Stat: "longevityDays", Mode: "flat", ValuePerPortion: 1095},
		{Kind: "stat_delta", Stat: "naturalProtectionDays", Mode: "flat", ValuePerPortion: 1095},
	}}
}
func pondCareTestFish() FishInstanceData {
	return FishInstanceData{SpeciesID: "test", LifeState: "alive", AgeDays: 500, BioAgeFraction: .5, LongevityLimitDays: 501, Health: .7, Satiety: 100, Condition: 1, LengthCM: 20, WeightKG: .08, Genes: FishGenes{Vitality: .1}}
}

func TestPondCareWholePortionRepeatableAndRevisionGuard(t *testing.T) {
	s, feed := pondFeedTestStore(), pondCareTestFeed()
	fish := pondCareTestFish()
	original := fish
	for i := int64(0); i < 100; i++ {
		if err := s.applyFishingPondTargetedFeed(&fish, feed, i); err != nil {
			t.Fatal(err)
		}
	}
	if fish.LongevityLimitDays != original.LongevityLimitDays+109500 || fish.NaturalProtectionUntilAgeDays != 500.5+109500 || fish.AgeDays != original.AgeDays || fish.BioAgeFraction != original.BioAgeFraction || !reflect.DeepEqual(fish.Genes, original.Genes) || fish.Health != original.Health || fish.LengthCM != original.LengthCM {
		t.Fatalf("invalid extension: %+v", fish)
	}
	before := fish
	if err := s.applyFishingPondTargetedFeed(&fish, feed, 99); err == nil || !reflect.DeepEqual(before, fish) {
		t.Fatal("stale retry mutated fish")
	}
	raw, err := json.Marshal(fish)
	if err != nil {
		t.Fatal(err)
	}
	var loaded FishInstanceData
	if err := json.Unmarshal(raw, &loaded); err != nil || !reflect.DeepEqual(loaded, fish) {
		t.Fatal("care lost on persistence", err)
	}
	view := s.fishingPondFishView(fishingPondFishRow{Fish: fish})
	if view.CareRevision != 100 || math.Abs(view.NaturalProtectionHours-7200) > 1e-9 || view.Fish.LongevityLimitDays != 0 || view.Fish.NaturalProtectionUntilAgeDays != 0 {
		t.Fatalf("invalid owner projection: %+v", view)
	}
}

func TestPondCareProtectsNaturalDeathButNotStarvation(t *testing.T) {
	s := pondFeedTestStore()
	fish := pondCareTestFish()
	if err := s.applyFishingPondTargetedFeed(&fish, pondCareTestFeed(), 0); err != nil {
		t.Fatal(err)
	}
	profile := s.gameData.FishingPondSpeciesByID("test")
	if fishingPondDeathChance(fish, profile, 1) != 0 {
		t.Fatal("protected fish has random death risk")
	}
	fish.Health = 0
	fish.Satiety = 0
	fish.StarvationHours = 100
	row := fishingPondFishRow{InstanceID: 1, Fish: fish}
	s.settleFishingPondFishStep(&row, map[string]float64{}, 7, time.Unix(1800000000, 0), 1)
	if row.Fish.LifeState != "dead" || row.Fish.DeathCause != "starvation" {
		t.Fatal("protection granted invulnerability")
	}
	before := row.Fish
	if s.applyFishingPondTargetedFeed(&row.Fish, pondCareTestFeed(), 1) == nil || !reflect.DeepEqual(row.Fish, before) {
		t.Fatal("dead fish revived")
	}
}

func TestPondCareExpirationOnlyChargesUnprotectedFraction(t *testing.T) {
	s := pondFeedTestStore()
	fish := pondCareTestFish()
	fish.NaturalProtectionUntilAgeDays = 500.5 - fishingPondBioDaysPerRealDay/24*.25
	chance := fishingPondDeathChance(fish, s.gameData.FishingPondSpeciesByID("test"), 1)
	fish.NaturalProtectionUntilAgeDays = 0
	base := fishingPondDeathChance(fish, s.gameData.FishingPondSpeciesByID("test"), 1)
	if math.Abs(chance-base*.25) > 1e-10 {
		t.Fatalf("expiration risk=%v base=%v", chance, base)
	}
}

func TestPondCareDoesNotEnterSharedAllocation(t *testing.T) {
	s := pondFeedTestStore()
	feed := pondCareTestFeed()
	feed.Nutrition = 10
	s.gameData.Items = append(s.gameData.Items, catalog.Item{ID: "care", PondFeed: feed})
	stock := map[string]float64{"care": 100, "basic": 100}
	result := s.consumeFishingPondNutritionForFish(stock, pondCareTestFish(), s.gameData.FishingPondSpeciesByID("test"), 0, 50)
	if stock["care"] != 100 || result.Effective != 50 {
		t.Fatal(stock, result)
	}
}

func TestPondCareSevenDaysWithRenewalNeverDiesNaturally(t *testing.T) {
	s := pondFeedTestStore()
	fish := pondCareTestFish()
	fish.LongevityLimitDays = 1
	rows := []fishingPondFishRow{{InstanceID: 1, Fish: fish}}
	now := time.Unix(1800000000, 0)
	for hour := 0; hour < 168; hour++ {
		if hour%48 == 0 {
			if err := s.applyFishingPondTargetedFeed(&rows[0].Fish, pondCareTestFeed(), rows[0].Fish.PondCareRevision); err != nil {
				t.Fatal(err)
			}
		}
		s.settleFishingPondFeedingStep(rows, map[string]float64{"basic": 1000000}, nil, 7, now.Add(time.Duration(hour+1)*time.Hour), 1)
		if rows[0].Fish.LifeState != "alive" {
			t.Fatalf("died with active renewal at hour %d", hour)
		}
	}
}
