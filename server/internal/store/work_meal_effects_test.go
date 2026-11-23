package store

import (
	"encoding/json"
	"fmt"
	"math"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
	"testing"
	"time"
)

func workTestSnapshot(kind, params string) operationeffect.Snapshot {
	return operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainLifeAction, Kind: kind, Params: []byte(params)}}}
}

func TestWorkMealDurationAndBaseExperience(t *testing.T) {
	meal := workTestSnapshot(lifeaction.KindPatientHarvest, `{"chance":0.08,"durationPenaltyRate":0.04}`)
	for _, base := range []time.Duration{time.Second, 2 * time.Second, 10 * time.Second, 16001 * time.Millisecond} {
		want := time.Duration(math.Ceil(float64(normalizeActionQueueRunDuration(base)/time.Millisecond)*1.04)) * time.Millisecond
		for i := 0; i < 3; i++ {
			got, err := workMealDuration(base, meal)
			if err != nil || got != want {
				t.Fatalf("duration=%v want=%v err=%v", got, want, err)
			}
		}
		got, err := workMealDuration(base, operationeffect.Snapshot{})
		if err != nil || got != base {
			t.Fatal("no-effect duration changed")
		}
	}
	xp := workTestSnapshot(lifeaction.KindWorkExperience, `{"experienceBonusRate":0.08}`)
	spec := ActionSpec{MainType: "collecting", StatusExpReward: map[string]float64{"collecting": 10, "luck": 20}}
	bonus, err := workMealExperience(spec, []operationeffect.Snapshot{xp, {}, xp})
	if err != nil || bonus["collecting"] != 1.6 || len(bonus) != 1 {
		t.Fatalf("base skill only: %v %v", bonus, err)
	}
	spec.MainType = "cooking"
	bonus, err = workMealExperience(spec, []operationeffect.Snapshot{xp})
	if err != nil || len(bonus) != 0 {
		t.Fatal("crafting XP affected")
	}
}

func TestWorkMealQualityAndFrozenLifetimes(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, tc := range []struct {
		id, field  string
		plain, max float64
		charges    int64
		seconds    int64
	}{
		{"field_honey", "chance", .015, .0272, 500, 2000},
		{"patient_dual", "chance", .015, .0272, 500, 2000},
		{"clear_mind", "experienceBonusRate", .07, .10, 500, 2000},
		{"gentle_current", "lossReductionRate", .07, .10, 500, 2000},
	} {
		for _, form := range []string{"grill", "soup"} {
			recipe := *s.gameData.FishFoodRecipeByActionID("cook_" + tc.id + "_" + form)
			for _, mutated := range []bool{false, true} {
				mutations := []string(nil)
				if mutated {
					mutations = []string{"shimmering", "prismatic", "rainbow", "shimmering"}
				}
				selected := recipeTestFish(s, recipe, 1, 1.25, mutations)
				out, err := newFishFoodExecutor(s, recipe).buildSpecificOutputSnapshot(selected)
				if err != nil {
					t.Fatal(err)
				}
				def, ok, err := s.OperationEffectDefinitionForItem(recipe.OutputItemID, out)
				if err != nil || !ok {
					t.Fatal(err)
				}
				var p map[string]float64
				if err := json.Unmarshal(def.Params, &p); err != nil {
					t.Fatal(err)
				}
				want := tc.plain
				if mutated {
					want = tc.max
				}
				if math.Abs(p[tc.field]-want) > 1e-6 {
					t.Fatalf("%s mutated=%t params=%v want=%v", recipe.ID, mutated, p, want)
				}
				if p["durationPenaltyRate"] != 0 {
					t.Fatal("retired time cost returned")
				}
				if mutated && form == "grill" && def.Lifetime.Charges != tc.charges {
					t.Fatalf("%s charges=%d want=%d", recipe.ID, def.Lifetime.Charges, tc.charges)
				}
				if mutated && form == "soup" && def.Lifetime.DurationSeconds != tc.seconds {
					t.Fatalf("%s seconds=%d", recipe.ID, def.Lifetime.DurationSeconds)
				}
			}
		}
	}
}

