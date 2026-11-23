package store

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
)

func alchemyDerivedReference(totals map[string]float64) alchemyDerived {
	copy := map[string]float64{}
	for k, v := range totals {
		copy[k] = v
	}
	return alchemyDerived{Level: math.Max(1, copy["alchemy_level"]), Precision: cleanAlchemyFloat(copy["alchemy_precision"]),
		LuckyChance: clampAlchemyFloat(copy["alchemy_lucky_chance"], 0, .20), Stability: cleanAlchemyFloat(copy["alchemy_craft_success"])}
}

func TestOfflineAlchemyDerivedReadOnly(t *testing.T) {
	for _, values := range []map[string]float64{nil, {}, {"alchemy_level": 50, "alchemy_precision": 10, "alchemy_lucky_chance": .8, "alchemy_craft_success": 2},
		{"alchemy_level": -3, "alchemy_precision": math.Inf(1), "alchemy_lucky_chance": -.1, "alchemy_craft_success": math.NaN()}} {
		item := QueueRuntimeItem{WorkLoadoutRuntime: &WorkLoadoutRuntimeResolution{Available: true, AttributeTotals: values}}
		got, err := (&Store{}).resolveAlchemyDerivedForItemTx(context.Background(), nil, 1, item)
		if err != nil || !reflect.DeepEqual(got, alchemyDerivedReference(values)) {
			t.Fatal("derived values changed", got, err)
		}
		if values != nil {
			values["alchemy_level"] = 77
			next, err := (&Store{}).resolveAlchemyDerivedForItemTx(context.Background(), nil, 1, item)
			if err != nil || next.Level != 77 {
				t.Fatal("stale derived values were cached", next, err)
			}
		}
	}
}

var offlineDerivedBenchmarkResult alchemyDerived

func BenchmarkOfflineAlchemyDerived(b *testing.B) {
	values := make(map[string]float64)
	for i := 0; i < 256; i++ {
		values[fmt.Sprintf("attribute_%d", i)] = float64(i)
	}
	values["alchemy_level"] = 50
	item := QueueRuntimeItem{WorkLoadoutRuntime: &WorkLoadoutRuntimeResolution{Available: true, AttributeTotals: values}}
	s := &Store{}
	b.Run("copy_reference", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			offlineDerivedBenchmarkResult = alchemyDerivedReference(values)
		}
	})
	b.Run("read_only", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			offlineDerivedBenchmarkResult, _ = s.resolveAlchemyDerivedForItemTx(context.Background(), nil, 1, item)
		}
	})
}

func TestOfflineEnhanceEffectDoesNotMutateBaseContext(t *testing.T) {
	base := enhanceRunContext{BaseSuccessRate: .4, SuccessRateBonus: .02, SuccessRate: .42, Costs: map[string]int64{"wood": 3}, DeltaAttrs: map[string]float64{"attack": 2}}
	before := normalizeEnhanceRunContext(base)
	copy := base
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainEnhancement, Kind: "enhancement.success_bonus", Params: []byte(`{"bonusRate":0.1}`)}}}
	if err := applyEnhanceOperationEffects(&copy, snapshot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(base, before) || copy.SuccessRate != .52 {
		t.Fatal("per-start effects contaminated base context", base, copy)
	}
}
