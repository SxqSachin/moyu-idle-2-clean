package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func arenaTestThreeLayerBoxesDefinition() arenaFormatDefinition {
	return arenaFormatDefinition{
		ID: "three_layer_boxes", BaseFormatID: arenaFormatStandardID, SchemaVersion: 1, RulesetVersion: arenaFormatRulesetVersion,
		BalanceRevision: 1, NameKey: "game.arena.formats.three_layer_boxes.name",
		ShortDescKey: "game.arena.formats.three_layer_boxes.shortDesc", DescKey: "game.arena.formats.three_layer_boxes.desc",
		Icon: "game-icons:cardboard-box", SortOrder: 20, Enabled: true,
		Triggers: []arenaFormatTrigger{{
			ID: "opening_boxes", Event: "battle_start",
			Effects: []arenaFormatEffect{{
				Kind: "turn_barrier", TargetScope: "all_units", BuffID: "arena_format_three_layer_boxes", Charges: 3,
			}},
		}},
	}
}

func arenaTestStandardDefinition() arenaFormatDefinition {
	return arenaFormatDefinition{
		ID: arenaFormatStandardID, SchemaVersion: 1, RulesetVersion: arenaFormatRulesetVersion,
		BalanceRevision: 1, NameKey: "game.arena.formats.standard.name",
		ShortDescKey: "game.arena.formats.standard.shortDesc", DescKey: "game.arena.formats.standard.desc",
		Icon: "game-icons:heart-shield", SortOrder: 0, Enabled: true,
		Triggers: []arenaFormatTrigger{{
			ID: "standard_matchup_balance", Event: "battle_start",
			Effects: []arenaFormatEffect{
				{
					Kind: "matchup_hp_buff", TargetScope: "formal_units_by_side", BuffID: arenaMatchupHPBuffID, PayloadKey: "hpBonus",
					MatchupHP: &arenaFormatMatchupHPAdjustment{LevelSource: "player_battle_level", BothAtLeastLevel: 400, BothSidesFlatBonus: 50000, LowerSideBonusPerLevel: 100},
				},
				{
					Kind: "warmup_buff", TargetScope: "eligible_side_units", IncludeSummons: true, BuffID: arenaOutputWarmupBuffID,
					Warmup: &arenaFormatWarmup{
						LevelSource: "player_battle_level", HigherSideGapExceeds: 100, AlwaysAboveLevel: 400, NormalActions: 2,
						StartMultiplier: 0.4, RecoveryPerNormalAction: 0.3, MaximumMultiplier: 1, Outputs: []string{"damage", "heal"},
					},
				},
			},
		}},
	}
}

func arenaTestNaturalTalentsDefinition() arenaFormatDefinition {
	disabled := false
	return arenaFormatDefinition{
		ID: "natural_talents", BaseFormatID: arenaFormatStandardID, SchemaVersion: 1, RulesetVersion: arenaFormatRulesetVersion,
		BalanceRevision: 1, NameKey: "game.arena.formats.natural_talents.name",
		ShortDescKey: "game.arena.formats.natural_talents.shortDesc", DescKey: "game.arena.formats.natural_talents.desc",
		Icon: "lucide:circle-off", SortOrder: 10, Enabled: true,
		Triggers: []arenaFormatTrigger{{
			ID: "natural_talents_specialization_policy", Event: "lineup_snapshot",
			Effects: []arenaFormatEffect{{
				Kind: "specialization_policy", TargetScope: "player_units", SpecializationEnabled: &disabled,
			}},
		}},
	}
}

func arenaTestLivelyArrivalsDefinition() arenaFormatDefinition {
	return arenaFormatDefinition{
		ID: "lively_arrivals", BaseFormatID: arenaFormatStandardID, SchemaVersion: 1, RulesetVersion: arenaFormatRulesetVersion,
		BalanceRevision: 1, NameKey: "game.arena.formats.lively_arrivals.name",
		ShortDescKey: "game.arena.formats.lively_arrivals.shortDesc", DescKey: "game.arena.formats.lively_arrivals.desc",
		Icon: "game-icons:cat", SortOrder: 30, Enabled: true,
		Triggers: []arenaFormatTrigger{{
			ID: "arrival_ripple", Event: "after_summon",
			Effects: []arenaFormatEffect{{
				Kind: "damage", TargetScope: "all_alive_units", IncludeSummons: true,
				DamageType: "physical", SkipFinalDamageModifiers: true, Simultaneous: true,
				Amount: arenaFormatAmount{Basis: "target_max_hp", Multiplier: 0.04, Flat: 30, Minimum: 1},
			}},
		}},
	}
}

func arenaTestFormatRow(definition arenaFormatDefinition) map[string]any {
	raw, _ := json.Marshal(definition)
	var row map[string]any
	_ = json.Unmarshal(raw, &row)
	return row
}

func TestArenaLeaderboardShowsTopTwentyFiveOnly(t *testing.T) {
	if arenaLeaderboardLimit != 25 {
		t.Fatalf("arena leaderboard limit=%d, want 25", arenaLeaderboardLimit)
	}
}

func TestArenaChallengeChargePayloadUsesAuthoritativeState(t *testing.T) {
	serverTime := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	nextRecovery := serverTime.Add(7 * time.Minute)
	payload := arenaChallengeChargePayload(config.ArenaSettings{OpponentChallengeCooldownSeconds: 900}, store.ArenaChallengeChargeState{
		ServerTime: serverTime, Remaining: 4, Capacity: 10, RecoverySeconds: 600, NextRecoveryAt: &nextRecovery,
	})
	if payload["serverTime"] != serverTime || payload["remaining"] != 4 || payload["capacity"] != 10 {
		t.Fatalf("unexpected challenge charge payload: %#v", payload)
	}
	if payload["recoverySeconds"] != int64(600) || payload["nextRecoveryAt"] != &nextRecovery || payload["opponentCooldownSeconds"] != int64(900) {
		t.Fatalf("unexpected challenge charge policy payload: %#v", payload)
	}
}

