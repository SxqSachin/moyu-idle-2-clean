package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func TestBattleSkillExpRewardsFromTimeline(t *testing.T) {
	finalResult := map[string]any{
		"finalState": map[string]any{
			"players": []map[string]any{
				{
					"unitId": "player-1",
					"userId": 1,
					"skills": []map[string]any{
						{"id": "fireball", "level": 3, "exp": 0},
						{"id": "basic_attack", "level": 1, "exp": 0},
						{"id": "maxed", "level": 200, "exp": 0},
					},
				},
				{
					"unitId": "player-2",
					"userId": 2,
					"skills": []map[string]any{
						{"id": "slash", "level": 1, "exp": 0},
					},
				},
			},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "fireball"}),
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "fireball"}),
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "basic_attack"}),
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "maxed"}),
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "missing"}),
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-2", "skillId": "slash"}),
		mustBattleTimelineEvent(t, "battle.turn.item_use", map[string]any{"actorId": "player-1", "skillId": "fireball"}),
	}

	rewards := battleSkillExpRewardsFromTimeline(timeline, finalResult, func(skillID string) int {
		switch skillID {
		case "fireball":
			return 10
		case "slash":
			return 2
		case "maxed":
			return 200
		default:
			return 200
		}
	})

	if got := rewards[1]["fireball"]; got != 4 {
		t.Fatalf("player-1 fireball exp = %d, want 4", got)
	}
	if _, ok := rewards[1]["basic_attack"]; ok {
		t.Fatalf("basic attack should not gain exp")
	}
	if _, ok := rewards[1]["maxed"]; ok {
		t.Fatalf("max-level skill should not gain exp")
	}
	if _, ok := rewards[1]["missing"]; ok {
		t.Fatalf("skill without level should not gain exp")
	}
	if got := rewards[2]["slash"]; got != 2 {
		t.Fatalf("player-2 slash exp = %d, want 2", got)
	}
}

func TestSettleBattleSkillExpGrantsFullKittySkillExp(t *testing.T) {
	s := &Server{}
	s.deps.Config.SharedGlobalSettings = config.DefaultSharedGlobalSettings()
	finalResult := map[string]any{
		"finalState": map[string]any{
			"players": []map[string]any{
				{
					"unitId":   "kitty:101",
					"unitType": "kitty",
					"userId":   float64(1),
					"kittyId":  float64(101),
					"skills": []map[string]any{
						{"id": "scratch", "level": 1, "exp": 0},
					},
				},
			},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "kitty:101", "skillId": "scratch"}),
	}

	_, rows, err := s.settleBattleSkillExpFromTimeline(context.Background(), timeline, finalResult)
	if err != nil {
		t.Fatalf("settleBattleSkillExpFromTimeline error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("skill exp rows=%d, want 1: %#v", len(rows), rows)
	}
	row := rows[0]
	if row.ActorType != "kitty" || row.ActorID != 101 || row.SkillID != "scratch" {
		t.Fatalf("unexpected kitty skill exp row: %#v", row)
	}
	if row.Exp != 2 {
		t.Fatalf("kitty skill exp=%d, want full per-use exp 2", row.Exp)
	}
}

func TestFinalStateFallbackInfersKittyIdentityForSkillExp(t *testing.T) {
	finalResult := map[string]any{
		"finalState": map[string]any{
			"players": []map[string]any{{
				"unitId": "kitty:101",
				"userId": 1,
				"skills": []map[string]any{{"id": "scratch", "level": 1}},
			}},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "kitty:101", "skillId": "scratch"}),
	}

	rewards := battleSkillExpRewardsFromTimeline(timeline, finalResult, func(string) int { return 200 })
	if got := rewards[-101]["scratch"]; got != store.BattleSkillExpPerUse {
		t.Fatalf("kitty fallback skill exp=%d, want %d; rewards=%#v", got, store.BattleSkillExpPerUse, rewards)
	}
	if _, misattributed := rewards[1]; misattributed {
		t.Fatalf("kitty fallback skill exp was attributed to owner player: %#v", rewards)
	}
}

