package httpapi

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBattleCollectRuntimeDamageAmpFromEquipmentHooksAcceptsDamageAmpKinds(t *testing.T) {
	s := &Server{}
	cases := []struct {
		name     string
		kind     string
		expected float64
	}{
		{name: "runtime_damage_amp", kind: "runtime_damage_amp", expected: 0.12},
		{name: "add_damage_percent", kind: "add_damage_percent", expected: 0.25},
	}
	for _, tc := range cases {
		self := &battleUnitState{
			UnitID: "player:1",
			EquipmentHooks: []battleEquipmentHookRule{
				{
					ID:           "hook:" + tc.name,
					SourceItemID: "base5_bloodforge_armor",
					Trigger:      "on_before_deal_damage",
					Effects:      []map[string]any{{"kind": tc.kind, "value": tc.expected}},
					Conditions: []map[string]any{
						{"damageTypes": []any{"physical"}},
					},
				},
			},
		}
		amp, logs, sources := s.battleCollectRuntimeDamageAmpFromEquipmentHooks(nil, time.UnixMilli(0), self, &battleUnitState{UnitID: "enemy:1"}, "on_before_deal_damage", 100, "physical")
		if amp != tc.expected {
			t.Fatalf("%s amp=%v, want %v", tc.name, amp, tc.expected)
		}
		if len(logs) != 1 {
			t.Fatalf("%s logs=%#v, want 1 log", tc.name, logs)
		}
		if got := logs[0]["meta"].(map[string]any)["runtimeDamageAmp"]; got != tc.expected {
			t.Fatalf("%s meta runtimeDamageAmp=%v, want %v", tc.name, got, tc.expected)
		}
		if len(sources) != 1 {
			t.Fatalf("%s sources=%#v, want 1 source", tc.name, sources)
		}
		if got := sources[0]["kind"]; got != "equipment" {
			t.Fatalf("%s source kind=%v, want equipment", tc.name, got)
		}
	}
}

func TestBattleUnitMarkedTargetEffectIsSemanticOnly(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "marked_target_runtime_test",
			"buffId":  "marked_target_runtime_test",
			"trigger": "while_active",
			"effects": []map[string]any{{"kind": "marked_target"}},
		},
	}
	unit := &battleUnitState{
		UnitID:    "enemy:1",
		Side:      "enemy",
		Alive:     true,
		HP:        100,
		MaxHP:     100,
		Stats:     map[string]float64{"speed": 12, "physics_damage_resist": 0.2},
		BaseStats: map[string]float64{"speed": 12, "physics_damage_resist": 0.2},
		Buffs:     []battleBuffState{{BuffID: "marked_target_runtime_test", RemainTurns: 2}},
	}

	if !s.battleUnitHasWhileActiveEffect(unit, "marked_target", nil) {
		t.Fatalf("expected marked_target while_active semantic effect")
	}
	s.battleRecomputeUnitStats(unit)
	if unit.Stats["speed"] != 12 || unit.Stats["physics_damage_resist"] != 0.2 {
		t.Fatalf("marked_target should not mutate stats, stats=%#v", unit.Stats)
	}
}

func TestRuntimeDamageAmpFormulaUsesEnhanceLevelAndFrozenTarget(t *testing.T) {
	s := &Server{}
	self := &battleUnitState{
		UnitID: "player:1",
		EquipmentHooks: []battleEquipmentHookRule{
			{
				ID:                 "frostbreaker_charm_shatter_amp",
				SourceItemID:       "frostbreaker_charm",
				SourceEnhanceLevel: 40,
				Trigger:            "on_before_deal_damage",
				Conditions: []map[string]any{
					{"damageTypes": []any{"physical"}},
					{"type": "target_has_buff", "buffId": "frozen_action_suppressed"},
				},
				Effects: []map[string]any{
					{
						"kind":     "runtime_damage_amp",
						"value":    0.1,
						"valueMin": 0.1,
						"valueMax": 0.3,
						"valueFormula": map[string]any{
							"baseAdd": 0,
							"terms": []map[string]any{
								{"attrKey": "equipment_enhance_level", "coefficient": 0.005},
							},
						},
						"damageTypes": []any{"physical"},
					},
				},
			},
		},
	}
	frozenTarget := &battleUnitState{UnitID: "enemy:1", Buffs: []battleBuffState{{BuffID: "frozen_action_suppressed", RemainTurns: 1}}}
	amp, _, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooks(nil, time.UnixMilli(0), self, frozenTarget, "on_before_deal_damage", 100, "physical")
	if amp != 0.3 {
		t.Fatalf("enhanced frozen amp=%v, want 0.3", amp)
	}
	self.EquipmentHooks[0].SourceEnhanceLevel = 0
	amp, _, _ = s.battleCollectRuntimeDamageAmpFromEquipmentHooks(nil, time.UnixMilli(0), self, frozenTarget, "on_before_deal_damage", 100, "physical")
	if amp != 0.1 {
		t.Fatalf("base frozen amp=%v, want 0.1", amp)
	}
	amp, _, _ = s.battleCollectRuntimeDamageAmpFromEquipmentHooks(nil, time.UnixMilli(0), self, frozenTarget, "on_before_deal_damage", 100, "magical")
	if amp != 0 {
		t.Fatalf("magical damage should not get frostbreaker amp, got %v", amp)
	}
	notFrozen := &battleUnitState{UnitID: "enemy:2"}
	amp, _, _ = s.battleCollectRuntimeDamageAmpFromEquipmentHooks(nil, time.UnixMilli(0), self, notFrozen, "on_before_deal_damage", 100, "physical")
	if amp != 0 {
		t.Fatalf("non-frozen target should not get frostbreaker amp, got %v", amp)
	}
}

