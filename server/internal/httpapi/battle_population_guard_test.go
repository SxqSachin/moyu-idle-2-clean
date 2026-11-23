package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func populationGuardSummon(unitID string, order int64) battleUnitState {
	return battleUnitState{
		UnitID:                     unitID,
		Side:                       "player",
		Alive:                      true,
		HP:                         10,
		MaxHP:                      10,
		SummonTemplateUnitID:       battlePlayerCloneTemplateID,
		SummonSourceUnitID:         "player:1",
		SummonMaxCountPerUnit:      3,
		SummonPersistAcrossBattles: true,
		SummonOnCapOverflow:        battleSummonOnCapOverflowReplace,
		SummonOrder:                order,
		Stats:                      map[string]float64{"hp": 10},
		SummonSourceEnhanceLevel:   0,
		SummonRemainTurns:          3,
		SummonOnCasterDeath:        battleSummonOnCasterDeathKeep,
		SummonOnDispel:             battleSummonOnDispelDespawn,
		SummonOnDurationEnd:        battleSummonOnDurationEndDespawn,
	}
}

func TestBattlePrepareSummonCapacityRepairsExistingOverflow(t *testing.T) {
	state := &battleRuntimeState{Players: []battleUnitState{{UnitID: "player:1", Alive: true, HP: 100, MaxHP: 100}}}
	for order := int64(1); order <= 5; order++ {
		state.Players = append(state.Players, populationGuardSummon(fmt.Sprintf("summon_old_shadow_%d", order), order))
	}
	actions, ok := battlePrepareSummonCapacity(
		state,
		battlePlayerCloneTemplateID,
		"player:1",
		3,
		battleSummonOnCapOverflowReplace,
	)
	if !ok || len(actions) != 3 {
		t.Fatalf("overflow repair result ok=%v actions=%#v", ok, actions)
	}
	if got := battleCountSummonsByTemplateAndSource(state, battlePlayerCloneTemplateID, "player:1"); got != 2 {
		t.Fatalf("alive summons after making one slot=%d, want 2", got)
	}
	for _, expiredID := range []string{"summon_old_shadow_1", "summon_old_shadow_2", "summon_old_shadow_3"} {
		unit := battleUnitByID(state, expiredID)
		if unit == nil || unit.Alive || unit.HP != 0 {
			t.Fatalf("oldest overflow summon %s was not expired: %#v", expiredID, unit)
		}
	}
	state.Players = append(state.Players, populationGuardSummon("summon_old_shadow_6", 6))
	if got := battleCountSummonsByTemplateAndSource(state, battlePlayerCloneTemplateID, "player:1"); got != 3 {
		t.Fatalf("new summon exceeded repaired cap: %d", got)
	}
}

func TestBattlePrepareSummonCapacityHonorsFailNewSummon(t *testing.T) {
	state := &battleRuntimeState{Players: []battleUnitState{
		{UnitID: "player:1", Alive: true, HP: 100, MaxHP: 100},
		populationGuardSummon("summon_old_shadow_1", 1),
	}}
	actions, ok := battlePrepareSummonCapacity(
		state,
		battlePlayerCloneTemplateID,
		"player:1",
		1,
		battleSummonOnCapOverflowFail,
	)
	if ok || len(actions) != 0 || !state.Players[1].Alive {
		t.Fatalf("fail-new-summon policy mutated existing summon: ok=%v actions=%#v unit=%#v", ok, actions, state.Players[1])
	}
}

