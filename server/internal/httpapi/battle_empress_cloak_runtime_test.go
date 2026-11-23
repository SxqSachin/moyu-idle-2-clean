package httpapi

import (
	"context"
	"testing"
	"time"
)

func newEmpressCloakTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{
		{
			"id": "empress_command", "durationTurns": 4, "stackPolicy": "refresh_duration",
		},
		{
			"id": "empress_cloak_enthroned", "durationTurns": 0, "permanent": true,
			"dispellable": false, "stackPolicy": "refresh_duration",
		},
	}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id": "empress_command_while_active", "buffId": "empress_command",
			"trigger": "while_active", "priority": 0,
			"effects": []map[string]any{
				{"kind": "add_stat_percent", "attr": "strength", "value": 0, "payloadValueKey": "mainStatPercent", "payloadValueCoeff": 1},
				{"kind": "add_stat_percent", "attr": "dexterity", "value": 0, "payloadValueKey": "mainStatPercent", "payloadValueCoeff": 1},
				{"kind": "add_stat_percent", "attr": "intelligence", "value": 0, "payloadValueKey": "mainStatPercent", "payloadValueCoeff": 1},
			},
		},
		{
			"id": "empress_cloak_enthroned_while_active", "buffId": "empress_cloak_enthroned",
			"trigger": "while_active", "priority": 100,
			"effects": []map[string]any{{"kind": "turn_ineligible"}},
		},
	}
	return s
}

func empressCloakUniqueWearerCondition() []map[string]any {
	return []map[string]any{{
		"type":   "player_party_equipped_unit_count_eq",
		"itemId": "empress_cloak",
		"value":  1,
	}}
}

func empressCloakTestHooks(enhanceLevel int) []battleEquipmentHookRule {
	return []battleEquipmentHookRule{
		{
			ID: "empress_cloak_enthrone", SourceKind: "equipment", SourceItemID: "empress_cloak",
			SourceEnhanceLevel: enhanceLevel, Trigger: "on_battle_start", Priority: 1000,
			MaxTriggersPerBattle: 1, Conditions: empressCloakUniqueWearerCondition(),
			Effects: []map[string]any{{
				"kind": "add_buff", "buffId": "empress_cloak_enthroned", "durationTurns": 0,
				"targetMode": "self", "dispatchHooks": false,
			}},
		},
		{
			ID: "empress_cloak_command", SourceKind: "equipment", SourceItemID: "empress_cloak",
			SourceEnhanceLevel: enhanceLevel, Trigger: "on_wave_start", Priority: 10,
			Conditions: empressCloakUniqueWearerCondition(),
			Effects: []map[string]any{{
				"kind": "add_buff", "buffId": "empress_command", "durationTurns": 4,
				"targetMode": "targeting",
				"targeting": map[string]any{
					"targetPositions": []any{"vanguard", "midguard", "rearguard"},
					"secondarySeek":   "none", "targetScope": "ally", "selectRule": "first",
					"fallbackSelfIfEmpty": false, "excludeSelf": true,
					"targetCount": map[string]any{"base": 999, "formula": []any{}, "max": 999},
				},
				"payloadFormula": map[string]any{
					"mainStatPercent": map[string]any{
						"baseAdd": 0.08,
						"terms": []any{map[string]any{
							"attrKey": "equipment_enhance_level", "coefficient": 0.002, "perSkillLevel": 0,
						}},
					},
				},
			}},
		},
	}
}

func empressCloakTestUnit(unitID, position string) battleUnitState {
	baseStats := map[string]float64{
		"hp": 100, "mp": 100, "strength": 100, "dexterity": 100, "intelligence": 100, "speed": 10,
	}
	return battleUnitState{
		UnitID: unitID, Side: "player", OriginSide: "player", BaseSide: "player", PerspectiveSide: "player",
		UnitType: "player", Position: position, BasePosition: position, Alive: true,
		HP: 100, MaxHP: 100, MP: 100, MaxMP: 100, StatMode: battleStatModeDirect,
		BaseStats: cloneFloatMap(baseStats), Stats: cloneFloatMap(baseStats),
	}
}

func countUnitBuff(unit *battleUnitState, buffID string) int {
	if unit == nil {
		return 0
	}
	count := 0
	for _, buff := range unit.Buffs {
		if buff.BuffID == buffID {
			count++
		}
	}
	return count
}