func TestMarkedTargetFollowupProcChanceFormulaCapsAtOne(t *testing.T) {
	effect := map[string]any{
		"procChanceBase": 0.5,
		"procChanceMin":  0,
		"procChanceMax":  1,
		"procChanceFormula": map[string]any{
			"baseAdd": 0,
			"terms": []map[string]any{
				{"attrKey": "equipment_enhance_level", "coefficient": 0.0125},
			},
		},
	}
	chance, configured := battleResolveEquipmentHookProcChance(&battleUnitState{}, battleEquipmentHookRule{SourceEnhanceLevel: 0}, effect)
	if !configured || chance != 0.5 {
		t.Fatalf("level 0 chance configured=%v chance=%v, want 0.5", configured, chance)
	}
	chance, configured = battleResolveEquipmentHookProcChance(&battleUnitState{}, battleEquipmentHookRule{SourceEnhanceLevel: 40}, effect)
	if !configured || chance != 1 {
		t.Fatalf("level 40 chance configured=%v chance=%v, want 1", configured, chance)
	}
	chance, configured = battleResolveEquipmentHookProcChance(&battleUnitState{}, battleEquipmentHookRule{SourceEnhanceLevel: 80}, effect)
	if !configured || chance != 1 {
		t.Fatalf("level 80 chance should cap at 1, configured=%v chance=%v", configured, chance)
	}
}

func TestBattleEvaluateBuffConditionSelfWeaponClass(t *testing.T) {
	condition := map[string]any{
		"type":          "self_weapon_class",
		"weaponClasses": []any{"bow"},
	}
	archer := &battleUnitState{UnitID: "player:1", WeaponClasses: []string{"bow"}}
	crossbowman := &battleUnitState{UnitID: "player:2", WeaponClasses: []string{"crossbow"}}

	if !battleEvaluateBuffCondition(condition, archer, nil, 0, "", nil, 0) {
		t.Fatalf("bow bearer should satisfy self_weapon_class condition")
	}
	if battleEvaluateBuffCondition(condition, crossbowman, nil, 0, "", nil, 0) {
		t.Fatalf("crossbow bearer should not satisfy bow self_weapon_class condition")
	}
}

func TestBattleEvaluateBuffConditionSourceWeaponClass(t *testing.T) {
	condition := map[string]any{
		"type":          "source_weapon_class",
		"weaponClasses": []any{"special"},
	}
	target := &battleUnitState{UnitID: "player:1", WeaponClasses: []string{"bow"}}
	threadwheelSource := &battleUnitState{UnitID: "enemy:1", WeaponClasses: []string{"special"}}
	daggerSource := &battleUnitState{UnitID: "enemy:2", WeaponClasses: []string{"dagger"}}

	if !battleEvaluateBuffCondition(condition, target, threadwheelSource, 0, "", nil, 0) {
		t.Fatalf("special source should satisfy source_weapon_class condition")
	}
	if battleEvaluateBuffCondition(condition, target, daggerSource, 0, "", nil, 0) {
		t.Fatalf("dagger source should not satisfy special source_weapon_class condition")
	}
}

func TestBattleApplyBeforeDealDamageBuffHooksReportsSources(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "counter_edge",
			"durationTurns": 2,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "counter_edge_amp",
			"buffId":   "counter_edge",
			"trigger":  "on_before_deal_damage",
			"priority": 0,
			"effects": []map[string]any{
				{"kind": "add_damage_percent", "value": 0.25},
			},
		},
	}
	source := &battleUnitState{
		UnitID: "player:1",
		Buffs: []battleBuffState{
			{BuffID: "counter_edge", RemainTurns: 2},
		},
	}
	damage, logs, sources := s.battleApplyBeforeDealDamageBuffHooks(nil, time.UnixMilli(0), source, &battleUnitState{UnitID: "enemy:1"}, 100, "physical")
	if damage != 125 {
		t.Fatalf("damage=%v, want 125", damage)
	}
	if len(logs) != 1 {
		t.Fatalf("logs=%#v, want 1 log", logs)
	}
	if len(sources) != 1 {
		t.Fatalf("sources=%#v, want 1 source", sources)
	}
	if got := sources[0]["kind"]; got != "buff" {
		t.Fatalf("source kind=%v, want buff", got)
	}
	if got := sources[0]["id"]; got != "counter_edge" {
		t.Fatalf("source id=%v, want counter_edge", got)
	}
}

func TestBoneShieldFixedDamageConsumesChargesAndExpires(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "bone_shield",
			"durationTurns": 3,
			"maxCharges":    3,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "guardian_reduce_damage",
			"buffId":   "guardian",
			"trigger":  "on_before_take_damage",
			"priority": 10,
			"effects": []map[string]any{
				{"kind": "reduce_damage_percent", "value": 0.5},
			},
		},
		{
			"id":       "bone_shield_take_damage",
			"buffId":   "bone_shield",
			"trigger":  "on_before_take_damage",
			"priority": -100,
			"effects": []map[string]any{
				{"kind": "set_damage_value", "value": 1},
				{"kind": "consume_charge"},
			},
		},
	}

	target := &battleUnitState{
		UnitID: "target:1",
		HP:     100,
		MaxHP:  100,
		Alive:  true,
		Buffs: []battleBuffState{
			{BuffID: "guardian", RemainTurns: 3},
			{BuffID: "bone_shield", RemainTurns: 3},
		},
	}

	damage, logs := s.battleApplyBeforeTakeDamageBuffHooks(nil, time.UnixMilli(0), nil, target, 40, "physical")
	if damage != 1 {
		t.Fatalf("first damage=%v, want 1", damage)
	}
	if len(logs) == 0 {
		t.Fatalf("expected logs for first hit")
	}
	if target.Buffs[1].Triggered != 1 {
		t.Fatalf("first charge not consumed: %#v", target.Buffs[1])
	}

	damage, _ = s.battleApplyBeforeTakeDamageBuffHooks(nil, time.UnixMilli(1), nil, target, 40, "physical")
	if damage != 1 {
		t.Fatalf("second damage=%v, want 1", damage)
	}
	if target.Buffs[1].Triggered != 2 {
		t.Fatalf("second charge not consumed: %#v", target.Buffs[1])
	}

	damage, logs = s.battleApplyBeforeTakeDamageBuffHooks(nil, time.UnixMilli(2), nil, target, 40, "physical")
	if damage != 1 {
		t.Fatalf("third damage=%v, want 1", damage)
	}
	if battleHasBuff(target, "bone_shield") {
		t.Fatalf("bone_shield should expire after third trigger, buffs=%#v", target.Buffs)
	}
	if len(logs) == 0 || logs[len(logs)-1]["kind"] != "state_detach" {
		t.Fatalf("expected state_detach log on third trigger, got %#v", logs)
	}
}

