package httpapi

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func loadCurseLureTalismanSourceRow(t *testing.T, filename, tableName, id string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../packages/game-data/source/json/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(wrapper[tableName], &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if rowID, _ := row["id"].(string); rowID == id {
			return row
		}
	}
	t.Fatalf("%s.%s missing", tableName, id)
	return nil
}

func TestCurseLureTalismanAddsNormalRewardEnemyOnRefresh(t *testing.T) {
	itemRow := loadCurseLureTalismanSourceRow(t, "items.json", "items", "curse_lure_talisman")
	battleItemRow := loadCurseLureTalismanSourceRow(t, "battle_usable_items.json", "battle_usable_items", "battle_item_curse_lure_talisman")
	buffRow := loadCurseLureTalismanSourceRow(t, "buff.json", "buff", "curse_lure_enemy_refresh")
	buffEffectRow := loadCurseLureTalismanSourceRow(t, "buff_effects.json", "buff_effects", "curse_lure_enemy_refresh_on_enemy_refresh")
	slimeRow := loadCurseLureTalismanSourceRow(t, "enemies.json", "enemies", "slime_001")
	curseBatRow := loadCurseLureTalismanSourceRow(t, "enemies.json", "enemies", "curse_bat_001")
	for _, enemyRow := range []map[string]any{slimeRow, curseBatRow} {
		enemyRow["exp"] = 10
		enemyRow["drops"] = map[string]any{
			"curse_lure_test_loot": map[string]any{
				"range":   map[string]any{"min": 1, "max": 1},
				"percent": 1,
			},
		}
	}
	itemJSON, err := json.Marshal(itemRow)
	if err != nil {
		t.Fatal(err)
	}
	var itemDefinition catalog.Item
	if err := json.Unmarshal(itemJSON, &itemDefinition); err != nil {
		t.Fatal(err)
	}

	s := &Server{}
	s.deps.Config = config.Config{
		GlobalSettings:       config.DefaultGlobalSettings(),
		SharedGlobalSettings: config.DefaultSharedGlobalSettings(),
	}
	s.deps.GameData.Items = []catalog.Item{itemDefinition}
	s.deps.GameData.BattleUsableItems = []map[string]any{battleItemRow}
	s.deps.GameData.Buff = []map[string]any{buffRow}
	s.deps.GameData.BuffEffects = []map[string]any{buffEffectRow}
	s.deps.GameData.Enemies = []map[string]any{slimeRow, curseBatRow}

	item := s.battleItemByItemID("curse_lure_talisman")
	if item.ID != "battle_item_curse_lure_talisman" || item.CooldownTurns != 5 {
		t.Fatalf("curse-lure talisman linkage/cooldown mismatch: %#v", item)
	}
	if len(item.Effects) != 1 || item.Effects[0].Kind != "status" {
		t.Fatalf("curse-lure talisman must be driven by one status effect: %#v", item.Effects)
	}
	statusPayload := item.Effects[0].StatusPayload
	if len(statusPayload.StatusIDs) != 1 || statusPayload.StatusIDs[0] != "curse_lure_enemy_refresh" || statusPayload.DurationBaseTurns != 5 {
		t.Fatalf("curse-lure talisman status DSL mismatch: %#v", statusPayload)
	}

	for _, tc := range []struct {
		mapID      string
		expectedID string
	}{
		{mapID: "test_map_slime", expectedID: "slime_001"},
		{mapID: "test_map_curse_bat", expectedID: "curse_bat_001"},
	} {
		t.Run(tc.mapID, func(t *testing.T) {
			enemyCfg := s.battleEnemyCatalog()[tc.expectedID]
			baseEnemy := s.buildBattleEnemyUnit(enemyCfg, "enemy:base", false, s.battleDefaultBasicSkillID())
			battleItem := item
			battleItem.Quantity = 2
			actor := battleUnitState{
				UnitID: "player:1", Side: "player", OriginSide: "player", BaseSide: "player", PerspectiveSide: "player",
				UserID: 1, IsPlayerUnit: true, RewardEligible: true, Position: "vanguard", BasePosition: "vanguard", Alive: true,
				HP: 100, MaxHP: 100, MP: 20, MaxMP: 20,
				Stats:       map[string]float64{"hp": 100, "mp": 20, "speed": 10},
				BattleItems: []battleItemState{battleItem}, ItemRules: map[string]battleSkillRuleState{},
			}
			baselineEnemy := battleUnitState{
				UnitID: "enemy:baseline", Side: "enemy", OriginSide: "enemy", BaseSide: "enemy", PerspectiveSide: "enemy",
				Position: "vanguard", BasePosition: "vanguard", Alive: true,
				HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100, "speed": 1},
			}
			normalizeBattleUnitState(&actor)
			normalizeBattleUnitState(&baselineEnemy)
			state := &battleRuntimeState{
				Seed: 1, Wave: 1, DifficultyLevel: 2, DifficultyMult: 2,
				Players: []battleUnitState{actor},
				Enemies: []battleUnitState{baselineEnemy},
			}

			s.tryExecuteBattleTurnItem(state, time.Unix(0, 0), &state.Players[0], "player", false)

			if len(state.Enemies) != 1 {
				t.Fatalf("using the talisman must not summon immediately: enemy count = %d, want 1", len(state.Enemies))
			}
			if len(state.Players[0].Buffs) != 1 || state.Players[0].Buffs[0].BuffID != "curse_lure_enemy_refresh" || state.Players[0].Buffs[0].RemainTurns != 5 {
				t.Fatalf("curse-lure refresh status mismatch: %#v", state.Players[0].Buffs)
			}
			if state.Players[0].BattleItems[0].Quantity != 1 {
				t.Fatalf("curse-lure talisman quantity = %d, want 1", state.Players[0].BattleItems[0].Quantity)
			}
			decrementBattleItemCooldowns(&state.Players[0])
			if state.Players[0].BattleItems[0].CooldownRemain != 5 {
				t.Fatalf("curse-lure talisman cooldown remain = %d, want 5 after use turn", state.Players[0].BattleItems[0].CooldownRemain)
			}

			state.Wave = 2
			worldCfg := battleWorldConfig{
				ID:                    tc.mapID,
				EnemyIDs:              []string{tc.expectedID},
				NormalEnemyCountRange: battleCountRange{Min: 1, Max: 1},
			}
			s.spawnWaveEnemies(context.Background(), state, worldCfg, s.battleEnemyCatalog())
			if len(state.Enemies) != 2 {
				t.Fatalf("enemy refresh count = %d, want normal 1 + extra 1", len(state.Enemies))
			}
			var extraEnemy *battleUnitState
			for index := range state.Enemies {
				enemy := &state.Enemies[index]
				if enemy.EnemyID != tc.expectedID {
					t.Fatalf("enemy %s came from outside current map: %#v", enemy.EnemyID, state.Enemies)
				}
				if enemy.SummonTemplateUnitID != "" || enemy.SummonSourceUnitID != "" || enemy.SummonRemainTurns != 0 {
					t.Fatalf("refreshed enemy must be a normal enemy, not a summon: %#v", enemy)
				}
				if strings.Contains(enemy.UnitID, ":extra1:") {
					extraEnemy = enemy
				}
			}
			if extraEnemy == nil {
				t.Fatalf("extra refreshed enemy missing: %#v", state.Enemies)
			}
			if want := baseEnemy.MaxHP * 2; math.Abs(extraEnemy.MaxHP-want) > 0.0001 {
				t.Fatalf("extra enemy max hp = %v, want difficulty-scaled %v", extraEnemy.MaxHP, want)
			}

			for index := range state.Enemies {
				state.Enemies[index].Alive = false
				state.Enemies[index].HP = 0
			}
			state.StartedAt = time.Now().UnixMilli()
			settlement := s.resolveBattleWaveSettlement(*state, worldCfg, time.Now())
			reward := settlement.Rewards[1]
			expectedBattleExp := 20.0
			for _, level := range s.deps.Config.SharedGlobalSettings.CombatDifficulty.Levels {
				if level.Level == state.DifficultyLevel {
					expectedBattleExp *= 1 + level.BattleExpBonus
					break
				}
			}
			if math.Abs(reward.AttrExp["battle"]-expectedBattleExp) > 0.0001 {
				t.Fatalf("normal plus extra enemy battle exp = %v, want %v", reward.AttrExp["battle"], expectedBattleExp)
			}
			if reward.ItemRewards["curse_lure_test_loot"] != 2 {
				t.Fatalf("normal plus extra enemy drops = %#v, want 2", reward.ItemRewards)
			}

			for turn := 0; turn < 5; turn++ {
				tickBattleBuffDurations(&state.Players[0])
				decrementBattleItemCooldowns(&state.Players[0])
			}
			if len(state.Players[0].Buffs) != 0 {
				t.Fatalf("curse-lure status must expire after 5 turns: %#v", state.Players[0].Buffs)
			}
			if state.Players[0].BattleItems[0].CooldownRemain != 0 {
				t.Fatalf("curse-lure cooldown remain = %d, want 0 after 5 more turns", state.Players[0].BattleItems[0].CooldownRemain)
			}
			state.Wave = 3
			s.spawnWaveEnemies(context.Background(), state, worldCfg, s.battleEnemyCatalog())
			if len(state.Enemies) != 1 {
				t.Fatalf("enemy refresh after status expiry = %d, want normal count 1", len(state.Enemies))
			}
		})
	}
}