func TestArenaMatchupHPAdjustmentUsesAuthoredStandardRule(t *testing.T) {
	rule := arenaTestStandardDefinition().Triggers[0].Effects[0].MatchupHP
	tests := []struct {
		ownLevel      int64
		opponentLevel int64
		want          float64
	}{
		{ownLevel: 500, opponentLevel: 500, want: 50000},
		{ownLevel: 500, opponentLevel: 401, want: 50000},
		{ownLevel: 401, opponentLevel: 500, want: 59900},
		{ownLevel: 500, opponentLevel: 400, want: 50000},
		{ownLevel: 400, opponentLevel: 500, want: 60000},
		{ownLevel: 500, opponentLevel: 399, want: 0},
		{ownLevel: 399, opponentLevel: 500, want: 10100},
		{ownLevel: 500, opponentLevel: 200, want: 0},
		{ownLevel: 200, opponentLevel: 500, want: 30000},
	}
	for _, test := range tests {
		if got := arenaFormatMatchupHPBonus(rule, test.ownLevel, test.opponentLevel); got != test.want {
			t.Fatalf("levels %d/%d bonus=%.0f, want %.0f", test.ownLevel, test.opponentLevel, got, test.want)
		}
	}
}

func TestArenaWarmupEligibilityUsesStrictThresholds(t *testing.T) {
	rule := arenaTestStandardDefinition().Triggers[0].Effects[1].Warmup
	tests := []struct {
		ownLevel, opponentLevel int64
		want                    bool
	}{
		{ownLevel: 400, opponentLevel: 300, want: false},
		{ownLevel: 400, opponentLevel: 299, want: true},
		{ownLevel: 401, opponentLevel: 400, want: true},
		{ownLevel: 400, opponentLevel: 401, want: false},
		{ownLevel: 500, opponentLevel: 401, want: true},
		{ownLevel: 401, opponentLevel: 500, want: true},
	}
	for _, test := range tests {
		if got := arenaFormatWarmupEligible(rule, test.ownLevel, test.opponentLevel); got != test.want {
			t.Fatalf("levels %d/%d eligible=%v, want %v", test.ownLevel, test.opponentLevel, got, test.want)
		}
	}
}

func TestNormalizeArenaSelectionAllowsOptionalKitties(t *testing.T) {
	valid := []arenaLineupSelection{
		{PlayerPosition: "vanguard", FormatID: arenaFormatStandardID},
		{PlayerPosition: "midguard", FormatID: arenaFormatStandardID, Kitties: []arenaLineupKittySelection{{KittyID: 10, Position: "vanguard"}}},
		{PlayerPosition: "rearguard", FormatID: arenaFormatStandardID, Kitties: []arenaLineupKittySelection{{KittyID: 10, Position: "vanguard"}, {KittyID: 11, Position: "midguard"}}},
		{PlayerPosition: "vanguard", FormatID: arenaFormatStandardID, Kitties: []arenaLineupKittySelection{{KittyID: 10, Position: "vanguard"}, {KittyID: 11, Position: "vanguard"}}},
	}
	for _, selection := range valid {
		if _, err := normalizeArenaSelection(selection); err != nil {
			t.Fatalf("selection %+v should be valid: %v", selection, err)
		}
	}

	invalid := []arenaLineupSelection{
		{PlayerPosition: "vanguard"},
		{PlayerPosition: "", FormatID: arenaFormatStandardID},
		{PlayerPosition: "vanguard", FormatID: arenaFormatStandardID, Kitties: []arenaLineupKittySelection{{KittyID: 0, Position: "midguard"}}},
		{PlayerPosition: "vanguard", FormatID: arenaFormatStandardID, Kitties: []arenaLineupKittySelection{{KittyID: 10, Position: "midguard"}, {KittyID: 10, Position: "rearguard"}}},
		{PlayerPosition: "vanguard", FormatID: arenaFormatStandardID, Kitties: []arenaLineupKittySelection{{KittyID: 10, Position: "midguard"}, {KittyID: 11, Position: "rearguard"}, {KittyID: 12, Position: "vanguard"}}},
	}
	for _, selection := range invalid {
		if _, err := normalizeArenaSelection(selection); err == nil {
			t.Fatalf("selection %+v should be invalid", selection)
		}
	}
}

func TestArenaFormatSpecializationPolicyComesFromFormatDSL(t *testing.T) {
	standard := arenaTestStandardDefinition()
	if !arenaFormatSpecializationEnabled(&standard) {
		t.Fatal("standard format should keep player specialization enabled")
	}
	natural := arenaTestNaturalTalentsDefinition()
	if arenaFormatSpecializationEnabled(&natural) {
		t.Fatal("natural talents should disable player specialization")
	}
}