func TestEmpressCloakRequiresExactlyOneRealPlayerWearer(t *testing.T) {
	t.Run("one real wearer and an illusion copy", func(t *testing.T) {
		s := newEmpressCloakTestServer()
		owner := empressCloakTestUnit("player:empress", "vanguard")
		owner.EquipmentHooks = empressCloakTestHooks(0)
		ally := empressCloakTestUnit("player:ally", "midguard")
		illusion := empressCloakTestUnit("summon_empress_illusion_1", "rearguard")
		illusion.UnitType = "summon"
		illusion.SummonTemplateUnitID = "empress_illusion"
		illusion.SummonSourceUnitID = owner.UnitID
		illusion.EquipmentHooks = empressCloakTestHooks(0)
		state := &battleRuntimeState{
			Wave: 1, Players: []battleUnitState{owner, ally, illusion},
			PreparationOrder: []string{owner.UnitID, ally.UnitID},
		}

		if got := battlePlayerPartyEquippedUnitCount(state, "empress_cloak"); got != 1 {
			t.Fatalf("real equipped wearer count = %d, want 1", got)
		}
		if _, ok := s.executeEquipmentExtraTurn(state, time.UnixMilli(1), &state.Players[0], "test", battleEquipmentHookRule{ID: "test_extra_turn"}, nil, "full", false); ok {
			t.Fatal("wearer gained an extra turn before the opening passive attached")
		}

		s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(2))
		if !battleHasBuff(&state.Players[0], "empress_cloak_enthroned") || !state.Players[0].TurnIneligible {
			t.Fatalf("unique wearer did not become turn-ineligible: %#v", state.Players[0])
		}
		if battleHasBuff(&state.Players[2], "empress_cloak_enthroned") || state.Players[2].TurnIneligible {
			t.Fatal("illusion copy incorrectly received the wearer passive")
		}

		s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(3))
		if got := countUnitBuff(&state.Players[1], "empress_command"); got != 1 {
			t.Fatalf("ally command buff count = %d, want 1", got)
		}
		rebuilt := cloneBattleRuntimeState(*state)
		rebuilt.PreparationOrder = []string{owner.UnitID, ally.UnitID}
		rebuilt.PreparationSeedOrder = []string{owner.UnitID, ally.UnitID}
		s.rebuildBattleOrder(&rebuilt)
		for _, unitID := range append(append(append(append([]string{}, rebuilt.PreparationOrder...), rebuilt.PreparationSeedOrder...), rebuilt.FullOrder...), rebuilt.Order...) {
			if unitID == owner.UnitID {
				t.Fatalf("rebuilt action queues retained the wearer: prep=%v seed=%v full=%v ordered=%v", rebuilt.PreparationOrder, rebuilt.PreparationSeedOrder, rebuilt.FullOrder, rebuilt.Order)
			}
		}
		cloned := cloneBattleRuntimeState(*state)
		cloned.Players[0].UserID = 1
		cloned.Players[1].UserID = 2
		cloned.PreparationOrder = []string{owner.UnitID, ally.UnitID}
		cloned.PreparationSeedOrder = []string{owner.UnitID, ally.UnitID}
		cloned.Enemies = []battleUnitState{{
			UnitID: "enemy:test", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
			Position: "vanguard", BasePosition: "vanguard", Alive: true, HP: 100, MaxHP: 100,
			BaseStats: map[string]float64{"hp": 100, "speed": 1}, Stats: map[string]float64{"hp": 100, "speed": 1},
		}}
		if cloned.Players[0].TurnIneligible {
			t.Fatal("runtime clone unexpectedly retained the derived turn-ineligible flag")
		}
		if _, _, err := s.processBattleTurn(context.Background(), &cloned, battleWorldConfig{}, nil, time.UnixMilli(4), nil); err != nil {
			t.Fatalf("process cloned turn: %v", err)
		}
		acted := map[string]bool{}
		for _, unitID := range cloned.ActedUnits {
			acted[unitID] = true
		}
		if acted[owner.UnitID] || !acted[ally.UnitID] {
			t.Fatalf("cloned runtime acted units = %v, want ally only", cloned.ActedUnits)
		}
		for _, unitID := range cloned.PreparationSeedOrder {
			if unitID == owner.UnitID {
				t.Fatalf("cloned runtime retained wearer in preparation seed order: %v", cloned.PreparationSeedOrder)
			}
		}

		state.reseedOrder()
		for _, unitID := range append(append([]string{}, state.FullOrder...), state.Order...) {
			if unitID == owner.UnitID {
				t.Fatalf("turn-ineligible wearer entered an action order: full=%v ordered=%v", state.FullOrder, state.Order)
			}
		}
		actor, _, _, preparation := battleNextActor(state)
		if actor == nil || actor.UnitID != ally.UnitID || !preparation {
			t.Fatalf("preparation actor = %#v preparation=%v, want ally", actor, preparation)
		}
		if _, ok := s.executeEquipmentExtraTurn(state, time.UnixMilli(5), &state.Players[0], "test", battleEquipmentHookRule{ID: "test_extra_turn"}, nil, "full", false); ok {
			t.Fatal("active wearer passive allowed an extra turn")
		}
	})

	t.Run("two real wearers disable both battle effects", func(t *testing.T) {
		s := newEmpressCloakTestServer()
		first := empressCloakTestUnit("player:first-empress", "vanguard")
		first.EquipmentHooks = empressCloakTestHooks(0)
		second := empressCloakTestUnit("player:second-empress", "midguard")
		second.EquipmentHooks = empressCloakTestHooks(10)
		ally := empressCloakTestUnit("player:ally", "rearguard")
		state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{first, second, ally}}

		if got := battlePlayerPartyEquippedUnitCount(state, "empress_cloak"); got != 2 {
			t.Fatalf("real equipped wearer count = %d, want 2", got)
		}
		if actions := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1)); len(actions) != 0 {
			t.Fatalf("two wearers triggered opening effects: %#v", actions)
		}
		if actions := s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(2)); len(actions) != 0 {
			t.Fatalf("two wearers triggered wave effects: %#v", actions)
		}
		for index := range state.Players {
			if len(state.Players[index].Buffs) != 0 || state.Players[index].TurnIneligible {
				t.Fatalf("unit %s retained a cloak battle effect: %#v", state.Players[index].UnitID, state.Players[index])
			}
		}
	})
}

