package httpapi

import (
	"testing"
	"time"
)

func TestBattleSkillCooldownSelectorFiltersCategoryAndUsesStableHighestRemaining(t *testing.T) {
	s := &Server{}
	unit := battleUnitState{
		UnitID: "player:mage",
		Skills: []battleSkillState{
			{ID: "arcane_a", Category: "arcane", CooldownRemain: 3},
			{ID: "perception_b", Category: "perception", CooldownRemain: 5},
			{ID: "snowpact_c", Category: "snowpact", CooldownRemain: 5},
			{ID: "blade_d", Category: "blade", CooldownRemain: 9},
			{ID: "arcane_ready", Category: "arcane", CooldownRemain: 0},
			{ID: "basic_attack", Category: "arcane", CooldownRemain: 4},
		},
	}
	selector := battleSkillCooldownSelector{
		SkillCategories: []string{"arcane", "perception", "snowpact"},
		SelectionMode:   "maxremaining",
		MaxSkills:       1,
	}
	if got := battleCountCoolingSkills(&unit, selector); got != 3 {
		t.Fatalf("cooling eligible skill count = %d, want 3", got)
	}
	action, changed := s.battleAdjustUnitSkillCooldownsWithSelector(&unit, -1, selector, unit.UnitID, "test", 0, nil)
	if !changed || action["kind"] != "skill_cooldown" {
		t.Fatalf("expected cooldown action, changed=%v action=%#v", changed, action)
	}
	if meta, _ := action["meta"].(map[string]any); meta["selectionMode"] != "max_remaining" {
		t.Fatalf("selection mode should stay protocol-canonical: %#v", action)
	}
	if got := unit.Skills[1].CooldownRemain; got != 4 {
		t.Fatalf("first highest remaining skill = %d, want 4", got)
	}
	if got := unit.Skills[2].CooldownRemain; got != 5 {
		t.Fatalf("stable tie-break must preserve later skill, got %d", got)
	}
	if got := unit.Skills[3].CooldownRemain; got != 9 {
		t.Fatalf("non-mage category must be untouched, got %d", got)
	}
	if got := unit.Skills[5].CooldownRemain; got != 4 {
		t.Fatalf("basic attack must be excluded, got %d", got)
	}
}

func TestEquipmentCooldownHookConsumesMPUsesLockoutAndAwakensAtMaxEnhance(t *testing.T) {
	s := &Server{}
	turnHook := battleEquipmentHookRule{
		ID:                 "timekeeper_turn",
		SourceItemID:       "timekeeper_eight_tick_wheel",
		SourceEnhanceLevel: 39,
		Trigger:            "on_turn_start",
		Conditions: []map[string]any{
			{"type": "self_mp_gte", "value": 20},
			{"type": "self_skill_cooldown_count_gte", "value": 2, "skillCategories": []string{"arcane", "perception", "snowpact"}},
			{"type": "self_buff_count_eq", "buffId": "timekeeper_eight_tick_lockout", "value": 0},
		},
		Effects: []map[string]any{
			{"kind": "consume_mp", "amountFormula": map[string]any{"baseAdd": 20, "terms": []any{}}, "requireFullAmount": true, "abortHookOnFail": true},
			{"kind": "adjust_skill_cooldown", "amount": -1, "targetMode": "self", "skillCategories": []string{"arcane", "perception", "snowpact"}, "selectionMode": "max_remaining", "maxSkills": 1},
			{"kind": "add_buff", "buffId": "timekeeper_eight_tick_lockout", "durationTurns": 3, "targetMode": "self"},
		},
	}
	waveHook := battleEquipmentHookRule{
		ID:                 "timekeeper_wave",
		SourceItemID:       "timekeeper_eight_tick_wheel",
		SourceEnhanceLevel: 39,
		Trigger:            "on_wave_start",
		Conditions: []map[string]any{
			{"type": "equipment_enhance_level_gte", "value": 40},
			{"type": "self_mp_gte", "value": 20},
			{"type": "self_skill_cooldown_count_gte", "value": 2, "skillCategories": []string{"arcane", "perception", "snowpact"}},
		},
		Effects: []map[string]any{
			{"kind": "consume_mp", "amountFormula": map[string]any{"baseAdd": 20, "terms": []any{}}, "requireFullAmount": true, "abortHookOnFail": true},
			{"kind": "adjust_skill_cooldown", "amount": -1, "targetMode": "self", "skillCategories": []string{"arcane", "perception", "snowpact"}, "selectionMode": "max_remaining", "maxSkills": 1},
			{"kind": "add_buff", "buffId": "timekeeper_eight_tick_lockout", "durationTurns": 2, "targetMode": "self"},
		},
	}
	state := &battleRuntimeState{Players: []battleUnitState{{
		UnitID:         "player:mage",
		Side:           "player",
		Alive:          true,
		HP:             100,
		MaxHP:          100,
		MP:             100,
		MaxMP:          100,
		Stats:          map[string]float64{"hp": 100, "mp": 100},
		BaseStats:      map[string]float64{"hp": 100, "mp": 100},
		EquipmentHooks: []battleEquipmentHookRule{turnHook, waveHook},
		Skills: []battleSkillState{
			{ID: "arcane_a", Category: "arcane", CooldownRemain: 4},
			{ID: "perception_b", Category: "perception", CooldownRemain: 3},
			{ID: "blade_c", Category: "blade", CooldownRemain: 8},
		},
	}}}
	actor := &state.Players[0]
	actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(0, 0), actor, nil, "on_turn_start", 0, "")
	if len(actions) < 3 || actor.MP != 80 || actor.Skills[0].CooldownRemain != 3 || actor.Skills[2].CooldownRemain != 8 {
		t.Fatalf("turn-start hook mismatch: actions=%#v actor=%#v", actions, actor)
	}
	if buff := findBuffByID(actor, "timekeeper_eight_tick_lockout"); buff == nil || buff.RemainTurns != 3 {
		t.Fatalf("turn-start lockout mismatch: %#v", actor.Buffs)
	}
	if repeat := s.battleTriggerUnitEquipmentHooks(state, time.Unix(1, 0), actor, nil, "on_turn_start", 0, ""); len(repeat) != 0 {
		t.Fatalf("lockout should suppress repeated turn-start trigger: %#v", repeat)
	}
	if premature := s.battleTriggerUnitEquipmentHooks(state, time.Unix(2, 0), actor, nil, "on_wave_start", 0, ""); len(premature) != 0 {
		t.Fatalf("enhance level 39 must not trigger wave awakening: %#v", premature)
	}
	actor.EquipmentHooks[1].SourceEnhanceLevel = 40
	waveActions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(3, 0), actor, nil, "on_wave_start", 0, "")
	if len(waveActions) < 3 || actor.MP != 60 || actor.Skills[0].CooldownRemain != 2 {
		t.Fatalf("awakened wave hook mismatch: actions=%#v actor=%#v", waveActions, actor)
	}
	if buff := findBuffByID(actor, "timekeeper_eight_tick_lockout"); buff == nil || buff.RemainTurns != 2 {
		t.Fatalf("wave-start trigger should refresh lockout for two upcoming turns: %#v", actor.Buffs)
	}
}