func TestArenaStandardFormatAppliesMatchupHPAndWarmupBuffs(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	s := &Server{deps: app.Dependencies{
		Config: config.Config{GlobalSettings: settings},
		GameData: catalog.Data{
			Buff: []map[string]any{
				{
					"id": arenaMatchupHPBuffID, "durationTurns": -1, "permanent": true,
					"kind": "buff", "stackPolicy": "refresh_duration", "dispellable": false,
				},
				{
					"id": arenaOutputWarmupBuffID, "durationTurns": -1, "permanent": true, "maxCharges": 2,
					"kind": "buff", "stackPolicy": "refresh_duration", "dispellable": false,
				},
			},
			BuffEffects: []map[string]any{
				{
					"id": arenaMatchupHPRuleID, "buffId": arenaMatchupHPBuffID, "trigger": "while_active",
					"effects": []any{map[string]any{"kind": "add_stat", "attr": "hp", "value": 0, "payloadValueKey": "hpBonus"}},
				},
				{
					"id": "arena_output_warmup_while_active", "buffId": arenaOutputWarmupBuffID, "trigger": "while_active",
					"effects": []any{map[string]any{
						"kind": "linear_output_multiplier", "outputs": []any{"damage", "heal"},
						"startMultiplier": 0.4, "startMultiplierPayloadKey": "startMultiplier",
						"recoveryPerChargeConsumed": 0.3, "recoveryPerChargeConsumedPayloadKey": "recoveryPerNormalAction",
						"maximumMultiplier": 1.0, "maximumMultiplierPayloadKey": "maximumMultiplier",
					}},
				},
			},
		},
	}}
	format, err := normalizeArenaFormatDefinition(arenaTestStandardDefinition(), true)
	if err != nil {
		t.Fatal(err)
	}
	state := battleRuntimeState{
		BattleKind: battleKindArena, ArenaFormat: &format,
		Players: []battleUnitState{
			{UnitID: "player:1", UnitType: "player", BattleLevel: 500, Alive: true, HP: 100, MaxHP: 100, BaseStats: map[string]float64{"hp": 100}, Stats: map[string]float64{"hp": 100}, StatMode: battleStatModeProjected},
			{UnitID: "kitty:2", UnitType: "kitty", KittyID: 2, BattleLevel: 300, Alive: true, HP: 80, MaxHP: 80, BaseStats: map[string]float64{"hp": 80}, Stats: map[string]float64{"hp": 80}, StatMode: battleStatModeProjected},
			{UnitID: "summon_1", UnitType: "kitty", BattleLevel: 500, Alive: true, HP: 50, MaxHP: 50, BaseStats: map[string]float64{"hp": 50}, Stats: map[string]float64{"hp": 50}, StatMode: battleStatModeProjected, SummonTemplateUnitID: "shadow", SummonSourceUnitID: "player:1"},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", UnitType: "player", BattleLevel: 400, Alive: true, HP: 60, MaxHP: 60, BaseStats: map[string]float64{"hp": 60}, Stats: map[string]float64{"hp": 60}, StatMode: battleStatModeProjected},
		},
	}
	actions := s.battleApplyArenaFormatBattleStart(&state)
	if len(actions) != 6 {
		t.Fatalf("standard format actions=%d, want 6", len(actions))
	}
	if state.Players[0].MaxHP != 50100 || state.Players[0].HP != 50100 {
		t.Fatalf("player hp=%v/%v, want 50100/50100", state.Players[0].HP, state.Players[0].MaxHP)
	}
	if state.Players[1].MaxHP != 50080 || state.Players[1].HP != 50080 {
		t.Fatalf("kitty hp=%v/%v, want 50080/50080", state.Players[1].HP, state.Players[1].MaxHP)
	}
	if state.Enemies[0].MaxHP != 60060 || state.Enemies[0].HP != 60060 {
		t.Fatalf("defender hp=%v/%v, want 60060/60060", state.Enemies[0].HP, state.Enemies[0].MaxHP)
	}
	if state.Players[2].MaxHP != 50 || len(state.Players[2].Buffs) != 1 || state.Players[2].Buffs[0].BuffID != arenaOutputWarmupBuffID {
		t.Fatalf("summon should receive only warmup: %+v", state.Players[2])
	}
	for _, unit := range []*battleUnitState{&state.Players[0], &state.Players[1]} {
		if len(unit.Buffs) != 2 || findBuffByID(unit, arenaMatchupHPBuffID) == nil || findBuffByID(unit, arenaOutputWarmupBuffID) == nil {
			t.Fatalf("high-level unit %s arena buffs=%+v", unit.UnitID, unit.Buffs)
		}
	}
	if len(state.Enemies[0].Buffs) != 1 || state.Enemies[0].Buffs[0].BuffID != arenaMatchupHPBuffID {
		t.Fatalf("level 400 defender should receive only matchup HP: %+v", state.Enemies[0].Buffs)
	}
	if s.battleBuffDispellableByID(arenaMatchupHPBuffID) || s.battleBuffDispellableByID(arenaOutputWarmupBuffID) {
		t.Fatal("arena balancing buffs must not be dispellable")
	}
}

func TestArenaWarmupBuffScalesDamageAndHealingPerNormalAction(t *testing.T) {
	s := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Buff: []map[string]any{{
			"id": arenaOutputWarmupBuffID, "durationTurns": -1, "permanent": true, "maxCharges": 2,
			"kind": "buff", "stackPolicy": "refresh_duration", "dispellable": false,
		}},
		BuffEffects: []map[string]any{
			{
				"id": "arena_output_warmup_while_active", "buffId": arenaOutputWarmupBuffID, "trigger": "while_active",
				"conditions": []any{}, "effects": []any{map[string]any{
					"kind": "linear_output_multiplier", "outputs": []any{"damage", "heal"},
					"startMultiplier": 0.4, "startMultiplierPayloadKey": "startMultiplier",
					"recoveryPerChargeConsumed": 0.3, "recoveryPerChargeConsumedPayloadKey": "recoveryPerNormalAction",
					"maximumMultiplier": 1.0, "maximumMultiplierPayloadKey": "maximumMultiplier",
				}},
			},
			{
				"id": "arena_output_warmup_after_normal_action", "buffId": arenaOutputWarmupBuffID, "trigger": "on_turn_settled",
				"eventFilter": map[string]any{"turnKinds": []any{"ordered"}, "includePreparation": false},
				"conditions":  []any{}, "effects": []any{map[string]any{"kind": "consume_charge"}},
			},
		},
	}}}
	unit := battleUnitState{UnitID: "player:1", Alive: true, HP: 100, MaxHP: 100}
	battleApplyOrRefreshBuff(&unit, arenaOutputWarmupBuffID, -1, map[string]any{
		"startMultiplier": 0.4, "recoveryPerNormalAction": 0.3, "maximumMultiplier": 1.0,
	}, "arena_format:standard", "refresh_duration")
	state := &battleRuntimeState{
		Players: []battleUnitState{unit},
		Enemies: []battleUnitState{{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 1000, MaxHP: 1000}},
	}
	actor := &state.Players[0]

	assertOutput := func(want float64) {
		t.Helper()
		if got, _ := s.battleApplyOutgoingBuffModifiers(actor, "damage", 100); got != want {
			t.Fatalf("damage output=%v, want %v", got, want)
		}
		if got, _ := s.battleApplyOutgoingBuffModifiers(actor, "heal", 100); got != want {
			t.Fatalf("heal output=%v, want %v", got, want)
		}
	}
	assertOutput(40)
	runtime := newBattleSkillActionRuntime(state, actor, time.UnixMilli(0), s, []string{"enemy:1"})
	damage := runtime.ApplyDamage(battleskills.DamageAction{
		SourceID: actor.UnitID, TargetID: "enemy:1", Amount: 100, DamageType: "physical", SuppressEquipmentHooks: true,
		Meta: map[string]any{"suppressSourceHooks": true, "suppressBuffHooks": true},
	})
	if damage.Amount != 40 || state.Enemies[0].HP != 960 {
		t.Fatalf("warmup damage pipeline effect=%+v target_hp=%v", damage, state.Enemies[0].HP)
	}
	actor.HP = 10
	heal := runtime.ApplyHeal(battleskills.HealAction{SourceID: actor.UnitID, TargetID: actor.UnitID, Amount: 100})
	if heal.Amount != 40 || actor.HP != 50 {
		t.Fatalf("warmup heal pipeline effect=%+v actor_hp=%v", heal, actor.HP)
	}
	resource := runtime.ApplyResource(battleskills.ResourceAction{
		SourceID: actor.UnitID, TargetID: actor.UnitID, Resource: "hp", Amount: 100,
	})
	if resource.Amount != 40 || actor.HP != 90 {
		t.Fatalf("warmup positive hp resource effect=%+v actor_hp=%v", resource, actor.HP)
	}
	s.battleTriggerUnitBuffHooksWithContext(state, time.UnixMilli(1), actor, nil, "on_turn_settled", &battleEquipmentHookContext{TurnKind: "ordered", IsPreparation: true})
	s.battleTriggerUnitBuffHooksWithContext(state, time.UnixMilli(2), actor, nil, "on_turn_settled", &battleEquipmentHookContext{TurnKind: "extra"})
	if buff := findBuffByID(actor, arenaOutputWarmupBuffID); buff == nil || buff.Triggered != 0 {
		t.Fatalf("preparation and extra turns must not advance warmup: %+v", buff)
	}
	s.battleTriggerUnitBuffHooksWithContext(state, time.UnixMilli(3), actor, nil, "on_turn_end", &battleEquipmentHookContext{TurnKind: "ordered"})
	assertOutput(40)
	s.battleTriggerUnitBuffHooksWithContext(state, time.UnixMilli(4), actor, nil, "on_turn_settled", &battleEquipmentHookContext{TurnKind: "ordered"})
	assertOutput(70)
	s.battleTriggerUnitBuffHooksWithContext(state, time.UnixMilli(5), actor, nil, "on_turn_settled", &battleEquipmentHookContext{TurnKind: "ordered"})
	if findBuffByID(actor, arenaOutputWarmupBuffID) != nil {
		t.Fatal("warmup should detach after two normal ordered actions")
	}
	assertOutput(100)
}