func TestEmpressCommandRefreshesWithoutStackingAndExpiresToBaseStats(t *testing.T) {
	s := newEmpressCloakTestServer()
	owner := empressCloakTestUnit("player:empress", "vanguard")
	owner.EquipmentHooks = empressCloakTestHooks(0)
	ally := empressCloakTestUnit("player:ally", "midguard")
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{owner, ally}}

	s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1))
	s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(2))
	for _, attr := range []string{"strength", "dexterity", "intelligence"} {
		if got := state.Players[1].Stats[attr]; got != 108 {
			t.Fatalf("wave 1 %s = %v, want 108", attr, got)
		}
	}
	if got := countUnitBuff(&state.Players[1], "empress_command"); got != 1 {
		t.Fatalf("wave 1 command buff count = %d, want 1", got)
	}

	state.Players[0].EquipmentHooks = empressCloakTestHooks(10)
	state.Wave = 2
	s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(3))
	if got := countUnitBuff(&state.Players[1], "empress_command"); got != 1 {
		t.Fatalf("refreshed command buff count = %d, want 1", got)
	}
	command := findBuffByID(&state.Players[1], "empress_command")
	if command == nil || command.RemainTurns != 4 {
		t.Fatalf("refreshed command duration = %#v, want 4", command)
	}
	for _, attr := range []string{"strength", "dexterity", "intelligence"} {
		if got := state.Players[1].Stats[attr]; got != 110 {
			t.Fatalf("refreshed %s = %v, want 110 rather than a stacked value", attr, got)
		}
	}

	for turn := 0; turn < 4; turn++ {
		s.executeUnitTurn(state, time.UnixMilli(int64(4+turn)), &state.Players[1], "player", false)
	}
	if battleHasBuff(&state.Players[1], "empress_command") {
		t.Fatalf("command buff did not expire: %#v", state.Players[1].Buffs)
	}
	for _, attr := range []string{"strength", "dexterity", "intelligence"} {
		if got := state.Players[1].Stats[attr]; got != 100 {
			t.Fatalf("expired %s = %v, want base value 100", attr, got)
		}
		if got := state.Players[1].StatModifiers[attr]; got != 0 {
			t.Fatalf("expired %s modifier = %v, want 0", attr, got)
		}
	}
}
