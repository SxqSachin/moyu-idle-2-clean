package alchemy_test

import (
	"encoding/json"
	"math"
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestCapacitySuccessPenaltyValidationAndBoundaries(t *testing.T) {
	resolve := func(params string) (alchemy.Modifiers, error) {
		return alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionPreciousCapacityBonus, Params: json.RawMessage(params)}}})
	}
	for _, raw := range []string{`{"extraMaterialQuantity":25,"successRatePenalty":-0.01}`, `{"extraMaterialQuantity":25,"successRatePenalty":0.201}`, `{"extraMaterialQuantity":25,"successRatePenalty":"0.06"}`} {
		if _, err := resolve(raw); err == nil {
			t.Fatal("accepted invalid penalty", raw)
		}
	}
	m, err := resolve(`{"extraMaterialQuantity":25,"successRatePenalty":0.06}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ fail, floor, want float64 }{{0, 0, .06}, {.3, 0, .36}, {.97, 0, 1}, {.82, .16, .84}} {
		if got := m.ApplyFailureChance(tc.fail, tc.floor); math.Abs(got-tc.want) > 1e-9 {
			t.Fatal(tc, got)
		}
	}
	legacy, err := resolve(`{"extraMaterialQuantity":25}`)
	if err != nil || legacy.ApplyFailureChance(.123, 0) != .123 {
		t.Fatal("legacy success changed", legacy, err)
	}
}

func TestMaterialTierValidationAndNonCompoundingCapacity(t *testing.T) {
	rule := func(raw string) operationeffect.RuleSnapshot {
		return operationeffect.RuleSnapshot{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionBulkCapacityFloorTradeoff, Params: json.RawMessage(raw)}
	}
	for _, raw := range []string{
		`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12,"materialTier":3}`,
		`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12,"materialTier":1.5}`,
		`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12,"materialTier":1,"quantityLimitBonusRate":0.201}`,
		`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12,"materialTier":2,"quantityLimitBonusRate":0.151}`,
	} {
		if _, err := alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{rule(raw)}}); err == nil {
			t.Fatal("invalid scope accepted", raw)
		}
	}
	mods, err := alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{
		rule(`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12,"materialTier":1,"quantityLimitBonusRate":0.2}`),
		rule(`{"extraMaterialQuantity":35,"positiveFloorReductionRate":0.08,"materialTier":2,"quantityLimitBonusRate":0.15}`),
		rule(`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12,"quantityLimitBonusRate":0.17}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		rarity     string
		base, want int64
	}{{"common", 10000, 2000}, {"epic", 10000, 1700}, {"mythic", 10000, 0}, {"common", 100, 50}, {"common", 1, 2}, {"common", 0, 0}} {
		if got := mods.QuantityBonus(tc.rarity, tc.base); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestResolveCombinesEveryConsumedInfusionRuleWithCaps(t *testing.T) {
	capacity, _ := json.Marshal(alchemy.CapacityFloorTradeoffParams{
		ExtraMaterialQuantity:      12,
		PositiveFloorReductionRate: 0.4,
	})
	precision, _ := json.Marshal(alchemy.PrecisionCeilingBonusParams{PositiveCeilingBonusRate: 0.2})
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{
		{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionCapacityFloorTradeoff, Params: capacity},
		{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionCapacityFloorTradeoff, Params: capacity},
		{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionPrecisionCeilingBonus, Params: precision},
		{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionPrecisionCeilingBonus, Params: precision},
	}}

	modifiers, err := alchemy.Resolve(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if modifiers.ExtraMaterialQuantity != 24 || modifiers.PositiveFloorReductionRate != 0.8 || modifiers.PositiveCeilingBonusRate != 0.25 {
		t.Fatalf("unexpected modifiers: %#v", modifiers)
	}
}

func TestResolveIgnoresOtherAlchemyKinds(t *testing.T) {
	precision, _ := json.Marshal(alchemy.PrecisionCeilingBonusParams{PositiveCeilingBonusRate: 0.1})
	snapshot := operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{
		{Domain: operationeffect.DomainAlchemy, Kind: "alchemy.future_unrelated_effect", Params: json.RawMessage(`{"future":true}`)},
		{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionPrecisionCeilingBonus, Params: precision},
	}}

	modifiers, err := alchemy.Resolve(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if modifiers.PositiveCeilingBonusRate != 0.1 {
		t.Fatalf("precision bonus=%v want 0.1", modifiers.PositiveCeilingBonusRate)
	}
}

func TestPreciousCapacityValidationScopeAndLimits(t *testing.T) {
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	h, _ := registry.Handler(alchemy.KindInfusionPreciousCapacityBonus)
	for _, raw := range []string{`{"extraMaterialQuantity":0}`, `{"extraMaterialQuantity":1.5}`, `{"extraMaterialQuantity":1000001}`, `{"extraMaterialQuantity":25,"positiveFloorReductionRate":0.1}`} {
		if h.ValidateParams(json.RawMessage(raw)) == nil {
			t.Fatalf("accepted unsafe params %s", raw)
		}
	}
	raw := json.RawMessage(`{"extraMaterialQuantity":91}`)
	if err := h.ValidateParams(raw); err != nil {
		t.Fatal(err)
	}
	op := operationeffect.Operation{Domain: operationeffect.DomainAlchemy, ActionID: "alchemy.bench", ActorKind: operationeffect.ActorPlayer, Capabilities: map[string]bool{alchemy.CapabilityInfusion: true, alchemy.CapabilityBulkMaterial: true}}
	if h.Applies(op, raw) {
		t.Fatal("low rarity consumed precious food")
	}
	op.Capabilities[alchemy.CapabilityPreciousMaterial] = true
	for _, actor := range []operationeffect.ActorKind{operationeffect.ActorPlayer, operationeffect.ActorKitty} {
		op.ActorKind = actor
		if !h.Applies(op, raw) {
			t.Fatal("shared precious effect not applied")
		}
	}
	m, err := alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionPreciousCapacityBonus, Params: raw}}})
	if err != nil || m.PreciousMaterialQuantity != 91 || m.PositiveFloorReductionRate != 0 {
		t.Fatal(m, err)
	}
	m.BulkMaterialQuantity = 363
	for _, tc := range []struct {
		rarity     string
		base, want int64
	}{{"legendary", 50, 91}, {"mythic", 10, 20}, {"epic", 100, 200}, {"unknown", 100, 0}, {"mythic", 0, 0}} {
		if got := m.QuantityBonus(tc.rarity, tc.base); got != tc.want {
			t.Fatalf("%+v got %d", tc, got)
		}
	}
}