func TestArenaWarmupSettlesAfterEndTurnHealing(t *testing.T) {
	s := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Buff: []map[string]any{
			{"id": arenaOutputWarmupBuffID, "maxCharges": 2, "dispellable": false},
			{"id": "test_end_turn_heal", "dispellable": false},
		},
		BuffEffects: []map[string]any{
			{
				"id": "arena_output_warmup_while_active", "buffId": arenaOutputWarmupBuffID, "trigger": "while_active",
				"conditions": []any{}, "effects": []any{map[string]any{
					"kind": "linear_output_multiplier", "outputs": []any{"damage", "heal"},
					"startMultiplier": 0.4, "startMultiplierPayloadKey": "startMultiplier",
					"recoveryPerChargeConsumed": 0.3, "recoveryPerChargeConsumedPayloadKey": "recoveryPerNormalAction",
					"maximumMultiplier": 1.0, "maximumMultiplierPayloadKey": "maximumMultiplier",
				}},
			},
			{
				"id": "arena_output_warmup_after_normal_action", "buffId": arenaOutputWarmupBuffID, "trigger": "on_turn_settled",
				"eventFilter": map[string]any{"turnKinds": []any{"ordered"}, "includePreparation": false},
				"conditions":  []any{}, "effects": []any{map[string]any{"kind": "consume_charge"}},
			},
			{
				"id": "test_end_turn_heal_rule", "buffId": "test_end_turn_heal", "trigger": "on_turn_end",
				"conditions": []any{}, "effects": []any{map[string]any{"kind": "heal", "value": 100.0}},
			},
		},
	}}}
	state := &battleRuntimeState{
		Players: []battleUnitState{{
			UnitID: "player:1", Side: "player", UnitType: "player", Alive: true, HP: 10, MaxHP: 100,
			Stats: map[string]float64{"hp": 100, "speed": 1}, BaseStats: map[string]float64{"hp": 100, "speed": 1},
			Buffs: []battleBuffState{
				{BuffID: arenaOutputWarmupBuffID, RemainTurns: -1, Payload: map[string]any{
					"startMultiplier": 0.4, "recoveryPerNormalAction": 0.3, "maximumMultiplier": 1.0,
				}},
				{BuffID: "test_end_turn_heal", RemainTurns: -1},
			},
		}},
		Enemies: []battleUnitState{{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100}},
	}

	payload := s.executeUnitTurn(state, time.UnixMilli(1), &state.Players[0], "player", false)
	actor := &state.Players[0]
	if actor.HP != 50 {
		t.Fatalf("end-turn heal should use pre-settlement 40%% output, hp=%v payload=%+v", actor.HP, payload)
	}
	if buff := findBuffByID(actor, arenaOutputWarmupBuffID); buff == nil || buff.Triggered != 1 {
		t.Fatalf("warmup should advance only after end-turn outputs settle: %+v", buff)
	}
	chargeLogged := false
	for _, action := range payload["actionLog"].([]map[string]any) {
		if action["kind"] == "buff_charge" && action["buffId"] == arenaOutputWarmupBuffID && toInt64Loose(action["buffTriggered"], 0) == 1 {
			chargeLogged = true
			break
		}
	}
	if !chargeLogged {
		t.Fatalf("warmup progress must be present in playback actions: %+v", payload["actionLog"])
	}
	if got, _ := s.battleApplyOutgoingBuffModifiers(actor, "heal", 100); got != 70 {
		t.Fatalf("next ordered action should use 70%% output, got=%v", got)
	}
}

