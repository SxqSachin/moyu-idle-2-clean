package transmute

import (
	"math"
	"mysecondwebgame/server/internal/operationeffect"
	"testing"
)

func TestBonusGoldBoundsAndPerItemRounding(t *testing.T) {
	for _, tc := range []struct {
		gold int64
		rate float64
		want int64
	}{{99, .04, 3}, {1, .10, 0}, {1000, .10, 100}, {math.MaxInt64, .10, 922337203685477580}, {100, .11, 0}, {100, math.NaN(), 0}} {
		if got := BonusGold(tc.gold, tc.rate); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	if BonusGold(99, .04)*2 == BonusGold(198, .04) {
		t.Fatal("fixture must detect aggregate rounding")
	}
}
func TestTransmuteValidationAndScope(t *testing.T) {
	r := operationeffect.NewRegistry()
	if err := Register(r); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"durationSeconds":0}`, `{"durationSeconds":0.5}`, `{"durationSeconds":1,"bonusRate":0.1}`, `{"durationSeconds":1,"unknown":1}`} {
		if (handler{Speed}).ValidateParams([]byte(raw)) == nil {
			t.Fatal(raw)
		}
	}
	op := operationeffect.Operation{Domain: operationeffect.DomainAlchemy, ActionID: "alchemy.transmute", ActorKind: operationeffect.ActorKitty, Capabilities: map[string]bool{GoldEligible: true}}
	if !(handler{Gold}).Applies(op, nil) || !(handler{Speed}).Applies(op, nil) {
		t.Fatal("kitty not supported")
	}
	op.Capabilities[GoldEligible] = false
	if (handler{Gold}).Applies(op, nil) {
		t.Fatal("unapproved input")
	}
	op.ActionID = "alchemy.bench"
	if (handler{Speed}).Applies(op, nil) {
		t.Fatal("infusion changed")
	}
}
