package store

import (
	"encoding/json"
	"math"
	"math/rand"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	fishingeffect "mysecondwebgame/server/internal/operationeffect/fishing"
	"testing"
	"time"
)

func TestFishingFightCuisineIsolation(t *testing.T) {
	species := catalog.FishingSpecies{FightPower: 1, HookingChallenge: 1}
	fish := FishInstanceData{WeightKG: 2}
	loadout := FishingLoadoutSnapshot{MaxLoadKG: 1, Control: 1, Hooking: 1}
	for _, ruleset := range []string{FishingRulesetVersion, fishingLegacyRulesetVersion} {
		breaks := 0
		for seed := int64(0); seed < 200; seed++ {
			oldRNG, zeroRNG := rand.New(rand.NewSource(seed)), rand.New(rand.NewSource(seed))
			base := resolveFishingFightForRuleset(species, fish, loadout, "balanced", 80, ruleset, oldRNG)
			zero := resolveFishingFightWithEffects(species, fish, loadout, "balanced", 80, ruleset, fishingeffect.Modifiers{}, zeroRNG)
			if base != zero || oldRNG.Float64() != zeroRNG.Float64() {
				t.Fatal("empty effects changed settlement or RNG")
			}
			landing := resolveFishingFightWithEffects(species, fish, loadout, "balanced", 80, ruleset, fishingeffect.Modifiers{LandingBonusRate: 0.06}, rand.New(rand.NewSource(seed)))
			if base.LineLoad != landing.LineLoad || base.LineBreak != landing.LineBreak {
				t.Fatal("landing bypassed break check")
			}
			if !base.LineBreak && math.Abs(landing.Chance-math.Min(0.97, base.Chance+0.06)) > 1e-12 {
				t.Fatal("landing rate mismatch")
			}
			relief := resolveFishingFightWithEffects(species, fish, loadout, "balanced", 80, ruleset, fishingeffect.Modifiers{LineLoadReductionRate: 0.08}, rand.New(rand.NewSource(seed)))
			if math.Abs(relief.LineLoad-roundFloat(fishingLineLoadForRuleset(species, fish, "balanced", ruleset)*0.92, 3)) > 1e-12 {
				t.Fatal("load reduction mismatch")
			}
			if relief.LineBreak {
				breaks++
			}
		}
		if breaks == 0 {
			t.Fatal("load reduction guaranteed survival")
		}
	}
}

func TestFishCuisineLifetimeBonusesAreRecipeSpecific(t *testing.T) {
	recipe := catalog.FishFoodRecipe{Kind: "grilled_fish", MutationLifetimeBonuses: map[string]catalog.FishFoodMutationLifetimeBonus{"prismatic": {Charges: 8}, "shimmering": {Charges: 3}}}
	charges, seconds := fishFoodLifetimeBonus(recipe, map[string]bool{"prismatic": true, "shimmering": true})
	if charges != 11 || seconds != 0 {
		t.Fatal("bonuses hardcoded")
	}
	recipe.MutationLifetimeBonuses = map[string]catalog.FishFoodMutationLifetimeBonus{}
	if charges, _ := fishFoodLifetimeBonus(recipe, nil); charges != 0 {
		t.Fatal("empty config restored defaults")
	}
}

func TestFishCuisineReplacedItemsUseFrozenIngredients(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, recipe := range s.gameData.FishFoodRecipes {
		for _, oldID := range recipe.ReplacedEffectIDs {
			output, err := newFishFoodExecutor(s, recipe).buildSpecificOutputSnapshot(recipeTestFish(s, recipe, 0.5, 1, []string{"prismatic", "prismatic", "rainbow"}))
			if err != nil {
				t.Fatal(err)
			}
			old := s.operationEffectDefinitions[oldID]
			oldLife := old.Lifetime
			if oldLife.Kind == operationeffect.LifetimeCharges {
				oldLife.Charges++
			} else {
				oldLife.DurationSeconds += 900
			}
			output["operationEffect"] = operationEffectItemSnapshot{SchemaVersion: 2, DefinitionID: old.ID, RulesetVersion: old.RulesetVersion, Params: old.Params, Lifetime: &oldLife}
			got, ok, err := s.OperationEffectDefinitionForItem(recipe.OutputItemID, output)
			if err != nil || !ok || got.ID == oldID {
				t.Fatalf("%s: %+v %v", recipe.ID, got, err)
			}
			if recipe.Kind == "grilled_fish" && got.Domain == operationeffect.DomainFishingCast && got.Lifetime.Charges != 20 {
				t.Fatalf("old fish upgrade: %+v", got.Lifetime)
			}
			if s.isRetiredFishFoodEffect(old) != (old.Kind != got.Kind || old.Stacking.Key == "food.primary_work" && old.RulesetVersion == "primary-work-food-v1") {
				t.Fatal("wrong grant retirement")
			}
			output["fishFood"] = map[string]any{"schemaVersion": 1, "recipeId": "foreign"}
			if _, _, err = s.OperationEffectDefinitionForItem(recipe.OutputItemID, output); err == nil {
				t.Fatal("foreign snapshot accepted")
			}
		}
	}
}

