package ratebonus

import (
	"encoding/json"
	"math"
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
)

func TestRateBonusValidationAndDomainBoundaries(t *testing.T) {
	r := operationeffect.NewRegistry()
	if err := Register(r); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{EnhanceSuccess, FishingBite} {
		h, _ := r.Handler(kind)
		for _, raw := range []string{`{"bonusRate":-1}`, `{"bonusRate":2}`, `{"bonusRate":0.01,"other":1}`, `{"bonusRate":0.01} {}`} {
			if h.ValidateParams(json.RawMessage(raw)) == nil {
				t.Fatalf("accepted %s", raw)
			}
		}
		op := operationeffect.Operation{Domain: h.Domain(), ActorKind: operationeffect.ActorPlayer, ActionID: "equipment_enhance"}
		if kind == FishingBite {
			op.ActionID = "fishing.trip"
		}
		if !h.Applies(op, nil) {
			t.Fatal("matching operation rejected")
		}
		op.ActorKind = operationeffect.ActorKitty
		if !h.Applies(op, nil) {
			t.Fatal("kitty operation did not match")
		}
		op.ActorKind = operationeffect.ActorPlayer
		op.Domain = operationeffect.DomainLifeAction
		if h.Applies(op, nil) {
			t.Fatal("cross-domain match")
		}
	}
}

func TestRateBonusEmptyAndBoundedFold(t *testing.T) {
	if bonus, err := Resolve(operationeffect.Snapshot{}, EnhanceSuccess); err != nil || bonus != 0 {
		t.Fatal("empty snapshot changed behavior")
	}
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainEnhancement, Kind: EnhanceSuccess, Params: json.RawMessage(`{"bonusRate":0.07}`)}, {Domain: operationeffect.DomainEnhancement, Kind: EnhanceSuccess, Params: json.RawMessage(`{"bonusRate":0.07}`)}}}
	if bonus, err := Resolve(snapshot, EnhanceSuccess); err != nil || math.Abs(bonus-0.1) > 1e-9 {
		t.Fatal("hard ceiling not enforced")
	}
}
