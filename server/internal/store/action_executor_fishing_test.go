package store

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func TestFishingActionUsesFishingExecutor(t *testing.T) {
	store := &Store{}
	if got := store.executorForAction(FishingTripActionID).ActionID(); got != "fishing_trip" {
		t.Fatalf("executorForAction(%q).ActionID() = %q, want fishing_trip", FishingTripActionID, got)
	}
	payload, err := marshalFishingEnvelope(FishingTripActionID, fishingExecutorRuntime{DurationMS: 37_000})
	if err != nil {
		t.Fatalf("marshal fishing runtime: %v", err)
	}
	if got := parseQueueRuntimeDuration(payload); got != 37*time.Second {
		t.Fatalf("parseQueueRuntimeDuration(fishing) = %v, want 37s", got)
	}
}

func TestFishingFastForwardStateLoadsShareOneLockedBatch(t *testing.T) {
	speciesIDs := []string{"common_carp", "crucian_carp"}
	loadBatch := newFishingFastForwardStateLoadBatch(42, "rules-v2", speciesIDs)
	if loadBatch.Len() != 2 {
		t.Fatalf("state load batch length = %d, want 2", loadBatch.Len())
	}

	knowledgeQuery := loadBatch.QueuedQueries[0]
	knowledgeSQL := strings.Join(strings.Fields(strings.ToLower(knowledgeQuery.SQL)), " ")
	if !strings.Contains(knowledgeSQL, "from fishing_species_knowledge") || !strings.Contains(knowledgeSQL, "for update") {
		t.Fatalf("knowledge load lost its locked read: %s", knowledgeSQL)
	}
	if len(knowledgeQuery.Arguments) != 2 || knowledgeQuery.Arguments[0] != int64(42) {
		t.Fatalf("knowledge load arguments = %#v", knowledgeQuery.Arguments)
	}
	knowledgeSpecies, ok := knowledgeQuery.Arguments[1].([]string)
	if !ok || strings.Join(knowledgeSpecies, ",") != strings.Join(speciesIDs, ",") {
		t.Fatalf("knowledge species arguments = %#v", knowledgeQuery.Arguments[1])
	}

	recordQuery := loadBatch.QueuedQueries[1]
	recordSQL := strings.Join(strings.Fields(strings.ToLower(recordQuery.SQL)), " ")
	if !strings.Contains(recordSQL, "from fishing_records") || !strings.Contains(recordSQL, "for update") {
		t.Fatalf("record load lost its locked read: %s", recordSQL)
	}
	if len(recordQuery.Arguments) != 3 || recordQuery.Arguments[0] != int64(42) || recordQuery.Arguments[1] != "rules-v2" {
		t.Fatalf("record load arguments = %#v", recordQuery.Arguments)
	}
	recordSpecies, ok := recordQuery.Arguments[2].([]string)
	if !ok || strings.Join(recordSpecies, ",") != strings.Join(speciesIDs, ",") {
		t.Fatalf("record species arguments = %#v", recordQuery.Arguments[2])
	}
}

func TestFishingTripRecoveryRequiresItsMatchingRunningQueue(t *testing.T) {
	for name, raw := range map[string]string{
		"preflight": orphanedFishingTripExistsSQL,
		"locked":    fishingTripStopSelectSQL,
	} {
		query := strings.Join(strings.Fields(strings.ToLower(raw)), " ")
		for _, required := range []string{
			"q.id = ft.queue_id",
			"q.user_id = ft.user_id",
			"q.action_id = $2",
			"q.actor_type = ft.actor_type",
			"q.actor_id = ft.actor_id",
			"q.status = 'running'",
		} {
			if !strings.Contains(query, required) {
				t.Fatalf("%s fishing recovery query missing %q", name, required)
			}
		}
	}
	lockedQuery := strings.Join(strings.Fields(strings.ToLower(fishingTripStopSelectSQL)), " ")
	if !strings.Contains(lockedQuery, "for update of ft") {
		t.Fatal("locked fishing recovery query must protect the trip row while returning supplies")
	}
	preflightQuery := strings.Join(strings.Fields(strings.ToLower(orphanedFishingTripExistsSQL)), " ")
	if !strings.Contains(preflightQuery, "and not exists (") {
		t.Fatal("fishing recovery preflight must select only trips without a matching running queue")
	}

	for _, tc := range []struct {
		name            string
		orphanedOnly    bool
		hasRunningQueue bool
		want            bool
	}{
		{name: "manual stop remains forceful", hasRunningQueue: true, want: true},
		{name: "recovery preserves active trip", orphanedOnly: true, hasRunningQueue: true, want: false},
		{name: "recovery closes orphan", orphanedOnly: true, hasRunningQueue: false, want: true},
	} {
		if got := shouldStopFishingTrip(tc.orphanedOnly, tc.hasRunningQueue); got != tc.want {
			t.Fatalf("%s: shouldStopFishingTrip()=%t, want %t", tc.name, got, tc.want)
		}
	}
}

func TestFishingRuntimeKeepsSelectedKittyAngler(t *testing.T) {
	runtime := fishingExecutorRuntime{
		DurationMS:         42_000,
		TripID:             "trip-kitty",
		ActorType:          ActionActorTypeKitty,
		ActorID:            73,
		SpotID:             "reed-bank",
		TournamentDivision: fishingTournamentDivisionStandard,
		TargetLayer:        fishingTargetLayerMidwater,
		Pace:               fishingPaceSearch,
		EquipmentKittyIDs:  []int64{18, 73},
		Supplies: FishingTripSupplies{
			Wear: FishingSupplyWear{MainLine: 0.42, Leader: 0.73},
		},
	}
	payload, err := marshalFishingEnvelope(FishingTripActionID, runtime)
	if err != nil {
		t.Fatalf("marshal fishing runtime: %v", err)
	}
	parsed, ok := parseFishingExecutorRuntime(payload)
	if !ok {
		t.Fatal("parse fishing runtime failed")
	}
	if parsed.ActorType != ActionActorTypeKitty || parsed.ActorID != 73 {
		t.Fatalf("parsed angler=(%q,%d), want=(%q,73)", parsed.ActorType, parsed.ActorID, ActionActorTypeKitty)
	}
	if parsed.TargetLayer != fishingTargetLayerMidwater || parsed.Pace != fishingPaceSearch {
		t.Fatalf("parsed advanced settings=(%q,%q)", parsed.TargetLayer, parsed.Pace)
	}
	if parsed.TournamentDivision != fishingTournamentDivisionStandard {
		t.Fatalf("parsed tournament division=%q, want standard", parsed.TournamentDivision)
	}
	if len(parsed.EquipmentKittyIDs) != 2 || parsed.EquipmentKittyIDs[0] != 18 || parsed.EquipmentKittyIDs[1] != 73 {
		t.Fatalf("parsed equipment kitty ids=%v, want [18 73]", parsed.EquipmentKittyIDs)
	}
	if parsed.Supplies.Wear.MainLine != 0.42 || parsed.Supplies.Wear.Leader != 0.73 {
		t.Fatalf("parsed fishing wear=%+v, want main=0.42 leader=0.73", parsed.Supplies.Wear)
	}
}

func TestFishingTripEventsStayBoundedAndOrdered(t *testing.T) {
	runtime := fishingExecutorRuntime{DurationMS: 1_000, TripID: "trip-events", SpotID: "spot-events"}
	start := time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)
	eventCount := fishingRecentEventLimit + 7
	for index := 0; index < eventCount; index++ {
		appendFishingTripEvent(&runtime, FishingTripEvent{
			Kind: "cast", At: start.Add(time.Duration(index) * time.Second), CastNumber: int64(index + 1),
		})
	}
	if got := len(runtime.Summary.RecentEvents); got != fishingRecentEventLimit {
		t.Fatalf("recent event count=%d, want %d", got, fishingRecentEventLimit)
	}
	first := runtime.Summary.RecentEvents[0]
	last := runtime.Summary.RecentEvents[len(runtime.Summary.RecentEvents)-1]
	if first.Sequence != 8 || first.CastNumber != 8 {
		t.Fatalf("first retained event=%+v, want sequence and cast 8", first)
	}
	if last.Sequence != int64(fishingRecentEventLimit+7) || runtime.TimelineSeq != last.Sequence {
		t.Fatalf("last retained event=%+v timeline=%d", last, runtime.TimelineSeq)
	}
	if got := len(runtime.pendingEvents); got != eventCount {
		t.Fatalf("pending event count=%d, want complete history %d", got, eventCount)
	}
	if runtime.pendingEvents[0].Sequence != 1 || runtime.pendingEvents[eventCount-1].Sequence != int64(eventCount) {
		t.Fatalf("pending event sequence range=%d..%d, want 1..%d", runtime.pendingEvents[0].Sequence, runtime.pendingEvents[eventCount-1].Sequence, eventCount)
	}
	payload, err := marshalFishingEnvelope(FishingTripActionID, runtime)
	if err != nil {
		t.Fatalf("marshal fishing event runtime: %v", err)
	}
	parsed, ok := parseFishingExecutorRuntime(payload)
	if !ok {
		t.Fatal("parse fishing event runtime failed")
	}
	if len(parsed.pendingEvents) != 0 || len(parsed.Summary.RecentEvents) != fishingRecentEventLimit {
		t.Fatalf("serialized runtime pending=%d recent=%d", len(parsed.pendingEvents), len(parsed.Summary.RecentEvents))
	}
	if parsed.TournamentDivision != fishingTournamentDivisionOpen {
		t.Fatalf("legacy runtime division=%q, want open", parsed.TournamentDivision)
	}
}

