package fishing

import (
	"encoding/json"
	"mysecondwebgame/server/internal/operationeffect"
	"testing"
)

func TestStrictParamsAndIndependentCaps(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"rate":0}`, `{"rate":-0.1}`, `{"rate":0.151}`, `{"rate":0.1,"other":1}`, `{"rate":0.1} {}`} {
		if _, err := decode(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, rate := range []string{`{"rate":0.005}`, `{"rate":0.15}`} {
		if _, err := decode(json.RawMessage(rate)); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := operationeffect.Snapshot{}
	for _, kind := range []string{LandingBonus, LandingBonus, LineLoadReduction} {
		snapshot.Rules = append(snapshot.Rules, operationeffect.RuleSnapshot{Domain: operationeffect.DomainFishingCast, Kind: kind, Params: json.RawMessage(`{"rate":0.1}`)})
	}
	got, err := Resolve(snapshot)
	if err != nil || got.LandingBonusRate != 0.15 || got.LineLoadReductionRate != 0.1 {
		t.Fatalf("%+v %v", got, err)
	}
	if !(handler{}).Applies(operationeffect.Operation{Domain: operationeffect.DomainFishingCast, ActionID: "fishing.trip", ActorKind: operationeffect.ActorKitty}, nil) {
		t.Fatal("kitty cast did not match")
	}
	for _, op := range []operationeffect.Operation{{Domain: operationeffect.DomainLifeAction, ActionID: "fishing.trip", ActorKind: operationeffect.ActorPlayer}, {Domain: operationeffect.DomainFishingCast, ActionID: "unrelated", ActorKind: operationeffect.ActorKitty}} {
		if (handler{}).Applies(op, nil) {
			t.Fatal("foreign action matched")
		}
	}
}