func TestCapacityBreakthroughAndLegacyFallback(t *testing.T) {
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{alchemy.KindInfusionBulkCapacityFloorTradeoff, alchemy.KindInfusionPreciousCapacityBonus} {
		h, _ := registry.Handler(kind)
		for _, rate := range []float64{-.1, .1701, 1} {
			params := map[string]any{"extraMaterialQuantity": 25, "quantityLimitBonusRate": rate}
			if kind == alchemy.KindInfusionBulkCapacityFloorTradeoff {
				params["positiveFloorReductionRate"] = .1
			}
			raw, _ := json.Marshal(params)
			if h.ValidateParams(raw) == nil {
				t.Fatal("unsafe breakthrough accepted", kind, rate)
			}
		}
	}
	m := alchemy.Modifiers{BulkMaterialQuantity: 363, PreciousMaterialQuantity: 91, BulkQuantityLimitBonusRate: .17, PreciousQuantityLimitBonusRate: .17}
	for _, tc := range []struct {
		rarity     string
		base, want int64
	}{{"epic", 6400, 1088}, {"legendary", 6400, 1088}, {"mythic", 1600, 272}, {"epic", 200, 363}, {"mythic", 50, 91}, {"unknown", 6400, 0}} {
		if got := m.QuantityBonus(tc.rarity, tc.base); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	old, err := alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionPreciousCapacityBonus, Params: json.RawMessage(`{"extraMaterialQuantity":25}`)}}})
	if err != nil || old.QuantityBonus("mythic", 1600) != 25 {
		t.Fatal("old frozen food gained an unpromised rate", old, err)
	}
}