func TestFishingEscapeReasonNamesTheDominantFailure(t *testing.T) {
	species := catalog.FishingSpecies{FightPower: 1, HookingChallenge: 3}
	fish := FishInstanceData{WeightKG: 1}
	if got := fishingEscapeReason(species, fish, FishingLoadoutSnapshot{Control: 4, Hooking: 0.1}, "balanced"); got != "hook_slipped" {
		t.Fatalf("weak hook hold reason=%q, want hook_slipped", got)
	}
	if got := fishingEscapeReason(species, FishInstanceData{WeightKG: 25}, FishingLoadoutSnapshot{Control: 0.1, Hooking: 4}, "balanced"); got != "lost_control" {
		t.Fatalf("weak control reason=%q, want lost_control", got)
	}
}

func TestFishingAdvancedSettingsValidateAndNormalize(t *testing.T) {
	executor := newFishingExecutor(&Store{})
	raw := []byte(`{"spotId":"reed-bank","loadoutId":"set-1","tournamentDivision":" Standard ","durationSeconds":600,"targetLayer":" MidWater ","pace":" Patient ","strategy":"conservative","keepRule":{"mode":"keep_all"},"supplies":{"mainLine":1,"leader":1,"hook":1,"floatOrSinker":1,"bait":1}}`)
	parsed, err := executor.ValidateAndNormalizeArgs(raw)
	if err != nil {
		t.Fatalf("ValidateAndNormalizeArgs() error = %v", err)
	}
	args := parsed.(fishingExecutorArgs)
	if args.TargetLayer != fishingTargetLayerMidwater || args.Pace != fishingPacePatient || args.TournamentDivision != fishingTournamentDivisionStandard {
		t.Fatalf("normalized advanced settings = (%q,%q,%q), want (%q,%q,%q)", args.TargetLayer, args.Pace, args.TournamentDivision, fishingTargetLayerMidwater, fishingPacePatient, fishingTournamentDivisionStandard)
	}
	legacyRaw := []byte(`{"spotId":"reed-bank","loadoutId":"set-1","strategy":"aggressive","keepRule":{"mode":"keep_all"},"supplies":{"mainLine":1,"leader":1}}`)
	legacyParsed, err := executor.ValidateAndNormalizeArgs(legacyRaw)
	if err != nil {
		t.Fatalf("legacy ValidateAndNormalizeArgs() error = %v", err)
	}
	if legacyParsed.(fishingExecutorArgs).Pace != "" {
		t.Fatalf("legacy request unexpectedly gained pace %q", legacyParsed.(fishingExecutorArgs).Pace)
	}
	if legacyParsed.(fishingExecutorArgs).TournamentDivision != fishingTournamentDivisionOpen {
		t.Fatalf("legacy request division=%q, want open", legacyParsed.(fishingExecutorArgs).TournamentDivision)
	}

	for _, invalid := range []string{
		`{"spotId":"reed-bank","loadoutId":"set-1","targetLayer":"abyss","keepRule":{"mode":"keep_all"},"supplies":{"mainLine":1,"leader":1}}`,
		`{"spotId":"reed-bank","loadoutId":"set-1","pace":"frantic","keepRule":{"mode":"keep_all"},"supplies":{"mainLine":1,"leader":1}}`,
		`{"spotId":"reed-bank","loadoutId":"set-1","tournamentDivision":"practice","keepRule":{"mode":"keep_all"},"supplies":{"mainLine":1,"leader":1}}`,
	} {
		if _, err := executor.ValidateAndNormalizeArgs([]byte(invalid)); !errors.Is(err, ErrFishingArgsInvalid) {
			t.Fatalf("invalid advanced settings error = %v, want ErrFishingArgsInvalid", err)
		}
	}
}

func TestFishingLayerTargetingUsesSpeciesEcology(t *testing.T) {
	surfaceFish := catalog.FishingSpecies{ID: "surface", PreferredLayers: []string{fishingTargetLayerSurface}}
	bottomFish := catalog.FishingSpecies{ID: "bottom", PreferredLayers: []string{fishingTargetLayerNearBottom}}
	if got := fishingSpeciesLayerWeight(surfaceFish, fishingTargetLayerSurface); got != 1.45 {
		t.Fatalf("matching layer weight = %v, want 1.45", got)
	}
	if got := fishingSpeciesLayerWeight(bottomFish, fishingTargetLayerSurface); got != 0.58 {
		t.Fatalf("nonmatching layer weight = %v, want 0.58", got)
	}
	if got := fishingSpeciesLayerWeight(surfaceFish, fishingTargetLayerAuto); got != 1 {
		t.Fatalf("automatic layer weight = %v, want 1", got)
	}
	spot := catalog.FishingSpot{WaterID: "water", SpeciesWeights: map[string]float64{"surface": 1, "bottom": 1}}
	surfaceFish.WaterID = "water"
	bottomFish.WaterID = "water"
	if got := fishingLayerBiteMultiplier([]catalog.FishingSpecies{surfaceFish, bottomFish}, spot, FishingLoadoutSnapshot{}, FishingConditions{}, fishingTargetLayerSurface); math.Abs(got-0.86) > 0.0001 {
		t.Fatalf("mixed layer bite multiplier = %v, want 0.86", got)
	}
	if got := fishingLayerBiteMultiplier([]catalog.FishingSpecies{bottomFish}, catalog.FishingSpot{WaterID: "water", SpeciesWeights: map[string]float64{"bottom": 1}}, FishingLoadoutSnapshot{}, FishingConditions{}, fishingTargetLayerSurface); got != 0.62 {
		t.Fatalf("empty target layer bite multiplier = %v, want 0.62", got)
	}
	legacy := catalog.FishingSpecies{ID: "legacy", Depth: "deep"}
	if layers := fishingSpeciesLayers(legacy); len(layers) != 1 || layers[0] != fishingTargetLayerNearBottom {
		t.Fatalf("legacy depth fallback = %v, want near_bottom", layers)
	}
	if got := effectiveFishingTargetLayer(fishingTargetLayerAuto); got != fishingTargetLayerAuto {
		t.Fatalf("automatic layer = %q, want auto", got)
	}
	if got := effectiveFishingTargetLayer(""); got != "" {
		t.Fatalf("legacy runtime without a layer changed to %q", got)
	}
}

func TestFishingPaceTradesTimeForBaitEfficiency(t *testing.T) {
	spot := catalog.FishingSpot{ID: "spot", WaterID: "water", BaseBiteRate: 1}
	loadout := FishingLoadoutSnapshot{BiteMultiplier: 1}
	conditions := FishingConditions{Activity: 1}
	patientWait := fishingWaitDuration(spot, loadout, fishingPacePatient, 0, conditions, rand.New(rand.NewSource(9)))
	balancedWait := fishingWaitDuration(spot, loadout, fishingPaceBalanced, 0, conditions, rand.New(rand.NewSource(9)))
	searchWait := fishingWaitDuration(spot, loadout, fishingPaceSearch, 0, conditions, rand.New(rand.NewSource(9)))
	if !(patientWait > balancedWait && balancedWait > searchWait) {
		t.Fatalf("pace waits patient=%v balanced=%v search=%v", patientWait, balancedWait, searchWait)
	}
	patientBite := fishingBiteChance(spot, loadout, fishingPacePatient, 0, conditions, 1)
	balancedBite := fishingBiteChance(spot, loadout, fishingPaceBalanced, 0, conditions, 1)
	searchBite := fishingBiteChance(spot, loadout, fishingPaceSearch, 0, conditions, 1)
	if !(patientBite > balancedBite && balancedBite > searchBite) {
		t.Fatalf("pace bite chances patient=%v balanced=%v search=%v", patientBite, balancedBite, searchBite)
	}
	if got := effectiveFishingPace("", "conservative"); got != "legacy_conservative" {
		t.Fatalf("legacy conservative pace = %q", got)
	}
}

