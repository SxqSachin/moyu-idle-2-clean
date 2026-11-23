package httpapi

import (
	"context"
	"testing"
	"time"
)

func summonDurationTestUnit(id string) battleUnitState {
	return battleUnitState{
		UnitID:       id,
		Side:         "player",
		OriginSide:   "player",
		BaseSide:     "player",
		Alive:        true,
		HP:           100,
		MaxHP:        100,
		MP:           10,
		MaxMP:        10,
		Position:     "vanguard",
		BasePosition: "vanguard",
		Stats:        map[string]float64{"hp": 100, "mp": 10, "speed": 10},
		BaseStats:    map[string]float64{"hp": 100, "mp": 10, "speed": 10},
	}
}

func summonDurationTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": battleSummonBindingBuffID, "onExpire": "remove_unit"},
		{"id": "skip_turn_summon_role", "durationTurns": -1, "permanent": true},
	}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id":      "skip_turn_summon_role_while_active",
		"buffId":  "skip_turn_summon_role",
		"trigger": "while_active",
		"effects": []map[string]any{{"kind": "skip_turn_order"}},
	}}
	return s
}

func summonBindingRemainTurns(t *testing.T, unit *battleUnitState) int64 {
	t.Helper()
	binding := findBuffByID(unit, battleSummonBindingBuffID)
	if binding == nil {
		t.Fatalf("summon binding missing from %#v", unit.Buffs)
	}
	return binding.RemainTurns
}

func TestSkipTurnSummonDurationUsesSourceFormalTurnsAndExpires(t *testing.T) {
	s := summonDurationTestServer()
	owner := summonDurationTestUnit("player:owner")
	owner.UserID = 1
	plush := summonDurationTestUnit("player:plush")
	plush.SkipTurnOrder = true
	plush.SummonTemplateUnitID = "plush_stunt_double_001"
	plush.SummonSourceUnitID = owner.UnitID
	plush.SummonRemainTurns = 3
	plush.Buffs = []battleBuffState{{
		BuffID:      battleSummonBindingBuffID,
		RemainTurns: 3,
		SourceID:    owner.UnitID,
	}, {BuffID: "skip_turn_summon_role", RemainTurns: -1}}
	enemy := summonDurationTestUnit("enemy:target")
	enemy.Side = "enemy"
	enemy.OriginSide = "enemy"
	enemy.BaseSide = "enemy"
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{owner, plush}, Enemies: []battleUnitState{enemy}, Order: []string{owner.UnitID, enemy.UnitID}}
	now := time.Unix(100, 0)

	s.executeUnitTurn(state, now, &state.Players[0], "player", true)
	if got := summonBindingRemainTurns(t, &state.Players[1]); got != 3 {
		t.Fatalf("preparation turn changed summon duration: got=%d want=3", got)
	}

	queued, ok := s.executeEquipmentExtraTurn(
		state,
		now.Add(time.Second),
		&state.Players[0],
		"on_probability_failure",
		battleEquipmentHookRule{ID: "extra_turn_test"},
		nil,
		"full",
	)
	if !ok || !parseBooleanLoose(queued["queued"], false) {
		t.Fatal("expected a queued formal extra turn")
	}
	if got := summonBindingRemainTurns(t, &state.Players[1]); got != 3 {
		t.Fatalf("queueing extra turn changed summon duration inline: got=%d want=3", got)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, now.Add(time.Second), nil); err != nil {
		t.Fatalf("process formal extra turn: %v", err)
	}
	if got := summonBindingRemainTurns(t, &state.Players[1]); got != 2 {
		t.Fatalf("extra full turn did not tick summon duration once: got=%d want=2", got)
	}

	s.executeUnitTurn(state, now.Add(2*time.Second), &state.Players[0], "player", false)
	if got := summonBindingRemainTurns(t, &state.Players[1]); got != 1 {
		t.Fatalf("second formal source turn duration: got=%d want=1", got)
	}

	payload := s.executeUnitTurn(state, now.Add(3*time.Second), &state.Players[0], "player", false)
	if state.Players[1].Alive || state.Players[1].HP != 0 || state.Players[1].SummonRemainTurns != 0 {
		t.Fatalf("summon did not expire after three source turns: %#v", state.Players[1])
	}
	if battleHasBuff(&state.Players[1], battleSummonBindingBuffID) {
		t.Fatalf("expired summon retained binding: %#v", state.Players[1].Buffs)
	}
	actions := payload["actionLog"].([]map[string]any)
	detachIndex, expireIndex := -1, -1
	for index, action := range actions {
		switch action["kind"] {
		case "state_detach":
			if action["buffId"] == battleSummonBindingBuffID {
				detachIndex = index
			}
		case "summon_expire":
			if action["targetId"] == plush.UnitID {
				expireIndex = index
			}
		}
	}
	if detachIndex < 0 || expireIndex <= detachIndex {
		t.Fatalf("expected binding detach before summon_expire, actions=%#v", actions)
	}
}

func TestSkipTurnSourceDurationDoesNotDoubleTickActingOrSyncedSummons(t *testing.T) {
	s := summonDurationTestServer()
	owner := summonDurationTestUnit("player:owner")
	acting := summonDurationTestUnit("player:acting")
	acting.SummonTemplateUnitID = "acting_summon"
	acting.SummonSourceUnitID = owner.UnitID
	acting.SummonRemainTurns = 3
	acting.Buffs = []battleBuffState{{BuffID: battleSummonBindingBuffID, RemainTurns: 3, SourceID: owner.UnitID}}
	synced := summonDurationTestUnit("player:synced")
	synced.SkipTurnOrder = true
	synced.SummonTemplateUnitID = "synced_summon"
	synced.SummonSourceUnitID = owner.UnitID
	synced.SummonRemainTurns = 3
	synced.Buffs = []battleBuffState{
		{BuffID: battleSummonBindingBuffID, RemainTurns: 3, SourceID: owner.UnitID},
		{BuffID: "skip_turn_summon_role", RemainTurns: -1},
	}
	state := &battleRuntimeState{Players: []battleUnitState{owner, acting, synced}}
	now := time.Unix(200, 0)

	s.executeUnitTurn(state, now, &state.Players[0], "player", false)
	if got := summonBindingRemainTurns(t, &state.Players[1]); got != 3 {
		t.Fatalf("ordered summon was incorrectly ticked by its source: got=%d want=3", got)
	}
	if got := summonBindingRemainTurns(t, &state.Players[2]); got != 2 {
		t.Fatalf("skip-turn summon source tick: got=%d want=2", got)
	}

	s.executeUnitTurn(state, now.Add(time.Second), &state.Players[2], "player", false)
	if got := summonBindingRemainTurns(t, &state.Players[2]); got != 2 {
		t.Fatalf("synced skip-turn summon double-ticked its binding: got=%d want=2", got)
	}

	s.executeUnitTurn(state, now.Add(2*time.Second), &state.Players[1], "player", false)
	if got := summonBindingRemainTurns(t, &state.Players[1]); got != 2 {
		t.Fatalf("ordered summon did not tick on its own turn: got=%d want=2", got)
	}
}
