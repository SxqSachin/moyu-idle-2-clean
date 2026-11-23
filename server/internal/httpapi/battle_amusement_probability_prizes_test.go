package httpapi

import (
	"context"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
)

func newAmusementProbabilityPrizeTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "jackpot_payout_ticket", "durationTurns": 2, "maxCharges": 1, "stackPolicy": "refresh_duration"},
		{"id": "sugar_shell", "durationTurns": 2, "maxCharges": 1, "stackPolicy": "refresh_duration"},
		{"id": "last_parade_light", "durationTurns": 2, "maxCharges": 1, "stackPolicy": "refresh_duration"},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id": "jackpot_payout_ticket_probability", "buffId": "jackpot_payout_ticket",
			"trigger": "on_before_probability_check", "priority": -1000,
			"eventFilter": map[string]any{"probabilityKinds": []any{"status_apply"}},
			"conditions":  []map[string]any{{"type": "self_charges_remaining_gte", "value": 1}},
			"effects":     []map[string]any{{"kind": "set_probability", "value": 1}},
		},
		{
			"id": "jackpot_payout_ticket_redeem_success", "buffId": "jackpot_payout_ticket",
			"trigger":     "on_probability_success",
			"eventFilter": map[string]any{"probabilityKinds": []any{"status_apply"}},
			"conditions":  []map[string]any{{"type": "self_charges_remaining_gte", "value": 1}},
			"effects":     []map[string]any{{"kind": "consume_charge"}},
		},
		{
			"id": "jackpot_payout_ticket_redeem_failure", "buffId": "jackpot_payout_ticket",
			"trigger":     "on_probability_failure",
			"eventFilter": map[string]any{"probabilityKinds": []any{"status_apply"}},
			"conditions":  []map[string]any{{"type": "self_charges_remaining_gte", "value": 1}},
			"effects":     []map[string]any{{"kind": "consume_charge"}},
		},
		{
			"id": "sugar_shell_break", "buffId": "sugar_shell",
			"trigger": "on_before_take_damage", "priority": 900,
			"conditions": []map[string]any{
				{"type": "incoming_damage_gte", "value": 1},
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{
				{"kind": "set_damage_multiplier", "value": 0},
				{"kind": "consume_charge"},
			},
		},
		{
			"id": "last_parade_light_catch_lethal", "buffId": "last_parade_light",
			"trigger": "on_before_take_damage", "priority": 950,
			"conditions": []map[string]any{
				{"type": "incoming_damage_would_be_lethal"},
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{
				{"kind": "set_damage_multiplier", "value": 0},
				{"kind": "consume_charge"},
			},
		},
	}
	return s
}

func amusementProbabilityPrizeUnit(id, side string) battleUnitState {
	unit := probabilityTestUnit(id)
	unit.Side = side
	unit.OriginSide = side
	unit.BaseSide = side
	unit.PerspectiveSide = side
	unit.HP = 100
	unit.MaxHP = 100
	unit.Stats["hp"] = 100
	unit.BaseStats["hp"] = 100
	return unit
}

func TestAmusementJackpotTicketGuaranteesAndConsumesNextStatusCheck(t *testing.T) {
	s := newAmusementProbabilityPrizeTestServer()
	actor := amusementProbabilityPrizeUnit("player:token", "player")
	actor.Buffs = []battleBuffState{{BuffID: "jackpot_payout_ticket", RemainTurns: 2}}
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID:       "jackpot_biased_token_status_tax",
		Trigger:  "on_before_probability_check",
		Priority: 20,
		EventFilter: map[string]any{
			"probabilityKinds": []any{"status_apply"},
		},
		Effects: []map[string]any{{"kind": "add_probability", "value": -0.08}},
	}}
	target := amusementProbabilityPrizeUnit("enemy:target", "enemy")
	state := &battleRuntimeState{Seed: 17, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}

	check, result := s.battleResolveStatusApplyProbability(state, &state.Players[0], &state.Enemies[0], 0.1, "skill:candy")
	if !result.Succeeded || result.FinalChance != 1 || result.Rolled || state.RNGIndex != 0 {
		t.Fatalf("payout ticket result=%#v rngIndex=%d, want guaranteed success without RNG", result, state.RNGIndex)
	}
	actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if findBuffByID(&state.Players[0], "jackpot_payout_ticket") != nil {
		t.Fatalf("payout ticket was not consumed: buffs=%#v actions=%#v", state.Players[0].Buffs, actions)
	}
	if got := countActionKindRecursive(actions, "state_detach"); got != 1 {
		t.Fatalf("payout redemption should emit one detach action, got %d actions=%#v", got, actions)
	}
}