func TestBattleSkillExpRewardsFromTimelineNode(t *testing.T) {
	finalResult := map[string]any{
		"finalState": map[string]any{
			"players": []map[string]any{
				{
					"unitId": "player-1",
					"userId": 1,
					"skills": []map[string]any{
						{"id": "fireball", "level": 3, "exp": 0},
					},
				},
			},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		mustBattleTimelineEvent(t, "battle.timeline.node", map[string]any{
			"o": "player-1",
			"p": []map[string]any{
				{"k": "c", "s": "player-1", "i": "potion", "ph": "i"},
				{"k": "c", "s": "player-1", "i": "fireball", "ph": "a"},
				{"k": "c", "s": "hook-1", "i": "hook_proc", "ph": "a"},
			},
		}),
	}

	rewards := battleSkillExpRewardsFromTimeline(timeline, finalResult, func(skillID string) int { return 200 })
	if got := rewards[1]["fireball"]; got != 2 {
		t.Fatalf("player-1 fireball exp from node = %d, want 2", got)
	}
}

func TestBattleSkillExpRewardsPreferBattleStats(t *testing.T) {
	finalResult := map[string]any{
		"battleStats": map[string]any{
			"skillCastsByUnit": map[string]any{
				"player-1": map[string]any{
					"fireball":     float64(3),
					"basic_attack": float64(4),
					"maxed":        float64(2),
				},
			},
		},
		"finalState": map[string]any{
			"players": []map[string]any{
				{
					"unitId": "player-1",
					"userId": 1,
					"skills": []map[string]any{
						{"id": "fireball", "level": 3, "exp": 0},
						{"id": "basic_attack", "level": 1, "exp": 0},
						{"id": "maxed", "level": 200, "exp": 0},
					},
				},
			},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "fireball"}),
	}

	rewards := battleSkillExpRewardsFromTimeline(timeline, finalResult, func(skillID string) int {
		if skillID == "maxed" {
			return 200
		}
		return 10
	})

	if got := rewards[1]["fireball"]; got != 6 {
		t.Fatalf("player-1 fireball exp from stats = %d, want 6", got)
	}
	if _, ok := rewards[1]["basic_attack"]; ok {
		t.Fatalf("basic attack should not gain exp from stats")
	}
	if _, ok := rewards[1]["maxed"]; ok {
		t.Fatalf("max-level skill should not gain exp from stats")
	}
}

func TestBattleSkillExpRewardsFallsBackWhenBattleStatsEmpty(t *testing.T) {
	finalResult := map[string]any{
		"battleStats": map[string]any{},
		"achievementStats": map[string]any{
			"skillCastsByUnit": map[string]any{
				"player-1": map[string]any{
					"fireball": float64(2),
				},
			},
		},
		"finalState": map[string]any{
			"players": []map[string]any{
				{
					"unitId": "player-1",
					"userId": 1,
					"skills": []map[string]any{
						{"id": "fireball", "level": 3, "exp": 0},
					},
				},
			},
		},
	}
	timeline := []wsbus.BattleTimelineEvent{
		mustBattleTimelineEvent(t, "battle.turn.action", map[string]any{"actorId": "player-1", "skillId": "fireball"}),
	}

	rewards := battleSkillExpRewardsFromTimeline(timeline, finalResult, func(skillID string) int { return 10 })
	if got := rewards[1]["fireball"]; got != 4 {
		t.Fatalf("player-1 fireball exp from fallback stats = %d, want 4", got)
	}
}

func mustBattleTimelineEvent(t *testing.T, eventType string, data map[string]any) wsbus.BattleTimelineEvent {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"eventType": eventType,
		"data":      data,
	})
	if err != nil {
		t.Fatalf("marshal timeline payload: %v", err)
	}
	return wsbus.BattleTimelineEvent{Payload: payload}
}

