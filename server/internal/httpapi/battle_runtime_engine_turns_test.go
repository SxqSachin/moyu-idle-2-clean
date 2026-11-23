package httpapi

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/rules"
	"mysecondwebgame/server/internal/store"
)

func TestTickBattleBuffDurationsTurnBased(t *testing.T) {
	unit := &battleUnitState{
		Buffs: []battleBuffState{
			{BuffID: "permanent", RemainTurns: -1},
			{BuffID: "expire_now", RemainTurns: 1},
			{BuffID: "keep_one", RemainTurns: 2},
		},
	}

	changed, expired := tickBattleBuffDurations(unit)
	if !changed {
		t.Fatalf("tick should report changed=true")
	}
	if len(expired) != 1 || expired[0] != "expire_now" {
		t.Fatalf("unexpected expired buffs: %#v", expired)
	}
	if len(unit.Buffs) != 2 {
		t.Fatalf("expected 2 buffs after tick, got %d", len(unit.Buffs))
	}
	if unit.Buffs[0].BuffID != "permanent" || unit.Buffs[0].RemainTurns != -1 {
		t.Fatalf("permanent buff changed unexpectedly: %#v", unit.Buffs[0])
	}
	if unit.Buffs[1].BuffID != "keep_one" || unit.Buffs[1].RemainTurns != 1 {
		t.Fatalf("turn buff not decremented correctly: %#v", unit.Buffs[1])
	}
}

func TestProcessBattleTurnSuppressedActionStillTicksTurnState(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "frozen", "durationTurns": 1},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "frozen_action_suppressed",
			"buffId":  "frozen",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": battleEffectTurnActionSuppressed}},
		},
	}
	state := &battleRuntimeState{
		Seed:  1,
		Wave:  1,
		Order: []string{"player:1"},
		Players: []battleUnitState{
			{
				UnitID: "player:1",
				Side:   "player",
				UserID: 1,
				Alive:  true,
				HP:     50,
				MaxHP:  100,
				MP:     5,
				MaxMP:  10,
				Stats:  map[string]float64{"hp": 100, "mp": 10, "hp_regen": 3, "mp_regen": 2, "speed": 5},
				Skills: []battleSkillState{
					{ID: "power_strike", MPCost: 0, CooldownTurns: 3, CooldownRemain: 2},
				},
				Buffs: []battleBuffState{{BuffID: "frozen", RemainTurns: 1}},
			},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"speed": 1}},
		},
	}

	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	player := state.Players[0]
	if player.HP != 53 || player.MP != 7 {
		t.Fatalf("turn regen should still run while action is suppressed, hp=%v mp=%v", player.HP, player.MP)
	}
	if player.Skills[0].CooldownRemain != 1 {
		t.Fatalf("skill cooldown should still tick while action is suppressed, got %d", player.Skills[0].CooldownRemain)
	}
	if len(player.Buffs) != 0 {
		t.Fatalf("buff duration should still tick and expire, got %#v", player.Buffs)
	}
	if len(state.Events) == 0 {
		t.Fatalf("expected emitted turn event")
	}
	last := state.Events[len(state.Events)-1]
	if last.Type != "battle.turn.action" {
		t.Fatalf("last event=%s, want battle.turn.action", last.Type)
	}
	meta := normalizeAnyMap(last.Data["resultMeta"])
	if meta["reason"] != battleEffectTurnActionSuppressed {
		t.Fatalf("resultMeta.reason=%#v, want %s", meta["reason"], battleEffectTurnActionSuppressed)
	}
	if last.Data["skillId"] != "" {
		t.Fatalf("suppressed action should not cast a skill, skillId=%#v", last.Data["skillId"])
	}
}

func TestProcessBattleTurnAllActionsSuppressedStillSettlesAndAdvancesField(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "test_skill_suppressed", "durationTurns": 1},
		{"id": "test_basic_suppressed", "durationTurns": 1},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{"id": "test_skill_suppressed_active", "buffId": "test_skill_suppressed", "trigger": "while_active", "effects": []map[string]any{{"kind": battleEffectSkillActionSuppressed}}},
		{"id": "test_basic_suppressed_active", "buffId": "test_basic_suppressed", "trigger": "while_active", "effects": []map[string]any{{"kind": battleEffectBasicAttackSuppressed}}},
	}
	definition := meteorBattlefieldTestDefinition()
	state := battleRuntimeState{
		Seed:  1,
		Wave:  1,
		Order: []string{"player:1", "enemy:1"},
		Players: []battleUnitState{{
			UnitID: "player:1", Side: "player", UnitType: "player", UserID: 1, IsPlayerUnit: true, Alive: true,
			HP: 50, MaxHP: 100, MP: 5, MaxMP: 10,
			Stats: map[string]float64{"hp": 100, "mp": 10, "hp_regen": 3, "mp_regen": 2, "speed": 5},
			Skills: []battleSkillState{
				{ID: "power_strike", CooldownTurns: 3, CooldownRemain: 2},
				{ID: "basic_attack", CooldownTurns: 0, CooldownRemain: 0},
			},
			Buffs: []battleBuffState{{BuffID: "test_skill_suppressed", RemainTurns: 1}, {BuffID: "test_basic_suppressed", RemainTurns: 1}},
		}},
		Enemies: []battleUnitState{{
			UnitID: "enemy:1", EnemyID: "dummy", Side: "enemy", UnitType: "enemy", Alive: true,
			HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100, "speed": 1}, BaseStats: map[string]float64{"hp": 100, "speed": 1},
		}},
		BattlefieldEffects:           []battlefieldEffectState{{ID: definition.ID, Remaining: 8, Goal: 1, Windows: map[string]int{}}},
		battlefieldEffectDefinitions: []battleWorldBattlefieldEffect{definition},
	}

	status, _, err := s.processBattleTurn(context.Background(), &state, battleWorldConfig{}, nil, time.UnixMilli(8_000), nil)
	if err != nil || status != "" {
		t.Fatalf("all-suppressed turn failed: status=%q err=%v", status, err)
	}
	player := state.Players[0]
	if player.HP != 53 || player.MP != 7 || player.Skills[0].CooldownRemain != 1 || len(player.Buffs) != 0 {
		t.Fatalf("all-suppressed turn did not fully settle: %#v", player)
	}
	if state.BattlefieldEffects[0].Remaining != 7 {
		t.Fatalf("all-suppressed ordered turn must count for battlefield progress: %#v", state.BattlefieldEffects[0])
	}
	var actionEvent *battleEvent
	for index := range state.Events {
		if state.Events[index].Type == "battle.turn.action" {
			actionEvent = &state.Events[index]
		}
	}
	if actionEvent == nil || normalizeAnyMap(actionEvent.Data["resultMeta"])["reason"] != battleEffectAllActionsSuppressed {
		t.Fatalf("missing authoritative all-actions-suppressed result: %#v", state.Events)
	}
}

func TestOtherUnitTurnEndMarkedTargetFollowupBasicAttack(t *testing.T) {
	s := newMarkedFollowupTestServer()
	state := newMarkedFollowupTestState(true, true)

	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	if len(state.Events) != 2 {
		t.Fatalf("expected main turn plus follow-up event, got %d events: %#v", len(state.Events), state.Events)
	}
	followup := state.Events[1]
	if followup.Type != "battle.turn.action" {
		t.Fatalf("follow-up event type=%s", followup.Type)
	}
	if followup.Data["actorId"] != "player:archer" || followup.Data["skillId"] != "basic_attack" {
		t.Fatalf("unexpected follow-up actor/skill: %#v", followup.Data)
	}
	meta := normalizeAnyMap(followup.Data["resultMeta"])
	if !parseBooleanLoose(meta["markedTargetFollowUp"], false) {
		t.Fatalf("expected markedTargetFollowUp meta, got %#v", meta)
	}
	if got := meta["procChance"]; got != 1.0 {
		t.Fatalf("procChance meta=%#v, want 1", got)
	}
	damagedByFollowup := 0
	for _, enemy := range state.Enemies {
		if enemy.HP <= 80 {
			damagedByFollowup++
		}
	}
	if damagedByFollowup != 1 {
		t.Fatalf("expected exactly one marked enemy to be damaged, enemies=%#v", state.Enemies)
	}
}

func TestOtherUnitTurnEndMarkedTargetFollowupSkipsSelfTurnAndUnmarkedTargets(t *testing.T) {
	t.Run("not_own_turn_end", func(t *testing.T) {
		s := newMarkedFollowupTestServer()
		state := newMarkedFollowupTestState(true, false)
		state.TurnCursor = 1

		_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
		if err != nil {
			t.Fatalf("processBattleTurn error: %v", err)
		}
		if len(state.Events) != 1 {
			t.Fatalf("archer should not trigger off its own turn end, events=%#v", state.Events)
		}
	})
	t.Run("no_marked_target", func(t *testing.T) {
		s := newMarkedFollowupTestServer()
		state := newMarkedFollowupTestState(false, false)

		_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
		if err != nil {
			t.Fatalf("processBattleTurn error: %v", err)
		}
		if len(state.Events) != 1 {
			t.Fatalf("unmarked enemies should not trigger follow-up, events=%#v", state.Events)
		}
	})
	t.Run("proc_chance_zero", func(t *testing.T) {
		s := newMarkedFollowupTestServer()
		state := newMarkedFollowupTestState(true, false)
		state.Players[1].EquipmentHooks[0].Effects[0]["procChanceBase"] = 0
		state.Players[1].EquipmentHooks[0].Effects[0]["procChanceMax"] = 0

		_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
		if err != nil {
			t.Fatalf("processBattleTurn error: %v", err)
		}
		if len(state.Events) != 1 {
			t.Fatalf("zero proc chance should not trigger follow-up, events=%#v", state.Events)
		}
	})
	t.Run("preparation_turn", func(t *testing.T) {
		s := newMarkedFollowupTestServer()
		state := newMarkedFollowupTestState(true, false)
		state.PreparationOrder = []string{"player:other"}

		_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
		if err != nil {
			t.Fatalf("processBattleTurn error: %v", err)
		}
		if len(state.Events) != 1 {
			t.Fatalf("preparation turn should not trigger follow-up, events=%#v", state.Events)
		}
	})
}

func TestOtherUnitTurnEndMarkedTargetFollowupHookIndexUsesCurrentEffects(t *testing.T) {
	s := newMarkedFollowupTestServer()
	state := newMarkedFollowupTestState(true, false)
	completed := &state.Players[0]

	s.executeOtherUnitTurnEndEquipmentHooks(state, time.Unix(0, 0), completed, 1)
	if len(state.Events) != 1 {
		t.Fatalf("expected initial follow-up event, got %#v", state.Events)
	}

	state.Players[1].EquipmentHooks[0].Effects = []map[string]any{
		{
			"kind":           "basic_attack_marked_target",
			"procChanceBase": 0,
			"procChanceMin":  0,
			"procChanceMax":  0,
		},
	}
	s.executeOtherUnitTurnEndEquipmentHooks(state, time.Unix(0, 0), completed, 2)
	if len(state.Events) != 1 {
		t.Fatalf("cached hook index should use current effect values, events=%#v", state.Events)
	}
}

func TestOtherUnitTurnEndMarkedTargetFollowupCachesEmptyHookOwners(t *testing.T) {
	s := newMarkedFollowupTestServer()
	state := newMarkedFollowupTestState(true, false)
	state.Players[1].EquipmentHooks = nil
	completed := &state.Players[0]

	s.executeOtherUnitTurnEndEquipmentHooks(state, time.Unix(0, 0), completed, 1)
	if len(state.Events) != 0 {
		t.Fatalf("state with no other-unit-turn-end hooks should not emit events, got %#v", state.Events)
	}
	if !state.otherUnitTurnEndHookIndexBuilt {
		t.Fatalf("expected other-unit-turn-end hook owner index to be built")
	}
	if len(state.otherUnitTurnEndHookUnitIDs) != 0 {
		t.Fatalf("expected empty hook owner index, got %#v", state.otherUnitTurnEndHookUnitIDs)
	}
}

func newMarkedFollowupTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack", "category": "bow"}}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "marked_target_runtime_test",
			"buffId":  "marked_target_runtime_test",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "marked_target"}},
		},
	}
	s.deps.GameData.CombatConfigRules = []map[string]any{{"default_basic_skill_id": "basic_attack"}}
	return s
}