func TestAmusementJackpotTicketCannotOverrideForcedFailureButIsSpent(t *testing.T) {
	s := newAmusementProbabilityPrizeTestServer()
	actor := amusementProbabilityPrizeUnit("player:token", "player")
	actor.Buffs = []battleBuffState{{BuffID: "jackpot_payout_ticket", RemainTurns: 2}}
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID:       "hollow_heart_deny_chance",
		Trigger:  "on_before_probability_check",
		Priority: 1000,
		Effects:  []map[string]any{{"kind": "force_probability_failure"}},
	}}
	target := amusementProbabilityPrizeUnit("enemy:target", "enemy")
	state := &battleRuntimeState{Seed: 19, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}

	check, result := s.battleResolveStatusApplyProbability(state, &state.Players[0], &state.Enemies[0], 1, "skill:candy")
	if result.Succeeded || !result.ForcedFailure || result.Rolled || state.RNGIndex != 0 {
		t.Fatalf("forced-failure payout result=%#v rngIndex=%d", result, state.RNGIndex)
	}
	s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if findBuffByID(&state.Players[0], "jackpot_payout_ticket") != nil {
		t.Fatalf("a forced failure must still spend the promised ticket: %#v", state.Players[0].Buffs)
	}
}

func TestAmusementSugarShellCancelsExactlyOneHit(t *testing.T) {
	s := newAmusementProbabilityPrizeTestServer()
	attacker := amusementProbabilityPrizeUnit("enemy:attacker", "enemy")
	defender := amusementProbabilityPrizeUnit("player:pin", "player")
	defender.Buffs = []battleBuffState{{BuffID: "sugar_shell", RemainTurns: 2}}
	state := &battleRuntimeState{Seed: 1, Players: []battleUnitState{defender}, Enemies: []battleUnitState{attacker}}
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)

	first := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: attacker.UnitID, TargetID: defender.UnitID, Amount: 30, DamageType: "physical", Reason: "sugar_shell_test",
	})
	if first.Amount != 0 || state.Players[0].HP != 100 || findBuffByID(&state.Players[0], "sugar_shell") != nil {
		t.Fatalf("first hit amount=%v hp=%v buffs=%#v, want blocked and shell spent", first.Amount, state.Players[0].HP, state.Players[0].Buffs)
	}
	second := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: attacker.UnitID, TargetID: defender.UnitID, Amount: 30, DamageType: "physical", Reason: "sugar_shell_test",
	})
	if second.Amount != 30 || state.Players[0].HP != 70 {
		t.Fatalf("second hit amount=%v hp=%v, want unblocked 30 damage", second.Amount, state.Players[0].HP)
	}
}

