package httpapi

import (
	"math"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
)

func TestAmusementParadeCurtainAdvancesAcrossOrderedTurns(t *testing.T) {
	s := newAmusementMechanicsTestServer()
	boss := amusementMechanicsTestUnit("enemy:parade-king", "enemy")
	boss.Buffs = []battleBuffState{{
		BuffID:      "parade_three_act_curtain",
		RemainTurns: 99,
		SourceID:    boss.UnitID,
	}}
	state := &battleRuntimeState{
		Seed:    1,
		Wave:    1,
		Enemies: []battleUnitState{boss},
	}

	s.executeUnitTurn(state, time.UnixMilli(1), &state.Enemies[0], "enemy", false)
	curtain := findBuffByID(&state.Enemies[0], "parade_three_act_curtain")
	if got := battleBuffChargesRemaining(curtain, 3); got != 2 {
		t.Fatalf("curtain charges after first ordered turn = %d, want 2; buff=%#v", got, curtain)
	}
	if findBuffByID(&state.Enemies[0], "parade_second_act") == nil {
		t.Fatal("first ordered turn did not open the second act")
	}
	if findBuffByID(&state.Enemies[0], "parade_finale") != nil {
		t.Fatal("first ordered turn opened the finale too early")
	}
	assertAmusementFloat(t, "first-act speed", state.Enemies[0].Stats["speed"], 25)
	assertAmusementFloat(t, "first-act physical amplification", state.Enemies[0].Stats["physics_damage_amplification"], 0.15)

	s.executeUnitTurn(state, time.UnixMilli(2), &state.Enemies[0], "enemy", false)
	curtain = findBuffByID(&state.Enemies[0], "parade_three_act_curtain")
	if got := battleBuffChargesRemaining(curtain, 3); got != 1 {
		t.Fatalf("curtain charges after second ordered turn = %d, want 1; buff=%#v", got, curtain)
	}
	if findBuffByID(&state.Enemies[0], "parade_second_act") == nil || findBuffByID(&state.Enemies[0], "parade_finale") == nil {
		t.Fatalf("second ordered turn should overlap second act and finale: %#v", state.Enemies[0].Buffs)
	}
	assertAmusementFloat(t, "overlapped physical amplification", state.Enemies[0].Stats["physics_damage_amplification"], 0.40)
	assertAmusementFloat(t, "overlapped magical amplification", state.Enemies[0].Stats["magical_damage_amplification"], 0.40)

	s.executeUnitTurn(state, time.UnixMilli(3), &state.Enemies[0], "enemy", false)
	if curtain = findBuffByID(&state.Enemies[0], "parade_three_act_curtain"); curtain != nil {
		t.Fatalf("third ordered turn should exhaust the curtain: %#v", curtain)
	}
	vulnerability := findBuffByID(&state.Enemies[0], "amusement_curtain_fall_vulnerability")
	if vulnerability == nil || vulnerability.RemainTurns != 1 {
		t.Fatalf("third ordered turn should open a full one-turn vulnerability window: %#v", vulnerability)
	}
	if findBuffByID(&state.Enemies[0], "parade_second_act") == nil || findBuffByID(&state.Enemies[0], "parade_finale") == nil {
		t.Fatalf("phase buffs should still overlap when the curtain falls: %#v", state.Enemies[0].Buffs)
	}
	if got := state.CompletedTurnCounts[boss.UnitID]; got != 3 {
		t.Fatalf("completed ordered turns = %d, want 3", got)
	}
}

