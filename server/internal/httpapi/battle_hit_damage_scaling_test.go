package httpapi

import (
	"math"
	"testing"
)

func TestBattleHitDamageScalingGoldenVectors(t *testing.T) {
	scaling := &battleHitDamageScaling{
		Kind:                battleHitDamageScalingKindPoweredExponentialDecay,
		SecondHitMultiplier: 0.998,
		CurvePower:          2.1708,
	}
	tests := []struct {
		hitIndex int
		want     float64
	}{
		{hitIndex: 1, want: 1},
		{hitIndex: 2, want: 0.998},
		{hitIndex: 3, want: 0.9910260309249609},
		{hitIndex: 5, want: 0.9602229711864223},
		{hitIndex: 10, want: 0.7897711078627216},
		{hitIndex: 20, want: 0.30269140703314235},
		{hitIndex: 40, want: 0.003369181516522036},
		{hitIndex: 64, want: 9.941009501614309e-8},
	}
	for _, test := range tests {
		got := battleResolveHitDamageMultiplier(scaling, test.hitIndex)
		if math.Abs(got-test.want) > 1e-12 {
			t.Fatalf("hit %d multiplier = %.16g, want %.16g", test.hitIndex, got, test.want)
		}
	}
	tail := battleResolveHitDamageMultiplier(scaling, 64)
	compact := compactBattleTimelineStepMeta(map[string]any{"hitDamageMultiplier": tail})
	if got := toFloat64Loose(compact["hm"], 0); got != tail || got == 0 {
		t.Fatalf("compact tail multiplier = %.16g, want preserved %.16g", got, tail)
	}
}

