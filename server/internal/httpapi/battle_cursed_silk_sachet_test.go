package httpapi

import (
	"context"
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func cursedSilkSachetBossRefreshHook(enhanceLevel int) battleEquipmentHookRule {
	return battleEquipmentHookRule{
		ID:                 cursedSilkSachetItemID + "_extra_boss_refresh",
		SourceKind:         "equipment",
		SourceItemID:       cursedSilkSachetItemID,
		SourceEnhanceLevel: enhanceLevel,
		Trigger:            "on_enemy_refresh",
		Effects: []map[string]any{{
			"kind":      "extra_enemy_refresh",
			"enemyPool": "current_wave_boss",
			"countFormula": map[string]any{
				"baseAdd": 1,
				"terms": []any{map[string]any{
					"attrKey": "equipment_enhance_level", "coefficient": 0.05, "perSkillLevel": 0,
				}},
			},
			"maxCount":       3,
			"partyStackMode": "highest",
			"settlementMode": "normal",
		}},
	}
}

func TestCursedSilkSachetReappliesCurseAtEveryWaveStart(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Buff = []map[string]any{{
		"id": cursedSilkSachetItemID + "_curse", "durationTurns": -1, "permanent": true, "dispellable": true,
	}}
	s.deps.GameData.BuffEffects = []map[string]any{
		{
			"id": cursedSilkSachetItemID + "_curse_while_active", "buffId": cursedSilkSachetItemID + "_curse",
			"trigger": "while_active", "effects": []map[string]any{{"kind": "add_stat_percent", "attr": "speed", "value": -0.2}},
		},
		{
			"id": cursedSilkSachetItemID + "_curse_before_take_damage", "buffId": cursedSilkSachetItemID + "_curse",
			"trigger": "on_before_take_damage", "effects": []map[string]any{{"kind": "set_damage_multiplier", "value": 1.2}},
		},
	}
	state := &battleRuntimeState{
		Wave: 1,
		Players: []battleUnitState{{
			UnitID: "player:1", Side: "player", Alive: true, HP: 1000, MaxHP: 1000,
			BaseStats: map[string]float64{"hp": 1000, "speed": 100}, Stats: map[string]float64{"hp": 1000, "speed": 100},
			EquipmentHooks: []battleEquipmentHookRule{{
				ID: cursedSilkSachetItemID + "_apply_curse", SourceKind: "equipment", SourceItemID: cursedSilkSachetItemID,
				Trigger: "on_wave_start",
				Effects: []map[string]any{{"kind": "add_buff", "buffId": cursedSilkSachetItemID + "_curse", "targetMode": "self"}},
			}},
		}},
	}

	if actions := s.battleTriggerBattleStartEquipmentHooks(state, time.UnixMilli(1)); len(actions) != 0 {
		t.Fatalf("wave curse must not use the one-time battle-start trigger: %#v", actions)
	}
	s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(2))
	unit := &state.Players[0]
	if !battleHasBuff(unit, cursedSilkSachetItemID+"_curse") {
		t.Fatalf("expected cursed sachet status on wave 1, buffs=%#v", unit.Buffs)
	}
	if unit.Stats["speed"] != 80 {
		t.Fatalf("speed=%v, want 80 after a 20%% reduction", unit.Stats["speed"])
	}
	damage, _ := s.battleApplyBeforeTakeDamageBuffHooks(state, time.UnixMilli(3), nil, unit, 100, "physical")
	if damage != 120 {
		t.Fatalf("incoming damage=%v, want 120", damage)
	}
	if !s.battleBuffDispellableByID(cursedSilkSachetItemID + "_curse") {
		t.Fatal("sachet curse should be dispellable within a wave")
	}

	unit.Buffs = nil
	s.battleRecomputeUnitStats(unit)
	state.Wave = 2
	s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(4))
	if !battleHasBuff(unit, cursedSilkSachetItemID+"_curse") || unit.Stats["speed"] != 80 {
		t.Fatalf("wave 2 should reapply a dispelled curse, unit=%#v", unit)
	}
}

