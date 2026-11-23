package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func combatTrialTestFormat() *combatTrialFormatDefinition {
	return &combatTrialFormatDefinition{
		ID:              "standard",
		SchemaVersion:   1,
		RulesetVersion:  combatTrialFormatRulesetVersionV1,
		BalanceRevision: 1,
		Enabled:         true,
		ActionBudget:    20,
		Metrics: []combatTrialMetricDefinition{
			{ID: "damage_total", SchemaVersion: 1, EventType: "damage", Aggregation: "sum", SourceScope: "player_side", TargetScope: "trial_dummies", Enabled: true},
			{ID: "damage_max_segment", SchemaVersion: 1, EventType: "damage", Aggregation: "max", SourceScope: "player_side", TargetScope: "trial_dummies", Enabled: true},
		},
		Dummy: combatTrialDummyDefinition{
			Count: 3, Positions: []string{"vanguard", "midguard", "rearguard"}, Defense: 125,
			DefenseSource: "combat_balance_defense_k", HPMode: "immutable_full", MaxHP: 1_000_000_000_000, PassiveTurns: true,
			CriticalDamageResist: 0.5,
		},
		LineupRules: &combatTrialLineupRulesDefinition{PositionPolicy: "shared"},
	}
}

func combatTrialTestUnit(id, side string) battleUnitState {
	return battleUnitState{
		UnitID: id, Side: side, BaseSide: side, OriginSide: side, PerspectiveSide: side,
		Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100, "speed": 10}, BaseStats: map[string]float64{"hp": 100, "speed": 10},
		Buffs: []battleBuffState{}, Skills: []battleSkillState{}, BattleItems: []battleItemState{},
	}
}

func TestCombatTrialSharedPositionPolicyAllowsMultipleUnitsPerLane(t *testing.T) {
	selection, err := normalizeCombatTrialSelection(combatTrialLineupSelection{
		PlayerPosition: "vanguard",
		FormatID:       "standard",
		Kitties: []competitiveLineupKittySelection{
			{KittyID: 1, Position: "midguard"},
			{KittyID: 2, Position: "midguard"},
		},
	})
	if err != nil {
		t.Fatalf("normalize shared-position lineup: %v", err)
	}
	if err := validateCombatTrialSelectionForFormat(selection, &combatTrialFormatDefinition{
		LineupRules: &combatTrialLineupRulesDefinition{PositionPolicy: "shared"},
	}); err != nil {
		t.Fatalf("shared position policy rejected a valid lineup: %v", err)
	}
}

func TestCombatTrialDistinctPositionPolicyRejectsMultipleUnitsPerLane(t *testing.T) {
	selection, err := normalizeCombatTrialSelection(combatTrialLineupSelection{
		PlayerPosition: "vanguard",
		FormatID:       "standard",
		Kitties: []competitiveLineupKittySelection{
			{KittyID: 1, Position: "midguard"},
			{KittyID: 2, Position: "midguard"},
		},
	})
	if err != nil {
		t.Fatalf("normalize distinct-position lineup: %v", err)
	}
	if err := validateCombatTrialSelectionForFormat(selection, &combatTrialFormatDefinition{
		LineupRules: &combatTrialLineupRulesDefinition{PositionPolicy: "distinct"},
	}); err == nil {
		t.Fatal("distinct position policy must reject multiple units in the same lane")
	}
}

func TestCombatTrialPublicLeaderboardEntryOmitsUserID(t *testing.T) {
	entry := combatTrialPublicLeaderboardEntry(store.CombatTrialLeaderboardEntry{
		Rank: 2, UserID: 99, Username: "tester", MetricID: "damage_total", Value: 123,
		EndReason: "action_budget_reached",
		Metrics:   map[string]float64{"damage_total": 123, "actions_to_victory": 20, "private_debug": 999},
	}, map[string]combatTrialMetricDefinition{
		"damage_total": {
			ID: "damage_total", Ranked: true,
			EligibleEndReasons: []string{"action_budget_reached", "team_defeated"},
		},
		"actions_to_victory": {
			ID: "actions_to_victory", Ranked: true,
			EligibleEndReasons: []string{"enemy_team_defeated"},
		},
	})
	if _, exposed := entry["userId"]; exposed {
		t.Fatalf("leaderboard entry exposed user id: %#v", entry)
	}
	if entry["rank"] != int64(2) || entry["username"] != "tester" {
		t.Fatalf("leaderboard public summary mismatch: %#v", entry)
	}
	metrics, ok := entry["metrics"].(map[string]float64)
	if !ok || metrics["damage_total"] != 123 {
		t.Fatalf("leaderboard public metrics mismatch: %#v", entry["metrics"])
	}
	if _, exposed := metrics["private_debug"]; exposed {
		t.Fatalf("leaderboard exposed an unranked metric: %#v", metrics)
	}
	if _, exposed := metrics["actions_to_victory"]; exposed {
		t.Fatalf("leaderboard exposed a metric ineligible for the source outcome: %#v", metrics)
	}
}

func TestCombatTrialDummyContract(t *testing.T) {
	format := combatTrialTestFormat()
	dummies := combatTrialDummyUnits(format)
	if len(dummies) != 3 {
		t.Fatalf("dummy count = %d, want 3", len(dummies))
	}
	for index, dummy := range dummies {
		if dummy.Position != format.Dummy.Positions[index] || dummy.UnitType != combatTrialDummyUnitType {
			t.Fatalf("dummy %d identity mismatch: %#v", index, dummy)
		}
		if !dummy.Alive || dummy.HP != dummy.MaxHP || dummy.Stats["physics_defence"] != 125 || dummy.Stats["magical_defence"] != 125 || dummy.Stats["crit_damage_resist"] != 0.5 {
			t.Fatalf("dummy %d frozen stats mismatch: %#v", index, dummy)
		}
		if len(dummy.Skills) != 0 || dummy.RewardEligible {
			t.Fatalf("dummy %d must be passive and reward-ineligible: %#v", index, dummy)
		}
	}
	compact := compactBattleSnapshotUnits(competitivePublicUnitSlice(dummies))
	if len(compact) != 3 || compact[0]["t"] != combatTrialDummyUnitType {
		t.Fatalf("compact replay snapshot lost dummy identity: %#v", compact)
	}
}

func TestCombatTrialDamageMetricsAndImmutableHP(t *testing.T) {
	format := combatTrialTestFormat()
	player := combatTrialTestUnit("combat_trial:player:1", "player")
	dummies := combatTrialDummyUnits(format)
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{player}, Enemies: dummies,
		CombatTrialFormat: format, CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	first := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: player.UnitID, TargetID: dummies[0].UnitID, Amount: 42.5, DamageType: "physical",
		SuppressEquipmentHooks: true, Meta: map[string]any{"skipFinalDamageModifiers": true},
	})
	second := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: player.UnitID, TargetID: dummies[1].UnitID, Amount: 17.25, DamageType: "magical",
		SuppressEquipmentHooks: true, Meta: map[string]any{"skipFinalDamageModifiers": true},
	})
	if first.Amount != 42.5 || second.Amount != 17.25 {
		t.Fatalf("final damage mismatch: first=%v second=%v", first.Amount, second.Amount)
	}
	if got := state.CombatTrialMetrics["damage_total"]; got != 59.75 {
		t.Fatalf("damage_total = %v, want 59.75", got)
	}
	if got := state.CombatTrialMetrics["damage_max_segment"]; got != 42.5 {
		t.Fatalf("damage_max_segment = %v, want 42.5", got)
	}
	for index := range state.Enemies {
		if !state.Enemies[index].Alive || state.Enemies[index].HP != state.Enemies[index].MaxHP {
			t.Fatalf("dummy %d lost hp or died: %#v", index, state.Enemies[index])
		}
	}
}

func TestCombatTrialExecuteDamageIsCountedForDefeatableObjectives(t *testing.T) {
	format := combatTrialTestFormat()
	for index := range format.Metrics {
		format.Metrics[index].TargetScope = "objective_units"
	}
	format.Encounter = combatTrialEncounterDefinition{Kind: "unit_roster", Units: []combatTrialEncounterUnitDefinition{{
		ID: "boss", Kind: "authored_enemy", Position: "vanguard", Behavior: "active", HealthMode: "normal", ObjectiveTarget: true,
	}}}
	player := combatTrialTestUnit("combat_trial:player:1", "player")
	player.EquipmentHooks = []battleEquipmentHookRule{{
		ID: "trial_execute", SourceKind: "equipment", Trigger: "on_after_deal_damage",
		Effects: []map[string]any{{"kind": "execute", "targetMode": "source", "hpPercentThreshold": 0.5}},
	}}
	boss := combatTrialTestUnit("combat_trial:opponent:boss", "enemy")
	boss.HP = 40
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{player}, Enemies: []battleUnitState{boss}, CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
		CombatTrialObjectiveTargetIDs: map[string]struct{}{boss.UnitID: {}},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	rows := (&Server{}).battleTriggerUnitEquipmentHooksWithContext(
		&state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], &state.Enemies[0],
		"on_after_deal_damage", 1, "physical", nil,
	)
	if len(rows) != 1 || state.Enemies[0].Alive || state.Enemies[0].HP != 0 {
		t.Fatalf("execute did not defeat the authored objective: rows=%#v target=%#v", rows, state.Enemies[0])
	}
	if total, segment := state.CombatTrialMetrics["damage_total"], state.CombatTrialMetrics["damage_max_segment"]; total != 40 || segment != 40 {
		t.Fatalf("execute damage metrics mismatch: metrics=%#v", state.CombatTrialMetrics)
	}
}

