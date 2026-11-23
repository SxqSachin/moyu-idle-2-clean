package store

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

func TestInitializeFishingPondFishIsDeterministicAndBounded(t *testing.T) {
	profile := &catalog.FishingPondSpecies{SpeciesID: "common_carp", LifespanDays: 7300}
	now := time.Unix(1_800_000_000, 0).UTC()
	left := FishInstanceData{SpeciesID: "common_carp", Health: 1, Condition: 1}
	right := left
	store := &Store{}
	store.initializeFishingPondFish(&left, 42, 88, now, profile)
	store.initializeFishingPondFish(&right, 42, 88, now, profile)
	if !reflect.DeepEqual(left.Genes, right.Genes) || left.LongevityLimitDays != right.LongevityLimitDays {
		t.Fatalf("pond initialization is not deterministic: left=%+v right=%+v", left, right)
	}
	for name, value := range map[string]float64{
		"size": left.Genes.SizePotential, "growth": left.Genes.GrowthRate,
		"feed": left.Genes.FeedEfficiency, "vitality": left.Genes.Vitality,
		"lifespan": left.Genes.Lifespan, "fertility": left.Genes.Fertility,
	} {
		if value < -1 || value > 1 {
			t.Fatalf("%s gene=%v outside normalized range", name, value)
		}
	}
	if left.LongevityLimitDays < float64(profile.LifespanDays)*0.7 || left.LongevityLimitDays > float64(profile.LifespanDays)*1.8 {
		t.Fatalf("longevity=%v outside supported tail", left.LongevityLimitDays)
	}
	if left.Satiety != 100 || left.LifeState != "alive" {
		t.Fatalf("unexpected initial welfare: satiety=%v life=%q", left.Satiety, left.LifeState)
	}
	if left.PondZone != fishingPondZoneMain {
		t.Fatalf("legacy pond fish zone=%q, want main", left.PondZone)
	}
	if left.Sex != "female" && left.Sex != "male" {
		t.Fatalf("legacy fish did not receive a deterministic sex: %q", left.Sex)
	}
	if left.Sex != normalizedPondSex("", 88, left.SpeciesID) {
		t.Fatalf("legacy fish sex differs between picker and stocking: %q", left.Sex)
	}
}

func TestInitializeFishingPondFishDoesNotExpireOlderCatchOnEntry(t *testing.T) {
	profile := &catalog.FishingPondSpecies{SpeciesID: "snakehead", MaturityAgeDays: 730, LifespanDays: 2920}
	fish := FishInstanceData{SpeciesID: "snakehead", AgeDays: 3650, Health: 1, Condition: 1}
	(&Store{}).initializeFishingPondFish(&fish, 42, 99, time.Unix(1_800_000_000, 0).UTC(), profile)
	minimumGraceDays := math.Max(365, float64(profile.MaturityAgeDays)*0.5)
	if fish.LongevityLimitDays < float64(fish.AgeDays)+minimumGraceDays {
		t.Fatalf("older catch expires on pond entry: age=%d longevity=%v", fish.AgeDays, fish.LongevityLimitDays)
	}
}

func TestFishingPondDeathChanceRisesWithAge(t *testing.T) {
	profile := &catalog.FishingPondSpecies{LifespanDays: 3650}
	base := FishInstanceData{Health: 1, LongevityLimitDays: 3650}
	young := base
	young.AgeDays = 1000
	old := base
	old.AgeDays = 3650
	veryOld := base
	veryOld.AgeDays = 5400
	youngChance := fishingPondDeathChance(young, profile, 24)
	oldChance := fishingPondDeathChance(old, profile, 24)
	veryOldChance := fishingPondDeathChance(veryOld, profile, 24)
	if !(youngChance < oldChance && oldChance < veryOldChance) {
		t.Fatalf("mortality must rise with age: young=%v old=%v veryOld=%v", youngChance, oldChance, veryOldChance)
	}
	if veryOldChance > 0.5+1e-9 {
		t.Fatalf("daily mortality=%v exceeds cap", veryOldChance)
	}
}

func TestFishingPondSettlementWindowCoversExactTimeAndOfflineCap(t *testing.T) {
	last := time.Date(2026, time.September, 2, 8, 17, 0, 0, time.UTC)
	start, settledAt, hours := fishingPondSettlementWindow(last, last.Add(59*time.Minute))
	if hours != 0 || !start.Equal(last) || !settledAt.Equal(last.Add(59*time.Minute)) {
		t.Fatalf("partial-hour settlement drifted: start=%v settled=%v hours=%d", start, settledAt, hours)
	}
	if remainder := fishingPondSettlementRemainder(start, settledAt, hours); remainder != 59*time.Minute {
		t.Fatalf("partial-hour remainder=%v, want 59m", remainder)
	}
	start, settledAt, hours = fishingPondSettlementWindow(last, last.Add(2*time.Hour+45*time.Minute))
	if hours != 2 || !start.Equal(last) || !settledAt.Equal(last.Add(2*time.Hour+45*time.Minute)) {
		t.Fatalf("whole-hour settlement drifted: start=%v settled=%v hours=%d", start, settledAt, hours)
	}
	if remainder := fishingPondSettlementRemainder(start, settledAt, hours); remainder != 45*time.Minute {
		t.Fatalf("mixed settlement remainder=%v, want 45m", remainder)
	}
	now := last.Add(40 * 24 * time.Hour)
	start, settledAt, hours = fishingPondSettlementWindow(last, now)
	if hours != fishingPondMaxOfflineHours || !start.Equal(now.Add(-fishingPondMaxOfflineHours*time.Hour)) || !settledAt.Equal(now) {
		t.Fatalf("offline cap window invalid: start=%v settled=%v hours=%d", start, settledAt, hours)
	}
	if remainder := fishingPondSettlementRemainder(start, settledAt, hours); remainder != 0 {
		t.Fatalf("capped settlement remainder=%v, want 0", remainder)
	}
}

func TestFishingPondSettlementWindowIsMonotonicAndIdempotent(t *testing.T) {
	last := time.Date(2026, time.September, 2, 8, 17, 0, 0, time.UTC)
	firstNow := last.Add(37 * time.Minute)
	firstStart, firstSettledAt, firstHours := fishingPondSettlementWindow(last, firstNow)
	secondNow := firstNow.Add(35 * time.Minute)
	secondStart, secondSettledAt, secondHours := fishingPondSettlementWindow(firstSettledAt, secondNow)

	processed := firstSettledAt.Sub(firstStart) + secondSettledAt.Sub(secondStart)
	if processed != 72*time.Minute || !secondSettledAt.Equal(secondNow) || firstHours != 0 || secondHours != 0 {
		t.Fatalf("split settlement lost time: processed=%v first=%v second=%v", processed, firstSettledAt, secondSettledAt)
	}
	repeatStart, repeatSettledAt, repeatHours := fishingPondSettlementWindow(secondSettledAt, secondNow)
	if repeatHours != 0 || !repeatStart.Equal(secondNow) || !repeatSettledAt.Equal(secondNow) {
		t.Fatalf("same-time settlement is not idempotent: start=%v settled=%v hours=%d", repeatStart, repeatSettledAt, repeatHours)
	}
}