func TestWhileActiveDamageReductionSettlesWithFixedAndPayloadValues(t *testing.T) {
	tests := []struct {
		name       string
		buffID     string
		payload    map[string]any
		effects    []map[string]any
		wantDamage float64
		wantSpeed  float64
	}{
		{
			name: "parade royal aegis", buffID: "parade_royal_aegis",
			effects:    []map[string]any{{"kind": "reduce_damage_percent", "value": 0.15}},
			wantDamage: 85, wantSpeed: 10,
		},
		{
			name: "enhanced parade tail light", buffID: "last_parade_light",
			payload: map[string]any{"speedBonus": 14.0, "damageResistBonus": 0.09},
			effects: []map[string]any{
				{"kind": "add_stat", "attr": "speed", "value": 0, "payloadValueKey": "speedBonus", "payloadValueCoeff": 1},
				{"kind": "reduce_damage_percent", "value": 0, "payloadValueKey": "damageResistBonus", "payloadValueCoeff": 1},
			},
			wantDamage: 91, wantSpeed: 24,
		},
		{
			name: "plush stunt double", buffID: "plush_stunt_double_role",
			effects: []map[string]any{
				{"kind": "skip_turn_order"},
				{"kind": "prefer_as_enemy_target"},
				{"kind": "reduce_damage_percent", "value": 0.25},
			},
			wantDamage: 75, wantSpeed: 10,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &Server{}
			s.deps.GameData.Buff = []map[string]any{{"id": test.buffID, "durationTurns": 2}}
			s.deps.GameData.BuffEffects = []map[string]any{{
				"id": test.buffID + "_while_active", "buffId": test.buffID, "trigger": "while_active", "effects": test.effects,
			}}
			target := &battleUnitState{
				UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100,
				Stats: map[string]float64{"hp": 100, "speed": 10}, BaseStats: map[string]float64{"hp": 100, "speed": 10},
				Buffs: []battleBuffState{{BuffID: test.buffID, RemainTurns: 2, Payload: test.payload}},
			}
			s.battleRecomputeUnitStats(target)
			if target.Stats["speed"] != test.wantSpeed {
				t.Fatalf("speed=%v, want %v", target.Stats["speed"], test.wantSpeed)
			}
			damage, logs := s.battleApplyBeforeTakeDamageBuffHooks(nil, time.UnixMilli(1), nil, target, 100, "physical")
			if damage != test.wantDamage || len(logs) != 1 {
				t.Fatalf("damage=%v logs=%#v, want damage %v with one settlement", damage, logs, test.wantDamage)
			}
		})
	}
}

func TestBoneMendFixedHealConsumesChargesAndExpires(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "bone_mend",
			"durationTurns": 3,
			"maxCharges":    3,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "bone_mend_heal",
			"buffId":   "bone_mend",
			"trigger":  "on_before_heal",
			"priority": -100,
			"effects": []map[string]any{
				{"kind": "set_heal_value", "value": 1},
				{"kind": "consume_charge"},
			},
		},
	}

	target := &battleUnitState{
		UnitID: "target:1",
		HP:     40,
		MaxHP:  100,
		Alive:  true,
		Buffs: []battleBuffState{
			{BuffID: "bone_mend", RemainTurns: 3},
		},
	}

	heal, logs := s.battleApplyBeforeHealBuffHooks(nil, time.UnixMilli(0), nil, target, 20)
	if heal != 1 {
		t.Fatalf("first heal=%v, want 1", heal)
	}
	if len(logs) == 0 {
		t.Fatalf("expected logs for first heal")
	}
	if target.Buffs[0].Triggered != 1 {
		t.Fatalf("first charge not consumed: %#v", target.Buffs[0])
	}

	heal, _ = s.battleApplyBeforeHealBuffHooks(nil, time.UnixMilli(1), nil, target, 20)
	if heal != 1 {
		t.Fatalf("second heal=%v, want 1", heal)
	}
	if target.Buffs[0].Triggered != 2 {
		t.Fatalf("second charge not consumed: %#v", target.Buffs[0])
	}

	heal, logs = s.battleApplyBeforeHealBuffHooks(nil, time.UnixMilli(2), nil, target, 20)
	if heal != 1 {
		t.Fatalf("third heal=%v, want 1", heal)
	}
	if battleHasBuff(target, "bone_mend") {
		t.Fatalf("bone_mend should expire after third trigger, buffs=%#v", target.Buffs)
	}
	if len(logs) == 0 || logs[len(logs)-1]["kind"] != "state_detach" {
		t.Fatalf("expected state_detach log on third trigger, got %#v", logs)
	}
}