func TestArenaWarmupScalesBuffDamageFromItsSource(t *testing.T) {
	s := &Server{deps: app.Dependencies{GameData: catalog.Data{
		Buff: []map[string]any{
			{"id": arenaOutputWarmupBuffID, "maxCharges": 2, "dispellable": false},
			{"id": "test_warmup_dot", "dispellable": true},
		},
		BuffEffects: []map[string]any{
			{
				"id": "arena_output_warmup_while_active", "buffId": arenaOutputWarmupBuffID, "trigger": "while_active",
				"conditions": []any{}, "effects": []any{map[string]any{
					"kind": "linear_output_multiplier", "outputs": []any{"damage", "heal"},
					"startMultiplier": 0.4, "startMultiplierPayloadKey": "startMultiplier",
					"recoveryPerChargeConsumed": 0.3, "recoveryPerChargeConsumedPayloadKey": "recoveryPerNormalAction",
					"maximumMultiplier": 1.0, "maximumMultiplierPayloadKey": "maximumMultiplier",
				}},
			},
			{
				"id": "test_warmup_dot_on_turn_start", "buffId": "test_warmup_dot", "trigger": "on_turn_start",
				"conditions": []any{}, "effects": []any{map[string]any{"kind": "damage", "value": 100.0, "damageType": "magical"}},
			},
		},
	}}}
	state := &battleRuntimeState{
		Players: []battleUnitState{{
			UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100,
			Buffs: []battleBuffState{{BuffID: arenaOutputWarmupBuffID, RemainTurns: -1, Payload: map[string]any{
				"startMultiplier": 0.4, "recoveryPerNormalAction": 0.3, "maximumMultiplier": 1.0,
			}}},
		}},
		Enemies: []battleUnitState{{
			UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 1000, MaxHP: 1000,
			Stats: map[string]float64{"hp": 1000, "speed": 1}, BaseStats: map[string]float64{"hp": 1000, "speed": 1},
			Buffs: []battleBuffState{{BuffID: "test_warmup_dot", RemainTurns: 2, SourceID: "player:1"}},
		}},
	}
	actions := s.battleTriggerUnitBuffHooks(state, time.UnixMilli(1), &state.Enemies[0], "on_turn_start")
	if state.Enemies[0].HP != 960 || len(actions) == 0 {
		t.Fatalf("warmup sourced buff damage should be 40, hp=%v actions=%+v", state.Enemies[0].HP, actions)
	}
}

func TestArenaSummonReceivesFreshWarmupWithoutMatchupHP(t *testing.T) {
	format, err := normalizeArenaFormatDefinition(arenaTestStandardDefinition(), true)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	state := &battleRuntimeState{
		BattleKind: battleKindArena, ArenaFormat: &format,
		Players: []battleUnitState{{UnitID: "player:1", UnitType: "player", Side: "player", BattleLevel: 500, Alive: true}},
		Enemies: []battleUnitState{{UnitID: "enemy:1", UnitType: "player", Side: "enemy", BattleLevel: 200, Alive: true}},
	}
	summon := battleUnitState{
		UnitID: "summon:1", UnitType: "summon", Side: "player", Alive: true,
		SummonTemplateUnitID: "helper", SummonSourceUnitID: "player:1",
	}
	state.Players = append(state.Players, summon)
	s.battleQueueArenaFormatAfterSummon(state, &state.Players[0], &state.Players[1])
	buff := findBuffByID(&state.Players[1], arenaOutputWarmupBuffID)
	if buff == nil || buff.Triggered != 0 {
		t.Fatalf("summon warmup=%+v, want a fresh buff", buff)
	}
	if findBuffByID(&state.Players[1], arenaMatchupHPBuffID) != nil {
		t.Fatal("summon must not receive matchup HP adjustment")
	}
}

func TestArenaRuntimeLineupUsesArenaSidesAndDisablesConsumables(t *testing.T) {
	amountMultiplier := 1.25
	specializationHook := specializationTestHook(
		"arena_snapshot_hook",
		"onTurnStart",
		"arena_snapshot_node",
		map[string]any{"kind": "cycle_counter", "counterId": "arena_snapshot", "gain": 1, "maxStacks": 3},
	)
	lineup := []battleUnitState{
		{
			UserID: 1, UnitType: "player", UnitID: "old-player", Position: "vanguard", Alive: true,
			HP: 10, MaxHP: 100, MP: 1, MaxMP: 20,
			BaseStats: map[string]float64{"hp": 100, "attack": 175}, Stats: map[string]float64{"hp": 100, "attack": 175},
			BattleItems:          []battleItemState{{ItemID: "potion"}},
			SpecializationTreeID: "five_paths", SpecializationRulesetVersion: "specialization-v6",
			SpecializationBalanceRevision: 8, SpecializationRevision: 12, SpecializationDigest: "arena-specialization-digest",
			SpecializationHooks: []battleEquipmentHookRule{specializationHook},
			SpecializationEffectMutations: []battleAffixEffectMutation{{
				ID: "arena_snapshot_mutation", SourceKind: "specialization", SourceID: "arena_snapshot_node",
				Scope: "skill", SelectAll: true, AmountMultiplier: &amountMultiplier,
			}},
		},
		{UserID: 1, UnitType: "kitty", KittyID: 10, UnitID: "old-kitty-a", Position: "midguard", Alive: true, HP: 10, MaxHP: 80, MP: 1, MaxMP: 10},
		{UserID: 1, UnitType: "kitty", KittyID: 11, UnitID: "old-kitty-b", Position: "rearguard", Alive: true, HP: 10, MaxHP: 90, MP: 1, MaxMP: 12},
	}
	raw, err := json.Marshal(lineup)
	if err != nil {
		t.Fatal(err)
	}
	got, err := competitiveRuntimeLineup(raw, "enemy", "arena:defender")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range got {
		if unit.Side != "enemy" || unit.BaseSide != "enemy" || unit.PerspectiveSide != "enemy" {
			t.Fatalf("unit %s side was not rewritten: %+v", unit.UnitID, unit)
		}
		if unit.HP != unit.MaxHP || unit.MP != unit.MaxMP {
			t.Fatalf("unit %s did not start at full resources", unit.UnitID)
		}
		if len(unit.BattleItems) != 0 || len(unit.ItemRules) != 0 {
			t.Fatalf("unit %s retained consumables", unit.UnitID)
		}
	}
	player := got[0]
	if player.SpecializationTreeID != "five_paths" || player.SpecializationRulesetVersion != "specialization-v6" ||
		player.SpecializationBalanceRevision != 8 || player.SpecializationRevision != 12 || player.SpecializationDigest != "arena-specialization-digest" {
		t.Fatalf("defender specialization identity was not preserved: %+v", player)
	}
	if player.Stats["attack"] != 175 || player.BaseStats["attack"] != 175 {
		t.Fatalf("specialization stat contribution was not preserved: stats=%v base=%v", player.Stats, player.BaseStats)
	}
	if len(player.SpecializationHooks) != 1 || player.SpecializationHooks[0].ID != specializationHook.ID || len(player.SpecializationEffectMutations) != 1 {
		t.Fatalf("specialization runtime contribution was not preserved: hooks=%+v mutations=%+v", player.SpecializationHooks, player.SpecializationEffectMutations)
	}
}