func TestSettleFishingPondFishStepAdvancesFractionalTime(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		Items:              []catalog.Item{{ID: "pond_feed", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal"}}},
		FishingSpecies:     []catalog.FishingSpecies{{ID: "test_fish", MinLengthCM: 10, MaxLengthCM: 100, ShapeFactor: 1}},
		FishingPondSpecies: []catalog.FishingPondSpecies{{SpeciesID: "test_fish", DietClass: "herbal", MaturityAgeDays: 30, LifespanDays: 0}},
	}}
	row := fishingPondFishRow{InstanceID: 11, Fish: FishInstanceData{
		SpeciesID: "test_fish", LengthCM: 10, WeightKG: 0.1, Condition: 1, Health: 1,
		LifeState: "alive", Satiety: 100, Genes: FishGenes{FeedEfficiency: 0.2},
	}}
	stock := map[string]float64{"pond_feed": 10}
	now := time.Date(2026, time.September, 2, 8, 47, 0, 0, time.UTC)
	store.settleFishingPondFishStep(&row, stock, 7, now, 0.5)

	expectedBioDays := fishingPondBioDaysPerRealDay * 0.5 / 24
	expectedWholeDays := int(math.Floor(expectedBioDays))
	expectedFraction := expectedBioDays - float64(expectedWholeDays)
	if row.Fish.AgeDays != expectedWholeDays || math.Abs(row.Fish.BioAgeFraction-expectedFraction) > 1e-9 {
		t.Fatalf("fractional step age=%d+%v, want %d+%v", row.Fish.AgeDays, row.Fish.BioAgeFraction, expectedWholeDays, expectedFraction)
	}
	if stock["pond_feed"] >= 10 {
		t.Fatalf("fractional step did not consume feed: stock=%v", stock["pond_feed"])
	}
	if row.Fish.PondEnteredAt == nil || !row.Fish.PondEnteredAt.Equal(now) {
		t.Fatalf("fractional step did not initialize pond entry time: %v", row.Fish.PondEnteredAt)
	}
}

func TestFishingPondTimeScaleKeepsMaturityInsidePlayerTime(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	for _, species := range data.FishingSpecies {
		realHours := float64(species.MaturityAgeDays) / fishingPondBioDaysPerRealDay * 24
		if realHours < 24 || realHours > 192 {
			t.Fatalf("%s maturity age %d maps to %.1f real hours, want 1-8 days", species.ID, species.MaturityAgeDays, realHours)
		}
	}
}

func TestPondPhenotypeEffectsExposePlayerFacingValues(t *testing.T) {
	effects := pondPhenotypeEffects(FishGenes{SizePotential: 0.5, FeedEfficiency: 0.4, GrowthRate: 0.1})
	if len(effects) != 2 {
		t.Fatalf("effects=%+v, want two visible phenotype effects", effects)
	}
	byKey := map[string]float64{}
	for _, effect := range effects {
		byKey[effect.Key] = effect.Value
	}
	if math.Abs(byKey["large_frame"]-0.05) > 1e-9 || math.Abs(byKey["efficient_feeder"]-0.08) > 1e-9 {
		t.Fatalf("phenotype effect values=%+v", effects)
	}
}

func TestLargeFramePhenotypeMatchesSameAgeTargetLength(t *testing.T) {
	species := &catalog.FishingSpecies{MinLengthCM: 10, MaxLengthCM: 100, MaturityLengthCM: 35}
	profile := &catalog.FishingPondSpecies{MaturityAgeDays: 730, LifespanDays: 7300}
	baseline := fishingPondTargetLength(FishInstanceData{AgeDays: 500}, species, profile)
	largeFrame := fishingPondTargetLength(FishInstanceData{AgeDays: 500, Genes: FishGenes{SizePotential: 0.5}}, species, profile)
	if math.Abs(largeFrame/baseline-1.05) > 0.000001 {
		t.Fatalf("large-frame target ratio=%v, want 1.05", largeFrame/baseline)
	}
}

func TestPondFeedConditionsPreventIneligibleFishFromEating(t *testing.T) {
	feed := &catalog.PondFeedConfig{
		Nutrition: 14, FeedClass: "herbal", RulesetVersion: pondFeedRulesetVersion,
		Conditions: []catalog.PondFeedCondition{{Field: "dietClass", Op: "eq", Value: "herbal"}},
	}
	store := &Store{gameData: catalog.Data{Items: []catalog.Item{{ID: "selective", PondFeed: feed}}}}
	stock := map[string]float64{"selective": 14}
	result := store.consumeFishingPondNutritionForFish(stock, FishInstanceData{Health: 1}, &catalog.FishingPondSpecies{DietClass: "protein"}, 0, 5)
	if result.Effective != 0 || stock["selective"] != 14 {
		t.Fatalf("ineligible fish consumed selective feed: result=%+v stock=%v", result, stock)
	}
	result = store.consumeFishingPondNutritionForFish(stock, FishInstanceData{Health: 1}, &catalog.FishingPondSpecies{DietClass: "herbal"}, 0, 5)
	if result.Effective <= 0 || stock["selective"] >= 14 {
		t.Fatalf("eligible herbivore did not consume selective feed: result=%+v stock=%v", result, stock)
	}
}

func TestPondFeedConditionsSupportNumericThresholdsAndFailClosed(t *testing.T) {
	profile := &catalog.FishingPondSpecies{DietClass: "protein"}
	feed := &catalog.PondFeedConfig{
		RulesetVersion: pondFeedRulesetVersion,
		Conditions: []catalog.PondFeedCondition{
			{Field: "health", Op: "gte", Value: 0.35},
			{Field: "satiety", Op: "lt", Value: 80.0},
		},
	}
	if !pondFeedConditionsMatch(FishInstanceData{Health: 0.35, Satiety: 79.9}, profile, feed) {
		t.Fatal("valid numeric pond feed conditions did not match")
	}
	if pondFeedConditionsMatch(FishInstanceData{Health: 0.35, Satiety: 80}, profile, feed) {
		t.Fatal("exclusive numeric pond feed threshold matched its boundary")
	}
	feed.Conditions = append(feed.Conditions, catalog.PondFeedCondition{Field: "unsupported", Op: "eq", Value: 1.0})
	if pondFeedConditionsMatch(FishInstanceData{Health: 1, Satiety: 1}, profile, feed) {
		t.Fatal("unsupported pond feed condition must fail closed")
	}
}

