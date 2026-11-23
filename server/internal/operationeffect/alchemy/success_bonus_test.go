package alchemy_test

import (
	"encoding/json"
	"math"
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestInfusionSuccessBonusValidationScopeAndIsolation(t *testing.T) {
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	h, _ := registry.Handler(alchemy.KindInfusionSuccessBonus)
	for _, raw := range []string{`{}`, `{"bonusRate":0}`, `{"bonusRate":-0.01}`, `{"bonusRate":0.101}`, `{"bonusRate":"0.03"}`, `{"bonusRate":0.03,"extraMaterialQuantity":1}`} {
		if h.ValidateParams(json.RawMessage(raw)) == nil {
			t.Fatal("accepted invalid success bonus", raw)
		}
	}
	raw := json.RawMessage(`{"bonusRate":0.03}`)
	for _, actor := range []operationeffect.ActorKind{operationeffect.ActorPlayer, operationeffect.ActorKitty} {
		op := operationeffect.Operation{Domain: operationeffect.DomainAlchemy, ActionID: "alchemy.bench", ActorKind: actor, Capabilities: map[string]bool{alchemy.CapabilityInfusion: true}}
		if !h.Applies(op, raw) {
			t.Fatal("ordinary infusion excluded", actor)
		}
		op.Capabilities = nil
		if h.Applies(op, raw) {
			t.Fatal("fish infusion or empty selection matched")
		}
		op.Capabilities = map[string]bool{alchemy.CapabilityInfusion: true}
		op.ActionID = "alchemy.convert"
		if h.Applies(op, raw) {
			t.Fatal("conversion matched")
		}
	}
	rule := operationeffect.RuleSnapshot{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionSuccessBonus, Params: raw}
	m, err := alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{rule, rule, rule, rule}})
	if err != nil || m != (alchemy.Modifiers{SuccessRateBonus: .10}) {
		t.Fatal("bonus leaks into capacity or ranges", m, err)
	}
}

func TestInfusionSuccessBonusCombinesBeforeClamping(t *testing.T) {
	for _, tc := range []struct{ fail, penalty, bonus, floor, want float64 }{
		{.08, 0, .03, 0, .05}, {.08, .06, .03, 0, .11}, {.01, 0, .03, 0, 0},
		{0, .04, .03, 0, .01}, {.82, .06, .03, .16, .84}, {.82, .06, .06, .16, .82},
		{.2, 0, .2, 0, .10}, {.123, 0, 0, 0, .123},
	} {
		m := alchemy.Modifiers{SuccessRateBonus: tc.bonus, SuccessRatePenalty: tc.penalty}
		if got := m.ApplyFailureChance(tc.fail, tc.floor); math.Abs(got-tc.want) > 1e-9 {
			t.Fatal(tc, got)
		}
	}
}