func TestBattleTriggerUnitBuffHooksSupportsTurnStartDamage(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "burning_mark",
			"durationTurns": 2,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "burning_mark_turn_start_damage",
			"buffId":   "burning_mark",
			"trigger":  "on_turn_start",
			"priority": 0,
			"effects": []map[string]any{
				{"kind": "damage", "damageType": "magical", "value": 7},
			},
		},
	}
	unit := &battleUnitState{
		UnitID: "enemy:burn",
		HP:     20,
		MaxHP:  20,
		Alive:  true,
		Stats:  map[string]float64{"hp": 20},
		Buffs: []battleBuffState{
			{BuffID: "burning_mark", RemainTurns: 2, SourceID: "player:caster"},
		},
	}

	actions := s.battleTriggerUnitBuffHooks(&battleRuntimeState{}, time.UnixMilli(0), unit, "on_turn_start")
	if unit.HP != 13 {
		t.Fatalf("hp=%v, want 13", unit.HP)
	}
	if !unit.Alive {
		t.Fatalf("unit should still be alive")
	}
	if len(actions) == 0 || actions[0]["kind"] != "damage" {
		t.Fatalf("actions=%#v, want damage action first", actions)
	}
	if got := actions[0]["sourceId"]; got != "player:caster" {
		t.Fatalf("sourceId=%v, want player:caster", got)
	}
}

func TestBattleTriggerUnitBuffHooksSupportsTurnStartDamageFormulaFromSource(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "burning_mark",
			"durationTurns": 2,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "burning_mark_turn_start_damage",
			"buffId":   "burning_mark",
			"trigger":  "on_turn_start",
			"priority": 0,
			"effects": []map[string]any{
				{
					"kind": "damage",
					"amountFormula": map[string]any{
						"baseAdd": 3,
						"terms": []map[string]any{
							{"attrKey": "strength", "coefficient": 0.05},
						},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID: "player:caster",
				Stats:  map[string]float64{"strength": 200},
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID: "enemy:burn",
				HP:     30,
				MaxHP:  30,
				Alive:  true,
				Stats:  map[string]float64{"hp": 30},
				Buffs: []battleBuffState{
					{BuffID: "burning_mark", RemainTurns: 2, SourceID: "player:caster"},
				},
			},
		},
	}

	unit := &state.Enemies[0]
	actions := s.battleTriggerUnitBuffHooks(state, time.UnixMilli(0), unit, "on_turn_start")
	if unit.HP != 17 {
		t.Fatalf("hp=%v, want 17", unit.HP)
	}
	if len(actions) == 0 || actions[0]["kind"] != "damage" {
		t.Fatalf("actions=%#v, want damage action", actions)
	}
}

func TestBattleTriggerUnitBuffHooksDamageFormulaFallsBackWithoutSource(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "burning_mark",
			"durationTurns": 2,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "burning_mark_turn_start_damage",
			"buffId":   "burning_mark",
			"trigger":  "on_turn_start",
			"priority": 0,
			"effects": []map[string]any{
				{
					"kind": "damage",
					"amountFormula": map[string]any{
						"terms": []map[string]any{
							{"attrKey": "strength", "coefficient": 0.05},
						},
					},
				},
			},
		},
	}
	unit := &battleUnitState{
		UnitID: "enemy:burn",
		HP:     20,
		MaxHP:  20,
		Alive:  true,
		Stats:  map[string]float64{"hp": 20},
		Buffs: []battleBuffState{
			{BuffID: "burning_mark", RemainTurns: 2},
		},
	}

	actions := s.battleTriggerUnitBuffHooks(&battleRuntimeState{}, time.UnixMilli(0), unit, "on_turn_start")
	if unit.HP != 20 {
		t.Fatalf("hp=%v, want 20", unit.HP)
	}
	for _, action := range actions {
		if action["kind"] == "damage" {
			t.Fatalf("actions=%#v, want no damage action", actions)
		}
	}
}

func TestBattleTriggerUnitBuffHooksCanMarkDamageSource(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "mirror_paw_guard",
			"durationTurns": 2,
			"maxCharges":    1,
			"stackPolicy":   "refresh_duration",
		},
		{
			"id":            "shadow_paw_mark",
			"durationTurns": 2,
			"stackPolicy":   "refresh_duration",
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":       "mirror_paw_guard_mark_source",
			"buffId":   "mirror_paw_guard",
			"trigger":  "on_after_take_damage",
			"priority": 0,
			"conditions": []map[string]any{
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{
				{"kind": "add_buff", "buffId": "shadow_paw_mark", "durationTurns": 2, "targetMode": "source"},
				{"kind": "consume_charge"},
			},
		},
		{
			"id":      "shadow_paw_mark_slow",
			"buffId":  "shadow_paw_mark",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": "add_stat", "attr": "speed", "value": -7},
			},
		},
	}
	defender := &battleUnitState{
		UnitID: "enemy:mirror",
		HP:     100,
		MaxHP:  100,
		Alive:  true,
		Buffs: []battleBuffState{
			{BuffID: "mirror_paw_guard", RemainTurns: 2},
		},
	}
	attacker := &battleUnitState{
		UnitID:    "player:blade",
		HP:        100,
		MaxHP:     100,
		Alive:     true,
		BaseStats: map[string]float64{"hp": 100, "speed": 50},
		Stats:     map[string]float64{"hp": 100, "speed": 50},
	}

	actions := s.battleTriggerUnitBuffHooksWithSource(&battleRuntimeState{}, time.UnixMilli(0), defender, attacker, "on_after_take_damage")
	if !battleHasBuff(attacker, "shadow_paw_mark") {
		t.Fatalf("attacker should receive shadow_paw_mark, buffs=%#v", attacker.Buffs)
	}
	if battleHasBuff(defender, "mirror_paw_guard") {
		t.Fatalf("mirror_paw_guard should be consumed, buffs=%#v", defender.Buffs)
	}
	if attacker.Stats["speed"] != 43 {
		t.Fatalf("attacker speed=%v, want 43", attacker.Stats["speed"])
	}
	foundBuffAction := false
	foundRecalcAction := false
	for _, action := range actions {
		switch action["kind"] {
		case "buff":
			foundBuffAction = action["targetId"] == "player:blade" && action["sourceId"] == "enemy:mirror"
		case "stat_recalc":
			if action["targetId"] == "player:blade" {
				foundRecalcAction = true
			}
		}
	}
	if !foundBuffAction {
		t.Fatalf("actions=%#v, want buff action targeting attacker", actions)
	}
	if !foundRecalcAction {
		t.Fatalf("actions=%#v, want stat_recalc for attacker", actions)
	}
}