func TestPondFeedPriorityPrecedesDietEfficiency(t *testing.T) {
	store := &Store{gameData: catalog.Data{Items: []catalog.Item{
		{ID: "ordinary", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal"}},
		{ID: "special", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "protein", RulesetVersion: pondFeedRulesetVersion, ConsumePriority: 60}},
	}}}
	stock := map[string]float64{"ordinary": 10, "special": 10}
	result := store.consumeFishingPondNutritionForFish(stock, FishInstanceData{}, &catalog.FishingPondSpecies{DietClass: "herbal"}, 0, 1)
	if result.Effective < 0.999999 || stock["ordinary"] != 10 || stock["special"] >= 10 {
		t.Fatalf("priority did not win before efficiency: result=%+v stock=%v", result, stock)
	}
}

func TestPondFeedEffectsUseEffectivePortionHourlyCap(t *testing.T) {
	ceiling := 1.0
	feed := &catalog.PondFeedConfig{
		RulesetVersion: pondFeedRulesetVersion,
		Effects:        []catalog.PondFeedEffect{{Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: 0.0025, Ceiling: &ceiling}},
	}
	store := &Store{}
	fish := FishInstanceData{SpeciesID: "test", Health: 0.5, Condition: 1, LengthCM: 20, WeightKG: 0.08}
	species := &catalog.FishingSpecies{ID: "test", MinLengthCM: 10, MaxLengthCM: 100, ShapeFactor: 1}
	profile := &catalog.FishingPondSpecies{SpeciesID: "test", DietClass: "herbal", MaturityAgeDays: 100, LifespanDays: 1000}
	store.applyFishingPondFeedEffects(&fish, species, profile, []fishingPondFeedDose{{Feed: feed, Portions: 50}}, 1)
	want := 0.5 + 0.0025*fishingPondMaxEffectPortionsHour
	if math.Abs(fish.Health-want) > 0.0000001 {
		t.Fatalf("health=%v, want capped effect %v", fish.Health, want)
	}
}

func TestPondFeedEffectsShareHourlyCapAcrossFeedTypes(t *testing.T) {
	feedA := &catalog.PondFeedConfig{RulesetVersion: pondFeedRulesetVersion, Effects: []catalog.PondFeedEffect{{Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: 0.02}}}
	feedB := &catalog.PondFeedConfig{RulesetVersion: pondFeedRulesetVersion, Effects: []catalog.PondFeedEffect{{Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: 0.02}}}
	store := &Store{}
	fish := FishInstanceData{Health: 0.5}
	store.applyFishingPondFeedEffects(
		&fish,
		&catalog.FishingSpecies{MaxLengthCM: 100, ShapeFactor: 1},
		&catalog.FishingPondSpecies{},
		[]fishingPondFeedDose{{Feed: feedA, Portions: 0.4}, {Feed: feedB, Portions: 0.4}},
		1,
	)
	want := 0.5 + 0.02*fishingPondMaxEffectPortionsHour
	if math.Abs(fish.Health-want) > 0.0000001 {
		t.Fatalf("mixed feed effects bypassed shared dose cap: health=%v want=%v", fish.Health, want)
	}
}

func TestPondFeedEffectsComposeGrowthRecoveryAndTradeoffs(t *testing.T) {
	healthCeiling := 1.0
	healthFloor := 0.1
	feed := &catalog.PondFeedConfig{
		RulesetVersion: pondFeedRulesetVersion,
		Effects: []catalog.PondFeedEffect{
			{Kind: "stat_delta", Stat: "lengthCm", Mode: "bonus_band", ValuePerPortion: 0.05, MaxTargetMultiplier: 1.04},
			{Kind: "stat_delta", Stat: "weightKg", Mode: "bonus_band", ValuePerPortion: 0.05, MaxTargetMultiplier: 1.07},
			{Kind: "stat_delta", Stat: "satiety", Mode: "flat", ValuePerPortion: 3},
			{Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: 0.01, Ceiling: &healthCeiling},
			{Kind: "stat_delta", Stat: "health", Mode: "flat", ValuePerPortion: -0.008, Floor: &healthFloor},
		},
	}
	store := &Store{}
	fish := FishInstanceData{SpeciesID: "test", AgeDays: 50, Health: 0.5, Satiety: 40, Condition: 1, LengthCM: 15, WeightKG: 0.03}
	species := &catalog.FishingSpecies{ID: "test", MinLengthCM: 10, MaturityLengthCM: 30, MaxLengthCM: 100, ShapeFactor: 1}
	profile := &catalog.FishingPondSpecies{SpeciesID: "test", DietClass: "protein", MaturityAgeDays: 100, LifespanDays: 1000}
	store.applyFishingPondFeedEffects(&fish, species, profile, []fishingPondFeedDose{{Feed: feed, Portions: 1}}, 1)
	if fish.LengthCM <= 15 || fish.WeightKG <= 0.03 {
		t.Fatalf("growth effects were not composed: %+v", fish)
	}
	if math.Abs(fish.Satiety-41.5) > 0.0000001 || math.Abs(fish.Health-0.501) > 0.0000001 {
		t.Fatalf("flat effects did not use the shared half-portion dose: %+v", fish)
	}
	if fish.LengthCM > species.MaxLengthCM || fish.WeightKG > fishingPondMaximumWeight(species) {
		t.Fatalf("composed effects exceeded species hard caps: %+v", fish)
	}
}

func TestFishingPondSizeClampEnforcesSpeciesHardCaps(t *testing.T) {
	store := &Store{}
	species := &catalog.FishingSpecies{ID: "test", MaxLengthCM: 42, ShapeFactor: 1.08}
	fish := FishInstanceData{LengthCM: 420, WeightKG: 1000}
	store.clampFishingPondFishSize(&fish, species)
	if fish.LengthCM != species.MaxLengthCM || fish.WeightKG != fishingPondMaximumWeight(species) {
		t.Fatalf("hard cap failed: fish=%+v maxWeight=%v", fish, fishingPondMaximumWeight(species))
	}
	if !fishingPondFishSizeValid(fish, species) {
		t.Fatalf("clamped fish should satisfy size invariant: %+v", fish)
	}
}

