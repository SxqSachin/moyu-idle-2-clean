package httpapi

import (
	"testing"
	"time"
)

func TestTemplateSummonKeepsInitialBuffsAndAllowsNoSkills(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Enemies = []map[string]any{{
		"id":             "plush_stunt_double_001",
		"name":           "Plush Double",
		"position":       "vanguard",
		"type":           "summon",
		"skillIds":       []any{},
		"initialBuffIds": []any{"plush_stunt_double_role"},
		"hp":             1.0,
		"mp":             0.0,
	}}
	s.deps.GameData.Buff = []map[string]any{{
		"id": "plush_stunt_double_role", "durationTurns": -1, "permanent": true, "stackPolicy": "refresh_duration",
	}}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id": "plush_stunt_double_role_while_active", "buffId": "plush_stunt_double_role", "trigger": "while_active",
		"effects": []any{map[string]any{"kind": "skip_turn_order"}},
	}}

	state := &battleRuntimeState{
		Players: []battleUnitState{{UnitID: "player:1", Side: "player", BaseSide: "player", Alive: true, HP: 100, MaxHP: 100}},
	}
	actor := &state.Players[0]
	runtime, ok := s.buildBattleSkillSummonRuntime(state, actor, battleSkillSummonPayload{
		UnitID: "plush_stunt_double_001", Side: "player", DurationTurns: 3,
	}, 1, 0)
	if !ok {
		t.Fatal("expected summon runtime")
	}
	if !runtime.AllowNoSkills || len(runtime.SkillIDs) != 0 || len(runtime.InitialBuffIDs) != 1 {
		t.Fatalf("summon template contract lost: %#v", runtime)
	}
	if !addBattleSummon(state, "player", &runtime, s, time.Unix(0, 0)) {
		t.Fatal("expected summon to be added")
	}
	if len(state.Players) != 2 {
		t.Fatalf("players=%d, want summoner + summon", len(state.Players))
	}
	summon := &state.Players[1]
	if len(summon.SkillIDs) != 0 || len(summon.Skills) != 0 {
		t.Fatalf("non-acting summon received skills: ids=%#v skills=%#v", summon.SkillIDs, summon.Skills)
	}
	if !battleHasBuff(summon, "plush_stunt_double_role") || !summon.SkipTurnOrder {
		t.Fatalf("summon initial role was not applied: buffs=%#v skip=%v", summon.Buffs, summon.SkipTurnOrder)
	}
}