func TestApplyBuffEmitsStatRecalcForAdvancePosition(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "basic_hook_claw_dragged", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "basic_hook_claw_dragged_while_active",
			"buffId":  "basic_hook_claw_dragged",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectAdvancePosition, "steps": 1},
			},
		},
	}

	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:       "player:1",
				Side:         "player",
				OriginSide:   "player",
				BaseSide:     "player",
				Position:     "vanguard",
				BasePosition: "vanguard",
				Alive:        true,
				HP:           100,
				MaxHP:        100,
				Stats:        map[string]float64{"hp": 100},
				BaseStats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:       "enemy:1",
				Side:         "enemy",
				OriginSide:   "enemy",
				BaseSide:     "enemy",
				Position:     "rearguard",
				BasePosition: "rearguard",
				Alive:        true,
				HP:           100,
				MaxHP:        100,
				Stats:        map[string]float64{"hp": 100},
				BaseStats:    map[string]float64{"hp": 100},
			},
		},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(0), s, nil)
	runtime.ApplyBuff(battleskills.BuffAction{
		TargetID:      "enemy:1",
		SourceID:      "player:1",
		BuffID:        "basic_hook_claw_dragged",
		DurationTurns: 2,
		Reason:        "unit_test",
	})

	if state.Enemies[0].Position != "midguard" {
		t.Fatalf("enemy position=%s, want midguard after hook buff", state.Enemies[0].Position)
	}
	actions := runtime.ActionLog()
	found := false
	for _, row := range actions {
		if row == nil || row["kind"] != "stat_recalc" {
			continue
		}
		if row["targetId"] != "enemy:1" {
			continue
		}
		fromPos := row["fromPosition"]
		toPos := row["position"]
		if fromPos == "rearguard" && toPos == "midguard" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stat_recalc action with fromPosition rearguard -> position midguard, got %#v", actions)
	}
}

func TestApplyBuffRefreshDoesNotReTriggerOnAttachHooks(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "basic_hook_claw_dragged", "durationTurns": 2},
		{"id": "hook_attach_marker", "durationTurns": 1},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "basic_hook_claw_dragged_on_attach",
			"buffId":  "basic_hook_claw_dragged",
			"trigger": "on_attach",
			"effects": []map[string]any{
				{"kind": "add_buff", "buffId": "hook_attach_marker", "durationTurns": 1, "targetMode": "self"},
			},
		},
	}

	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:       "player:1",
				Side:         "player",
				OriginSide:   "player",
				BaseSide:     "player",
				Position:     "vanguard",
				BasePosition: "vanguard",
				Alive:        true,
				HP:           100,
				MaxHP:        100,
				Stats:        map[string]float64{"hp": 100},
				BaseStats:    map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:       "enemy:1",
				Side:         "enemy",
				OriginSide:   "enemy",
				BaseSide:     "enemy",
				Position:     "rearguard",
				BasePosition: "rearguard",
				Alive:        true,
				HP:           100,
				MaxHP:        100,
				Stats:        map[string]float64{"hp": 100},
				BaseStats:    map[string]float64{"hp": 100},
			},
		},
	}

	first := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(0), s, nil)
	first.ApplyBuff(battleskills.BuffAction{
		TargetID:      "enemy:1",
		SourceID:      "player:1",
		BuffID:        "basic_hook_claw_dragged",
		DurationTurns: 2,
		Reason:        "first_apply",
	})
	firstHookTriggered := false
	for _, row := range first.ActionLog() {
		if row == nil || row["kind"] != "buff" {
			continue
		}
		if row["buffId"] == "hook_attach_marker" {
			firstHookTriggered = true
			break
		}
	}
	if !firstHookTriggered {
		t.Fatalf("expected first attach to trigger on_attach hook marker buff")
	}

	second := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(100), s, nil)
	second.ApplyBuff(battleskills.BuffAction{
		TargetID:      "enemy:1",
		SourceID:      "player:1",
		BuffID:        "basic_hook_claw_dragged",
		DurationTurns: 2,
		Reason:        "refresh_apply",
	})
	for _, row := range second.ActionLog() {
		if row == nil || row["kind"] != "buff" {
			continue
		}
		if row["buffId"] == "hook_attach_marker" {
			t.Fatalf("refresh apply should not retrigger on_attach hook, got actions=%#v", second.ActionLog())
		}
	}
}