func TestGeneratedPondFeedDSLAndRecipeContracts(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	wanted := map[string][]string{
		"pond_feed_selective_herbal": {},
		"pond_feed_length_growth":    {"lengthCm"},
		"pond_feed_weight_growth":    {"weightKg"},
		"pond_feed_health_recovery":  {"health"},
		"pond_feed_satiety_risk":     {"satiety", "health"},
		"pond_feed_forced_growth":    {"lengthCm", "weightKg", "health"},
	}
	for itemID, effectStats := range wanted {
		item := data.ItemByID(itemID)
		if item == nil || item.PondFeed == nil || item.PondFeed.RulesetVersion != pondFeedRulesetVersion {
			t.Fatalf("%s missing pond feed DSL", itemID)
		}
		if len(item.PondFeed.Effects) != len(effectStats) {
			t.Fatalf("%s effects=%+v, want stats %v", itemID, item.PondFeed.Effects, effectStats)
		}
		for index, stat := range effectStats {
			if item.PondFeed.Effects[index].Stat != stat {
				t.Fatalf("%s effect %d stat=%q, want %q", itemID, index, item.PondFeed.Effects[index].Stat, stat)
			}
		}
	}
	selective := data.ItemByID("pond_feed_selective_herbal").PondFeed
	if len(selective.Conditions) != 1 || selective.Conditions[0].Field != "dietClass" || selective.Conditions[0].Op != "eq" || selective.Conditions[0].Value != "herbal" {
		t.Fatalf("selective herbal feed condition=%+v", selective.Conditions)
	}
	healing := data.ItemByID("pond_feed_health_recovery").PondFeed.Effects[0]
	risky := data.ItemByID("pond_feed_satiety_risk").PondFeed.Effects
	forced := data.ItemByID("pond_feed_forced_growth").PondFeed.Effects
	if healing.ValuePerPortion <= 0 || risky[0].ValuePerPortion <= 0 || risky[1].ValuePerPortion >= 0 || forced[2].ValuePerPortion >= 0 {
		t.Fatalf("authored feed tradeoffs have incorrect signs: healing=%+v risky=%+v forced=%+v", healing, risky, forced)
	}
	for _, action := range data.Actions {
		output := data.ItemByID(action.MainOutput)
		if output == nil || output.PondFeed == nil {
			continue
		}
		if _, gated := action.BuildingRequirement[FishingPondBuildingID]; gated {
			t.Fatalf("ordinary fish feed recipe %s still requires the pond building", action.ID)
		}
		reward := action.ItemReward[action.MainOutput]
		if reward.Count == nil || *reward.Count <= 0 {
			t.Fatalf("%s has invalid feed output=%+v", action.ID, reward)
		}
		feedInputs := 0
		inputValue := 0.0
		hasLootInput := false
		for ingredientID, quantity := range action.Costs {
			if quantity < *reward.Count {
				t.Fatalf("%s output exceeds ingredient %s quantity", action.ID, ingredientID)
			}
			if ingredient := data.ItemByID(ingredientID); ingredient != nil {
				inputValue += float64(quantity) * ingredient.EconomyBaseValue
				if ingredient.PondFeed != nil {
					feedInputs++
				}
				if ingredient.Category == "loot" {
					hasLootInput = true
				}
				for _, itemType := range ingredient.Type {
					if itemType == "loot" {
						hasLootInput = true
					}
				}
			}
		}
		if feedInputs > 1 {
			t.Fatalf("%s nests %d finished fish feeds", action.ID, feedInputs)
		}
		outputValue := float64(*reward.Count) * output.EconomyBaseValue
		if outputValue <= 0 || inputValue < outputValue*1.2 {
			t.Fatalf("%s input value %.2f is below 120%% of output value %.2f", action.ID, inputValue, outputValue)
		}
		if _, isNew := wanted[action.MainOutput]; !isNew {
			continue
		}
		if !hasLootInput {
			t.Fatalf("%s new feed recipe has no loot ingredient", action.ID)
		}
		if *reward.Count != 10 {
			t.Fatalf("%s output=%+v, want 10", action.ID, reward)
		}
		delete(wanted, action.MainOutput)
	}
	if len(wanted) != 0 {
		t.Fatalf("new fish feeds missing recipes: %v", wanted)
	}
	for actionID, level := range map[string]int64{"cook_pond_broodstock_mash": 3, "cook_pond_lineage_reagent": 2} {
		var found *catalog.ActionDefinition
		for index := range data.Actions {
			if data.Actions[index].ID == actionID {
				found = &data.Actions[index]
				break
			}
		}
		if found == nil || found.BuildingRequirement[FishingPondBuildingID] != level {
			t.Fatalf("exception recipe %s lost pond level %d", actionID, level)
		}
	}
}

func TestFishingPondNurseryStopsGrowthAtMaturity(t *testing.T) {
	species := &catalog.FishingSpecies{MinLengthCM: 10, MaxLengthCM: 100, MaturityLengthCM: 35}
	profile := &catalog.FishingPondSpecies{MaturityAgeDays: 100, LifespanDays: 1000}
	mainTarget := fishingPondTargetLength(FishInstanceData{AgeDays: 500, PondZone: fishingPondZoneMain}, species, profile)
	nurseryTarget := fishingPondTargetLength(FishInstanceData{AgeDays: 500, PondZone: fishingPondZoneNursery}, species, profile)
	maturityTarget := fishingPondTargetLength(FishInstanceData{AgeDays: 100, PondZone: fishingPondZoneMain}, species, profile)
	if mainTarget <= nurseryTarget || math.Abs(nurseryTarget-maturityTarget) > 0.000001 {
		t.Fatalf("nursery target=%v maturity=%v main adult=%v", nurseryTarget, maturityTarget, mainTarget)
	}
}

func TestSettleFishingPondFishStepHandlesStarvationAndDeath(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		FishingSpecies:     []catalog.FishingSpecies{{ID: "test_fish", MinLengthCM: 10, MaxLengthCM: 100, ShapeFactor: 1}},
		FishingPondSpecies: []catalog.FishingPondSpecies{{SpeciesID: "test_fish", DietClass: "herbal", LifespanDays: 0}},
	}}
	row := fishingPondFishRow{InstanceID: 12, Fish: FishInstanceData{
		SpeciesID: "test_fish", LengthCM: 10, WeightKG: 0.1, Condition: 1, Health: 1,
		LifeState: "alive", Satiety: 10, Genes: FishGenes{Vitality: 0.2},
	}}
	now := time.Date(2026, time.September, 2, 14, 0, 0, 0, time.UTC)
	store.settleFishingPondFishStep(&row, map[string]float64{}, 7, now, 6)
	if row.Fish.Satiety != 0 || row.Fish.StarvationHours != 6 || !(row.Fish.Condition < 1) {
		t.Fatalf("feed exhaustion did not advance starvation: satiety=%v starvation=%v condition=%v", row.Fish.Satiety, row.Fish.StarvationHours, row.Fish.Condition)
	}

	row.Fish.Health = 0
	deathAt := now.Add(15 * time.Minute)
	store.settleFishingPondFishStep(&row, map[string]float64{}, 7, deathAt, 0.25)
	if row.Fish.LifeState != "dead" || row.Fish.DeathCause != "starvation" || row.Fish.DeathAt == nil || !row.Fish.DeathAt.Equal(deathAt) {
		t.Fatalf("fractional death settlement invalid: %+v", row.Fish)
	}
}