func TestBattleRecomputeUnitPositionAdvancePositionIsTemporary(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "hook_dragged", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "hook_dragged_shift",
			"buffId":  "hook_dragged",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectAdvancePosition, "steps": 1},
			},
		},
	}
	unit := &battleUnitState{
		UnitID:       "enemy:1",
		Position:     "rearguard",
		BasePosition: "rearguard",
		Buffs: []battleBuffState{
			{BuffID: "hook_dragged", RemainTurns: 1},
		},
	}

	if !s.battleRecomputeUnitPosition(unit) {
		t.Fatalf("expected position to change while buff active")
	}
	if unit.Position != "midguard" {
		t.Fatalf("position=%s, want midguard", unit.Position)
	}

	changed, expired := tickBattleBuffDurations(unit)
	if !changed || len(expired) != 1 || expired[0] != "hook_dragged" {
		t.Fatalf("expected hook_dragged to expire after tick, changed=%v expired=%#v", changed, expired)
	}
	if !s.battleRecomputeUnitPosition(unit) {
		t.Fatalf("expected position to revert after buff expired")
	}
	if unit.Position != "rearguard" {
		t.Fatalf("position=%s, want rearguard after buff expiry", unit.Position)
	}
}

func TestBattleRecomputeUnitPositionAdvancePositionRespectsRows(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "hook_dragged", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "hook_dragged_shift",
			"buffId":  "hook_dragged",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectAdvancePosition, "steps": 1},
			},
		},
	}
	cases := []struct {
		name     string
		initial  string
		expected string
	}{
		{name: "rear_to_mid", initial: "rearguard", expected: "midguard"},
		{name: "mid_to_front", initial: "midguard", expected: "vanguard"},
		{name: "front_stays_front", initial: "vanguard", expected: "vanguard"},
	}
	for _, tc := range cases {
		unit := &battleUnitState{
			UnitID:       "enemy:1",
			Position:     tc.initial,
			BasePosition: tc.initial,
			Buffs: []battleBuffState{
				{BuffID: "hook_dragged", RemainTurns: 2},
			},
		}
		s.battleRecomputeUnitPosition(unit)
		if unit.Position != tc.expected {
			t.Fatalf("%s: position=%s, want %s", tc.name, unit.Position, tc.expected)
		}
	}
}

func TestBattleRecomputeUnitPositionSideAndPerspectiveAreTemporary(t *testing.T) {
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
	unit := &battleUnitState{
		UnitID:          "player:1",
		Side:            "player",
		OriginSide:      "player",
		BaseSide:        "player",
		PerspectiveSide: "player",
		Position:        "midguard",
		BasePosition:    "midguard",
		Buffs: []battleBuffState{
			{BuffID: "left_dagger_shadow_cross", RemainTurns: 1},
			{BuffID: "left_dagger_betrayal", RemainTurns: 1},
		},
	}

	if !s.battleRecomputeUnitPosition(unit) {
		t.Fatalf("expected side/perspective/position to change while buffs active")
	}
	if unit.Side != "enemy" {
		t.Fatalf("side=%s, want enemy", unit.Side)
	}
	if unit.PerspectiveSide != "player" {
		t.Fatalf("perspectiveSide=%s, want player", unit.PerspectiveSide)
	}
	if unit.Position != "rearguard" {
		t.Fatalf("position=%s, want rearguard", unit.Position)
	}

	changed, expired := tickBattleBuffDurations(unit)
	if !changed || len(expired) != 2 {
		t.Fatalf("expected both buffs to expire, changed=%v expired=%#v", changed, expired)
	}
	if !s.battleRecomputeUnitPosition(unit) {
		t.Fatalf("expected side/perspective/position to revert after buff expiry")
	}
	if unit.Side != "player" {
		t.Fatalf("side=%s, want player", unit.Side)
	}
	if unit.PerspectiveSide != "player" {
		t.Fatalf("perspectiveSide=%s, want player", unit.PerspectiveSide)
	}
	if unit.Position != "midguard" {
		t.Fatalf("position=%s, want midguard", unit.Position)
	}
}

func TestBattleTargetRelationUsesPerspectiveSide(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:          "player:1",
				Side:            "enemy",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "enemy",
				Alive:           true,
				HP:              100,
			},
			{
				UnitID:          "player:2",
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              100,
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:1",
				Side:            "enemy",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Alive:           true,
				HP:              100,
			},
		},
	}

	actor := &state.Players[0]
	if relation := s.battleTargetRelationForActor(state, actor, &state.Players[1]); relation != "enemy" {
		t.Fatalf("relation to player ally=%s, want enemy", relation)
	}
	if relation := s.battleTargetRelationForActor(state, actor, &state.Enemies[0]); relation != "ally" {
		t.Fatalf("relation to enemy=%s, want ally", relation)
	}
}

func TestBattleRecomputeUnitPositionSupportsSideAndPerspectiveOverride(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "shadow_cross", "durationTurns": 2},
		{"id": "betrayal", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "shadow_cross_while_active",
			"buffId":  "shadow_cross",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectSideOverride, "side": "enemy"},
			},
		},
		{
			"id":      "betrayal_while_active",
			"buffId":  "betrayal",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": battleEffectInvertRelationView},
			},
		},
	}
	unit := &battleUnitState{
		UnitID:          "player:1",
		Side:            "player",
		OriginSide:      "player",
		BaseSide:        "player",
		PerspectiveSide: "player",
		Position:        "midguard",
		BasePosition:    "midguard",
		Buffs: []battleBuffState{
			{BuffID: "shadow_cross", RemainTurns: 2},
			{BuffID: "betrayal", RemainTurns: 2},
		},
	}
	if !s.battleRecomputeUnitPosition(unit) {
		t.Fatalf("expected recompute to apply side/perspective overrides")
	}
	if unit.Side != "enemy" {
		t.Fatalf("side=%s, want enemy", unit.Side)
	}
	if unit.PerspectiveSide != "player" {
		t.Fatalf("perspectiveSide=%s, want player", unit.PerspectiveSide)
	}
	unit.Buffs = nil
	if !s.battleRecomputeUnitPosition(unit) {
		t.Fatalf("expected recompute to restore side/perspective after buffs removed")
	}
	if unit.Side != "player" {
		t.Fatalf("side=%s, want player after restore", unit.Side)
	}
	if unit.PerspectiveSide != "player" {
		t.Fatalf("perspectiveSide=%s, want player after restore", unit.PerspectiveSide)
	}
}

