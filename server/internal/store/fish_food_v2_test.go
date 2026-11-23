package store

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
)

func fishCuisineTestStore(t *testing.T) *Store {
	t.Helper()
	data, err := catalog.Load("../../../packages/game-data/generated/game-data.json")
	if err != nil {
		t.Fatal(err)
	}
	registry, defs, err := buildOperationEffectRuntime(data)
	if err != nil {
		t.Fatal(err)
	}
	return &Store{gameData: data, operationEffectRegistry: registry, operationEffectDefinitions: defs}
}

func recipeTestFish(s *Store, recipe catalog.FishFoodRecipe, lengthScore, condition float64, mutations []string) []selectedFishFoodIngredient {
	var selected []selectedFishFoodIngredient
	for _, ingredient := range recipe.FishIngredients {
		species := s.gameData.FishingSpeciesByID(ingredient.SpeciesID)
		length := species.MinLengthCM + (species.MaxLengthCM-species.MinLengthCM)*lengthScore
		selected = append(selected, selectedFishFoodIngredient{Fish: FishInstanceData{
			SpeciesID: species.ID, LengthCM: length, WeightKG: species.ShapeFactor * math.Pow(length, 3) / 100_000 * condition,
			Genes: FishGenes{Mutations: mutations},
		}})
	}
	return selected
}

func TestFishCuisineV2AllPairsAndFrozenCustomLifetimes(t *testing.T) {
	s := fishCuisineTestStore(t)
	if len(s.gameData.FishFoodRecipes) != 30 {
		t.Fatalf("recipes=%d", len(s.gameData.FishFoodRecipes))
	}
	groups := map[string][]catalog.FishFoodRecipe{}
	for _, recipe := range s.gameData.FishFoodRecipes {
		e := newFishFoodExecutor(s, recipe)
		def, ok, err := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, nil)
		if err != nil || !ok {
			t.Fatalf("%s: %v", recipe.ID, err)
		}
		groups[def.Kind+def.Stacking.Key] = append(groups[def.Kind+def.Stacking.Key], recipe)
		for _, condition := range []float64{0.75, 1, 1.25} {
			selected := recipeTestFish(s, recipe, condition-0.5, condition, []string{"shimmering", "prismatic", "rainbow", "prismatic"})
			output, err := e.buildSpecificOutputSnapshot(selected)
			if err != nil {
				t.Fatalf("%s: %v", recipe.ID, err)
			}
			food := output["fishFood"].(fishFoodItemSnapshot)
			if food.Quality.SpecialMultiplier != 1.815 {
				t.Fatalf("duplicate special mutations multiplied twice: %+v", food.Quality)
			}
			view, ok, err := s.OperationEffectDefinitionForItem(recipe.OutputItemID, output)
			if err != nil || !ok {
				t.Fatalf("custom roundtrip %s: %v", recipe.ID, err)
			}
			wantCharges := int64(2)
			if def.Domain == operationeffect.DomainFishingCast {
				wantCharges = 20
			}
			if def.Kind == "alchemy.infusion_success_bonus" {
				wantCharges = 10
			}
			if recipe.LifetimeScaling != nil {
				wantCharges = def.Lifetime.Charges + food.Quality.ExtraCharges
			}
			if def.Kind == "enchantment.high_rarity_weight" {
				wantCharges = 1
			}
			if recipe.Kind == "grilled_fish" && (view.Lifetime.Charges != wantCharges || food.Quality.ExtraDurationSeconds != 0) {
				t.Fatal("grill lifetime mismatch")
			}
			if recipe.Kind == "fish_soup" && (view.Lifetime.DurationSeconds != def.Lifetime.DurationSeconds+food.Quality.ExtraDurationSeconds || food.Quality.ExtraCharges != 0) {
				t.Fatal("soup lifetime mismatch")
			}
		}
	}
	for kind, recipes := range groups {
		if len(recipes) == 1 && (kind == "alchemy.transmute_durationfood.transmute" || kind == "enchantment.high_rarity_weightfood.enchantment") {
			continue
		}
		if len(recipes) != 2 || recipes[0].Kind == recipes[1].Kind || !reflect.DeepEqual(recipes[0].FishIngredients, recipes[1].FishIngredients) {
			t.Fatalf("unpaired %s", kind)
		}
	}
}