func TestWorkMealPostgresOfflineStartTransitions(t *testing.T) {
	for _, form := range []string{"grill", "soup"} {
		t.Run(form, func(t *testing.T) {
			ctx, tx := operationEffectPostgresTx(t)
			s := fishCuisineTestStore(t)
			at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
			def := s.operationEffectDefinitions["food_patient_dual_"+form+"_v3"]
			if form == "grill" {
				def.Lifetime.Charges = 2
			} else {
				def.Lifetime.DurationSeconds = 19
			}
			raw, _ := json.Marshal(def)
			target := operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}
			if form == "soup" {
				target.Kind = operationeffect.TargetPlayer
			}
			if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "patient_dual_"+form, def, target, raw, at); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue VALUES(1,41,'running',$1,$2)`, at, at.Add(10*time.Second)); err != nil {
				t.Fatal(err)
			}
			session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, at.Add(time.Hour))
			if err != nil || session == nil {
				t.Fatalf("historical load: %v", err)
			}
			started := at
			payload := json.RawMessage(`{"actionId":"gather_dew","executorId":"generic","runtime":{"baseDurationMs":10000,"durationMs":10000}}`)
			executor := &genericExecutor{store: s}
			for i := 0; i < 4; i++ {
				op := operationeffect.Operation{Key: fmt.Sprintf("work:%d", i), Domain: operationeffect.DomainLifeAction, MainType: "collecting", ActionID: "gather_dew", ActorKind: operationeffect.ActorPlayer, StartedAt: started, Capabilities: map[string]bool{lifeaction.CapabilityMainOutput: true}}
				if i%2 == 1 && form == "grill" {
					op.ActorKind = operationeffect.ActorKitty
					op.ActorID = 2
				}
				frozen, err := session.Commit(session.Prepare(op))
				if err != nil {
					t.Fatal(err)
				}
				item := QueueRuntimeItem{ActionID: "gather_dew", ActorType: string(op.ActorKind), ActorID: op.ActorID,
					Spec: ActionSpec{ID: "gather_dew", MainType: "collecting", DurationS: 10}, PayloadRaw: payload,
					OperationEffects: &frozen, WorkLoadoutRuntime: &WorkLoadoutRuntimeResolution{Available: true, AttributeTotals: map[string]float64{"stamina": 0}}}
				nextPayload, err := executor.StartRunTx(ctx, tx, 41, item, started)
				if err != nil {
					t.Fatal(err)
				}
				duration := parseQueueRuntimeDuration(nextPayload)
				payload, err = setQueueOperationEffectSnapshot(nextPayload, frozen)
				if err != nil {
					t.Fatal(err)
				}
				want := 10 * time.Second
				if i < 2 {
					want = 9700 * time.Millisecond
				}
				if duration != want {
					t.Fatalf("start %d=%v duration=%v want=%v", i, started, duration, want)
				}
				if i == 1 && form == "soup" && !started.Add(duration).After(at.Add(19*time.Second)) {
					t.Fatal("did not test expiry during operation")
				}
				started = started.Add(duration)
				if _, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, at.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			if want := at.Add(39400 * time.Millisecond); !started.Equal(want) {
				t.Fatalf("logical clock=%v want=%v", started, want)
			}
			var receipts int
			if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM player_operation_effect_charge_receipts`).Scan(&receipts); err != nil {
				t.Fatal(err)
			}
			want := 0
			if form == "grill" {
				want = 2
			}
			if receipts != want {
				t.Fatal(receipts)
			}
		})
	}
}

func TestWorkMealReplacesOversizedLegacyServings(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, form := range []string{"grill", "soup"} {
		recipe := *s.gameData.FishFoodRecipeByActionID("cook_patient_dual_" + form)
		selected := recipeTestFish(s, recipe, 1, 1.25, []string{"shimmering", "prismatic", "rainbow"})
		out, err := newFishFoodExecutor(s, recipe).buildSpecificOutputSnapshot(selected)
		if err != nil {
			t.Fatal(err)
		}
		old := s.operationEffectDefinitions["food_patient_dual_"+form+"_v1"]
		lifetime := old.Lifetime
		if form == "grill" {
			lifetime.Charges = old.Stacking.MaxCharges
		} else {
			lifetime.DurationSeconds = old.Stacking.MaxDurationSeconds
		}
		out["operationEffect"] = operationEffectItemSnapshot{SchemaVersion: 2, DefinitionID: old.ID, RulesetVersion: old.RulesetVersion, Params: old.Params, Lifetime: &lifetime}
		def, ok, err := s.OperationEffectDefinitionForItem(recipe.OutputItemID, out)
		if err != nil || !ok {
			t.Fatalf("legacy %s rejected: %v", form, err)
		}
		if def.ID != "food_patient_dual_"+form+"_v3" || def.Kind != lifeaction.KindFocusedHarvest {
			t.Fatal("old recipe not replaced")
		}
		if form == "grill" && def.Lifetime.Charges != 500 || form == "soup" && def.Lifetime.DurationSeconds != 2000 {
			t.Fatal("old oversized serving leaked", def.Lifetime)
		}
		if !s.isRetiredFishFoodEffect(old) {
			t.Fatal("old food may attach to new starts")
		}
	}
}

