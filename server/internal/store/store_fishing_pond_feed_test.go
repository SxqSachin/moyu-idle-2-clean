package store

import (
	"encoding/json"
	"math"
	"mysecondwebgame/server/internal/catalog"
	"path/filepath"
	"testing"
	"time"
)

func pondFeedTestStore() *Store {
	return &Store{gameData: catalog.Data{
		Items: []catalog.Item{
			{ID: "basic", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal"}},
			{ID: "special", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal", RulesetVersion: pondFeedRulesetVersion, Role: "functional", ConsumePriority: 10, MaxPortionsPerFishHour: .5, Effects: []catalog.PondFeedEffect{{Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: .01}}}},
		},
		FishingSpecies:     []catalog.FishingSpecies{{ID: "test", MinLengthCM: 10, MaxLengthCM: 100, MaturityLengthCM: 30, ShapeFactor: 1}},
		FishingPondSpecies: []catalog.FishingPondSpecies{{SpeciesID: "test", DietClass: "herbal", MaturityAgeDays: 100, LifespanDays: 10000}},
	}}
}

func TestPondFeedFunctionalConsumptionMatchesDoseAndFallsBack(t *testing.T) {
	s := pondFeedTestStore()
	stock := map[string]float64{"basic": 100, "special": 100}
	result := s.consumeFishingPondNutritionForFish(stock, FishInstanceData{Health: .5}, &catalog.FishingPondSpecies{DietClass: "herbal"}, 0, 25)
	if stock["special"] != 95 || stock["basic"] != 80 || result.Effective != 25 || result.Doses[0].Portions != .5 {
		t.Fatalf("wasted functional feed: stock=%v result=%+v", stock, result)
	}
}

func TestPondFeedDoseUsesPhysicalPortionsRegardlessOfDiet(t *testing.T) {
	s := pondFeedTestStore()
	stock := map[string]float64{"special": 100}
	result := s.consumeFishingPondNutritionForFish(stock, FishInstanceData{Health: .5}, &catalog.FishingPondSpecies{DietClass: "protein"}, -1, 100)
	if result.Doses[0].Portions != .5 || stock["special"] != 95 {
		t.Fatalf("dose changed with absorption: %+v stock=%v", result, stock)
	}
	if math.Abs(result.Effective-1.4) > 1e-9 {
		t.Fatal(result.Effective)
	}
}

func TestPondFeedAllocationIsProportionalAndConservesStock(t *testing.T) {
	s := pondFeedTestStore()
	profile := &catalog.FishingPondSpecies{DietClass: "herbal"}
	stock := map[string]float64{"basic": 20, "special": 5}
	fish := FishInstanceData{Health: .5}
	candidates := s.pondFeedCandidates(fish, profile, stock, nil, 0)
	results := allocatePondFeed([]pondFeedDemand{{fish, 30, candidates}, {fish, 30, candidates}}, stock, 1)
	for _, result := range results {
		if result.Effective != 12.5 || result.Doses[0].Portions != .25 {
			t.Fatalf("unfair allocation: %+v", results)
		}
	}
	if stock["basic"] != 0 || stock["special"] != 0 {
		t.Fatalf("stock not conserved: %v", stock)
	}
}

func TestPondFeedPausedAndIneligibleStockIsNotConsumed(t *testing.T) {
	s := pondFeedTestStore()
	stock := map[string]float64{"basic": 20, "special": 20}
	cs := s.pondFeedCandidates(FishInstanceData{}, &catalog.FishingPondSpecies{DietClass: "herbal"}, stock, map[string]bool{"special": true}, 0)
	result := allocatePondFeed([]pondFeedDemand{{FishInstanceData{}, 30, cs}}, stock, 1)
	if stock["special"] != 20 || result[0].Effective != 20 {
		t.Fatal(stock, result)
	}
}

func TestPondFeedCompoundEffectsAreOrderIndependent(t *testing.T) {
	s := pondFeedTestStore()
	left := FishInstanceData{SpeciesID: "test", AgeDays: 50, Health: .6, Satiety: 50, Condition: 1, LengthCM: 20, WeightKG: .08}
	right := left
	effects := []catalog.PondFeedEffect{{Kind: "stat_delta", Stat: "lengthCm", Mode: "bonus_band", ValuePerPortion: .05, MaxTargetMultiplier: 1.04}, {Kind: "stat_delta", Stat: "weightKg", Mode: "bonus_band", ValuePerPortion: .05, MaxTargetMultiplier: 1.07}}
	a := &catalog.PondFeedConfig{RulesetVersion: pondFeedRulesetVersion, Effects: effects}
	b := &catalog.PondFeedConfig{RulesetVersion: pondFeedRulesetVersion, Effects: []catalog.PondFeedEffect{effects[1], effects[0]}}
	s.applyFishingPondFeedEffects(&left, s.gameData.FishingSpeciesByID("test"), s.gameData.FishingPondSpeciesByID("test"), []fishingPondFeedDose{{Feed: a, Portions: .5}}, 1)
	s.applyFishingPondFeedEffects(&right, s.gameData.FishingSpeciesByID("test"), s.gameData.FishingPondSpeciesByID("test"), []fishingPondFeedDose{{Feed: b, Portions: .5}}, 1)
	if left.LengthCM != right.LengthCM || left.WeightKG != right.WeightKG {
		t.Fatalf("order-dependent: %+v %+v", left, right)
	}
}

func TestPondFeedHealthFloorCannotGrantFreeBenefitsOrHeal(t *testing.T) {
	floor := .1
	feed := &catalog.PondFeedConfig{RulesetVersion: pondFeedRulesetVersion, Effects: []catalog.PondFeedEffect{{Kind: "stat_delta", Stat: "satiety", Mode: "flat", ValuePerPortion: 3}, {Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: -.01, Floor: &floor}}}
	s := pondFeedTestStore()
	for _, health := range []float64{.05, .1, .102} {
		fish := FishInstanceData{Health: health, Satiety: 50}
		dose := pondFeedPayableDose(fish, feed, .5)
		s.applyFishingPondFeedEffects(&fish, s.gameData.FishingSpeciesByID("test"), s.gameData.FishingPondSpeciesByID("test"), []fishingPondFeedDose{{Feed: feed, Portions: .5}}, 1)
		if math.Abs(fish.Satiety-(50+3*dose)) > 1e-9 || fish.Health > health || health <= .1 && dose != 0 {
			t.Fatalf("unpaid benefit: health=%v dose=%v fish=%+v", health, dose, fish)
		}
	}
}

func TestPondFeedStockUpgradePreservesFractionsAndPause(t *testing.T) {
	s := pondFeedTestStore()
	stock, paused, err := s.decodePondFeedStock([]byte(`{"basic":15,"special":2.5}`))
	if err != nil {
		t.Fatal(err)
	}
	paused["special"] = true
	raw, err := s.encodePondFeedStock(stock, paused)
	if err != nil {
		t.Fatal(err)
	}
	var doc pondFeedStockDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != 2 || doc.Feeds["basic"].Portions != 1.5 || doc.Feeds["special"].Portions != .25 {
		t.Fatal(string(raw))
	}
	s.gameData.Items[0].PondFeed.Nutrition = 20
	stock, paused, err = s.decodePondFeedStock(raw)
	if err != nil || stock["basic"] != 30 || !paused["special"] {
		t.Fatal(stock, paused, err)
	}
}

func TestPondFeedStockRejectsMalformedAndUnknownVersions(t *testing.T) {
	s := pondFeedTestStore()
	for _, raw := range []string{`null`, `{"version":2}`, `{"version":3}`, `{"basic":-1}`, `{"version":2,"feeds":{"missing":{"portions":1}}}`} {
		if _, _, err := s.decodePondFeedStock([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}

func TestPondFeedStapleAllocationWaitsForFunctionalPhase(t *testing.T) {
	s := pondFeedTestStore()
	profile := &catalog.FishingPondSpecies{DietClass: "herbal"}
	stock := map[string]float64{"basic": 9, "special": 5}
	fish := FishInstanceData{Health: .5}
	all := s.pondFeedCandidates(fish, profile, stock, nil, 0)
	staples := s.pondFeedCandidates(fish, profile, stock, map[string]bool{"special": true}, 0)
	results := allocatePondFeed([]pondFeedDemand{{fish, 10, all}, {fish, 10, staples}}, stock, 1)
	if results[0].Effective != 8 || results[1].Effective != 6 {
		t.Fatalf("staples consumed before functional fallback: %+v", results)
	}
}

func TestPondFeedExtraLengthSurvivesNormalGrowth(t *testing.T) {
	s := pondFeedTestStore()
	row := fishingPondFishRow{InstanceID: 1, Fish: FishInstanceData{SpeciesID: "test", AgeDays: 10, Health: .7, Condition: 1, LengthCM: 25, WeightKG: .05, Satiety: 80, LifeState: "alive", LongevityLimitDays: 10000, Genes: FishGenes{Vitality: .1}}}
	s.settleFishingPondFishStep(&row, map[string]float64{"basic": 100}, 7, time.Unix(1800000000, 0).UTC(), 1)
	if row.Fish.LengthCM < 25 || row.Fish.WeightKG <= .05 {
		t.Fatalf("normal growth erased length bonus: %+v", row.Fish)
	}
}

func TestPondFeedFractionalBonusCannotShrinkOnSmallNaturalGain(t *testing.T) {
	s := pondFeedTestStore()
	row := fishingPondFishRow{InstanceID: 1, Fish: FishInstanceData{SpeciesID: "test", AgeDays: 10, Health: .7, Condition: 1, LengthCM: 25, WeightKG: .1564, Satiety: 80, LifeState: "alive", LongevityLimitDays: 10000, Genes: FishGenes{Vitality: .1}}}
	// The natural gain is smaller than the stored rounding precision.
	profile, species := s.gameData.FishingPondSpeciesByID("test"), s.gameData.FishingSpeciesByID("test")
	row.Fish.LengthCM = math.Cbrt(.1565 * 100000)
	preview := row.Fish
	now := time.Unix(1800000000, 0).UTC()
	maintenance, _, _, _ := s.preparePondFeedDemand(&preview, 1, 7, now, 1, species, profile)
	s.settleFishingPondFishWithFeed(&row, 7, now, 1, fishingPondNutritionResult{Effective: maintenance + .000001*fishingPondNutritionPerKG})
	if row.Fish.WeightKG < .1564 {
		t.Fatalf("rounding erased fractional bonus: %v", row.Fish.WeightKG)
	}
}

func TestPondFeedAuthoredRecipesSevenDaySettlementInvariants(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{gameData: data}
	now := time.Unix(1800000000, 0).UTC()
	for _, item := range data.Items {
		if item.PondFeed == nil || item.PondFeed.Delivery == "targeted" || item.PondFeed.RulesetVersion != pondFeedRulesetVersion {
			continue
		}
		t.Run(item.ID, func(t *testing.T) {
			stock := map[string]float64{item.ID: 1000000}
			for _, staple := range data.Items {
				if staple.PondFeed != nil && pondFeedRole(staple.PondFeed) == "staple" && staple.ID != item.ID {
					stock[staple.ID] = 1000000
				}
			}
			var rows []fishingPondFishRow
			for _, profile := range data.FishingPondSpecies {
				species := data.FishingSpeciesByID(profile.SpeciesID)
				if species == nil {
					continue
				}
				length := species.MaturityLengthCM
				rows = append(rows, fishingPondFishRow{InstanceID: int64(len(rows) + 1), Fish: FishInstanceData{SpeciesID: species.ID, AgeDays: profile.MaturityAgeDays, Health: .7, Condition: 1, LengthCM: length, WeightKG: species.ShapeFactor * math.Pow(length, 3) / 100000, Satiety: 50, LifeState: "alive", LongevityLimitDays: 10000, Genes: FishGenes{Vitality: .1}}})
			}
			for hour := 1; hour <= 7*24; hour++ {
				before := stock[item.ID]
				s.settleFishingPondFeedingStep(rows, stock, nil, 7, now.Add(time.Duration(hour)*time.Hour), 1)
				used := (before - stock[item.ID]) / item.PondFeed.Nutrition
				if used < 0 || pondFeedRole(item.PondFeed) == "functional" && used > float64(len(rows))*pondFeedDoseLimit(item.PondFeed)+1e-8 {
					t.Fatalf("hour %d invalid consumption %v", hour, used)
				}
				for _, row := range rows {
					fish := row.Fish
					if !fishingPondFishSizeValid(fish, data.FishingSpeciesByID(fish.SpeciesID)) || math.IsNaN(fish.Health) || fish.Health < 0 || fish.Health > 1 || math.IsNaN(fish.Satiety) || fish.Satiety < 0 || fish.Satiety > 100 {
						t.Fatalf("hour %d invalid fish: %+v", hour, fish)
					}
				}
				if hour == 24 || hour == 7*24 {
					t.Logf("%dh: %d species, %.3f portions consumed", hour, len(rows), (1000000-stock[item.ID])/item.PondFeed.Nutrition)
				}
			}
		})
	}
}

func TestPondFeedFullSettlementKeepsIdenticalFishEqual(t *testing.T) {
	s := pondFeedTestStore()
	now := time.Unix(1800000000, 0).UTC()
	base := FishInstanceData{SpeciesID: "test", AgeDays: 50, Health: .7, Condition: 1, LengthCM: 20, WeightKG: .08, Satiety: 80, LifeState: "alive", LongevityLimitDays: 10000}
	rows := []fishingPondFishRow{{InstanceID: 1, Fish: base}, {InstanceID: 999, Fish: base}}
	for i := range rows {
		rows[i].Fish.Genes = FishGenes{Vitality: .1}
	}
	stock := map[string]float64{"basic": 1, "special": .2}
	s.settleFishingPondFeedingStep(rows, stock, nil, 7, now, 1)
	if rows[0].Fish.WeightKG != rows[1].Fish.WeightKG || rows[0].Fish.Satiety != rows[1].Fish.Satiety || rows[0].Fish.Health != rows[1].Fish.Health {
		t.Fatalf("ID bias: %+v", rows)
	}
}