func TestCombatTrialExecuteCannotKillImmutableObjective(t *testing.T) {
	format := combatTrialTestFormat()
	player := combatTrialTestUnit("combat_trial:player:1", "player")
	player.EquipmentHooks = []battleEquipmentHookRule{{
		ID: "trial_execute", SourceKind: "equipment", Trigger: "on_after_deal_damage",
		Effects: []map[string]any{{"kind": "execute", "targetMode": "source", "hpPercentThreshold": 2.0}},
	}}
	dummy := combatTrialDummyUnits(format)[0]
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{player}, Enemies: []battleUnitState{dummy}, CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	rows := (&Server{}).battleTriggerUnitEquipmentHooksWithContext(
		&state, time.UnixMilli(1), &state.Players[0], &state.Enemies[0], &state.Enemies[0],
		"on_after_deal_damage", 1, "physical", nil,
	)
	if len(rows) != 0 || !state.Enemies[0].Alive || state.Enemies[0].HP != state.Enemies[0].MaxHP || len(state.CombatTrialMetrics) != 0 {
		t.Fatalf("execute affected immutable objective: rows=%#v target=%#v metrics=%#v", rows, state.Enemies[0], state.CombatTrialMetrics)
	}
}

func TestCombatTrialDummyIgnoresHPResourceMutation(t *testing.T) {
	format := combatTrialTestFormat()
	player := combatTrialTestUnit("combat_trial:player:1", "player")
	dummies := combatTrialDummyUnits(format)
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{player}, Enemies: dummies,
		CombatTrialFormat: format,
	}
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	runtime.ApplyResource(battleskills.ResourceAction{
		SourceID: player.UnitID,
		TargetID: dummies[0].UnitID,
		Resource: "hp",
		Amount:   -format.Dummy.MaxHP,
	})
	if !state.Enemies[0].Alive || state.Enemies[0].HP != state.Enemies[0].MaxHP {
		t.Fatalf("hp resource mutation changed immutable dummy health: %#v", state.Enemies[0])
	}
}

func TestCombatTrialDummyRestorePreventsDirectDeath(t *testing.T) {
	format := combatTrialTestFormat()
	dummy := combatTrialDummyUnits(format)[0]
	dummy.HP = 0
	dummy.MaxHP = 0
	dummy.Alive = false
	dummy.Side = "player"
	dummy.Position = "rearguard"
	dummy.BasePosition = ""
	state := battleRuntimeState{BattleKind: battleKindCombatTrial, CombatTrialFormat: format, Enemies: []battleUnitState{dummy}}
	battleQueueUnitDeath(&state, &state.Enemies[0], nil)
	if !state.Enemies[0].Alive || state.Enemies[0].HP != format.Dummy.MaxHP || state.Enemies[0].MaxHP != format.Dummy.MaxHP || state.Enemies[0].Side != "enemy" || state.Enemies[0].Position != "vanguard" {
		t.Fatalf("dummy immutable-health guard failed: %#v", state.Enemies[0])
	}
	if len(state.pendingUnitDeaths) != 0 {
		t.Fatalf("dummy death must not be queued: %#v", state.pendingUnitDeaths)
	}
}

func TestTrialDummyTypeHasNoImmutableHealthOutsideCombatTrial(t *testing.T) {
	player := combatTrialTestUnit("player:1", "player")
	target := combatTrialTestUnit("enemy:trial_dummy", "enemy")
	target.UnitType = combatTrialDummyUnitType
	target.HP = 10
	target.MaxHP = 10
	state := battleRuntimeState{Players: []battleUnitState{player}, Enemies: []battleUnitState{target}}
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	effect := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: player.UnitID, TargetID: target.UnitID, Amount: 20, DamageType: "physical",
		SuppressEquipmentHooks: true, Meta: map[string]any{"skipFinalDamageModifiers": true},
	})
	if effect.Amount != 20 || state.Enemies[0].Alive || state.Enemies[0].HP != 0 {
		t.Fatalf("ordinary battle target was incorrectly protected by trial health rules: effect=%#v target=%#v", effect, state.Enemies[0])
	}
}

func TestCombatTrialHealthModeIsDefinedByEncounterInsteadOfUnitType(t *testing.T) {
	player := combatTrialTestUnit("player:1", "player")
	target := combatTrialTestUnit("combat_trial:opponent:boss", "enemy")
	target.UnitType = combatTrialDummyUnitType
	target.HP = 10
	target.MaxHP = 10
	format := combatTrialTestFormat()
	format.Encounter = combatTrialEncounterDefinition{Kind: "unit_roster", Units: []combatTrialEncounterUnitDefinition{{
		ID: "boss", Kind: "authored_enemy", Position: "vanguard", Behavior: "active", HealthMode: "normal", ObjectiveTarget: true,
	}}}
	state := battleRuntimeState{BattleKind: battleKindCombatTrial, CombatTrialFormat: format, Players: []battleUnitState{player}, Enemies: []battleUnitState{target}}
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: player.UnitID, TargetID: target.UnitID, Amount: 20, DamageType: "physical",
		SuppressEquipmentHooks: true, Meta: map[string]any{"skipFinalDamageModifiers": true},
	})
	if state.Enemies[0].Alive || state.Enemies[0].HP != 0 {
		t.Fatalf("normal-health encounter was incorrectly protected by its unit type: %#v", state.Enemies[0])
	}
}

func TestCombatTrialTimelineNodeCarriesGenericProgressSnapshot(t *testing.T) {
	format := combatTrialTestFormat()
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format, CombatTrialActionCount: 3,
		CombatTrialMetrics: map[string]float64{"damage_total": 159, "damage_max_segment": 103},
	}
	state.emit("battle.turn.action", time.UnixMilli(10), map[string]any{
		"wave": 1, "actorId": "player:1", "skillId": "basic_attack", "actionLog": []map[string]any{}, "hooks": []map[string]any{},
	})
	progress, ok := state.Events[0].Data["combatTrial"].(map[string]any)
	if !ok || progress["progressVersion"] != combatTrialProgressSchemaV1 || progress["actionCount"] != 3 || progress["actionBudget"] != 20 {
		t.Fatalf("combat trial event progress mismatch: %#v", state.Events[0].Data)
	}
	metrics, ok := progress["metrics"].(map[string]float64)
	if !ok || metrics["damage_total"] != 159 || metrics["damage_max_segment"] != 103 {
		t.Fatalf("combat trial event metrics mismatch: %#v", progress)
	}
	node := buildBattleTimelineNodeData(state.Events[0], state.Events[0].Data, nil, nil)
	nodeProgress, ok := node["combatTrial"].(map[string]any)
	if !ok || nodeProgress["actionCount"] != 3 {
		t.Fatalf("timeline node dropped combat trial progress: %#v", node)
	}
}

func TestCombatTrialDamageKeepsRegisteredSummonAttribution(t *testing.T) {
	format := combatTrialTestFormat()
	summon := combatTrialTestUnit("summon_player_1_fire_1", "player")
	target := combatTrialDummyUnits(format)[0]
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format, CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &summon)
	battleCombatTrialRecordDamage(&state, summon.UnitID, nil, &target, 9)
	if got := state.CombatTrialMetrics["damage_total"]; got != 9 {
		t.Fatalf("registered summon damage_total = %v, want 9", got)
	}
}

func TestCombatTrialRegistersSummonBeforeDeferredDamage(t *testing.T) {
	format := combatTrialTestFormat()
	source := combatTrialTestUnit("combat_trial:player:1", "player")
	summon := combatTrialTestUnit("summon_player_1_fire_1", "player")
	summon.SummonTemplateUnitID = "fire"
	summon.SummonSourceUnitID = source.UnitID
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	server := &Server{}
	server.battleTriggerAfterSummonHooks(&state, time.UnixMilli(1), &source, &summon)

	target := combatTrialDummyUnits(format)[0]
	battleCombatTrialRecordDamage(&state, summon.UnitID, nil, &target, 9)
	if got := state.CombatTrialMetrics["damage_total"]; got != 9 {
		t.Fatalf("deferred summon damage_total = %v, want 9", got)
	}
}

func TestOrdinaryBattleSummonDoesNotInitializeCombatTrialState(t *testing.T) {
	source := combatTrialTestUnit("player:1", "player")
	summon := combatTrialTestUnit("summon_player_1_fire_1", "player")
	state := battleRuntimeState{}
	server := &Server{}

	server.battleTriggerAfterSummonHooks(&state, time.UnixMilli(1), &source, &summon)
	if state.CombatTrialPlayerSourceIDs != nil {
		t.Fatalf("ordinary summon mutated combat-trial attribution state: %#v", state.CombatTrialPlayerSourceIDs)
	}
}