func TestReducedServingsReprojectStoredFoodWithoutRevokingActiveGrants(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, series := range []string{"field_honey", "patient_dual", "clear_mind", "gentle_current"} {
		for _, form := range []string{"grill", "soup"} {
			recipe := *s.gameData.FishFoodRecipeByActionID("cook_" + series + "_" + form)
			old := s.operationEffectDefinitions[recipe.ReplacedEffectIDs[len(recipe.ReplacedEffectIDs)-1]]
			if s.isRetiredFishFoodEffect(old) {
				t.Fatal("serving-only change revoked an active meal", old.ID)
			}
			for _, mutations := range [][]string{nil, {"shimmering", "prismatic", "rainbow"}} {
				out, err := newFishFoodExecutor(s, recipe).buildSpecificOutputSnapshot(recipeTestFish(s, recipe, 1, 1.25, mutations))
				if err != nil {
					t.Fatal(err)
				}
				want, _, err := s.OperationEffectDefinitionForItem(recipe.OutputItemID, out)
				if err != nil {
					t.Fatal(err)
				}
				oldLife := old.Lifetime
				if form == "grill" {
					oldLife.Charges = 1000
				} else {
					oldLife.DurationSeconds = 4000
				}
				out["operationEffect"] = operationEffectItemSnapshot{SchemaVersion: 2, DefinitionID: old.ID, RulesetVersion: old.RulesetVersion, Params: old.Params, Lifetime: &oldLife}
				got, ok, err := s.OperationEffectDefinitionForItem(recipe.OutputItemID, out)
				if err != nil || !ok || got.ID != want.ID || got.Lifetime != want.Lifetime || string(got.Params) != string(want.Params) {
					t.Fatal("saved ingredient projection differs from a newly cooked meal", recipe.ID, got, want, err)
				}
			}
		}
	}
}

func TestWorkMealEmptySnapshotRestoresBatching(t *testing.T) {
	s := fishCuisineTestStore(t)
	executor := &genericExecutor{store: s}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	op := operationeffect.Operation{Key: "work:1", Domain: operationeffect.DomainLifeAction, ActionID: "gather_dew", MainType: "collecting", ActorKind: operationeffect.ActorPlayer, StartedAt: at}
	empty := operationeffect.EmptySnapshot(op)
	payload, err := json.Marshal(queueRuntimeEnvelope{ActionID: "gather_dew", ExecutorID: "generic", Runtime: json.RawMessage(`{"durationMs":10000}`), OperationEffects: &empty})
	if err != nil {
		t.Fatal(err)
	}
	item := QueueRuntimeItem{QueueID: 1, ActionID: "gather_dew", ActorType: ActionActorTypePlayer, Spec: ActionSpec{MainType: "collecting"}, PayloadRaw: payload}
	timing := queueRunTiming{StartAt: at, CurrentDuration: 10 * time.Second, NextDuration: 10 * time.Second, HasCurrentRun: true}
	if executor.needsEffectfulGenericSettlement(item, timing, at.Add(time.Hour)) {
		t.Fatal("empty frozen snapshot permanently disabled batching")
	}
	frozen := operationeffect.EmptySnapshot(op)
	frozen.Rules = workTestSnapshot(lifeaction.KindFocusedHarvest, `{"chance":0.01,"durationReductionRate":0.03}`).Rules
	item.PayloadRaw, err = setQueueOperationEffectSnapshot(payload, frozen)
	if err != nil {
		t.Fatal(err)
	}
	if !executor.needsEffectfulGenericSettlement(item, timing, at.Add(time.Hour)) {
		t.Fatal("last paid operation lost its per-start boundary")
	}
}