func TestAmusementCurtainSoftensDamageAndStillSpotlights(t *testing.T) {
	s := newAmusementMechanicsTestServer()
	attacker := amusementMechanicsTestUnit("player:attacker", "player")
	defender := amusementMechanicsTestUnit("enemy:healer", "enemy")
	defender.Buffs = []battleBuffState{
		{BuffID: "amusement_opening_curtain", RemainTurns: 99, SourceID: defender.UnitID},
		{BuffID: "starlight_backstage_pass_buff", RemainTurns: 2, SourceID: defender.UnitID},
	}
	state := &battleRuntimeState{
		Seed:    1,
		Wave:    1,
		Players: []battleUnitState{attacker},
		Enemies: []battleUnitState{defender},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)

	effect := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   attacker.UnitID,
		TargetID:   defender.UnitID,
		Amount:     100,
		DamageType: "physical",
		Reason:     "curtain_contract_test",
	})

	assertAmusementFloat(t, "curtain damage", effect.Amount, 20)
	assertAmusementFloat(t, "defender hp", state.Enemies[0].HP, 80)
	spotlight := findBuffByID(&state.Players[0], "amusement_spotlight")
	if spotlight == nil || spotlight.RemainTurns != 1 || spotlight.SourceID != defender.UnitID {
		t.Fatalf("curtain hit did not spotlight its source: %#v", spotlight)
	}
	curtain := findBuffByID(&state.Enemies[0], "amusement_opening_curtain")
	if curtain == nil || curtain.Triggered != 0 {
		t.Fatalf("taking a hit should not spend the turn-based curtain: %#v", curtain)
	}
	backstage := findBuffByID(&state.Enemies[0], "starlight_backstage_pass_buff")
	if backstage != nil {
		t.Fatalf("the lower-priority backstage pass should mitigate and be spent after curtain softens the hit: %#v", backstage)
	}
}

func TestAmusementCurtainReductionOpensByRemainingActs(t *testing.T) {
	tests := []struct {
		name      string
		triggered int64
		want      float64
	}{
		{name: "three acts", triggered: 0, want: 1},
		{name: "two acts", triggered: 1, want: 35},
		{name: "final act", triggered: 2, want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAmusementMechanicsTestServer()
			attacker := amusementMechanicsTestUnit("player:attacker", "player")
			defender := amusementMechanicsTestUnit("enemy:parade-king", "enemy")
			defender.Buffs = []battleBuffState{{
				BuffID:      "parade_three_act_curtain",
				RemainTurns: 99,
				SourceID:    defender.UnitID,
				Triggered:   tt.triggered,
			}}
			state := &battleRuntimeState{
				Seed:    1,
				Wave:    1,
				Players: []battleUnitState{attacker},
				Enemies: []battleUnitState{defender},
			}
			runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)

			effect := runtime.ApplyDamage(battleskills.DamageAction{
				SourceID:   attacker.UnitID,
				TargetID:   defender.UnitID,
				Amount:     100,
				DamageType: "physical",
				Reason:     "layered_curtain_contract_test",
			})

			assertAmusementFloat(t, "curtain damage", effect.Amount, tt.want)
			curtain := findBuffByID(&state.Enemies[0], "parade_three_act_curtain")
			if curtain == nil || curtain.Triggered != tt.triggered {
				t.Fatalf("taking damage should not advance the curtain: %#v", curtain)
			}
		})
	}
}