func TestEquipmentExtraBossRefreshCountUsesStrongestEnhancementPartyWide(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		level int
		want  int
	}{{0, 1}, {19, 1}, {20, 2}, {39, 2}, {40, 3}, {100, 3}} {
		state := &battleRuntimeState{Players: []battleUnitState{{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(tc.level)}}}}
		if got := s.battleEquipmentExtraBossRefreshCount(state); got != tc.want {
			t.Fatalf("enhance +%d extra boss count=%d, want %d", tc.level, got, tc.want)
		}
	}

	state := &battleRuntimeState{Players: []battleUnitState{
		{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(0)}},
		{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(40)}},
	}}
	if got := s.battleEquipmentExtraBossRefreshCount(state); got != 3 {
		t.Fatalf("party should use the strongest sachet instead of summing them, got %d", got)
	}

	summon := battleUnitState{Alive: true, HP: 1, SummonTemplateUnitID: "player_echo", SummonSourceUnitID: "player:1", EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(40)}}
	if got := s.battleEquipmentExtraBossRefreshCount(&battleRuntimeState{Players: []battleUnitState{summon}}); got != 0 {
		t.Fatalf("summoned copies must not duplicate equipment refresh effects, got %d", got)
	}
	dead := battleUnitState{EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(40)}}
	if got := s.battleEquipmentExtraBossRefreshCount(&battleRuntimeState{Players: []battleUnitState{dead}}); got != 0 {
		t.Fatalf("defeated wearers must not create enemies for later waves, got %d", got)
	}

	unsafeHook := cursedSilkSachetBossRefreshHook(0)
	unsafeHook.Effects[0]["countFormula"] = map[string]any{"baseAdd": 999, "terms": []any{}}
	unsafeHook.Effects[0]["maxCount"] = 999
	if got := s.battleEquipmentExtraBossRefreshCount(&battleRuntimeState{Players: []battleUnitState{{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{unsafeHook}}}}); got != 8 {
		t.Fatalf("runtime safety cap should limit malformed data to 8 extra enemies, got %d", got)
	}
}

func TestCursedSilkSachetAuthoredHookResolvesIntoEnemyRefreshRuntime(t *testing.T) {
	data, err := catalog.Load("../../../packages/game-data/source/json/equipment_templates.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.deps.GameData = data
	hooks := s.resolveBattleEquipmentHooks([]store.EquippedItem{{
		InstanceID: 77, TemplateItemID: cursedSilkSachetItemID, EquipType: "battle", EnhanceLevel: 20,
	}})
	triggers := map[string]bool{}
	for _, hook := range hooks {
		triggers[hook.Trigger] = true
	}
	if len(hooks) != 2 || !triggers["onenemyrefresh"] || !triggers["onwavestart"] {
		t.Fatalf("authored sachet hooks did not reach the battle snapshot: %#v", hooks)
	}
	state := &battleRuntimeState{Players: []battleUnitState{{Alive: true, HP: 1, EquipmentHooks: hooks}}}
	if got := s.battleEquipmentExtraBossRefreshCount(state); got != 2 {
		t.Fatalf("authored +20 sachet extra boss count=%d, want 2", got)
	}
}

func TestAppendEquipmentExtraBossRefreshesCreatesDirectScaledEnemies(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed:    17,
		Wave:    10,
		Players: []battleUnitState{{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(40)}}},
		Enemies: []battleUnitState{{
			UnitID: "enemy:10:s1:boss_a", EnemyID: "boss_a", EnemyType: "boss",
			BaseStats: map[string]float64{"hp": 320, "speed": 12}, Stats: map[string]float64{"hp": 480, "speed": 18},
			HP: 480, MaxHP: 480, MP: 25, MaxMP: 25, Alive: true, StatMode: battleStatModeProjected,
			Position: "vanguard", BasePosition: "vanguard",
		}},
	}
	catalog := map[string]battleEnemyConfig{
		"boss_a": {ID: "boss_a", Type: "boss", Position: "vanguard", Stats: map[string]float64{"hp": 100, "mp": 25}},
	}

	s.appendBattleEquipmentExtraBossRefreshes(state, catalog, "")
	if len(state.Enemies) != 4 {
		t.Fatalf("expected authored boss plus three +40 extras, got %d enemies", len(state.Enemies))
	}
	seen := map[string]bool{}
	for index := 1; index < len(state.Enemies); index++ {
		extra := state.Enemies[index]
		if seen[extra.UnitID] || extra.UnitID == state.Enemies[0].UnitID {
			t.Fatalf("extra bosses need unique ids: %#v", state.Enemies)
		}
		seen[extra.UnitID] = true
		if extra.EnemyID != "boss_a" || extra.EnemyType != "boss" || extra.MaxHP != 480 || extra.HP != 480 || extra.Stats["speed"] != 18 {
			t.Fatalf("extra boss should preserve the spawned boss scaling: %#v", extra)
		}
		if battleUnitIsSummon(&extra) || extra.SummonTemplateUnitID != "" || extra.SummonSourceUnitID != "" {
			t.Fatalf("extra boss must be a direct enemy, never a summon: %#v", extra)
		}
	}

	s.appendBattleEquipmentExtraBossRefreshes(state, catalog, "")
	if len(state.Enemies) != 4 {
		t.Fatalf("re-entering the spawn phase must be idempotent, got %d enemies", len(state.Enemies))
	}
}

