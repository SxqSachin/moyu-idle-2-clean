package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func meteorBattlefieldTestDefinition() battleWorldBattlefieldEffect {
	return battleWorldBattlefieldEffect{
		ID: "test_starburst",
		Counter: battleWorldBattlefieldCounter{
			Source:                   "turn_consumed",
			Aggregation:              "alive_roster_cycle",
			CountSuppressedTurns:     true,
			Initial:                  8,
			ResetTo:                  8,
			CarryProgressAcrossWaves: true,
			ActorFilter: battleWorldBattlefieldActorFilter{
				Sides:              []string{"player"},
				UnitTypes:          []string{"player", "kitty"},
				IncludeSummons:     false,
				TurnKinds:          []string{"ordered"},
				IncludePreparation: false,
			},
		},
		TriggerEffects: []battleWorldBattlefieldTriggerEffect{{
			Kind:           "damage",
			TargetScope:    "all_alive_players",
			IncludeSummons: true,
			DamageType:     "magical",
			AmountByDifficulty: battleWorldBattlefieldDifficultyCurve{
				Base:         3520,
				PerLevelMult: 0.17,
			},
		}},
		PostTriggerWindows: []battleWorldBattlefieldWindow{{
			ID: "timekeeper_exposed", Duration: 2, Unit: "player_round",
		}},
		ConditionalModifiers: []battleWorldBattlefieldConditionalMod{{
			Kind: "incoming_damage_reduction", TargetScope: "special_enemies", Value: 0.75, InactiveDuringWindow: "timekeeper_exposed",
		}},
	}
}

func libraryAuditBattlefieldTestDefinition() battleWorldBattlefieldEffect {
	definition := meteorBattlefieldTestDefinition()
	definition.ID = "test_library_audit"
	definition.Counter.Initial = 1
	definition.Counter.ResetTo = 1
	definition.TriggerEffects = []battleWorldBattlefieldTriggerEffect{{
		Kind:        "weighted_random_per_target",
		TargetScope: "alive_players",
		TargetFilter: battleWorldBattlefieldTargetFilter{
			UnitTypes: []string{"player", "kitty"}, IncludeSummons: false,
		},
		TargetCount: battleWorldBattlefieldTargetCount{
			Source: "eligible_count", Divisor: 4, Rounding: "ceil", Min: 1, Max: 2,
		},
		Selection: "uniform_without_replacement",
		Choices: []battleWorldBattlefieldChoice{
			{ID: "silence", Weight: 0.4, Effect: battleWorldBattlefieldChoiceEffect{Kind: "status", StatusID: "silence", DurationTurns: 1}},
			{ID: "sealed", Weight: 0.6, Effect: battleWorldBattlefieldChoiceEffect{Kind: "adjust_skill_cooldown", Amount: 1, IncludeBasic: false}},
		},
	}}
	definition.PostTriggerWindows = nil
	definition.ConditionalModifiers = nil
	return definition
}

func TestBattlefieldWeightedTargetCountScalesAndExcludesSummons(t *testing.T) {
	definition := libraryAuditBattlefieldTestDefinition()
	players := make([]battleUnitState, 0, 9)
	for index := 0; index < 8; index++ {
		unitType := "player"
		if index%2 == 1 {
			unitType = "kitty"
		}
		unit := battlefieldTestUnit(fmt.Sprintf("unit-%d", index), unitType, 5000)
		unit.Skills = []battleSkillState{{ID: "spell", CooldownRemain: 0}, {ID: "basic_attack", CooldownRemain: 0}}
		players = append(players, unit)
	}
	summon := battlefieldTestUnit("summon:unit-0:echo", "summon", 1000)
	players = append(players, summon)
	state := battlefieldTestState(definition, 1, players...)
	state.Seed = 77
	battlefieldTestServer().battleTriggerBattlefieldEffect(&state, definition, &state.BattlefieldEffects[0], time.UnixMilli(7_000))

	if len(state.Events) != 1 || state.Events[0].Type != "battle.field.trigger" {
		t.Fatalf("audit should emit one trigger event: %#v", state.Events)
	}
	targets := battleAnyMapSlice(state.Events[0].Data["targets"])
	if len(targets) != 2 {
		t.Fatalf("eight eligible units should select two targets, got %#v", targets)
	}
	seen := map[string]bool{}
	for _, target := range targets {
		unitID := strings.TrimSpace(fmt.Sprintf("%v", target["unitId"]))
		if unitID == "summon:unit-0:echo" || seen[unitID] {
			t.Fatalf("selection must be summon-free and without replacement: %#v", targets)
		}
		seen[unitID] = true
	}
}