func TestBattleTargetRelationRespectsPerspectiveFlip(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "betrayal", "durationTurns": 2},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "betrayal_while_active",
			"buffId":  "betrayal",
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
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
				Buffs: []battleBuffState{
					{BuffID: "betrayal", RemainTurns: 2},
				},
			},
			{
				UnitID:          "player:2",
				Side:            "player",
				OriginSide:      "player",
				BaseSide:        "player",
				PerspectiveSide: "player",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:          "enemy:1",
				Side:            "enemy",
				OriginSide:      "enemy",
				BaseSide:        "enemy",
				PerspectiveSide: "enemy",
				Alive:           true,
				HP:              100,
				MaxHP:           100,
			},
		},
	}
	s.battleRecomputeUnitPosition(&state.Players[0])
	if got := s.battleTargetRelationForActor(state, &state.Players[0], &state.Players[1]); got != "enemy" {
		t.Fatalf("betrayed actor -> ally relation=%s, want enemy", got)
	}
	if got := s.battleTargetRelationForActor(state, &state.Players[0], &state.Enemies[0]); got != "ally" {
		t.Fatalf("betrayed actor -> enemy relation=%s, want ally", got)
	}
	if got := s.battleTargetRelationForActor(state, &state.Players[1], &state.Players[0]); got != "enemy" {
		t.Fatalf("normal ally -> betrayed relation=%s, want enemy", got)
	}
	if got := s.battleTargetRelationForActor(state, &state.Enemies[0], &state.Players[0]); got != "ally" {
		t.Fatalf("enemy -> betrayed relation=%s, want ally", got)
	}
}

func TestBattleAliveChecksUseOriginSide(t *testing.T) {
	state := &battleRuntimeState{
		Players: []battleUnitState{
			{
				UnitID:     "player:1",
				UserID:     1,
				Side:       "player",
				OriginSide: "player",
				Alive:      false,
				HP:         0,
				MaxHP:      100,
			},
		},
		Enemies: []battleUnitState{
			{
				UnitID:     "player:2",
				UserID:     2,
				Side:       "enemy",
				OriginSide: "player",
				Alive:      true,
				HP:         50,
				MaxHP:      100,
			},
			{
				UnitID:     "enemy:1",
				Side:       "player",
				OriginSide: "enemy",
				Alive:      true,
				HP:         50,
				MaxHP:      100,
			},
		},
	}
	if !battlePlayerMembersAlive(state) {
		t.Fatalf("player members should be alive based on origin side + user id")
	}
	if !battleTeamAlive(state, "enemy") {
		t.Fatalf("enemy team should be alive based on origin side even when migrated")
	}
}

func TestBattleRecomputeUnitStatsAppliesAndRemovesAddStat(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "armor_break_runtime_test",
			"durationTurns": 1,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "armor_break_runtime_test_while_active",
			"buffId":  "armor_break_runtime_test",
			"trigger": "while_active",
			"effects": []map[string]any{
				{"kind": "add_stat", "attr": "physics_percent_defense_penetration", "value": 0.5},
			},
		},
	}

	unit := &battleUnitState{
		UnitID: "player:1",
		Alive:  true,
		HP:     100,
		MaxHP:  100,
		Stats:  map[string]float64{"hp": 100},
		Buffs:  []battleBuffState{{BuffID: "armor_break_runtime_test", RemainTurns: 1}},
	}
	s.battleRecomputeUnitStats(unit)
	if roundTo4(unit.Stats["physics_percent_defense_penetration"]) != 0.5 {
		t.Fatalf("while_active add_stat not applied, got=%v stats=%#v delta=%#v", unit.Stats["physics_percent_defense_penetration"], unit.Stats, unit.StatModifiers)
	}

	changed, expired := tickBattleBuffDurations(unit)
	if !changed || len(expired) != 1 || expired[0] != "armor_break_runtime_test" {
		t.Fatalf("expected armor_break_runtime_test to expire, changed=%v expired=%#v", changed, expired)
	}
	s.battleRecomputeUnitStats(unit)
	if roundTo4(unit.Stats["physics_percent_defense_penetration"]) != 0 {
		t.Fatalf("add_stat should be removed after buff expiry, got=%v stats=%#v delta=%#v", unit.Stats["physics_percent_defense_penetration"], unit.Stats, unit.StatModifiers)
	}
}