func TestFishingQueuePreviewsGearAndStartAppliesItBeforeFreezing(t *testing.T) {
	executorRaw, err := os.ReadFile("action_executor_fishing.go")
	if err != nil {
		t.Fatal(err)
	}
	executorBody := string(executorRaw)
	freezeStart := strings.Index(executorBody, "func (e *fishingExecutor) FreezeRunContextTx")
	startRunStart := strings.Index(executorBody, "func (e *fishingExecutor) StartRunTx")
	cancelStart := strings.Index(executorBody, "func (e *fishingExecutor) CancelRunTx")
	if freezeStart < 0 || startRunStart <= freezeStart || cancelStart <= startRunStart {
		t.Fatal("fishing executor function boundaries not found")
	}
	freezeBody := executorBody[freezeStart:startRunStart]
	if !strings.Contains(freezeBody, "previewFishingLoadoutForActorTx") {
		t.Fatal("queue validation no longer previews the selected angler wearing the tackle set")
	}
	startBody := executorBody[startRunStart:cancelStart]
	applyAt := strings.Index(startBody, "applyFishingLoadoutEquipmentForActorTx")
	freezeAt := strings.Index(startBody, "freezeFishingLoadoutForActorTx")
	reserveAt := strings.Index(startBody, "reserveFishingSuppliesTx")
	insertAt := strings.Index(startBody, "INSERT INTO fishing_trips")
	if applyAt < 0 || freezeAt <= applyAt || reserveAt <= freezeAt || insertAt <= reserveAt {
		t.Fatal("trip start must equip, freeze, reserve supplies, and persist the trip in that order")
	}
}

func TestFishingGearPreviewAlwaysRollsBack(t *testing.T) {
	raw, err := os.ReadFile("store_fishing.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) previewFishingLoadoutForActorTx")
	end := strings.Index(body, "func (s *Store) applyFishingLoadoutEquipmentForActorTx")
	if start < 0 || end <= start {
		t.Fatal("fishing gear preview function not found")
	}
	previewBody := body[start:end]
	applyAt := strings.Index(previewBody, "applyFishingLoadoutEquipmentForActorTx")
	freezeAt := strings.Index(previewBody, "freezeFishingLoadoutForActorTx")
	rollbackAt := strings.LastIndex(previewBody, "Rollback(ctx)")
	if applyAt < 0 || freezeAt <= applyAt || rollbackAt <= freezeAt || strings.Contains(previewBody, ".Commit(ctx)") {
		t.Fatal("fishing gear preview must trial-equip, freeze, and roll back without committing equipment")
	}
}

func TestFishingStartCancelsWhenReservedTackleCanNoLongerMove(t *testing.T) {
	executor := newFishingExecutor(&Store{})
	for _, err := range []error{
		ErrInstanceNotOwned,
		ErrInvalidInstanceState,
		ErrFishingTackleInUse,
		ErrEquipTargetNotAllowed,
		ErrKittyNotFound,
	} {
		cancel, reason := executor.ClassifyStartError(err)
		if !cancel || reason != ActionQueueCancelReasonTargetInvalid {
			t.Fatalf("ClassifyStartError(%v)=(%v,%q), want target-invalid cancellation", err, cancel, reason)
		}
	}
}

func TestFishingSupplyShortageIdentifiesExactItem(t *testing.T) {
	loadout := FishingLoadoutSnapshot{Components: FishingLoadoutComponents{
		MainLineItemID: "z_main_line",
		LeaderItemID:   "m_leader",
		BaitItemID:     "a_bait",
	}}
	entries := sortedFishingSupplyEntries(loadout, FishingSupplyCounts{
		MainLine: 8,
		Leader:   8,
		Bait:     12,
	})
	shortage := firstFishingSupplyShortage(entries, map[string]int64{
		"z_main_line": 8,
		"m_leader":    8,
		"a_bait":      11,
	})
	if shortage == nil {
		t.Fatal("expected fishing supply shortage")
	}
	if shortage.ItemID != "a_bait" || shortage.Required != 12 || shortage.Available != 11 {
		t.Fatalf("shortage = %+v, want a_bait required=12 available=11", shortage)
	}
	if !errors.Is(shortage, ErrInsufficientItem) {
		t.Fatalf("errors.Is(shortage, ErrInsufficientItem) = false")
	}
}

func TestFishingConditionsFollowFourHourDay(t *testing.T) {
	water := catalog.FishingWater{ID: "test_water", BaseTemperatureC: 18, WaterKind: "temperate_river"}
	spot := catalog.FishingSpot{ID: "test_spot", WaterID: water.ID, Oxygen: 0.84, BaseBiteRate: 1}
	start := time.Unix(1_800_000_000, 0).UTC()
	initial := fishingConditionsAt(water, spot, start)
	fullDay := fishingConditionsAt(water, spot, start.Add(4*time.Hour))
	if initial.GameHour != fullDay.GameHour {
		t.Fatalf("game hour changed after one four-hour day: before=%d after=%d", initial.GameHour, fullDay.GameHour)
	}
	if initial.Season != fullDay.Season {
		t.Fatalf("season changed within one real day: before=%s after=%s", initial.Season, fullDay.Season)
	}
	seasonLater := fishingConditionsAt(water, spot, start.Add(7*24*time.Hour))
	if seasonLater.Season == initial.Season {
		t.Fatalf("season did not advance after seven real days: %s", initial.Season)
	}
}

func TestFishingTripRulesStayFrozenAfterStart(t *testing.T) {
	water := catalog.FishingWater{ID: "water", BaseTemperatureC: 18, WaterKind: "temperate_river"}
	spot := catalog.FishingSpot{ID: "spot", WaterID: water.ID, BaseBiteRate: 1, SpeciesWeights: map[string]float64{"trout": 1}}
	store := &Store{
		gameData: catalog.Data{FishingSpecies: []catalog.FishingSpecies{{ID: "trout", WaterID: water.ID, FightPower: 1.4}}},
		config: RuntimeConfig{Fishing: config.FishingSettings{Mutations: []config.FishingMutationSettings{{
			ID: "prismatic", AppearanceChance: 0.25, Alchemy: config.FishingMutationAlchemySettings{BonusAttributes: []config.FishingMutationAlchemyBonusAttribute{{AttrKey: "luck", Min: 1, Max: 2}}},
		}}}},
	}
	rules := newFishingExecutor(store).rulesSnapshot(water, spot)
	store.gameData.FishingSpecies[0].FightPower = 9
	store.config.Fishing.Mutations[0].AppearanceChance = 1
	store.config.Fishing.Mutations[0].Alchemy.BonusAttributes[0].Min = 99
	if got := rules.speciesByID("trout"); got == nil || got.FightPower != 1.4 {
		t.Fatalf("running trip rules changed with the live catalog: %+v", got)
	}
	if len(rules.Mutations) != 1 || rules.Mutations[0].AppearanceChance != 0.25 || rules.Mutations[0].Alchemy.BonusAttributes[0].Min != 1 {
		t.Fatalf("running trip mutations changed with live config: %+v", rules.Mutations)
	}
}

func TestGenerateFishingIndividualStaysWithinBiology(t *testing.T) {
	species := catalog.FishingSpecies{
		ID: "test_carp", MinLengthCM: 12, MaxLengthCM: 84,
		MaturityLengthCM: 34, MaturityAgeDays: 730, LifespanDays: 7300,
		ShapeFactor: 1.15, FightPower: 1.3,
	}
	rng := rand.New(rand.NewSource(424242))
	for i := 0; i < 2_000; i++ {
		fish := generateFishingIndividual(species, "water", "spot", FishingRulesetVersion, time.Unix(1_800_000_000, 0), 0.3, rng)
		if fish.LengthCM < species.MinLengthCM || fish.LengthCM > species.MaxLengthCM {
			t.Fatalf("length outside species range: %.2f", fish.LengthCM)
		}
		if fish.WeightKG <= 0 || fish.Condition < 0.82 || fish.Condition > 1.12 || fish.Health < 0.78 || fish.Health > 1 {
			t.Fatalf("invalid fish condition: %+v", fish)
		}
		if fish.AgeDays <= 0 || (fish.Sex != "male" && fish.Sex != "female") {
			t.Fatalf("invalid individual identity: %+v", fish)
		}
		if fish.AgeDays >= species.LifespanDays {
			t.Fatalf("wild fish exceeds species lifespan: %+v", fish)
		}
		if fish.LengthCM > species.MaturityLengthCM && fish.AgeDays < species.MaturityAgeDays {
			t.Fatalf("mature-sized fish is younger than the species maturity age: %+v", fish)
		}
	}
}