func newMarkedFollowupTestState(marked bool, includeSecondMarked bool) *battleRuntimeState {
	targeting := testSingleEnemyTargeting("vanguard")
	spec := normalizeBattleDamageSpecForRuntime(battleDamageSpec{
		DamageType:    "physical",
		AmountFormula: battleSkillAmountFormula{BaseAdd: 20},
	}, "physical", battleSkillAmountFormula{BaseAdd: 20})
	basicEffect := battleSkillEffect{
		Kind:          "damage",
		DamageType:    "physical",
		Targeting:     targeting,
		AmountFormula: battleSkillAmountFormula{BaseAdd: 20},
		DamageSpec:    spec,
	}
	enemyBuffs := []battleBuffState(nil)
	if marked {
		enemyBuffs = []battleBuffState{{BuffID: "marked_target_runtime_test", RemainTurns: 2}}
	}
	secondBuffs := []battleBuffState(nil)
	if includeSecondMarked {
		secondBuffs = []battleBuffState{{BuffID: "marked_target_runtime_test", RemainTurns: 2}}
	}
	return &battleRuntimeState{
		Seed:  1,
		Wave:  1,
		Order: []string{"player:other", "player:archer", "enemy:marked", "enemy:other"},
		Players: []battleUnitState{
			{
				UnitID:          "player:other",
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				UserID:          1,
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              20,
				MaxMP:           20,
				Stats:           map[string]float64{"speed": 20, "hp": 100, "mp": 20},
				BaseStats:       map[string]float64{"speed": 20, "hp": 100, "mp": 20},
			},
			{
				UnitID:          "player:archer",
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				UserID:          2,
				Position:        "midguard",
				BasePosition:    "midguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              20,
				MaxMP:           20,
				Stats:           map[string]float64{"speed": 10, "hp": 100, "mp": 20},
				BaseStats:       map[string]float64{"speed": 10, "hp": 100, "mp": 20},
				WeaponClasses:   []string{"bow"},
				BasicAttack:     &battleBasicAttackProfile{Effects: []battleSkillEffect{basicEffect}},
				EquipmentHooks: []battleEquipmentHookRule{
					{
						ID:                 "markseeker_quiver_marked_followup",
						SourceItemID:       "markseeker_quiver",
						SourceEnhanceLevel: 40,
						Trigger:            "on_other_unit_turn_end",
						Conditions:         []map[string]any{{"type": "self_weapon_class", "weaponClasses": []any{"bow"}}},
						Effects: []map[string]any{
							{
								"kind":           "basic_attack_marked_target",
								"procChanceBase": 1,
								"procChanceMin":  0,
								"procChanceMax":  1,
							},
						},
					},
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:marked",
				Side:            "enemy",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"speed": 5, "hp": 100},
				BaseStats:       map[string]float64{"speed": 5, "hp": 100},
				Buffs:           enemyBuffs,
			},
			{
				UnitID:          "enemy:other",
				Side:            "enemy",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"speed": 4, "hp": 100},
				BaseStats:       map[string]float64{"speed": 4, "hp": 100},
				Buffs:           secondBuffs,
			},
		},
	}
}

func TestBattleTurnItemsApplyFoodEffects(t *testing.T) {
	selfTargeting := battleSkillTargeting{
		TargetPositions:     []string{},
		SecondarySeek:       "none",
		TargetScope:         "ally",
		SelectRule:          "self",
		FallbackSelfIfEmpty: true,
		TargetCount:         battleSkillTargetCount{Base: 1},
	}
	newState := func(item battleItemState) *battleRuntimeState {
		return &battleRuntimeState{
			Seed: 1,
			Wave: 1,
			Players: []battleUnitState{
				{
					UnitID:      "player:1",
					Side:        "player",
					UserID:      1,
					Alive:       true,
					HP:          40,
					MaxHP:       100,
					MP:          10,
					MaxMP:       50,
					Stats:       map[string]float64{"speed": 5},
					BattleItems: []battleItemState{item},
					ItemRules:   map[string]battleSkillRuleState{},
				},
			},
			Enemies: []battleUnitState{
				{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"speed": 1}},
			},
		}
	}

	t.Run("heal", func(t *testing.T) {
		s := &Server{}
		state := newState(battleItemState{
			ID:            "battle_item_morning_egg",
			ItemID:        "battle_morning_egg",
			Name:          "Morning Egg",
			Quantity:      2,
			CooldownTurns: 2,
			Targeting:     selfTargeting,
			Effects: []battleSkillEffect{
				{
					Kind:          "heal",
					Targeting:     selfTargeting,
					AmountFormula: battleSkillAmountFormula{BaseAdd: 35},
				},
			},
		})

		s.tryExecuteBattleTurnItem(state, time.Unix(0, 0), &state.Players[0], "player", false)

		player := state.Players[0]
		if player.HP != 75 {
			t.Fatalf("food heal should restore HP, got %v", player.HP)
		}
		if got := player.BattleItems[0].Quantity; got != 1 {
			t.Fatalf("food quantity = %d, want 1", got)
		}
		if got := player.BattleItems[0].CooldownRemain; got != 3 {
			t.Fatalf("food cooldown remain = %d, want 3 before turn-end decrement", got)
		}
		if len(state.Events) != 1 || state.Events[0].Type != "battle.turn.item_use" {
			t.Fatalf("expected battle.turn.item_use event, got %#v", state.Events)
		}
	})

	t.Run("resource", func(t *testing.T) {
		s := &Server{}
		state := newState(battleItemState{
			ID:            "battle_item_sweet_milk",
			ItemID:        "battle_sweet_milk",
			Name:          "Sweet Milk",
			Quantity:      1,
			CooldownTurns: 2,
			Targeting:     selfTargeting,
			Effects: []battleSkillEffect{
				{
					Kind:          "resource",
					Resource:      "mp",
					Targeting:     selfTargeting,
					AmountFormula: battleSkillAmountFormula{BaseAdd: 25},
				},
			},
		})

		s.tryExecuteBattleTurnItem(state, time.Unix(0, 0), &state.Players[0], "player", false)

		if got := state.Players[0].MP; got != 35 {
			t.Fatalf("food resource effect should restore MP, got %v", got)
		}
		if got := state.Players[0].BattleItems[0].Quantity; got != 0 {
			t.Fatalf("food quantity = %d, want 0", got)
		}
	})

	t.Run("status", func(t *testing.T) {
		s := &Server{}
		s.deps.GameData.Buff = []map[string]any{
			{"id": "food_mending_broth_regen", "durationTurns": 24, "stackPolicy": "refresh_duration"},
		}
		state := newState(battleItemState{
			ID:            "battle_item_mending_broth",
			ItemID:        "battle_mending_broth",
			Name:          "Mending Broth",
			Quantity:      1,
			CooldownTurns: 23,
			Targeting:     selfTargeting,
			Effects: []battleSkillEffect{
				{
					Kind:      "status",
					Targeting: selfTargeting,
					StatusPayload: battleSkillStatusPayload{
						StatusIDs:         []string{"food_mending_broth_regen"},
						DurationBaseTurns: 24,
					},
				},
			},
		})

		s.tryExecuteBattleTurnItem(state, time.Unix(0, 0), &state.Players[0], "player", false)

		buffs := state.Players[0].Buffs
		if len(buffs) != 1 || buffs[0].BuffID != "food_mending_broth_regen" || buffs[0].RemainTurns != 24 {
			t.Fatalf("food status effect not applied correctly: %#v", buffs)
		}
	})
}

func TestMirrorCastSkillTriggersForOtherNonBasicSkill(t *testing.T) {
	s := newMirrorCastTestServer()
	state := newMirrorCastTestState()

	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	actions := battleTurnActionEventsForTest(state)
	if len(actions) != 2 {
		t.Fatalf("expected primary cast and mirror cast events, got %d: %#v", len(actions), state.Events)
	}
	primary := actions[0]
	mirror := actions[1]
	if primary.Data["actorId"] != "player:caster" || primary.Data["skillId"] != "arcane_bolt" {
		t.Fatalf("unexpected primary event: %#v", primary.Data)
	}
	if mirror.Data["actorId"] != "player:mirror" || mirror.Data["skillId"] != "arcane_bolt" {
		t.Fatalf("unexpected mirror event: %#v", mirror.Data)
	}
	if got := toInt64Loose(mirror.Data["parentSeq"], 0); got != primary.Seq {
		t.Fatalf("mirror parentSeq=%d, want %d", got, primary.Seq)
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", mirror.Data["batchGroup"])); got != fmt.Sprintf("mirror:%d", primary.Seq) {
		t.Fatalf("mirror batchGroup=%q", got)
	}
	meta := normalizeAnyMap(mirror.Data["resultMeta"])
	if !parseBooleanLoose(meta["mirrorCast"], false) {
		t.Fatalf("mirror event missing resultMeta.mirrorCast: %#v", meta)
	}
	mirrorUnit := battleUnitByID(state, "player:mirror")
	if mirrorUnit == nil {
		t.Fatalf("mirror unit missing")
	}
	if got := mirrorUnit.MP; got != 87 {
		t.Fatalf("expected discounted mirror MP cost floor(20*0.67)=13, mp=%v", got)
	}
	if got := mirrorUnit.Skills[0].CooldownRemain; got != 2 {
		t.Fatalf("expected mirror skill cooldown 2, got %d", got)
	}
}

func TestMirrorCastSkillDoesNotTriggerForBasicAttack(t *testing.T) {
	s := newMirrorCastTestServer()
	state := newMirrorCastTestState()
	state.Players[0].Skills = []battleSkillState{mirrorCastBasicSkill()}
	state.Players[0].SkillIDs = []string{"basic_attack"}
	state.Players[0].SkillSlots = []string{"basic_attack"}

	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	if len(battleTurnActionEventsForTest(state)) != 1 {
		t.Fatalf("basic attack should not trigger mirror cast, events=%#v", state.Events)
	}
}

func TestMirrorCastSkillRequiresOwnedReadySkillAndResources(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*battleRuntimeState)
	}{
		{
			name: "missing_skill",
			mutate: func(state *battleRuntimeState) {
				state.Players[1].Skills = []battleSkillState{mirrorCastBasicSkill()}
				state.Players[1].SkillIDs = []string{"basic_attack"}
			},
		},
		{
			name: "cooldown",
			mutate: func(state *battleRuntimeState) {
				state.Players[1].Skills[0].CooldownRemain = 1
			},
		},
		{
			name: "mp",
			mutate: func(state *battleRuntimeState) {
				state.Players[1].MP = 12
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMirrorCastTestServer()
			state := newMirrorCastTestState()
			tt.mutate(state)
			_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
			if err != nil {
				t.Fatalf("processBattleTurn error: %v", err)
			}
			if len(battleTurnActionEventsForTest(state)) != 1 {
				t.Fatalf("mirror should be blocked, events=%#v", state.Events)
			}
		})
	}
}

func TestMirrorCastSkillTriggerScope(t *testing.T) {
	tests := []struct {
		scope      string
		wantMirror bool
	}{
		{scope: "all", wantMirror: true},
		{scope: "ally", wantMirror: true},
		{scope: "enemy", wantMirror: false},
	}
	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			s := newMirrorCastTestServer()
			state := newMirrorCastTestState()
			state.Players[1].EquipmentHooks[0].Effects = []map[string]any{{
				"kind":             "mirror_cast_skill",
				"triggerScope":     tt.scope,
				"targetingMode":    "reselect",
				"mpCostMultiplier": 0.67,
			}}
			_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
			if err != nil {
				t.Fatalf("processBattleTurn error: %v", err)
			}
			gotMirror := len(battleTurnActionEventsForTest(state)) == 2
			if gotMirror != tt.wantMirror {
				t.Fatalf("mirror=%t, want %t, events=%#v", gotMirror, tt.wantMirror, state.Events)
			}
		})
	}
}

func TestMirrorCastSkillFilters(t *testing.T) {
	tests := []struct {
		name       string
		effect     map[string]any
		wantMirror bool
	}{
		{
			name:       "skill_id_match",
			effect:     map[string]any{"skillIds": []string{"arcane_bolt"}},
			wantMirror: true,
		},
		{
			name:       "skill_id_mismatch",
			effect:     map[string]any{"skillIds": []string{"other_skill"}},
			wantMirror: false,
		},
		{
			name:       "category_match",
			effect:     map[string]any{"skillCategories": []string{"arcane"}},
			wantMirror: true,
		},
		{
			name:       "category_mismatch",
			effect:     map[string]any{"skillCategories": []string{"holy"}},
			wantMirror: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMirrorCastTestServer()
			state := newMirrorCastTestState()
			effect := map[string]any{
				"kind":             "mirror_cast_skill",
				"triggerScope":     "all",
				"targetingMode":    "reselect",
				"mpCostMultiplier": 0.67,
			}
			for key, value := range tt.effect {
				effect[key] = value
			}
			state.Players[1].EquipmentHooks[0].Effects = []map[string]any{effect}
			_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
			if err != nil {
				t.Fatalf("processBattleTurn error: %v", err)
			}
			gotMirror := len(battleTurnActionEventsForTest(state)) == 2
			if gotMirror != tt.wantMirror {
				t.Fatalf("mirror=%t, want %t, events=%#v", gotMirror, tt.wantMirror, state.Events)
			}
		})
	}
}

func TestMirrorCastSkillCanSkipCooldown(t *testing.T) {
	s := newMirrorCastTestServer()
	state := newMirrorCastTestState()
	state.Players[1].EquipmentHooks[0].Effects = []map[string]any{{
		"kind":             "mirror_cast_skill",
		"triggerScope":     "all",
		"targetingMode":    "reselect",
		"mpCostMultiplier": 0.67,
		"applyCooldown":    false,
	}}

	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	actions := battleTurnActionEventsForTest(state)
	if len(actions) != 2 {
		t.Fatalf("expected mirror cast, got %d actions: %#v", len(actions), state.Events)
	}
	mirrorUnit := battleUnitByID(state, "player:mirror")
	if mirrorUnit == nil {
		t.Fatalf("mirror unit missing")
	}
	if got := mirrorUnit.Skills[0].CooldownRemain; got != 0 {
		t.Fatalf("expected mirror skill cooldown to stay 0, got %d", got)
	}
	meta := normalizeAnyMap(actions[1].Data["resultMeta"])
	if parseBooleanLoose(meta["applyCooldown"], true) {
		t.Fatalf("expected resultMeta.applyCooldown=false, got %#v", meta)
	}
}

func TestMirrorCastSkillDoesNotChainMirroredCastsByDefault(t *testing.T) {
	s := newMirrorCastTestServer()
	state := newMirrorCastTestState()
	state.Players = append(state.Players, battleUnitState{
		UnitID:          "player:mirror:2",
		Side:            "player",
		UserID:          3,
		Position:        "vanguard",
		BasePosition:    "vanguard",
		OriginSide:      "player",
		BaseSide:        "player",
		PerspectiveSide: "player",
		Alive:           true,
		HP:              100,
		MaxHP:           100,
		MP:              100,
		MaxMP:           100,
		Stats:           map[string]float64{"speed": 7, "hp": 100, "mp": 100, "magical_damage": 1},
		BaseStats:       map[string]float64{"speed": 7, "hp": 100, "mp": 100, "magical_damage": 1},
		Skills:          []battleSkillState{mirrorCastArcaneSkill()},
		SkillIDs:        []string{"arcane_bolt"},
		SkillSlots:      []string{"arcane_bolt"},
		EquipmentHooks:  []battleEquipmentHookRule{mirrorCastTestHook()},
	})

	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	actions := battleTurnActionEventsForTest(state)
	if len(actions) != 3 {
		t.Fatalf("expected primary plus two direct mirrors, got %d: %#v", len(actions), state.Events)
	}
	for _, event := range actions[1:] {
		meta := normalizeAnyMap(event.Data["resultMeta"])
		if !parseBooleanLoose(meta["mirrorCast"], false) {
			t.Fatalf("expected mirror event, got %#v", event.Data)
		}
	}
}