func forbiddenBindingTransferEffect() map[string]any {
	categories := []any{"arcane", "perception", "snowpact", "holy", "holy_cat"}
	return map[string]any{
		"kind":        "transfer_skill_cooldown",
		"amount":      1,
		"targetMode":  "self",
		"requireBoth": true,
		"from": map[string]any{
			"includeBasic":        false,
			"skillCategories":     categories,
			"excludeContextSkill": true,
			"requireCooling":      true,
			"selectionMode":       "max_remaining",
			"maxSkills":           1,
		},
		"to": map[string]any{
			"includeBasic":    false,
			"skillCategories": categories,
			"selectionMode":   "context_skill",
		},
		"waiveIncreaseBuffId": "forbidden_binding_free_transfer",
		"consumeWaiverBuff":   true,
	}
}

func TestTransferSkillCooldownFiltersExistingCategoriesAndIsAtomic(t *testing.T) {
	s := &Server{}
	unit := battleUnitState{
		UnitID: "player:binding",
		Skills: []battleSkillState{
			{ID: "holy_cast", Category: "holy", CooldownRemain: 2},
			{ID: "holy_cat_long", Category: "holy_cat", CooldownRemain: 5},
			{ID: "perception_tie", Category: "perception", CooldownRemain: 5},
			{ID: "blade_longer", Category: "blade", CooldownRemain: 9},
			{ID: "basic_attack", Category: "holy", CooldownRemain: 4},
		},
	}
	action, consumed, changed := s.battleTransferSkillCooldown(&unit, "holy_cast", forbiddenBindingTransferEffect(), unit.UnitID, "test", nil)
	if !changed || consumed != "" {
		t.Fatalf("expected a paid transfer, changed=%v consumed=%q action=%#v", changed, consumed, action)
	}
	if unit.Skills[0].CooldownRemain != 3 {
		t.Fatalf("cast skill cooldown = %d, want 3", unit.Skills[0].CooldownRemain)
	}
	if unit.Skills[1].CooldownRemain != 4 || unit.Skills[2].CooldownRemain != 5 {
		t.Fatalf("stable highest eligible selection mismatch: %#v", unit.Skills)
	}
	if unit.Skills[3].CooldownRemain != 9 || unit.Skills[4].CooldownRemain != 4 {
		t.Fatalf("blade and basic attack must be excluded: %#v", unit.Skills)
	}

	noCandidate := battleUnitState{UnitID: "player:solo", Skills: []battleSkillState{{ID: "holy_cast", Category: "holy", CooldownRemain: 2}}}
	if action, _, changed := s.battleTransferSkillCooldown(&noCandidate, "holy_cast", forbiddenBindingTransferEffect(), noCandidate.UnitID, "test", nil); changed || action != nil || noCandidate.Skills[0].CooldownRemain != 2 {
		t.Fatalf("missing counterpart must not charge the cast skill: changed=%v action=%#v unit=%#v", changed, action, noCandidate)
	}
}