func TestFishingLineBreakStartsAboveDisplayedRigRating(t *testing.T) {
	species := catalog.FishingSpecies{ID: "strong_fish", FightPower: 3}
	loadout := FishingLoadoutSnapshot{MaxLoadKG: 8, Control: 2, Sensitivity: 2, Hooking: 2, FishingLevel: 20}
	withinRating := FishInstanceData{WeightKG: 5}
	for seed := int64(0); seed < 2_000; seed++ {
		result := resolveFishingFight(species, withinRating, loadout, "aggressive", 70, rand.New(rand.NewSource(seed)))
		if result.LineLoad > loadout.MaxLoadKG {
			t.Fatalf("test fixture exceeds displayed load: %+v", result)
		}
		if result.LineBreak {
			t.Fatalf("line broke below displayed rating on seed %d: %+v", seed, result)
		}
	}
	overloaded := FishInstanceData{WeightKG: 12}
	breaks := 0
	for seed := int64(0); seed < 2_000; seed++ {
		if resolveFishingFight(species, overloaded, loadout, "balanced", 82, rand.New(rand.NewSource(seed))).LineBreak {
			breaks++
		}
	}
	if breaks == 0 {
		t.Fatal("an overloaded rig never broke")
	}
}

func TestLegacyFishingLineBreakKeepsSafetyThresholdReason(t *testing.T) {
	species := catalog.FishingSpecies{ID: "legacy_fish", FightPower: 1}
	loadout := FishingLoadoutSnapshot{MaxLoadKG: 8}
	fish := FishInstanceData{WeightKG: 7}
	for seed := int64(0); seed < 2_000; seed++ {
		result := resolveFishingFightForRuleset(species, fish, loadout, "balanced", 50, fishingLegacyRulesetVersion, rand.New(rand.NewSource(seed)))
		if result.LineBreak {
			if result.LineLoad >= loadout.MaxLoadKG {
				t.Fatalf("fixture must stay below the displayed rating: %+v", result)
			}
			if result.FailureReason != "line_overload" {
				t.Fatalf("legacy overload reason changed: %+v", result)
			}
			return
		}
	}
	t.Fatal("legacy safety overload never produced a line break")
}

func TestFishingPressureDecaysWithSixHourHalfLife(t *testing.T) {
	got := decayFishingPressure(0.8, 6*time.Hour)
	if got < 0.399 || got > 0.401 {
		t.Fatalf("pressure half-life mismatch: got=%f want=0.4", got)
	}
	if got := decayFishingPressure(0.8, 12*time.Hour); got < 0.199 || got > 0.201 {
		t.Fatalf("pressure twelve-hour decay mismatch: got=%f want=0.2", got)
	}
}

func TestFishingForecastShowsPersonalSpotPressure(t *testing.T) {
	conditions := FishingConditions{Activity: 1.2, ActivityBand: "busy"}
	if got := fishingForecastConditions(conditions, 0.8); got.ActivityBand != "quiet" {
		t.Fatalf("high personal pressure should make the spot quiet, got=%+v", got)
	}
	if got := fishingForecastConditions(conditions, 0); got.ActivityBand != "busy" {
		t.Fatalf("fresh spot should preserve busy activity, got=%+v", got)
	}
}

func TestFishingWaitAndFightStayInsideSafetyBounds(t *testing.T) {
	spot := catalog.FishingSpot{ID: "spot", WaterID: "water", BaseBiteRate: 1, Oxygen: 0.9}
	loadout := FishingLoadoutSnapshot{Control: 2.4, Sensitivity: 2.1, Efficiency: 0.11, MaxLoadKG: 8, BiteMultiplier: 1.1, FishingLevel: 12}
	conditions := FishingConditions{Activity: 1.08}
	rng := rand.New(rand.NewSource(77))
	for i := 0; i < 500; i++ {
		wait := fishingWaitDuration(spot, loadout, "balanced", 0.2, conditions, rng)
		if wait < 28*time.Second || wait > 210*time.Second {
			t.Fatalf("wait duration outside contract: %s", wait)
		}
	}
	baseWait := fishingWaitDuration(spot, FishingLoadoutSnapshot{BiteMultiplier: 1}, "balanced", 0.2, conditions, rand.New(rand.NewSource(23)))
	efficientWait := fishingWaitDuration(spot, FishingLoadoutSnapshot{BiteMultiplier: 1, Efficiency: 0.5}, "balanced", 0.2, conditions, rand.New(rand.NewSource(23)))
	if efficientWait >= baseWait {
		t.Fatalf("fishing efficiency should shorten the same seeded wait: base=%s efficient=%s", baseWait, efficientWait)
	}
	species := catalog.FishingSpecies{ID: "pike", FightPower: 2.6}
	fish := FishInstanceData{WeightKG: 9.5}
	conservativeBreaks := 0
	aggressiveBreaks := 0
	for seed := int64(0); seed < 10_000; seed++ {
		if resolveFishingFight(species, fish, loadout, "conservative", 82, rand.New(rand.NewSource(seed))).LineBreak {
			conservativeBreaks++
		}
		if resolveFishingFight(species, fish, loadout, "aggressive", 82, rand.New(rand.NewSource(seed))).LineBreak {
			aggressiveBreaks++
		}
	}
	if aggressiveBreaks <= conservativeBreaks {
		t.Fatalf("aggressive strategy should carry more line-break risk: aggressive=%d conservative=%d", aggressiveBreaks, conservativeBreaks)
	}
}

func TestFishingOnlineAndOfflineUseSameDeterministicStream(t *testing.T) {
	species := catalog.FishingSpecies{ID: "trout", MinLengthCM: 18, MaxLengthCM: 92, ShapeFactor: 1.02, FightPower: 1.55}
	loadout := FishingLoadoutSnapshot{Control: 2.3, Sensitivity: 1.8, MaxLoadKG: 9.5, FishingLevel: 20}
	simulate := func(batch int) (landed, broken int, weight float64) {
		runtime := fishingExecutorRuntime{Seed: 918273645}
		remaining := 6_000
		for remaining > 0 {
			step := batch
			if step > remaining {
				step = remaining
			}
			for i := 0; i < step; i++ {
				rng := fishingRuntimeRNG(&runtime)
				fish := generateFishingIndividual(species, "water", "spot", FishingRulesetVersion, time.Unix(1_800_000_000, 0), 0.25, rng)
				fight := resolveFishingFight(species, fish, loadout, "balanced", 82, rng)
				if fight.LineBreak {
					broken++
				} else if fight.Landed {
					landed++
					weight += fish.WeightKG
				}
			}
			remaining -= step
		}
		return landed, broken, roundFloat(weight, 3)
	}
	onlineLanded, onlineBroken, onlineWeight := simulate(1)
	offlineLanded, offlineBroken, offlineWeight := simulate(2_000)
	if onlineLanded != offlineLanded || onlineBroken != offlineBroken || onlineWeight != offlineWeight {
		t.Fatalf("batch mode changed fishing outcome: online=(%d,%d,%.3f) offline=(%d,%d,%.3f)", onlineLanded, onlineBroken, onlineWeight, offlineLanded, offlineBroken, offlineWeight)
	}
	budgetLanded, budgetBroken, budgetWeight := simulate(fishingFastForwardPhaseLimit(FastForwardBudget{MaxIterations: 37}))
	if onlineLanded != budgetLanded || onlineBroken != budgetBroken || onlineWeight != budgetWeight {
		t.Fatalf("budgeted batching changed fishing outcome: online=(%d,%d,%.3f) budgeted=(%d,%d,%.3f)", onlineLanded, onlineBroken, onlineWeight, budgetLanded, budgetBroken, budgetWeight)
	}
}