func mirrorCastHearthsideThawSkill() battleSkillState {
	targeting := battleSkillTargeting{
		TargetPositions:     []string{"vanguard", "midguard", "rearguard"},
		SecondarySeek:       "none",
		TargetScope:         "ally",
		SelectRule:          "hp_percent_min",
		FallbackSelfIfEmpty: true,
		TargetCount:         battleSkillTargetCount{Base: 1},
	}
	return battleSkillState{
		ID:            "hearthside_thaw",
		Name:          "Hearthside Thaw",
		Category:      "snowpact",
		MPCost:        18,
		CooldownTurns: 4,
		Targeting:     targeting,
		CastGuards:    []battleSkillCastGuard{{Kind: "ally_side_has_non_full_hp"}},
		Effects: []battleSkillEffect{{
			Kind:      "heal",
			Targeting: targeting,
			AmountFormula: battleSkillAmountFormula{
				Terms: []battleAttackTerm{
					{AttrKey: "magical_damage", Coefficient: 0.9},
					{AttrKey: "intelligence", Coefficient: 0.7},
				},
			},
		}},
	}
}

func TestMirrorCastHearthsideThawTriggersForKittyAndTeammate(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack", "category": "blade"},
		{"id": "hearthside_thaw", "category": "snowpact"},
	}

	kittyHook := mirrorCastTestHook()
	teammateHook := mirrorCastTestHook()
	hearthsideSkill := mirrorCastHearthsideThawSkill()
	basicSkill := mirrorCastBasicSkill()

	state := &battleRuntimeState{
		Seed:  1,
		Wave:  1,
		Order: []string{"player:caster", "kitty:1", "player:teammate", "enemy:target"},
		Players: []battleUnitState{
			{
				UnitID:          "player:caster",
				Side:            "player",
				UserID:          1,
				Position:        "vanguard",
				BasePosition:    "vanguard",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              100,
				MaxMP:           100,
				Stats:           map[string]float64{"speed": 10, "hp": 100, "mp": 100, "magical_damage": 10, "intelligence": 10},
				BaseStats:       map[string]float64{"speed": 10, "hp": 100, "mp": 100, "magical_damage": 10, "intelligence": 10},
				Skills:          []battleSkillState{hearthsideSkill, basicSkill},
				SkillIDs:        []string{"hearthside_thaw", "basic_attack"},
				SkillSlots:      []string{"hearthside_thaw", "basic_attack"},
			},
			{
				UnitID:          "kitty:1",
				Side:            "player",
				UserID:          1,
				UnitType:        "kitty",
				KittyID:         1,
				Position:        "midguard",
				BasePosition:    "midguard",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              50,
				MaxHP:           100,
				MP:              100,
				MaxMP:           100,
				Stats:           map[string]float64{"speed": 8, "hp": 100, "mp": 100, "magical_damage": 10, "intelligence": 10},
				BaseStats:       map[string]float64{"speed": 8, "hp": 100, "mp": 100, "magical_damage": 10, "intelligence": 10},
				Skills:          []battleSkillState{hearthsideSkill, basicSkill},
				SkillIDs:        []string{"hearthside_thaw", "basic_attack"},
				SkillSlots:      []string{"hearthside_thaw", "basic_attack"},
				EquipmentHooks:  []battleEquipmentHookRule{kittyHook},
			},
			{
				UnitID:          "player:teammate",
				Side:            "player",
				UserID:          2,
				Position:        "rearguard",
				BasePosition:    "rearguard",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              30,
				MaxHP:           100,
				MP:              100,
				MaxMP:           100,
				Stats:           map[string]float64{"speed": 7, "hp": 100, "mp": 100, "magical_damage": 10, "intelligence": 10},
				BaseStats:       map[string]float64{"speed": 7, "hp": 100, "mp": 100, "magical_damage": 10, "intelligence": 10},
				Skills:          []battleSkillState{hearthsideSkill, basicSkill},
				SkillIDs:        []string{"hearthside_thaw", "basic_attack"},
				SkillSlots:      []string{"hearthside_thaw", "basic_attack"},
				EquipmentHooks:  []battleEquipmentHookRule{teammateHook},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:target",
				Side:            "enemy",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              0,
				MaxMP:           0,
				Stats:           map[string]float64{"speed": 1, "hp": 100},
				BaseStats:       map[string]float64{"speed": 1, "hp": 100},
			},
		},
	}

	// Force caster to act first by processing one turn.
	_, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}

	actions := battleTurnActionEventsForTest(state)
	if len(actions) == 0 {
		t.Fatalf("expected at least one action event, got 0")
	}

	primary := actions[0]
	if primary.Data["skillId"] != "hearthside_thaw" {
		t.Fatalf("expected primary cast of hearthside_thaw, got skillId=%v", primary.Data["skillId"])
	}

	mirrorCount := 0
	mirrorActors := []string{}
	for _, event := range actions[1:] {
		meta := normalizeAnyMap(event.Data["resultMeta"])
		if parseBooleanLoose(meta["mirrorCast"], false) {
			mirrorCount++
			mirrorActors = append(mirrorActors, fmt.Sprintf("%v", event.Data["actorId"]))
			if event.Data["skillId"] != "hearthside_thaw" {
				t.Fatalf("mirror cast should be hearthside_thaw, got %v", event.Data["skillId"])
			}
		}
	}

	if mirrorCount != 2 {
		t.Fatalf("expected 2 mirror casts (kitty + teammate), got %d. actors: %v. total events: %d", mirrorCount, mirrorActors, len(state.Events))
	}

	hasKitty := false
	hasTeammate := false
	for _, actor := range mirrorActors {
		if actor == "kitty:1" {
			hasKitty = true
		}
		if actor == "player:teammate" {
			hasTeammate = true
		}
	}
	if !hasKitty {
		t.Fatalf("expected kitty to mirror cast, but actors were: %v", mirrorActors)
	}
	if !hasTeammate {
		t.Fatalf("expected teammate to mirror cast, but actors were: %v", mirrorActors)
	}

	// Check that mirror cast targets include allies (not just self).
	for _, event := range actions[1:] {
		meta := normalizeAnyMap(event.Data["resultMeta"])
		if !parseBooleanLoose(meta["mirrorCast"], false) {
			continue
		}
		actorID := fmt.Sprintf("%v", event.Data["actorId"])
		targetIDs := []string{}
		for _, row := range battleAnyMapSlice(event.Data["effects"]) {
			if tid := fmt.Sprintf("%v", row["targetId"]); tid != "" && tid != "<nil>" {
				targetIDs = append(targetIDs, tid)
			}
		}
		// Verify each mirror cast targeted an ally (not only self).
		hasAllyTarget := false
		for _, tid := range targetIDs {
			if tid != actorID {
				hasAllyTarget = true
				break
			}
		}
		if !hasAllyTarget {
			t.Fatalf("mirror actor=%v only targeted self, no ally targets. targets=%v", actorID, targetIDs)
		}
	}
}

func newMirrorCastTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack", "category": "blade"},
		{"id": "arcane_bolt", "category": "arcane"},
	}
	return s
}

func mirrorCastTestHook() battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID:           "folding_boundary_orb_magic_fold_mirror",
		SourceItemID: "folding_boundary_orb",
		Trigger:      "on_other_unit_cast_skill",
		Priority:     0,
		Conditions:   []map[string]any{},
		Effects: []map[string]any{{
			"kind":                     "mirror_cast_skill",
			"triggerScope":             "all",
			"targetingMode":            "reselect",
			"mpCostMultiplier":         0.67,
			"includeBasicAttack":       false,
			"allowMirroredCastTrigger": false,
			"applyCooldown":            true,
		}},
	}
}

func newMirrorCastTestState() *battleRuntimeState {
	return &battleRuntimeState{
		Seed:  1,
		Wave:  1,
		Order: []string{"player:caster", "player:mirror", "enemy:target"},
		Players: []battleUnitState{
			{
				UnitID:          "player:caster",
				Side:            "player",
				UserID:          1,
				Position:        "vanguard",
				BasePosition:    "vanguard",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              100,
				MaxMP:           100,
				Stats:           map[string]float64{"speed": 10, "hp": 100, "mp": 100, "magical_damage": 1},
				BaseStats:       map[string]float64{"speed": 10, "hp": 100, "mp": 100, "magical_damage": 1},
				Skills:          []battleSkillState{mirrorCastArcaneSkill(), mirrorCastBasicSkill()},
				SkillIDs:        []string{"arcane_bolt", "basic_attack"},
				SkillSlots:      []string{"arcane_bolt", "basic_attack"},
			},
			{
				UnitID:          "player:mirror",
				Side:            "player",
				UserID:          2,
				Position:        "vanguard",
				BasePosition:    "vanguard",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              100,
				MaxMP:           100,
				Stats:           map[string]float64{"speed": 9, "hp": 100, "mp": 100, "magical_damage": 1},
				BaseStats:       map[string]float64{"speed": 9, "hp": 100, "mp": 100, "magical_damage": 1},
				Skills:          []battleSkillState{mirrorCastArcaneSkill(), mirrorCastBasicSkill()},
				SkillIDs:        []string{"arcane_bolt", "basic_attack"},
				SkillSlots:      []string{"arcane_bolt", "basic_attack"},
				EquipmentHooks:  []battleEquipmentHookRule{mirrorCastTestHook()},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:target",
				Side:            "enemy",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				MP:              0,
				MaxMP:           0,
				Stats:           map[string]float64{"speed": 1, "hp": 100},
				BaseStats:       map[string]float64{"speed": 1, "hp": 100},
			},
		},
	}
}

func mirrorCastArcaneSkill() battleSkillState {
	targeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount:     battleSkillTargetCount{Base: 1},
	}
	return battleSkillState{
		ID:            "arcane_bolt",
		Name:          "Arcane Bolt",
		Category:      "arcane",
		MPCost:        20,
		CooldownTurns: 2,
		Targeting:     targeting,
		Effects: []battleSkillEffect{{
			Kind:          "damage",
			DamageType:    "magical",
			Targeting:     targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
			DamageSpec: battleDamageSpec{
				DamageType:    "magical",
				AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
				HitCount:      battleSkillTargetCount{Base: 1},
			},
		}},
	}
}

func mirrorCastBasicSkill() battleSkillState {
	targeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount:     battleSkillTargetCount{Base: 1},
	}
	return battleSkillState{
		ID:        "basic_attack",
		Name:      "Basic Attack",
		Category:  "blade",
		Targeting: targeting,
		Effects: []battleSkillEffect{{
			Kind:          "damage",
			DamageType:    "physical",
			Targeting:     targeting,
			AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
			DamageSpec: battleDamageSpec{
				DamageType:    "physical",
				AmountFormula: battleSkillAmountFormula{BaseAdd: 1},
				HitCount:      battleSkillTargetCount{Base: 1},
			},
		}},
	}
}

func battleTurnActionEventsForTest(state *battleRuntimeState) []battleEvent {
	if state == nil {
		return nil
	}
	out := make([]battleEvent, 0, len(state.Events))
	for _, event := range state.Events {
		if event.Type == "battle.turn.action" {
			out = append(out, event)
		}
	}
	return out
}

func TestResolveSpecialWaveEnemyIDsPeriodicRandom(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 4242,
		Wave: 15,
	}
	cfg := battleWorldConfig{
		SpecialEnemyIDs:     []string{"plain_boss_001"},
		SpecialWaveEvery:    15,
		SpecialWaveMinCount: 1,
		SpecialWaveMaxCount: 2,
	}
	ids := s.resolveSpecialWaveEnemyIDs(state, cfg)
	if len(ids) < 1 || len(ids) > 2 {
		t.Fatalf("expected periodic special count in [1,2], got %d (%#v)", len(ids), ids)
	}
	for _, id := range ids {
		if id != "plain_boss_001" {
			t.Fatalf("unexpected special enemy id: %s", id)
		}
	}

	state.Wave = 14
	ids = s.resolveSpecialWaveEnemyIDs(state, cfg)
	if len(ids) != 0 {
		t.Fatalf("expected no periodic special enemies on non-hit wave, got %#v", ids)
	}
}

func TestResolveSpecialWaveEnemyIDsFixedWaveCompatibility(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 99,
		Wave: 3,
	}
	cfg := battleWorldConfig{
		SpecialEnemyIDs: []string{"boss_a", "boss_b"},
		SpecialWaves: map[int]struct{}{
			3: {},
		},
	}
	ids := s.resolveSpecialWaveEnemyIDs(state, cfg)
	if len(ids) != 2 {
		t.Fatalf("expected 2 fixed special enemies, got %#v", ids)
	}
	if ids[0] != "boss_a" || ids[1] != "boss_b" {
		t.Fatalf("fixed wave special enemies mismatch: %#v", ids)
	}
}

