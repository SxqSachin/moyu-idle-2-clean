package store

import (
	"context"
	"encoding/json"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/enchantment"
	"mysecondwebgame/server/internal/operationeffect/transmute"
	"reflect"
	"testing"
	"time"
)

func TestNewCuisineQualityServingRoundTrip(t *testing.T) {
	s := fishCuisineTestStore(t)
	for _, tc := range []struct {
		id                         string
		plain, mutated             int64
		param                      string
		base, maxPlain, maxMutated float64
	}{
		{"honeyflame_quick_grill", 40, 80, "durationSeconds", 1, 1, 1},
		{"sunhoney_gold_grill", 24, 50, "bonusRate", .04, .06, .10},
		{"sunhoney_gold_soup", 0, 0, "bonusRate", .04, .06, .10},
		{"moonglass_dual_fish_grill", 1, 1, "weightBonusRate", .10, .15, .25},
	} {
		recipe := *s.gameData.FishFoodRecipeByOutputItemID(tc.id)
		def, _, err := s.resolveOperationEffectDefinitionForItem(tc.id, nil)
		if err != nil {
			t.Fatal(err)
		}
		var standard map[string]float64
		_ = json.Unmarshal(def.Params, &standard)
		if standard[tc.param] != tc.base {
			t.Fatal(tc.id, standard)
		}
		for _, mutated := range []bool{false, true} {
			var mutations []string
			if mutated {
				mutations = []string{"shimmering", "prismatic", "rainbow", "shimmering"}
			}
			output, err := newFishFoodExecutor(s, recipe).buildSpecificOutputSnapshot(recipeTestFish(s, recipe, 1, 1.25, mutations))
			if err != nil {
				t.Fatal(tc.id, err)
			}
			view, ok, err := s.OperationEffectDefinitionForItem(tc.id, output)
			if err != nil || !ok {
				t.Fatal(tc.id, err)
			}
			var params map[string]float64
			_ = json.Unmarshal(view.Params, &params)
			want, charges := tc.maxPlain, tc.plain
			if mutated {
				want, charges = tc.maxMutated, tc.mutated
			}
			if params[tc.param] != want || view.Lifetime.Charges != charges {
				t.Fatal(tc.id, mutated, params, view.Lifetime)
			}
			if recipe.Kind == "fish_soup" {
				seconds := int64(600)
				if mutated {
					seconds = 1500
				}
				if view.Lifetime.DurationSeconds != seconds {
					t.Fatal(view.Lifetime)
				}
			}
		}
	}
}

func TestTransmuteFoodOneSecondWindowPreservesLegacyFloor(t *testing.T) {
	if (&alchemyExecutor{actionID: "alchemy.transmute"}).durationRefreshMode().allows("running") {
		t.Fatal("attribute changes must not rewrite a started transmute duration")
	}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	snapshot := operationeffect.Snapshot{SchemaVersion: operationeffect.SnapshotSchemaVersion, OperationKey: "queue:1:1", StartedAt: at,
		Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: transmute.Speed, Params: json.RawMessage(`{"durationSeconds":1}`)}}}
	for _, tc := range []struct {
		name, executor string
		effect         bool
		want           time.Duration
	}{
		{"frozen speed", "alchemy.transmute", true, time.Second},
		{"legacy transmute", "alchemy.transmute", false, 2 * time.Second},
		{"other executor", "generic", true, 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := queueRuntimeEnvelope{ExecutorID: tc.executor, Runtime: json.RawMessage(`{"durationMs":1000}`)}
			if tc.effect {
				env.OperationEffects = &snapshot
			}
			payload, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			duration := parseQueueRuntimeDuration(payload)
			if duration != tc.want {
				t.Fatalf("start/display duration=%v, want %v", duration, tc.want)
			}
			ends := at.Add(duration)
			item := QueueRuntimeItem{PayloadRaw: payload, RunStartAt: &at, RunEndsAt: &ends}
			timing := resolveQueueRunTiming(item, at.Add(time.Hour), duration)
			if timing.NextDuration != tc.want {
				t.Fatal(timing)
			}
			completed, completedAt := completedRunsForQueueTiming(timing, at.Add(20*tc.want), 20)
			if completed != 20 || !completedAt.Equal(at.Add(20*tc.want)) {
				t.Fatal(completed, completedAt)
			}
			for _, offline := range []bool{false, true} {
				start, end := nextQueueRunWindow(at.Add(time.Hour), SettleResult{NextRunStartAt: ends, NextRunEndsAt: ends.Add(duration)}, payload, offline, offline)
				if end.Sub(start) != tc.want {
					t.Fatal("carry duration", offline, end.Sub(start))
				}
			}
		})
	}
}

