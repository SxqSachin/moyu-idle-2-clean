package httpapi

import (
	"context"
	"testing"
	"time"
)

func probabilityExtraTurnQueueState(s *Server, chance float64, outcomeTrigger string, reward battleEquipmentHookRule) *battleRuntimeState {
	actor := amusementCrownOwner(0)
	actor.UnitID = "player:queue"
	actor.UserID = 1
	actor.HP = 400
	actor.MP = 100
	actor.Buffs = []battleBuffState{{BuffID: "queued_reward_fuel", RemainTurns: 3, SourceID: actor.UnitID}}
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:                   "queued_probability_probe",
			SourceItemID:         "queue_probe",
			Trigger:              "on_turn_start",
			MaxTriggersPerBattle: 1,
			Effects: []map[string]any{{
				"kind": "heal", "value": 1, "targetMode": "self", "procChanceBase": chance,
			}},
		},
		reward,
	}
	actor.EquipmentHooks[1].Trigger = outcomeTrigger
	actor.Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
	return &battleRuntimeState{
		Wave:    1,
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{{
			UnitID: "enemy:queue-target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1000, MaxHP: 1000,
			Stats: map[string]float64{"hp": 1000}, BaseStats: map[string]float64{"hp": 1000},
		}},
	}
}

func TestProbabilityRewardDefersEarnedExtraTurnUntilActiveTurnCompletes(t *testing.T) {
	for _, tc := range []struct {
		name           string
		chance         float64
		outcomeTrigger string
	}{
		{name: "success rewards such as applause crown", chance: 1, outcomeTrigger: "on_probability_success"},
		{name: "failure rewards such as hollow heart and exit ticket", chance: 0, outcomeTrigger: "on_probability_failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newAmusementCrownTestServer()
			s.deps.GameData.Buff = []map[string]any{{
				"id": "queued_reward_fuel", "kind": "buff", "durationTurns": 3, "stackPolicy": "refresh_duration",
			}}
			reward := battleEquipmentHookRule{
				ID:                   "queued_probability_reward",
				SourceItemID:         "queued_probability_item",
				MaxTriggersPerBattle: 1,
				Effects: []map[string]any{
					{"kind": "remove_buff", "buffId": "queued_reward_fuel", "targetMode": "self"},
					{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
				},
			}
			state := probabilityExtraTurnQueueState(s, tc.chance, tc.outcomeTrigger, reward)
			starter := battleEquipmentHookRule{ID: "unrelated_extra_turn_starter", SourceItemID: "starter"}

			queued, ok := s.executeEquipmentExtraTurn(
				state, time.UnixMilli(1), &state.Players[0], "on_test_start", starter, nil, "full", true,
			)
			if !ok {
				t.Fatal("initial extra turn was not queued")
			}
			if !parseBooleanLoose(queued["queued"], false) || state.Enemies[0].HP != 1000 || len(state.ExtraTurnQueue) != 1 {
				t.Fatalf("queueing must not settle the turn inline: queued=%#v enemyHp=%v queue=%#v", queued, state.Enemies[0].HP, state.ExtraTurnQueue)
			}

			restored := decodeBattleRuntimeState(state.toMap())
			state = &restored
			if len(state.ExtraTurnQueue) != 1 {
				t.Fatalf("queued formal turn was not persisted: %#v", state.ExtraTurnQueue)
			}
			if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
				t.Fatalf("process first formal extra turn: %v", err)
			}
			if state.Enemies[0].HP != 990 || len(state.ExtraTurnQueue) != 1 {
				t.Fatalf("first extra turn should attack then queue its earned successor: enemyHp=%v queue=%#v", state.Enemies[0].HP, state.ExtraTurnQueue)
			}
			if findBuffByID(&state.Players[0], "queued_reward_fuel") != nil {
				t.Fatalf("reward fuel was not consumed: buffs=%#v", state.Players[0].Buffs)
			}
			_, battleKey := battleHookCounterKeys(state, &state.Players[0], battleEquipmentHookCounterGroup(reward))
			if state.BattleHookBattleCounts[battleKey] != 1 {
				t.Fatalf("reward battle limit count=%d, want 1", state.BattleHookBattleCounts[battleKey])
			}
			firstTurn := lastBattleTurnActionData(t, state)
			if !parseBooleanLoose(normalizeAnyMap(firstTurn["resultMeta"])["extraTurn"], false) {
				t.Fatalf("queued action did not become an independent extra turn: %#v", firstTurn)
			}
			if got := countActionKindRecursive(battleAnyMapSlice(firstTurn["actionLog"]), "extra_turn"); got != 1 {
				t.Fatalf("first formal turn should visibly queue one successor, got %d payload=%#v", got, firstTurn)
			}

			if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(3), nil); err != nil {
				t.Fatalf("process earned formal extra turn: %v", err)
			}
			if state.Enemies[0].HP != 980 || len(state.ExtraTurnQueue) != 0 {
				t.Fatalf("earned successor did not settle exactly once: enemyHp=%v queue=%#v", state.Enemies[0].HP, state.ExtraTurnQueue)
			}
			if len(state.activeExtraTurnUnitIDs) != 0 || state.activeExtraTurnRequest != nil {
				t.Fatalf("extra-turn execution state leaked after settlement: %#v", state)
			}
		})
	}
}

func TestUnboundedProbabilityRewardCannotRecursivelySustainExtraTurns(t *testing.T) {
	s := newAmusementCrownTestServer()
	reward := battleEquipmentHookRule{
		ID:           "unbounded_probability_reward",
		SourceItemID: "unbounded_probability_item",
		Effects: []map[string]any{
			{"kind": "consume_mp", "value": 7, "targetMode": "self"},
			{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
		},
	}
	state := probabilityExtraTurnQueueState(s, 1, "on_probability_success", reward)
	// Let every executed turn establish another success. The reward itself has no
	// battle cap, so the queue's ancestry guard is the only recursion boundary.
	state.Players[0].EquipmentHooks[0].MaxTriggersPerBattle = 0
	state.Players[0].Buffs = nil
	starter := battleEquipmentHookRule{ID: "unbounded_chain_starter"}

	queued, ok := s.executeEquipmentExtraTurn(
		state, time.UnixMilli(1), &state.Players[0], "on_test_start", starter, nil, "full", true,
	)
	if !ok || !parseBooleanLoose(queued["queued"], false) {
		t.Fatal("initial extra turn was not queued")
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
		t.Fatalf("process first extra turn: %v", err)
	}
	if len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("unbounded reward should extend the chain once: %#v", state.ExtraTurnQueue)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(3), nil); err != nil {
		t.Fatalf("process second extra turn: %v", err)
	}
	if len(state.ExtraTurnQueue) != 0 {
		t.Fatalf("unbounded reward recursively sustained the chain: %#v", state.ExtraTurnQueue)
	}
	if state.Players[0].MP != 93 {
		t.Fatalf("blocked recursive reward consumed MP, got %v want 93", state.Players[0].MP)
	}
	if state.Enemies[0].HP != 980 {
		t.Fatalf("expected exactly the current and one deferred action, enemy hp=%v", state.Enemies[0].HP)
	}
}

func lastBattleTurnActionData(t *testing.T, state *battleRuntimeState) map[string]any {
	t.Helper()
	for index := len(state.Events) - 1; index >= 0; index-- {
		if state.Events[index].Type == "battle.turn.action" {
			return state.Events[index].Data
		}
	}
	t.Fatal("battle.turn.action event not found")
	return nil
}