func TestAfterDealHealEquipmentHookCountsEffectiveHealingPerTarget(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "five_vow_answered_prayer",
			"durationTurns": -1,
			"permanent":     true,
			"stackPolicy":   "stack",
			"maxStacks":     5,
		},
	}
	answeredPrayerHook := battleEquipmentHookRule{
		ID:      "five_vow_echo_record_heal",
		Trigger: "on_after_deal_heal",
		Conditions: []map[string]any{
			{"type": "incoming_heal_gte", "value": 1},
			{"type": "self_buff_count_lte", "buffId": "five_vow_answered_prayer", "value": 4},
		},
		Effects: []map[string]any{
			{"kind": "add_buff", "buffId": "five_vow_answered_prayer", "durationTurns": 999, "targetMode": "self"},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:         "player:healer",
				Side:           "player",
				OriginSide:     "player",
				BaseSide:       "player",
				Alive:          true,
				HP:             80,
				MaxHP:          100,
				Stats:          map[string]float64{"hp": 100},
				BaseStats:      map[string]float64{"hp": 100},
				EquipmentHooks: []battleEquipmentHookRule{answeredPrayerHook},
			},
			{
				UnitID:     "player:full",
				Side:       "player",
				OriginSide: "player",
				BaseSide:   "player",
				Alive:      true,
				HP:         100,
				MaxHP:      100,
				Stats:      map[string]float64{"hp": 100},
				BaseStats:  map[string]float64{"hp": 100},
			},
			{
				UnitID:     "player:wounded-a",
				Side:       "player",
				OriginSide: "player",
				BaseSide:   "player",
				Alive:      true,
				HP:         40,
				MaxHP:      100,
				Stats:      map[string]float64{"hp": 100},
				BaseStats:  map[string]float64{"hp": 100},
			},
			{
				UnitID:     "player:wounded-b",
				Side:       "player",
				OriginSide: "player",
				BaseSide:   "player",
				Alive:      true,
				HP:         70,
				MaxHP:      100,
				Stats:      map[string]float64{"hp": 100},
				BaseStats:  map[string]float64{"hp": 100},
			},
		},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(0), s, nil)

	runtime.ApplyHeal(battleskills.HealAction{SourceID: "player:healer", TargetID: "player:full", Amount: 50, Reason: "aoe_test"})
	if got := battleCountBuff(&state.Players[0], "five_vow_answered_prayer"); got != 0 {
		t.Fatalf("overhealing a full-health target should not add a stack, got %d", got)
	}

	runtime.ApplyHeal(battleskills.HealAction{SourceID: "player:healer", TargetID: "player:wounded-a", Amount: 50, Reason: "aoe_test"})
	runtime.ApplyHeal(battleskills.HealAction{SourceID: "player:healer", TargetID: "player:wounded-b", Amount: 50, Reason: "aoe_test"})
	if got := battleCountBuff(&state.Players[0], "five_vow_answered_prayer"); got != 2 {
		t.Fatalf("two effectively healed AOE targets should add two stacks, got %d", got)
	}

	runtime.ApplyHeal(battleskills.HealAction{SourceID: "player:healer", TargetID: "player:healer", Amount: 20, Reason: "self_heal_test"})
	if got := battleCountBuff(&state.Players[0], "five_vow_answered_prayer"); got != 2 {
		t.Fatalf("self-healing should not add a stack, got %d", got)
	}

	state.activeExtraTurnUnitIDs = map[string]int{"player:healer": 1}
	state.Players[2].HP = 40
	runtime.ApplyHeal(battleskills.HealAction{SourceID: "player:healer", TargetID: "player:wounded-a", Amount: 50, Reason: "extra_turn_aoe_test"})
	if got := battleCountBuff(&state.Players[0], "five_vow_answered_prayer"); got != 2 {
		t.Fatalf("healing during an active extra turn should not refill stacks, got %d", got)
	}
}

