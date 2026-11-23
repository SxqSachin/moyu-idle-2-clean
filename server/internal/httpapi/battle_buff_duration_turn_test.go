package httpapi

import (
	"testing"
	"time"
)

func TestTickBattleBuffDurationsForTurnPreservesNewAndRefreshedBuffs(t *testing.T) {
	unit := &battleUnitState{
		Buffs: []battleBuffState{
			{BuffID: "preexisting", RemainTurns: 1},
			{BuffID: "refreshed", RemainTurns: 1},
		},
	}
	battleApplyOrRefreshBuffForTurn(unit, "refreshed", 1, nil, unit.UnitID, "refresh_duration", 0, 7)
	battleApplyOrRefreshBuffForTurn(unit, "new", 1, nil, unit.UnitID, "refresh_duration", 0, 7)

	changed, expired := tickBattleBuffDurationsForTurn(unit, 7)
	if !changed || len(expired) != 1 || expired[0] != "preexisting" {
		t.Fatalf("same-turn tick changed=%v expired=%#v, want only preexisting", changed, expired)
	}
	for _, buffID := range []string{"refreshed", "new"} {
		buff := findBuffByID(unit, buffID)
		if buff == nil || buff.RemainTurns != 1 || buff.AppliedBuffTurnSerial != 0 {
			t.Fatalf("buff %s was not preserved for its full duration: %#v", buffID, buff)
		}
	}

	changed, expired = tickBattleBuffDurationsForTurn(unit, 8)
	if !changed || len(expired) != 2 || len(unit.Buffs) != 0 {
		t.Fatalf("next-turn tick changed=%v expired=%#v buffs=%#v", changed, expired, unit.Buffs)
	}
}

func TestExecuteUnitTurnBuffDurationStartsAtItsApplicationPhase(t *testing.T) {
	tests := []struct {
		name       string
		trigger    string
		wantActive bool
	}{
		{name: "turn start counts current turn", trigger: "on_turn_start", wantActive: false},
		{name: "turn end starts after current turn", trigger: "on_turn_end", wantActive: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &Server{}
			s.deps.GameData.Buff = []map[string]any{{
				"id": "phase_buff", "durationTurns": 1, "stackPolicy": "refresh_duration",
			}}
			state := &battleRuntimeState{
				Seed: 1,
				Wave: 1,
				Players: []battleUnitState{{
					UnitID: "player:1", Side: "player", BaseSide: "player", Alive: true,
					HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100, "speed": 10},
					EquipmentHooks: []battleEquipmentHookRule{{
						ID: "phase_hook", Trigger: test.trigger,
						Effects: []map[string]any{{
							"kind": "add_buff", "buffId": "phase_buff", "durationTurns": 1, "targetMode": "self",
						}},
					}},
				}},
				Enemies: []battleUnitState{{
					UnitID: "enemy:1", Side: "enemy", BaseSide: "enemy", Alive: true,
					HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100, "speed": 1},
				}},
			}

			s.executeUnitTurn(state, time.Unix(1, 0), &state.Players[0], "player", false)
			buff := findBuffByID(&state.Players[0], "phase_buff")
			if test.wantActive {
				if buff == nil || buff.RemainTurns != 1 || buff.AppliedBuffTurnSerial != 0 {
					t.Fatalf("turn-end buff should retain one full turn: %#v", buff)
				}
			} else if buff != nil {
				t.Fatalf("turn-start duration-one buff should expire at this turn end: %#v", buff)
			}
		})
	}
}

func TestProbabilitySuccessBuffCanReachThreeStacksWithoutLosingItsFirstTurn(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{{
		"id": "full_house_applause", "durationTurns": 2, "stackPolicy": "stack", "maxStacks": 3,
	}}
	state := &battleRuntimeState{Seed: 1, Wave: 1, activeBuffDurationTurnSerial: 11}
	actor := battleUnitState{
		UnitID: "player:1", Side: "player", BaseSide: "player", Alive: true,
		HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
		EquipmentHooks: []battleEquipmentHookRule{{
			ID: "full_house_applause_crown_success", Trigger: "on_probability_success",
			Effects: []map[string]any{{
				"kind": "add_buff", "buffId": "full_house_applause", "durationTurns": 2, "targetMode": "self",
			}},
		}},
	}
	state.Players = []battleUnitState{actor}
	for index := 0; index < 3; index++ {
		actions := s.battleTriggerUnitEquipmentHooksWithContext(
			state,
			time.Unix(int64(index+1), 0),
			&state.Players[0],
			nil,
			nil,
			"on_probability_success",
			0,
			"",
			&battleEquipmentHookContext{ProbabilityKind: battleProbabilityEquipmentProc},
		)
		if len(actions) == 0 {
			t.Fatalf("success %d produced no applause action", index+1)
		}
	}
	if got := battleCountBuff(&state.Players[0], "full_house_applause"); got != 3 {
		t.Fatalf("applause stacks=%d, want 3", got)
	}
	if _, expired := tickBattleBuffDurationsForTurn(&state.Players[0], 11); len(expired) != 0 {
		t.Fatalf("new applause stacks lost a turn immediately: %#v", expired)
	}
	for _, buff := range state.Players[0].Buffs {
		if buff.BuffID == "full_house_applause" && buff.RemainTurns != 2 {
			t.Fatalf("new applause stack duration=%d, want 2", buff.RemainTurns)
		}
	}
}