func TestSettleFishingPondPredationConsumesSmallestEligiblePrey(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		FishingSpecies: []catalog.FishingSpecies{
			{ID: "predator", MaxLengthCM: 100, ShapeFactor: 1},
			{ID: "prey", MaxLengthCM: 40, ShapeFactor: 1},
		},
		FishingPondSpecies: []catalog.FishingPondSpecies{
			{SpeciesID: "predator", Predation: &catalog.FishingPondPredation{MaxPreyLengthRatio: 0.45, HuntBelowSatiety: 70, DailyHuntChance: 1}},
			{SpeciesID: "prey"},
		},
	}}
	rows := []fishingPondFishRow{
		{InstanceID: 10, CustomData: ItemCustomization{Name: "Hunter"}, Fish: FishInstanceData{SpeciesID: "predator", LengthCM: 50, WeightKG: 1.25, Condition: 1, Health: 1, Satiety: 0, LifeState: "alive"}},
		{InstanceID: 20, CustomData: ItemCustomization{Name: "Larger prey"}, Fish: FishInstanceData{SpeciesID: "prey", LengthCM: 20, WeightKG: 0.08, Condition: 1, Health: 1, LifeState: "alive"}},
		{InstanceID: 30, CustomData: ItemCustomization{Name: "Small fry"}, Fish: FishInstanceData{SpeciesID: "prey", LengthCM: 10, WeightKG: 0.01, Condition: 1, Health: 1, LifeState: "alive"}},
	}
	at := time.Date(2026, time.September, 2, 18, 0, 0, 0, time.UTC)
	events := store.settleFishingPondPredationStep(rows, 7, at, 24)

	if len(events) != 1 || events[0].PredatorInstanceID != 10 || events[0].PreyInstanceID != 30 || events[0].PreyName != "Small fry" {
		t.Fatalf("unexpected predation events: %+v", events)
	}
	if rows[2].Fish.LifeState != "dead" || rows[2].Fish.DeathCause != "predation" || rows[2].Fish.DeathAt == nil || !rows[2].Fish.DeathAt.Equal(at) {
		t.Fatalf("smallest prey was not consumed: %+v", rows[2].Fish)
	}
	if rows[1].Fish.LifeState != "alive" {
		t.Fatalf("larger eligible prey should survive the predator's single hunt: %+v", rows[1].Fish)
	}
	if rows[0].Fish.Satiety <= 0 || rows[0].Fish.WeightKG <= 1.25 || rows[0].Fish.StarvationHours != 0 {
		t.Fatalf("predator did not benefit from its meal: %+v", rows[0].Fish)
	}
}

func TestSettleFishingPondPredationRequiresHungerAndSizeAdvantage(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		FishingSpecies: []catalog.FishingSpecies{{ID: "predator", MaxLengthCM: 100, ShapeFactor: 1}, {ID: "prey", MaxLengthCM: 100, ShapeFactor: 1}},
		FishingPondSpecies: []catalog.FishingPondSpecies{
			{SpeciesID: "predator", Predation: &catalog.FishingPondPredation{MaxPreyLengthRatio: 0.4, HuntBelowSatiety: 60, DailyHuntChance: 1}},
			{SpeciesID: "prey"},
		},
	}}
	rows := []fishingPondFishRow{
		{InstanceID: 1, Fish: FishInstanceData{SpeciesID: "predator", LengthCM: 50, WeightKG: 1.25, Condition: 1, Health: 1, Satiety: 60, LifeState: "alive"}},
		{InstanceID: 2, Fish: FishInstanceData{SpeciesID: "prey", LengthCM: 10, WeightKG: 0.01, Condition: 1, Health: 1, LifeState: "alive"}},
	}
	if events := store.settleFishingPondPredationStep(rows, 7, time.Unix(100, 0), 24); len(events) != 0 {
		t.Fatalf("well-fed predator hunted: %+v", events)
	}
	rows[0].Fish.Satiety = 0
	rows[1].Fish.LengthCM = 21
	if events := store.settleFishingPondPredationStep(rows, 7, time.Unix(200, 0), 24); len(events) != 0 {
		t.Fatalf("predator attacked oversized prey: %+v", events)
	}
}

func TestSettleFishingPondPredationDoesNotCrossNurseryBoundary(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		FishingSpecies: []catalog.FishingSpecies{{ID: "predator", MaxLengthCM: 100, ShapeFactor: 1}, {ID: "prey", MaxLengthCM: 40, ShapeFactor: 1}},
		FishingPondSpecies: []catalog.FishingPondSpecies{
			{SpeciesID: "predator", Predation: &catalog.FishingPondPredation{MaxPreyLengthRatio: 0.5, HuntBelowSatiety: 70, DailyHuntChance: 1}},
			{SpeciesID: "prey"},
		},
	}}
	rows := []fishingPondFishRow{
		{InstanceID: 1, Fish: FishInstanceData{SpeciesID: "predator", PondZone: fishingPondZoneMain, LengthCM: 50, WeightKG: 1, Satiety: 0, LifeState: "alive"}},
		{InstanceID: 2, Fish: FishInstanceData{SpeciesID: "prey", PondZone: fishingPondZoneNursery, LengthCM: 10, WeightKG: 0.01, LifeState: "alive"}},
		{InstanceID: 3, Fish: FishInstanceData{SpeciesID: "predator", PondZone: fishingPondZoneNursery, LengthCM: 50, WeightKG: 1, Satiety: 0, LifeState: "alive"}},
		{InstanceID: 4, Fish: FishInstanceData{SpeciesID: "prey", PondZone: fishingPondZoneMain, LengthCM: 30, WeightKG: 0.2, LifeState: "alive"}},
	}
	if events := store.settleFishingPondPredationStep(rows, 7, time.Unix(100, 0), 24); len(events) != 0 {
		t.Fatalf("predation crossed nursery boundary: %+v", events)
	}
}