func TestAmusementCurtainDispelPeelsOneAct(t *testing.T) {
	s := newAmusementMechanicsTestServer()
	actor := amusementMechanicsTestUnit("player:dispel", "player")
	target := amusementMechanicsTestUnit("enemy:parade-king", "enemy")
	target.Buffs = []battleBuffState{{
		BuffID:      "parade_three_act_curtain",
		RemainTurns: 99,
		SourceID:    target.UnitID,
	}}
	state := &battleRuntimeState{
		Seed:    1,
		Wave:    1,
		Players: []battleUnitState{actor},
		Enemies: []battleUnitState{target},
	}
	state.Players[0].SpecializationHooks = []battleEquipmentHookRule{
		specializationTestHook("curtain-dispel", "onAfterDealDamage", "curtain-dispel-node", map[string]any{
			"kind": "remove_buff_by_kind", "buffKind": "buff", "selectionMode": "max_remaining", "maxBuffs": 1,
		}),
	}

	for wantRemaining := int64(2); wantRemaining >= 1; wantRemaining-- {
		actions := s.battleTriggerUnitEquipmentHooksWithContext(
			state,
			time.UnixMilli(3-wantRemaining),
			&state.Players[0],
			&state.Players[0],
			&state.Enemies[0],
			"onAfterDealDamage",
			10,
			"physical",
			&battleEquipmentHookContext{SkillID: "basic_attack"},
		)
		curtain := findBuffByID(&state.Enemies[0], "parade_three_act_curtain")
		if got := battleBuffChargesRemaining(curtain, 3); got != wantRemaining {
			t.Fatalf("remaining curtain acts = %d, want %d; buff=%#v", got, wantRemaining, curtain)
		}
		var chargeAction map[string]any
		for _, action := range actions {
			if action["kind"] == "buff_charge" {
				chargeAction = action
				break
			}
		}
		if chargeAction == nil || toInt64Loose(chargeAction["buffTriggered"], 0) != 3-wantRemaining {
			t.Fatalf("partial dispel action mismatch: %#v", actions)
		}
	}

	actions := s.battleTriggerUnitEquipmentHooksWithContext(
		state,
		time.UnixMilli(3),
		&state.Players[0],
		&state.Players[0],
		&state.Enemies[0],
		"onAfterDealDamage",
		10,
		"physical",
		&battleEquipmentHookContext{SkillID: "basic_attack"},
	)
	if curtain := findBuffByID(&state.Enemies[0], "parade_three_act_curtain"); curtain != nil {
		t.Fatalf("third dispel should remove the final curtain act: %#v", curtain)
	}
	detached := false
	for _, action := range actions {
		if action["kind"] == "state_detach" {
			detached = true
			break
		}
	}
	if !detached {
		t.Fatalf("final dispel action mismatch: %#v", actions)
	}
	if vulnerability := findBuffByID(&state.Enemies[0], "amusement_curtain_fall_vulnerability"); vulnerability != nil {
		t.Fatalf("active dispel should not create the natural-fall vulnerability: %#v", vulnerability)
	}
}

func TestAmusementPlushRedirectBypassesSourceCurtain(t *testing.T) {
	s := newAmusementMechanicsTestServer()
	attacker := amusementMechanicsTestUnit("player:attacker", "player")
	protected := amusementMechanicsTestUnit("enemy:protected", "enemy")
	guard := amusementMechanicsTestUnit("enemy:plush-guard", "enemy")
	protected.Buffs = []battleBuffState{{
		BuffID:      "plush_guard_cover_link",
		RemainTurns: 2,
		SourceID:    guard.UnitID,
	}}
	guard.Buffs = []battleBuffState{{
		BuffID:      "amusement_opening_curtain",
		RemainTurns: 99,
		SourceID:    guard.UnitID,
	}}
	state := &battleRuntimeState{
		Seed:    1,
		Wave:    1,
		Players: []battleUnitState{attacker},
		Enemies: []battleUnitState{protected, guard},
	}
	runtime := newBattleSkillActionRuntime(state, &state.Players[0], time.UnixMilli(1), s, nil)

	effect := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID:   attacker.UnitID,
		TargetID:   protected.UnitID,
		Amount:     100,
		DamageType: "physical",
		Reason:     "plush_redirect_contract_test",
	})

	assertAmusementFloat(t, "protected damage after cover and redirect", effect.Amount, 38.25)
	assertAmusementFloat(t, "protected hp", state.Enemies[0].HP, 61.75)
	assertAmusementFloat(t, "guard hp", state.Enemies[1].HP, 53.25)
	curtain := findBuffByID(&state.Enemies[1], "amusement_opening_curtain")
	if curtain == nil || curtain.Triggered != 0 {
		t.Fatalf("redirected damage should bypass the guard's own curtain: %#v", curtain)
	}

	redirectFound := false
	for _, row := range runtime.ActionLog() {
		if row["kind"] != "damage_redirect" {
			continue
		}
		redirectFound = true
		if row["targetId"] != guard.UnitID {
			t.Fatalf("redirect target = %v, want %s", row["targetId"], guard.UnitID)
		}
		assertAmusementFloat(t, "redirect action amount", toFloat64(row["amount"], 0), 46.75)
	}
	if !redirectFound {
		t.Fatalf("damage redirect action missing: %#v", runtime.ActionLog())
	}
}

func newAmusementMechanicsTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{"id": "amusement_opening_curtain", "durationTurns": 99, "maxCharges": 1, "kind": "buff", "dispellable": true, "stackPolicy": "refresh_duration"},
		{"id": "amusement_vip_curtain", "durationTurns": 99, "maxCharges": 2, "kind": "buff", "dispellable": true, "stackPolicy": "refresh_duration"},
		{"id": "parade_three_act_curtain", "durationTurns": 99, "maxCharges": 3, "kind": "buff", "dispellable": true, "stackPolicy": "refresh_duration"},
		{"id": "amusement_curtain_fall_vulnerability", "durationTurns": 1, "stackPolicy": "refresh_duration"},
		{"id": "amusement_spotlight", "durationTurns": 1, "stackPolicy": "refresh_duration"},
		{"id": "plush_guard_cover_link", "durationTurns": 2, "stackPolicy": "refresh_duration"},
		{"id": "starlight_backstage_pass_buff", "durationTurns": 2, "maxCharges": 1, "stackPolicy": "refresh_duration"},
		{"id": "parade_second_act", "durationTurns": 3, "stackPolicy": "refresh_duration"},
		{"id": "parade_finale", "durationTurns": 3, "stackPolicy": "refresh_duration"},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id": "amusement_opening_curtain_block", "buffId": "amusement_opening_curtain",
			"trigger": "on_before_take_damage", "priority": 1000,
			"conditions": []map[string]any{
				{"type": "incoming_damage_gte", "value": 1},
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{{"kind": "set_damage_multiplier", "value": 0.5}},
		},
		{
			"id": "amusement_opening_curtain_spotlight_source", "buffId": "amusement_opening_curtain",
			"trigger": "on_after_take_damage", "priority": 0, "maxTriggersPerTurn": 1,
			"conditions": []map[string]any{{"type": "self_charges_remaining_gte", "value": 1}},
			"effects": []map[string]any{
				{"kind": "add_buff", "buffId": "amusement_spotlight", "durationTurns": 1, "targetMode": "source"},
			},
		},
		{
			"id": "parade_three_act_curtain_block", "buffId": "parade_three_act_curtain",
			"trigger": "on_before_take_damage", "priority": 1000,
			"conditions": []map[string]any{
				{"type": "incoming_damage_gte", "value": 1},
				{"type": "self_charges_remaining_gte", "value": 3},
				{"type": "self_charges_remaining_lte", "value": 3},
			},
			"effects": []map[string]any{{"kind": "set_damage_multiplier", "value": 0.01}},
		},
		{
			"id": "parade_three_act_curtain_block_second_act", "buffId": "parade_three_act_curtain",
			"trigger": "on_before_take_damage", "priority": 1000,
			"conditions": []map[string]any{
				{"type": "incoming_damage_gte", "value": 1},
				{"type": "self_charges_remaining_gte", "value": 2},
				{"type": "self_charges_remaining_lte", "value": 2},
			},
			"effects": []map[string]any{{"kind": "set_damage_multiplier", "value": 0.35}},
		},
		{
			"id": "parade_three_act_curtain_block_final_act", "buffId": "parade_three_act_curtain",
			"trigger": "on_before_take_damage", "priority": 1000,
			"conditions": []map[string]any{
				{"type": "incoming_damage_gte", "value": 1},
				{"type": "self_charges_remaining_gte", "value": 1},
				{"type": "self_charges_remaining_lte", "value": 1},
			},
			"effects": []map[string]any{{"kind": "set_damage_multiplier", "value": 0.5}},
		},
		{
			"id": "parade_three_act_curtain_fall", "buffId": "parade_three_act_curtain",
			"trigger": "on_turn_end", "priority": 30, "maxTriggersPerTurn": 1,
			"sharedTriggerGroup": "parade_three_act_curtain_step",
			"eventFilter": map[string]any{
				"turnKinds": []any{"ordered"}, "includePreparation": false, "includeSuppressedTurns": true,
			},
			"conditions": []map[string]any{{"type": "self_charges_remaining_lte", "value": 1}},
			"effects": []map[string]any{
				{"kind": "add_buff", "buffId": "amusement_curtain_fall_vulnerability", "durationTurns": 1, "targetMode": "self"},
				{"kind": "consume_charge"},
			},
		},
		{
			"id": "parade_three_act_curtain_open_finale", "buffId": "parade_three_act_curtain",
			"trigger": "on_turn_end", "priority": 20, "maxTriggersPerTurn": 1,
			"sharedTriggerGroup": "parade_three_act_curtain_step",
			"eventFilter": map[string]any{
				"turnKinds": []any{"ordered"}, "includePreparation": false, "includeSuppressedTurns": true,
			},
			"conditions": []map[string]any{
				{"type": "self_charges_remaining_gte", "value": 2},
				{"type": "self_charges_remaining_lte", "value": 2},
			},
			"effects": []map[string]any{
				{"kind": "add_buff", "buffId": "parade_finale", "durationTurns": 3, "targetMode": "self"},
				{"kind": "consume_charge"},
			},
		},
		{
			"id": "parade_three_act_curtain_open_second_act", "buffId": "parade_three_act_curtain",
			"trigger": "on_turn_end", "priority": 10, "maxTriggersPerTurn": 1,
			"sharedTriggerGroup": "parade_three_act_curtain_step",
			"eventFilter": map[string]any{
				"turnKinds": []any{"ordered"}, "includePreparation": false, "includeSuppressedTurns": true,
			},
			"conditions": []map[string]any{
				{"type": "self_charges_remaining_gte", "value": 3},
				{"type": "self_charges_remaining_lte", "value": 3},
			},
			"effects": []map[string]any{
				{"kind": "add_buff", "buffId": "parade_second_act", "durationTurns": 3, "targetMode": "self"},
				{"kind": "consume_charge"},
			},
		},
		{
			"id": "plush_guard_cover_link_while_active", "buffId": "plush_guard_cover_link",
			"trigger": "while_active", "priority": 0,
			"effects": []map[string]any{
				{"kind": "reduce_damage_percent", "value": 0.15},
				{"kind": "damage_redirect_to_source", "percent": 0.55},
			},
		},
		{
			"id": "starlight_backstage_pass_before_damage", "buffId": "starlight_backstage_pass_buff",
			"trigger": "on_before_take_damage", "priority": 20,
			"conditions": []map[string]any{
				{"type": "incoming_damage_gte", "value": 1},
				{"type": "self_charges_remaining_gte", "value": 1},
			},
			"effects": []map[string]any{
				{"kind": "reduce_damage_percent", "value": 0.6},
				{"kind": "consume_charge"},
			},
		},
		{
			"id": "parade_second_act_while_active", "buffId": "parade_second_act",
			"trigger": "while_active", "priority": 0,
			"effects": []map[string]any{
				{"kind": "add_stat", "attr": "speed", "value": 15},
				{"kind": "add_stat", "attr": "physics_damage_amplification", "value": 0.15},
				{"kind": "add_stat", "attr": "magical_damage_amplification", "value": 0.15},
			},
		},
		{
			"id": "parade_finale_while_active", "buffId": "parade_finale",
			"trigger": "while_active", "priority": 0,
			"effects": []map[string]any{
				{"kind": "add_stat", "attr": "physics_damage_amplification", "value": 0.25},
				{"kind": "add_stat", "attr": "magical_damage_amplification", "value": 0.25},
			},
		},
	}
	return s
}

func amusementMechanicsTestUnit(unitID, side string) battleUnitState {
	baseStats := map[string]float64{
		"hp":                           100,
		"mp":                           100,
		"speed":                        10,
		"physics_damage_amplification": 0,
		"magical_damage_amplification": 0,
	}
	return battleUnitState{
		UnitID:          unitID,
		Side:            side,
		OriginSide:      side,
		BaseSide:        side,
		PerspectiveSide: side,
		Alive:           true,
		HP:              100,
		MP:              100,
		MaxHP:           100,
		MaxMP:           100,
		BaseStats:       cloneFloatMap(baseStats),
		Stats:           cloneFloatMap(baseStats),
	}
}

func assertAmusementFloat(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}