func TestArenaRuntimeLineupAllowsPlayerWithOptionalKitties(t *testing.T) {
	for kittyCount := 0; kittyCount <= 2; kittyCount++ {
		lineup := []battleUnitState{{UserID: 1, UnitType: "player", Position: "vanguard", MaxHP: 100, MaxMP: 20}}
		if kittyCount >= 1 {
			lineup = append(lineup, battleUnitState{UserID: 1, UnitType: "kitty", KittyID: 10, Position: "midguard", MaxHP: 80, MaxMP: 10})
		}
		if kittyCount >= 2 {
			lineup = append(lineup, battleUnitState{UserID: 1, UnitType: "kitty", KittyID: 11, Position: "rearguard", MaxHP: 90, MaxMP: 12})
		}
		raw, err := json.Marshal(lineup)
		if err != nil {
			t.Fatal(err)
		}
		got, err := competitiveRuntimeLineup(raw, "player", "arena:attacker")
		if err != nil {
			t.Fatalf("kitty_count=%d: %v", kittyCount, err)
		}
		if len(got) != kittyCount+1 {
			t.Fatalf("kitty_count=%d units=%d", kittyCount, len(got))
		}
	}
}

func TestArenaBatchWirePayloadUsesIndependentStream(t *testing.T) {
	payload := toArenaBattleEventBatchWirePayload(42, wsbus.BattleEventBatchPayload{RunID: 42})
	if payload.K != battleKindArena {
		t.Fatalf("arena wire kind=%q", payload.K)
	}
	if got := battleEventStreamKind(payload); got != battleStreamKindArena {
		t.Fatalf("arena stream kind=%q", got)
	}
	if got := battleEventStreamKind(toBattleEventBatchWirePayload(42, wsbus.BattleEventBatchPayload{RunID: 7})); got != battleStreamKindRoom {
		t.Fatalf("room stream kind=%q", got)
	}
}

func TestArenaAndRoomBattleStreamSubscriptionsAreIndependent(t *testing.T) {
	s := &Server{
		battleStreamSubs:      make(map[string]struct{}),
		arenaBattleStreamSubs: make(map[string]struct{}),
	}
	const clientID = "client-1"

	s.setBattleStreamKindSubscribed(clientID, battleStreamKindRoom, true)
	s.setBattleStreamKindSubscribed(clientID, battleStreamKindArena, true)
	if !s.isBattleStreamKindSubscribed(clientID, battleStreamKindRoom) || !s.isBattleStreamKindSubscribed(clientID, battleStreamKindArena) {
		t.Fatal("room and arena streams should be subscribed simultaneously")
	}

	s.setBattleStreamKindSubscribed(clientID, battleStreamKindArena, false)
	if !s.isBattleStreamKindSubscribed(clientID, battleStreamKindRoom) {
		t.Fatal("disabling arena stream must not disable the room stream")
	}
	if s.isBattleStreamKindSubscribed(clientID, battleStreamKindArena) {
		t.Fatal("arena stream should be disabled independently")
	}

	s.clearBattleStreamSubscribed(clientID)
	if s.isBattleStreamKindSubscribed(clientID, battleStreamKindRoom) || s.isBattleStreamKindSubscribed(clientID, battleStreamKindArena) {
		t.Fatal("disconnect cleanup must clear both stream subscriptions")
	}
}

func TestArenaSpecializationHooksRunSymmetricallyForBothSides(t *testing.T) {
	s := newSpecializationExecutionTestServer()
	attacker := specializationTestUnit("arena:attacker:player:1", "player", 100, 100)
	defender := specializationTestUnit("arena:defender:player:2", "enemy", 100, 100)
	attacker.UnitType = "player"
	defender.UnitType = "player"

	startHook := specializationTestHook(
		"arena_shared_start",
		"onBattleStart",
		"arena_shared_start_node",
		map[string]any{"kind": "mark_target", "markId": "arena_start", "targetMode": "enemy", "selectionMode": "highest_attack"},
	)
	startHook.MaxTriggersPerBattle = 1
	waveHook := specializationTestHook(
		"arena_shared_wave",
		"onWaveStart",
		"arena_shared_wave_node",
		map[string]any{"kind": "cycle_counter", "counterId": "arena_wave", "gain": 1, "maxStacks": 3},
	)
	waveHook.MaxTriggersPerWave = 1
	attacker.SpecializationHooks = []battleEquipmentHookRule{startHook, waveHook}
	defender.SpecializationHooks = []battleEquipmentHookRule{startHook, waveHook}
	state := &battleRuntimeState{
		BattleKind: battleKindArena,
		Wave:       1,
		Players:    []battleUnitState{attacker},
		Enemies:    []battleUnitState{defender},
	}

	startActions := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1))
	if len(startActions) < 2 || !battleHasBuff(&state.Players[0], battleSpecializationMarkBuffID("arena_start")) || !battleHasBuff(&state.Enemies[0], battleSpecializationMarkBuffID("arena_start")) {
		t.Fatalf("battle-start specialization was not symmetric: actions=%+v players=%+v enemies=%+v", startActions, state.Players[0].Buffs, state.Enemies[0].Buffs)
	}

	waveActions := s.battleTriggerCompetitiveWaveStartHooks(state, time.UnixMilli(2))
	if len(waveActions) < 2 || battleSpecializationCounterStacks(&state.Players[0], "arena_wave") != 1 || battleSpecializationCounterStacks(&state.Enemies[0], "arena_wave") != 1 {
		t.Fatalf("wave-start specialization was not symmetric: actions=%+v player=%d defender=%d", waveActions, battleSpecializationCounterStacks(&state.Players[0], "arena_wave"), battleSpecializationCounterStacks(&state.Enemies[0], "arena_wave"))
	}
}

func TestArenaBattleEventCarriesKindMarker(t *testing.T) {
	state := battleRuntimeState{BattleKind: battleKindArena}
	state.emit("battle.test", time.Unix(0, 0), map[string]any{"value": 1})
	if len(state.Events) != 1 || state.Events[0].Data["battleKind"] != battleKindArena {
		t.Fatalf("arena event marker missing: %+v", state.Events)
	}
}

