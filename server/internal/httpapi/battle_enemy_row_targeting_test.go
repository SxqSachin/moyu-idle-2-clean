package httpapi

import (
	"testing"
	"time"
)

func TestEnemyRowTargetingCanIgnoreTauntPriority(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "taunt_prefer_target",
			"buffId":  "taunt",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "prefer_as_enemy_target"}},
		},
	}
	targeting := battleSkillTargeting{
		TargetPositions:      []string{"vanguard"},
		SecondarySeek:        "none",
		TargetScope:          "enemy",
		SelectRule:           "first",
		IgnoreTargetPriority: true,
		TargetCount:          battleSkillTargetCount{Base: 999, Max: 999, MaxSet: true},
	}
	skill := testDamageSkill("enemy_front_row_attack", 1, targeting, battleDamageSpec{
		DamageType: "physical",
		AmountFormula: battleSkillAmountFormula{
			BaseAdd: 10,
		},
		HitCount: battleSkillTargetCount{Base: 1},
	})
	state := &battleRuntimeState{
		Seed: 13,
		Players: []battleUnitState{
			{UnitID: "player:front:1", Side: "player", BaseSide: "player", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"physics_defence": 0, "hp": 100}},
			{UnitID: "player:front:2", Side: "player", BaseSide: "player", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"physics_defence": 0, "hp": 100}},
			{
				UnitID: "player:rear:taunt", Side: "player", BaseSide: "player", Position: "rearguard", Alive: true, HP: 100, MaxHP: 100,
				Stats: map[string]float64{"physics_defence": 0, "hp": 100}, Buffs: []battleBuffState{{BuffID: "taunt", RemainTurns: 2}},
			},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:actor", Side: "enemy", BaseSide: "enemy", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"physics_damage": 10, "hp": 100}},
		},
	}

	s.executeBattleSkill(state, time.UnixMilli(1), &state.Enemies[0], skill, "active", nil)

	if state.Players[0].HP >= 100 || state.Players[1].HP >= 100 {
		t.Fatalf("both front-row targets should be hit: %#v", state.Players)
	}
	if state.Players[2].HP != 100 {
		t.Fatalf("rear taunt should not pull a fixed-row attack, hp=%v", state.Players[2].HP)
	}
}

func TestParseBattleSkillTargetingPreservesIgnoreTargetPriority(t *testing.T) {
	targeting := parseBattleSkillTargeting(map[string]any{
		"targetPositions":      []any{"midguard"},
		"secondarySeek":        "none",
		"ignoreTargetPriority": true,
	}, "vanguard", "front_to_rear", 1, nil, 0, false, nil)
	if !targeting.IgnoreTargetPriority {
		t.Fatal("ignoreTargetPriority was dropped by runtime parser")
	}
}