func TestTransmuteActualStartScopeAndOfflineBoundary(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.transmute"}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	operation := func(seq int, id string, started time.Time) operationeffect.Operation {
		run, _ := json.Marshal(alchemyTransmuteArgs{ItemID: id})
		payload, _ := json.Marshal(queueRuntimeEnvelope{RunContext: run})
		return e.EnrichOperationEffectContext(queueOperationEffectOperation(operationeffect.DomainAlchemy, QueueRuntimeItem{QueueID: 9, ActionID: e.actionID, ActorType: ActionActorTypeKitty, ActorID: 2}, started, seq, payload), payload)
	}
	def := s.operationEffectDefinitions["food_sunhoney_gold_grill_v1"]
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, InitialCharges: 12, RemainingCharges: 12}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"gold_coin", "missing_item", "map_missing"} {
		if p := session.Prepare(operation(0, id, at)); len(p.Snapshot().Rules) > 0 {
			t.Fatal("unapproved input", id)
		}
	}
	for seq := 1; seq <= 13; seq++ {
		ids := []string{"wood", "bottled_starlight", "sunhoney_gold_grill", "wolf_king_fang"}
		op := operation(seq, ids[(seq-1)%len(ids)], at.Add(time.Duration(seq)*time.Second))
		snap, err := session.Commit(session.Prepare(op))
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if seq == 13 {
			want = 0
		}
		if len(snap.Rules) != want {
			t.Fatal(seq, snap)
		}
		retry, err := session.Commit(session.Prepare(op))
		if err != nil || !reflect.DeepEqual(retry, snap) {
			t.Fatal("replayed charge", err)
		}
	}
	if len(session.ChargeReceipts()) != 12 {
		t.Fatal("shared charges")
	}
	def = s.operationEffectDefinitions["food_sunhoney_gold_soup_v1"]
	until := at.Add(600 * time.Second)
	session, err = operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 2, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: 2}, ValidFrom: at, ValidUntil: &until}})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := session.Commit(session.Prepare(operation(1, "wood", until.Add(-time.Second))))
	if err != nil {
		t.Fatal(err)
	}
	mods, err := transmute.Resolve(frozen)
	if err != nil || mods.BonusRate != .04 {
		t.Fatal(mods, err)
	}
	if snap, _ := session.Commit(session.Prepare(operation(2, "wolf_king_fang", until))); len(snap.Rules) != 0 {
		t.Fatal("expired effect")
	}
	if snap, _ := session.Commit(session.Prepare(operation(3, "wolf_king_fang", at.Add(-time.Second)))); len(snap.Rules) != 0 {
		t.Fatal("retroactive effect")
	}
}

func TestEnchantFoodOneUseAndNoUnchangedCandidateCharge(t *testing.T) {
	s := fishCuisineTestStore(t)
	def := s.operationEffectDefinitions["food_moonglass_dual_fish_grill_v1"]
	at := time.Now()
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1}})
	if err != nil {
		t.Fatal(err)
	}
	op := operationeffect.Operation{Key: "enchant:1", Domain: enchantment.Domain, ActionID: "equipment.enchant", ActorKind: operationeffect.ActorPlayer, StartedAt: at, Capabilities: map[string]bool{}}
	if snap, _ := session.Commit(session.Prepare(op)); len(snap.Rules) != 0 {
		t.Fatal("empty candidates spent use")
	}
	op.Key = "enchant:2"
	op.Capabilities[enchantment.Eligible] = true
	snap, err := session.Commit(session.Prepare(op))
	if err != nil || len(snap.Rules) != 1 {
		t.Fatal(snap, err)
	}
	op.Key = "enchant:3"
	if snap, _ := session.Commit(session.Prepare(op)); len(snap.Rules) != 0 {
		t.Fatal("second use")
	}
}

func TestTransmuteFoodEligibilityMatchesLegalInputsWithoutShopLookup(t *testing.T) {
	s := fishCuisineTestStore(t)
	e := &alchemyExecutor{store: s, actionID: "alchemy.transmute"}
	for _, item := range s.gameData.Items {
		want := s.itemAllowedForAlchemyTransmute(&item) && s.gameData.AlchemyTransmuteRuleForItemID(item.ID) != nil
		eligible, err := s.TransmuteFoodEligible(context.Background(), item.ID)
		if err != nil || eligible != want {
			t.Fatal("preview differs from point-gold rules", item.ID, eligible, want, err)
		}
		run, _ := json.Marshal(alchemyTransmuteArgs{ItemID: item.ID})
		payload, _ := json.Marshal(queueRuntimeEnvelope{RunContext: run})
		op := e.EnrichOperationEffectContext(operationeffect.Operation{Domain: operationeffect.DomainAlchemy, ActionID: "alchemy.transmute"}, payload)
		if op.Capabilities[transmute.GoldEligible] != want {
			t.Fatal("actual start differs from preview", item.ID)
		}
	}
}