func TestBattlefieldWeightedTargetCountForOneThroughEightEligibleUnits(t *testing.T) {
	definition := libraryAuditBattlefieldTestDefinition()
	trigger := definition.TriggerEffects[0]
	for eligible := 1; eligible <= 8; eligible++ {
		t.Run(fmt.Sprintf("eligible-%d", eligible), func(t *testing.T) {
			players := make([]battleUnitState, 0, eligible+1)
			for index := 0; index < eligible; index++ {
				players = append(players, battlefieldTestUnit(fmt.Sprintf("unit-%d", index), "player", 5000))
			}
			players = append(players, battlefieldTestUnit("summon:unit-0:echo", "summon", 1000))
			state := battlefieldTestState(definition, 1, players...)
			state.Seed = int64(100 + eligible)
			targets := battlefieldWeightedTargets(&state, trigger)
			want := (eligible + 3) / 4
			if want > 2 {
				want = 2
			}
			if len(targets) != want {
				t.Fatalf("eligible=%d targets=%d want=%d", eligible, len(targets), want)
			}
			seen := map[string]bool{}
			for _, target := range targets {
				if target == nil || target.UnitID == "summon:unit-0:echo" || seen[target.UnitID] {
					t.Fatalf("invalid without-replacement selection: %#v", targets)
				}
				seen[target.UnitID] = true
			}
		})
	}
}

func TestBattlefieldWeightedTargetCountRoundingModes(t *testing.T) {
	definition := libraryAuditBattlefieldTestDefinition()
	trigger := definition.TriggerEffects[0]
	trigger.TargetCount.Min = 1
	trigger.TargetCount.Max = 8
	for _, testCase := range []struct {
		mode string
		want int
	}{
		{mode: "ceil", want: 2},
		{mode: "floor", want: 1},
		{mode: "round", want: 1},
	} {
		trigger.TargetCount.Rounding = testCase.mode
		if got := battlefieldWeightedTargetCount(trigger, 5); got != testCase.want {
			t.Fatalf("rounding=%s got=%d want=%d", testCase.mode, got, testCase.want)
		}
	}
}

func TestBattlefieldWeightedCooldownResultIsAuthoritativeAndExcludesBasic(t *testing.T) {
	definition := libraryAuditBattlefieldTestDefinition()
	definition.TriggerEffects[0].Choices = []battleWorldBattlefieldChoice{{
		ID: "sealed", Weight: 1, Effect: battleWorldBattlefieldChoiceEffect{Kind: "adjust_skill_cooldown", Amount: 1, IncludeBasic: false},
	}}
	player := battlefieldTestUnit("player-1", "player", 5000)
	player.Skills = []battleSkillState{{ID: "spell", CooldownRemain: 2}, {ID: "basic_attack", CooldownRemain: 0}}
	state := battlefieldTestState(definition, 1, player)
	battlefieldTestServer().battleTriggerBattlefieldEffect(&state, definition, &state.BattlefieldEffects[0], time.UnixMilli(7_500))

	if state.Players[0].Skills[0].CooldownRemain != 3 || state.Players[0].Skills[1].CooldownRemain != 0 {
		t.Fatalf("audit cooldown result mismatch: %#v", state.Players[0].Skills)
	}
	targets := battleAnyMapSlice(state.Events[0].Data["targets"])
	if len(targets) != 1 || strings.TrimSpace(fmt.Sprintf("%v", targets[0]["outcomeId"])) != "sealed" {
		t.Fatalf("authoritative target payload missing outcome: %#v", targets)
	}
	changes := battleAnyMapSlice(targets[0]["cooldownChanges"])
	if len(changes) != 1 || toInt64Loose(changes[0]["before"], -1) != 2 || toInt64Loose(changes[0]["after"], -1) != 3 {
		t.Fatalf("authoritative cooldown before/after mismatch: %#v", changes)
	}
}

