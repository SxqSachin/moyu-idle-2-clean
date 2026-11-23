package httpapi

import "testing"

func TestBattlePositionMapRotatesFromBasePositionAndExpiresCleanly(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{{"id": "reverse_carousel", "durationTurns": 2}}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id": "reverse_carousel_map", "buffId": "reverse_carousel", "trigger": "while_active", "priority": 5,
		"effects": []map[string]any{{
			"kind": "position_map",
			"mapping": map[string]any{"vanguard": "rearguard", "midguard": "vanguard", "rearguard": "midguard"},
		}},
	}}
	cases := map[string]string{"vanguard": "rearguard", "midguard": "vanguard", "rearguard": "midguard"}
	for base, want := range cases {
		unit := &battleUnitState{
			UnitID: "player:" + base, Side: "player", BaseSide: "player", OriginSide: "player",
			Position: base, BasePosition: base, Buffs: []battleBuffState{{BuffID: "reverse_carousel", RemainTurns: 2}},
		}
		s.battleRecomputeUnitPosition(unit)
		if unit.Position != want {
			t.Fatalf("base %s mapped to %s, want %s", base, unit.Position, want)
		}
		unit.Buffs = nil
		s.battleRecomputeUnitPosition(unit)
		if unit.Position != base {
			t.Fatalf("base %s did not restore after expiry: %s", base, unit.Position)
		}
	}
}

func TestBattlePositionMapThenAdvanceThenOverride(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "low_map"}, {"id": "high_map"}, {"id": "advance"}, {"id": "override"},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{"id": "z_low", "buffId": "low_map", "trigger": "while_active", "priority": 1, "effects": []map[string]any{{"kind": "position_map", "mapping": map[string]any{"rearguard": "vanguard"}}}},
		{"id": "a_high", "buffId": "high_map", "trigger": "while_active", "priority": 10, "effects": []map[string]any{{"kind": "position_map", "mapping": map[string]any{"rearguard": "rearguard"}}}},
		{"id": "advance", "buffId": "advance", "trigger": "while_active", "effects": []map[string]any{{"kind": "advance_position", "steps": 1}}},
		{"id": "override", "buffId": "override", "trigger": "while_active", "effects": []map[string]any{{"kind": "position_override", "position": "rearguard"}}},
	}
	unit := &battleUnitState{
		UnitID: "player:1", Side: "player", BaseSide: "player", OriginSide: "player",
		Position: "rearguard", BasePosition: "rearguard",
		Buffs: []battleBuffState{{BuffID: "low_map"}, {BuffID: "override"}, {BuffID: "advance"}, {BuffID: "high_map"}},
	}
	s.battleRecomputeUnitPosition(unit)
	if unit.Position != "rearguard" {
		t.Fatalf("explicit override must win after map and advance, got %s", unit.Position)
	}
	unit.Buffs = []battleBuffState{{BuffID: "low_map"}, {BuffID: "advance"}, {BuffID: "high_map"}}
	s.battleRecomputeUnitPosition(unit)
	if unit.Position != "midguard" {
		t.Fatalf("highest-priority map should run before advance, got %s", unit.Position)
	}
}
