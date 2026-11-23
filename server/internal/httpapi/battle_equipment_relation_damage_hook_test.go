package httpapi

import (
	"fmt"
	"math"
	"testing"
	"time"
)

const fiveTickStarfallTestBuffID = "five_tick_starfall_count"

func fiveTickStarfallTestHooks(enhanceLevel int) []battleEquipmentHookRule {
	eventFilter := map[string]any{
		"turnKinds":              []string{"ordered"},
		"includePreparation":     false,
		"includeSuppressedTurns": true,
	}
	return []battleEquipmentHookRule{
		{
			ID:                 "five_tick_starfall_count",
			SourceItemID:       "five_tick_starfall_lunar_veil_robe",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            "on_turn_end",
			Priority:           10,
			EventFilter:        eventFilter,
			Conditions:         []map[string]any{},
			Effects: []map[string]any{{
				"kind":       "add_buff",
				"buffId":     fiveTickStarfallTestBuffID,
				"targetMode": "self",
			}},
		},
		{
			ID:                 "five_tick_starfall_burst",
			SourceItemID:       "five_tick_starfall_lunar_veil_robe",
			SourceEnhanceLevel: enhanceLevel,
			Trigger:            "on_turn_end",
			Priority:           0,
			EventFilter:        eventFilter,
			Conditions: []map[string]any{{
				"type":   "self_buff_count_gte",
				"buffId": fiveTickStarfallTestBuffID,
				"value":  5,
			}},
			Effects: []map[string]any{
				{
					"kind":         "damage",
					"targetMode":   "targeting",
					"damageType":   "magical",
					"relationMode": "team_side",
					"amountFormulaByRelation": map[string]any{
						"ally": map[string]any{
							"baseAdd": 600,
							"terms": []any{
								map[string]any{"attrKey": "magical_damage", "coefficient": 0.45, "perSkillLevel": 0},
								map[string]any{"attrKey": "equipment_enhance_level", "coefficient": -8.0, "perSkillLevel": 0},
							},
						},
						"enemy": map[string]any{
							"baseAdd": 600,
							"terms": []any{
								map[string]any{"attrKey": "magical_damage", "coefficient": 0.45, "perSkillLevel": 0},
								map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 60.0, "perSkillLevel": 0},
							},
						},
					},
					"targeting": map[string]any{
						"targetPositions": []any{"vanguard", "midguard", "rearguard"},
						"secondarySeek":   "none",
						"targetScope":     "all",
						"selectRule":      "first",
						"targetCount":     map[string]any{"base": 999, "formula": []any{}, "max": 999},
					},
					"simultaneous":           true,
					"dispatchEquipmentHooks": false,
				},
				{"kind": "remove_buff", "buffId": fiveTickStarfallTestBuffID, "targetMode": "self"},
			},
		},
	}
}

func newFiveTickStarfallTestServer() *Server {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{{
		"id":            fiveTickStarfallTestBuffID,
		"permanent":     true,
		"stackPolicy":   "stack",
		"maxStacks":     5,
		"durationTurns": -1,
	}}
	return s
}

func fiveTickStarfallUnit(id, side, unitType string, hp float64) battleUnitState {
	return battleUnitState{
		UnitID:         id,
		Side:           side,
		BaseSide:       side,
		OriginSide:     side,
		UnitType:       unitType,
		Position:       "vanguard",
		BasePosition:   "vanguard",
		Alive:          true,
		HP:             hp,
		MaxHP:          hp,
		Stats:          map[string]float64{"hp": hp, "magical_defence": 100},
		BaseStats:      map[string]float64{"hp": hp, "magical_defence": 100},
		EquipmentHooks: nil,
	}
}