func TestOrdinaryBattleIgnoresPrefilledCombatTrialInternals(t *testing.T) {
	format := combatTrialTestFormat()
	format.Modifiers = []combatTrialModifierDefinition{{
		ID: "arcane_focus", Kind: "outgoing_damage_multiplier", SourceScope: "player_side", IncludeSummons: true, DamageType: "magical", Multiplier: 2,
	}}
	player := combatTrialTestUnit("player:1", "player")
	target := combatTrialDummyUnits(format)[0]
	target.HP = 1
	state := battleRuntimeState{
		BattleKind: "dungeon", CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{player.UnitID: {}},
		CombatTrialObjectiveTargetIDs: map[string]struct{}{target.UnitID: {}},
	}

	if got := battleCombatTrialDamageAmplification(&state, player.UnitID, &player, "magical"); got != 0 {
		t.Fatalf("ordinary battle combat-trial amplification = %v, want 0", got)
	}
	battleRecordCombatTrialMainAction(&state, &player, false, nil)
	battleCombatTrialRecordDamage(&state, player.UnitID, &player, &target, 10)
	battleRestoreCombatTrialImmutableTarget(&state, &target)
	if state.CombatTrialActionCount != 0 || len(state.CombatTrialMetrics) != 0 {
		t.Fatalf("ordinary battle consumed combat-trial state: actions=%d metrics=%#v", state.CombatTrialActionCount, state.CombatTrialMetrics)
	}
	if target.HP != 1 {
		t.Fatalf("ordinary battle target hp was restored by combat-trial rules: %v", target.HP)
	}
	if sourceID, source := battleCombatTrialBuffMetricSource(&state, &battleBuffState{SourceID: player.UnitID}, &player); sourceID != "" || source != nil {
		t.Fatalf("ordinary battle exposed combat-trial buff attribution: source_id=%q source=%#v", sourceID, source)
	}
}

func TestCombatTrialGenericHealMetricUsesEffectiveAmount(t *testing.T) {
	format := combatTrialTestFormat()
	format.Metrics = append(format.Metrics, combatTrialMetricDefinition{
		ID: "healing_total", SchemaVersion: 1, EventType: "heal", Aggregation: "sum", SourceScope: "player_side", TargetScope: "player_side", Enabled: true,
	})
	healer := combatTrialTestUnit("player:healer", "player")
	target := combatTrialTestUnit("player:target", "player")
	target.HP = 60
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{healer, target}, CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	runtime.ApplyHeal(battleskills.HealAction{SourceID: healer.UnitID, TargetID: target.UnitID, Amount: 80})
	if got := state.CombatTrialMetrics["healing_total"]; got != 40 {
		t.Fatalf("healing_total = %v, want effective heal 40", got)
	}
	state.Players[1].HP = 50
	runtime.ApplyResource(battleskills.ResourceAction{SourceID: healer.UnitID, TargetID: target.UnitID, Resource: "hp", Amount: 20})
	if got := state.CombatTrialMetrics["healing_total"]; got != 60 {
		t.Fatalf("healing_total after hp resource = %v, want 60", got)
	}
}

func TestCombatTrialRecoveryProjectCountsEffectiveHealingAndDeterioratesPatient(t *testing.T) {
	patientDefinition := combatTrialEncounterUnitDefinition{
		ID: "wounded_patient", Kind: "scripted_ally", NameKey: "game.combatTrial.objectives.woundedPatient",
		Side: "player", Position: "midguard", Behavior: "passive", HealthMode: "normal", ObjectiveTarget: true,
		InitialHPRatio: 0.5, SkipTurnOrder: true, MaxHP: 100_000_000,
		InitialBuffs: []combatTrialInitialBuffDefinition{{
			BuffID: "combat_trial_patient_deterioration", Payload: map[string]float64{"max_hp_loss_ratio": 0.05},
		}},
	}
	clockDefinitions := []combatTrialEncounterUnitDefinition{
		{ID: "recovery_clock_vanguard", Kind: "training_target", Position: "vanguard", Behavior: "passive", HealthMode: "immutable_full", DefenseSource: "combat_balance_defense_k", Defense: 100, MaxHP: 1_000_000},
		{ID: "recovery_clock_midguard", Kind: "training_target", Position: "midguard", Behavior: "passive", HealthMode: "immutable_full", DefenseSource: "combat_balance_defense_k", Defense: 100, MaxHP: 1_000_000},
		{ID: "recovery_clock_rearguard", Kind: "training_target", Position: "rearguard", Behavior: "passive", HealthMode: "immutable_full", DefenseSource: "combat_balance_defense_k", Defense: 100, MaxHP: 1_000_000},
	}
	encounterUnits := append([]combatTrialEncounterUnitDefinition{patientDefinition}, clockDefinitions...)
	format := &combatTrialFormatDefinition{
		ID: "recovery", SchemaVersion: 2, RulesetVersion: combatTrialFormatRulesetVersion, BalanceRevision: 2, Enabled: true,
		ActionBudget: 20,
		Metrics: []combatTrialMetricDefinition{
			{ID: "effective_healing", SchemaVersion: 2, EventType: "heal", Aggregation: "sum", SourceScope: "player_side", TargetScope: "objective_units", Enabled: true},
			{ID: "objective_remaining_hp", SchemaVersion: 2, EventType: "battle_end", Aggregation: "final", ValueSource: "objective_units_remaining_hp", SourceScope: "player_side", TargetScope: "objective_units", Enabled: true},
		},
		Encounter: combatTrialEncounterDefinition{Kind: "unit_roster", Units: encounterUnits},
	}
	scriptedPlayers, enemies, err := combatTrialEncounterParticipants(format, 0)
	if err != nil || len(scriptedPlayers) != 1 || len(enemies) != 3 {
		t.Fatalf("build recovery encounter: players=%#v enemies=%#v err=%v", scriptedPlayers, enemies, err)
	}
	healer := combatTrialTestUnit("player:healer", "player")
	healer.UserID = 1
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Buff: []map[string]any{{
			"id": "combat_trial_patient_deterioration", "durationTurns": 0, "permanent": true,
			"dispellable": false, "stackPolicy": "refresh_duration",
		}},
		BuffEffects: []map[string]any{
			{
				"id": "combat_trial_patient_deterioration_on_turn_start", "buffId": "combat_trial_patient_deterioration",
				"trigger": "on_turn_start", "priority": 100, "conditions": []any{},
				"effects": []any{map[string]any{
					"kind": "resource", "resource": "hp", "value": 0.0,
					"payloadValueKey": "max_hp_loss_ratio", "payloadValueCoeff": -1.0,
					"valueScale": "max_resource", "minimumHp": 1.0,
				}},
			},
			{
				"id": "ordinary_patient_hot_on_turn_start", "buffId": "ordinary_patient_hot",
				"trigger": "on_turn_start", "priority": 110, "conditions": []any{},
				"effects": []any{map[string]any{"kind": "resource", "resource": "hp", "value": 100.0}},
			},
		},
	}}}
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format,
		Players: append([]battleUnitState{healer}, scriptedPlayers...), Enemies: enemies,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
		CombatTrialObjectiveTargetIDs: map[string]struct{}{scriptedPlayers[0].UnitID: {}},
		CombatTrialSupportUnitIDs:     map[string]struct{}{scriptedPlayers[0].UnitID: {}},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	if err := server.applyCombatTrialEncounterInitialBuffs(&state, 0); err != nil {
		t.Fatalf("apply patient initial buff: %v", err)
	}
	if buffs := state.Players[1].Buffs; len(buffs) != 1 || buffs[0].BuffID != "combat_trial_patient_deterioration" || buffs[0].RemainTurns != -1 || buffs[0].Payload["max_hp_loss_ratio"] != 0.05 {
		t.Fatalf("patient initial buff mismatch: %#v", buffs)
	}
	state.Players[1].Buffs = append(state.Players[1].Buffs, battleBuffState{
		BuffID: "ordinary_patient_hot", RemainTurns: 2, SourceID: healer.UnitID,
	})
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	runtime.ApplyHeal(battleskills.HealAction{SourceID: healer.UnitID, TargetID: scriptedPlayers[0].UnitID, Amount: 80_000_000})
	if got := state.CombatTrialMetrics["effective_healing"]; got != 50_000_000 {
		t.Fatalf("effective healing = %v, want 50000000", got)
	}
	payload := server.executeUnitTurn(&state, time.UnixMilli(2), &state.Players[0], "player", false)
	rows := battleAnyMapSlice(payload["actionLog"])
	if state.Players[1].HP != 95_000_000 {
		t.Fatalf("patient deterioration mismatch: hp=%v", state.Players[1].HP)
	}
	resourceRows := make([]map[string]any, 0, 1)
	for _, row := range rows {
		if row["kind"] == "resource" && row["targetId"] == scriptedPlayers[0].UnitID {
			resourceRows = append(resourceRows, row)
		}
	}
	if len(resourceRows) != 1 || resourceRows[0]["reason"] != "on_turn_start" || resourceRows[0]["amount"] != float64(-5_000_000) {
		t.Fatalf("patient deterioration replay row mismatch: %#v", rows)
	}
	battleRecordCombatTrialMainAction(&state, &state.Players[0], false, payload)
	if state.CombatTrialActionCount != 1 || state.Players[1].HP != 95_000_000 {
		t.Fatalf("patient deterioration mismatch: actions=%d hp=%v", state.CombatTrialActionCount, state.Players[1].HP)
	}
	for _, row := range battleAnyMapSlice(payload["actionLog"]) {
		if strings.HasPrefix(fmt.Sprintf("%v", row["reason"]), "combat_trial_rule:") {
			t.Fatalf("new recovery format must not append legacy post-action deterioration: %#v", row)
		}
	}
	battleCombatTrialRecordHeal(&state, state.Players[1].UnitID, &state.Players[1], &state.Players[1], 100)
	if got := state.CombatTrialMetrics["effective_healing"]; got != 50_000_000 {
		t.Fatalf("scripted ally must not contribute to score: %v", got)
	}
	state.Players[1].HP = 2
	server.battleTriggerCombatTrialSupportTurnStartHooks(&state, time.UnixMilli(3), &state.Players[0], false)
	if state.Players[1].HP != 1 || !state.Players[1].Alive {
		t.Fatalf("patient deterioration must remain non-lethal: %#v", state.Players[1])
	}
	state.CombatTrialEndReason = "action_budget_reached"
	battleCombatTrialRecordBattleEndMetrics(&state)
	if got := state.CombatTrialMetrics["objective_remaining_hp"]; got != 1 {
		t.Fatalf("remaining patient hp = %v, want 1", got)
	}
	combatTrialRestoreSupportTurnExclusions(&state)
	for _, unitID := range state.Order {
		if unitID == scriptedPlayers[0].UnitID {
			t.Fatalf("scripted patient entered normal turn order: %#v", state.Order)
		}
	}
	selected := server.battleSelectTemplateTargets(&state, &state.Players[0], battleSkillState{Level: 1}, battleSkillTargeting{
		TargetScope: "ally", TargetPositions: []string{"vanguard", "midguard", "rearguard"},
		SelectRule: "hp_percent_min", ExcludeSelf: true, TargetCount: battleSkillTargetCount{Base: 1},
	}, false, func() float64 { return 0.5 })
	if len(selected) != 1 || selected[0].UnitID != scriptedPlayers[0].UnitID {
		t.Fatalf("scripted patient must remain heal-targetable: %#v", selected)
	}
}

