package enchantment

import (
	"math"
	"testing"
)

func TestOnlyCompatibleHighWeightsChange(t *testing.T) {
	weights := map[string]float64{"uncommon": 60, "rare": 28, "epic": 8, "legendary": 4}
	for _, bonus := range []float64{.10, .25} {
		got := AdjustWeights(weights, bonus)
		if got["uncommon"] != 60 || got["rare"] != 28 || got["epic"] != 8*(1+bonus) || got["legendary"] != 4*(1+bonus) || len(got) != 4 || weights["epic"] != 8 {
			t.Fatal(got)
		}
		if bonus == .25 && math.Abs(got["legendary"]/(60+28+10+5)-.0485436893) > 1e-9 {
			t.Fatal(got)
		}
	}
	for _, w := range []map[string]float64{{"epic": 8, "legendary": 4}, {"rare": 100}, {"rare": 0, "legendary": 4}, {}} {
		if EligibleWeights(w) {
			t.Fatal("unchanged distribution eligible", w)
		}
		for k, v := range AdjustWeights(w, .25) {
			if w[k] != v {
				t.Fatal("unchanged distribution modified")
			}
		}
	}
	for _, raw := range []string{`{"weightBonusRate":0}`, `{"weightBonusRate":0.26}`, `{"weightBonusRate":0.1,"chance":1}`} {
		if _, err := decode([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}