func TestArenaPublicUnitsAddArenaOnlyIdentityFields(t *testing.T) {
	units := []battleUnitState{{
		UnitID: "player:1", UnitType: "player", UserID: 1, BattleLevel: 500,
		Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100},
	}}
	normal := battlePublicUnitSlice(units)
	for _, key := range []string{"unitType", "kittyId", "battleLevel"} {
		if _, exists := normal[0][key]; exists {
			t.Fatalf("normal battle payload unexpectedly contains %s", key)
		}
	}
	arena := competitivePublicUnitSlice(units)
	if arena[0]["unitType"] != "player" || arena[0]["battleLevel"] != int64(500) {
		t.Fatalf("arena identity fields missing: %+v", arena[0])
	}
}

func TestArenaSummonBaseStatsRemoveMatchupHPAdjustment(t *testing.T) {
	state := &battleRuntimeState{BattleKind: battleKindArena}
	source := &battleUnitState{
		HP: 5050, MaxHP: 10100, Stats: map[string]float64{"hp": 10100, "attack": 20},
		StatSources: map[string]map[string]float64{"hp": {arenaMatchupHPRuleID: 10000}},
	}
	stats := battleArenaSummonBaseStats(state, source, map[string]float64{"hp": 10100, "attack": 20})
	if stats["hp"] != 100 || stats["attack"] != 20 {
		t.Fatalf("summon base stats=%v, want hp=100 attack=20", stats)
	}
	formulaSource := battleArenaSummonFormulaSource(state, source)
	if formulaSource == source || formulaSource.MaxHP != 100 || formulaSource.HP != 50 || formulaSource.Stats["hp"] != 100 {
		t.Fatalf("summon formula source=%+v, want base hp 50/100", formulaSource)
	}
}

func TestArenaFormatCatalogPublishesStandardAndFrozenDefinitions(t *testing.T) {
	boxes := arenaTestThreeLayerBoxesDefinition()
	boxes.BaseFormatID = arenaFormatStandardID
	boxes.SortOrder = 20
	lively := arenaTestLivelyArrivalsDefinition()
	lively.BaseFormatID = arenaFormatStandardID
	lively.SortOrder = 30
	s := &Server{deps: app.Dependencies{GameData: catalog.Data{ArenaFormats: []map[string]any{
		arenaTestFormatRow(lively),
		arenaTestFormatRow(arenaTestNaturalTalentsDefinition()),
		arenaTestFormatRow(arenaTestStandardDefinition()),
		arenaTestFormatRow(boxes),
	}}}}

	formats, err := s.arenaPublicFormats()
	if err != nil {
		t.Fatalf("load public formats: %v", err)
	}
	if len(formats) != 4 || formats[0].ID != arenaFormatStandardID || !formats[0].Standard || formats[1].ID != "natural_talents" || formats[2].ID != "three_layer_boxes" || formats[3].ID != "lively_arrivals" {
		t.Fatalf("public format order=%+v", formats)
	}
	if formats[0].DefinitionDigest == "" || formats[1].DefinitionDigest == "" || formats[2].DefinitionDigest == "" || formats[3].DefinitionDigest == "" {
		t.Fatalf("authored formats must expose immutable definition digests: %+v", formats)
	}
	if len(formats[0].Rules) != 2 || formats[0].Rules[0].EffectKind != "matchup_hp_buff" || formats[0].Rules[0].MatchupHP == nil ||
		formats[0].Rules[1].EffectKind != "warmup_buff" || formats[0].Rules[1].Warmup == nil {
		t.Fatalf("standard format public rules=%+v", formats[0].Rules)
	}
	if len(formats[1].Rules) != 3 || formats[1].Rules[2].EffectKind != "specialization_policy" || formats[1].Rules[2].SpecializationEnabled == nil || *formats[1].Rules[2].SpecializationEnabled {
		t.Fatalf("natural talents public rules=%+v", formats[1].Rules)
	}
	if len(formats[2].Rules) != 3 || formats[2].Rules[2].Event != "battle_start" || formats[2].Rules[2].EffectKind != "turn_barrier" || formats[2].Rules[2].Charges != 3 {
		t.Fatalf("barrier format public rules=%+v", formats[2].Rules)
	}
	if len(formats[3].Rules) != 3 || formats[3].Rules[2].Event != "after_summon" || formats[3].Rules[2].Amount == nil || formats[3].Rules[2].Amount.Multiplier != 0.04 || formats[3].Rules[2].Amount.Flat != 30 {
		t.Fatalf("summon format public rules=%+v", formats[3].Rules)
	}

	frozen, err := s.resolveArenaFormatForPublish("three_layer_boxes")
	if err != nil || frozen == nil {
		t.Fatalf("resolve published format: definition=%+v err=%v", frozen, err)
	}
	raw, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeArenaFormatSnapshot(raw)
	if err != nil || decoded == nil || decoded.DefinitionDigest != frozen.DefinitionDigest {
		t.Fatalf("decode frozen snapshot: definition=%+v err=%v", decoded, err)
	}
	if len(decoded.Triggers) != 2 || decoded.Triggers[0].Effects[0].MatchupHP == nil || decoded.Triggers[0].Effects[1].Warmup == nil {
		t.Fatalf("frozen child must contain inherited standard rules: %+v", decoded.Triggers)
	}
	if _, err := decodeArenaFormatSnapshot(json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing format snapshot should be rejected")
	}

	var tampered arenaFormatDefinition
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.BalanceRevision++
	tamperedRaw, _ := json.Marshal(tampered)
	if _, err := decodeArenaFormatSnapshot(tamperedRaw); err == nil {
		t.Fatal("tampered frozen format snapshot should fail its definition digest")
	}
}

func TestGeneratedArenaFormatCatalogIncludesEveryPlayableFormatAndRules(t *testing.T) {
	data, err := catalog.Load("../../../packages/game-data/generated/game-data.json")
	if err != nil {
		t.Fatalf("load generated game data: %v", err)
	}
	s := &Server{deps: app.Dependencies{GameData: data}}
	formats, err := s.arenaPublicFormats()
	if err != nil {
		t.Fatalf("load generated arena formats: %v", err)
	}
	wantIDs := []string{"standard", "natural_talents", "three_layer_boxes", "lively_arrivals"}
	if len(formats) != len(wantIDs) {
		t.Fatalf("generated arena format count=%d, want %d: %+v", len(formats), len(wantIDs), formats)
	}
	for index, wantID := range wantIDs {
		if formats[index].ID != wantID || len(formats[index].Rules) == 0 {
			t.Fatalf("generated arena format[%d]=%+v, want id=%s with public rules", index, formats[index], wantID)
		}
	}
	if len(formats[1].Rules) != 3 || formats[1].Rules[2].EffectKind != "specialization_policy" || formats[1].Rules[2].SpecializationEnabled == nil || *formats[1].Rules[2].SpecializationEnabled {
		t.Fatalf("generated natural-talents rules=%+v", formats[1].Rules)
	}
}

