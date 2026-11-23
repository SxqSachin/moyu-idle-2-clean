package httpapi

import (
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func TestBattleEquipmentHookParsesAndResetsPerWaveLimit(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{{
		ID: "wave_limited_ticket",
		BattleHooks: []map[string]any{{
			"id": "wave_limited_failure", "trigger": "on_probability_failure", "maxTriggersPerWave": 1,
			"effects": []map[string]any{{"kind": "extra_turn", "targetMode": "self", "turnMode": "full"}},
		}},
	}}
	hooks := s.resolveBattleEquipmentHooks([]store.EquippedItem{{
		InstanceID: 301, TemplateItemID: "wave_limited_ticket", EquipType: "battle", EquippedSlot: "cloak",
	}})
	if len(hooks) != 1 || hooks[0].MaxTriggersPerWave != 1 || hooks[0].MaxTriggersPerBattle != 0 {
		t.Fatalf("equipment wave limit was not parsed: %#v", hooks)
	}
	actor := specializationTestUnit("player:wave", "player", 100, 0)
	actor.EquipmentHooks = hooks
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}}

	if _, ok := battleActivateEquipmentHook(state, &state.Players[0], hooks[0], "on_probability_failure"); !ok {
		t.Fatal("first wave activation was rejected")
	}
	if _, ok := battleActivateEquipmentHook(state, &state.Players[0], hooks[0], "on_probability_failure"); ok {
		t.Fatal("second activation in the same wave bypassed maxTriggersPerWave")
	}
	state.Wave = 2
	s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(2))
	if _, ok := battleActivateEquipmentHook(state, &state.Players[0], hooks[0], "on_probability_failure"); !ok {
		t.Fatal("new enemy-refresh wave did not reset maxTriggersPerWave")
	}
}

func TestBattleEquipmentHooksFromSameTemplateKeepInstanceIdentity(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{{
		ID: "paired_blade",
		BattleHooks: []map[string]any{{
			"id": "paired_focus", "trigger": "on_before_deal_damage", "maxTriggersPerTurn": 1,
			"effects": []map[string]any{{"kind": "runtime_damage_amp", "value": .1}},
		}},
	}}
	hooks := s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{InstanceID: 101, TemplateItemID: "paired_blade", EquipType: "battle", EquippedSlot: "right_hand"},
		{InstanceID: 202, TemplateItemID: "paired_blade", EquipType: "battle", EquippedSlot: "right_hand_2"},
	})
	if len(hooks) != 2 || hooks[0].ID == hooks[1].ID || hooks[0].SourceInstanceID == hooks[1].SourceInstanceID {
		t.Fatalf("same-template hooks lost instance identity: %#v", hooks)
	}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	actor.EquipmentHooks = hooks
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	amp, actions, sources := s.battleCollectRuntimeDamageAmpFromEquipmentHooksWithContextForTest(state, &state.Players[0], &state.Enemies[0], nil)
	if math.Abs(amp-.2) > .001 || len(actions) != 2 || len(sources) != 2 {
		t.Fatalf("independent instance limits did not both settle: amp=%v actions=%#v sources=%#v", amp, actions, sources)
	}
	seen := map[int64]bool{}
	for _, action := range actions {
		meta := normalizeAnyMap(action["meta"])
		instanceID := toInt64Loose(meta["sourceInstanceId"], 0)
		if meta["sourceKind"] != "equipment" || instanceID <= 0 || meta["hookDefinitionId"] != "paired_focus" {
			t.Fatalf("timeline action lost equipment instance source: %#v", action)
		}
		seen[instanceID] = true
	}
	if !seen[101] || !seen[202] {
		t.Fatalf("timeline did not distinguish both equipped instances: %#v", seen)
	}
	compact := compactBattleTimelineStepMeta(actions[0]["meta"])
	if toInt64Loose(compact["sourceInstanceId"], 0) <= 0 || compact["hookDefinitionId"] != "paired_focus" {
		t.Fatalf("compact timeline dropped equipment instance identity: %#v", compact)
	}
}

func TestBattleEquipmentExplicitSharedTriggerGroupStillSharesAcrossInstances(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	s.deps.GameData.EquipmentTemplates = []catalog.EquipmentTemplate{{
		ID: "paired_blade",
		BattleHooks: []map[string]any{{
			"id": "paired_focus", "trigger": "on_before_deal_damage", "maxTriggersPerTurn": 1,
			"sharedTriggerGroup": "paired_focus_shared",
			"effects":            []map[string]any{{"kind": "runtime_damage_amp", "value": .1}},
		}},
	}}
	actor := specializationTestUnit("player:actor", "player", 100, 0)
	target := specializationTestUnit("enemy:target", "enemy", 100, 0)
	actor.EquipmentHooks = s.resolveBattleEquipmentHooks([]store.EquippedItem{
		{InstanceID: 101, TemplateItemID: "paired_blade", EquipType: "battle", EquippedSlot: "right_hand"},
		{InstanceID: 202, TemplateItemID: "paired_blade", EquipType: "battle", EquippedSlot: "right_hand_2"},
	})
	state := &battleRuntimeState{Wave: 1, Players: []battleUnitState{actor}, Enemies: []battleUnitState{target}}
	amp, actions, _ := s.battleCollectRuntimeDamageAmpFromEquipmentHooksWithContextForTest(state, &state.Players[0], &state.Enemies[0], nil)
	if math.Abs(amp-.1) > .001 || len(actions) != 1 {
		t.Fatalf("explicit sharedTriggerGroup did not share its limit: amp=%v actions=%#v", amp, actions)
	}
}