func TestAmusementTailLightOnlyCatchesLethalHit(t *testing.T) {
	s := newAmusementProbabilityPrizeTestServer()
	attacker := amusementProbabilityPrizeUnit("enemy:attacker", "enemy")
	defender := amusementProbabilityPrizeUnit("player:light", "player")
	defender.HP = 75
	defender.Buffs = []battleBuffState{{BuffID: "last_parade_light", RemainTurns: 2}}
	state := &battleRuntimeState{Seed: 1, Players: []battleUnitState{defender}, Enemies: []battleUnitState{attacker}}
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)

	nonlethal := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: attacker.UnitID, TargetID: defender.UnitID, Amount: 74, DamageType: "magical", Reason: "tail_light_test",
	})
	if nonlethal.Amount != 74 || state.Players[0].HP != 1 || findBuffByID(&state.Players[0], "last_parade_light") == nil {
		t.Fatalf("nonlethal hit amount=%v hp=%v buffs=%#v, light should remain", nonlethal.Amount, state.Players[0].HP, state.Players[0].Buffs)
	}
	lethal := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: attacker.UnitID, TargetID: defender.UnitID, Amount: 1, DamageType: "magical", Reason: "tail_light_test",
	})
	if lethal.Amount != 0 || state.Players[0].HP != 1 || findBuffByID(&state.Players[0], "last_parade_light") != nil {
		t.Fatalf("lethal hit amount=%v hp=%v buffs=%#v, want caught and light spent", lethal.Amount, state.Players[0].HP, state.Players[0].Buffs)
	}
}

func TestAmusementTailLightUsesDamageAfterOrdinaryMitigation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		startingHP float64
		wantDamage float64
		wantLight  bool
	}{
		{name: "mitigated hit is no longer lethal", startingHP: 60, wantDamage: 50, wantLight: true},
		{name: "mitigated hit remains lethal at equality", startingHP: 50, wantDamage: 0, wantLight: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newAmusementProbabilityPrizeTestServer()
			s.deps.GameData.Buff = append(s.deps.GameData.Buff,
				map[string]any{"id": "test_half_damage", "durationTurns": 2, "stackPolicy": "refresh_duration"},
			)
			s.deps.GameData.BuffEffects = append(s.deps.GameData.BuffEffects, map[string]any{
				"id": "test_half_damage_while_active", "buffId": "test_half_damage",
				"trigger": "while_active", "priority": -1000,
				"conditions": []map[string]any{},
				"effects":    []map[string]any{{"kind": "reduce_damage_percent", "value": 0.5}},
			})
			attacker := amusementProbabilityPrizeUnit("enemy:attacker", "enemy")
			defender := amusementProbabilityPrizeUnit("player:light", "player")
			defender.HP = tc.startingHP
			defender.Buffs = []battleBuffState{
				{BuffID: "last_parade_light", RemainTurns: 2},
				{BuffID: "test_half_damage", RemainTurns: 2},
			}
			state := &battleRuntimeState{Seed: 1, Players: []battleUnitState{defender}, Enemies: []battleUnitState{attacker}}
			runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)

			result := runtime.ApplyDamage(battleskills.DamageAction{
				SourceID: attacker.UnitID, TargetID: defender.UnitID, Amount: 100,
				DamageType: "physical", Reason: "tail_light_mitigation_order_test",
			})
			if result.Amount != tc.wantDamage {
				t.Fatalf("damage=%v, want %v", result.Amount, tc.wantDamage)
			}
			if hasLight := findBuffByID(&state.Players[0], "last_parade_light") != nil; hasLight != tc.wantLight {
				t.Fatalf("light remaining=%v, want %v; buffs=%#v", hasLight, tc.wantLight, state.Players[0].Buffs)
			}
		})
	}
}

