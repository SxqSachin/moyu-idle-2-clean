package httpapi

import (
	"testing"
	"time"

	"mysecondwebgame/server/internal/rules"
)

func battleRuleTurnTestUnit(unitID, side string, withPassedTurnRule bool) battleUnitState {
	unit := battleUnitState{
		UnitID:          unitID,
		Side:            side,
		OriginSide:      side,
		BaseSide:        side,
		PerspectiveSide: side,
		Position:        "vanguard",
		BasePosition:    "vanguard",
		Alive:           true,
		HP:              100,
		MaxHP:           100,
		BaseStats:       map[string]float64{"hp": 100},
		Stats:           map[string]float64{"hp": 100},
		Skills:          []battleSkillState{{ID: "basic_attack"}},
	}
	if withPassedTurnRule {
		unit.SkillRules = map[string]battleSkillRuleState{
			"basic_attack": {
				Enabled: true,
				Groups: []rules.ConditionGroup{{Conditions: []rules.Condition{{
					CompareType:         "value",
					Target:              "none",
					DataType:            "passedTurn",
					ValueCompareAction:  "<=",
					ValueConditionValue: 3,
				}}}},
			},
		}
	}
	return unit
}

func TestArenaBasicAttackRepeatsWithoutReleaseRule(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	state := &battleRuntimeState{
		BattleKind: battleKindArena,
		Players: []battleUnitState{
			battleRuleTurnTestUnit("arena:attacker:player:1", "player", false),
		},
		Enemies: []battleUnitState{
			battleRuleTurnTestUnit("arena:defender:player:2", "enemy", false),
		},
	}

	for turn := 1; turn <= 2; turn++ {
		payload := s.executeUnitTurn(state, time.UnixMilli(int64(turn)), &state.Players[0], "player", false)
		if payload["skillId"] != "basic_attack" {
			t.Fatalf("turn %d skill=%v, want basic_attack", turn, payload["skillId"])
		}
	}
	if got := state.RuleTurnCounts[state.Players[0].UnitID]; got != 2 {
		t.Fatalf("actor rule turn count=%d, want 2", got)
	}
}

func TestArenaPassedTurnRuleUsesActorTurnsInsteadOfRosterSize(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	state := &battleRuntimeState{
		BattleKind: battleKindArena,
		Players: []battleUnitState{
			battleRuleTurnTestUnit("arena:attacker:player:1", "player", true),
			battleRuleTurnTestUnit("arena:attacker:kitty:1", "player", false),
			battleRuleTurnTestUnit("arena:attacker:kitty:2", "player", false),
		},
		Enemies: []battleUnitState{
			battleRuleTurnTestUnit("arena:defender:player:2", "enemy", false),
			battleRuleTurnTestUnit("arena:defender:kitty:3", "enemy", false),
			battleRuleTurnTestUnit("arena:defender:kitty:4", "enemy", false),
		},
	}

	if payload := s.executeUnitTurn(state, time.UnixMilli(1), &state.Players[0], "player", false); payload["skillId"] != "basic_attack" {
		t.Fatalf("first actor turn skill=%v, want basic_attack", payload["skillId"])
	}
	s.executeUnitTurn(state, time.UnixMilli(2), &state.Players[1], "player", false)
	s.executeUnitTurn(state, time.UnixMilli(3), &state.Players[2], "player", false)
	s.executeUnitTurn(state, time.UnixMilli(4), &state.Enemies[0], "enemy", false)
	s.executeUnitTurn(state, time.UnixMilli(5), &state.Enemies[1], "enemy", false)
	s.executeUnitTurn(state, time.UnixMilli(6), &state.Enemies[2], "enemy", false)

	if len(state.ActedUnits) != 6 {
		t.Fatalf("acted unit membership=%v, want all six units", state.ActedUnits)
	}
	ctx := s.buildBattleRuleEvalContext(state, state.Players[0], "player", state.Players[0].Skills[0], false)
	if ctx.PassedTurn != 2 {
		t.Fatalf("second actor turn passedTurn=%v, want 2", ctx.PassedTurn)
	}
	if payload := s.executeUnitTurn(state, time.UnixMilli(7), &state.Players[0], "player", false); payload["skillId"] != "basic_attack" {
		t.Fatalf("second actor turn skill=%v, want basic_attack", payload["skillId"])
	}
	if payload := s.executeUnitTurn(state, time.UnixMilli(8), &state.Players[0], "player", false); payload["skillId"] != "basic_attack" {
		t.Fatalf("third actor turn skill=%v, want basic_attack", payload["skillId"])
	}
	if payload := s.executeUnitTurn(state, time.UnixMilli(9), &state.Players[0], "player", false); payload["skillId"] != "" {
		t.Fatalf("fourth actor turn skill=%v, want release rule to block", payload["skillId"])
	}
}