func TestBattleBuffAddSkillCooldownSkipsBasicByDefault(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id":            "smoke_lock_runtime_test",
			"durationTurns": 1,
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "smoke_lock_runtime_test_attach",
			"buffId":  "smoke_lock_runtime_test",
			"trigger": "on_attach",
			"effects": []map[string]any{
				{"kind": "add_skill_cooldown", "value": 2},
			},
		},
	}
	unit := &battleUnitState{
		UnitID: "enemy:1",
		Alive:  true,
		HP:     100,
		MaxHP:  100,
		Skills: []battleSkillState{
			{ID: "power_strike", CooldownTurns: 3, CooldownRemain: 1},
			{ID: "basic_attack", CooldownTurns: 0, CooldownRemain: 0},
		},
		Buffs: []battleBuffState{{BuffID: "smoke_lock_runtime_test", RemainTurns: 1}},
	}
	state := &battleRuntimeState{Enemies: []battleUnitState{*unit}}

	rules := s.battleFindBuffRules("smoke_lock_runtime_test", "on_attach")
	if len(rules) != 1 || len(rules[0].EffectKinds) != 1 || rules[0].EffectKinds[0] != "addskillcooldown" {
		t.Fatalf("cooldown rule was not parsed correctly: %#v", rules)
	}
	if len(rules[0].Effects) != 1 || toInt64Loose(firstNonNil(rules[0].Effects[0]["turns"], rules[0].Effects[0]["value"], rules[0].Effects[0]["amount"]), 0) != 2 {
		t.Fatalf("cooldown effect payload was not parsed correctly: %#v", rules[0].Effects)
	}
	probeUnit := *unit
	probeUnit.Skills = append([]battleSkillState(nil), unit.Skills...)
	probeUnit.Buffs = append([]battleBuffState(nil), unit.Buffs...)
	if action, ok := s.battleApplySkillCooldownBuffEffect(&probeUnit, &probeUnit.Buffs[0], rules[0].Effects[0], rules[0].ID, "on_attach", 0); !ok || action["kind"] != "skill_cooldown" {
		t.Fatalf("cooldown helper did not apply: ok=%v basic=%q effect=%#v action=%#v unit=%#v", ok, s.battleConfiguredDefaultBasicSkillID(), rules[0].Effects[0], action, probeUnit)
	}
	actions := s.battleTriggerUnitBuffHooks(state, time.Unix(0, 0), unit, "on_attach", "smoke_lock_runtime_test")
	if !slices.ContainsFunc(actions, func(action map[string]any) bool {
		return action["kind"] == "skill_cooldown"
	}) {
		t.Fatalf("expected skill cooldown action, got %#v", actions)
	}
	if got := unit.Skills[0].CooldownRemain; got != 3 {
		t.Fatalf("non-basic cooldown remain = %d, want 3; actions=%#v", got, actions)
	}
	if got := unit.Skills[1].CooldownRemain; got != 0 {
		t.Fatalf("basic attack cooldown should be untouched, got %d", got)
	}
}

func TestBattleAdjustSkillCooldownSupportsSignedAmountsAndClampsAtZero(t *testing.T) {
	s := &Server{}
	unit := battleUnitState{
		UnitID: "player:1",
		Skills: []battleSkillState{
			{ID: "spell_a", CooldownRemain: 3},
			{ID: "spell_b", CooldownRemain: 0},
			{ID: "cooldown_correction", CooldownRemain: 5},
			{ID: "basic_attack", CooldownRemain: 0},
		},
	}
	action, ok := s.battleAdjustUnitSkillCooldowns(
		&unit, -1, false, nil, []string{"cooldown_correction"}, "player:1", "test", 0, nil,
	)
	if !ok || action["amount"] != -1 {
		t.Fatalf("signed cooldown adjustment failed: ok=%v action=%#v", ok, action)
	}
	if unit.Skills[0].CooldownRemain != 2 || unit.Skills[1].CooldownRemain != 0 || unit.Skills[2].CooldownRemain != 5 || unit.Skills[3].CooldownRemain != 0 {
		t.Fatalf("unexpected cooldown state: %#v", unit.Skills)
	}

	action, ok = s.battleAdjustUnitSkillCooldowns(&unit, 1, false, nil, nil, "battlefield:test", "test", 0, nil)
	if !ok || action["amount"] != 1 || unit.Skills[0].CooldownRemain != 3 || unit.Skills[1].CooldownRemain != 1 || unit.Skills[3].CooldownRemain != 0 {
		t.Fatalf("positive cooldown adjustment must include ready non-basic skills: action=%#v skills=%#v", action, unit.Skills)
	}
}

func TestBattleRecomputeUnitStatsRecoversBaseFromStatModifiers(t *testing.T) {
	s := &Server{}
	unit := &battleUnitState{
		UnitID: "player:carry",
		Alive:  true,
		HP:     100,
		MaxHP:  100,
		// Simulate a resumed state where temporary penetration was folded into Stats.
		Stats: map[string]float64{
			"hp":                                  100,
			"physics_percent_defense_penetration": 0.5,
		},
		StatModifiers: map[string]float64{
			"physics_percent_defense_penetration": 0.5,
		},
		BaseStats: nil,
		Buffs:     nil,
	}
	s.battleRecomputeUnitStats(unit)
	if roundTo4(unit.Stats["physics_percent_defense_penetration"]) != 0 {
		t.Fatalf("expected recovered base stats to clear stale penetration, got=%v stats=%#v base=%#v delta=%#v", unit.Stats["physics_percent_defense_penetration"], unit.Stats, unit.BaseStats, unit.StatModifiers)
	}
}

func TestBattleBuffMetadataCachesPreserveLegacyLookupSemantics(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		nil,
		{"id": "<nil>", "durationTurns": 99, "stackPolicy": "stack", "maxStacks": 99},
		{"id": " Permanent ", "permanent": "true", "durationTurns": 9, "stackPolicy": "stack_count", "maxStacks": 4},
		{"id": "limited", "duration": "3", "stackPolicy": "unexpected", "stackLimit": "5"},
		{"id": "non_positive", "durationTurns": -2, "stackPolicy": "stack", "maxStacks": -4},
		{"id": "duplicate", "durationTurns": 2, "stackPolicy": "refresh_duration", "maxStacks": 1},
		{"id": "DUPLICATE", "durationTurns": 8, "stackPolicy": "stack", "maxStacks": 8},
	}

	tests := []struct {
		id          string
		duration    int64
		stackPolicy string
		maxStacks   int
	}{
		{id: "<nil>", duration: 0, stackPolicy: "refresh_duration", maxStacks: 0},
		{id: " permanent ", duration: -1, stackPolicy: "stack", maxStacks: 4},
		{id: "LIMITED", duration: 3, stackPolicy: "refresh_duration", maxStacks: 5},
		{id: "non_positive", duration: 0, stackPolicy: "stack", maxStacks: 0},
		{id: "duplicate", duration: 2, stackPolicy: "refresh_duration", maxStacks: 1},
		{id: "missing", duration: 0, stackPolicy: "refresh_duration", maxStacks: 0},
		{id: "", duration: 0, stackPolicy: "refresh_duration", maxStacks: 0},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			if got := s.battleBuffDurationByIDTurns(test.id); got != test.duration {
				t.Fatalf("duration=%d, want %d", got, test.duration)
			}
			if got := s.battleBuffStackPolicyByID(test.id); got != test.stackPolicy {
				t.Fatalf("stack policy=%q, want %q", got, test.stackPolicy)
			}
			if got := s.battleBuffMaxStacksByID(test.id); got != test.maxStacks {
				t.Fatalf("max stacks=%d, want %d", got, test.maxStacks)
			}
		})
	}
}