func TestAmusementTailLightUsesRedirectedTargetDamageAndCatchesRedirectRecipient(t *testing.T) {
	s := newAmusementProbabilityPrizeTestServer()
	s.deps.GameData.Buff = append(s.deps.GameData.Buff,
		map[string]any{"id": "test_redirect_link", "durationTurns": 2, "stackPolicy": "refresh_duration"},
	)
	s.deps.GameData.BuffEffects = append(s.deps.GameData.BuffEffects, map[string]any{
		"id": "test_redirect_link_while_active", "buffId": "test_redirect_link",
		"trigger": "while_active", "priority": 0,
		"conditions": []map[string]any{},
		"effects":    []map[string]any{{"kind": "damage_redirect_to_source", "percent": 0.7}},
	})
	attacker := amusementProbabilityPrizeUnit("enemy:attacker", "enemy")
	protected := amusementProbabilityPrizeUnit("player:protected", "player")
	protected.HP = 40
	guard := amusementProbabilityPrizeUnit("player:guard", "player")
	guard.HP = 60
	protected.Buffs = []battleBuffState{
		{BuffID: "last_parade_light", RemainTurns: 2, SourceID: protected.UnitID},
		{BuffID: "test_redirect_link", RemainTurns: 2, SourceID: guard.UnitID},
	}
	guard.Buffs = []battleBuffState{{BuffID: "last_parade_light", RemainTurns: 2, SourceID: guard.UnitID}}
	state := &battleRuntimeState{Seed: 1, Players: []battleUnitState{protected, guard}, Enemies: []battleUnitState{attacker}}
	runtime := newBattleSkillActionRuntime(state, &state.Enemies[0], time.UnixMilli(1), s, nil)

	result := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: attacker.UnitID, TargetID: protected.UnitID, Amount: 100,
		DamageType: "physical", Reason: "tail_light_redirect_order_test",
	})
	if result.Amount != 30 || state.Players[0].HP != 10 {
		t.Fatalf("protected damage=%v hp=%v, want 30 damage and 10 hp", result.Amount, state.Players[0].HP)
	}
	if findBuffByID(&state.Players[0], "last_parade_light") == nil {
		t.Fatalf("redirect made protected damage nonlethal, but spent its light: %#v", state.Players[0].Buffs)
	}
	if state.Players[1].HP != 60 || findBuffByID(&state.Players[1], "last_parade_light") != nil {
		t.Fatalf("redirect recipient hp=%v buffs=%#v, want redirected lethal hit caught once", state.Players[1].HP, state.Players[1].Buffs)
	}
}

func TestAmusementHollowHeartTwentiethFailureEmptiesAndActs(t *testing.T) {
	s := newAmusementCrownTestServer()
	s.deps.GameData.Buff = []map[string]any{
		{"id": "dark_heart", "kind": "buff", "durationTurns": 3, "stackPolicy": "stack", "maxStacks": 20},
		{"id": "test_debuff", "kind": "debuff", "durationTurns": 4, "stackPolicy": "refresh_duration", "dispellable": false},
	}
	actor := amusementCrownOwner(0)
	actor.UnitID = "player:hollow"
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID:       "hollow_heart_deny_chance",
			Trigger:  "on_before_probability_check",
			Priority: 1000,
			Effects:  []map[string]any{{"kind": "force_probability_failure"}},
		},
		{
			ID:       "hollow_heart_gather_darkness",
			Trigger:  "on_probability_failure",
			Priority: 10,
			Effects: []map[string]any{{
				"kind": "add_buff", "buffId": "dark_heart", "durationTurns": 3, "targetMode": "self",
			}},
		},
		{
			ID:                   "hollow_heart_empty_beat",
			Trigger:              "on_probability_failure",
			Priority:             0,
			MaxTriggersPerBattle: 2,
			Conditions: []map[string]any{{
				"type": "self_buff_count_gte", "buffId": "dark_heart", "value": 20,
			}},
			Effects: []map[string]any{
				{
					"kind": "remove_buff_by_kind", "buffKind": "debuff", "selectionMode": "max_remaining",
					"maxBuffs": 16, "dispellableOnly": false, "targetMode": "self",
				},
				{"kind": "remove_buff", "buffId": "dark_heart", "targetMode": "self"},
				{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
			},
		},
	}
	for range 19 {
		actor.Buffs = append(actor.Buffs, battleBuffState{BuffID: "dark_heart", RemainTurns: 3, SourceID: actor.UnitID})
	}
	actor.UserID = 1
	actor.Buffs = append(actor.Buffs, battleBuffState{BuffID: "test_debuff", RemainTurns: 4, SourceID: "enemy:target"})
	state := &battleRuntimeState{
		Wave:    1,
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{{
			UnitID: "enemy:target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1000, MaxHP: 1000,
			Stats: map[string]float64{"hp": 1000}, BaseStats: map[string]float64{"hp": 1000},
		}},
	}
	state.Players[0].Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
	check, result := s.battleResolveStatusApplyProbability(state, &state.Players[0], &state.Enemies[0], 1, "status:test")
	if !result.ForcedFailure || state.RNGIndex != 0 {
		t.Fatalf("twentieth check result=%#v rng=%d, want deterministic forced failure", result, state.RNGIndex)
	}
	actions := s.battleFinalizeProbabilityResult(state, time.UnixMilli(1), check, result)
	if battleCountBuff(&state.Players[0], "dark_heart") != 0 || findBuffByID(&state.Players[0], "test_debuff") != nil {
		t.Fatalf("empty beat did not clear darkness and debuff: buffs=%#v actions=%#v", state.Players[0].Buffs, actions)
	}
	if got := countActionKindRecursive(actions, "extra_turn"); got != 1 || state.Enemies[0].HP != 1000 || len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("empty beat did not queue one formal extra action: extra=%d enemyHp=%v queue=%#v actions=%#v", got, state.Enemies[0].HP, state.ExtraTurnQueue, actions)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
		t.Fatalf("process hollow-heart formal extra turn: %v", err)
	}
	if state.Enemies[0].HP >= 1000 || len(state.ExtraTurnQueue) != 0 {
		t.Fatalf("empty beat formal turn did not settle one real action: enemyHp=%v queue=%#v", state.Enemies[0].HP, state.ExtraTurnQueue)
	}
	if state.RNGIndex != 1 {
		t.Fatalf("only the deterministic target selection for the extra action should consume RNG once, got %d", state.RNGIndex)
	}
}

