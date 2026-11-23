package store

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/transmute"
)

func TestOfflineAlchemySegmentTransmuteKernel(t *testing.T) {
	s := fishCuisineTestStore(t)
	rule := s.gameData.AlchemyTransmuteRuleForItemID("wood")
	rule.FailChanceBase, rule.MinFailChance, rule.LuckyChanceBase, rule.MinAlchemyLevel = .31, .02, .72, 1
	rule.LuckyTiers = []catalog.AlchemyTransmuteTier{{ID: "normal", Kind: "gold_multiplier", Weight: 3, Value: 2.3}, {ID: "large", Kind: "gold_multiplier", Weight: 1, Value: 11.7}}
	s.gameData.ItemByID("wood").EconomyBaseValue = 99.5
	derived := alchemyDerived{Level: 15, LuckyChance: .17, Stability: .43}
	for _, ratios := range []catalog.AlchemyReturnRatio{{MinRatio: .13, MaxRatio: 1.91}, {MinRatio: 1.1, MaxRatio: 1.1}, {MinRatio: 2.1, MaxRatio: .7}} {
		rule.ReturnRatio = ratios
		for _, bonusRate := range []float64{.04, .1} {
			params, _ := json.Marshal(transmute.Params{BonusRate: bonusRate})
			snapshot := operationeffect.Snapshot{SchemaVersion: operationeffect.SnapshotSchemaVersion, OperationKey: "kernel", Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: transmute.Gold, Params: params}}}
			run, err := marshalAlchemyRunContext(alchemyTransmuteArgs{ItemID: "wood"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(queueRuntimeEnvelope{RunContext: run, OperationEffects: &snapshot})
			if err != nil {
				t.Fatal(err)
			}
			item := QueueRuntimeItem{ActionID: "alchemy.transmute", PayloadRaw: payload, alchemyReads: &offlineAlchemyReads{derived: &derived}}
			p, ok, err := (&alchemyExecutor{store: s, actionID: item.ActionID}).compileAlchemySegmentTx(context.Background(), nil, 1, item)
			if err != nil || !ok {
				t.Fatal(ok, err)
			}
			failures, successes, lucky := 0, 0, 0
			for seed := int64(0); seed < 2048; seed++ {
				got, err := p.roll(rand.New(rand.NewSource(seed)))
				if err != nil {
					t.Fatal(err)
				}
				// Frozen single-operation arithmetic, including the original
				// RNG branch/draw order and bonus based on PRE-luck gold.
				rng := rand.New(rand.NewSource(seed))
				fail := applyAlchemyMinimumLossFailChance(applyAlchemyStabilityToFailChance(rule.FailChanceBase, derived.Stability), rule.MinFailChance)
				want := alchemySegmentRoll{}
				if rng.Float64() < fail {
					want.failed = true
					failures++
				} else {
					successes++
					base := max(int64(1), int64(math.Floor(99.5*sampleLuckSkewed(ratios.MinRatio, ratios.MaxRatio, derived.Stability, rng))))
					gold := base
					if rng.Float64() < clampAlchemyFloat(rule.LuckyChanceBase+derived.LuckyChance, 0, 1) {
						lucky++
						tier := rollAlchemyTier(rule.LuckyTiers, derived.LuckyChance, rng)
						if tier.Kind == "gold_multiplier" {
							gold = max(gold, int64(math.Floor(float64(base)*sampleLuckSkewed(1, math.Max(1, tier.Value), derived.LuckyChance*2, rng))))
						}
					}
					want = alchemySegmentRoll{itemID: GoldCoinItemID, qty: gold + transmute.BonusGold(base, bonusRate), bonus: transmute.BonusGold(base, bonusRate)}
				}
				if got != want {
					t.Fatalf("seed=%d got=%+v want=%+v", seed, got, want)
				}
			}
			if failures == 0 || successes == 0 || lucky == 0 {
				t.Fatal("missing branch coverage")
			}
		}
	}
}

func TestOfflineAlchemySegmentSummaryAdditionOrder(t *testing.T) {
	for _, gain := range []float64{.0001, .1, .3333, 0, -0.1} {
		for _, initial := range []float64{0, .1, 1e12} {
			want := initial
			for range 1024 {
				want += gain
			}
			got := map[string]float64{"alchemy_level": initial}
			for _, count := range []int{17, 513, 494} {
				accumulateSettlementAttrExp(got, SettleResult{Completed: count, exactSingleAttrExp: map[string]float64{"alchemy_level": gain}}, false)
			}
			if math.Float64bits(got["alchemy_level"]) != math.Float64bits(want) {
				t.Fatal("partition changed floating-point summary", gain, initial, want, got)
			}
		}
	}
	got := map[string]float64{}
	accumulateSettlementAttrExp(got, SettleResult{AttrExp: map[string]float64{"zero": 0}}, true)
	if len(got) != 0 {
		t.Fatal("legacy actor summary gained zero-valued keys")
	}
}

func TestOfflineAlchemySegmentWeightsPreserveDraws(t *testing.T) {
	for _, weights := range []map[string]float64{nil, {"zero": 0, "negative": -1}, {"b": 0.25, "a": 0.5, "d": 3, "c": -1}} {
		roll := compileAlchemySegmentWeights(weights)
		reused := rand.New(rand.NewSource(1))
		for seed := int64(0); seed < 1024; seed++ {
			reused.Seed(seed)
			reference := rand.New(rand.NewSource(seed))
			if roll(reused) != rollByWeight(weights, reference) || reused.Float64() != reference.Float64() {
				t.Fatal("weighted selection or RNG stream changed", weights, seed)
			}
		}
	}
}

func TestOfflineAlchemySegmentFrozenParamsAndBudget(t *testing.T) {
	rule := operationeffect.RuleSnapshot{GrantID: 7, Kind: transmute.Gold, Params: []byte(`{"bonusRate":0.1}`)}
	other := rule
	other.Params = []byte(`{ "bonusRate": 0.10 }`)
	if !alchemySegmentRulesEqual([]operationeffect.RuleSnapshot{rule}, []operationeffect.RuleSnapshot{other}) {
		t.Fatal("jsonb formatting changed contract")
	}
	other.GrantID++
	if alchemySegmentRulesEqual([]operationeffect.RuleSnapshot{rule}, []operationeffect.RuleSnapshot{other}) {
		t.Fatal("merged distinct grants")
	}
	item := QueueRuntimeItem{alchemyRandomSeed: func(ordinal int) int64 { return int64(ordinal) }}
	if !reflect.DeepEqual(alchemySingleRunRNG(item, 17), rand.New(rand.NewSource(17))) {
		t.Fatal("random input changed")
	}
	s := fishCuisineTestStore(t)
	item = QueueRuntimeItem{ActorType: ActionActorTypePlayer, ActionID: "alchemy.transmute", PreserveScheduledTimeline: true}
	if _, handled, err := (&alchemyExecutor{store: s, actionID: item.ActionID}).trySettleExactSegmentTx(context.Background(), nil, 1, item, time.Now(), FastForwardBudget{MaxIterations: 1}); err != nil || handled {
		t.Fatal("single-step budget entered segment", handled, err)
	}
}