func TestFishingPondPredationChanceIsStableAcrossHourlySettlement(t *testing.T) {
	daily := fishingPondPredationChance(0.65, 0.8, 24)
	hourly := fishingPondPredationChance(0.65, 0.8, 1)
	recombined := 1 - math.Pow(1-hourly, 24)
	if math.Abs(daily-recombined) > 1e-9 {
		t.Fatalf("hourly predation probability drifted: daily=%v recombined=%v", daily, recombined)
	}
	if fishingPondPredationChance(0.65, 0.8, 0) != 0 || fishingPondPredationChance(0, 0.8, 24) != 0 {
		t.Fatal("zero-duration or disabled predation should have no chance")
	}
}

func TestFishingPondNurseryCapacityReleasesConsumedPrey(t *testing.T) {
	rows := []fishingPondFishRow{
		{InstanceID: 1, Fish: FishInstanceData{PondZone: fishingPondZoneMain, LifeState: "alive", WeightKG: 2}},
		{InstanceID: 2, Fish: FishInstanceData{PondZone: fishingPondZoneNursery, LifeState: "alive", WeightKG: 3}},
		{InstanceID: 3, Fish: FishInstanceData{PondZone: fishingPondZoneNursery, LifeState: "dead", DeathCause: "old_age"}},
		{InstanceID: 4, Fish: FishInstanceData{PondZone: fishingPondZoneNursery, LifeState: "dead", DeathCause: "predation"}},
	}
	if occupied := fishingPondZoneOccupiedCount(rows, fishingPondZoneNursery); occupied != 2 {
		t.Fatalf("nursery capacity occupied=%d, want 2", occupied)
	}
	if occupied := fishingPondZoneOccupiedCount(rows, fishingPondZoneMain); occupied != 1 {
		t.Fatalf("main capacity occupied=%d, want 1", occupied)
	}
	if biomass := fishingPondMainBiomass(rows); biomass != 2 {
		t.Fatalf("main biomass=%v, want 2", biomass)
	}
}

func TestFishingPondBreedingWaitTargetsByRarity(t *testing.T) {
	tests := []struct {
		rarity string
		hours  float64
	}{
		{rarity: "common", hours: 6},
		{rarity: "uncommon", hours: 8},
		{rarity: "rare", hours: 10},
		{rarity: "epic", hours: 13},
		{rarity: "legendary", hours: 16},
		{rarity: "mythic", hours: 16},
	}
	for _, test := range tests {
		species := &catalog.FishingSpecies{Rarity: test.rarity}
		if got := fishingPondBreedExpectedHours(species, true); got != test.hours {
			t.Errorf("paired %s wait=%v hours, want %v", test.rarity, got, test.hours)
		}
		if got := fishingPondBreedExpectedHours(species, false); got != test.hours*2 {
			t.Errorf("scatter %s wait=%v hours, want %v", test.rarity, got, test.hours*2)
		}
	}
}

func TestFishingPondBreedingChanceMatchesTargetAndElapsedTime(t *testing.T) {
	species := &catalog.FishingSpecies{Rarity: "common"}
	hourly := fishingPondBreedChance(species, true, 0, false, 1)
	meanHours := 1 / hourly
	if math.Abs(meanHours-6) > 1e-9 {
		t.Fatalf("paired common mean wait=%v hours, want 6", meanHours)
	}
	sixHours := fishingPondBreedChance(species, true, 0, false, 6)
	recombined := 1 - math.Pow(1-hourly, 6)
	if math.Abs(sixHours-recombined) > 1e-9 {
		t.Fatalf("partitioned breeding probability drifted: direct=%v recombined=%v", sixHours, recombined)
	}
	if scatter := fishingPondBreedChance(species, false, 0, false, 1); !(scatter > 0 && scatter < hourly) {
		t.Fatalf("scatter chance=%v, want below paired chance=%v", scatter, hourly)
	}
	if boosted := fishingPondBreedChance(species, true, 0, true, 1); boosted <= hourly {
		t.Fatalf("mash chance=%v, want above base chance=%v", boosted, hourly)
	}
}

func TestPondFishCanBreedUsesShortPlayerCycle(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	lastBredAt := now.Add(-fishingPondBreedCooldown)
	profile := &catalog.FishingPondSpecies{MaturityAgeDays: 365}
	ready := FishInstanceData{
		AgeDays: 365, LifeState: "alive", Satiety: 80, Health: 0.85, Condition: 0.9,
		PreferredFeedHours: fishingPondBreedFeedHours, LastBredAt: &lastBredAt,
	}
	if !pondFishCanBreed(ready, profile, now) {
		t.Fatal("fish should breed after one fed hour and one cooldown hour")
	}
	notFed := ready
	notFed.PreferredFeedHours = fishingPondBreedFeedHours - 0.01
	if pondFishCanBreed(notFed, profile, now) {
		t.Fatal("fish bred before completing the shortened feed requirement")
	}
	onCooldown := ready
	recent := now.Add(-fishingPondBreedCooldown + time.Minute)
	onCooldown.LastBredAt = &recent
	if pondFishCanBreed(onCooldown, profile, now) {
		t.Fatal("fish bred twice inside the same cooldown hour")
	}
}