func TestArenaThreeLayerBoxesConsumesOnceAndBlocksForWholeTurn(t *testing.T) {
	format, err := normalizeArenaFormatDefinition(arenaTestThreeLayerBoxesDefinition(), true)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	state := battleRuntimeState{
		BattleKind:  battleKindArena,
		ArenaFormat: &format,
		Players:     []battleUnitState{{UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100}},
		Enemies:     []battleUnitState{{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 100, MaxHP: 100}},
	}
	if actions := s.battleApplyArenaFormatBattleStart(&state); len(actions) != 2 {
		t.Fatalf("battle-start format actions=%d, want 2", len(actions))
	}
	if len(state.Players[0].Buffs) != 1 || len(state.Enemies[0].Buffs) != 1 {
		t.Fatalf("format barrier missing: player=%+v enemy=%+v", state.Players[0].Buffs, state.Enemies[0].Buffs)
	}

	apply := func(turnSerial int64) battleskills.SkillEffect {
		state.activeBuffDurationTurnSerial = turnSerial
		runtime := newBattleSkillActionRuntime(&state, &state.Enemies[0], time.UnixMilli(turnSerial), s, []string{"player:1"})
		return runtime.ApplyDamage(battleskills.DamageAction{
			SourceID: "enemy:1", TargetID: "player:1", Amount: 40, DamageType: "physical",
			SuppressEquipmentHooks: true,
			Meta:                   map[string]any{"skipFinalDamageModifiers": true, "suppressSourceHooks": true, "suppressBuffHooks": true},
		})
	}

	if effect := apply(1); effect.Amount != 0 || state.Players[0].HP != 100 {
		t.Fatalf("first hit should be blocked: effect=%+v hp=%v", effect, state.Players[0].HP)
	}
	barrier := state.arenaFormatBarriers[state.arenaFormatBarrierOrder[0]]
	if barrier == nil || barrier.RemainingCharges != 2 {
		t.Fatalf("first hit should consume one layer: %+v", barrier)
	}
	if effect := apply(1); effect.Amount != 0 || barrier.RemainingCharges != 2 {
		t.Fatalf("later damage in the same turn should be free immunity: effect=%+v barrier=%+v", effect, barrier)
	}
	if effect := apply(2); effect.Amount != 0 || barrier.RemainingCharges != 1 {
		t.Fatalf("next turn should consume the second layer: effect=%+v barrier=%+v", effect, barrier)
	}
	if effect := apply(3); effect.Amount != 0 || barrier.RemainingCharges != 0 {
		t.Fatalf("third protected turn should consume the final layer: effect=%+v barrier=%+v", effect, barrier)
	}
	if effect := apply(3); effect.Amount != 0 || state.Players[0].HP != 100 {
		t.Fatalf("final layer must protect the rest of its turn: effect=%+v hp=%v", effect, state.Players[0].HP)
	}
	if actions := s.battleFinalizeArenaFormatTurn(&state); len(actions) != 1 || len(state.Players[0].Buffs) != 0 {
		t.Fatalf("exhausted barrier should detach after the turn: actions=%+v buffs=%+v", actions, state.Players[0].Buffs)
	}
	if effect := apply(4); effect.Amount != 40 || state.Players[0].HP != 60 {
		t.Fatalf("damage should resume after all layers are spent: effect=%+v hp=%v", effect, state.Players[0].HP)
	}
}

func TestArenaLivelyArrivalsDamagesEveryLivingUnitFromFrozenMaxHP(t *testing.T) {
	format, err := normalizeArenaFormatDefinition(arenaTestLivelyArrivalsDefinition(), true)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	state := battleRuntimeState{
		BattleKind:  battleKindArena,
		ArenaFormat: &format,
		Players: []battleUnitState{
			{UnitID: "player:1", Side: "player", Alive: true, HP: 100, MaxHP: 100},
			{UnitID: "summon:1", Side: "player", Alive: true, HP: 50, MaxHP: 50, SummonTemplateUnitID: "helper", SummonSourceUnitID: "player:1"},
		},
		Enemies: []battleUnitState{{UnitID: "enemy:1", Side: "enemy", Alive: true, HP: 200, MaxHP: 200}},
	}
	s.battleQueueArenaFormatAfterSummon(&state, &state.Players[0], &state.Players[1])
	actions := s.battleDrainArenaFormatActions(&state, time.UnixMilli(1))
	if state.Players[0].HP != 66 || state.Players[1].HP != 18 || state.Enemies[0].HP != 162 {
		t.Fatalf("summon field damage hp: player=%v summon=%v enemy=%v", state.Players[0].HP, state.Players[1].HP, state.Enemies[0].HP)
	}
	damageRows := 0
	for _, action := range actions {
		if action["kind"] != "damage" {
			continue
		}
		damageRows++
		meta, _ := action["meta"].(map[string]any)
		if meta["sourceKind"] != "arena_format" || meta["sourceId"] != "lively_arrivals" || meta["summonedUnitId"] != "summon:1" {
			t.Fatalf("format damage metadata=%+v", meta)
		}
	}
	if damageRows != 3 {
		t.Fatalf("format damage rows=%d, want one per living unit; actions=%+v", damageRows, actions)
	}
}

func TestArenaCompactSnapshotCarriesFormatForIndependentLivePlayback(t *testing.T) {
	payload := toArenaBattleEventBatchWirePayload(42, wsbus.BattleEventBatchPayload{
		RunID: 42,
		Snapshot: map[string]any{
			"players": []map[string]any{}, "enemies": []map[string]any{},
			"arenaFormat": map[string]any{"id": "three_layer_boxes", "nameKey": "game.arena.formats.three_layer_boxes.name"},
		},
	})
	format, ok := payload.N["af"].(map[string]any)
	if !ok || format["id"] != "three_layer_boxes" {
		t.Fatalf("compact arena snapshot dropped its format: %+v", payload.N)
	}
}