func TestFishingFastForwardPhaseLimitHonorsBudget(t *testing.T) {
	if got := fishingFastForwardPhaseLimit(FastForwardBudget{}); got != fishingMaxFastForwardPhases {
		t.Fatalf("default phase limit = %d, want %d", got, fishingMaxFastForwardPhases)
	}
	if got := fishingFastForwardPhaseLimit(FastForwardBudget{MaxIterations: 37}); got != 37 {
		t.Fatalf("budgeted phase limit = %d, want 37", got)
	}
	if got := fishingFastForwardPhaseLimit(FastForwardBudget{MaxIterations: fishingMaxFastForwardPhases + 1}); got != fishingMaxFastForwardPhases {
		t.Fatalf("hard-capped phase limit = %d, want %d", got, fishingMaxFastForwardPhases)
	}
}

func TestFishingFastForwardBudgetYieldsOnlyAfterProgress(t *testing.T) {
	started := time.Now().Add(-2 * time.Millisecond)
	budget := FastForwardBudget{MaxExec: time.Millisecond}
	if shouldYieldFishingFastForwardBudget(started, budget, 0) {
		t.Fatal("budget must allow one transition before yielding")
	}
	if !shouldYieldFishingFastForwardBudget(started, budget, 1) {
		t.Fatal("budget should yield after progress and max exec elapses")
	}
	if shouldYieldFishingFastForwardBudget(started, FastForwardBudget{}, 1) {
		t.Fatal("zero max exec should not yield")
	}
}

func TestFishingFastForwardCatchBatchPreservesSequentialRecords(t *testing.T) {
	species := catalog.FishingSpecies{ID: "trout", Rarity: "rare"}
	batch := newFishingFastForwardCatchBatch(fishingRulesSnapshot{
		RulesetVersion: FishingRulesetVersion,
		Species:        []catalog.FishingSpecies{species},
	})
	batch.Knowledge[species.ID] = &fishingBatchKnowledgeState{}
	batch.Records[species.ID] = &fishingBatchRecordState{BestLengthCM: 20, BestWeightKG: 1}
	rule := FishingKeepRule{Mode: "record_only"}
	start := time.Unix(1_800_000_000, 0)

	first := batch.prepareCatch(species, FishInstanceData{
		SpeciesID: species.ID, LengthCM: 30, WeightKG: 2, RulesetVersion: FishingRulesetVersion,
	}, rule, start)
	second := batch.prepareCatch(species, FishInstanceData{
		SpeciesID: species.ID, LengthCM: 29, WeightKG: 3, RulesetVersion: FishingRulesetVersion,
	}, rule, start.Add(time.Minute))
	third := batch.prepareCatch(species, FishInstanceData{
		SpeciesID: species.ID, LengthCM: 25, WeightKG: 1.5, RulesetVersion: FishingRulesetVersion,
	}, rule, start.Add(2*time.Minute))

	if first == nil || !first.IsNew || !first.Kept || !first.Record.Length || !first.Record.Weight || first.Gain != 29 {
		t.Fatalf("first catch mismatch: %+v", first)
	}
	if second == nil || second.IsNew || !second.Kept || second.Record.Length || !second.Record.Weight || second.Gain != 14 {
		t.Fatalf("second catch mismatch: %+v", second)
	}
	if third == nil || third.IsNew || third.Kept || third.Record.Length || third.Record.Weight || third.Gain != 6 {
		t.Fatalf("third catch mismatch: %+v", third)
	}
	record := batch.Records[species.ID]
	if record == nil || record.BestLengthCM != 30 || record.BestWeightKG != 3 || !record.Dirty {
		t.Fatalf("batched record mismatch: %+v", record)
	}
	knowledge := batch.Knowledge[species.ID]
	if knowledge == nil || knowledge.EncounterDelta != 1 || knowledge.CatchDelta != 3 || knowledge.ReleaseDelta != 1 || knowledge.FirstCaughtAt == nil || !knowledge.FirstCaughtAt.Equal(start) || knowledge.LastCaughtAt == nil || !knowledge.LastCaughtAt.Equal(start.Add(2*time.Minute)) {
		t.Fatalf("batched knowledge mismatch: %+v", knowledge)
	}
}

func TestFishingFastForwardCatchBatchHasBoundedPersistence(t *testing.T) {
	species := catalog.FishingSpecies{ID: "trout"}
	batch := newFishingFastForwardCatchBatch(fishingRulesSnapshot{
		RulesetVersion: FishingRulesetVersion,
		Species:        []catalog.FishingSpecies{species},
	})
	for index := 0; index < fishingMaxFastForwardCatches; index++ {
		batch.prepareCatch(species, FishInstanceData{
			SpeciesID: species.ID, LengthCM: float64(20 + index), WeightKG: float64(index + 1), RulesetVersion: FishingRulesetVersion,
		}, FishingKeepRule{Mode: "keep_all"}, time.Unix(int64(index+1), 0))
		if index < fishingMaxFastForwardCatches-1 && batch.full() {
			t.Fatalf("batch reported full after %d catches", index+1)
		}
	}
	if !batch.full() || len(batch.Pending) != fishingMaxFastForwardCatches {
		t.Fatalf("batch cap mismatch: full=%v pending=%d", batch.full(), len(batch.Pending))
	}
}

func TestFishingTournamentPendingSelectionUsesPersistedTarget(t *testing.T) {
	startsAt := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	tournament := FishingTournament{
		WeekKey:         fishingTournamentEntryRepairWeek,
		TargetSpeciesID: fishingTournamentEntryRepairTarget,
		RulesetVersion:  FishingRulesetVersion,
		StartsAt:        startsAt,
		EndsAt:          startsAt.Add(7 * 24 * time.Hour),
	}
	pending := []*fishingPendingCatch{
		{Fish: FishInstanceData{SpeciesID: "bluegill"}, CaughtAt: startsAt.Add(time.Hour)},
		{Fish: FishInstanceData{SpeciesID: fishingTournamentEntryRepairTarget}, CaughtAt: startsAt.Add(2 * time.Hour)},
		{Fish: FishInstanceData{SpeciesID: fishingTournamentEntryRepairTarget}, CaughtAt: startsAt.Add(-time.Second)},
		{Fish: FishInstanceData{SpeciesID: fishingTournamentEntryRepairTarget}, CaughtAt: tournament.EndsAt},
	}

	eligible := fishingTournamentEligiblePendingCatches(tournament, pending)
	if len(eligible) != 1 || eligible[0] != pending[1] {
		t.Fatalf("eligible tournament catches=%+v, want only the in-window Emberfin catch", eligible)
	}
}

func TestFishingCatchTalliesMergeAcrossOfflineBatches(t *testing.T) {
	first := map[string]FishingCatchTally{}
	addFishingCatchTally(first, "trout", true, 2.125, []string{"rainbow", "prismatic", "rainbow"})
	addFishingCatchTally(first, "trout", false, 1.25, nil)
	second := map[string]FishingCatchTally{}
	addFishingCatchTally(second, "trout", true, 3.375, []string{"prismatic", "rainbow"})
	addFishingCatchTally(second, "carp", true, 4.5, []string{"shimmering"})
	mergeFishingCatchTallies(first, second)

	if got := first["trout"]; got.Landed != 3 || got.Kept != 2 || got.Released != 1 || got.TotalWeightKG != 6.75 || got.MutatedLanded != 2 || got.MutatedKept != 2 {
		t.Fatalf("merged trout tally mismatch: %+v", got)
	}
	if got := first["trout"].MutationGroups; len(got) != 1 || got[0].Landed != 2 || !reflect.DeepEqual(got[0].Mutations, []string{"prismatic", "rainbow"}) {
		t.Fatalf("merged trout mutation groups mismatch: %+v", got)
	}
	if got := first["carp"]; got.Landed != 1 || got.Kept != 1 || got.Released != 0 || got.TotalWeightKG != 4.5 {
		t.Fatalf("merged carp tally mismatch: %+v", got)
	}
}

func TestFishingCatchTallyCapsMutationGroupsWithoutLosingTotals(t *testing.T) {
	tallies := map[string]FishingCatchTally{}
	for index := 0; index < fishingCatchMutationGroupLimit+3; index++ {
		addFishingCatchTally(tallies, "trout", true, 1, []string{fmt.Sprintf("mutation_%02d", index)})
	}

	got := tallies["trout"]
	if got.MutatedLanded != fishingCatchMutationGroupLimit+3 || got.MutatedKept != fishingCatchMutationGroupLimit+3 {
		t.Fatalf("mutation totals were truncated with groups: %+v", got)
	}
	if len(got.MutationGroups) != fishingCatchMutationGroupLimit {
		t.Fatalf("mutation group count = %d, want %d", len(got.MutationGroups), fishingCatchMutationGroupLimit)
	}
}