func TestDecodePlayersFromFinalResultPreservesHPAndBasicAttackProfile(t *testing.T) {
	state := battleRuntimeState{
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID: "player:1",
				Side:   "player",
				UserID: 1,
				Alive:  true,
				HP:     87.5,
				MaxHP:  120,
				MP:     15,
				MaxMP:  40,
				Stats: map[string]float64{
					"hp": 120,
					"mp": 40,
				},
				StatMode: battleStatModeProjected,
				SkillRules: map[string]battleSkillRuleState{
					"power_strike": {
						Enabled: false,
						Groups: []rules.ConditionGroup{
							{Conditions: []rules.Condition{{CompareType: "value", DataType: "enemy_count", Target: "enemies", ValueCompareAction: "<=", ValueConditionValue: 2}}},
						},
					},
				},
				Skills: []battleSkillState{
					{ID: "power_strike", CooldownTurns: 3, CooldownRemain: 2, Level: 4, Exp: 50},
				},
				Buffs: []battleBuffState{
					{BuffID: "test_buff", RemainTurns: 2, Payload: map[string]any{"x": float64(1)}},
				},
				BasicAttack: &battleBasicAttackProfile{
					TargetDirection:    "midguard",
					TargetSeekStrategy: "front_to_rear",
					DamageType:         "magical",
					Formula: []battleAttackTerm{
						{AttrKey: "magical_damage", Coefficient: 1},
					},
					DamageSpec: battleDamageSpec{
						DamageType: "magical",
						AmountFormula: battleSkillAmountFormula{
							BaseAdd: 0,
							Terms: []battleAttackTerm{
								{AttrKey: "magical_damage", Coefficient: 1},
							},
						},
						HitCount: battleSkillTargetCount{
							Base: 1,
						},
					},
					WeaponItemID: "wood_staff",
				},
			},
		},
	}
	finalResult := map[string]any{
		"finalState": battleSettlementStatePayload(state, "ended"),
		"carryState": battleCarryStatePayload(state, "ended"),
	}
	carried := decodeBattlePlayersFromFinalResult(finalResult)
	if len(carried) != 1 {
		t.Fatalf("expected 1 carried player, got %d", len(carried))
	}
	normalizeBattleUnitState(&carried[0])
	if !carried[0].Alive {
		t.Fatalf("carried player should stay alive")
	}
	if carried[0].HP <= 0 {
		t.Fatalf("carried player HP should be > 0, got %v", carried[0].HP)
	}
	if carried[0].BasicAttack == nil {
		t.Fatalf("carried player basicAttack should not be nil")
	}
	if carried[0].BasicAttack.DamageType != "magical" {
		t.Fatalf("carried player basicAttack damageType=%s, want magical", carried[0].BasicAttack.DamageType)
	}
	if carried[0].BasicAttack.WeaponItemID != "wood_staff" {
		t.Fatalf("carried player basicAttack weaponItemID=%s, want wood_staff", carried[0].BasicAttack.WeaponItemID)
	}
	if carried[0].StatMode != battleStatModeProjected {
		t.Fatalf("carried statMode=%s, want %s", carried[0].StatMode, battleStatModeProjected)
	}
	if len(carried[0].SkillRules["power_strike"].Groups) != 1 {
		t.Fatalf("carried skill rules missing: %#v", carried[0].SkillRules)
	}
	if len(carried[0].Skills) != 1 || carried[0].Skills[0].CooldownRemain != 2 || carried[0].Skills[0].Level != 4 {
		t.Fatalf("carried skills mismatch: %#v", carried[0].Skills)
	}
	if len(carried[0].Buffs) != 1 || carried[0].Buffs[0].BuffID != "test_buff" {
		t.Fatalf("carried buffs mismatch: %#v", carried[0].Buffs)
	}
}

func TestDecodePlayersFromFinalResultKeepsPlayerAndKittyEquipmentSeparate(t *testing.T) {
	state := battleRuntimeState{
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:          "player:1",
				Side:            "player",
				UserID:          1,
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"hp": 100, "physics_damage": 12},
				EquipmentDigest: []string{"wood_sword+3"},
				EquipmentHooks: []battleEquipmentHookRule{
					{ID: "player_hook", SourceItemID: "wood_sword", Trigger: "on_battle_start"},
				},
				BasicAttack: &battleBasicAttackProfile{
					DamageType:   "physical",
					WeaponItemID: "wood_sword",
				},
			},
			{
				UnitID:          "kitty:77",
				UnitType:        "kitty",
				Side:            "player",
				UserID:          1,
				Alive:           true,
				HP:              90,
				MaxHP:           90,
				Stats:           map[string]float64{"hp": 90, "magical_damage": 25},
				EquipmentDigest: []string{"rock_golem_silkwood_staff+0"},
				EquipmentHooks: []battleEquipmentHookRule{
					{ID: "kitty_hook", SourceItemID: "rock_golem_silkwood_staff", Trigger: "on_battle_start"},
				},
				BasicAttack: &battleBasicAttackProfile{
					DamageType:   "magical",
					WeaponItemID: "rock_golem_silkwood_staff",
				},
			},
		},
	}

	finalResult := map[string]any{
		"carryState": battleCarryStatePayload(state, "ended"),
	}
	carried := decodeBattlePlayersFromFinalResult(finalResult)
	if len(carried) != 2 {
		t.Fatalf("expected 2 carried units, got %d", len(carried))
	}

	byID := map[string]battleUnitState{}
	for _, unit := range carried {
		byID[unit.UnitID] = unit
	}

	player := byID["player:1"]
	if len(player.EquipmentDigest) != 1 || player.EquipmentDigest[0] != "wood_sword+3" {
		t.Fatalf("player equipment digest mismatch: %#v", player.EquipmentDigest)
	}
	if player.BasicAttack == nil || player.BasicAttack.WeaponItemID != "wood_sword" {
		t.Fatalf("player basic attack mismatch: %#v", player.BasicAttack)
	}
	if len(player.EquipmentHooks) != 1 || player.EquipmentHooks[0].SourceItemID != "wood_sword" {
		t.Fatalf("player equipment hooks mismatch: %#v", player.EquipmentHooks)
	}

	kitty := byID["kitty:77"]
	if len(kitty.EquipmentDigest) != 1 || kitty.EquipmentDigest[0] != "rock_golem_silkwood_staff+0" {
		t.Fatalf("kitty equipment digest mismatch: %#v", kitty.EquipmentDigest)
	}
	if kitty.BasicAttack == nil || kitty.BasicAttack.WeaponItemID != "rock_golem_silkwood_staff" {
		t.Fatalf("kitty basic attack mismatch: %#v", kitty.BasicAttack)
	}
	if len(kitty.EquipmentHooks) != 1 || kitty.EquipmentHooks[0].SourceItemID != "rock_golem_silkwood_staff" {
		t.Fatalf("kitty equipment hooks mismatch: %#v", kitty.EquipmentHooks)
	}
}

func TestNormalizeBattleUnitStateAliveFromHP(t *testing.T) {
	unit := &battleUnitState{
		UnitID: "player:1",
		Side:   "player",
		UserID: 1,
		Alive:  false, // legacy / malformed snapshot
		HP:     30,
		MaxHP:  100,
		MP:     10,
		MaxMP:  20,
	}
	normalizeBattleUnitState(unit)
	if !unit.Alive {
		t.Fatalf("unit with HP>0 should be normalized as alive")
	}
	if unit.HP <= 0 {
		t.Fatalf("unit HP should stay positive, got %v", unit.HP)
	}
}

func TestProcessBattleTurnFailsWhenPlayerStartsDeadFromZeroMaxHP(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 1,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID: "player:1",
				Side:   "player",
				UserID: 1,
				Alive:  false,
				HP:     0,
				MaxHP:  0,
				Stats:  map[string]float64{"speed": 1},
			},
		},
	}
	worldCfg := battleWorldConfig{
		EnemyIDs:              []string{"enemy_dummy"},
		NormalEnemyCountRange: battleCountRange{Min: 1, Max: 1},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"enemy_dummy": {
			ID:       "enemy_dummy",
			Name:     "Enemy",
			Position: "vanguard",
			Stats:    map[string]float64{"hp": 10, "speed": 1},
		},
	}

	status, advanced, err := s.processBattleTurn(context.Background(), state, worldCfg, enemyCatalog, time.Now(), map[int64]store.BattleWaveReward{})
	if err != nil {
		t.Fatalf("processBattleTurn returned error: %v", err)
	}
	if status != store.BattleRunStatusFailed {
		t.Fatalf("status=%s, want %s", status, store.BattleRunStatusFailed)
	}
	if advanced {
		t.Fatalf("dead player should not advance battle")
	}
}

func TestBattleNextActorRebuildsStaleOrderWhenAliveUnitsChanged(t *testing.T) {
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100, Stats: map[string]float64{"speed": 5}},
			{UnitID: "player:2", Side: "player", Alive: true, HP: 100, Stats: map[string]float64{"speed": 4}},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, Stats: map[string]float64{"speed": 3}},
			{UnitID: "enemy:2", Side: "enemy", Alive: true, HP: 100, Stats: map[string]float64{"speed": 2}},
			{UnitID: "enemy:3", Side: "enemy", Alive: true, HP: 100, Stats: map[string]float64{"speed": 1}},
		},
		// stale order only contains two units; runtime should rebuild this before selecting actor.
		Order:      []string{"player:1", "enemy:1"},
		TurnCursor: 0,
	}
	seen := map[string]struct{}{}
	for i := 0; i < 5; i++ {
		actor, _, idx, isPreparationTurn := battleNextActor(state)
		if actor == nil {
			t.Fatalf("expected actor on turn %d", i+1)
		}
		if isPreparationTurn {
			t.Fatalf("expected normal turn actor on turn %d", i+1)
		}
		seen[actor.UnitID] = struct{}{}
		state.advanceTurnCursor(idx)
	}
	if len(seen) != 5 {
		t.Fatalf("stale order should be rebuilt to include all alive units, seen=%#v order=%#v", seen, state.Order)
	}
}

func TestBattleRuntimeOrderSkipsUnitsMarkedSkipTurnOrder(t *testing.T) {
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:owner", Side: "player", Alive: true, HP: 100, Stats: map[string]float64{"speed": 10}},
			{UnitID: "player:summon", Side: "player", Alive: true, HP: 100, Stats: map[string]float64{"speed": 9}, SkipTurnOrder: true},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, Stats: map[string]float64{"speed": 8}},
		},
	}

	state.reseedOrder()

	if got, want := state.FullOrder, []string{"player:owner", "player:summon", "enemy:1"}; !slices.Equal(got, want) {
		t.Fatalf("full order mismatch: got=%#v want=%#v", got, want)
	}
	if got, want := state.Order, []string{"player:owner", "enemy:1"}; !slices.Equal(got, want) {
		t.Fatalf("runtime order mismatch: got=%#v want=%#v", got, want)
	}
	for i := 0; i < 4; i++ {
		actor, _, idx, prep := battleNextActor(state)
		if actor == nil {
			t.Fatalf("expected actor at turn %d", i+1)
		}
		if prep {
			t.Fatalf("unexpected preparation actor at turn %d", i+1)
		}
		if actor.UnitID == "player:summon" {
			t.Fatalf("skip-turn unit should not be selected as normal actor")
		}
		state.advanceTurnCursor(idx)
	}
}

func TestPlayerCloneTriggersAfterSummonHooks(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "summon_sync_entrained", "durationTurns": 6},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "summon_sync_entrained_wa",
			"buffId":  "summon_sync_entrained",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "skip_turn_order"}},
		},
	}

	source := battleUnitState{
		UnitID:       "player:owner",
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
		EquipmentHooks: []battleEquipmentHookRule{
			{
				ID:      "clone_on_wave_start",
				Trigger: "on_wave_start",
				Effects: []map[string]any{{
					"kind":          "player_clone",
					"durationTurns": 5,
				}},
			},
			{
				ID:      "entrain_on_after_summon",
				Trigger: "on_after_summon",
				Effects: []map[string]any{{
					"kind":          "add_buff",
					"buffId":        "summon_sync_entrained",
					"durationTurns": 6,
					"targetMode":    "source",
				}},
			},
		},
		Skills:     []battleSkillState{{ID: "basic_attack"}},
		SkillIDs:   []string{"basic_attack"},
		SkillSlots: []string{"basic_attack"},
	}

	state := &battleRuntimeState{
		Players: []battleUnitState{source},
		Order:   []string{"player:owner"},
	}

	actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[0], nil, "on_wave_start", 0, "")
	if len(actions) == 0 {
		t.Fatalf("expected player clone actions")
	}
	if len(state.Players) != 2 {
		t.Fatalf("expected clone to be added, players=%d", len(state.Players))
	}
	clone := state.Players[1]
	if clone.SummonSourceUnitID != "player:owner" {
		t.Fatalf("clone summon source mismatch: %#v", clone.SummonSourceUnitID)
	}
	if !clone.SkipTurnOrder {
		t.Fatalf("clone should receive skip-turn-order from on_after_summon hook")
	}
	if got, want := state.Order, []string{"player:owner"}; !slices.Equal(got, want) {
		t.Fatalf("runtime order mismatch after clone summon: got=%#v want=%#v", got, want)
	}
}

