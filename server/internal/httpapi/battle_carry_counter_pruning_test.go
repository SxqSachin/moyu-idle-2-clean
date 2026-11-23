package httpapi

import (
	"fmt"
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestBattleFilterCarryCounterMapsBoundsLongRoomState(t *testing.T) {
	t.Parallel()

	const retainedID = "player:1"
	battleCounts := map[string]int{battleRuntimeCounterKey(retainedID, "battle-hook"): 8}
	cooldownUntilTurns := map[string]int{battleRuntimeCounterKey(retainedID, "cooldown-hook"): 13}
	completedTurnCounts := map[string]int{retainedID: 21}
	for wave := 1; wave <= 3000; wave++ {
		enemyID := fmt.Sprintf("enemy:%d:1:slime", wave)
		battleCounts[battleRuntimeCounterKey(enemyID, "battle-hook")] = wave
		cooldownUntilTurns[battleRuntimeCounterKey(enemyID, "cooldown-hook")] = wave + 1
		completedTurnCounts[enemyID] = wave + 2
	}

	filteredBattle, filteredCooldown, filteredCompleted := battleFilterCarryCounterMaps(
		[]battleUnitState{{UnitID: retainedID}},
		battleCounts,
		cooldownUntilTurns,
		completedTurnCounts,
	)
	if len(filteredBattle) != 1 || filteredBattle[battleRuntimeCounterKey(retainedID, "battle-hook")] != 8 {
		t.Fatalf("long-room battle counters were not bounded: %#v", filteredBattle)
	}
	if len(filteredCooldown) != 1 || filteredCooldown[battleRuntimeCounterKey(retainedID, "cooldown-hook")] != 13 {
		t.Fatalf("long-room cooldown counters were not bounded: %#v", filteredCooldown)
	}
	if len(filteredCompleted) != 1 || filteredCompleted[retainedID] != 21 {
		t.Fatalf("long-room completed-turn counters were not bounded: %#v", filteredCompleted)
	}
	if len(battleCounts) != 3001 || len(cooldownUntilTurns) != 3001 || len(completedTurnCounts) != 3001 {
		t.Fatal("long-room filtering mutated source counters")
	}
}

func TestBattleCarryStatePayloadPrunesWaveLocalCounterOwners(t *testing.T) {
	t.Parallel()

	formalID := "player:1"
	persistentSummonID := "summon_persistent"
	ephemeralSummonID := "summon_ephemeral"
	expiredSummonID := "summon_expired"
	enemyID := "enemy:wave:77"
	absentFormalID := "kitty:7"
	players := []battleUnitState{
		{UnitID: formalID, Alive: true, HP: 100, MaxHP: 100},
		{
			UnitID:                     persistentSummonID,
			Alive:                      true,
			HP:                         40,
			MaxHP:                      40,
			SummonTemplateUnitID:       "spirit",
			SummonSourceUnitID:         formalID,
			SummonPersistAcrossBattles: true,
		},
		{
			UnitID:               ephemeralSummonID,
			Alive:                true,
			HP:                   30,
			MaxHP:                30,
			SummonTemplateUnitID: "echo",
			SummonSourceUnitID:   formalID,
		},
		{
			UnitID:                     expiredSummonID,
			MaxHP:                      30,
			SummonTemplateUnitID:       "guard",
			SummonSourceUnitID:         formalID,
			SummonPersistAcrossBattles: true,
		},
	}

	counterKey := func(unitID string) string {
		return battleRuntimeCounterKey(unitID, "shared:hook")
	}
	battleCounts := map[string]int{
		counterKey(formalID):           11,
		counterKey(persistentSummonID): 7,
		counterKey(ephemeralSummonID):  5,
		counterKey(expiredSummonID):    4,
		counterKey(enemyID):            3,
		counterKey(absentFormalID):     9,
	}
	cooldownUntilTurns := map[string]int{
		counterKey(formalID):           21,
		counterKey(persistentSummonID): 17,
		counterKey(ephemeralSummonID):  15,
		counterKey(expiredSummonID):    14,
		counterKey(enemyID):            13,
		counterKey(absentFormalID):     19,
	}
	completedTurnCounts := map[string]int{
		formalID:           31,
		persistentSummonID: 27,
		ephemeralSummonID:  25,
		expiredSummonID:    24,
		enemyID:            23,
		absentFormalID:     29,
	}

	carry := battleCarryStatePayload(battleRuntimeState{
		Players:                      players,
		BattleHookBattleCounts:       battleCounts,
		BattleHookCooldownUntilTurns: cooldownUntilTurns,
		CompletedTurnCounts:          completedTurnCounts,
	}, store.BattleRunStatusEnded)

	assertBattleCounterOwners(t, carry["battleHookBattleCounts"].(map[string]int), counterKey, map[string]int{
		formalID:           11,
		persistentSummonID: 7,
		absentFormalID:     9,
	})
	assertBattleCounterOwners(t, carry["battleHookCooldownUntilTurns"].(map[string]int), counterKey, map[string]int{
		formalID:           21,
		persistentSummonID: 17,
		absentFormalID:     19,
	})
	assertBattleCompletedTurns(t, carry["completedTurnCounts"].(map[string]int), map[string]int{
		formalID:           31,
		persistentSummonID: 27,
		absentFormalID:     29,
	})

	if len(battleCounts) != 6 || len(cooldownUntilTurns) != 6 || len(completedTurnCounts) != 6 {
		t.Fatal("carry filtering mutated runtime counter maps")
	}
	carriedPlayers := carry["players"].([]battleCarryUnitState)
	if len(carriedPlayers) != 2 || carriedPlayers[0].UnitID != formalID || carriedPlayers[1].UnitID != persistentSummonID {
		t.Fatalf("carry player set changed unexpectedly: %#v", carriedPlayers)
	}
}

func assertBattleCounterOwners(
	t *testing.T,
	got map[string]int,
	key func(string) string,
	want map[string]int,
) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("counter owner count=%d want=%d counters=%#v", len(got), len(want), got)
	}
	for unitID, count := range want {
		if got[key(unitID)] != count {
			t.Fatalf("counter for %s=%d want=%d counters=%#v", unitID, got[key(unitID)], count, got)
		}
	}
}

func assertBattleCompletedTurns(t *testing.T, got map[string]int, want map[string]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("completed-turn owner count=%d want=%d counters=%#v", len(got), len(want), got)
	}
	for unitID, count := range want {
		if got[unitID] != count {
			t.Fatalf("completed turns for %s=%d want=%d counters=%#v", unitID, got[unitID], count, got)
		}
	}
}