func TestFishingSupplyPackMatchesMethodAndLineBreakLoss(t *testing.T) {
	floatPack := FishingSupplyCounts{MainLine: 8, Leader: 8, Hook: 12, FloatOrSinker: 4, Bait: 120}
	if err := validateFishingSuppliesForLoadout(floatPack, "float"); err != nil {
		t.Fatalf("float supply pack rejected: %v", err)
	}
	if err := validateFishingSuppliesForLoadout(floatPack, "lure"); err == nil {
		t.Fatal("float supply pack unexpectedly accepted for lure fishing")
	}
	lurePack := FishingSupplyCounts{MainLine: 5, Leader: 5, Lure: 9}
	if err := validateFishingSuppliesForLoadout(lurePack, "lure"); err != nil {
		t.Fatalf("lure supply pack rejected: %v", err)
	}

	supplies := FishingTripSupplies{
		Initial: floatPack, Remaining: floatPack,
		Wear: FishingSupplyWear{MainLine: 0.72, Leader: 0.91},
	}
	loadout := FishingLoadoutSnapshot{Method: "float"}
	consumeFishingTerminalSupplies(&supplies, loadout, true)
	if supplies.Remaining.MainLine != 7 || supplies.Remaining.Leader != 7 || supplies.Remaining.Hook != 11 || supplies.Remaining.FloatOrSinker != 3 || supplies.Remaining.Bait != 120 {
		t.Fatalf("line break consumed the wrong supply set: %+v", supplies.Remaining)
	}
	if supplies.Consumed.MainLine != 1 || supplies.Consumed.Leader != 1 || supplies.Consumed.Hook != 1 || supplies.Consumed.FloatOrSinker != 1 {
		t.Fatalf("line break accounting mismatch: %+v", supplies.Consumed)
	}
	if supplies.Wear.MainLine != 0 || supplies.Wear.Leader != 0 {
		t.Fatalf("line break did not reset replaced line wear: %+v", supplies.Wear)
	}
}

func TestFishingTackleWearConsumesDeterministicallyAcrossFights(t *testing.T) {
	loadout := FishingLoadoutSnapshot{
		Components: FishingLoadoutComponents{MainLineItemID: "braided_main_line", LeaderItemID: "clear_leader"},
		FrozenComponents: []FishingFrozenComponent{
			{Slot: "main_line", ItemID: "braided_main_line", WearPerFight: 0.08},
			{Slot: "leader", ItemID: "clear_leader", WearPerFight: 0.2},
		},
	}
	species := catalog.FishingSpecies{MainLineWearMultiplier: 1, LeaderWearMultiplier: 1}
	supplies := FishingTripSupplies{Remaining: FishingSupplyCounts{MainLine: 20, Leader: 20}}
	costs := map[string]int64{}
	for fight := 0; fight < 13; fight++ {
		for itemID, quantity := range applyFishingTackleWear(&supplies, loadout, species) {
			costs[itemID] += quantity
		}
	}
	if supplies.Remaining.MainLine != 19 || supplies.Consumed.MainLine != 1 || costs["braided_main_line"] != 1 {
		t.Fatalf("main-line wear settlement mismatch: supplies=%+v costs=%v", supplies, costs)
	}
	if supplies.Remaining.Leader != 18 || supplies.Consumed.Leader != 2 || costs["clear_leader"] != 2 {
		t.Fatalf("leader wear settlement mismatch: supplies=%+v costs=%v", supplies, costs)
	}
	if math.Abs(supplies.Wear.MainLine-0.04) > 1e-9 || math.Abs(supplies.Wear.Leader-0.6) > 1e-9 {
		t.Fatalf("fractional wear did not carry forward: %+v", supplies.Wear)
	}
}

func TestFishingSpeciesWearMultipliersDifferentiateLeaderPressure(t *testing.T) {
	loadout := FishingLoadoutSnapshot{
		Components: FishingLoadoutComponents{MainLineItemID: "braided_main_line", LeaderItemID: "clear_leader"},
		FrozenComponents: []FishingFrozenComponent{
			{Slot: "main_line", WearPerFight: 0.08},
			{Slot: "leader", WearPerFight: 0.2},
		},
	}
	supplies := FishingTripSupplies{Remaining: FishingSupplyCounts{MainLine: 8, Leader: 8}}
	toothy := catalog.FishingSpecies{MainLineWearMultiplier: 1, LeaderWearMultiplier: 1.8}
	for fight := 0; fight < 3; fight++ {
		applyFishingTackleWear(&supplies, loadout, toothy)
	}
	if supplies.Consumed.MainLine != 0 || math.Abs(supplies.Wear.MainLine-0.24) > 1e-9 {
		t.Fatalf("toothy fish changed main-line pressure unexpectedly: %+v", supplies)
	}
	if supplies.Consumed.Leader != 1 || math.Abs(supplies.Wear.Leader-0.08) > 1e-9 {
		t.Fatalf("toothy fish did not accelerate leader wear: %+v", supplies)
	}
}

func TestFishingTackleWearDoesNotCarryPastTheLastSpare(t *testing.T) {
	remaining := int64(1)
	consumed := int64(0)
	progress := 0.9
	if used := consumeFishingWear(&remaining, &consumed, &progress, 0.2); used != 1 {
		t.Fatalf("used=%d, want 1", used)
	}
	if remaining != 0 || consumed != 1 || progress != 0 {
		t.Fatalf("last-spare wear leaked onto a future item: remaining=%d consumed=%d progress=%.3f", remaining, consumed, progress)
	}
}

func TestFishingManualStopKeepsMainLineWearAndReplacesLeader(t *testing.T) {
	supplies := FishingTripSupplies{
		Remaining: FishingSupplyCounts{MainLine: 4, Leader: 4, Hook: 4, FloatOrSinker: 4},
		Wear:      FishingSupplyWear{MainLine: 0.48, Leader: 0.76},
	}
	consumeFishingTerminalSupplies(&supplies, FishingLoadoutSnapshot{Method: "float"}, false)
	if supplies.Remaining.MainLine != 4 || supplies.Wear.MainLine != 0.48 {
		t.Fatalf("manual stop should preserve main-line wear: %+v", supplies)
	}
	if supplies.Remaining.Leader != 3 || supplies.Wear.Leader != 0 {
		t.Fatalf("manual stop should replace the leader and reset its wear: %+v", supplies)
	}
}

func TestFishingWearBalanceDataMatchesContract(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated fishing data: %v", err)
	}
	wantTackleWear := map[string]float64{
		"braided_main_line":  0.08,
		"stretch_mono_line":  0.05,
		"cushion_braid_line": 0.12,
		"trophy_core_line":   0.04,
		"clear_leader":       0.2,
		"wire_leader":        0.1,
		"shock_leader":       0.16,
	}
	for itemID, want := range wantTackleWear {
		definition := data.FishingTackleByItemID(itemID)
		if definition == nil || math.Abs(definition.WearPerFight-want) > 1e-9 {
			t.Fatalf("%s wearPerFight=%v, want %.3f", itemID, definition, want)
		}
	}
	wantSpeciesWear := map[string][2]float64{
		"crucian_carp": {0.65, 0.65}, "common_carp": {1.2, 1.25}, "bluegill": {0.65, 0.65},
		"grass_carp": {1.2, 1.25}, "mud_loach": {0.65, 0.65}, "snakehead": {1, 1.8},
		"lotus_koi": {1.2, 1.25}, "emberfin": {1.35, 1.5}, "brown_trout": {1.2, 1.25},
		"chub": {1, 1}, "dace": {0.65, 0.65}, "barbel": {1.2, 1.25},
		"freshwater_perch": {1, 1}, "river_eel": {1, 1.8}, "glassback_darter": {1.35, 1.5},
		"thunder_whisker": {1.35, 1.5}, "lake_trout": {1.2, 1.25}, "arctic_char": {1.2, 1.25},
		"burbot": {1.2, 1.25}, "whitefish": {0.65, 0.65}, "northern_pike": {1, 1.8},
		"grayling": {1, 1}, "moonmirror_carp": {1.5, 1.75}, "frostveil_sturgeon": {1.5, 1.75},
	}
	if len(data.FishingSpecies) != len(wantSpeciesWear) {
		t.Fatalf("fishing species count=%d, want %d", len(data.FishingSpecies), len(wantSpeciesWear))
	}
	for _, species := range data.FishingSpecies {
		want, ok := wantSpeciesWear[species.ID]
		if !ok {
			t.Fatalf("missing wear contract for species %s", species.ID)
		}
		if math.Abs(species.MainLineWearMultiplier-want[0]) > 1e-9 || math.Abs(species.LeaderWearMultiplier-want[1]) > 1e-9 {
			t.Fatalf("species %s wear multipliers=(%.2f,%.2f), want=(%.2f,%.2f)", species.ID, species.MainLineWearMultiplier, species.LeaderWearMultiplier, want[0], want[1])
		}
	}
}