func TestPlayerCloneAfterSummonSyncConsumesMPAndRequiresEnoughMP(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "summon_sync_entrained", "durationTurns": 6},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "summon_sync_entrained_wa",
			"buffId":  "summon_sync_entrained",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "skip_turn_order"}},
		},
	}

	buildSource := func(mp float64) battleUnitState {
		return battleUnitState{
			UnitID:       "player:owner",
			Side:         "player",
			OriginSide:   "player",
			BaseSide:     "player",
			Alive:        true,
			HP:           100,
			MaxHP:        100,
			MP:           mp,
			MaxMP:        100,
			Position:     "vanguard",
			BasePosition: "vanguard",
			Stats:        map[string]float64{"hp": 100, "mp": 100, "speed": 10},
			BaseStats:    map[string]float64{"hp": 100, "mp": 100, "speed": 10},
			EquipmentHooks: []battleEquipmentHookRule{
				{
					ID:      "clone_on_wave_start",
					Trigger: "on_wave_start",
					Effects: []map[string]any{{
						"kind":          "player_clone",
						"durationTurns": 5,
					}},
				},
				{
					ID:      "entrain_on_after_summon",
					Trigger: "on_after_summon",
					Effects: []map[string]any{
						{
							"kind": "consume_mp",
							"amountFormula": map[string]any{
								"baseAdd": 25,
								"terms":   []map[string]any{},
							},
							"requireFullAmount": true,
							"abortHookOnFail":   true,
						},
						{
							"kind":          "add_buff",
							"buffId":        "summon_sync_entrained",
							"durationTurns": 6,
							"targetMode":    "source",
						},
					},
				},
			},
			Skills:     []battleSkillState{{ID: "basic_attack"}},
			SkillIDs:   []string{"basic_attack"},
			SkillSlots: []string{"basic_attack"},
		}
	}

	t.Run("enough mp", func(t *testing.T) {
		state := &battleRuntimeState{
			Players: []battleUnitState{buildSource(40)},
			Order:   []string{"player:owner"},
		}

		actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[0], nil, "on_wave_start", 0, "")
		if len(actions) == 0 {
			t.Fatalf("expected clone actions")
		}
		if got := state.Players[0].MP; got != 15 {
			t.Fatalf("expected owner MP to become 15, got %v", got)
		}
		if len(state.Players) != 2 {
			t.Fatalf("expected clone to be added, players=%d", len(state.Players))
		}
		if !state.Players[1].SkipTurnOrder {
			t.Fatalf("expected clone to receive synced-turn buff when MP is enough")
		}
		if got, want := state.Order, []string{"player:owner"}; !slices.Equal(got, want) {
			t.Fatalf("runtime order mismatch: got=%#v want=%#v", got, want)
		}
	})

	t.Run("insufficient mp", func(t *testing.T) {
		state := &battleRuntimeState{
			Players: []battleUnitState{buildSource(24)},
			Order:   []string{"player:owner"},
		}

		actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[0], nil, "on_wave_start", 0, "")
		if len(actions) == 0 {
			t.Fatalf("expected clone actions")
		}
		if got := state.Players[0].MP; got != 24 {
			t.Fatalf("expected owner MP to stay 24, got %v", got)
		}
		if len(state.Players) != 2 {
			t.Fatalf("expected clone to be added, players=%d", len(state.Players))
		}
		if state.Players[1].SkipTurnOrder {
			t.Fatalf("clone should not receive synced-turn buff when MP is insufficient")
		}
		if got, want := state.Order, []string{"player:owner", state.Players[1].UnitID}; !slices.Equal(got, want) {
			t.Fatalf("runtime order mismatch: got=%#v want=%#v", got, want)
		}
	})
}

func TestPlayerCloneAfterSummonHookChainIsBounded(t *testing.T) {
	s := &Server{}
	source := battleUnitState{
		UnitID:       "player:owner",
		Side:         "player",
		OriginSide:   "player",
		BaseSide:     "player",
		Alive:        true,
		HP:           100,
		MaxHP:        100,
		MP:           100,
		MaxMP:        100,
		Position:     "vanguard",
		BasePosition: "vanguard",
		Stats:        map[string]float64{"hp": 100, "mp": 100, "speed": 10},
		BaseStats:    map[string]float64{"hp": 100, "mp": 100, "speed": 10},
		EquipmentHooks: []battleEquipmentHookRule{
			{
				ID:      "clone_on_wave_start",
				Trigger: "on_wave_start",
				Effects: []map[string]any{{"kind": "player_clone", "durationTurns": 5}},
			},
			{
				ID:      "clone_again_on_after_summon",
				Trigger: "on_after_summon",
				Effects: []map[string]any{{"kind": "player_clone", "durationTurns": 5}},
			},
		},
		Skills:     []battleSkillState{{ID: "basic_attack"}},
		SkillIDs:   []string{"basic_attack"},
		SkillSlots: []string{"basic_attack"},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{source},
		Order:   []string{"player:owner"},
	}

	actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), &state.Players[0], nil, "on_wave_start", 0, "")
	if len(actions) == 0 {
		t.Fatalf("expected player clone actions")
	}
	if got, want := len(state.Players), 1+1+battleMaxAfterSummonHookDepth; got != want {
		t.Fatalf("bounded summon chain mismatch: got=%d want=%d", got, want)
	}
}

func TestFinalizeTurnActionPayloadAppliesSummonCasterDeathCleanup(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:               "player:summon",
				Side:                 "player",
				Alive:                true,
				HP:                   50,
				MaxHP:                50,
				MP:                   10,
				MaxMP:                10,
				Stats:                map[string]float64{"speed": 8},
				Buffs:                []battleBuffState{{BuffID: battleSummonBindingBuffID, RemainTurns: 3, SourceID: "player:owner"}},
				SummonTemplateUnitID: "dummy",
				SummonSourceUnitID:   "player:owner",
				SummonOnCasterDeath:  battleSummonOnCasterDeathDespawn,
			},
		},
		Order: []string{"player:owner", "player:summon"},
	}
	payload := map[string]any{"actionLog": []map[string]any{}}

	s.finalizeTurnActionPayload(state, payload)

	if state.Players[0].Alive {
		t.Fatalf("summon should despawn when caster is dead")
	}
	logRows := payload["actionLog"].([]map[string]any)
	if len(logRows) != 1 || logRows[0]["kind"] != "summon_expire" {
		t.Fatalf("expected summon_expire action, got %#v", logRows)
	}
	if got, ok := payload["order"].([]string); !ok || !slices.Equal(got, []string{"player:owner", "player:summon"}) {
		t.Fatalf("order mismatch after finalize: %#v", payload["order"])
	}
}

func TestExecuteSyncedSummonTurnsResolvesUnitsByIDAfterReallocation(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: make([]battleUnitState, 0, 3),
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"speed": 1}},
		},
	}
	state.Players = append(state.Players,
		battleUnitState{
			UnitID:                 "player:owner",
			Side:                   "player",
			OriginSide:             "player",
			BaseSide:               "player",
			PerspectiveSide:        "player",
			Alive:                  true,
			HP:                     100,
			MaxHP:                  100,
			MP:                     100,
			MaxMP:                  100,
			Stats:                  map[string]float64{"hp": 100, "mp": 100, "speed": 10},
			BaseStats:              map[string]float64{"hp": 100, "mp": 100, "speed": 10},
			HasSyncedSummonTrigger: true,
		},
		battleUnitState{
			UnitID:             "player:summon:1",
			Side:               "player",
			OriginSide:         "player",
			BaseSide:           "player",
			PerspectiveSide:    "player",
			Alive:              true,
			HP:                 100,
			MaxHP:              100,
			MP:                 20,
			MaxMP:              100,
			Stats:              map[string]float64{"hp": 100, "mp": 100, "speed": 9},
			BaseStats:          map[string]float64{"hp": 100, "mp": 100, "speed": 9},
			SkipTurnOrder:      true,
			SummonSourceUnitID: "player:owner",
			EquipmentHooks: []battleEquipmentHookRule{
				{
					ID:      "clone_on_turn_start",
					Trigger: "on_turn_start",
					Effects: []map[string]any{{"kind": "player_clone", "durationTurns": 5}},
				},
			},
			Skills:     []battleSkillState{{ID: "basic_attack"}},
			SkillIDs:   []string{"basic_attack"},
			SkillSlots: []string{"basic_attack"},
		},
		battleUnitState{
			UnitID:             "player:summon:2",
			Side:               "player",
			OriginSide:         "player",
			BaseSide:           "player",
			PerspectiveSide:    "player",
			Alive:              true,
			HP:                 100,
			MaxHP:              100,
			MP:                 20,
			MaxMP:              100,
			Stats:              map[string]float64{"hp": 100, "mp": 100, "speed": 8},
			BaseStats:          map[string]float64{"hp": 100, "mp": 100, "speed": 8},
			SkipTurnOrder:      true,
			SummonSourceUnitID: "player:owner",
			Skills: []battleSkillState{
				{ID: "power_strike", MPCost: 0, CooldownTurns: 3, CooldownRemain: 2},
				{ID: "basic_attack", MPCost: 10},
			},
			SkillIDs:   []string{"power_strike", "basic_attack"},
			SkillSlots: []string{"power_strike", "basic_attack"},
		},
	)

	s.executeSyncedSummonTurns(state, time.Unix(0, 0), &state.Players[0], "player", 1)

	second := battleUnitByID(state, "player:summon:2")
	if second == nil {
		t.Fatalf("expected second summon to remain addressable")
	}
	if got := second.MP; got != 10 {
		t.Fatalf("expected second summon MP to update on live state after reallocation, got %v", got)
	}
	if got := second.Skills[0].CooldownRemain; got != 1 {
		t.Fatalf("expected cooldown to tick exactly once, got %d", got)
	}
}

func TestBattleNextActorConsumesPreparationOrderBeforeNormalTurn(t *testing.T) {
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100, Stats: map[string]float64{"speed": 10}},
			{UnitID: "player:2", Side: "player", Alive: true, HP: 100, Stats: map[string]float64{"speed": 9}},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, Stats: map[string]float64{"speed": 8}},
		},
		Order:            []string{"player:1", "player:2", "enemy:1"},
		TurnCursor:       0,
		PreparationOrder: []string{"player:2"},
	}

	actor, _, idx, isPreparationTurn := battleNextActor(state)
	if actor == nil || actor.UnitID != "player:2" {
		t.Fatalf("expected preparation actor player:2, got %#v", actor)
	}
	if !isPreparationTurn {
		t.Fatalf("expected preparation turn")
	}
	if idx != -1 {
		t.Fatalf("preparation turn index should be -1, got %d", idx)
	}
	if len(state.PreparationOrder) != 0 {
		t.Fatalf("preparation order should be consumed, got %#v", state.PreparationOrder)
	}

	nextActor, _, nextIdx, nextIsPreparationTurn := battleNextActor(state)
	if nextActor == nil || nextActor.UnitID != "player:1" {
		t.Fatalf("expected normal actor player:1 after preparation turn, got %#v", nextActor)
	}
	if nextIsPreparationTurn {
		t.Fatalf("expected normal turn after preparation order exhausted")
	}
	if nextIdx != 0 {
		t.Fatalf("expected normal turn index=0, got %d", nextIdx)
	}
}

func TestBattleNextPreparationOrderFromStateFallsBackToNormalTurnWhenPreparationSeedConsumed(t *testing.T) {
	state := battleRuntimeState{
		Order:                []string{"player:1", "enemy:1", "player:4", "player:5", "player:2", "player:3"},
		TurnCursor:           2,
		PreparationSeedOrder: []string{"player:2", "player:3"},
		ActedUnits:           []string{"player:1", "player:2", "player:3"},
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100},
			{UnitID: "player:2", Side: "player", Alive: true, HP: 80},
			{UnitID: "player:3", Side: "player", Alive: true, HP: 90},
			{UnitID: "player:4", Side: "player", Alive: true, HP: 70},
			{UnitID: "player:5", Side: "player", Alive: true, HP: 60},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100},
		},
	}

	order := battleNextPreparationOrderFromState(state)
	if len(order) != 2 || order[0] != "player:4" || order[1] != "player:5" {
		t.Fatalf("unexpected fallback preparation order: %#v", order)
	}
}

func TestBattleNextPreparationOrderFromStatePrefersRemainingPreparationUnits(t *testing.T) {
	state := battleRuntimeState{
		Order:                []string{"player:1", "enemy:1", "player:3", "player:5", "player:4", "player:2"},
		TurnCursor:           2,
		PreparationSeedOrder: []string{"player:2", "player:3", "player:4"},
		ActedUnits:           []string{"player:1", "player:2"},
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100},
			{UnitID: "player:2", Side: "player", Alive: true, HP: 80},
			{UnitID: "player:3", Side: "player", Alive: true, HP: 90},
			{UnitID: "player:4", Side: "player", Alive: true, HP: 70},
			{UnitID: "player:5", Side: "player", Alive: true, HP: 60},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100},
		},
	}

	order := battleNextPreparationOrderFromState(state)
	if len(order) != 2 || order[0] != "player:3" || order[1] != "player:4" {
		t.Fatalf("unexpected next preparation order: %#v", order)
	}
}

