package store

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestCreamfinFoodQualityAndSuccessOnlyStamp(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, id := range []string{"creamfin_dual_fish_grill", "creamfin_dual_fish_soup"} {
		recipe := *s.gameData.FishFoodRecipeByOutputItemID(id)
		def, ok, err := s.resolveOperationEffectDefinitionForItem(id, nil)
		if err != nil || !ok {
			t.Fatal(id, err)
		}
		for _, tc := range []struct{ length, weight, trait, mutation, want float64 }{
			{.5, .5, 0, 1, .05}, {0, 0, -1, 1, .0275}, {1, 1, 1, 1, .0725},
			{1, .5, 0, 1, .06}, {.5, 1, 0, 1, .0525}, {1, 1, 1, 1.815, .10},
		} {
			q := fishFoodQualitySnapshot{LengthScore: tc.length, WeightScore: tc.weight, TraitAverage: tc.trait, SpecialMultiplier: tc.mutation}
			raw, err := scaleFishFoodParamsV2(def.Params, recipe.ParameterScaling, &q)
			if err != nil {
				t.Fatal(err)
			}
			var params map[string]float64
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatal(err)
			}
			if len(params) != 1 || params["bonusRate"] != tc.want {
				t.Fatal(id, tc, params)
			}
			frozen := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: def.Domain, Kind: def.Kind, DefinitionID: def.ID, RulesetVersion: def.RulesetVersion, Params: raw}}}
			craft := &resolvedAlchemyCraft{Components: []alchemyCraftComponent{{}}}
			if err := stampAlchemyCraftOperationEffects(craft, frozen); err != nil {
				t.Fatal(err)
			}
			m, err := resolveAlchemyOperationEffectStamp(craft.Components[0].OperationEffects)
			if err != nil || m != (alchemyeffect.Modifiers{SuccessRateBonus: tc.want}) {
				t.Fatal(m, err)
			}
		}
	}
}

func TestSuccessFoodChangesOnlyProbabilityForTraitAndCombo(t *testing.T) {
	for _, combo := range []string{"", "test_combo"} {
		s := arenaAlchemyGateTestStore()
		if combo == "" {
			s.gameData.AlchemyComboRecipes = nil
		}
		derived := alchemyDerived{Level: 50, Stability: 0}
		materials := map[string]int64{"test_material": 1}
		base, err := s.resolveAlchemyCraftWithModifiers(materials, combo, nil, derived, alchemyeffect.Modifiers{}, rand.New(rand.NewSource(3)))
		if err != nil {
			t.Fatal(err)
		}
		food, err := s.resolveAlchemyCraftWithModifiers(materials, combo, nil, derived, alchemyeffect.Modifiers{SuccessRateBonus: .03}, rand.New(rand.NewSource(3)))
		if err != nil || math.Abs(food.FailChance-math.Max(0, base.FailChance-.03)) > 1e-9 {
			t.Fatal(combo, base.FailChance, food.FailChance, err)
		}
		food.FailChance = base.FailChance
		if !reflect.DeepEqual(base, food) {
			t.Fatal("success food changed generated numeric values", combo)
		}
		baseFloor := s.computeFloorAttrModsWithModifiers(materials, combo, nil, derived, alchemyeffect.Modifiers{})
		foodFloor := s.computeFloorAttrModsWithModifiers(materials, combo, nil, derived, alchemyeffect.Modifiers{SuccessRateBonus: .06})
		if !reflect.DeepEqual(baseFloor, foodFloor) {
			t.Fatal("failure results changed", combo)
		}
	}
}

func TestSuccessFoodActualQueueScopeAndOfflineFrozenExpiry(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.bench"}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	op := func(seq int, start time.Time, actor string, args alchemyBenchArgs) operationeffect.Operation {
		run, _ := json.Marshal(args)
		raw, _ := json.Marshal(queueRuntimeEnvelope{RunContext: run})
		item := QueueRuntimeItem{QueueID: 12, ActionID: "alchemy.bench", ActorType: actor, ActorID: 2}
		return enrichOperationEffectContext(e, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, start, seq, raw), raw)
	}
	def := s.operationEffectDefinitions["food_creamfin_dual_fish_grill_v1"]
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, InitialCharges: 6, RemainingCharges: 6}})
	if err != nil {
		t.Fatal(err)
	}
	fish := op(0, at, ActionActorTypePlayer, alchemyBenchArgs{FishInstanceID: 123})
	if snapshot, err := session.Commit(session.Prepare(fish)); err != nil || len(snapshot.Rules) != 0 {
		t.Fatal("fish spent a use", snapshot, err)
	}
	for i := 1; i <= 7; i++ {
		actor := ActionActorTypePlayer
		if i%2 == 0 {
			actor = ActionActorTypeKitty
		}
		operation := op(i, at.Add(time.Duration(i)*time.Second), actor, alchemyBenchArgs{Materials: map[string]int64{"coal": 1, "base2_moss_spore": 1}})
		snapshot, err := session.Commit(session.Prepare(operation))
		want := 1
		if i == 7 {
			want = 0
		}
		if err != nil || len(snapshot.Rules) != want {
			t.Fatal("shared operation count", i, snapshot, err)
		}
		retry, err := session.Commit(session.Prepare(operation))
		if err != nil || !reflect.DeepEqual(retry, snapshot) {
			t.Fatal("retry spent twice", err)
		}
	}
	if len(session.ChargeReceipts()) != 6 {
		t.Fatal("wrong shared ledger")
	}
	def = s.operationEffectDefinitions["food_creamfin_dual_fish_soup_v1"]
	until := at.Add(10 * time.Minute)
	session, err = operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 2, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, ValidUntil: &until}})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := session.Commit(session.Prepare(op(1, until.Add(-time.Second), ActionActorTypeKitty, alchemyBenchArgs{Materials: map[string]int64{"coal": 1}})))
	if err != nil || len(frozen.Rules) != 1 {
		t.Fatal(err, frozen)
	}
	after, err := session.Commit(session.Prepare(op(2, until, ActionActorTypePlayer, alchemyBenchArgs{Materials: map[string]int64{"coal": 1}})))
	if err != nil || len(after.Rules) != 0 {
		t.Fatal("expired soup applied", err)
	}
	encoded, _ := json.Marshal(frozen)
	var restored operationeffect.Snapshot
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	m, err := alchemyeffect.Resolve(restored)
	if err != nil || m.SuccessRateBonus != .03 {
		t.Fatal("pre-expiry offline start lost its frozen effect", m, err)
	}
}