func TestNewFishingTackleAreParetoTradeoffs(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated fishing data: %v", err)
	}
	newTackle := map[string]bool{
		"cushion_braid_line": true, "trophy_core_line": true, "shock_leader": true,
		"circle_hook": true, "quill_float": true, "distance_sinker": true,
	}
	supportsEveryMethod := func(candidate, other catalog.FishingTackleDefinition) bool {
		methods := make(map[string]bool, len(candidate.Methods))
		for _, method := range candidate.Methods {
			methods[method] = true
		}
		for _, method := range other.Methods {
			if !methods[method] {
				return false
			}
		}
		return true
	}
	benefits := func(row catalog.FishingTackleDefinition) []float64 {
		biteMultiplier := row.BiteMultiplier
		if biteMultiplier <= 0 {
			biteMultiplier = 1
		}
		return []float64{row.Control, row.Sensitivity, row.Casting, row.Hooking, row.Efficiency, row.MaxLoadKG, biteMultiplier, -row.WearPerFight}
	}
	for _, candidate := range data.FishingTackle {
		for _, other := range data.FishingTackle {
			if candidate.ItemID == other.ItemID || candidate.Slot != other.Slot || (!newTackle[candidate.ItemID] && !newTackle[other.ItemID]) || !supportsEveryMethod(candidate, other) {
				continue
			}
			candidateBenefits, otherBenefits := benefits(candidate), benefits(other)
			allAtLeastAsGood, anyBetter := true, false
			for index := range candidateBenefits {
				if candidateBenefits[index]+1e-9 < otherBenefits[index] {
					allAtLeastAsGood = false
					break
				}
				if candidateBenefits[index] > otherBenefits[index]+1e-9 {
					anyBetter = true
				}
			}
			if allAtLeastAsGood && anyBetter {
				t.Fatalf("%s is a straight upgrade over %s; new tackle must keep a tradeoff", candidate.ItemID, other.ItemID)
			}
		}
	}
}

func TestFishingScaleglowUsesIndependentDeterministicStream(t *testing.T) {
	const seed = int64(918273645)
	baseline := fishingExecutorRuntime{Seed: seed}
	withDrops := fishingExecutorRuntime{Seed: seed}
	for ordinal := int64(1); ordinal <= 500; ordinal++ {
		want := fishingRuntimeRNG(&baseline).Int63()
		_ = fishingScaleglowDrop(seed, ordinal)
		got := fishingRuntimeRNG(&withDrops).Int63()
		if got != want || withDrops.EventSeq != baseline.EventSeq {
			t.Fatalf("drop roll changed fishing stream at catch %d: got=%d want=%d", ordinal, got, want)
		}
		if fishingScaleglowDrop(seed, ordinal) != fishingScaleglowDrop(seed, ordinal) {
			t.Fatalf("drop roll is not deterministic at catch %d", ordinal)
		}
	}
}

func TestFishingMarketMultiplierIsCapped(t *testing.T) {
	for demand := 0.7; demand <= 1.4; demand += 0.01 {
		for supply := 0.6; supply <= 1.4; supply += 0.02 {
			value := fishingSaleMarketMultiplier(demand, supply)
			if value < fishingSaleMarketMinimum || value > fishingSaleMarketMaximum {
				t.Fatalf("market multiplier escaped cap: %f", value)
			}
		}
	}
}

func TestFishingLegacyStandardTournamentUsesOneFixedRig(t *testing.T) {
	rules := fishingStandardTournamentRules()
	loadout := rules.loadoutSnapshot()
	if rules.Strategy != "balanced" || loadout.FishingLevel != 20 || loadout.Control != 2.4 || loadout.Sensitivity != 1.8 || loadout.Hooking != 0 || loadout.MaxLoadKG != 9.5 {
		t.Fatalf("published standard rules do not match the tournament loadout: rules=%+v loadout=%+v", rules, loadout)
	}

	species := catalog.FishingSpecies{ID: "test_pike", FightPower: 2.4}
	fish := FishInstanceData{SpeciesID: species.ID, WeightKG: 7.8}
	seenLanded := false
	seenMissed := false
	for catchID := int64(1); catchID <= 500; catchID++ {
		first := fishingStandardTournamentQualifies("weekly_test", catchID, species, fish)
		second := fishingStandardTournamentQualifies("weekly_test", catchID, species, fish)
		if first != second {
			t.Fatalf("standard result changed for catch %d", catchID)
		}
		seenLanded = seenLanded || first
		seenMissed = seenMissed || !first
	}
	if !seenLanded || !seenMissed {
		t.Fatalf("fixed standard rig should produce both qualifying and non-qualifying fish: landed=%v missed=%v", seenLanded, seenMissed)
	}
}

func TestFishingTournamentStandardLoadoutUsesExactExistingItemIDs(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load game data: %v", err)
	}
	store := &Store{gameData: data}
	standard, err := store.validatedFishingTournamentStandardLoadout()
	if err != nil {
		t.Fatalf("validate standard loadout: %v", err)
	}
	want := []FishingTournamentStandardTackleItem{
		{Slot: "rod", ItemID: "basic_fishing_rod"},
		{Slot: "reel", ItemID: "basic_spinning_reel"},
		{Slot: "main_line", ItemID: "braided_main_line"},
		{Slot: "leader", ItemID: "clear_leader"},
		{Slot: "hook", ItemID: "fine_fishing_hook"},
		{Slot: "float_or_sinker", ItemID: "reed_float"},
		{Slot: "bait", ItemID: "earthworm_bait"},
	}
	if standard.Method != "float" || !reflect.DeepEqual(standard.Items, want) {
		t.Fatalf("standard loadout=%+v, want method=float items=%+v", standard, want)
	}

	snapshot := FishingLoadoutSnapshot{Method: standard.Method, FrozenComponents: make([]FishingFrozenComponent, 0, len(standard.Items))}
	for _, item := range standard.Items {
		snapshot.FrozenComponents = append(snapshot.FrozenComponents, FishingFrozenComponent{Slot: item.Slot, ItemID: item.ItemID})
	}
	if !fishingLoadoutMatchesTournamentStandard(snapshot, standard) {
		t.Fatal("exact standard item ids should qualify")
	}
	newTournament := FishingTournament{WeekKey: fishingTournamentStandardLoadoutStartWeek}
	if err := store.validateFishingTournamentSelection(newTournament, fishingTournamentDivisionStandard, snapshot); err != nil {
		t.Fatalf("exact standard selection was rejected: %v", err)
	}
	if err := store.validateFishingTournamentSelection(newTournament, fishingTournamentDivisionOpen, FishingLoadoutSnapshot{}); err != nil {
		t.Fatalf("open selection should accept a free loadout: %v", err)
	}
	snapshot.FrozenComponents[1].ItemID = "sealed_drag_reel"
	if fishingLoadoutMatchesTournamentStandard(snapshot, standard) {
		t.Fatal("a different item id must not qualify")
	}
	if err := store.validateFishingTournamentSelection(newTournament, fishingTournamentDivisionStandard, snapshot); !errors.Is(err, ErrFishingLoadoutInvalid) {
		t.Fatalf("mismatched standard selection error=%v, want ErrFishingLoadoutInvalid", err)
	}
	snapshot.FrozenComponents[1].ItemID = want[1].ItemID
	snapshot.FrozenComponents = append(snapshot.FrozenComponents, FishingFrozenComponent{Slot: "lure", ItemID: "silver_shad_lure"})
	if fishingLoadoutMatchesTournamentStandard(snapshot, standard) {
		t.Fatal("a loadout with an extra component must not qualify")
	}
}

func TestFishingTournamentStandardLoadoutStartsAfterExistingV2Week(t *testing.T) {
	if FishingRulesetVersion != "fishing-v2" {
		t.Fatalf("ruleset version=%q, existing fishing-v2 data must not be versioned over", FishingRulesetVersion)
	}
	legacy := FishingTournament{WeekKey: "2026-08-31", RulesetVersion: FishingRulesetVersion}
	next := FishingTournament{WeekKey: fishingTournamentStandardLoadoutStartWeek, RulesetVersion: FishingRulesetVersion}
	if fishingTournamentUsesStandardLoadout(legacy) {
		t.Fatal("the existing fishing-v2 tournament must retain its historical rules")
	}
	if !fishingTournamentUsesStandardLoadout(next) {
		t.Fatal("the next tournament should use designated item ids")
	}
}