func TestCombatTrialSupportTurnStartAdapterDoesNotAffectOrdinaryBattles(t *testing.T) {
	player := combatTrialTestUnit("player:1", "player")
	patient := combatTrialTestUnit("support:patient", "player")
	patient.Buffs = []battleBuffState{{
		BuffID: "combat_trial_patient_deterioration", RemainTurns: -1,
		Payload: map[string]any{"max_hp_loss_ratio": 0.05}, SourceID: patient.UnitID,
	}}
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{BuffEffects: []map[string]any{{
		"id": "combat_trial_patient_deterioration_on_turn_start", "buffId": "combat_trial_patient_deterioration",
		"trigger": "on_turn_start", "priority": 100, "conditions": []any{},
		"effects": []any{map[string]any{
			"kind": "resource", "resource": "hp", "value": 0.0, "payloadValueKey": "max_hp_loss_ratio",
			"payloadValueCoeff": -1.0, "valueScale": "max_resource", "minimumHp": 1.0,
		}},
	}}}}}
	state := battleRuntimeState{
		BattleKind: "dungeon", Players: []battleUnitState{player, patient},
		CombatTrialSupportUnitIDs: map[string]struct{}{patient.UnitID: {}},
	}
	if actions := server.battleTriggerCombatTrialSupportTurnStartHooks(&state, time.UnixMilli(1), &state.Players[0], false); len(actions) != 0 {
		t.Fatalf("ordinary battle unexpectedly ran combat-trial support hooks: %#v", actions)
	}
	if state.Players[1].HP != patient.HP {
		t.Fatalf("ordinary battle patient hp changed: got %v want %v", state.Players[1].HP, patient.HP)
	}
}

func TestCombatTrialWaveRosterAdvancesWithoutOrdinarySettlement(t *testing.T) {
	player := combatTrialTestUnit("player:1", "player")
	player.UserID = 1
	player.HP = 75
	player.EquipmentHooks = []battleEquipmentHookRule{{
		ID: "wave_opening_damage", SourceKind: "equipment", SourceItemID: "test_wave_weapon", Trigger: "on_wave_start",
		Effects: []map[string]any{{
			"kind": "damage", "targetMode": "enemy", "selectionMode": "highest_attack", "damageType": "physical",
			"amountFormula": map[string]any{"baseAdd": 10, "terms": []any{}},
		}},
	}}
	firstEnemy := combatTrialTestUnit("combat_trial:opponent:first_enemy", "enemy")
	firstEnemy.HP = 0
	firstEnemy.Alive = false
	nextEnemy := combatTrialTestUnit("ignored", "enemy")
	nextEnemy.EnemyID = "second_enemy"
	nextEnemy.HP = 20
	format := &combatTrialFormatDefinition{
		ID: "gauntlet", SchemaVersion: 2, RulesetVersion: combatTrialFormatRulesetVersion, BalanceRevision: 1, Enabled: true,
		ActionBudget: 36,
		Metrics: []combatTrialMetricDefinition{{
			ID: "actions_to_victory", SchemaVersion: 2, EventType: "battle_end", Aggregation: "final", ValueSource: "player_main_actions",
			SourceScope: "player_side", TargetScope: "objective_units", Ranked: true, RankDirection: "asc",
			EligibleEndReasons: []string{"all_waves_cleared"}, Enabled: true,
		}},
		Encounter: combatTrialEncounterDefinition{Kind: "wave_roster", Waves: []combatTrialEncounterWaveDefinition{
			{ID: "first", Units: []combatTrialEncounterUnitDefinition{{ID: "first_enemy", Kind: "authored_enemy", EnemyID: "first_enemy", Position: "vanguard", Behavior: "active", HealthMode: "normal", ObjectiveTarget: true}}},
			{ID: "second", Units: []combatTrialEncounterUnitDefinition{{ID: "second_enemy", Kind: "authored_enemy", EnemyID: "second_enemy", Position: "rearguard", Behavior: "active", HealthMode: "normal", ObjectiveTarget: true, MaxHP: 100, ResolvedUnit: &nextEnemy}}},
		}},
		EndConditions: []combatTrialEndConditionDefinition{
			{Kind: "player_team_defeated", Reason: "team_defeated"},
			{Kind: "encounter_completed", Reason: "all_waves_cleared"},
			{Kind: "player_main_actions_reached", Reason: "action_budget_reached"},
		},
	}
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format,
		Wave: 1, Players: []battleUnitState{player}, Enemies: []battleUnitState{firstEnemy},
		CombatTrialActionCount: 17,
		CombatTrialMetrics:     map[string]float64{}, CombatTrialObjectiveTargetIDs: map[string]struct{}{
			firstEnemy.UnitID: {}, combatTrialEncounterRuntimeUnitID(format.Encounter.Waves[1].Units[0]): {},
		},
		CombatTrialSupportUnitIDs:       map[string]struct{}{},
		CombatTrialDefeatedObjectiveIDs: map[string]struct{}{},
	}
	combatTrialCaptureObjectiveDefeats(&state)
	advanced, err := (&Server{}).advanceCombatTrialWave(&state, time.UnixMilli(2))
	if err != nil || !advanced {
		t.Fatalf("advance combat trial wave: advanced=%t err=%v", advanced, err)
	}
	if state.CompletedBattles != 1 || state.CombatTrialWaveIndex != 1 || state.Wave != 2 || len(state.Enemies) != 1 || state.Enemies[0].UnitID != "combat_trial:opponent:second_enemy" {
		t.Fatalf("wave transition state mismatch: %#v", state)
	}
	if state.Players[0].HP != 75 || state.Enemies[0].HP != 90 || len(state.Events) != 2 || state.Events[0].Type != "battle.wave.win" || state.Events[1].Type != "battle.wave.spawn" {
		t.Fatalf("wave transition did not preserve player state, opening effects, or replay events: player_hp=%v enemy_hp=%v hooks=%#v events=%#v", state.Players[0].HP, state.Enemies[0].HP, state.Players[0].EquipmentHooks, state.Events)
	}
	state.Enemies[0].HP = 0
	state.Enemies[0].Alive = false
	combatTrialCaptureObjectiveDefeats(&state)
	advanced, err = (&Server{}).advanceCombatTrialWave(&state, time.UnixMilli(3))
	if err != nil || advanced || !state.CombatTrialEncounterCompleted || state.CompletedBattles != 2 {
		t.Fatalf("complete combat trial encounter: advanced=%t completed=%d done=%t err=%v", advanced, state.CompletedBattles, state.CombatTrialEncounterCompleted, err)
	}
	reason, reached := combatTrialReachedEndCondition(&state)
	if !reached || reason != "all_waves_cleared" || len(state.CombatTrialDefeatedObjectiveIDs) != 2 {
		t.Fatalf("gauntlet completion mismatch: reached=%t reason=%q defeated=%#v", reached, reason, state.CombatTrialDefeatedObjectiveIDs)
	}
	state.CombatTrialEndReason = reason
	battleCombatTrialRecordBattleEndMetrics(&state)
	if state.CombatTrialMetrics["actions_to_victory"] != 17 || combatTrialRankDirections(format, reason)["actions_to_victory"] != "asc" {
		t.Fatalf("gauntlet clear-turn metric mismatch: metrics=%#v directions=%#v", state.CombatTrialMetrics, combatTrialRankDirections(format, reason))
	}
}