func TestBattleRecomputeUnitStatsReplacesStatSourcesWithoutAliasing(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id": "stat_source_runtime_test", "buffId": "stat_source_runtime_test", "trigger": "while_active",
		"effects": []map[string]any{{"kind": "add_stat", "attr": "attack", "value": 5}},
	}}
	unit := &battleUnitState{
		UnitID: "player:1", Alive: true, HP: 100, MaxHP: 100,
		Stats: map[string]float64{"hp": 100, "attack": 10},
		Buffs: []battleBuffState{{BuffID: "stat_source_runtime_test", RemainTurns: 1}},
	}

	s.battleRecomputeUnitStats(unit)
	previous := unit.StatSources
	if got := previous["attack"]["stat_source_runtime_test"]; got != 5 {
		t.Fatalf("initial stat source=%v, want 5", got)
	}
	unit.Buffs = nil
	s.battleRecomputeUnitStats(unit)
	if len(unit.StatSources) != 0 {
		t.Fatalf("recomputed stat sources=%#v, want empty", unit.StatSources)
	}
	unit.StatSources["attack"] = map[string]float64{"mutated": 1}
	if got := previous["attack"]["stat_source_runtime_test"]; got != 5 {
		t.Fatalf("new stat source mutation leaked into previous state, got %v", got)
	}
}

func BenchmarkBattleRecomputeUnitStats(b *testing.B) {
	s := &Server{}
	unit := &battleUnitState{
		UnitID:    "player:benchmark",
		Alive:     true,
		HP:        1000,
		MaxHP:     1000,
		Stats:     map[string]float64{"hp": 1000, "speed": 100, "attack": 200},
		BaseStats: map[string]float64{"hp": 1000, "speed": 100, "attack": 200},
	}
	for idx := 0; idx < 8; idx++ {
		buffID := "benchmark_buff_" + string(rune('a'+idx))
		s.deps.GameData.Buff = append(s.deps.GameData.Buff, map[string]any{
			"id": buffID, "durationTurns": 3, "stackPolicy": "refresh_duration",
		})
		s.deps.GameData.BuffEffects = append(s.deps.GameData.BuffEffects, map[string]any{
			"id": buffID + "_while_active", "buffId": buffID, "trigger": "while_active",
			"effects": []map[string]any{{"kind": "add_stat", "attr": "attack", "value": float64(idx + 1)}},
		})
		unit.Buffs = append(unit.Buffs, battleBuffState{BuffID: buffID, RemainTurns: 3})
	}
	s.battleParseBuffRules()
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		s.battleRecomputeUnitStats(unit)
	}
}

func BenchmarkBattleBuffMetadataLookupCached(b *testing.B) {
	s, targetID := benchmarkBattleBuffMetadataServer(512)
	s.battleParseBuffRules()
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		_, _, _ = s.battleBuffDurationByIDTurns(targetID), s.battleBuffStackPolicyByID(targetID), s.battleBuffMaxStacksByID(targetID)
	}
}

func BenchmarkBattleBuffMetadataLookupLegacyLinear(b *testing.B) {
	s, targetID := benchmarkBattleBuffMetadataServer(512)
	b.ReportAllocs()
	b.ResetTimer()
	for idx := 0; idx < b.N; idx++ {
		_, _, _ = legacyBattleBuffMetadataLookup(s.deps.GameData.Buff, targetID)
	}
}

func benchmarkBattleBuffMetadataServer(count int) (*Server, string) {
	s := &Server{}
	for idx := 0; idx < count; idx++ {
		id := fmt.Sprintf("benchmark_buff_%03d", idx)
		s.deps.GameData.Buff = append(s.deps.GameData.Buff, map[string]any{
			"id": id, "durationTurns": 3, "stackPolicy": "stack", "maxStacks": 5,
		})
	}
	return s, fmt.Sprintf("benchmark_buff_%03d", count-1)
}

func legacyBattleBuffMetadataLookup(rows []map[string]any, buffID string) (int64, string, int) {
	duration := int64(0)
	for _, raw := range rows {
		if strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", raw["id"]))) != buffID {
			continue
		}
		if parseBooleanLoose(raw["permanent"], false) {
			duration = -1
		} else if turns := toInt64Loose(raw["durationTurns"], toInt64Loose(raw["duration"], 0)); turns > 0 {
			duration = turns
		}
		break
	}
	stackPolicy := "refresh_duration"
	for _, raw := range rows {
		if strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", raw["id"]))) == buffID {
			stackPolicy = battleNormalizeBuffStackPolicy(fmt.Sprintf("%v", raw["stackPolicy"]))
			break
		}
	}
	maxStacks := 0
	for _, raw := range rows {
		if strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", raw["id"]))) == buffID {
			maxStacks = maxInt(0, int(toInt64Loose(firstNonNil(raw["maxStacks"], raw["stackLimit"]), 0)))
			break
		}
	}
	return duration, stackPolicy, maxStacks
}