func TestAmusementPaperShieldRedeemsTwoFailuresForCleanse(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "consolation_prize_guard", "kind": "buff", "durationTurns": 3, "stackPolicy": "stack", "maxStacks": 2},
		{"id": "test_debuff", "kind": "debuff", "durationTurns": 4, "stackPolicy": "refresh_duration", "dispellable": true},
	}
	actor := amusementProbabilityPrizeUnit("player:paper", "player")
	actor.Buffs = []battleBuffState{{BuffID: "test_debuff", RemainTurns: 4, SourceID: "enemy:target"}}
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID: "consolation_prize_paper_shield_collect_stub", Trigger: "on_probability_failure", Priority: 10,
			MaxTriggersPerTurn: 2,
			Effects: []map[string]any{{
				"kind": "add_buff", "buffId": "consolation_prize_guard", "durationTurns": 3, "targetMode": "self",
			}},
		},
		{
			ID: "consolation_prize_paper_shield_redeem", Trigger: "on_probability_failure", Priority: 0,
			MaxTriggersPerTurn: 1,
			Conditions: []map[string]any{{
				"type": "self_buff_count_gte", "buffId": "consolation_prize_guard", "value": 2,
			}},
			Effects: []map[string]any{
				{
					"kind": "remove_buff_by_kind", "buffKind": "debuff", "selectionMode": "max_remaining",
					"maxBuffs": 1, "dispellableOnly": true, "targetMode": "self",
				},
				{"kind": "remove_buff", "buffId": "consolation_prize_guard", "targetMode": "self"},
			},
		},
	}
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}
	state.beginBattleHookTurn()
	for attempt := 0; attempt < 2; attempt++ {
		check, result := s.battleResolveStatusApplyProbability(state, &state.Players[0], nil, 0, "status:test")
		s.battleFinalizeProbabilityResult(state, time.UnixMilli(int64(attempt+1)), check, result)
	}
	if findBuffByID(&state.Players[0], "test_debuff") != nil || battleCountBuff(&state.Players[0], "consolation_prize_guard") != 0 {
		t.Fatalf("two losing stubs were not redeemed for a cleanse: %#v", state.Players[0].Buffs)
	}
}

