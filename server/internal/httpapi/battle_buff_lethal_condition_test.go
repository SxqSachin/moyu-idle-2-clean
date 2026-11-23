package httpapi

import (
	"math"
	"testing"
	"time"
)

func TestBattleBuffIncomingDamageWouldBeLethalCondition(t *testing.T) {
	unit := &battleUnitState{UnitID: "player:1", Alive: true, HP: 75, MaxHP: 100}
	condition := map[string]any{"type": "incoming_damage_would_be_lethal"}

	for _, tc := range []struct {
		name     string
		damage   float64
		expected bool
	}{
		{name: "below current hp", damage: 74.99, expected: false},
		{name: "equal current hp", damage: 75, expected: true},
		{name: "above current hp", damage: 120, expected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := battleEvaluateBuffConditionWithTarget(condition, unit, nil, nil, tc.damage, "physical", nil, 0)
			if got != tc.expected {
				t.Fatalf("condition=%v for damage=%v, want %v", got, tc.damage, tc.expected)
			}
		})
	}

	if battleEvaluateBuffConditionWithTarget(condition, nil, nil, nil, 100, "physical", nil, 0) {
		t.Fatal("nil target must not satisfy lethal damage condition")
	}
}

func TestBattleBuffLethalRewriteCanHealAndAttachPhaseBuff(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "last_bow", "durationTurns": 99, "maxCharges": 1},
		{"id": "encore_form", "durationTurns": 99, "stackPolicy": "refresh_duration"},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id": "last_bow_rewrite", "buffId": "last_bow", "trigger": "on_before_take_damage", "priority": 100,
			"conditions": []map[string]any{
				{"type": "incoming_damage_would_be_lethal"},
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{
				{"kind": "set_damage_multiplier", "value": 0},
				{"kind": "heal", "amountFormula": map[string]any{"baseAdd": 0, "terms": []map[string]any{{"attrKey": "max_hp", "coefficient": 0.35}}}},
				{"kind": "add_buff", "buffId": "encore_form", "durationTurns": 99, "targetMode": "self"},
				{"kind": "consume_charge"},
			},
		},
		{
			"id": "encore_form_while_active", "buffId": "encore_form", "trigger": "while_active", "priority": 0,
			"conditions": []map[string]any{},
			"effects":    []map[string]any{{"kind": "add_stat", "attr": "speed", "value": 25}},
		},
	}
	target := &battleUnitState{
		UnitID: "enemy:understudy", Side: "enemy", Alive: true, HP: 20, MaxHP: 100,
		BaseStats: map[string]float64{"hp": 100, "speed": 10},
		Stats:     map[string]float64{"hp": 100, "speed": 10},
		Buffs:     []battleBuffState{{BuffID: "last_bow", RemainTurns: 99}},
	}

	damage, logs := s.battleApplyBeforeTakeDamageBuffHooks(nil, time.UnixMilli(1), nil, target, 20, "magical")
	if damage != 0 {
		t.Fatalf("damage=%v, want lethal hit negated", damage)
	}
	if math.Abs(target.HP-55) > 1e-9 || !target.Alive {
		t.Fatalf("hp=%v alive=%v, want 55 and alive", target.HP, target.Alive)
	}
	if battleHasBuff(target, "last_bow") || !battleHasBuff(target, "encore_form") {
		t.Fatalf("buffs=%#v, want last_bow consumed and encore_form attached", target.Buffs)
	}
	if target.Stats["speed"] != 35 {
		t.Fatalf("speed=%v, want encore form applied immediately", target.Stats["speed"])
	}
	kinds := make([]string, 0, len(logs))
	for _, log := range logs {
		if kind, _ := log["kind"].(string); kind != "" {
			kinds = append(kinds, kind)
		}
	}
	if len(kinds) != 3 || kinds[0] != "heal" || kinds[1] != "buff" || kinds[2] != "state_detach" {
		t.Fatalf("log kinds=%v, want heal, buff, state_detach", kinds)
	}
}

func TestBattleBuffDamageGatePreservesLethalRewriteCharge(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "curtain", "durationTurns": 99, "maxCharges": 1},
		{"id": "last_bow", "durationTurns": 99, "maxCharges": 1},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id": "curtain_block", "buffId": "curtain", "trigger": "on_before_take_damage", "priority": 1000,
			"conditions": []map[string]any{{"type": "self_charges_remaining_gte", "value": 1}},
			"effects":    []map[string]any{{"kind": "set_damage_multiplier", "value": 0}},
		},
		{
			"id": "last_bow_rewrite", "buffId": "last_bow", "trigger": "on_before_take_damage", "priority": 1100,
			"conditions": []map[string]any{
				{"type": "incoming_damage_would_be_lethal"},
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{{"kind": "set_damage_multiplier", "value": 0}, {"kind": "consume_charge"}},
		},
	}
	target := &battleUnitState{
		UnitID: "enemy:understudy", Alive: true, HP: 20, MaxHP: 100,
		Buffs: []battleBuffState{{BuffID: "curtain", RemainTurns: 99}, {BuffID: "last_bow", RemainTurns: 99}},
	}

	damage, _ := s.battleApplyBeforeTakeDamageBuffHooks(nil, time.UnixMilli(1), nil, target, 100, "physical")
	if damage != 0 {
		t.Fatalf("damage=%v, want curtain to block hit", damage)
	}
	if !battleHasBuff(target, "last_bow") {
		t.Fatalf("last_bow should remain while curtain already made damage nonlethal: %#v", target.Buffs)
	}
}