func TestEquipmentRelationDamageCountsOrderedTurnsAndHitsWholeField(t *testing.T) {
	s := newFiveTickStarfallTestServer()
	source := fiveTickStarfallUnit("player:source", "player", "player", 5000)
	source.Stats["magical_damage"] = 100
	source.BaseStats["magical_damage"] = 100
	source.EquipmentHooks = fiveTickStarfallTestHooks(10)
	ally := fiveTickStarfallUnit("player:ally", "player", "kitty", 5000)
	allySummon := fiveTickStarfallUnit("player:summon", "player", "summon", 5000)
	enemy := fiveTickStarfallUnit("enemy:target", "enemy", "enemy", 5000)
	enemySummon := fiveTickStarfallUnit("enemy:summon", "enemy", "summon", 5000)
	state := &battleRuntimeState{
		Players: []battleUnitState{source, ally, allySummon},
		Enemies: []battleUnitState{enemy, enemySummon},
	}
	actor := &state.Players[0]
	context := &battleEquipmentHookContext{TurnKind: "ordered"}

	for turn := 1; turn <= 4; turn++ {
		actions := s.battleTriggerUnitEquipmentHooksWithContext(state, time.Unix(int64(turn), 0), actor, nil, nil, "on_turn_end", 0, "", context)
		if stacks := battleCountBuff(actor, fiveTickStarfallTestBuffID); stacks != turn {
			t.Fatalf("turn %d should leave %d visible stacks, buffs=%#v actions=%#v", turn, turn, actor.Buffs, actions)
		}
		if actor.HP != 5000 || state.Enemies[0].HP != 5000 {
			t.Fatalf("starfall triggered before fifth turn: actor=%.2f enemy=%.2f", actor.HP, state.Enemies[0].HP)
		}
	}

	actions := s.battleTriggerUnitEquipmentHooksWithContext(state, time.Unix(5, 0), actor, nil, nil, "on_turn_end", 0, "", context)
	if findBuffByID(actor, fiveTickStarfallTestBuffID) != nil {
		t.Fatalf("fifth turn should clear counter stacks: %#v", actor.Buffs)
	}
	assertHP := func(label string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.001 {
			t.Fatalf("%s HP = %.3f, want %.3f", label, got, want)
		}
	}
	assertHP("wearer", state.Players[0].HP, 4717.5)
	assertHP("ally", state.Players[1].HP, 4717.5)
	assertHP("ally summon", state.Players[2].HP, 4717.5)
	assertHP("enemy", state.Enemies[0].HP, 4377.5)
	assertHP("enemy summon", state.Enemies[1].HP, 4377.5)

	relations := map[string]string{}
	for _, action := range actions {
		if action["kind"] != "damage" {
			continue
		}
		meta := normalizeAnyMap(action["meta"])
		targetID := fmt.Sprintf("%v", action["targetId"])
		relations[targetID] = fmt.Sprintf("%v", meta["relation"])
		if meta["simultaneous"] != true || meta["dispatchEquipmentHooks"] != false {
			t.Fatalf("damage metadata lost authoritative batch flags: %#v", action)
		}
	}
	if relations["player:summon"] != "ally" || relations["enemy:summon"] != "enemy" {
		t.Fatalf("stable team-side relation mismatch: %#v", relations)
	}
}

func TestEquipmentTurnEventFilterHandlesPreparationExtraAndSuppression(t *testing.T) {
	hook := fiveTickStarfallTestHooks(0)[0]
	unit := &battleUnitState{UnitID: "player:filter", Alive: true, HP: 1}

	if battleEquipmentHookEventAllowed(hook, unit, nil, &battleEquipmentHookContext{TurnKind: "ordered", IsPreparation: true}) {
		t.Fatal("preparation turn must be excluded")
	}
	if battleEquipmentHookEventAllowed(hook, unit, nil, &battleEquipmentHookContext{TurnKind: "extra"}) {
		t.Fatal("extra action must be excluded")
	}
	if !battleEquipmentHookEventAllowed(hook, unit, nil, &battleEquipmentHookContext{TurnKind: "ordered", ActionSuppressed: true}) {
		t.Fatal("suppressed ordered action should count when configured")
	}
	hook.EventFilter["includeSuppressedTurns"] = false
	if battleEquipmentHookEventAllowed(hook, unit, nil, &battleEquipmentHookContext{TurnKind: "ordered", ActionSuppressed: true}) {
		t.Fatal("suppressed ordered action should be excluded when configured")
	}
}