func TestAppendEquipmentExtraBossRefreshesExcludesTriggerItemBosses(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 3, Wave: 4,
		Players: []battleUnitState{{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(40)}}},
		Enemies: []battleUnitState{{UnitID: "enemy:4:t1:boss_a", EnemyID: "boss_a", EnemyType: "boss"}},
	}
	s.appendBattleEquipmentExtraBossRefreshes(state, map[string]battleEnemyConfig{"boss_a": {ID: "boss_a", Type: "boss"}}, "")
	if len(state.Enemies) != 1 {
		t.Fatalf("trigger-item boss must never become the authored source for extra bosses: %#v", state.Enemies)
	}
}

func TestSpawnWaveEnemiesCursedSilkSachetAddsScaledBossesToCustomLineup(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 23, Wave: 5, DifficultyMult: 2,
		Players: []battleUnitState{{Alive: true, HP: 1, EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(20)}}},
	}
	worldCfg := battleWorldConfig{CustomLineupConf: []battleCustomLineupConfig{{
		ID: "boss_lineup", Percent: 1, Lineup: []string{"boss_a"},
	}}}
	catalog := map[string]battleEnemyConfig{
		"boss_a": {ID: "boss_a", Type: "boss", Position: "vanguard", Stats: map[string]float64{"hp": 100, "speed": 10}},
	}

	s.spawnWaveEnemies(context.Background(), state, worldCfg, catalog)
	if state.CurrentWaveCustomLineupID != "boss_lineup" || len(state.Enemies) != 3 {
		t.Fatalf("expected authored boss plus two +20 extras, lineup=%q enemies=%#v", state.CurrentWaveCustomLineupID, state.Enemies)
	}
	for _, enemy := range state.Enemies {
		if enemy.MaxHP != 100 || enemy.Stats["speed"] != 10 {
			t.Fatalf("extra boss should copy the authored custom-lineup stats: %#v", enemy)
		}
	}
}

func TestCursedSilkSachetExtraBossesParticipateInNormalSettlement(t *testing.T) {
	s := &Server{}
	s.deps.Config = config.Config{
		GlobalSettings:       config.DefaultGlobalSettings(),
		SharedGlobalSettings: config.DefaultSharedGlobalSettings(),
	}
	s.deps.GameData.Enemies = []map[string]any{{
		"id": "boss_a", "type": "boss", "position": "vanguard", "exp": 10,
		"stats": map[string]any{"hp": 100, "speed": 10},
		"drops": map[string]any{"sachet_test_loot": map[string]any{
			"range": map[string]any{"min": 1, "max": 1}, "percent": 1,
		}},
	}}
	state := &battleRuntimeState{
		Seed: 5, Wave: 1, StartedAt: time.Now().UnixMilli(), DifficultyMult: 1,
		Players: []battleUnitState{{
			UnitID: "player:1", UserID: 1, IsPlayerUnit: true, RewardEligible: true, Alive: true,
			HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100, "battle_drop_rate_bonus": 0},
			EquipmentHooks: []battleEquipmentHookRule{cursedSilkSachetBossRefreshHook(40)},
		}},
	}
	worldCfg := battleWorldConfig{EnemyIDs: []string{"boss_a"}, NormalEnemyCountRange: battleCountRange{Min: 1, Max: 1}}
	s.spawnWaveEnemies(context.Background(), state, worldCfg, s.battleEnemyCatalog())
	if len(state.Enemies) != 4 {
		t.Fatalf("expected one authored and three extra bosses, got %#v", state.Enemies)
	}
	for index := range state.Enemies {
		state.Enemies[index].Alive = false
		state.Enemies[index].HP = 0
	}

	settlement := s.resolveBattleWaveSettlement(*state, worldCfg, time.Now())
	reward := settlement.Rewards[1]
	if math.Abs(reward.AttrExp["battle"]-40) > 0.0001 {
		t.Fatalf("all four direct bosses should grant battle EXP, got %v", reward.AttrExp["battle"])
	}
	if reward.ItemRewards["sachet_test_loot"] != 4 {
		t.Fatalf("all four direct bosses should roll normal drops, got %#v", reward.ItemRewards)
	}
}