func TestAuthoredOldShadowPendantCannotAccumulatePastThree(t *testing.T) {
	data, err := catalog.Load("../../../packages/game-data/source/json/equipment_templates.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.deps.GameData = data
	hooks := s.resolveBattleEquipmentHooks([]store.EquippedItem{{
		TemplateItemID: "old_shadow_pendant",
		EquipType:      "battle",
		EquippedSlot:   "accessory",
	}})
	if len(hooks) != 1 {
		t.Fatalf("old shadow pendant hook count=%d, want 1", len(hooks))
	}
	state := &battleRuntimeState{
		Seed: 17,
		Players: []battleUnitState{{
			UnitID: "player:1", IsPlayerUnit: true, UnitType: "player", Side: "player", Position: "midguard",
			Alive: true, HP: 100, MP: 20, MaxHP: 100, MaxMP: 20,
			Stats:          map[string]float64{"hp": 100, "mp": 20, "speed": 10, "perception": 30},
			EquipmentHooks: hooks,
		}},
	}
	ensureSliceSpareCap(&state.Players, 16)
	replacementSeen := false
	for _, wave := range []int{3, 6, 9, 12, 15} {
		state.Wave = wave
		actions := s.battleTriggerWaveStartEquipmentHooks(state, time.UnixMilli(int64(wave)))
		for _, action := range actions {
			if action["kind"] == "summon_expire" && action["reason"] == "summon_cap_replaced" {
				replacementSeen = true
			}
		}
		if alive := battleCountSummonsByTemplateAndSource(state, battlePlayerCloneTemplateID, "player:1"); alive > 3 {
			t.Fatalf("old shadows exceeded authored cap at wave %d: %d", wave, alive)
		}
	}
	if alive := battleCountSummonsByTemplateAndSource(state, battlePlayerCloneTemplateID, "player:1"); alive != 3 {
		t.Fatalf("old shadow count=%d, want capped 3", alive)
	}
	if !replacementSeen {
		t.Fatal("old shadow overflow never emitted a replacement action")
	}
}

func TestSpawnWaveEnemiesClampsScalingToWorldTeamSize(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{Seed: 29, Wave: 1}
	for index := 1; index <= 12; index++ {
		state.Players = append(state.Players, battleUnitState{UnitID: fmt.Sprintf("player:%d", index), Side: "player", Alive: true})
	}
	for index := 1; index <= 8; index++ {
		state.Players = append(state.Players, populationGuardSummon(fmt.Sprintf("summon_%d", index), int64(index)))
	}
	worldCfg := battleWorldConfig{
		EnemyIDs:              []string{"enemy_pool"},
		MaxTeamSize:           4,
		NormalEnemyCountRange: battleCountRange{Min: 1, Max: 1},
		ExtraEnemyPerPlayer:   battleCountRange{Min: 2, Max: 2},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"enemy_pool": {ID: "enemy_pool", Position: "vanguard", Stats: map[string]float64{"hp": 10, "mp": 0}},
	}
	s.spawnWaveEnemies(context.Background(), state, worldCfg, enemyCatalog)
	if len(state.Enemies) != 7 {
		t.Fatalf("enemy scaling escaped max team size: got %d, want 1 + (4-1)*2", len(state.Enemies))
	}
}

func TestSummonsCannotDuplicateEnemyRefreshBuffsOrTriggerItems(t *testing.T) {
	s := &Server{}
	s.deps.GameData.BuffEffects = []map[string]any{{
		"id":      "summon_refresh_guard",
		"buffId":  "refresh_lure",
		"trigger": "on_enemy_refresh",
		"effects": []map[string]any{{
			"kind":      "extra_enemy_refresh",
			"enemyPool": "current_world",
			"count":     1,
		}},
	}}
	formal := battleUnitState{
		UnitID: "player:1", Alive: true, HP: 10, MaxHP: 10,
		Buffs: []battleBuffState{{BuffID: "refresh_lure", RemainTurns: 3}},
	}
	if got := s.battleExtraEnemyRefreshCount(&battleRuntimeState{Wave: 2, Players: []battleUnitState{formal}}); got != 1 {
		t.Fatalf("test setup did not resolve formal refresh buff: got %d", got)
	}
	summon := populationGuardSummon("summon_refresh_copy", 1)
	summon.Buffs = []battleBuffState{{BuffID: "refresh_lure", RemainTurns: 3}}
	summon.BattleItems = []battleItemState{{
		ItemID: "summon_trigger_copy", Quantity: 1, IsTrigger: true,
		DungeonIDs: []string{"amusement_park_001"}, SpawnEnemyIDs: []string{"enemy_trigger"},
		MinWave: 1, PerWaveChance: 1,
	}}
	state := &battleRuntimeState{Seed: 1, Wave: 2, Players: []battleUnitState{summon}}
	if got := s.battleExtraEnemyRefreshCount(state); got != 0 {
		t.Fatalf("summoned copy duplicated an enemy refresh buff: got %d", got)
	}
	if spawns := s.resolveTriggerItemSpawns(state, "amusement_park_001"); len(spawns) != 0 {
		t.Fatalf("summoned copy duplicated a trigger item: %#v", spawns)
	}
	if state.Players[0].BattleItems[0].Quantity != 1 {
		t.Fatalf("ignored summon trigger item was consumed: %#v", state.Players[0].BattleItems[0])
	}
}
