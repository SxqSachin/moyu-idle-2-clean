package alchemy

import (
	"math"
	"mysecondwebgame/server/internal/operationeffect"
	"testing"
	"time"
)

func TestConversionFoodKeepsLossRiskAndRejectsUnsafeRates(t *testing.T) {
	for _, rate := range []string{`{"lossReductionRate":0}`, `{"lossReductionRate":0.101}`, `{"lossReductionRate":0.05,"bonus":1}`} {
		if (conversionHandler{}).ValidateParams([]byte(rate)) == nil {
			t.Fatal(rate)
		}
	}
	for _, tc := range []struct{ base, rate, want float64 }{{.1865, .05, .177175}, {.1865, .1, .16785}, {.15, .1, .135}} {
		raw := []byte(`{"lossReductionRate":0.05}`)
		if tc.rate == .1 {
			raw = []byte(`{"lossReductionRate":0.1}`)
		}
		snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: KindConversionLossReduction, Params: raw}}}
		got, err := ConversionFailureChance(tc.base, snapshot)
		if err != nil || math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("%v: %v %v", tc, got, err)
		}
	}
}

func TestConversionFoodOnlySpendsAtMatchingStarts(t *testing.T) {
	r := operationeffect.NewRegistry()
	if err := Register(r); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := operationeffect.Definition{ID: "convert", RulesetVersion: "test", Domain: operationeffect.DomainAlchemy, Kind: KindConversionLossReduction, NameKey: "test", DescKey: "test", AllowedTargets: []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}, Selector: operationeffect.Selector{ActionIDs: []string{"alchemy.convert"}}, Params: []byte(`{"lossReductionRate":0.05}`), Lifetime: operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 2}, Stacking: operationeffect.StackingDefinition{Key: "food.conversion", Exclusive: true, Mode: operationeffect.StackAddCharges}}
	session, err := operationeffect.NewSession(r, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at, InitialCharges: 2, RemainingCharges: 2}})
	if err != nil {
		t.Fatal(err)
	}
	for i, action := range []string{"alchemy.bench", "alchemy.transmute", "alchemy.convert", "alchemy.convert", "alchemy.convert"} {
		op := operationeffect.Operation{Key: action + time.Duration(i).String(), Domain: operationeffect.DomainAlchemy, ActionID: action, StartedAt: at, ActorKind: operationeffect.ActorPlayer}
		if i == 3 {
			op.ActorKind = operationeffect.ActorKitty
			op.ActorID = 2
		}
		snap, err := session.Commit(session.Prepare(op))
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if i == 2 || i == 3 {
			want = 1
		}
		if len(snap.Rules) != want {
			t.Fatal(i, snap)
		}
		if _, err := session.Commit(session.Prepare(op)); err != nil {
			t.Fatal(err)
		}
	}
	if len(session.ChargeReceipts()) != 2 {
		t.Fatal("retry or actor duplicated charges")
	}
}