func TestFishCuisineV2LengthAndWeightAreIndependent(t *testing.T) {
	s := fishCuisineTestStore(t)
	recipe := *s.gameData.FishFoodRecipeByActionID("cook_mossfire_dual_fish_grill")
	e := newFishFoodExecutor(s, recipe)
	base, _, _ := e.resolveQuality(recipeTestFish(s, recipe, 0.5, 1, nil))
	long, _, _ := e.resolveQuality(recipeTestFish(s, recipe, 1, 1, nil))
	heavy, _, _ := e.resolveQuality(recipeTestFish(s, recipe, 0.5, 1.25, nil))
	if long.WeightScore != base.WeightScore || heavy.LengthScore != base.LengthScore {
		t.Fatal("length counted twice as weight")
	}
	def, _, _ := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, nil)
	for _, q := range []*fishFoodQualitySnapshot{&base, &long, &heavy} {
		if _, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, q); err != nil {
			t.Fatal(err)
		}
	}
	if long.ParameterMultipliers["positiveFloorReductionRate"] <= heavy.ParameterMultipliers["positiveFloorReductionRate"] || heavy.ParameterMultipliers["extraMaterialQuantity"] <= long.ParameterMultipliers["extraMaterialQuantity"] {
		t.Fatal("parameter weights not differentiated")
	}
}

func TestFishCuisineShimmeringPotencyAndCaps(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, recipe := range s.gameData.FishFoodRecipes {
		e := newFishFoodExecutor(s, recipe)
		def, _, _ := s.resolveOperationEffectDefinitionForItem(recipe.OutputItemID, nil)
		resolve := func(mutations []string) (fishFoodQualitySnapshot, map[string]float64) {
			selected := recipeTestFish(s, recipe, 1, 1.25, mutations)
			for i := range selected {
				selected[i].Fish.Genes = FishGenes{SizePotential: 1, GrowthRate: 1, FeedEfficiency: 1, Vitality: 1, Lifespan: 1, Fertility: 1, Mutations: mutations}
			}
			quality, _, err := e.resolveQuality(selected)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, &quality)
			if err != nil {
				t.Fatal(err)
			}
			var params map[string]float64
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatal(err)
			}
			return quality, params
		}
		beforeQuality, before := resolve([]string{"prismatic", "rainbow"})
		afterQuality, after := resolve([]string{"shimmering", "prismatic", "rainbow", "shimmering"})
		if afterQuality.SpecialMultiplier != 1.815 || beforeQuality.ExtraCharges != afterQuality.ExtraCharges || beforeQuality.ExtraDurationSeconds != afterQuality.ExtraDurationSeconds {
			t.Fatalf("shimmering potency/lifetime mismatch: %s", recipe.ID)
		}
		for name, scale := range recipe.ParameterScaling {
			if def.Stacking.Key == "food.primary_work" && (scale.Min == scale.Max || before[name] == scale.Max) {
				if after[name] != scale.Max {
					t.Fatalf("work meal cap changed: %s.%s", recipe.ID, name)
				}
				continue
			}
			improved := after[name] > before[name]
			if scale.Inverse {
				improved = after[name] < before[name]
			}
			if !improved || after[name] < scale.Min || after[name] > scale.Max {
				t.Fatalf("shimmering hidden by cap: %s.%s before=%v after=%v", recipe.ID, name, before[name], after[name])
			}
		}
	}
}