func TestBulkCapacityRarityAndRelativeLimits(t *testing.T) {
	m, err := alchemy.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{{Domain: operationeffect.DomainAlchemy, Kind: alchemy.KindInfusionBulkCapacityFloorTradeoff, Params: json.RawMessage(`{"extraMaterialQuantity":363,"positiveFloorReductionRate":0.12}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		rarity     string
		base, want int64
	}{{"common", 200, 363}, {"epic", 50, 100}, {"epic", 120, 240}, {"legendary", 50, 0}, {"mythic", 50, 0}, {"unknown", 200, 0}, {"rare", 0, 0}} {
		if got := m.QuantityBonus(tc.rarity, tc.base); got != tc.want {
			t.Fatalf("%+v: got %d", tc, got)
		}
	}
	if m.ExtraMaterialQuantity != 0 || m.BulkMaterialQuantity != 363 {
		t.Fatal(m)
	}
	legacy := alchemy.Modifiers{ExtraMaterialQuantity: 40}
	if legacy.QuantityBonus("mythic", 50) != 40 {
		t.Fatal("historical stamp was changed")
	}
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	h, _ := registry.Handler(alchemy.KindInfusionBulkCapacityFloorTradeoff)
	op := operationeffect.Operation{Domain: operationeffect.DomainAlchemy, ActorKind: operationeffect.ActorPlayer, ActionID: "alchemy.bench", Capabilities: map[string]bool{alchemy.CapabilityInfusion: true}}
	raw := json.RawMessage(`{"extraMaterialQuantity":50,"positiveFloorReductionRate":0.12}`)
	if h.Applies(op, raw) {
		t.Fatal("high-rarity-only infusion consumed bulk use")
	}
	op.Capabilities[alchemy.CapabilityBulkMaterial] = true
	if !h.Applies(op, raw) {
		t.Fatal("eligible mixed infusion did not match")
	}
}

func TestHandlerAppliesToPlayerAndKittyInfusionStarts(t *testing.T) {
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	handler, ok := registry.Handler(alchemy.KindInfusionPrecisionCeilingBonus)
	if !ok {
		t.Fatal("precision handler was not registered")
	}
	base := operationeffect.Operation{
		Domain:       operationeffect.DomainAlchemy,
		ActorKind:    operationeffect.ActorPlayer,
		ActionID:     "alchemy.bench",
		Capabilities: map[string]bool{alchemy.CapabilityInfusion: true},
	}
	if !handler.Applies(base, nil) {
		t.Fatal("player infusion did not match")
	}
	base.ActionID = "alchemy.convert"
	if handler.Applies(base, nil) {
		t.Fatal("alchemy conversion unexpectedly matched infusion effect")
	}
	base.ActionID = "alchemy.bench"
	base.ActorKind = operationeffect.ActorKitty
	if !handler.Applies(base, nil) {
		t.Fatal("kitty infusion did not match")
	}
}

func TestRegistryRejectsUnsafeAlchemyParams(t *testing.T) {
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	definition := operationeffect.Definition{
		ID:             "bad-capacity",
		RulesetVersion: "test-v1",
		Domain:         operationeffect.DomainAlchemy,
		Kind:           alchemy.KindInfusionCapacityFloorTradeoff,
		NameKey:        "effect.bad.name",
		DescKey:        "effect.bad.desc",
		AllowedTargets: []operationeffect.TargetKind{operationeffect.TargetPlayer},
		Lifetime:       operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1},
		Stacking:       operationeffect.StackingDefinition{Key: "bad", Mode: operationeffect.StackAddCharges},
		Params:         json.RawMessage(`{"extraMaterialQuantity":0,"positiveFloorReductionRate":0.1}`),
	}
	if err := registry.ValidateDefinition(definition); err == nil {
		t.Fatal("zero extra quantity unexpectedly passed validation")
	}
}

func TestRegistryAcceptsKittyAlchemyTarget(t *testing.T) {
	registry := operationeffect.NewRegistry()
	if err := alchemy.Register(registry); err != nil {
		t.Fatal(err)
	}
	definition := operationeffect.Definition{
		ID:             "kitty-precision",
		RulesetVersion: "test-v1",
		Domain:         operationeffect.DomainAlchemy,
		Kind:           alchemy.KindInfusionPrecisionCeilingBonus,
		NameKey:        "effect.bad.name",
		DescKey:        "effect.bad.desc",
		AllowedTargets: []operationeffect.TargetKind{operationeffect.TargetKitty},
		Lifetime:       operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1},
		Stacking:       operationeffect.StackingDefinition{Key: "bad", Mode: operationeffect.StackAddCharges},
		Params:         json.RawMessage(`{"positiveCeilingBonusRate":0.1}`),
	}
	if err := registry.ValidateDefinition(definition); err != nil {
		t.Fatal(err)
	}
}