func TestBattlefieldWeightedChoiceUsesConfiguredDistribution(t *testing.T) {
	definition := libraryAuditBattlefieldTestDefinition()
	state := battleRuntimeState{Seed: 12345}
	silenceCount := 0
	for index := 0; index < 1000; index++ {
		choice, ok := battlefieldWeightedChoice(&state, definition.TriggerEffects[0].Choices)
		if !ok {
			t.Fatal("weighted choice unexpectedly failed")
		}
		if choice.ID == "silence" {
			silenceCount++
		}
	}
	if silenceCount < 350 || silenceCount > 450 {
		t.Fatalf("40%% silence distribution drifted: %d/1000", silenceCount)
	}
}

func battlefieldTestUnit(id string, unitType string, hp float64) battleUnitState {
	return battleUnitState{
		UnitID: id, Side: "player", UnitType: unitType, Alive: true, HP: hp, MaxHP: hp,
		Stats: map[string]float64{"hp": hp, "magical_defence": 100}, BaseStats: map[string]float64{"hp": hp, "magical_defence": 100},
		Buffs: []battleBuffState{},
	}
}

func battlefieldTestServer() *Server {
	s := &Server{}
	s.deps.Config = config.Config{
		GlobalSettings:       config.DefaultGlobalSettings(),
		SharedGlobalSettings: config.DefaultSharedGlobalSettings(),
	}
	return s
}

func battlefieldTestState(definition battleWorldBattlefieldEffect, remaining int, players ...battleUnitState) battleRuntimeState {
	return battleRuntimeState{
		Players:         players,
		Enemies:         []battleUnitState{},
		DifficultyLevel: 1,
		BattlefieldEffects: []battlefieldEffectState{{
			ID: definition.ID, Remaining: remaining, Goal: 1, Windows: map[string]int{},
		}},
		battlefieldEffectDefinitions: []battleWorldBattlefieldEffect{definition},
		Events:                       []battleEvent{},
	}
}

func TestBattlefieldAliveRosterCycleCountsPlayerAndKittyButNotSummon(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	player := battlefieldTestUnit("player-1", "player", 7350)
	kitty := battlefieldTestUnit("kitty-1", "kitty", 4200)
	summon := battlefieldTestUnit("summon_player-1_echo", "kitty", 1000)
	state := battlefieldTestState(definition, 8, player, kitty, summon)
	s := battlefieldTestServer()
	now := time.UnixMilli(1_000)

	s.battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, now)
	if got := state.BattlefieldEffects[0]; got.Remaining != 8 || got.Progress != 1 || got.Goal != 2 {
		t.Fatalf("after player turn got %#v", got)
	}
	s.battleAdvanceBattlefieldEffects(&state, &state.Players[2], false, map[string]any{}, now)
	if got := state.BattlefieldEffects[0].Progress; got != 1 {
		t.Fatalf("summon turn must not count, progress=%d", got)
	}
	s.battleAdvanceBattlefieldEffects(&state, &state.Players[1], false, map[string]any{}, now)
	if got := state.BattlefieldEffects[0]; got.Remaining != 7 || got.Progress != 0 || got.Goal != 2 {
		t.Fatalf("after kitty completes cycle got %#v", got)
	}
}

func TestBattlefieldRosterDeathClampsProgressWithoutExtraCycle(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	player := battlefieldTestUnit("player-1", "player", 7350)
	kitty := battlefieldTestUnit("kitty-1", "kitty", 4200)
	state := battlefieldTestState(definition, 8, player, kitty)
	state.BattlefieldEffects[0].Progress = 1
	state.Players[1].Alive = false
	state.Players[1].HP = 0

	battlefieldTestServer().battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(2_000))
	if got := state.BattlefieldEffects[0]; got.Remaining != 7 || got.Progress != 0 || got.Goal != 1 {
		t.Fatalf("death clamp must settle exactly one cycle, got %#v", got)
	}
}