func TestFiveVowEchoCooldownBuffPreventsExtraTurnChain(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "five_vow_answered_prayer", "durationTurns": -1, "permanent": true, "stackPolicy": "stack", "maxStacks": 5},
		{"id": "five_vow_echo_cooldown", "durationTurns": 2, "stackPolicy": "refresh_duration", "dispellable": false},
	}
	ringHook := battleEquipmentHookRule{
		ID:      "five_vow_echo_ring",
		Trigger: "on_turn_start",
		Conditions: []map[string]any{
			{"type": "self_buff_count_gte", "buffId": "five_vow_answered_prayer", "value": 5},
			{"type": "self_buff_count_eq", "buffId": "five_vow_echo_cooldown", "value": 0},
		},
		Effects: []map[string]any{
			{"kind": "remove_buff", "buffId": "five_vow_answered_prayer", "targetMode": "self"},
			{"kind": "add_buff", "buffId": "five_vow_echo_cooldown", "durationTurns": 2, "targetMode": "self"},
			{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
		},
	}
	prayerStacks := func() []battleBuffState {
		return []battleBuffState{
			{BuffID: "five_vow_answered_prayer", RemainTurns: -1},
			{BuffID: "five_vow_answered_prayer", RemainTurns: -1},
			{BuffID: "five_vow_answered_prayer", RemainTurns: -1},
			{BuffID: "five_vow_answered_prayer", RemainTurns: -1},
			{BuffID: "five_vow_answered_prayer", RemainTurns: -1},
		}
	}
	state := &battleRuntimeState{
		Seed:  3,
		Wave:  1,
		Order: []string{"player:healer", "enemy:1"},
		Players: []battleUnitState{{
			UnitID:         "player:healer",
			UserID:         1,
			Side:           "player",
			OriginSide:     "player",
			BaseSide:       "player",
			Alive:          true,
			HP:             100,
			MaxHP:          100,
			Stats:          map[string]float64{"hp": 100},
			BaseStats:      map[string]float64{"hp": 100},
			Buffs:          prayerStacks(),
			EquipmentHooks: []battleEquipmentHookRule{ringHook},
		}},
		Enemies: []battleUnitState{{
			UnitID:     "enemy:1",
			Side:       "enemy",
			OriginSide: "enemy",
			BaseSide:   "enemy",
			Alive:      true,
			HP:         100,
			MaxHP:      100,
			Stats:      map[string]float64{"hp": 100},
			BaseStats:  map[string]float64{"hp": 100},
		}},
	}
	healer := &state.Players[0]
	actions := s.battleTriggerUnitEquipmentHooks(state, time.UnixMilli(0), healer, nil, "on_turn_start", 0, "")
	if got := countActionKindRecursive(actions, "extra_turn"); got != 1 {
		t.Fatalf("five-vow echo should grant exactly one extra turn, got %d actions=%#v", got, actions)
	}
	if got := battleCountBuff(healer, "five_vow_answered_prayer"); got != 0 {
		t.Fatalf("answered-vow stacks should be cleared, got %d", got)
	}
	if cooldown := findBuffByID(healer, "five_vow_echo_cooldown"); cooldown == nil || cooldown.RemainTurns != 2 {
		t.Fatalf("queueing must not tick cooldown before the formal extra turn, buffs=%#v", healer.Buffs)
	}
	if len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("five-vow echo did not queue one formal extra turn: %#v", state.ExtraTurnQueue)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(1), nil); err != nil {
		t.Fatalf("process five-vow formal extra turn: %v", err)
	}
	healer = &state.Players[0]
	if cooldown := findBuffByID(healer, "five_vow_echo_cooldown"); cooldown == nil || cooldown.RemainTurns != 1 {
		t.Fatalf("formal extra turn should tick cooldown duration once, buffs=%#v", healer.Buffs)
	}

	healer.Buffs = append(healer.Buffs, prayerStacks()...)
	if repeat := s.battleTriggerUnitEquipmentHooks(state, time.UnixMilli(2), healer, nil, "on_turn_start", 0, ""); len(repeat) != 0 {
		t.Fatalf("cooldown buff should block a chained extra turn, actions=%#v", repeat)
	}
	if got := battleCountBuff(healer, "five_vow_answered_prayer"); got != 5 {
		t.Fatalf("blocked trigger must preserve accumulated stacks, got %d", got)
	}
}