func TestFishCuisineLegacyInstancesKeepTheirSixUses(t *testing.T) {
	s := fishCuisineTestStore(t)
	view, ok, err := s.OperationEffectDefinitionForItem("mossfire_dual_fish_grill", map[string]any{"operationEffect": operationEffectItemSnapshot{
		SchemaVersion: 1, DefinitionID: "food_mossfire_dual_fish_grill", RulesetVersion: "food-alchemy-v1", Params: json.RawMessage(`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.096}`),
	}})
	if err != nil || !ok || view.Lifetime.Charges != 6 || view.Kind != alchemyeffect.KindInfusionBulkCapacityFloorTradeoff {
		t.Fatalf("legacy instance changed: %+v %v", view, err)
	}
	_, _, err = s.OperationEffectDefinitionForItem("steadyfin_grill", map[string]any{"operationEffect": operationEffectItemSnapshot{SchemaVersion: 1, DefinitionID: "food_mossfire_dual_fish_grill", RulesetVersion: "food-alchemy-v1", Params: json.RawMessage(`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.096}`)}})
	if !errors.Is(err, ErrOperationEffectItemInvalid) {
		t.Fatal("foreign food override accepted")
	}
}

func TestFishCuisineEnhancementOnlineOfflineStartParity(t *testing.T) {
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := s.operationEffectDefinitions["food_steadyfin_soup_v2"]
	until := at.Add(600 * time.Second)
	newSession := func() *operationeffect.Session {
		session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, ValidUntil: &until}})
		if err != nil {
			t.Fatal(err)
		}
		return session
	}
	offline := QueueRuntimeItem{QueueID: 7, ActionID: "equipment_enhance", ActorType: ActionActorTypePlayer, EffectSession: newSession(), PreserveScheduledTimeline: true}
	online := offline
	online.EffectSession = newSession()
	online.PreserveScheduledTimeline = false
	timing := queueRunTiming{StartAt: at, CurrentDuration: 10 * time.Second, NextDuration: 10 * time.Second, HasCurrentRun: true}
	// Use explicit start instants so the boundary assertion does not depend on
	// any equipment duration formula or the current wall clock.
	for i, startedAt := range []time.Time{at, until.Add(-time.Millisecond), until, until.Add(time.Hour)} {
		op := queueOperationEffectOperation(operationeffect.DomainEnhancement, offline, startedAt, i+1, nil)
		frozen, err := offline.EffectSession.Commit(offline.EffectSession.Prepare(op))
		if err != nil {
			t.Fatal(err)
		}
		op = queueOperationEffectOperation(operationeffect.DomainEnhancement, online, startedAt, i+1, nil)
		live, err := online.EffectSession.Commit(online.EffectSession.Prepare(op))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(frozen, live) {
			t.Fatal("online/offline snapshot mismatch")
		}
		run := enhanceRunContext{BaseSuccessRate: 0.4, SuccessRate: 0.4}
		if err := applyEnhanceOperationEffects(&run, frozen); err != nil {
			t.Fatal(err)
		}
		want := 0.4
		if i < 2 {
			want = 0.42
		}
		if !almostEqual(run.SuccessRate, want, 1e-6) {
			t.Fatalf("rate=%v want=%v", run.SuccessRate, want)
		}
	}
	late := until.Add(time.Second)
	live, err := nextEnhanceOperationEffects(online, timing, late, 1)
	if err != nil {
		t.Fatal(err)
	}
	historical, err := nextEnhanceOperationEffects(offline, timing, late, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(live.Rules) != 0 || len(historical.Rules) != 1 {
		t.Fatal("online actual time was confused with offline logical time")
	}
}