func TestSpawnWaveEnemiesUsesCustomLineupOnly(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 12345,
		Wave: 1,
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true},
			{UnitID: "player:2", Side: "player", Alive: true},
		},
	}

	worldCfg := battleWorldConfig{
		EnemyIDs:              []string{"enemy_pool"},
		NormalEnemyCountRange: battleCountRange{Min: 3, Max: 5},
		ExtraEnemyPerPlayer:   battleCountRange{Min: 1, Max: 2},
		CustomLineupConf: []battleCustomLineupConfig{
			{
				ID:                  "lineup_fixed",
				Percent:             1,
				IsSpecialWaveLineup: false,
				Lineup:              []string{"enemy_lineup_1", "enemy_lineup_2"},
			},
		},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"enemy_pool":     {ID: "enemy_pool", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
		"enemy_lineup_1": {ID: "enemy_lineup_1", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
		"enemy_lineup_2": {ID: "enemy_lineup_2", Position: "midguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
	}
	s.spawnWaveEnemies(context.Background(), state, worldCfg, enemyCatalog)
	if state.CurrentWaveCustomLineupID != "lineup_fixed" {
		t.Fatalf("custom lineup not selected, got %q", state.CurrentWaveCustomLineupID)
	}
	if len(state.Enemies) != 2 {
		t.Fatalf("expected 2 custom lineup enemies, got %d", len(state.Enemies))
	}
	if state.Enemies[0].EnemyID != "enemy_lineup_1" || state.Enemies[1].EnemyID != "enemy_lineup_2" {
		t.Fatalf("unexpected custom lineup enemies: %#v", state.Enemies)
	}
}

func TestSpawnWaveEnemiesSpecialCustomLineupDoesNotOverlaySpecialPool(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 999,
		Wave: 10,
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true},
		},
	}

	worldCfg := battleWorldConfig{
		EnemyIDs:         []string{"enemy_pool"},
		SpecialEnemyIDs:  []string{"boss_a", "boss_b"},
		SpecialWaveEvery: 10,
		NormalEnemyCountRange: battleCountRange{
			Min: 3,
			Max: 3,
		},
		CustomLineupConf: []battleCustomLineupConfig{
			{
				ID:                  "special_lineup",
				Percent:             1,
				IsSpecialWaveLineup: true,
				Lineup:              []string{"enemy_special_lineup"},
			},
		},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"enemy_pool":           {ID: "enemy_pool", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
		"boss_a":               {ID: "boss_a", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
		"boss_b":               {ID: "boss_b", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
		"enemy_special_lineup": {ID: "enemy_special_lineup", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
	}
	s.spawnWaveEnemies(context.Background(), state, worldCfg, enemyCatalog)
	if len(state.Enemies) != 1 {
		t.Fatalf("expected only custom special lineup enemy, got %d", len(state.Enemies))
	}
	if state.Enemies[0].EnemyID != "enemy_special_lineup" {
		t.Fatalf("unexpected enemy spawned: %s", state.Enemies[0].EnemyID)
	}
}

func TestSpawnWaveEnemiesCountsKittyAsPlayerForExtraEnemies(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 20260603,
		Wave: 1,
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, UserID: 1},
			{UnitID: "kitty:101", UnitType: "kitty", Side: "player", Alive: true, UserID: 1, KittyID: 101},
		},
	}
	worldCfg := battleWorldConfig{
		EnemyIDs:              []string{"enemy_pool"},
		NormalEnemyCountRange: battleCountRange{Min: 1, Max: 1},
		ExtraEnemyPerPlayer:   battleCountRange{Min: 2, Max: 2},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"enemy_pool": {ID: "enemy_pool", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
	}

	s.spawnWaveEnemies(context.Background(), state, worldCfg, enemyCatalog)
	if len(state.Enemies) != 3 {
		t.Fatalf("expected 3 enemies when kitty counts as extra player, got %d", len(state.Enemies))
	}
}

func TestResolveBattleBasicAttackProfileFallsBackToEffectFormula(t *testing.T) {
	s := &Server{}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID:        "test_shield",
			EquipType: "battle",
			Slot:      "right_hand",
			BasicAttackFormula: catalog.BasicAttackFormula{
				DamageType: "physical",
				Formula:    nil,
			},
			BasicAttackEffects: []map[string]any{
				{
					"kind": "damage",
					"targeting": map[string]any{
						"targetPositions": []any{"vanguard"},
						"secondarySeek":   "none",
						"targetScope":     "enemy",
						"selectRule":      "first",
						"targetCount": map[string]any{
							"base": 1,
						},
					},
					"damageType": "physical",
					"amountFormula": map[string]any{
						"baseAdd": 0,
						"terms": []any{
							map[string]any{"attrKey": "stamina", "coefficient": 0.5},
							map[string]any{"attrKey": "physics_defence", "coefficient": 0.5},
						},
					},
				},
			},
		},
	}
	equipped := []store.EquippedItem{
		{
			TemplateItemID: "test_shield",
			EquipType:      "battle",
			EquippedSlot:   "right_hand",
		},
	}
	profile := s.resolveBattleBasicAttackProfile(equipped)
	if profile == nil {
		t.Fatalf("expected basic attack profile")
	}
	if len(profile.Formula) != 2 {
		t.Fatalf("expected formula terms from basicAttackEffects, got %#v", profile.Formula)
	}
	if profile.Formula[0].AttrKey != "stamina" || profile.Formula[1].AttrKey != "physics_defence" {
		t.Fatalf("unexpected formula terms: %#v", profile.Formula)
	}
	if len(profile.DamageSpec.AmountFormula.Terms) != 2 {
		t.Fatalf("expected damageSpec amount terms from basicAttackEffects, got %#v", profile.DamageSpec.AmountFormula.Terms)
	}
}

func TestBuildBattleEnemyUnitUsesConfiguredEquipment(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{{"id": "basic_attack"}}
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{
		{
			ID:            "enemy_bow",
			EquipType:     "battle",
			Slot:          "right_hand",
			BaseAttrs:     map[string]float64{"dexterity": 3},
			MaxDurability: 100,
			BasicAttackEffects: []map[string]any{
				{
					"kind": "damage",
					"targeting": map[string]any{
						"targetPositions": []any{"rearguard"},
						"secondarySeek":   "rear_to_front",
						"targetScope":     "enemy",
						"selectRule":      "first",
						"targetCount":     map[string]any{"base": 1},
					},
					"damageType": "physical",
					"amountFormula": map[string]any{
						"baseAdd": 0,
						"terms":   []any{map[string]any{"attrKey": "dexterity", "coefficient": 1.2}},
					},
				},
			},
		},
	}
	cfg := battleEnemyConfig{
		ID:       "archer_enemy",
		NameKey:  "enemy.archer_enemy.name",
		Position: "vanguard",
		Type:     "normal",
		Level:    1,
		Stats:    map[string]float64{"hp": 20, "mp": 0, "dexterity": 5},
		SkillIDs: []string{"basic_attack"},
		Equipment: []battleEnemyEquipment{
			{
				EquippedItem: store.EquippedItem{
					InstanceID:     -1,
					TemplateItemID: "enemy_bow",
					EquipType:      "battle",
					EquippedSlot:   "right_hand",
					State:          "equipped",
				},
			},
		},
	}

	unit := s.buildBattleEnemyUnit(cfg, "enemy:1:archer_enemy", false, "basic_attack")
	if unit.BasicAttack == nil {
		t.Fatalf("expected enemy equipment to produce a basic attack profile")
	}
	if unit.BasicAttack.WeaponItemID != "enemy_bow" {
		t.Fatalf("basic attack source = %s, want enemy_bow", unit.BasicAttack.WeaponItemID)
	}
	if unit.BasicAttack.TargetDirection != "rearguard" {
		t.Fatalf("target direction = %s, want rearguard", unit.BasicAttack.TargetDirection)
	}
	if got := unit.Stats["dexterity"]; got != 8 {
		t.Fatalf("dexterity = %v, want enemy base 5 + bow base 3", got)
	}
}

func TestEvaluateBattleSkillFormulaSupportsWorkAttributes(t *testing.T) {
	actor := &battleUnitState{
		Stats: map[string]float64{
			"sewing": 7,
		},
	}
	formula := []battleAttackTerm{
		{AttrKey: "sewing", Coefficient: 1.25, PerSkillLevel: 0.5},
	}
	got := evaluateBattleSkillFormula(actor, formula, 3)
	if got != 15.75 {
		t.Fatalf("evaluateBattleSkillFormula with work attr got %v, want 15.75", got)
	}
}

func TestProcessBattleTurnSpawnTriggersBattleStartEquipmentHooks(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "taunt", "durationTurns": 2},
	}
	state := &battleRuntimeState{
		Seed: 1001,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"physics_damage": 20, "hp": 100, "mp": 0},
				EquipmentHooks: []battleEquipmentHookRule{
					{
						Trigger:  "on_battle_start",
						Priority: 0,
						Effects: []map[string]any{
							{"kind": "add_buff", "buffId": "taunt", "durationTurns": 2, "targetMode": "self"},
						},
					},
				},
			},
		},
	}
	worldCfg := battleWorldConfig{
		EnemyIDs: []string{"enemy_test"},
		NormalEnemyCountRange: battleCountRange{
			Min: 1,
			Max: 1,
		},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"enemy_test": {
			ID:       "enemy_test",
			Position: "vanguard",
			Stats:    map[string]float64{"hp": 20, "mp": 0},
			SkillIDs: []string{"basic_attack"},
		},
	}
	_, _, err := s.processBattleTurn(context.Background(), state, worldCfg, enemyCatalog, time.Unix(0, 0), nil)
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	if len(state.Players[0].Buffs) == 0 || state.Players[0].Buffs[0].BuffID != "taunt" {
		t.Fatalf("on_battle_start hook should apply taunt at each spawned battle start, buffs=%#v", state.Players[0].Buffs)
	}
}

func TestBattleStartEquipmentHooksCanOverrideSideAndPerspective(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "left_dagger_shadow_cross", "durationTurns": 2},
		{"id": "left_dagger_betrayal", "durationTurns": 2},
		{"id": "left_dagger_armor_break", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "left_dagger_shadow_cross_while_active",
			"buffId":  "left_dagger_shadow_cross",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectSideOverride, "side": "enemy"},
			},
		},
		{
			"id":      "left_dagger_betrayal_while_active",
			"buffId":  "left_dagger_betrayal",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectInvertRelationView},
			},
		},
		{
			"id":      "left_dagger_armor_break_while_active",
			"buffId":  "left_dagger_armor_break",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": "add_stat", "attr": "physics_percent_defense_penetration", "value": 0.5},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:          "player:1",
				UserID:          1,
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"hp": 100},
				EquipmentHooks: []battleEquipmentHookRule{
					{
						Trigger:  "on_battle_start",
						Priority: 0,
						Effects: []map[string]any{
							{"kind": "add_buff", "buffId": "left_dagger_shadow_cross", "durationTurns": 2, "targetMode": "self"},
							{"kind": "add_buff", "buffId": "left_dagger_betrayal", "durationTurns": 2, "targetMode": "self"},
						},
					},
				},
			},
			{
				UnitID:          "player:2",
				UserID:          2,
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"hp": 100},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:1",
				Side:            "enemy",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"hp": 100},
			},
		},
	}

	s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	if got := len(s.battleFindBuffRules("left_dagger_armor_break", "while_active")); got == 0 {
		t.Fatalf("expected left_dagger_armor_break while_active rules")
	}
	for _, rule := range s.battleFindBuffRules("left_dagger_armor_break", "while_active") {
		for _, effect := range rule.Effects {
			if battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"])) != "addstat" {
				t.Fatalf("unexpected armor break effect kind: %#v", effect["kind"])
			}
			if strings.TrimSpace(fmt.Sprintf("%v", effect["attr"])) != "physics_percent_defense_penetration" {
				t.Fatalf("unexpected armor break attr: %#v", effect["attr"])
			}
		}
	}

	if !battleHasBuff(&state.Players[0], "left_dagger_shadow_cross") || !battleHasBuff(&state.Players[0], "left_dagger_betrayal") {
		t.Fatalf("expected battle start hooks to attach both buffs, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Side != "enemy" {
		t.Fatalf("side=%s, want enemy after side override", state.Players[0].Side)
	}
	if state.Players[0].PerspectiveSide != "player" {
		t.Fatalf("perspectiveSide=%s, want player after betrayal", state.Players[0].PerspectiveSide)
	}
	if state.Players[0].OriginSide != "player" {
		t.Fatalf("originSide=%s, want player", state.Players[0].OriginSide)
	}
	if relation := s.battleTargetRelationForActor(state, &state.Players[0], &state.Players[1]); relation != "ally" {
		t.Fatalf("betrayed actor relation to ally=%s, want ally", relation)
	}
	if relation := s.battleTargetRelationForActor(state, &state.Players[0], &state.Enemies[0]); relation != "enemy" {
		t.Fatalf("betrayed actor relation to enemy=%s, want enemy", relation)
	}
	if !battlePlayerMembersAlive(state) {
		t.Fatalf("player members should remain alive by origin-side ownership")
	}
	if !battleTeamAlive(state, "enemy") {
		t.Fatalf("enemy team should remain alive by origin side")
	}
}

func TestBattleEndEquipmentHooksRemoveTemporarySideBuffs(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "left_dagger_shadow_cross", "durationTurns": 2},
		{"id": "left_dagger_betrayal", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "left_dagger_shadow_cross_while_active",
			"buffId":  "left_dagger_shadow_cross",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectSideOverride, "side": "enemy"},
				{"kind": battleEffectPositionOverride, "position": "rearguard"},
			},
		},
		{
			"id":      "left_dagger_betrayal_while_active",
			"buffId":  "left_dagger_betrayal",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectInvertRelationView},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:          "player:1",
				UserID:          1,
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Position:        "vanguard",
				BasePosition:    "vanguard",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Stats:           map[string]float64{"hp": 100},
				EquipmentHooks: []battleEquipmentHookRule{
					{
						Trigger: "on_battle_start",
						Effects: []map[string]any{
							{"kind": "add_buff", "buffId": "left_dagger_shadow_cross", "durationTurns": 2, "targetMode": "self"},
							{"kind": "add_buff", "buffId": "left_dagger_betrayal", "durationTurns": 2, "targetMode": "self"},
							{"kind": "add_buff", "buffId": "left_dagger_armor_break", "durationTurns": 2, "targetMode": "self"},
						},
					},
					{
						Trigger: "on_battle_end",
						Effects: []map[string]any{
							{"kind": "remove_buff", "buffId": "left_dagger_shadow_cross", "targetMode": "self"},
							{"kind": "remove_buff", "buffId": "left_dagger_betrayal", "targetMode": "self"},
							{"kind": "remove_buff", "buffId": "left_dagger_armor_break", "targetMode": "self"},
						},
					},
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:1",
				Side:            "enemy",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Alive:           false,
				HP:              0,
				MaxHP:           100,
				Stats:           map[string]float64{"hp": 100},
			},
		},
	}

	s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))
	if !battleHasBuff(&state.Players[0], "left_dagger_armor_break") {
		t.Fatalf("battle start should attach armor break buff, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Side != "enemy" || state.Players[0].Position != "rearguard" {
		t.Fatalf("battle start should move to enemy rearguard, got side=%s position=%s", state.Players[0].Side, state.Players[0].Position)
	}
	status, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(1, 0), map[int64]store.BattleWaveReward{})
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	if status != store.BattleRunStatusEnded {
		t.Fatalf("status=%s, want %s", status, store.BattleRunStatusEnded)
	}
	if battleHasBuff(&state.Players[0], "left_dagger_shadow_cross") || battleHasBuff(&state.Players[0], "left_dagger_betrayal") || battleHasBuff(&state.Players[0], "left_dagger_armor_break") {
		t.Fatalf("battle end should remove temporary buffs, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Side != "player" || state.Players[0].PerspectiveSide != "player" {
		t.Fatalf("battle end should restore side/perspective, got side=%s perspective=%s", state.Players[0].Side, state.Players[0].PerspectiveSide)
	}
}