func TestTransferSkillCooldownConsumesWaveWaiverOnlyAfterSuccess(t *testing.T) {
	s := &Server{}
	unit := battleUnitState{
		UnitID: "player:awakened_binding",
		Skills: []battleSkillState{
			{ID: "perception_cast", Category: "perception", CooldownRemain: 2},
			{ID: "arcane_long", Category: "arcane", CooldownRemain: 4},
		},
	}
	battleApplyOrRefreshBuff(&unit, "forbidden_binding_free_transfer", -1, nil, unit.UnitID, "refresh_duration")
	action, consumed, changed := s.battleTransferSkillCooldown(&unit, "perception_cast", forbiddenBindingTransferEffect(), unit.UnitID, "test", nil)
	if !changed || consumed != "forbidden_binding_free_transfer" {
		t.Fatalf("expected successful waived transfer, changed=%v consumed=%q action=%#v", changed, consumed, action)
	}
	if unit.Skills[0].CooldownRemain != 2 || unit.Skills[1].CooldownRemain != 3 {
		t.Fatalf("waiver should prevent only the increase: %#v", unit.Skills)
	}
	if findBuffByID(&unit, "forbidden_binding_free_transfer") != nil {
		t.Fatalf("successful transfer must consume waiver: %#v", unit.Buffs)
	}

	unit.Skills[1].CooldownRemain = 0
	battleApplyOrRefreshBuff(&unit, "forbidden_binding_free_transfer", -1, nil, unit.UnitID, "refresh_duration")
	if _, consumed, changed := s.battleTransferSkillCooldown(&unit, "perception_cast", forbiddenBindingTransferEffect(), unit.UnitID, "test", nil); changed || consumed != "" || findBuffByID(&unit, "forbidden_binding_free_transfer") == nil {
		t.Fatalf("failed transfer must preserve waiver: changed=%v consumed=%q buffs=%#v", changed, consumed, unit.Buffs)
	}
}

func TestEquipmentHookDispatchesCooldownTransferFromActiveSkillContext(t *testing.T) {
	s := &Server{}
	unit := battleUnitState{
		UnitID: "player:binding_hook",
		Alive:  true,
		HP:     100,
		MaxHP:  100,
		EquipmentHooks: []battleEquipmentHookRule{{
			ID:           "binding_transfer",
			SourceItemID: "forbidden_binding_bracer",
			Trigger:      "on_after_skill_cooldown_tick",
			Effects:      []map[string]any{forbiddenBindingTransferEffect()},
		}},
		Skills: []battleSkillState{
			{ID: "holy_cat_cast", Category: "holy_cat", CooldownRemain: 3},
			{ID: "holy_long", Category: "holy", CooldownRemain: 5},
		},
	}
	state := &battleRuntimeState{Players: []battleUnitState{unit}}
	actor := &state.Players[0]
	if actions := s.battleTriggerUnitEquipmentHooksWithContext(
		state,
		time.Unix(0, 0),
		actor,
		nil,
		nil,
		"on_after_skill_cooldown_tick",
		0,
		"",
		&battleEquipmentHookContext{SkillID: "holy_cat_cast", CastKind: "active_turn"},
	); len(actions) != 1 || actor.Skills[0].CooldownRemain != 4 || actor.Skills[1].CooldownRemain != 4 {
		t.Fatalf("active cast context should dispatch one atomic transfer: actions=%#v actor=%#v", actions, actor)
	}

	before := append([]battleSkillState(nil), actor.Skills...)
	if actions := s.battleTriggerUnitEquipmentHooks(state, time.Unix(1, 0), actor, nil, "on_after_skill_cooldown_tick", 0, ""); len(actions) != 0 {
		t.Fatalf("trigger without authoritative cast context must be ignored: %#v", actions)
	}
	if actor.Skills[0].CooldownRemain != before[0].CooldownRemain || actor.Skills[1].CooldownRemain != before[1].CooldownRemain {
		t.Fatalf("missing cast context must not mutate cooldowns: before=%#v after=%#v", before, actor.Skills)
	}
}