func TestApplyBuffStackPolicyStackAllowsMultipleInstances(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "stackable_mark", "durationTurns": 2, "stackPolicy": "stack"},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				BaseStats: map[string]float64{
					"hp": 100,
				},
			},
		},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(0), s, nil)
	runtime.ApplyBuff(battleskills.BuffAction{
		TargetID:      "player:1",
		SourceID:      "player:1",
		BuffID:        "stackable_mark",
		DurationTurns: 2,
		Reason:        "first",
	})
	runtime.ApplyBuff(battleskills.BuffAction{
		TargetID:      "player:1",
		SourceID:      "player:1",
		BuffID:        "stackable_mark",
		DurationTurns: 2,
		Reason:        "second",
	})
	if len(state.Players[0].Buffs) != 2 {
		t.Fatalf("stack policy should keep both instances, buffs=%#v", state.Players[0].Buffs)
	}
}

func TestApplyBuffStackPolicyHonorsMaxStacks(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "stackable_poison", "durationTurns": 2, "stackPolicy": "stack", "maxStacks": 2},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:    "player:1",
				Side:      "player",
				Position:  "vanguard",
				Alive:     true,
				HP:        100,
				MaxHP:     100,
				Stats:     map[string]float64{"hp": 100},
				BaseStats: map[string]float64{"hp": 100},
			},
		},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(0), s, nil)
	for idx, sourceID := range []string{"source:1", "source:2", "source:3"} {
		runtime.ApplyBuff(battleskills.BuffAction{
			TargetID:      "player:1",
			SourceID:      sourceID,
			BuffID:        "stackable_poison",
			DurationTurns: int64(idx + 2),
			Reason:        "stack",
		})
	}
	if len(state.Players[0].Buffs) != 2 {
		t.Fatalf("maxStacks should cap instances at 2, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Buffs[0].SourceID != "source:3" || state.Players[0].Buffs[0].RemainTurns != 4 {
		t.Fatalf("capped stack should refresh oldest instance, buffs=%#v", state.Players[0].Buffs)
	}
}

func TestBattleSelectSkillFilteredAppliesCastGuards(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
		{
			"id":     "perception_mind_control",
			"mpCost": 20,
			"castGuards": []any{
				map[string]any{"kind": "enemy_side_not_has_effect", "effectKind": "invert_relation_perspective"},
			},
			"effects": []any{
				map[string]any{
					"kind": "status",
					"targeting": map[string]any{
						"targetPositions": []any{"rearguard"},
						"secondarySeek":   "rear_to_front",
						"targetScope":     "enemy",
						"selectRule":      "random",
						"targetCount":     map[string]any{"base": 1, "formula": []any{}},
					},
					"statusPayload": map[string]any{
						"statusIds":                  []any{"basic_left_dagger_betrayal"},
						"durationBaseTurns":          2,
						"durationPerSkillLevelTurns": 0,
					},
				},
			},
		},
	}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "basic_left_dagger_betrayal", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "basic_left_dagger_betrayal_while_active",
			"buffId":  "basic_left_dagger_betrayal",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": "invert_relation_perspective"},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       100,
				MaxMP:    100,
				Stats:    map[string]float64{"hp": 100, "mp": 100},
				Skills: []battleSkillState{
					s.battleSkillByID("perception_mind_control"),
					s.battleSkillByID("basic_attack"),
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100},
				Buffs: []battleBuffState{
					{BuffID: "basic_left_dagger_betrayal", RemainTurns: 2},
				},
			},
		},
	}
	skill, _ := s.battleSelectSkillFiltered(state, state.Players[0], "player", true, true)
	if skill.ID == "perception_mind_control" {
		t.Fatalf("mind control should be blocked when enemy side has invert_relation_perspective")
	}

	state.Enemies[0].Buffs = nil
	state.Players[0].Skills = []battleSkillState{
		s.battleSkillByID("perception_mind_control"),
		s.battleSkillByID("basic_attack"),
	}
	skill, _ = s.battleSelectSkillFiltered(state, state.Players[0], "player", true, true)
	if skill.ID != "perception_mind_control" {
		t.Fatalf("mind control should be selectable when guard condition is satisfied, got %s", skill.ID)
	}
}