func TestFishingTournamentEntryIntegrityRepairIsScopedToContinuityEvent(t *testing.T) {
	affected := FishingTournament{
		WeekKey:         fishingTournamentEntryRepairWeek,
		TargetSpeciesID: fishingTournamentEntryRepairTarget,
		RulesetVersion:  FishingRulesetVersion,
	}
	if !fishingTournamentNeedsEntryIntegrityRepair(affected) {
		t.Fatal("the migrated Emberfin tournament must enable the targeted repair")
	}
	for _, unaffected := range []FishingTournament{
		{WeekKey: fishingTournamentEntryRepairWeek, TargetSpeciesID: "bluegill", RulesetVersion: FishingRulesetVersion},
		{WeekKey: fishingTournamentStandardLoadoutStartWeek, TargetSpeciesID: fishingTournamentEntryRepairTarget, RulesetVersion: FishingRulesetVersion},
		{WeekKey: fishingTournamentEntryRepairWeek, TargetSpeciesID: fishingTournamentEntryRepairTarget, RulesetVersion: fishingLegacyRulesetVersion},
	} {
		if fishingTournamentNeedsEntryIntegrityRepair(unaffected) {
			t.Fatalf("unrelated tournament unexpectedly enabled repair: %+v", unaffected)
		}
	}
}

func TestFishingTournamentParticipationReachesCatchUpInFourWeeks(t *testing.T) {
	tokens, itemID, cosmeticKey := fishingTournamentRewardForRank(40)
	if tokens*4 != fishingTournamentCatchUpCost || itemID != "" || cosmeticKey != "" {
		t.Fatalf("participation reward=(%d,%q,%q), want four weeks to equal catch-up cost", tokens, itemID, cosmeticKey)
	}
	tokens, itemID, cosmeticKey = fishingTournamentRewardForRank(1)
	if tokens <= 50 || itemID != "champion_current_rod" || cosmeticKey != "fishing_champion" {
		t.Fatalf("champion reward=(%d,%q,%q), want immediate rod and exclusive cosmetic", tokens, itemID, cosmeticKey)
	}
}

func TestFishingRotatedTripValueStaysNearPeerProfessions(t *testing.T) {
	data, err := catalog.Load(filepath.Join("..", "..", "..", "packages", "game-data", "generated", "game-data.json"))
	if err != nil {
		t.Fatalf("load generated fishing data: %v", err)
	}
	marketMultipliers := []float64{fishingSaleMarketMinimum, 1, fishingSaleMarketMaximum}
	for _, water := range data.FishingWaters {
		if water.ID != "sunmere_shallows" && water.ID != "willowrun_valley" && water.ID != "frostglass_lake" {
			continue
		}
		spotTotals := make([]float64, len(marketMultipliers))
		spotCount := 0
		for _, spot := range data.FishingSpots {
			if spot.WaterID != water.ID {
				continue
			}
			bestMethodValues := make([]float64, len(marketMultipliers))
			for _, method := range spot.Methods {
				for marketIndex, marketMultiplier := range marketMultipliers {
					methodTotal := 0.0
					for seed := int64(1); seed <= 96; seed++ {
						methodTotal += simulateFishingValueHour(data, water, spot, method, seed, marketMultiplier)
					}
					methodAverage := methodTotal / 96
					if methodAverage > bestMethodValues[marketIndex] {
						bestMethodValues[marketIndex] = methodAverage
					}
				}
			}
			for marketIndex, bestMethodValue := range bestMethodValues {
				spotTotals[marketIndex] += bestMethodValue
			}
			spotCount++
		}
		if spotCount == 0 {
			t.Fatalf("water %s has no fishing spots", water.ID)
		}
		hourlyValues := make([]float64, len(marketMultipliers))
		for marketIndex, spotTotal := range spotTotals {
			hourlyValues[marketIndex] = spotTotal / float64(spotCount)
		}
		t.Logf("%s rotated-trip value: %.0f / %.0f / %.0f gold/hour at market %.2f / 1.00 / %.2f", water.ID, hourlyValues[0], hourlyValues[1], hourlyValues[2], fishingSaleMarketMinimum, fishingSaleMarketMaximum)
		if hourlyValues[0] < 3_600 || hourlyValues[2] > 5_400 {
			t.Fatalf("%s fishing value range %.0f-%.0f gold/hour escaped the 1-1.5 gold/second anchor", water.ID, hourlyValues[0], hourlyValues[2])
		}
	}
}

func simulateFishingValueHour(data catalog.Data, water catalog.FishingWater, spot catalog.FishingSpot, method string, seed int64, saleMultiplier float64) float64 {
	loadout := FishingLoadoutSnapshot{
		Method:       method,
		FishingLevel: water.RequiredFishingLevel,
		Efficiency:   float64(max(water.RequiredFishingLevel-1, 0)) * 0.01,
		Components: FishingLoadoutComponents{
			MainLineItemID: "braided_main_line",
			LeaderItemID:   "clear_leader",
		},
	}
	for _, itemID := range []string{loadout.Components.MainLineItemID, loadout.Components.LeaderItemID} {
		if definition := data.FishingTackleByItemID(itemID); definition != nil {
			loadout.FrozenComponents = append(loadout.FrozenComponents, FishingFrozenComponent{
				Slot: definition.Slot, ItemID: itemID, WearPerFight: definition.WearPerFight,
			})
		}
	}
	supplies := FishingTripSupplies{Remaining: FishingSupplyCounts{
		MainLine: 100_000, Leader: 100_000, Hook: 100_000, FloatOrSinker: 100_000, Lure: 100_000,
	}}
	terminalCost := 0.0
	switch method {
	case "float":
		loadout.Control, loadout.Sensitivity, loadout.MaxLoadKG, loadout.BiteMultiplier = 2.25, 2.85, 8, 1.1
		terminalCost = 21
	case "bottom":
		loadout.Control, loadout.Sensitivity, loadout.MaxLoadKG, loadout.BiteMultiplier = 2.45, 2.35, 8, 1.1
		terminalCost = 21
	default:
		loadout.Control, loadout.Sensitivity, loadout.MaxLoadKG, loadout.BiteMultiplier = 2.5, 2.15, 8, 1
		terminalCost = 31
	}
	start := time.Date(2031, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(seed*331) * time.Minute)
	end := start.Add(time.Hour)
	now := start
	pressure := 0.18
	rng := rand.New(rand.NewSource(seed + int64(stableFishingHash(spot.ID+":"+method))))
	value := 0.0
	for now.Before(end) {
		if method != "lure" {
			value -= 4
		}
		conditions := fishingConditionsAt(water, spot, now)
		wait := fishingWaitDuration(spot, loadout, "balanced", pressure, conditions, rng)
		if now.Add(wait).After(end) {
			break
		}
		pressure = decayFishingPressure(pressure, wait)
		now = now.Add(wait)
		conditions = fishingConditionsAt(water, spot, now)
		biteChance := fishingBiteChance(spot, loadout, "balanced", pressure, conditions, 1)
		pressure = clampFloat(pressure+0.006, 0, 1)
		if rng.Float64() >= biteChance {
			continue
		}
		species := selectFishingSpecies(data.FishingSpecies, spot, loadout, conditions, fishingTargetLayerAuto, rng)
		if species == nil {
			continue
		}
		pressure = clampFloat(pressure+0.012, 0, 1)
		fish := generateFishingIndividual(*species, water.ID, spot.ID, FishingRulesetVersion, now, pressure, rng)
		now = now.Add(3*time.Second + fishingFightDuration(*species, fish, loadout, "balanced"))
		fight := resolveFishingFight(*species, fish, loadout, "balanced", 82, rng)
		if fight.LineBreak {
			value -= terminalCost
			consumeFishingTerminalSupplies(&supplies, loadout, true)
		} else {
			for itemID, quantity := range applyFishingTackleWear(&supplies, loadout, *species) {
				if item := data.ItemByID(itemID); item != nil {
					value -= item.EconomyBaseValue * float64(quantity)
				}
			}
			if fight.Landed {
				value += species.BaseValue * fishingSaleSpecimenFactors(*species, fish).SpecimenMultiplier * saleMultiplier
			}
		}
		now = now.Add(4 * time.Second)
	}
	return value
}
