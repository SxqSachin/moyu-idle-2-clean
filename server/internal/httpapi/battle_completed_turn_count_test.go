package httpapi

import (
	"encoding/json"
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestBattleCompletedTurnCountConditionExcludesPreparation(t *testing.T) {
	state := &battleRuntimeState{}
	unit := &battleUnitState{UnitID: "player:1"}
	condition := map[string]any{
		"type":      "self_completed_turn_count_mod_eq",
		"divisor":   4,
		"remainder": 0,
	}

	state.recordCompletedTurn(unit, true)
	if got := state.CompletedTurnCounts[unit.UnitID]; got != 0 {
		t.Fatalf("preparation count=%d, want 0", got)
	}
	if got := state.RuleTurnCounts[unit.UnitID]; got != 0 {
		t.Fatalf("preparation rule turn count=%d, want 0", got)
	}
	for turn := 1; turn <= 3; turn++ {
		state.recordCompletedTurn(unit, false)
		if got := state.RuleTurnCounts[unit.UnitID]; got != turn {
			t.Fatalf("rule turn count=%d, want %d", got, turn)
		}
		if (&Server{}).battleEvaluateEquipmentHookCondition(state, condition, unit, unit, nil, 0, "", 0, nil) {
			t.Fatalf("condition matched after %d completed turns", turn)
		}
	}
	state.recordCompletedTurn(unit, false)
	if !(&Server{}).battleEvaluateEquipmentHookCondition(state, condition, unit, unit, nil, 0, "", 0, nil) {
		t.Fatal("condition should match on the fourth completed turn")
	}
}

func TestBattleCompletedTurnCountsCarryAcrossWaves(t *testing.T) {
	state := battleRuntimeState{
		Players:             []battleUnitState{{UnitID: "player:1", Alive: true, HP: 10}},
		CompletedTurnCounts: map[string]int{"player:1": 3, "kitty:7": 4},
		RuleTurnCounts:      map[string]int{"player:1": 2},
	}
	carry := battleCarryStatePayload(state, store.BattleRunStatusEnded)
	if _, exists := carry["ruleTurnCounts"]; exists {
		t.Fatal("wave-local release-rule turn counts must not carry across waves")
	}
	raw, err := json.Marshal(carry)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRequiredBattlePreviousWaveCarry(store.BattlePreviousWaveCarry{
		Wave: 1, Status: store.BattleRunStatusEnded, CarryStateRaw: raw,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.CompletedTurnCounts["player:1"] != 3 || decoded.CompletedTurnCounts["kitty:7"] != 4 || len(decoded.CompletedTurnCounts) != 2 {
		t.Fatalf("decoded completed turn counts=%#v", decoded.CompletedTurnCounts)
	}
}