func TestBattlefieldStarburstSettlesBeforeWindowAndUsesDifficultyCurve(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	player := battlefieldTestUnit("player-1", "player", 7350)
	state := battlefieldTestState(definition, 1, player)
	state.DifficultyLevel = 10
	battlefieldTestServer().battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(3_000))

	got := state.BattlefieldEffects[0]
	if got.Remaining != 8 || got.Windows["timekeeper_exposed"] != 2 {
		t.Fatalf("trigger must reset counter and open window, got %#v", got)
	}
	if diff := state.Players[0].HP - 2897.2; diff < -0.001 || diff > 0.001 {
		t.Fatalf("difficulty 10 with 100 magic defense should leave 2897.2 hp, got %.4f events=%#v", state.Players[0].HP, state.Events)
	}
	if len(state.Events) != 3 || state.Events[0].Type != "battle.field.tick" || state.Events[1].Type != "battle.field.trigger" || state.Events[2].Type != "battle.field.window" {
		t.Fatalf("unexpected field event order: %#v", state.Events)
	}
	trigger := state.Events[1].Data
	if gotRaw := toFloat64Loose(trigger["rawDamage"], 0); gotRaw != 8905.6 {
		t.Fatalf("raw difficulty damage=%v", gotRaw)
	}
}

func TestBattlefieldStarburstDifficultyOneUsesDefenseAndResistance(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	player := battlefieldTestUnit("player-1", "player", 7350)
	player.Stats["magical_damage_resist"] = 0.1
	player.BaseStats["magical_damage_resist"] = 0.1
	state := battlefieldTestState(definition, 1, player)
	battlefieldTestServer().battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(3_500))

	if diff := state.Players[0].HP - 5766; diff < -0.001 || diff > 0.001 {
		t.Fatalf("difficulty 1 should deal 1584 after defense and resistance, hp=%.4f", state.Players[0].HP)
	}
	trigger := state.Events[1].Data
	if gotRaw := toFloat64Loose(trigger["rawDamage"], 0); gotRaw != 3520 {
		t.Fatalf("raw difficulty damage=%v, want 3520", gotRaw)
	}
	targets := battleAnyMapSlice(trigger["targets"])
	if len(targets) != 1 || toFloat64Loose(targets[0]["damage"], 0) != 1584 {
		t.Fatalf("authoritative target damage mismatch: %#v", targets)
	}
}

func TestBattlefieldCounterPreparationAndSuppressionRules(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	state := battlefieldTestState(definition, 8, battlefieldTestUnit("player-1", "player", 7350))
	s := battlefieldTestServer()
	suppressed := map[string]any{"resultMeta": map[string]any{"reason": battleEffectTurnActionSuppressed}}

	s.battleAdvanceBattlefieldEffects(&state, &state.Players[0], true, suppressed, time.UnixMilli(3_600))
	if got := state.BattlefieldEffects[0]; got.Remaining != 8 || got.Progress != 0 {
		t.Fatalf("preparation turn must not count: %#v", got)
	}
	s.battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, suppressed, time.UnixMilli(3_700))
	if got := state.BattlefieldEffects[0]; got.Remaining != 7 || got.Progress != 0 {
		t.Fatalf("suppressed ordered turn should count when configured: %#v", got)
	}

	definition.Counter.CountSuppressedTurns = false
	state = battlefieldTestState(definition, 8, battlefieldTestUnit("player-1", "player", 7350))
	s.battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, suppressed, time.UnixMilli(3_800))
	if got := state.BattlefieldEffects[0]; got.Remaining != 8 || got.Progress != 0 {
		t.Fatalf("suppressed turn must be filtered when configured: %#v", got)
	}
}

func TestBattlefieldTriggerIncludesPlayerSummons(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	player := battlefieldTestUnit("player-1", "player", 7350)
	summon := battlefieldTestUnit("summon:player-1:echo", "summon", 3000)
	state := battlefieldTestState(definition, 1, player, summon)
	battlefieldTestServer().battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(3_900))

	trigger := state.Events[1].Data
	targets := battleAnyMapSlice(trigger["targets"])
	if len(targets) != 2 {
		t.Fatalf("starburst must include player summons, targets=%#v", targets)
	}
	if state.Players[1].HP != 1240 {
		t.Fatalf("summon should receive magical starburst damage, hp=%.4f", state.Players[1].HP)
	}
}