func TestSettleFishingPondBreedingSkipsFullNursery(t *testing.T) {
	rows := []fishingPondFishRow{
		{InstanceID: 1, Fish: FishInstanceData{PondZone: fishingPondZoneNursery, LifeState: "alive"}},
		{InstanceID: 2, Fish: FishInstanceData{PondZone: fishingPondZoneNursery, LifeState: "alive"}},
	}
	pairs := []fishingPondPairRow{}
	err := (&Store{}).settleFishingPondBreedingStepTx(
		context.Background(), nil, 7, time.Unix(100, 0), 24,
		FishingPondRulesetVersion, FishingPondLimits{FishCapacity: 12, NurseryCapacity: 2, BiomassKG: 100}, &rows, &pairs,
	)
	if err != nil {
		t.Fatalf("settle full nursery breeding: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("full nursery spawned offspring: fish=%d want=2", len(rows))
	}
}

func TestFishingPondBreedingRulesetsKeepLegacyHistoryOutOfNursery(t *testing.T) {
	limits := FishingPondLimits{FishCapacity: 21, NurseryCapacity: 12, BiomassKG: 150}
	legacy, err := fishingPondBreedingPolicyForRuleset(fishingPondPreviousRulesetVersion, limits)
	if err != nil {
		t.Fatalf("legacy breeding policy: %v", err)
	}
	if legacy.Capacity != limits.FishCapacity || legacy.NewbornZone != fishingPondZoneMain || legacy.NewbornRuleset != fishingPondPreviousRulesetVersion || !legacy.EnforceBiomass || !legacy.CountAllPondFish || legacy.ParentZone != "" {
		t.Fatalf("unexpected legacy breeding policy: %+v", legacy)
	}

	current, err := fishingPondBreedingPolicyForRuleset(FishingPondRulesetVersion, limits)
	if err != nil {
		t.Fatalf("current breeding policy: %v", err)
	}
	if current.Capacity != limits.NurseryCapacity || current.ParentZone != fishingPondZoneMain || current.NewbornZone != fishingPondZoneNursery || current.NewbornRuleset != FishingPondRulesetVersion || current.EnforceBiomass || current.CountAllPondFish {
		t.Fatalf("unexpected current breeding policy: %+v", current)
	}

	if _, err := fishingPondBreedingPolicyForRuleset("fishing-pond-v999", limits); !errors.Is(err, ErrFishingPondRulesetUnsupported) {
		t.Fatalf("unknown policy error=%v, want ErrFishingPondRulesetUnsupported", err)
	}
}

func TestSettleFishingPondLegacyBreedingSkipsFullCombinedPond(t *testing.T) {
	rows := []fishingPondFishRow{
		{InstanceID: 1, Fish: FishInstanceData{PondZone: fishingPondZoneMain, LifeState: "alive"}},
		{InstanceID: 2, Fish: FishInstanceData{PondZone: fishingPondZoneNursery, LifeState: "alive"}},
	}
	pairs := []fishingPondPairRow{}
	err := (&Store{}).settleFishingPondBreedingStepTx(
		context.Background(), nil, 7, time.Unix(100, 0), 24,
		fishingPondPreviousRulesetVersion, FishingPondLimits{FishCapacity: 2, NurseryCapacity: 12, BiomassKG: 100}, &rows, &pairs,
	)
	if err != nil {
		t.Fatalf("settle legacy full pond breeding: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("legacy full pond spawned offspring: fish=%d want=2", len(rows))
	}
}

func TestFishingPondSettlementStepSeedSeparatesFractionalIntervals(t *testing.T) {
	at := time.Date(2026, time.September, 2, 8, 47, 0, 0, time.UTC)
	halfHour := fishingPondSettlementStepSeed(at, 0.5)
	if halfHour != fishingPondSettlementStepSeed(at, 0.5) {
		t.Fatal("fractional settlement seed is not deterministic")
	}
	if halfHour == fishingPondSettlementStepSeed(at, 0.25) || halfHour == fishingPondSettlementStepSeed(at.Add(time.Minute), 0.5) {
		t.Fatalf("different fractional intervals share a seed: %q", halfHour)
	}
}

func fishingPondSettlementRemainder(start, settledAt time.Time, fullHours int) time.Duration {
	return settledAt.Sub(start) - time.Duration(fullHours)*time.Hour
}

func TestConsumeFishingPondNutritionUsesPreferenceWithoutCreatingNutrition(t *testing.T) {
	store := &Store{gameData: catalog.Data{Items: []catalog.Item{
		{ID: "herbal", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal"}},
		{ID: "protein", PondFeed: &catalog.PondFeedConfig{Nutrition: 20, FeedClass: "protein"}},
	}}}
	stock := map[string]float64{"herbal": 10, "protein": 10}
	profile := &catalog.FishingPondSpecies{DietClass: "herbal", SecondaryFeedClasses: []string{"protein"}}
	effective, preferred := store.consumeFishingPondNutrition(stock, profile, 0, 12)
	if !preferred {
		t.Fatal("preferred feed was not recognized")
	}
	if math.Abs(effective-12) > 0.001 {
		t.Fatalf("effective nutrition=%v, want 12", effective)
	}
	if stock["herbal"] > 0.001 || stock["protein"] >= 10 {
		t.Fatalf("unexpected stock consumption: %+v", stock)
	}
	if pondFeedTotal(stock) >= 10 {
		t.Fatalf("raw stock was not consumed: %+v", stock)
	}
}

func TestConsumeFishingPondNutritionPreservesSubUnitConsumption(t *testing.T) {
	store := &Store{gameData: catalog.Data{Items: []catalog.Item{
		{ID: "herbal", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal"}},
	}}}
	stock := map[string]float64{"herbal": 1}
	profile := &catalog.FishingPondSpecies{DietClass: "herbal"}
	for range 1000 {
		effective, preferred := store.consumeFishingPondNutrition(stock, profile, 0, 0.0001)
		if !preferred || math.Abs(effective-0.0001) > 1e-12 {
			t.Fatalf("small consumption returned effective=%v preferred=%v", effective, preferred)
		}
	}
	if math.Abs(stock["herbal"]-0.9) > 1e-9 {
		t.Fatalf("small settlement steps created free nutrition: stock=%v, want 0.9", stock["herbal"])
	}
}

func TestFishingPondZonesConsumeSharedFeedStock(t *testing.T) {
	store := &Store{gameData: catalog.Data{
		Items:              []catalog.Item{{ID: "herbal", PondFeed: &catalog.PondFeedConfig{Nutrition: 10, FeedClass: "herbal"}}},
		FishingSpecies:     []catalog.FishingSpecies{{ID: "test_fish", MinLengthCM: 10, MaxLengthCM: 100, ShapeFactor: 1}},
		FishingPondSpecies: []catalog.FishingPondSpecies{{SpeciesID: "test_fish", DietClass: "herbal", MaturityAgeDays: 30, LifespanDays: 3650}},
	}}
	stock := map[string]float64{"herbal": 20}
	rows := []fishingPondFishRow{
		{InstanceID: 1, Fish: FishInstanceData{SpeciesID: "test_fish", PondZone: fishingPondZoneMain, LengthCM: 10, WeightKG: 0.1, Condition: 1, Health: 1, Satiety: 100, LifeState: "alive"}},
		{InstanceID: 2, Fish: FishInstanceData{SpeciesID: "test_fish", PondZone: fishingPondZoneNursery, LengthCM: 10, WeightKG: 0.1, Condition: 1, Health: 1, Satiety: 100, LifeState: "alive"}},
	}
	store.settleFishingPondFishStep(&rows[0], stock, 7, time.Unix(100, 0), 0.5)
	afterMain := stock["herbal"]
	store.settleFishingPondFishStep(&rows[1], stock, 7, time.Unix(100, 0), 0.5)
	if !(afterMain < 20 && stock["herbal"] < afterMain) {
		t.Fatalf("zones did not consume the shared stock: after main=%v after nursery=%v", afterMain, stock["herbal"])
	}
}

func TestInheritPondGenesStaysNormalized(t *testing.T) {
	parentA := FishInstanceData{Genes: FishGenes{SizePotential: 1, GrowthRate: 1, FeedEfficiency: 1, Vitality: 1, Lifespan: 1, Fertility: 1}, ParentInstanceIDs: []int64{9}}
	parentB := FishInstanceData{Genes: FishGenes{SizePotential: 1, GrowthRate: 1, FeedEfficiency: 1, Vitality: 1, Lifespan: 1, Fertility: 1}, ParentInstanceIDs: []int64{9}}
	child := inheritPondGenes(parentA, parentB, "test-lineage")
	for name, value := range map[string]float64{"size": child.SizePotential, "growth": child.GrowthRate, "feed": child.FeedEfficiency, "vitality": child.Vitality, "life": child.Lifespan, "fertility": child.Fertility} {
		if value < -1 || value > 1 {
			t.Fatalf("%s gene=%v outside normalized range", name, value)
		}
	}
	if child.SizePotential >= 1 {
		t.Fatalf("soft inbreeding penalty did not reduce inherited maximum: %+v", child)
	}
}

func TestFishingPondTournamentEligibilityRequiresLongCycleWelfare(t *testing.T) {
	profile := &catalog.FishingPondSpecies{MaturityAgeDays: 720}
	eligible := FishInstanceData{Origin: "cultured", LifeState: "alive", AgeDays: 720, Health: 0.85, Condition: 0.9, Satiety: 80}
	if !fishingPondTournamentEligible(eligible, profile) {
		t.Fatal("healthy mature cultured fish should qualify for the open division")
	}
	checks := []FishInstanceData{eligible, eligible, eligible, eligible, eligible, eligible}
	checks[0].Origin = "wild"
	checks[1].AgeDays = 719
	checks[2].Health = 0.84
	checks[3].Condition = 0.89
	checks[4].Satiety = 79
	checks[5].PondZone = fishingPondZoneNursery
	for index, fish := range checks {
		if fishingPondTournamentEligible(fish, profile) {
			t.Fatalf("ineligible welfare case %d qualified", index)
		}
	}
}

func TestFishingPondMaximumFryWeightBoundsLegacySpawn(t *testing.T) {
	species := &catalog.FishingSpecies{MinLengthCM: 10, ShapeFactor: 1.5}
	maximum := fishingPondMaximumFryWeight(species)
	length := math.Max(2, species.MinLengthCM*0.18)
	for _, condition := range []float64{0.94, 0.98, 1.02} {
		actual := math.Max(0.005, species.ShapeFactor*math.Pow(length, 3)/100_000*condition)
		if actual > maximum+1e-12 {
			t.Fatalf("actual fry weight %v exceeds reserved maximum %v", actual, maximum)
		}
	}
}

func TestNormalizeFishingPondInstanceIDsSortsAndRejectsAmbiguousBatches(t *testing.T) {
	normalized, err := normalizeFishingPondInstanceIDs([]int64{19, 3, 11})
	if err != nil {
		t.Fatalf("normalize valid batch: %v", err)
	}
	want := []int64{3, 11, 19}
	for index := range want {
		if normalized[index] != want[index] {
			t.Fatalf("normalized=%v want=%v", normalized, want)
		}
	}
	for name, instanceIDs := range map[string][]int64{
		"empty": {}, "zero": {1, 0}, "duplicate": {7, 7}, "too_many": make([]int64, 201),
	} {
		if _, err := normalizeFishingPondInstanceIDs(instanceIDs); !errors.Is(err, ErrFishingPondFishInvalid) {
			t.Fatalf("%s batch error=%v, want ErrFishingPondFishInvalid", name, err)
		}
	}
}

func TestFishingPondCapacityProgressionKeepsLevelOneAndExpandsLaterLevels(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	store := &Store{gameData: data}
	want := []int64{12, 21, 27, 33, 39}
	for level, capacity := range want {
		limits := store.fishingPondLimits(int64(level + 1))
		if actual := limits.FishCapacity; actual != capacity {
			t.Fatalf("level %d capacity=%d want=%d", level+1, actual, capacity)
		}
		if limits.NurseryCapacity != 12 {
			t.Fatalf("level %d nursery capacity=%d want=12", level+1, limits.NurseryCapacity)
		}
	}
}

func TestFishingPondCatalogPredatorsHaveBoundedRules(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	want := map[string]bool{
		"snakehead": true, "brown_trout": true, "river_eel": true, "freshwater_perch": true,
		"lake_trout": true, "arctic_char": true, "burbot": true, "northern_pike": true,
	}
	seen := make(map[string]bool)
	for _, profile := range data.FishingPondSpecies {
		if profile.Predation == nil {
			continue
		}
		seen[profile.SpeciesID] = true
		if profile.Predation.MaxPreyLengthRatio < 0.1 || profile.Predation.MaxPreyLengthRatio > 0.6 || profile.Predation.HuntBelowSatiety < 20 || profile.Predation.HuntBelowSatiety > 90 || profile.Predation.DailyHuntChance <= 0 || profile.Predation.DailyHuntChance > 1 {
			t.Fatalf("predation config outside supported range for %s: %+v", profile.SpeciesID, profile.Predation)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("predator count=%d want=%d: %+v", len(seen), len(want), seen)
	}
	for speciesID := range want {
		if !seen[speciesID] {
			t.Fatalf("missing predator profile for %s", speciesID)
		}
	}
}

func TestFishingPondNPCSaleStaysBelowFeedOpportunityCost(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	itemByID := make(map[string]catalog.Item, len(data.Items))
	for _, item := range data.Items {
		itemByID[item.ID] = item
	}
	minRawCostPerNutrition := math.Inf(1)
	for _, action := range data.ActionDefinitions() {
		if action.SubType != "cooking_fish_feed" {
			continue
		}
		for itemID, reward := range action.ItemReward {
			item := itemByID[itemID]
			if item.PondFeed == nil || item.PondFeed.Nutrition <= 0 || reward.Count == nil || *reward.Count <= 0 {
				continue
			}
			inputValue := float64(0)
			for costID, quantity := range action.Costs {
				inputValue += itemByID[costID].EconomyBaseValue * float64(quantity)
			}
			costPerNutrition := inputValue / (float64(*reward.Count) * item.PondFeed.Nutrition)
			minRawCostPerNutrition = math.Min(minRawCostPerNutrition, costPerNutrition)
		}
	}
	if math.IsInf(minRawCostPerNutrition, 1) {
		t.Fatal("no cooked pond feed recipe was found")
	}
	bestEffectiveFeedCost := minRawCostPerNutrition / 1.08
	maxCulturedSalePerNutrition := fishingCulturedValuePerKGCap * fishingSaleSpecimenMaximum * fishingSaleMarketMaximum / fishingPondNutritionPerKG
	if maxCulturedSalePerNutrition >= bestEffectiveFeedCost {
		t.Fatalf("cultured sale can outpay feed before maintenance: sale=%v feed=%v", maxCulturedSalePerNutrition, bestEffectiveFeedCost)
	}
}