func TestCombatTrialFormatDamageModifierIsDataDriven(t *testing.T) {
	format := combatTrialTestFormat()
	format.Modifiers = []combatTrialModifierDefinition{{
		ID: "arcane_focus", Kind: "outgoing_damage_multiplier", SourceScope: "player_side", IncludeSummons: true, DamageType: "magical", Multiplier: 1.5,
	}}
	player := combatTrialTestUnit("player:1", "player")
	dummies := combatTrialDummyUnits(format)
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{player}, Enemies: dummies, CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	runtime := newBattleSkillActionRuntime(&state, &state.Players[0], time.UnixMilli(1), nil, nil)
	physical := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: player.UnitID, TargetID: dummies[0].UnitID, Amount: 10, DamageType: "physical", SuppressEquipmentHooks: true,
		Meta: map[string]any{"skipFinalDamageModifiers": true},
	})
	magical := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: player.UnitID, TargetID: dummies[1].UnitID, Amount: 10, DamageType: "magical", SuppressEquipmentHooks: true,
		Meta: map[string]any{"skipFinalDamageModifiers": true},
	})
	if physical.Amount != 10 || magical.Amount != 15 {
		t.Fatalf("format modifier mismatch: physical=%v magical=%v", physical.Amount, magical.Amount)
	}
}

func TestCombatTrialFormatDamageModifierCanExcludeSummons(t *testing.T) {
	format := combatTrialTestFormat()
	format.Modifiers = []combatTrialModifierDefinition{{
		ID: "arcane_focus", Kind: "outgoing_damage_multiplier", SourceScope: "player_side", IncludeSummons: false, DamageType: "magical", Multiplier: 1.5,
	}}
	summon := combatTrialTestUnit("summon_player_1_fire_1", "player")
	summon.SummonTemplateUnitID = "fire"
	summon.SummonSourceUnitID = "player:1"
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &summon)
	if got := battleCombatTrialDamageAmplification(&state, summon.UnitID, &summon, "magical"); got != 0 {
		t.Fatalf("summon amplification = %v, want 0 when summons are excluded", got)
	}
}

func TestCombatTrialMainActionCounter(t *testing.T) {
	format := combatTrialTestFormat()
	state := battleRuntimeState{BattleKind: battleKindCombatTrial, CombatTrialFormat: format}
	player := combatTrialTestUnit("player:1", "player")
	enemy := combatTrialTestUnit("enemy:1", "enemy")
	payload := map[string]any{}
	battleRecordCombatTrialMainAction(&state, &player, false, payload)
	battleRecordCombatTrialMainAction(&state, &player, true, payload)
	battleRecordCombatTrialMainAction(&state, &enemy, false, payload)
	battleRecordCombatTrialMainAction(&state, &player, false, payload)
	if state.CombatTrialActionCount != 2 {
		t.Fatalf("action count = %d, want 2", state.CombatTrialActionCount)
	}
	if payload["combatTrialActionCount"] != 2 || payload["combatTrialActionBudget"] != 20 {
		t.Fatalf("action payload mismatch: %#v", payload)
	}
}

func TestCombatTrialTwentiethActionSettlesMirrorChainWithoutDoubleCounting(t *testing.T) {
	server := newMirrorCastTestServer()
	state := newMirrorCastTestState()
	format := combatTrialTestFormat()
	state.BattleKind = battleKindCombatTrial
	state.CombatTrialFormat = format
	state.CombatTrialMetrics = map[string]float64{}
	state.CombatTrialPlayerSourceIDs = map[string]struct{}{}
	state.CombatTrialActionCount = 19
	state.Enemies = combatTrialDummyUnits(format)
	state.Order = []string{state.Players[0].UnitID, state.Players[1].UnitID, state.Enemies[0].UnitID, state.Enemies[1].UnitID, state.Enemies[2].UnitID}
	state.FullOrder = append([]string(nil), state.Order...)
	for index := range state.Players {
		battleCombatTrialRegisterPlayerSource(state, &state.Players[index])
	}

	if _, _, err := server.processBattleTurn(context.Background(), state, battleWorldConfig{}, nil, time.UnixMilli(1), nil); err != nil {
		t.Fatalf("process twentieth combat trial action: %v", err)
	}
	if state.CombatTrialActionCount != 20 {
		t.Fatalf("combat trial action count = %d, want 20", state.CombatTrialActionCount)
	}
	if actions := battleTurnActionEventsForTest(state); len(actions) != 2 {
		t.Fatalf("twentieth action mirror chain was not fully settled: %#v", state.Events)
	}
	if total, segment := state.CombatTrialMetrics["damage_total"], state.CombatTrialMetrics["damage_max_segment"]; total <= segment || segment <= 0 {
		t.Fatalf("mirror damage was not included as a child segment: metrics=%#v", state.CombatTrialMetrics)
	}
	for index := range state.Enemies {
		if !state.Enemies[index].Alive || state.Enemies[index].HP != state.Enemies[index].MaxHP {
			t.Fatalf("dummy %d changed after twentieth action chain: %#v", index, state.Enemies[index])
		}
	}
}

func TestCombatTrialDummyTurnSettlesWithoutAttacking(t *testing.T) {
	format := combatTrialTestFormat()
	dummy := combatTrialDummyUnits(format)[0]
	dummy.Buffs = []battleBuffState{{BuffID: "test_status", RemainTurns: 1, SourceID: "player:1"}}
	state := battleRuntimeState{BattleKind: battleKindCombatTrial, Enemies: []battleUnitState{dummy}, CombatTrialFormat: format}
	payload := (&Server{}).executeUnitTurn(&state, time.UnixMilli(1), &state.Enemies[0], "enemy", false)
	meta, _ := payload["resultMeta"].(map[string]any)
	if payload["skillId"] != "" || meta["reason"] != battleEffectTurnActionSuppressed {
		t.Fatalf("dummy turn should be passive: %#v", payload)
	}
	if got := state.CompletedTurnCounts[dummy.UnitID]; got != 1 {
		t.Fatalf("dummy completed turns = %d, want 1", got)
	}
	if len(state.Enemies[0].Buffs) != 0 {
		t.Fatalf("dummy status did not settle: %#v", state.Enemies[0].Buffs)
	}
}

func TestCombatTrialDummyPassiveTurnCountsDamageOverTime(t *testing.T) {
	format := combatTrialTestFormat()
	format.Dummy.Defense = 0
	player := combatTrialTestUnit("player:1", "player")
	dummy := combatTrialDummyUnits(format)[0]
	dummy.Buffs = []battleBuffState{{BuffID: "trial_dot", RemainTurns: 2, SourceID: player.UnitID}}
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Buff: []map[string]any{{"id": "trial_dot", "dispellable": true}},
		BuffEffects: []map[string]any{{
			"id": "trial_dot_turn_start", "buffId": "trial_dot", "trigger": "on_turn_start",
			"conditions": []any{}, "effects": []any{map[string]any{"kind": "damage", "value": 37.0, "damageType": "magical"}},
		}},
	}}}
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, Players: []battleUnitState{player}, Enemies: []battleUnitState{dummy}, CombatTrialFormat: format,
		CombatTrialMetrics: map[string]float64{}, CombatTrialPlayerSourceIDs: map[string]struct{}{},
	}
	battleCombatTrialRegisterPlayerSource(&state, &state.Players[0])
	server.executeUnitTurn(&state, time.UnixMilli(1), &state.Enemies[0], "enemy", false)
	if got := state.CombatTrialMetrics["damage_total"]; got != 37 {
		t.Fatalf("damage_total from damage-over-time = %v, want 37", got)
	}
	if got := state.CombatTrialMetrics["damage_max_segment"]; got != 37 {
		t.Fatalf("damage_max_segment from damage-over-time = %v, want 37", got)
	}
	if !state.Enemies[0].Alive || state.Enemies[0].HP != state.Enemies[0].MaxHP {
		t.Fatalf("damage-over-time must leave dummy at full health: %#v", state.Enemies[0])
	}
}

func TestCombatTrialTeamDefeatCompletesWithActualActionCount(t *testing.T) {
	format := combatTrialTestFormat()
	format.MetricIDs = []string{"damage_total", "damage_max_segment"}
	digest, err := combatTrialDefinitionDigest(*format)
	if err != nil {
		t.Fatalf("digest combat trial format: %v", err)
	}
	format.DefinitionDigest = digest
	formatRaw, err := json.Marshal(format)
	if err != nil {
		t.Fatalf("marshal combat trial format: %v", err)
	}
	player := combatTrialTestUnit("player:defeated", "player")
	player.UserID = 1
	player.UnitType = "player"
	player.HP = 1
	player.Buffs = []battleBuffState{{BuffID: "trial_self_damage", RemainTurns: 2, SourceID: "combat_trial:dummy:1"}}
	lineupRaw, err := json.Marshal([]battleUnitState{player})
	if err != nil {
		t.Fatalf("marshal combat trial lineup: %v", err)
	}
	server := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Buff: []map[string]any{{"id": "trial_self_damage", "dispellable": true}},
		BuffEffects: []map[string]any{{
			"id": "trial_self_damage_turn_start", "buffId": "trial_self_damage", "trigger": "on_turn_start",
			"conditions": []any{}, "effects": []any{map[string]any{"kind": "damage", "value": 1000.0, "damageType": "magical"}},
		}},
	}}}
	_, _, _, _, metrics, actionCount, endReason, err := server.compileCombatTrialRun(context.Background(), store.CombatTrialRun{
		Seed: 1, Lineup: lineupRaw, FormatSnapshot: formatRaw, FormatDefinitionDigest: digest, ActionBudget: 20,
	}, time.UnixMilli(1))
	if err != nil {
		t.Fatalf("compile defeated combat trial: %v", err)
	}
	if endReason != "team_defeated" || actionCount != 1 || metrics["damage_total"] != 0 || metrics["damage_max_segment"] != 0 {
		t.Fatalf("defeated combat trial result mismatch: reason=%q actions=%d metrics=%#v", endReason, actionCount, metrics)
	}
}