func TestBattlefieldExposureDisablesSpecialEnemyReductionForTwoPlayerRounds(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	state := battlefieldTestState(definition, 8, battlefieldTestUnit("player-1", "player", 7350))
	state.battlefieldSpecialEnemyIDs = map[string]struct{}{"boss-1": {}}
	boss := battleUnitState{UnitID: "enemy-1", EnemyID: "boss-1", Side: "enemy", Alive: true, HP: 10000}
	s := battlefieldTestServer()

	if got := s.battleApplyBattlefieldIncomingDamageModifiers(&state, &boss, 1000); got != 250 {
		t.Fatalf("armored boss damage=%v", got)
	}
	state.BattlefieldEffects[0].Windows["timekeeper_exposed"] = 2
	if got := s.battleApplyBattlefieldIncomingDamageModifiers(&state, &boss, 1000); got != 1000 {
		t.Fatalf("exposed boss damage=%v", got)
	}
	s.battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(4_000))
	if got := state.BattlefieldEffects[0].Windows["timekeeper_exposed"]; got != 1 {
		t.Fatalf("window after one player round=%d", got)
	}
	s.battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(5_000))
	if got := state.BattlefieldEffects[0].Windows["timekeeper_exposed"]; got != 0 {
		t.Fatalf("window must close after two complete player rounds, got %d", got)
	}
	if got := s.battleApplyBattlefieldIncomingDamageModifiers(&state, &boss, 1000); got != 250 {
		t.Fatalf("boss reduction must restore, damage=%v", got)
	}
}

func TestBattlefieldPerEventAndCompactSnapshotCompatibility(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	definition.Counter.Aggregation = "per_event"
	state := battlefieldTestState(definition, 8, battlefieldTestUnit("player-1", "player", 7350))
	battlefieldTestServer().battleAdvanceBattlefieldEffects(&state, &state.Players[0], false, map[string]any{}, time.UnixMilli(6_000))
	if got := state.BattlefieldEffects[0].Remaining; got != 7 {
		t.Fatalf("per_event remaining=%d", got)
	}
	compact := compactBattleSnapshot(map[string]any{
		"wave": 1, "players": []map[string]any{}, "enemies": []map[string]any{},
		"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
	})
	rows := battleAnyMapSlice(compact["bf"])
	if len(rows) != 1 || rows[0]["i"] != definition.ID || toInt64Loose(rows[0]["r"], 0) != 7 {
		t.Fatalf("compact bf mismatch: %#v", compact["bf"])
	}
	legacy := compactBattleSnapshot(map[string]any{"wave": 1, "players": []map[string]any{}, "enemies": []map[string]any{}})
	if _, exists := legacy["bf"]; exists {
		t.Fatalf("legacy snapshot must omit bf: %#v", legacy)
	}
}

func TestBattlefieldCarryStateClearsOnFailure(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	state := battlefieldTestState(definition, 6, battlefieldTestUnit("player-1", "player", 7350))
	state.BattlefieldEffects[0].Progress = 1
	state.BattlefieldEffects[0].Windows["timekeeper_exposed"] = 2
	ended := battleCarryStatePayload(state, "ended")
	if got := battleAnyMapSlice(ended["battlefieldEffects"]); len(got) != 1 {
		t.Fatalf("ended carry missing battlefield state: %#v", ended)
	}
	failed := battleCarryStatePayload(state, "failed")
	if _, exists := failed["battlefieldEffects"]; exists {
		t.Fatalf("failed carry must clear battlefield state: %#v", failed)
	}
	carriedEffectsRaw, err := json.Marshal(ended["battlefieldEffects"])
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeBattlefieldEffectStatesRaw(carriedEffectsRaw)
	if len(decoded) != 1 || decoded[0].Remaining != 6 || decoded[0].Progress != 1 || decoded[0].Windows["timekeeper_exposed"] != 2 {
		t.Fatalf("carry decode lost half-cycle or window: %#v", decoded)
	}
}