func TestBattleApplyBuffTargetingOverridesScopesByBasicAndSkillIDs(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "targeting_override", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "targeting_override_while_active",
			"buffId":  "targeting_override",
			"trigger": "while_active",
			"effects": []map[string]any{
				{
					"kind":               battleEffectOverrideTargeting,
					"applyToBasicAttack": false,
					"applyToSkills":      true,
					"skillIds":           []any{"skill_hit_mid"},
					"targeting": map[string]any{
						"targetPositions": []any{"midguard"},
						"secondarySeek":   "none",
					},
				},
			},
		},
	}
	actor := &battleUnitState{
		UnitID: "player:1",
		Buffs:  []battleBuffState{{BuffID: "targeting_override", RemainTurns: 2}},
	}
	baseTargeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount: battleSkillTargetCount{
			Base: 1,
		},
	}
	baseEffect := battleSkillEffect{
		Kind:      "damage",
		Targeting: baseTargeting,
	}
	basicSkill := battleSkillState{ID: "basic_attack", Targeting: baseTargeting, Effects: []battleSkillEffect{baseEffect}}
	basicSkill, basicApplied := s.battleApplyBuffTargetingOverrides(actor, basicSkill, true)
	if basicApplied {
		t.Fatalf("basic attack should not be overridden when applyToBasicAttack=false")
	}
	if firstNonEmptyTargetPosition(basicSkill.Targeting.TargetPositions) != "vanguard" {
		t.Fatalf("basic attack target=%s, want vanguard", firstNonEmptyTargetPosition(basicSkill.Targeting.TargetPositions))
	}

	matchedSkill := battleSkillState{ID: "skill_hit_mid", Targeting: baseTargeting, Effects: []battleSkillEffect{baseEffect}}
	matchedSkill, matchedApplied := s.battleApplyBuffTargetingOverrides(actor, matchedSkill, false)
	if !matchedApplied {
		t.Fatalf("skill_hit_mid should be overridden by skillIds rule")
	}
	if firstNonEmptyTargetPosition(matchedSkill.Targeting.TargetPositions) != "midguard" {
		t.Fatalf("matched skill target=%s, want midguard", firstNonEmptyTargetPosition(matchedSkill.Targeting.TargetPositions))
	}

	otherSkill := battleSkillState{ID: "skill_other", Targeting: baseTargeting, Effects: []battleSkillEffect{baseEffect}}
	otherSkill, otherApplied := s.battleApplyBuffTargetingOverrides(actor, otherSkill, false)
	if otherApplied {
		t.Fatalf("skill_other should not be overridden when not listed in skillIds")
	}
	if firstNonEmptyTargetPosition(otherSkill.Targeting.TargetPositions) != "vanguard" {
		t.Fatalf("other skill target=%s, want vanguard", firstNonEmptyTargetPosition(otherSkill.Targeting.TargetPositions))
	}
}

func TestBattleApplyBuffTargetingOverridesDoesNotMutateSourceEffects(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "targeting_override", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "targeting_override_while_active",
			"buffId":  "targeting_override",
			"trigger": "while_active",
			"effects": []map[string]any{
				{
					"kind":               battleEffectOverrideTargeting,
					"applyToBasicAttack": true,
					"applyToSkills":      true,
					"skillIds":           []any{},
					"targeting": map[string]any{
						"targetPositions": []any{"midguard"},
						"secondarySeek":   "none",
					},
				},
			},
		},
	}
	actor := &battleUnitState{
		UnitID: "player:1",
		Buffs:  []battleBuffState{{BuffID: "targeting_override", RemainTurns: 2}},
	}
	originalTargeting := battleSkillTargeting{
		TargetPositions: []string{"vanguard"},
		SecondarySeek:   "front_to_rear",
		TargetScope:     "enemy",
		SelectRule:      "first",
		TargetCount: battleSkillTargetCount{
			Base: 1,
		},
	}
	sourceEffects := []battleSkillEffect{
		{
			Kind:      "damage",
			Targeting: originalTargeting,
		},
	}
	skill := battleSkillState{
		ID:        "basic_attack",
		Targeting: originalTargeting,
		Effects:   sourceEffects,
	}

	overridden, applied := s.battleApplyBuffTargetingOverrides(actor, skill, true)
	if !applied {
		t.Fatalf("override should apply to basic attack")
	}
	if firstNonEmptyTargetPosition(overridden.Targeting.TargetPositions) != "midguard" {
		t.Fatalf("overridden targeting=%s, want midguard", firstNonEmptyTargetPosition(overridden.Targeting.TargetPositions))
	}
	if firstNonEmptyTargetPosition(sourceEffects[0].Targeting.TargetPositions) != "vanguard" {
		t.Fatalf("source effect targeting was mutated: %#v", sourceEffects[0].Targeting.TargetPositions)
	}
	if sourceEffects[0].Targeting.SecondarySeek != "front_to_rear" {
		t.Fatalf("source effect secondary seek was mutated: %s", sourceEffects[0].Targeting.SecondarySeek)
	}
}

func TestProcessBattleTurnNoWeaponWithTargetingOverrideStillTicksBuffDuration(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
	}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "targeting_override", "durationTurns": 3},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "targeting_override_while_active",
			"buffId":  "targeting_override",
			"trigger": "while_active",
			"effects": []map[string]any{
				{
					"kind":               battleEffectOverrideTargeting,
					"applyToBasicAttack": true,
					"applyToSkills":      true,
					"skillIds":           []any{},
					"targeting": map[string]any{
						"targetPositions": []any{"midguard"},
						"secondarySeek":   "none",
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed:       1,
		Wave:       1,
		Order:      []string{"player:1", "enemy:1"},
		TurnCursor: 0,
		Players: []battleUnitState{
			{
				UnitID: "player:1",
				Side:   "player",
				UserID: 1,
				Alive:  true,
				HP:     100,
				MaxHP:  100,
				MP:     0,
				MaxMP:  0,
				Stats: map[string]float64{
					"hp":             100,
					"mp":             0,
					"physics_damage": 25,
					"speed":          10,
				},
				Buffs: []battleBuffState{
					{BuffID: "targeting_override", RemainTurns: 3},
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:1",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":    100,
					"speed": 1,
				},
			},
		},
	}

	status, advanced, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.Unix(0, 0), map[int64]store.BattleWaveReward{})
	if err != nil {
		t.Fatalf("processBattleTurn error: %v", err)
	}
	if status != "" {
		t.Fatalf("status=%s, want empty ongoing status", status)
	}
	if advanced {
		t.Fatalf("ongoing turn should return advanced=false")
	}
	if len(state.Players[0].Buffs) != 1 {
		t.Fatalf("expected targeting override buff to still exist after one tick, buffs=%#v", state.Players[0].Buffs)
	}
	if state.Players[0].Buffs[0].RemainTurns != 2 {
		t.Fatalf("buff remain turns=%d, want 2", state.Players[0].Buffs[0].RemainTurns)
	}
}

func TestExecuteBattleSkillBuffTargetingOverrideLocksToMidguard(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "targeting_override", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "targeting_override_while_active",
			"buffId":  "targeting_override",
			"trigger": "while_active",
			"effects": []map[string]any{
				{
					"kind":               battleEffectOverrideTargeting,
					"applyToBasicAttack": true,
					"applyToSkills":      true,
					"skillIds":           []any{},
					"targeting": map[string]any{
						"targetPositions": []any{"midguard"},
						"secondarySeek":   "none",
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 100,
		Players: []battleUnitState{
			{
				UnitID:   "player:1",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats: map[string]float64{
					"hp":             100,
					"mp":             0,
					"physics_damage": 40,
				},
				Buffs: []battleBuffState{{BuffID: "targeting_override", RemainTurns: 2}},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:vanguard",
				Side:     "enemy",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100, "mp": 0},
			},
			{
				UnitID:   "enemy:midguard",
				Side:     "enemy",
				Position: "midguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				Stats:    map[string]float64{"hp": 100, "mp": 0},
			},
		},
	}
	actor := &state.Players[0]
	skill := battleSkillState{
		ID:    "basic_attack",
		Level: 1,
		Targeting: battleSkillTargeting{
			TargetPositions: []string{"vanguard"},
			SecondarySeek:   "front_to_rear",
			TargetScope:     "enemy",
			SelectRule:      "first",
			TargetCount: battleSkillTargetCount{
				Base: 1,
			},
		},
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "first",
					TargetCount: battleSkillTargetCount{
						Base: 1,
					},
				},
				DamageSpec: battleDamageSpec{
					DamageType: "physical",
					AmountFormula: battleSkillAmountFormula{
						Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
					},
				},
			},
		},
	}
	result := s.executeBattleSkill(state, time.Unix(0, 0), actor, skill, "turn", nil)
	if len(result.Targets) == 0 {
		t.Fatalf("expected at least one target after executing skill")
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", result.Targets[0]["targetId"])); got != "enemy:midguard" {
		t.Fatalf("override target=%s, want enemy:midguard", got)
	}
}

func TestProcessBattleTurnTauntAffixPrioritizesSingleTarget(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "taunt", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "taunt_prefer_target",
			"buffId":  "taunt",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "prefer_as_enemy_target"}},
		},
	}
	basicSkill := s.battleSkillByID(s.battleDefaultBasicSkillID())
	state := &battleRuntimeState{
		Seed:  2026,
		Wave:  1,
		Order: []string{"enemy:basic", "player:plain", "player:taunt"},
		Players: []battleUnitState{
			{
				UnitID:   "player:plain",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":    100,
					"mp":    0,
					"speed": 1,
				},
			},
			{
				UnitID:   "player:taunt",
				Side:     "player",
				Position: "rearguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":    100,
					"mp":    0,
					"speed": 1,
				},
				EquipmentHooks: []battleEquipmentHookRule{
					{
						Trigger:  "on_battle_start",
						Priority: 0,
						Effects: []map[string]any{
							{"kind": "add_buff", "buffId": "taunt", "durationTurns": 2, "targetMode": "self"},
						},
					},
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:basic",
				Side:     "enemy",
				EnemyID:  "enemy_basic",
				Position: "vanguard",
				Alive:    true,
				HP:       100,
				MaxHP:    100,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":             100,
					"mp":             0,
					"speed":          10,
					"physics_damage": 40,
				},
				SkillIDs: []string{basicSkill.ID},
				Skills:   []battleSkillState{basicSkill},
			},
		},
	}
	s.battleTriggerBattleStartEquipmentHooks(state, time.Unix(0, 0))

	actor := &state.Enemies[0]
	s.executeBattleSkill(state, time.Unix(0, 0), actor, actor.Skills[0], "active", nil)
	if len(state.Players[1].Buffs) == 0 || state.Players[1].Buffs[0].BuffID != "taunt" {
		t.Fatalf("taunt should be granted on battle start, buffs=%#v", state.Players[1].Buffs)
	}
	if state.Players[1].HP >= 100 {
		t.Fatalf("taunt target should be prioritized for single-target attack, tauntHP=%v plainHP=%v enemyHP=%v", state.Players[1].HP, state.Players[0].HP, state.Enemies[0].HP)
	}
	if state.Players[0].HP < 100 {
		t.Fatalf("non-taunt vanguard should not be picked when taunt target exists, hp=%v", state.Players[0].HP)
	}
}

func TestProcessBattleTurnCamouflageAffixRemapsEnemySingleTarget(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "camouflage", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "camouflage_remap",
			"buffId":  "camouflage",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "camouflage_targeting_remap"}},
		},
	}
	basicSkill := s.battleSkillByID(s.battleDefaultBasicSkillID())
	state := &battleRuntimeState{
		Seed:  2027,
		Wave:  1,
		Order: []string{"enemy:fast", "enemy:follow", "player:camouflage", "player:plain"},
		Players: []battleUnitState{
			{
				UnitID:   "player:camouflage",
				Side:     "player",
				Position: "vanguard",
				Alive:    true,
				HP:       120,
				MaxHP:    120,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":    120,
					"mp":    0,
					"speed": 1,
				},
				EquipmentHooks: []battleEquipmentHookRule{
					{
						Trigger:  "on_after_take_damage",
						Priority: 0,
						Effects: []map[string]any{
							{"kind": "add_buff", "buffId": "camouflage", "durationTurns": 2, "targetMode": "self"},
						},
					},
				},
			},
			{
				UnitID:   "player:plain",
				Side:     "player",
				Position: "rearguard",
				Alive:    true,
				HP:       120,
				MaxHP:    120,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":    120,
					"mp":    0,
					"speed": 1,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:   "enemy:fast",
				Side:     "enemy",
				EnemyID:  "enemy_fast",
				Position: "vanguard",
				Alive:    true,
				HP:       120,
				MaxHP:    120,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":             120,
					"mp":             0,
					"speed":          12,
					"physics_damage": 25,
				},
				SkillIDs: []string{basicSkill.ID},
				Skills:   []battleSkillState{basicSkill},
			},
			{
				UnitID:   "enemy:follow",
				Side:     "enemy",
				EnemyID:  "enemy_follow",
				Position: "midguard",
				Alive:    true,
				HP:       120,
				MaxHP:    120,
				MP:       0,
				MaxMP:    0,
				Stats: map[string]float64{
					"hp":             120,
					"mp":             0,
					"speed":          11,
					"physics_damage": 25,
				},
				SkillIDs: []string{basicSkill.ID},
				Skills:   []battleSkillState{basicSkill},
			},
		},
	}
	firstEnemy := &state.Enemies[0]
	s.executeBattleSkill(state, time.Unix(0, 0), firstEnemy, firstEnemy.Skills[0], "active", nil)
	if len(state.Players[0].Buffs) == 0 || state.Players[0].Buffs[0].BuffID != "camouflage" {
		t.Fatalf("camouflage should be applied after taking damage, buffs=%#v camoHP=%v plainHP=%v", state.Players[0].Buffs, state.Players[0].HP, state.Players[1].HP)
	}
	player0HPAfterFirst := state.Players[0].HP
	player1HPAfterFirst := state.Players[1].HP

	secondEnemy := &state.Enemies[1]
	s.executeBattleSkill(state, time.Unix(1, 0), secondEnemy, secondEnemy.Skills[0], "active", nil)
	if state.Players[1].HP >= player1HPAfterFirst {
		t.Fatalf("non-camouflage target should be selected when available, hp=%v", state.Players[1].HP)
	}
	if state.Players[0].HP < player0HPAfterFirst {
		t.Fatalf("camouflage target should be avoided while another enemy candidate exists, hp=%v", state.Players[0].HP)
	}
}