func TestCombatTrialFormatDigestIncludesLiveDefenseK(t *testing.T) {
	gameData := catalog.Data{
		CombatTrialMetrics: []map[string]any{
			{"id": "damage_total", "schemaVersion": 2, "eventType": "damage", "aggregation": "sum", "sourceScope": "player_side", "targetScope": "objective_units", "ranked": true, "rankDirection": "desc", "enabled": true},
			{"id": "damage_max_segment", "schemaVersion": 2, "eventType": "damage", "aggregation": "max", "sourceScope": "player_side", "targetScope": "objective_units", "ranked": true, "rankDirection": "desc", "enabled": true},
		},
		CombatTrialFormats: []map[string]any{{
			"id": "standard", "schemaVersion": 2, "rulesetVersion": combatTrialFormatRulesetVersion, "balanceRevision": 1,
			"enabled": true, "metricIds": []any{"damage_total", "damage_max_segment"},
			"lineupRules": map[string]any{"positionPolicy": "shared"},
			"limits":      []any{map[string]any{"kind": "player_main_actions", "value": 20}},
			"encounter": map[string]any{"kind": "unit_roster", "units": []any{
				map[string]any{"id": "front", "kind": "training_target", "position": "vanguard", "behavior": "passive", "healthMode": "immutable_full", "objectiveTarget": true, "defenseSource": "combat_balance_defense_k", "maxHp": 1_000_000_000_000, "damageResist": 0.2, "criticalDamageResist": 0.5},
				map[string]any{"id": "middle", "kind": "training_target", "position": "midguard", "behavior": "passive", "healthMode": "immutable_full", "objectiveTarget": true, "defenseSource": "combat_balance_defense_k", "maxHp": 1_000_000_000_000, "damageResist": 0.1, "criticalDamageResist": 0.5},
				map[string]any{"id": "rear", "kind": "training_target", "position": "rearguard", "behavior": "passive", "healthMode": "immutable_full", "objectiveTarget": true, "defenseSource": "combat_balance_defense_k", "maxHp": 1_000_000_000_000, "damageResist": 0.0, "criticalDamageResist": 0.5},
			}},
			"endConditions": []any{map[string]any{"kind": "player_team_defeated", "reason": "team_defeated"}, map[string]any{"kind": "player_main_actions_reached", "reason": "action_budget_reached"}},
			"triggers":      []any{},
		}},
	}
	server := &Server{deps: app.Dependencies{GameData: gameData, Config: config.Config{SharedGlobalSettings: config.SharedGlobalSettings{CombatBalanceModel: config.CombatBalanceModelSettings{DefenseK: 120}}}}}
	first, err := server.resolveCombatTrialFormat("standard")
	if err != nil {
		t.Fatalf("resolve first format: %v", err)
	}
	server.deps.Config.SharedGlobalSettings.CombatBalanceModel.DefenseK = 180
	second, err := server.resolveCombatTrialFormat("standard")
	if err != nil {
		t.Fatalf("resolve second format: %v", err)
	}
	if first.Dummy.Defense != 120 || second.Dummy.Defense != 180 || first.DefinitionDigest == second.DefinitionDigest {
		t.Fatalf("DefenseK snapshot/digest mismatch: first=%#v second=%#v", first, second)
	}
	targets, err := combatTrialEncounterUnits(first)
	if err != nil {
		t.Fatalf("resolve training targets: %v", err)
	}
	if len(targets) != 3 || targets[0].Stats["physics_damage_resist"] != 0.2 || targets[1].Stats["physics_damage_resist"] != 0.1 || targets[2].Stats["physics_damage_resist"] != 0 {
		t.Fatalf("lane damage resistance mismatch: %#v", targets)
	}
	server.deps.Config.SharedGlobalSettings.CombatBalanceModel.DefenseK = 999
	if got := server.battleDefenseKForState(&battleRuntimeState{BattleKind: battleKindCombatTrial, CombatTrialFormat: first}); got != 120 {
		t.Fatalf("combat trial runtime DefenseK = %v, want frozen 120", got)
	}
	if math.IsNaN(first.Dummy.Defense) || len(first.Metrics) != 2 {
		t.Fatalf("resolved format invalid: %#v", first)
	}
	snapshot, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal format snapshot: %v", err)
	}
	if _, err := decodeCombatTrialFormatSnapshot(snapshot); err != nil {
		t.Fatalf("decode valid format snapshot: %v", err)
	}
	legacyV2 := *first
	legacyV2.LineupRules = nil
	legacyV2.DefinitionDigest = ""
	legacyV2.DefinitionDigest, err = combatTrialDefinitionDigest(legacyV2)
	if err != nil {
		t.Fatalf("digest legacy V2 format snapshot: %v", err)
	}
	legacyV2Raw, err := json.Marshal(legacyV2)
	if err != nil {
		t.Fatalf("marshal legacy V2 format snapshot: %v", err)
	}
	decodedLegacyV2, err := decodeCombatTrialFormatSnapshot(legacyV2Raw)
	if err != nil || decodedLegacyV2.LineupRules == nil || decodedLegacyV2.LineupRules.PositionPolicy != "distinct" {
		t.Fatalf("decode legacy V2 lineup policy: format=%#v err=%v", decodedLegacyV2, err)
	}
	invalidPolicy := *first
	invalidPolicy.LineupRules = &combatTrialLineupRulesDefinition{PositionPolicy: "unknown"}
	invalidPolicy.DefinitionDigest = ""
	invalidPolicy.DefinitionDigest, err = combatTrialDefinitionDigest(invalidPolicy)
	if err != nil {
		t.Fatalf("digest invalid lineup policy: %v", err)
	}
	invalidPolicyRaw, err := json.Marshal(invalidPolicy)
	if err != nil {
		t.Fatalf("marshal invalid lineup policy: %v", err)
	}
	if _, err := decodeCombatTrialFormatSnapshot(invalidPolicyRaw); err == nil {
		t.Fatal("digest-valid snapshot with an unknown lineup policy must fail semantic validation")
	}
	tampered := *first
	tampered.Dummy.Defense = 121
	tamperedSnapshot, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("marshal tampered format snapshot: %v", err)
	}
	if _, err := decodeCombatTrialFormatSnapshot(tamperedSnapshot); err == nil {
		t.Fatal("tampered format snapshot must fail digest validation")
	}
	invalid := *first
	invalid.Encounter.Units = append([]combatTrialEncounterUnitDefinition(nil), first.Encounter.Units...)
	invalid.Encounter.Units[0].Behavior = "active"
	invalid.DefinitionDigest = ""
	invalidDigest, err := combatTrialDefinitionDigest(invalid)
	if err != nil {
		t.Fatalf("digest invalid semantic snapshot: %v", err)
	}
	invalid.DefinitionDigest = invalidDigest
	invalidSnapshot, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal invalid semantic snapshot: %v", err)
	}
	if _, err := decodeCombatTrialFormatSnapshot(invalidSnapshot); err == nil {
		t.Fatal("digest-valid snapshot with an active training target must fail semantic validation")
	}
	duplicateMetricIDs := *first
	duplicateMetricIDs.MetricIDs = []string{"damage_total", "damage_total"}
	duplicateMetricIDs.DefinitionDigest = ""
	duplicateMetricIDs.DefinitionDigest, err = combatTrialDefinitionDigest(duplicateMetricIDs)
	if err != nil {
		t.Fatalf("digest duplicate metric-id snapshot: %v", err)
	}
	duplicateMetricIDsRaw, err := json.Marshal(duplicateMetricIDs)
	if err != nil {
		t.Fatalf("marshal duplicate metric-id snapshot: %v", err)
	}
	if _, err := decodeCombatTrialFormatSnapshot(duplicateMetricIDsRaw); err == nil {
		t.Fatal("digest-valid snapshot with duplicate metric ids must fail semantic validation")
	}
	unknownMetricReason := *first
	unknownMetricReason.Metrics = append([]combatTrialMetricDefinition(nil), first.Metrics...)
	unknownMetricReason.Metrics[0].EligibleEndReasons = []string{"unknown_finish"}
	unknownMetricReason.DefinitionDigest = ""
	unknownMetricReason.DefinitionDigest, err = combatTrialDefinitionDigest(unknownMetricReason)
	if err != nil {
		t.Fatalf("digest unknown metric-reason snapshot: %v", err)
	}
	unknownMetricReasonRaw, err := json.Marshal(unknownMetricReason)
	if err != nil {
		t.Fatalf("marshal unknown metric-reason snapshot: %v", err)
	}
	if _, err := decodeCombatTrialFormatSnapshot(unknownMetricReasonRaw); err == nil {
		t.Fatal("digest-valid snapshot with an unavailable metric end reason must fail semantic validation")
	}
	gameData.CombatTrialFormats[0]["modifiers"] = []any{map[string]any{
		"id": "invalid_amp", "kind": "outgoing_damage_multiplier", "sourceScope": "player_side",
		"includeSummons": true, "damageType": "magical", "multiplier": 0,
	}}
	if _, err := server.resolveCombatTrialFormat("standard"); err == nil {
		t.Fatal("runtime format loading must reject an invalid authored modifier before a run is queued")
	}
}