func TestFishingCuisineTwentyCastsPersistAndExpire(t *testing.T) {
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"food_frugal_river_grill_v3", "food_harvest_river_grill_v3", "food_reedbank_grill_v3"} {
		def := s.operationEffectDefinitions[id]
		def.Lifetime.Charges = 20
		grant := operationeffect.Grant{ID: 1, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, InitialCharges: 20, RemainingCharges: 20}
		live, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{grant})
		if err != nil {
			t.Fatal(err)
		}
		for cast := 1; cast <= 21; cast++ {
			// Reload remaining charges as an offline chunk would, without sharing memory.
			raw, _ := json.Marshal(grant)
			var restored operationeffect.Grant
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			offline, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{restored})
			if err != nil {
				t.Fatal(err)
			}
			a := fishingExecutorRuntime{TripID: "casts", ActorType: ActionActorTypePlayer, effectSession: live, Summary: FishingTripSummary{Casts: int64(cast)}}
			b := a
			b.effectSession = offline
			started := at.Add(time.Duration(cast) * time.Minute)
			if err := freezeFishingCastEffects(&a, started); err != nil {
				t.Fatal(err)
			}
			if err := freezeFishingCastEffects(&b, started); err != nil {
				t.Fatal(err)
			}
			liveJSON, _ := json.Marshal(a.CastEffects)
			offlineJSON, _ := json.Marshal(b.CastEffects)
			if string(liveJSON) != string(offlineJSON) || (a.CastEffects != nil) != (cast <= 20) {
				t.Fatalf("%s cast %d mismatch", id, cast)
			}
			if cast <= 20 {
				grant.RemainingCharges--
			}
		}
		if len(live.ChargeReceipts()) != 20 {
			t.Fatal("incorrect charge consumption")
		}
	}
	for _, id := range []string{"food_frugal_river_soup_v3", "food_harvest_river_soup_v3"} {
		def := s.operationEffectDefinitions[id]
		until := at.Add(10 * time.Minute)
		session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: def.AllowedTargets[0]}, ValidFrom: at, ValidUntil: &until}})
		if err != nil {
			t.Fatal(err)
		}
		runtime := fishingExecutorRuntime{TripID: "expiry", SpotID: "spot", DurationMS: 1000, ActorType: ActionActorTypePlayer, effectSession: session, Summary: FishingTripSummary{Casts: 1}}
		if err := freezeFishingCastEffects(&runtime, until.Add(-time.Millisecond)); err != nil || runtime.CastEffects == nil {
			t.Fatal("historical cast missed")
		}
		payload, err := marshalFishingEnvelope(FishingTripActionID, runtime)
		if err != nil {
			t.Fatal(err)
		}
		resumed, ok := parseFishingExecutorRuntime(payload)
		if !ok {
			t.Fatal("resume failed")
		}
		if mods, err := fishingeffect.Resolve(*resumed.CastEffects); err != nil || (mods == fishingeffect.Modifiers{}) {
			t.Fatal("frozen effect lost")
		}
		runtime.Summary.Casts++
		if err := freezeFishingCastEffects(&runtime, until); err != nil || runtime.CastEffects != nil {
			t.Fatal("expired cast matched")
		}
	}
}

func TestFishCuisinePostgresRetirementAndTwentyCastLedger(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	activate := func(id string, charges int64) {
		def := s.operationEffectDefinitions[id]
		if charges > 0 {
			def.Lifetime.Charges = charges
		}
		raw, _ := json.Marshal(def)
		if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "food", def, operationeffect.Target{Kind: def.AllowedTargets[0]}, raw, at); err != nil {
			t.Fatal(err)
		}
	}
	activate("food_frugal_river_grill_v2", 0)
	activate("food_harvest_river_soup_v2", 0)
	if session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, at); err != nil || session != nil {
		t.Fatalf("retired grants loaded: %v", err)
	}
	activate("food_frugal_river_grill_v3", 20)
	for cast := 1; cast <= 21; cast++ {
		started := at.Add(time.Duration(cast) * time.Minute)
		session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, started)
		if err != nil {
			t.Fatal(err)
		}
		runtime := fishingExecutorRuntime{TripID: "pg-casts", ActorType: ActionActorTypePlayer, effectSession: session, Summary: FishingTripSummary{Casts: int64(cast)}}
		if err := freezeFishingCastEffects(&runtime, started); err != nil {
			t.Fatal(err)
		}
		if (runtime.CastEffects != nil) != (cast <= 20) {
			t.Fatalf("cast %d eligibility", cast)
		}
		for retry := 0; retry < 2; retry++ {
			if _, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, started); err != nil {
				t.Fatal(err)
			}
		}
	}
	var receipts, remaining int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM player_operation_effect_charge_receipts`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT remaining_charges FROM player_operation_effect_grants WHERE definition_id='food_frugal_river_grill_v3'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if receipts != 20 || remaining != 0 {
		t.Fatalf("receipts=%d remaining=%d", receipts, remaining)
	}
}