func TestFishCuisinePostgresExclusiveAndServingCaps(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := fishCuisineTestStore(t)
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	activate := func(id string) error {
		def := s.operationEffectDefinitions[id]
		if id == "food_mossfire_dual_fish_grill" {
			def.Kind = alchemyeffect.KindInfusionBulkCapacityFloorTradeoff
			def.RulesetVersion += ":bulk-materials"
		}
		raw, _ := json.Marshal(def)
		_, err := s.activateOperationEffectGrantTx(ctx, tx, 11, "food", def, operationeffect.Target{Kind: def.AllowedTargets[0]}, raw, now)
		return err
	}
	if err := activate("food_mossfire_dual_fish_grill_v4"); err != nil {
		t.Fatal(err)
	}
	if err := activate("food_mossfire_dual_fish_soup_v4"); !errors.Is(err, ErrOperationEffectStrengthConflict) {
		t.Fatalf("pair stacked: %v", err)
	}
	if err := activate("food_mossfire_dual_fish_grill"); !errors.Is(err, ErrOperationEffectStrengthConflict) {
		t.Fatalf("legacy bypassed exclusivity: %v", err)
	}
	if err := activate("food_mossfire_dual_fish_grill_v4"); err != nil {
		t.Fatal(err)
	}
	if err := activate("food_mossfire_dual_fish_grill_v4"); !errors.Is(err, ErrOperationEffectAtCap) {
		t.Fatalf("charge cap: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM player_operation_effect_grants`); err != nil {
		t.Fatal(err)
	}
	if err := activate("food_mossfire_dual_fish_grill"); err != nil {
		t.Fatal(err)
	}
	if err := activate("food_mossfire_dual_fish_soup_v4"); !errors.Is(err, ErrOperationEffectStrengthConflict) {
		t.Fatalf("new soup bypassed legacy grant: %v", err)
	}
	if err := activate("food_starlight_silverfish_soup_v2"); err != nil {
		t.Fatalf("unrelated recipe family blocked: %v", err)
	}
	if err := activate("food_starlight_silverfish_soup_v2"); err != nil {
		t.Fatal(err)
	}
	if err := activate("food_starlight_silverfish_soup_v2"); !errors.Is(err, ErrOperationEffectAtCap) {
		t.Fatalf("partial soup serving swallowed: %v", err)
	}
	for i, id := range []string{"food_frugal_river_grill_v2", "food_frugal_river_soup_v2"} {
		def := s.operationEffectDefinitions[id]
		raw, _ := json.Marshal(def)
		if _, err := s.activateOperationEffectGrantTx(ctx, tx, 11, "food", def, operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: int64(i + 1)}, raw, now); err != nil {
			t.Fatalf("different kitty targets conflict: %v", err)
		}
	}
}

func TestFishCuisineFishingCastSnapshotExpiryAndCharges(t *testing.T) {
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := s.operationEffectDefinitions["food_reedbank_grill_v2"]
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1}})
	if err != nil {
		t.Fatal(err)
	}
	runtime := fishingExecutorRuntime{TripID: "test", ActorType: ActionActorTypePlayer, effectSession: session, Summary: FishingTripSummary{Casts: 1}}
	if err := freezeFishingCastEffects(&runtime, at); err != nil {
		t.Fatal(err)
	}
	if runtime.CastEffects == nil {
		t.Fatal("first cast missed effect")
	}
	frozen := *runtime.CastEffects
	if err := freezeFishingCastEffects(&runtime, at); err != nil || len(session.ChargeReceipts()) != 1 {
		t.Fatal("retry consumed again")
	}
	runtime.Summary.Casts++
	if err := freezeFishingCastEffects(&runtime, at.Add(time.Minute)); err != nil || runtime.CastEffects != nil {
		t.Fatal("second cast borrowed charge")
	}
	bonus, err := ratebonus.Resolve(frozen, ratebonus.FishingBite)
	if err != nil || bonus != 0.1 {
		t.Fatal("frozen cast lost benefit")
	}
	def = s.operationEffectDefinitions["food_reedbank_soup_v2"]
	until := at.Add(10 * time.Minute)
	session, err = operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 2, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, ValidUntil: &until}})
	if err != nil {
		t.Fatal(err)
	}
	runtime.effectSession = session
	runtime.Summary.Casts = 1
	if err := freezeFishingCastEffects(&runtime, until.Add(-time.Millisecond)); err != nil || runtime.CastEffects == nil {
		t.Fatal("pre-expiry cast missing")
	}
	frozen = *runtime.CastEffects
	runtime.Summary.Casts++
	if err := freezeFishingCastEffects(&runtime, until); err != nil || runtime.CastEffects != nil {
		t.Fatal("expiry boundary matched")
	}
	if bonus, _ := ratebonus.Resolve(frozen, ratebonus.FishingBite); bonus != 0.08 {
		t.Fatal("offline frozen bonus lost")
	}
}