func TestAmusementApplauseCrownUsesCooldownInsteadOfBattleCap(t *testing.T) {
	s := newAmusementCrownTestServer()
	s.deps.GameData.Buff = []map[string]any{
		{"id": "full_house_applause", "kind": "buff", "durationTurns": 2, "stackPolicy": "stack", "maxStacks": 3},
		{"id": "full_house_applause_cooldown", "kind": "buff", "durationTurns": 3, "dispellable": false, "stackPolicy": "refresh_duration"},
	}
	actor := amusementCrownOwner(0)
	actor.UnitID = "player:applause"
	actor.UserID = 1
	actor.EquipmentHooks = []battleEquipmentHookRule{
		{
			ID: "full_house_applause_crown_gather", Trigger: "on_probability_success", Priority: 10,
			MaxTriggersPerTurn: 3,
			Effects: []map[string]any{{
				"kind": "add_buff", "buffId": "full_house_applause", "durationTurns": 2, "targetMode": "self",
			}},
		},
		{
			ID: "full_house_applause_crown_encore", Trigger: "on_probability_success", Priority: 0,
			MaxTriggersPerTurn: 1,
			Conditions: []map[string]any{
				{"type": "self_buff_count_gte", "buffId": "full_house_applause", "value": 3},
				{"type": "self_buff_count_eq", "buffId": "full_house_applause_cooldown", "value": 0},
			},
			Effects: []map[string]any{
				{"kind": "remove_buff", "buffId": "full_house_applause", "targetMode": "self"},
				{"kind": "add_buff", "buffId": "full_house_applause_cooldown", "durationTurns": 3, "targetMode": "self"},
				{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
			},
		},
	}
	state := &battleRuntimeState{
		Wave:    1,
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{{
			UnitID: "enemy:target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1_000_000, MaxHP: 1_000_000,
			Stats: map[string]float64{"hp": 1_000_000}, BaseStats: map[string]float64{"hp": 1_000_000},
		}},
	}
	state.Players[0].Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
	var eventTime int64
	succeed := func() []map[string]any {
		state.beginBattleHookTurn()
		eventTime++
		check, result := s.battleResolveStatusApplyProbability(state, &state.Players[0], &state.Enemies[0], 1, "status:test")
		return s.battleFinalizeProbabilityResult(state, time.UnixMilli(eventTime), check, result)
	}
	expireCooldown := func() {
		for turn := 0; turn < 3 && findBuffByID(&state.Players[0], "full_house_applause_cooldown") != nil; turn++ {
			tickBattleBuffDurations(&state.Players[0])
		}
		if findBuffByID(&state.Players[0], "full_house_applause_cooldown") != nil {
			t.Fatalf("applause cooldown did not expire: %#v", state.Players[0].Buffs)
		}
	}

	for encore := 1; encore <= 4; encore++ {
		var triggerActions []map[string]any
		for success := 1; success <= 3; success++ {
			triggerActions = succeed()
			wantExtraTurns := 0
			if success == 3 {
				wantExtraTurns = 1
			}
			if got := countActionKindRecursive(triggerActions, "extra_turn"); got != wantExtraTurns {
				t.Fatalf("encore %d success %d extra turns=%d actions=%#v", encore, success, got, triggerActions)
			}
		}
		if battleCountBuff(&state.Players[0], "full_house_applause") != 0 {
			t.Fatalf("encore %d did not spend three applause stacks: %#v", encore, state.Players[0].Buffs)
		}
		if cooldown := findBuffByID(&state.Players[0], "full_house_applause_cooldown"); cooldown == nil || cooldown.RemainTurns != 3 {
			t.Fatalf("encore %d did not apply a three-turn cooldown: %#v", encore, state.Players[0].Buffs)
		}
		if len(state.ExtraTurnQueue) != 1 {
			t.Fatalf("encore %d should queue exactly one formal turn: %#v", encore, state.ExtraTurnQueue)
		}
		if encore == 1 {
			blocked := succeed()
			if got := countActionKindRecursive(blocked, "extra_turn"); got != 0 {
				t.Fatalf("active cooldown allowed another encore: %#v", blocked)
			}
			if got := battleCountBuff(&state.Players[0], "full_house_applause"); got != 1 {
				t.Fatalf("cooldown should preserve newly accumulated applause, got %d buffs=%#v", got, state.Players[0].Buffs)
			}
		}
		eventTime++
		if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(eventTime), nil); err != nil {
			t.Fatalf("process applause encore %d: %v", encore, err)
		}
		if cooldown := findBuffByID(&state.Players[0], "full_house_applause_cooldown"); cooldown == nil || cooldown.RemainTurns != 2 {
			t.Fatalf("encore %d formal turn must count toward cooldown: %#v", encore, state.Players[0].Buffs)
		}
		if len(state.ExtraTurnQueue) != 0 {
			t.Fatalf("encore %d formal turn did not settle: %#v", encore, state.ExtraTurnQueue)
		}
		expireCooldown()
	}
	if state.Enemies[0].HP >= 1_000_000 {
		t.Fatalf("four applause encore turns dealt no damage: enemyHp=%v", state.Enemies[0].HP)
	}
}