func TestBattleHitDamageScalingParserAndSerializerAreStrict(t *testing.T) {
	valid := map[string]any{
		"kind":                "powered_exponential_decay",
		"secondHitMultiplier": 0.998,
		"curvePower":          2.1708,
	}
	spec := parseBattleDamageSpec(map[string]any{
		"damageType":       "magical",
		"hitDamageScaling": valid,
	}, "physical", battleSkillAmountFormula{BaseAdd: 10})
	if spec.HitDamageScaling == nil {
		t.Fatal("valid hitDamageScaling was dropped")
	}
	if spec.HitDamageScaling.Kind != battleHitDamageScalingKindPoweredExponentialDecay ||
		spec.HitDamageScaling.SecondHitMultiplier != 0.998 || spec.HitDamageScaling.CurvePower != 2.1708 {
		t.Fatalf("parsed hitDamageScaling = %#v", spec.HitDamageScaling)
	}

	serialized := battleDamageSpecToAnyMap(spec)
	serializedScaling := normalizeAnyMap(serialized["hitDamageScaling"])
	if serializedScaling["kind"] != battleHitDamageScalingKindPoweredExponentialDecay ||
		toFloat64Loose(serializedScaling["secondHitMultiplier"], 0) != 0.998 ||
		toFloat64Loose(serializedScaling["curvePower"], 0) != 2.1708 {
		t.Fatalf("serialized hitDamageScaling = %#v", serializedScaling)
	}
	reparsed := parseBattleDamageSpec(serialized, "physical", battleSkillAmountFormula{})
	if reparsed.HitDamageScaling == nil || reparsed.HitDamageScaling.CurvePower != 2.1708 {
		t.Fatalf("serialized hitDamageScaling did not round-trip: %#v", reparsed.HitDamageScaling)
	}
	basicAttack := toBattleSkillBasicAttackProfile(&battleBasicAttackProfile{DamageType: "magical", DamageSpec: spec})
	if basicAttack == nil || len(normalizeAnyMap(basicAttack.DamageSpec["hitDamageScaling"])) == 0 {
		t.Fatalf("basic attack serialization dropped hitDamageScaling: %#v", basicAttack)
	}

	invalid := []struct {
		name string
		raw  map[string]any
	}{
		{name: "kind alias", raw: map[string]any{"kind": "stretched_exponential", "secondHitMultiplier": 0.998, "curvePower": 2}},
		{name: "kind case", raw: map[string]any{"kind": "Powered_Exponential_Decay", "secondHitMultiplier": 0.998, "curvePower": 2}},
		{name: "string number", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": "0.998", "curvePower": 2}},
		{name: "non decay", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": 1, "curvePower": 2}},
		{name: "zero multiplier", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": 0, "curvePower": 2}},
		{name: "curve below one", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": 0.998, "curvePower": 0.999}},
		{name: "curve above three", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": 0.998, "curvePower": 3.001}},
		{name: "missing curve", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": 0.998}},
		{name: "unknown field", raw: map[string]any{"kind": "powered_exponential_decay", "secondHitMultiplier": 0.998, "curvePower": 2, "minimumMultiplier": 0.1}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			parsed := parseBattleDamageSpec(map[string]any{"hitDamageScaling": test.raw}, "physical", battleSkillAmountFormula{})
			if parsed.HitDamageScaling != nil {
				t.Fatalf("invalid scaling reached runtime: %#v", parsed.HitDamageScaling)
			}
		})
	}
	invalidTyped := normalizeBattleDamageSpecForRuntime(battleDamageSpec{
		HitDamageScaling: &battleHitDamageScaling{
			Kind:                battleHitDamageScalingKindPoweredExponentialDecay,
			SecondHitMultiplier: 1,
			CurvePower:          2,
		},
	}, "physical", battleSkillAmountFormula{})
	if invalidTyped.HitDamageScaling != nil {
		t.Fatalf("invalid typed carry value reached runtime: %#v", invalidTyped.HitDamageScaling)
	}

	legacy := parseBattleDamageSpec(map[string]any{"damageType": "physical"}, "magical", battleSkillAmountFormula{})
	if legacy.HitDamageScaling != nil || battleResolveHitDamageMultiplier(legacy.HitDamageScaling, 64) != 1 {
		t.Fatalf("legacy damage spec changed behavior: %#v", legacy.HitDamageScaling)
	}
	if _, exists := battleDamageSpecToAnyMap(legacy)["hitDamageScaling"]; exists {
		t.Fatal("legacy damage spec serialized a synthetic hitDamageScaling")
	}
}

func TestTemplateDamageAppliesHitScalingAfterCoreDamage(t *testing.T) {
	scaling := &battleHitDamageScaling{
		Kind:                battleHitDamageScalingKindPoweredExponentialDecay,
		SecondHitMultiplier: 0.998,
		CurvePower:          2.1708,
	}
	skill := testDamageSkill(
		"scaled_multi_hit",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType:       "magical",
			AmountFormula:    battleSkillAmountFormula{BaseAdd: 100},
			HitCount:         battleSkillTargetCount{Base: 5},
			HitDamageScaling: scaling,
		},
	)
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
	}
	target := battleUnitState{
		UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
		HP: 10_000, MaxHP: 10_000, Stats: map[string]float64{"hp": 10_000},
	}
	execution, _ := runDamageSkill(t, &Server{}, actor, target, skill)
	damageActions := battleDamageActions(execution.Actions)
	if len(execution.Effects) != 5 || len(damageActions) != 5 {
		t.Fatalf("scaled hit count: effects=%d actions=%d, want 5", len(execution.Effects), len(damageActions))
	}
	for index, action := range damageActions {
		hitIndex := index + 1
		wantMultiplier := battleResolveHitDamageMultiplier(scaling, hitIndex)
		wantDamage := roundTo4(100 * wantMultiplier)
		if got := toFloat64Loose(action["amount"], -1); got != wantDamage {
			t.Fatalf("hit %d damage = %v, want %v", hitIndex, got, wantDamage)
		}
		meta := normalizeAnyMap(action["meta"])
		if got := toFloat64Loose(meta["hitDamageMultiplier"], -1); math.Abs(got-wantMultiplier) > 1e-12 {
			t.Fatalf("hit %d meta multiplier = %.16g, want %.16g", hitIndex, got, wantMultiplier)
		}
		compact := compactBattleTimelineStepMeta(meta)
		if got := toFloat64Loose(compact["hm"], -1); math.Abs(got-wantMultiplier) > 1e-12 {
			t.Fatalf("hit %d compact multiplier = %.16g, want %.16g", hitIndex, got, wantMultiplier)
		}
	}
}

func TestTemplateDamageScalingRunsAfterCoreMinimum(t *testing.T) {
	scaling := &battleHitDamageScaling{
		Kind:                battleHitDamageScalingKindPoweredExponentialDecay,
		SecondHitMultiplier: 0.998,
		CurvePower:          2.1708,
	}
	skill := testDamageSkill(
		"scaled_core_minimum",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType:       "magical",
			AmountFormula:    battleSkillAmountFormula{BaseAdd: 0.5},
			HitCount:         battleSkillTargetCount{Base: 2},
			HitDamageScaling: scaling,
		},
	)
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
	}
	target := battleUnitState{
		UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
	}
	execution, _ := runDamageSkill(t, &Server{}, actor, target, skill)
	damageActions := battleDamageActions(execution.Actions)
	if len(damageActions) != 2 {
		t.Fatalf("damage actions = %d, want 2", len(damageActions))
	}
	if first := toFloat64Loose(damageActions[0]["amount"], -1); first != 1 {
		t.Fatalf("core minimum first hit = %v, want 1", first)
	}
	if second := toFloat64Loose(damageActions[1]["amount"], -1); second != 0.998 {
		t.Fatalf("scaled second hit = %v, want 0.998 below the core minimum", second)
	}
}

func TestTemplateDamageWithoutHitScalingKeepsLegacyPerHitDamage(t *testing.T) {
	skill := testDamageSkill(
		"legacy_multi_hit",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType:    "physical",
			AmountFormula: battleSkillAmountFormula{BaseAdd: 100},
			HitCount:      battleSkillTargetCount{Base: 3},
		},
	)
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
	}
	target := battleUnitState{
		UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
		HP: 1_000, MaxHP: 1_000, Stats: map[string]float64{"hp": 1_000},
	}
	execution, _ := runDamageSkill(t, &Server{}, actor, target, skill)
	damageActions := battleDamageActions(execution.Actions)
	if len(damageActions) != 3 {
		t.Fatalf("legacy hit actions = %d, want 3", len(damageActions))
	}
	for index, action := range damageActions {
		if got := toFloat64Loose(action["amount"], -1); got != 100 {
			t.Fatalf("legacy hit %d damage = %v, want 100", index+1, got)
		}
		if _, exists := normalizeAnyMap(action["meta"])["hitDamageMultiplier"]; exists {
			t.Fatalf("legacy hit %d gained scaling metadata", index+1)
		}
	}
}