func TestBattlefieldCompileFallbackPreservesPreviousWaveCarry(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	players := []battleUnitState{
		battlefieldTestUnit("player-1", "player", 7350),
		battlefieldTestUnit("player-2", "player", 7350),
	}
	loads := 0
	got := buildBattleMissingInitialBattlefieldEffects(
		battleWorldConfig{BattlefieldEffects: []battleWorldBattlefieldEffect{definition}},
		players,
		2,
		func(expectedWave int) (battlePreviousWaveCarryState, error) {
			loads++
			if expectedWave != 1 {
				t.Fatalf("expected previous wave=%d, want 1", expectedWave)
			}
			return battlePreviousWaveCarryState{BattlefieldEffects: []battlefieldEffectState{{
				ID: definition.ID, Remaining: 6, Progress: 1, Windows: map[string]int{"timekeeper_exposed": 2},
			}}}, nil
		},
	)
	if loads != 1 {
		t.Fatalf("previous carry loads=%d, want 1", loads)
	}
	if len(got) != 1 || got[0].Remaining != 6 || got[0].Progress != 1 || got[0].Windows["timekeeper_exposed"] != 2 {
		t.Fatalf("compile fallback lost previous battlefield carry: %#v", got)
	}

	loads = 0
	fresh := buildBattleMissingInitialBattlefieldEffects(
		battleWorldConfig{BattlefieldEffects: []battleWorldBattlefieldEffect{definition}},
		players,
		1,
		func(int) (battlePreviousWaveCarryState, error) {
			loads++
			return battlePreviousWaveCarryState{}, nil
		},
	)
	if loads != 0 || len(fresh) != 1 || fresh[0].Remaining != definition.Counter.Initial {
		t.Fatalf("first wave fallback must not load carry: loads=%d state=%#v", loads, fresh)
	}

	reset := buildBattleMissingInitialBattlefieldEffects(
		battleWorldConfig{BattlefieldEffects: []battleWorldBattlefieldEffect{definition}},
		players,
		2,
		func(int) (battlePreviousWaveCarryState, error) {
			return battlePreviousWaveCarryState{}, fmt.Errorf("carry unavailable")
		},
	)
	if len(reset) != 1 || reset[0].Remaining != definition.Counter.Initial || reset[0].Progress != 0 || len(reset[0].Windows) != 0 {
		t.Fatalf("failed optional carry load must retain legacy fresh-state fallback: %#v", reset)
	}
}

func TestBattlefieldStarburstCanFailAfterLastEnemyDies(t *testing.T) {
	definition := meteorBattlefieldTestDefinition()
	s := battlefieldTestServer()
	state := battlefieldTestState(definition, 1, battlefieldTestUnit("player:1", "player", 1000))
	state.Seed = 1
	state.Wave = 1
	state.Order = []string{"player:1", "enemy:1"}
	state.Players[0].UserID = 1
	state.Players[0].IsPlayerUnit = true
	basicAttack := mirrorCastBasicSkill()
	state.Players[0].Skills = []battleSkillState{basicAttack}
	state.Players[0].SkillIDs = []string{"basic_attack"}
	state.Players[0].SkillSlots = []string{"basic_attack"}
	state.Players[0].BasicAttack = &battleBasicAttackProfile{Effects: basicAttack.Effects}
	state.Players[0].Stats["physics_damage"] = 100
	state.Players[0].BaseStats["physics_damage"] = 100
	state.Enemies = []battleUnitState{{
		UnitID: "enemy:1", EnemyID: "last-enemy", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
		UnitType: "enemy", Position: "vanguard", BasePosition: "vanguard", Alive: true,
		HP: 1, MaxHP: 1, Stats: map[string]float64{"hp": 1}, BaseStats: map[string]float64{"hp": 1},
	}}

	status, _, err := s.processBattleTurn(context.Background(), &state, battleWorldConfig{}, nil, time.UnixMilli(4_100), map[int64]store.BattleWaveReward{})
	if err != nil || status != "" {
		t.Fatalf("resolving action should remain in-turn, status=%q err=%v", status, err)
	}
	if state.Enemies[0].Alive || state.Players[0].Alive {
		t.Fatalf("last enemy and player should both be dead: player=%#v enemy=%#v", state.Players[0], state.Enemies[0])
	}
	for _, event := range state.Events {
		if event.Type == "battle.wave.win" {
			t.Fatalf("wave must not be won before starburst failure: %#v", state.Events)
		}
	}
	status, _, err = s.processBattleTurn(context.Background(), &state, battleWorldConfig{}, nil, time.UnixMilli(4_200), map[int64]store.BattleWaveReward{})
	if err != nil || status != store.BattleRunStatusFailed {
		t.Fatalf("next settlement must fail the run, status=%q err=%v", status, err)
	}
	if got := state.Events[len(state.Events)-1].Type; got != "battle.failed" {
		t.Fatalf("final event=%s, want battle.failed", got)
	}
}