func TestAmusementExitTicketTriggersOncePerWaveWithCooldown(t *testing.T) {
	s := newAmusementCrownTestServer()
	s.deps.GameData.Buff = []map[string]any{
		{"id": "haunted_exit_route", "kind": "buff", "durationTurns": 1, "stackPolicy": "refresh_duration"},
		{"id": "haunted_exit_route_cooldown", "kind": "debuff", "durationTurns": 3, "dispellable": false, "stackPolicy": "refresh_duration"},
	}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id": "haunted_exit_route_while_active", "buffId": "haunted_exit_route", "trigger": "while_active", "priority": 80,
		"effects": []map[string]any{{"kind": "position_override", "position": "rearguard", "priority": 80}},
	}}
	actor := amusementCrownOwner(0)
	actor.UnitID = "player:exit"
	actor.UserID = 1
	actor.Position = "vanguard"
	actor.BasePosition = "vanguard"
	actor.EquipmentHooks = []battleEquipmentHookRule{{
		ID: "haunted_house_exit_ticket_first_failure", Trigger: "on_probability_failure", MaxTriggersPerWave: 1,
		Conditions: []map[string]any{{
			"type": "self_buff_count_eq", "buffId": "haunted_exit_route_cooldown", "value": 0,
		}},
		Effects: []map[string]any{
			{"kind": "add_buff", "buffId": "haunted_exit_route", "durationTurns": 1, "targetMode": "self"},
			{"kind": "add_buff", "buffId": "haunted_exit_route_cooldown", "durationTurns": 3, "targetMode": "self"},
			{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"},
		},
	}}
	state := &battleRuntimeState{
		Wave:    1,
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{{
			UnitID: "enemy:target", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 1000, MaxHP: 1000,
			Stats: map[string]float64{"hp": 1000}, BaseStats: map[string]float64{"hp": 1000},
		}},
	}
	state.Players[0].Skills = []battleSkillState{s.battleSkillByID("basic_attack")}
	failStatusApplication := func(at time.Time) []map[string]any {
		state.beginBattleHookTurn()
		check, result := s.battleResolveStatusApplyProbability(state, &state.Players[0], &state.Enemies[0], 0, "status:test")
		return s.battleFinalizeProbabilityResult(state, at, check, result)
	}
	expireCooldown := func() {
		for attempt := 0; attempt < 4 && findBuffByID(&state.Players[0], "haunted_exit_route_cooldown") != nil; attempt++ {
			tickBattleBuffDurations(&state.Players[0])
		}
		if findBuffByID(&state.Players[0], "haunted_exit_route_cooldown") != nil {
			t.Fatalf("exit-ticket cooldown did not expire: %#v", state.Players[0].Buffs)
		}
	}

	actions := failStatusApplication(time.UnixMilli(1))
	if got := countActionKindRecursive(actions, "extra_turn"); got != 1 || state.Enemies[0].HP != 1000 || len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("first failure did not queue a formal exit action: extra=%d enemyHp=%v queue=%#v actions=%#v", got, state.Enemies[0].HP, state.ExtraTurnQueue, actions)
	}
	if cooldown := findBuffByID(&state.Players[0], "haunted_exit_route_cooldown"); cooldown == nil || cooldown.RemainTurns != 3 {
		t.Fatalf("exit ticket did not apply its three-turn cooldown debuff: %#v", state.Players[0].Buffs)
	}
	rearIndex := -1
	for index, action := range actions {
		if action["kind"] == "stat_recalc" && action["position"] == "rearguard" && rearIndex < 0 {
			rearIndex = index
		}
	}
	if rearIndex < 0 || state.Players[0].Position != "rearguard" {
		t.Fatalf("exit route must move rear before queue execution: rearIndex=%d position=%s actions=%#v", rearIndex, state.Players[0].Position, actions)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(2), nil); err != nil {
		t.Fatalf("process exit-ticket formal extra turn: %v", err)
	}
	extraAction := lastBattleTurnActionData(t, state)
	if countActionKindRecursive(battleAnyMapSlice(extraAction["actionLog"]), "damage") == 0 || state.Enemies[0].HP >= 1000 {
		t.Fatalf("exit ticket formal turn did not attack from the rear state: event=%#v enemy=%#v", extraAction, state.Enemies[0])
	}
	if state.Players[0].Position != "vanguard" || findBuffByID(&state.Players[0], "haunted_exit_route") != nil {
		t.Fatalf("one-turn exit route did not expire cleanly, position=%s buffs=%#v", state.Players[0].Position, state.Players[0].Buffs)
	}
	if cooldown := findBuffByID(&state.Players[0], "haunted_exit_route_cooldown"); cooldown == nil || cooldown.RemainTurns != 2 {
		t.Fatalf("the granted extra turn must count toward the three-turn cooldown: %#v", state.Players[0].Buffs)
	}

	state.Wave = 2
	s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(3))
	blockedByCooldown := failStatusApplication(time.UnixMilli(4))
	if got := countActionKindRecursive(blockedByCooldown, "extra_turn"); got != 0 {
		t.Fatalf("new wave bypassed the active cooldown debuff: %#v", blockedByCooldown)
	}

	expireCooldown()
	secondWave := failStatusApplication(time.UnixMilli(5))
	if got := countActionKindRecursive(secondWave, "extra_turn"); got != 1 || len(state.ExtraTurnQueue) != 1 {
		t.Fatalf("eligible failure in the next wave did not trigger: queue=%#v actions=%#v", state.ExtraTurnQueue, secondWave)
	}
	if _, _, err := s.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(6), nil); err != nil {
		t.Fatalf("process second-wave exit-ticket turn: %v", err)
	}
	expireCooldown()
	sameWave := failStatusApplication(time.UnixMilli(7))
	if got := countActionKindRecursive(sameWave, "extra_turn"); got != 0 {
		t.Fatalf("exit ticket triggered twice in one enemy-refresh wave: %#v", sameWave)
	}

	hook := state.Players[0].EquipmentHooks[0]
	key := battleRuntimeCounterKey(state.Players[0].UnitID, battleEquipmentHookCounterGroup(hook))
	if state.SpecializationHookWaveCounts[key] != 1 || state.BattleHookBattleCounts[key] != 2 {
		t.Fatalf("wave and battle counters lost their distinct semantics: wave=%#v battle=%#v", state.SpecializationHookWaveCounts, state.BattleHookBattleCounts)
	}
}