func TestTemplateMultiHitStopsAfterTargetDies(t *testing.T) {
	skill := testDamageSkill(
		"lethal_multi_hit",
		1,
		testSingleEnemyTargeting("vanguard"),
		battleDamageSpec{
			DamageType:    "physical",
			AmountFormula: battleSkillAmountFormula{BaseAdd: 100},
			HitCount:      battleSkillTargetCount{Base: 4},
		},
	)
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", Position: "vanguard", Alive: true,
		HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
	}
	target := battleUnitState{
		UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true,
		HP: 150, MaxHP: 150, Stats: map[string]float64{"hp": 150},
	}
	execution, state := runDamageSkill(t, &Server{}, actor, target, skill)
	damageActions := battleDamageActions(execution.Actions)
	if len(execution.Effects) != 2 || len(damageActions) != 2 {
		t.Fatalf("post-lethal hits were not stopped: effects=%d actions=%d", len(execution.Effects), len(damageActions))
	}
	if state.Enemies[0].Alive || state.Enemies[0].HP != 0 {
		t.Fatalf("target state after lethal hit = alive:%v hp:%v", state.Enemies[0].Alive, state.Enemies[0].HP)
	}
	lastMeta := normalizeAnyMap(damageActions[len(damageActions)-1]["meta"])
	if got := toInt64Loose(lastMeta["hitIndex"], 0); got != 2 {
		t.Fatalf("last settled hit index = %d, want 2", got)
	}
}

func battleDamageActions(actions []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		if action["kind"] == "damage" {
			out = append(out, action)
		}
	}
	return out
}