func TestCombatTrialV2FormatInheritanceFreezesAuthoredEnemyAndResultMetric(t *testing.T) {
	metrics := []map[string]any{
		{"id": "damage_total", "schemaVersion": 2, "eventType": "damage", "aggregation": "sum", "sourceScope": "player_side", "targetScope": "objective_units", "ranked": true, "rankDirection": "desc", "enabled": true},
		{"id": "damage_max_segment", "schemaVersion": 2, "eventType": "damage", "aggregation": "max", "sourceScope": "player_side", "targetScope": "objective_units", "ranked": true, "rankDirection": "desc", "enabled": true},
		{"id": "actions_to_victory", "schemaVersion": 2, "eventType": "battle_end", "aggregation": "final", "valueSource": "player_main_actions", "sourceScope": "player_side", "targetScope": "objective_units", "ranked": true, "rankDirection": "asc", "eligibleEndReasons": []any{"enemy_team_defeated"}, "enabled": true},
	}
	trainingUnit := func(id, position string) map[string]any {
		return map[string]any{
			"id": id, "kind": "training_target", "position": position, "behavior": "passive", "healthMode": "immutable_full",
			"objectiveTarget": true, "defenseSource": "combat_balance_defense_k", "maxHp": 1_000_000,
		}
	}
	formats := []map[string]any{
		{
			"id": "standard", "schemaVersion": 2, "rulesetVersion": combatTrialFormatRulesetVersion, "balanceRevision": 1, "enabled": true,
			"metricIds":   []any{"damage_total", "damage_max_segment"},
			"lineupRules": map[string]any{"positionPolicy": "shared"},
			"limits":      []any{map[string]any{"kind": "player_main_actions", "value": 20}},
			"encounter": map[string]any{"kind": "unit_roster", "units": []any{
				trainingUnit("front", "vanguard"), trainingUnit("middle", "midguard"), trainingUnit("rear", "rearguard"),
			}},
			"endConditions": []any{map[string]any{"kind": "player_team_defeated", "reason": "team_defeated"}, map[string]any{"kind": "player_main_actions_reached", "reason": "action_budget_reached"}},
			"triggers":      []any{},
		},
		{
			"id": "boss_sprint", "baseFormatId": "standard", "schemaVersion": 2, "rulesetVersion": combatTrialFormatRulesetVersion, "balanceRevision": 1, "enabled": true,
			"metricIds": []any{"actions_to_victory"},
			"encounter": map[string]any{"kind": "unit_roster", "units": []any{map[string]any{
				"id": "boss", "kind": "authored_enemy", "enemyId": "trial_boss", "position": "midguard", "behavior": "active", "healthMode": "normal", "objectiveTarget": true,
			}}},
			"endConditions": []any{
				map[string]any{"kind": "player_team_defeated", "reason": "team_defeated"},
				map[string]any{"kind": "enemy_team_defeated", "reason": "enemy_team_defeated"},
				map[string]any{"kind": "player_main_actions_reached", "reason": "action_budget_reached"},
			},
			"triggers": []any{map[string]any{
				"id": "arcane_surge", "event": "damage_calculation", "effects": []any{map[string]any{
					"kind": "outgoing_damage_multiplier", "sourceScope": "player_side", "includeSummons": true, "damageType": "magical", "multiplier": 1.25,
				}},
			}},
		},
	}
	server := &Server{deps: app.Dependencies{
		GameData: catalog.Data{
			CombatTrialMetrics: metrics,
			CombatTrialFormats: formats,
			Enemies: []map[string]any{{
				"id": "trial_boss", "name": "Trial Boss", "nameKey": "enemy.trial_boss.name", "position": "vanguard",
				"level": 50, "type": "boss", "hp": 5000.0, "speed": 20.0, "skillIds": []any{},
			}},
		},
		Config: config.Config{SharedGlobalSettings: config.SharedGlobalSettings{CombatBalanceModel: config.CombatBalanceModelSettings{DefenseK: 150}}},
	}}
	standard, err := server.resolveCombatTrialFormat("standard")
	if err != nil {
		t.Fatalf("resolve standard format: %v", err)
	}
	format, err := server.resolveCombatTrialFormat("boss_sprint")
	if err != nil {
		t.Fatalf("resolve inherited boss format: %v", err)
	}
	if format.ActionBudget != 20 || format.LineupRules == nil || format.LineupRules.PositionPolicy != "shared" || len(format.Encounter.Units) != 1 || format.Encounter.Units[0].ResolvedUnit == nil {
		t.Fatalf("inherited format did not resolve its limit and enemy snapshot: %#v", format)
	}
	if got := format.Encounter.Units[0].ResolvedUnit; got.EnemyID != "trial_boss" || got.Position != "midguard" || got.RewardEligible {
		t.Fatalf("authored enemy snapshot mismatch: %#v", got)
	}
	if format.DefinitionDigest == standard.DefinitionDigest || len(format.Triggers) != 1 {
		t.Fatalf("format inheritance did not create an independent rules digest: %#v", format)
	}
	raw, err := json.Marshal(format)
	if err != nil {
		t.Fatalf("marshal boss format: %v", err)
	}
	decoded, err := decodeCombatTrialFormatSnapshot(raw)
	if err != nil {
		t.Fatalf("decode frozen boss format: %v", err)
	}
	enemies, err := combatTrialEncounterUnits(decoded)
	if err != nil || len(enemies) != 1 || battleCombatTrialPassiveTarget(&battleRuntimeState{BattleKind: battleKindCombatTrial, CombatTrialFormat: decoded}, &enemies[0]) {
		t.Fatalf("active boss encounter runtime mismatch: enemies=%#v err=%v", enemies, err)
	}
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: decoded, CombatTrialActionCount: 7,
		CombatTrialEndReason: "enemy_team_defeated", CombatTrialMetrics: map[string]float64{},
	}
	battleCombatTrialRecordBattleEndMetrics(&state)
	if state.CombatTrialMetrics["actions_to_victory"] != 7 {
		t.Fatalf("battle-end metric value = %v, want 7", state.CombatTrialMetrics["actions_to_victory"])
	}
	if got := combatTrialRankDirections(decoded, "enemy_team_defeated"); got["actions_to_victory"] != "asc" {
		t.Fatalf("victory metric rank direction mismatch: %#v", got)
	}
	if got := combatTrialRankDirections(decoded, "action_budget_reached"); len(got) != 0 {
		t.Fatalf("failed boss attempt must not enter the victory leaderboard: %#v", got)
	}
	state.CombatTrialEndReason = "action_budget_reached"
	state.CombatTrialMetrics["actions_to_victory"] = 0
	battleCombatTrialRecordBattleEndMetrics(&state)
	if state.CombatTrialMetrics["actions_to_victory"] != 0 {
		t.Fatalf("failed boss attempt must not report a victory action count: %#v", state.CombatTrialMetrics)
	}
	player := combatTrialTestUnit("player:1", "player")
	battleCombatTrialRegisterPlayerSource(&state, &player)
	if got := battleCombatTrialDamageAmplification(&state, player.UnitID, &player, "magical"); got != 0.25 {
		t.Fatalf("inherited DSL trigger magical amplification = %v, want 0.25", got)
	}
}