func TestResolveBattleWaveSettlementAddsCustomLineupDrops(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Enemies = []map[string]any{
		{
			"id":       "enemy_a",
			"name":     "Enemy A",
			"position": "vanguard",
			"exp":      0,
			"drops":    map[string]any{},
			"skillIds": []any{"basic_attack"},
		},
	}
	state := battleRuntimeState{
		Seed: 77,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:         "player:1",
				Side:           "player",
				UserID:         1,
				Alive:          true,
				RewardEligible: true,
				Stats:          map[string]float64{"luck": 0},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:  "enemy:1",
				Side:    "enemy",
				EnemyID: "enemy_a",
				Alive:   false,
			},
		},
		CurrentWaveCustomLineupID: "lineup_drop",
	}
	worldCfg := battleWorldConfig{
		CustomLineupConf: []battleCustomLineupConfig{
			{
				ID:      "lineup_drop",
				Percent: 1,
				Lineup:  []string{"enemy_a"},
				Drops: battleCustomLineupDrops{
					Items: map[string]battleDropRule{
						"bonus_token": {Min: 1, Max: 1, Percent: 1},
					},
				},
			},
		},
	}
	settlement := s.resolveBattleWaveSettlement(state, worldCfg, time.Now())
	reward, ok := settlement.Rewards[1]
	if !ok {
		t.Fatalf("missing reward for user 1")
	}
	if reward.ItemRewards["bonus_token"] != 1 {
		t.Fatalf("expected custom lineup drop bonus_token=1, got %#v", reward.ItemRewards)
	}
}

func TestBattleSettlementDropPercentUsesFullDisplayedBonus(t *testing.T) {
	tests := []struct {
		name                    string
		basePercent             float64
		dropBonusRate           float64
		difficultyDropBonusRate float64
		luck                    float64
		dropLuckFactor          float64
		want                    float64
	}{
		{
			name:          "below five hundred percent bonus is unchanged",
			basePercent:   0.01,
			dropBonusRate: 4,
			want:          0.05,
		},
		{
			name:          "five hundred percent bonus is unchanged at threshold",
			basePercent:   0.01,
			dropBonusRate: 5,
			want:          0.06,
		},
		{
			name:          "full one thousand percent bonus",
			basePercent:   0.01,
			dropBonusRate: 10,
			want:          0.11,
		},
		{
			name:          "full two thousand percent bonus",
			basePercent:   0.01,
			dropBonusRate: 20,
			want:          0.21,
		},
		{
			name:          "very large bonus is not hard capped",
			basePercent:   0.000000001,
			dropBonusRate: 1000000,
			want:          0.001000001,
		},
		{
			name:                    "difficulty bonus adds after player multiplier",
			basePercent:             0.01,
			dropBonusRate:           0.5,
			difficultyDropBonusRate: 0.2,
			luck:                    10,
			dropLuckFactor:          0.01,
			want:                    0.0185,
		},
		{
			name:                    "difficulty bonus adds after the full player bonus",
			basePercent:             0.01,
			dropBonusRate:           10,
			difficultyDropBonusRate: 0.6,
			want:                    0.116,
		},
		{
			name:          "probability remains capped at one",
			basePercent:   0.5,
			dropBonusRate: 100,
			want:          1,
		},
		{
			name:          "negative effective multiplier cannot create drops",
			basePercent:   0.5,
			dropBonusRate: -2,
			want:          0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := battleSettlementDropPercent(tt.basePercent, tt.dropBonusRate, tt.difficultyDropBonusRate, tt.luck, tt.dropLuckFactor)
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("battleSettlementDropPercent() = %.12f, want %.12f", got, tt.want)
			}
		})
	}
}

func TestResolveBattleWaveSettlementGrantsFullEnemyAndCustomLineupDrops(t *testing.T) {
	s := &Server{}
	settings, err := config.LoadGlobalSettings("../../../packages/game-config/source/global-config.json")
	if err != nil {
		t.Fatal(err)
	}
	s.deps.Config.GlobalSettings = settings
	s.deps.GameData.Items = []catalog.Item{{ID: "enemy_loot", EconomyBaseValue: 1000}}
	s.deps.GameData.Enemies = []map[string]any{
		{
			"id":       "enemy_a",
			"name":     "Enemy A",
			"position": "vanguard",
			"exp":      0,
			"drops": map[string]any{
				"enemy_loot": map[string]any{
					"range":   map[string]any{"min": 4, "max": 4},
					"percent": 0.05,
				},
			},
			"skillIds": []any{"basic_attack"},
		},
	}
	state := battleRuntimeState{
		Seed: 77,
		Wave: 1,
		Players: []battleUnitState{
			{
				UnitID:         "player:1",
				Side:           "player",
				UserID:         1,
				Alive:          true,
				RewardEligible: true,
				Stats: map[string]float64{
					"battle_drop_rate_bonus": 20,
					"luck":                   0,
				},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:  "enemy:1",
				Side:    "enemy",
				EnemyID: "enemy_a",
				Alive:   false,
			},
		},
		CurrentWaveCustomLineupID: "lineup_drop",
	}
	worldCfg := battleWorldConfig{
		CustomLineupConf: []battleCustomLineupConfig{
			{
				ID:      "lineup_drop",
				Percent: 1,
				Lineup:  []string{"enemy_a"},
				Drops: battleCustomLineupDrops{
					Items: map[string]battleDropRule{
						"lineup_loot": {Min: 4, Max: 4, Percent: 0.05},
					},
				},
			},
		},
	}

	settlement := s.resolveBattleWaveSettlement(state, worldCfg, time.Now())
	reward, ok := settlement.Rewards[1]
	if !ok {
		t.Fatalf("missing reward for user 1")
	}
	if reward.ItemRewards["enemy_loot"] != 4 || reward.ItemRewards["lineup_loot"] != 4 {
		t.Fatalf("expected full configured rewards with a 2000%% bonus, got %#v", reward.ItemRewards)
	}
}

func TestResolveBattleWaveSettlementGrantsFullKittyBattleAttrExp(t *testing.T) {
	s := &Server{}
	s.deps.Config = config.Config{
		GlobalSettings:       config.DefaultGlobalSettings(),
		SharedGlobalSettings: config.DefaultSharedGlobalSettings(),
	}
	s.deps.Config.SharedGlobalSettings.Kitty.BattleAttrExpMultiplier = 1
	s.deps.GameData.Enemies = []map[string]any{
		{
			"id":       "enemy_a",
			"name":     "Enemy A",
			"position": "vanguard",
			"exp":      120,
			"drops":    map[string]any{},
			"skillIds": []any{"basic_attack"},
		},
	}
	state := battleRuntimeState{
		Seed:      78,
		Wave:      1,
		StartedAt: time.Now().UnixMilli(),
		Players: []battleUnitState{
			{
				UnitID:         "player:1",
				UnitType:       "player",
				Side:           "player",
				UserID:         1,
				Alive:          true,
				RewardEligible: true,
				Stats:          map[string]float64{"luck": 0},
			},
			{
				UnitID:         "kitty:101",
				UnitType:       "kitty",
				Side:           "player",
				UserID:         1,
				KittyID:        101,
				Alive:          true,
				RewardEligible: true,
				Stats:          map[string]float64{},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:  "enemy:1",
				Side:    "enemy",
				EnemyID: "enemy_a",
				Alive:   false,
			},
		},
	}

	settlement := s.resolveBattleWaveSettlement(state, battleWorldConfig{}, time.Now())
	reward, ok := settlement.Rewards[1]
	if !ok {
		t.Fatalf("missing reward for user 1")
	}
	if got := reward.AttrExp["battle"]; got != 120 {
		t.Fatalf("player battle exp=%v, want 120", got)
	}
	if got := reward.KittyAttrExp[101]["battle"]; got != 120 {
		t.Fatalf("kitty battle exp=%v, want 120", got)
	}
}

func TestBattleSelectSkillDoesNotBypassBlockedBasicSkillRule(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100},
		},
	}
	unit := battleUnitState{
		UnitID: "player:1",
		Side:   "player",
		Alive:  true,
		HP:     100,
		MP:     100,
		Skills: []battleSkillState{
			{ID: "basic_attack", Name: "basic_attack"},
		},
		SkillRules: map[string]battleSkillRuleState{
			"basic_attack": {
				Enabled: true,
				Groups: []rules.ConditionGroup{
					{
						Conditions: []rules.Condition{
							{
								CompareType:         "value",
								DataType:            "enemy_count",
								Target:              "enemies",
								EnemyCountPosition:  "all",
								ValueCompareAction:  ">",
								ValueConditionValue: 1,
							},
						},
					},
				},
			},
		},
	}
	selected, idx := s.battleSelectSkill(state, unit, "player")
	if idx != -1 {
		t.Fatalf("expected no skill index when all rules block, got %d", idx)
	}
	if selected.ID != "" {
		t.Fatalf("expected no selected skill when basic skill rule blocks cast, got %q", selected.ID)
	}
}

func TestBattleSelectSkillFallbacksToBasicSkillWhenMissingFromSnapshot(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
	}
	selected, idx := s.battleSelectSkill(&battleRuntimeState{}, battleUnitState{
		UnitID: "player:1",
		Side:   "player",
		Alive:  true,
		HP:     100,
		MP:     100,
		Skills: []battleSkillState{},
	}, "player")
	if idx != -1 {
		t.Fatalf("expected fallback index to be -1, got %d", idx)
	}
	if selected.ID != "basic_attack" {
		t.Fatalf("expected basic fallback skill, got %q", selected.ID)
	}
}

func TestBattleSelectSkillBlocksArcaneMissileWhenEnemyCountRuleNotMet(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
		{"id": "arcane_missile"},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100},
		},
	}
	unit := battleUnitState{
		UnitID: "player:1",
		Side:   "player",
		Alive:  true,
		HP:     100,
		MP:     100,
		Skills: []battleSkillState{
			{ID: "arcane_missile", Name: "arcane_missile"},
			{ID: "basic_attack", Name: "basic_attack"},
		},
		SkillRules: map[string]battleSkillRuleState{
			"arcane_missile": {
				Enabled: true,
				Groups: []rules.ConditionGroup{
					{
						Conditions: []rules.Condition{
							{
								CompareType:         "value",
								DataType:            "enemy_count",
								Target:              "enemies",
								EnemyCountPosition:  "all",
								ValueCompareAction:  ">",
								ValueConditionValue: 1,
							},
						},
					},
				},
			},
		},
	}
	selected, idx := s.battleSelectSkill(state, unit, "player")
	if selected.ID == "arcane_missile" {
		t.Fatalf("arcane_missile should be blocked when enemy_count=1 and condition is >1")
	}
	if selected.ID != "basic_attack" {
		t.Fatalf("expected fallback to basic_attack after blocking arcane_missile, got %q", selected.ID)
	}
	if idx != 1 {
		t.Fatalf("expected selected index 1 (basic_attack), got %d", idx)
	}
}

func TestBattleSelectSkillTreatsStoredDisabledRuleAsEnabled(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
		{"id": "power_strike"},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{{UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100}},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100},
			{UnitID: "enemy:2", Side: "enemy", Alive: true, HP: 100, MaxHP: 100},
			{UnitID: "enemy:3", Side: "enemy", Alive: true, HP: 100, MaxHP: 100},
		},
	}
	unit := battleUnitState{
		UnitID: "player:1",
		Side:   "player",
		Alive:  true,
		HP:     100,
		MP:     100,
		Skills: []battleSkillState{
			{ID: "power_strike", Name: "power_strike"},
			{ID: "basic_attack", Name: "basic_attack"},
		},
		SkillRules: map[string]battleSkillRuleState{
			"power_strike": {
				Enabled: false,
				Groups: []rules.ConditionGroup{
					{
						Conditions: []rules.Condition{
							{
								CompareType:         "value",
								DataType:            "enemy_count",
								Target:              "enemies",
								EnemyCountPosition:  "all",
								ValueCompareAction:  "<=",
								ValueConditionValue: 2,
							},
						},
					},
				},
			},
		},
	}
	selected, _ := s.battleSelectSkill(state, unit, "player")
	if selected.ID == "power_strike" {
		t.Fatalf("power_strike should be blocked by rule groups even when legacy Enabled=false")
	}
	if selected.ID != "basic_attack" {
		t.Fatalf("expected fallback to basic_attack, got %q", selected.ID)
	}
}
