package httpapi

import (
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
)

func hushedLunarVeilDamageHook(applicationKinds []string) battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID:                 "hushed_lunar_veil_silence_echo",
		SourceItemID:       "hushed_lunar_veil_pointed_hat",
		SourceEnhanceLevel: 10,
		Trigger:            battleEquipmentTriggerAfterApplyBuff,
		EventFilter: map[string]any{
			"buffIds":          []string{"silence", "holy_cat_silent_liturgy"},
			"applicationKinds": applicationKinds,
			"excludeSelf":      true,
		},
		Conditions: []map[string]any{},
		Effects: []map[string]any{{
			"kind":       "damage",
			"targetMode": "source",
			"damageType": "magical",
			"amountFormula": map[string]any{
				"baseAdd": 0,
				"terms": []any{
					map[string]any{"attrKey": "perception", "coefficient": 1.6, "perSkillLevel": 0},
					map[string]any{"attrKey": "equipment_enhance_level", "coefficient": 4.0, "perSkillLevel": 0},
				},
			},
			"dispatchEquipmentHooks": false,
		}},
	}
}

func newHushedLunarVeilTestState(hook battleEquipmentHookRule) *battleRuntimeState {
	return &battleRuntimeState{
		Players: []battleUnitState{{
			UnitID:    "player:perception",
			Side:      "player",
			Alive:     true,
			HP:        1000,
			MaxHP:     1000,
			MP:        100,
			MaxMP:     100,
			Stats:     map[string]float64{"hp": 1000, "mp": 100, "perception": 100},
			BaseStats: map[string]float64{"hp": 1000, "mp": 100, "perception": 100},
			EquipmentHooks: []battleEquipmentHookRule{hook, {
				ID:      "must_not_chain_from_echo_damage",
				Trigger: "on_after_deal_damage",
				Effects: []map[string]any{{
					"kind":          "consume_mp",
					"amountFormula": map[string]any{"baseAdd": 10, "terms": []any{}},
				}},
			}},
		}},
		Enemies: []battleUnitState{{
			UnitID:    "enemy:target",
			Side:      "enemy",
			Alive:     true,
			HP:        1000,
			MaxHP:     1000,
			Stats:     map[string]float64{"hp": 1000, "magical_defence": 100},
			BaseStats: map[string]float64{"hp": 1000, "magical_defence": 100},
		}},
	}
}

func applyTestBuff(t *testing.T, s *Server, state *battleRuntimeState, sourceID, targetID, buffID string) []map[string]any {
	t.Helper()
	runtime := newBattleSkillActionRuntime(state, battleUnitByID(state, sourceID), time.Unix(0, 0), s, nil)
	runtime.ApplyBuff(battleskills.BuffAction{
		SourceID:      sourceID,
		TargetID:      targetID,
		BuffID:        buffID,
		DurationTurns: 1,
		Reason:        "test_apply_buff",
	})
	return runtime.ActionLog()
}

func TestEquipmentAfterApplyBuffDamageFiltersBuffTargetAndApplicationKind(t *testing.T) {
	s := &Server{}
	state := newHushedLunarVeilTestState(hushedLunarVeilDamageHook([]string{"attach", "refresh"}))
	source := &state.Players[0]
	target := &state.Enemies[0]

	actions := applyTestBuff(t, s, state, source.UnitID, target.UnitID, "silence")
	if target.HP != 900 {
		t.Fatalf("silence echo damage = %.2f HP remaining, want 900; actions=%#v", target.HP, actions)
	}
	if source.MP != 100 {
		t.Fatalf("echo damage must not dispatch equipment damage hooks, MP=%.2f", source.MP)
	}
	buffActionIndex := -1
	damageActionIndex := -1
	for index, action := range actions {
		if action["kind"] == "buff" && buffActionIndex < 0 {
			buffActionIndex = index
		}
		if action["kind"] == "damage" {
			damageActionIndex = index
		}
	}
	if buffActionIndex < 0 || damageActionIndex <= buffActionIndex {
		t.Fatalf("buff result must precede echo damage: %#v", actions)
	}
	if meta, _ := actions[len(actions)-1]["meta"].(map[string]any); meta["rawDamage"] != float64(200) || meta["triggerBuffId"] != "silence" || meta["buffApplicationKind"] != "attach" {
		t.Fatalf("echo damage metadata mismatch: %#v", actions[len(actions)-1])
	}

	applyTestBuff(t, s, state, source.UnitID, target.UnitID, "silence")
	if target.HP != 800 {
		t.Fatalf("refresh should trigger a second echo, HP=%.2f", target.HP)
	}
	applyTestBuff(t, s, state, source.UnitID, target.UnitID, "forbidden_library_magic_defence_down")
	if target.HP != 800 {
		t.Fatalf("non-silence buff must not trigger echo, HP=%.2f", target.HP)
	}
	applyTestBuff(t, s, state, source.UnitID, source.UnitID, "silence")
	if source.HP != 1000 {
		t.Fatalf("self-applied silence must be excluded, HP=%.2f", source.HP)
	}
}

func TestEquipmentAfterApplyBuffCanRestrictToFirstAttach(t *testing.T) {
	s := &Server{}
	state := newHushedLunarVeilTestState(hushedLunarVeilDamageHook([]string{"attach"}))
	source := &state.Players[0]
	target := &state.Enemies[0]

	applyTestBuff(t, s, state, source.UnitID, target.UnitID, "holy_cat_silent_liturgy")
	applyTestBuff(t, s, state, source.UnitID, target.UnitID, "holy_cat_silent_liturgy")
	if target.HP != 900 {
		t.Fatalf("attach-only filter must ignore refresh, HP=%.2f", target.HP)
	}
}

func TestEquipmentAddBuffDispatchesGenericAfterApplyBuffEvent(t *testing.T) {
	s := &Server{}
	state := newHushedLunarVeilTestState(hushedLunarVeilDamageHook([]string{"attach", "refresh"}))
	source := &state.Players[0]
	target := &state.Enemies[0]
	source.EquipmentHooks = append([]battleEquipmentHookRule{{
		ID:      "test_silence_delivery",
		Trigger: "on_after_deal_damage",
		Effects: []map[string]any{{
			"kind":          "add_buff",
			"buffId":        "silence",
			"durationTurns": 1,
			"targetMode":    "source",
		}},
	}}, source.EquipmentHooks...)

	actions := s.battleTriggerUnitEquipmentHooksWithTarget(state, time.Unix(0, 0), source, target, target, "on_after_deal_damage", 1, "magical")
	if findBuffByID(target, "silence") == nil || target.HP != 900 {
		t.Fatalf("equipment-applied silence must dispatch the generic event: target=%#v actions=%#v", target, actions)
	}
}