func TestCombatTrialEnemyStatProfileFreezesWaveStatsWithoutMutatingBaseEnemy(t *testing.T) {
	server := &Server{deps: app.Dependencies{
		GameData: catalog.Data{Enemies: []map[string]any{{
			"id": "profile_enemy", "name": "Profile Enemy", "nameKey": "enemy.profile_enemy.name", "position": "vanguard",
			"level": 40, "type": "normal", "hp": 100.0, "hp_regen": 4.0, "strength": 10.0,
			"physics_damage": 20.0, "speed": 5.0, "skillIds": []any{},
		}}},
		Config: config.Config{SharedGlobalSettings: config.SharedGlobalSettings{CombatBalanceModel: config.CombatBalanceModelSettings{
			DefenseK: 100, EnemyHpRegenRatio: 1,
		}}},
	}}
	profile := &combatTrialEnemyStatProfileDefinition{
		DifficultyLevel: 20,
		Multiplier:      21,
		FlatBonuses: map[string]float64{
			"hp": 3050, "physics_damage": 129, "physics_defence": 129, "physics_damage_resist": 0.138,
		},
	}
	format := &combatTrialFormatDefinition{
		ID: "gauntlet", DefenseK: 100,
		Encounter: combatTrialEncounterDefinition{Kind: "wave_roster", Waves: []combatTrialEncounterWaveDefinition{{
			ID: "crown", EnemyStatProfile: profile, Units: []combatTrialEncounterUnitDefinition{{
				ID: "profile_target", Kind: "authored_enemy", EnemyID: "profile_enemy", Position: "rearguard",
				Behavior: "active", HealthMode: "normal", ObjectiveTarget: true, StatOverrides: map[string]float64{
					"hp": 1250000, "physics_damage": 9000, "physics_defence": 400, "crit_damage_resist": 0.8,
				},
			}},
		}}},
	}

	baseBefore := server.buildBattleEnemyUnit(server.battleEnemyCatalog()["profile_enemy"], "base:before", false, server.battleDefaultBasicSkillID())
	if err := server.resolveCombatTrialEncounter(format); err != nil {
		t.Fatalf("resolve profiled wave encounter: %v", err)
	}
	definition := format.Encounter.Waves[0].Units[0]
	resolved := definition.ResolvedUnit
	if resolved == nil {
		t.Fatal("profiled enemy snapshot is missing")
	}
	expected := map[string]float64{
		"hp": 1250000, "hp_regen": 84, "strength": 210, "physics_damage": 9000,
		"physics_defence": 400, "physics_damage_resist": 0.138, "speed": 105, "crit_damage_resist": 0.8,
	}
	for key, want := range expected {
		if got := resolved.Stats[key]; got != want {
			t.Fatalf("profiled %s=%v, want %v", key, got, want)
		}
	}
	if definition.MaxHP != 1250000 || !combatTrialStatMapsEqual(definition.ResolvedStats, resolved.Stats) {
		t.Fatalf("public stat snapshot was not frozen with the resolved enemy: %#v", definition)
	}
	players, enemies, err := combatTrialEncounterParticipants(format, 0)
	if err != nil || len(players) != 0 || len(enemies) != 1 || enemies[0].MaxHP != 1250000 || enemies[0].Stats["physics_damage"] != 9000 {
		t.Fatalf("profiled runtime participant mismatch: players=%#v enemies=%#v err=%v", players, enemies, err)
	}
	server.battleRecomputeUnitStats(&enemies[0])
	if enemies[0].MaxHP != 1250000 || enemies[0].Stats["physics_damage"] != 9000 || enemies[0].Stats["physics_defence"] != 400 {
		t.Fatalf("profiled stats did not survive runtime recompute: %#v", enemies[0].Stats)
	}
	baseAfter := server.buildBattleEnemyUnit(server.battleEnemyCatalog()["profile_enemy"], "base:after", false, server.battleDefaultBasicSkillID())
	if baseBefore.MaxHP != 100 || baseAfter.MaxHP != baseBefore.MaxHP || baseAfter.Stats["physics_damage"] != baseBefore.Stats["physics_damage"] {
		t.Fatalf("combat trial profile leaked into the ordinary enemy builder: before=%#v after=%#v", baseBefore.Stats, baseAfter.Stats)
	}

	public := combatTrialPublicFormat(*format)
	publicUnit := public.Encounter.Waves[0].Units[0]
	if publicUnit.ResolvedUnit != nil || !combatTrialStatMapsEqual(publicUnit.ResolvedStats, resolved.Stats) {
		t.Fatalf("public format must expose only the stat preview, not the internal unit: %#v", publicUnit)
	}
	publicUnit.ResolvedStats["hp"] = 1
	publicUnit.StatOverrides["hp"] = 1
	public.Encounter.Waves[0].EnemyStatProfile.FlatBonuses["hp"] = 1
	if resolved.Stats["hp"] != 1250000 || definition.StatOverrides["hp"] != 1250000 || profile.FlatBonuses["hp"] != 3050 {
		t.Fatal("public stat preview aliases the frozen runtime snapshot")
	}
}

func TestCombatTrialEnemyStatProfileRejectsInvalidDSL(t *testing.T) {
	tests := []combatTrialEnemyStatProfileDefinition{
		{DifficultyLevel: 0, Multiplier: 1},
		{DifficultyLevel: 20, Multiplier: 0},
		{DifficultyLevel: 20, Multiplier: 1, FlatBonuses: map[string]float64{"not_a_combat_stat": 1}},
		{DifficultyLevel: 20, Multiplier: 1, FlatBonuses: map[string]float64{"hp": math.Inf(1)}},
	}
	for index := range tests {
		if err := validateCombatTrialEnemyStatProfile(&tests[index]); err == nil {
			t.Fatalf("invalid enemy stat profile %d was accepted: %#v", index, tests[index])
		}
	}
}

func TestCombatTrialEnemyStatOverridesRejectInvalidDSL(t *testing.T) {
	tests := []map[string]float64{
		{"hp": 0},
		{"mp": -1},
		{"crit_damage_resist": 1.01},
		{"physics_damage_resist": -0.01},
		{"not_a_combat_stat": 1},
		{"hp": math.Inf(1)},
	}
	for index := range tests {
		if err := validateCombatTrialEnemyStatOverrides(tests[index]); err == nil {
			t.Fatalf("invalid enemy stat overrides %d were accepted: %#v", index, tests[index])
		}
	}
}

func TestCombatTrialEndConditionsAreEvaluatedFromFormatOrder(t *testing.T) {
	format := combatTrialTestFormat()
	format.EndConditions = []combatTrialEndConditionDefinition{
		{Kind: "enemy_team_defeated", Reason: "enemy_team_defeated"},
		{Kind: "player_main_actions_reached", Reason: "action_budget_reached"},
	}
	player := combatTrialTestUnit("player:1", "player")
	player.UserID = 1
	enemy := combatTrialTestUnit("enemy:1", "enemy")
	enemy.Alive = false
	enemy.HP = 0
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format, Players: []battleUnitState{player}, Enemies: []battleUnitState{enemy},
		CombatTrialActionCount: 20,
	}
	reason, reached := combatTrialReachedEndCondition(&state)
	if !reached || reason != "enemy_team_defeated" {
		t.Fatalf("format end-condition order mismatch: reached=%t reason=%q", reached, reason)
	}
}

func TestCombatTrialOwnsTerminalSettlementInsteadOfOrdinaryBattle(t *testing.T) {
	format := combatTrialTestFormat()
	format.Encounter = combatTrialEncounterDefinition{Kind: "unit_roster", Units: []combatTrialEncounterUnitDefinition{{
		ID: "boss", Kind: "authored_enemy", Position: "vanguard", Behavior: "active", HealthMode: "normal", ObjectiveTarget: true,
	}}}
	format.EndConditions = []combatTrialEndConditionDefinition{
		{Kind: "player_team_defeated", Reason: "team_defeated"},
		{Kind: "enemy_team_defeated", Reason: "enemy_team_defeated"},
		{Kind: "player_main_actions_reached", Reason: "action_budget_reached"},
	}
	player := combatTrialTestUnit("player:1", "player")
	player.UserID = 1
	boss := combatTrialTestUnit("combat_trial:opponent:boss", "enemy")
	boss.HP = 0
	boss.Alive = false
	state := battleRuntimeState{
		BattleKind: battleKindCombatTrial, CombatTrialFormat: format, Players: []battleUnitState{player}, Enemies: []battleUnitState{boss},
		CombatTrialMetrics: map[string]float64{},
	}
	server := &Server{}
	status, advanced, err := server.processBattleTurn(context.Background(), &state, battleWorldConfig{}, nil, time.UnixMilli(1), map[int64]store.BattleWaveReward{})
	if err != nil || status != store.BattleRunStatusEnded || !advanced {
		t.Fatalf("combat trial terminal precheck mismatch: status=%q advanced=%t err=%v", status, advanced, err)
	}
	if state.CompletedBattles != 0 || len(state.Events) != 0 {
		t.Fatalf("ordinary battle settlement leaked into combat trial: completed=%d events=%#v", state.CompletedBattles, state.Events)
	}
	reason, reached := combatTrialReachedEndCondition(&state)
	if !reached || reason != "enemy_team_defeated" {
		t.Fatalf("combat trial end condition mismatch: reached=%t reason=%q", reached, reason)
	}
	server.finishCombatTrial(&state, time.UnixMilli(1), reason, false)
	if len(state.Events) != 1 || state.Events[0].Type != "battle.ended" || state.Events[0].Data["reason"] != "enemy_team_defeated" {
		t.Fatalf("combat trial must emit exactly one authored terminal event: %#v", state.Events)
	}
}

func TestCombatTrialPublicPayloadStripsResolvedEnemyAndKeepsFrozenRunSummary(t *testing.T) {
	format := combatTrialTestFormat()
	format.NameKey = "game.combatTrial.formats.standard.name"
	format.ShortDescKey = "game.combatTrial.formats.standard.shortDesc"
	format.Icon = "lucide:target"
	digest, err := combatTrialDefinitionDigest(*format)
	if err != nil {
		t.Fatalf("digest combat trial format: %v", err)
	}
	format.DefinitionDigest = digest
	raw, err := json.Marshal(format)
	if err != nil {
		t.Fatalf("marshal combat trial format: %v", err)
	}
	payload := combatTrialPublicRun(store.CombatTrialRun{ID: 8, FormatID: format.ID, FormatDefinitionDigest: digest, FormatSnapshot: raw})
	if len(payload.FormatSnapshot) != 0 || payload.Format == nil || payload.Format.DefinitionDigest != digest || len(payload.Format.Metrics) != 2 {
		t.Fatalf("public run frozen format summary mismatch: %#v", payload)
	}

	resolvedEnemy := combatTrialTestUnit("combat_trial:opponent:boss", "enemy")
	public := combatTrialPublicFormat(combatTrialFormatDefinition{Encounter: combatTrialEncounterDefinition{Units: []combatTrialEncounterUnitDefinition{{ID: "boss", ResolvedUnit: &resolvedEnemy}}}})
	if public.Encounter.Units[0].ResolvedUnit != nil {
		t.Fatalf("public format exposed internal resolved enemy snapshot: %#v", public.Encounter.Units[0])
	}
}
