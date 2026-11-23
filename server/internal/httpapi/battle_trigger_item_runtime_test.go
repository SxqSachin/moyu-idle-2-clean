package httpapi

import "testing"

func TestPartyTriggerGroupSpawnsAndConsumesOnlyStableWinner(t *testing.T) {
	s := &Server{}
	players := make([]battleUnitState, 4)
	for index, userID := range []int64{40, 10, 30, 20} {
		players[index] = battleUnitState{
			UserID: userID,
			BattleItems: []battleItemState{{
				ItemID: "haunted_after_hours_ticket", Quantity: 1, IsTrigger: true,
				DungeonIDs: []string{"amusement_park_001"}, SpawnEnemyIDs: []string{"midnight_ticket_collector_001"},
				MinWave: 17, IntervalWaves: 8, PerWaveChance: 1, PartyTriggerGroup: "amusement_after_hours",
			}},
		}
	}
	state := &battleRuntimeState{Seed: 123, Wave: 17, Players: players}
	spawns := s.resolveTriggerItemSpawns(state, "amusement_park_001")
	if len(spawns) != 1 || spawns[0] != "midnight_ticket_collector_001" {
		t.Fatalf("expected exactly one ticket collector, got %#v", spawns)
	}
	for index := range state.Players {
		item := state.Players[index].BattleItems[0]
		wantQuantity := int64(1)
		if state.Players[index].UserID == 10 {
			wantQuantity = 0
		}
		if item.Quantity != wantQuantity {
			t.Fatalf("user %d quantity=%d, want %d", state.Players[index].UserID, item.Quantity, wantQuantity)
		}
		if item.LastSpawnWave != 17 {
			t.Fatalf("group cooldown was not shared with user %d: %#v", state.Players[index].UserID, item)
		}
	}

	state.Wave = 24
	if got := s.resolveTriggerItemSpawns(state, "amusement_park_001"); len(got) != 0 {
		t.Fatalf("group spawned before its interval elapsed: %#v", got)
	}
	state.Wave = 25
	if got := s.resolveTriggerItemSpawns(state, "amusement_park_001"); len(got) != 1 {
		t.Fatalf("group should spawn again at wave 25, got %#v", got)
	}
	for index := range state.Players {
		if state.Players[index].UserID == 20 && state.Players[index].BattleItems[0].Quantity != 0 {
			t.Fatalf("next stable winner did not consume its ticket")
		}
	}
}

func TestPartyTriggerGroupChanceFailureDoesNotConsume(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{Seed: 9, Wave: 17, Players: []battleUnitState{{
		UserID: 1,
		BattleItems: []battleItemState{{
			ItemID: "haunted_after_hours_ticket", Quantity: 2, IsTrigger: true,
			DungeonIDs: []string{"amusement_park_001"}, SpawnEnemyIDs: []string{"midnight_ticket_collector_001"},
			MinWave: 17, IntervalWaves: 8, PerWaveChance: 0, PartyTriggerGroup: "amusement_after_hours",
		}},
	}}}
	if got := s.resolveTriggerItemSpawns(state, "amusement_park_001"); len(got) != 0 {
		t.Fatalf("zero chance unexpectedly spawned: %#v", got)
	}
	item := state.Players[0].BattleItems[0]
	if item.Quantity != 2 || item.LastSpawnWave != 0 {
		t.Fatalf("failed trigger mutated inventory/cooldown: %#v", item)
	}
}

func TestFogPhantomWaveStillBuildsAndConsumesTriggerEncounter(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 9,
		Wave: 17,
		Enemies: []battleUnitState{{
			UnitID: "fog:phantom:1", EnemyType: "fog_player", Side: "enemy", Alive: true, HP: 100, MaxHP: 100,
		}},
		Players: []battleUnitState{{
			UserID: 1,
			BattleItems: []battleItemState{{
				ItemID: "haunted_after_hours_ticket", Quantity: 2, IsTrigger: true,
				DungeonIDs: []string{"amusement_park_001"}, SpawnEnemyIDs: []string{"midnight_ticket_collector_001"},
				MinWave: 17, IntervalWaves: 8, PerWaveChance: 1, PartyTriggerGroup: "amusement_after_hours",
			}},
		}},
	}
	enemyCatalog := map[string]battleEnemyConfig{
		"midnight_ticket_collector_001": {
			ID: "midnight_ticket_collector_001", Name: "Ticket Inspector", Type: "elite", Position: "vanguard",
			Stats: map[string]float64{"hp": 18000, "mp": 0, "speed": 70},
		},
	}

	s.appendFogPhantomAuthoredEnemies(state, enemyCatalog, battleWorldConfig{ID: "amusement_park_001"}, "")
	if len(state.Enemies) != 2 || state.Enemies[1].EnemyID != "midnight_ticket_collector_001" {
		t.Fatalf("phantom wave did not append its ticket inspector: %#v", state.Enemies)
	}
	item := state.Players[0].BattleItems[0]
	if item.Quantity != 1 || item.LastSpawnWave != 17 {
		t.Fatalf("phantom-wave trigger was not authoritatively consumed: %#v", item)
	}

	state.Wave = 25
	state.Enemies = state.Enemies[:1]
	s.appendFogPhantomAuthoredEnemies(state, enemyCatalog, battleWorldConfig{ID: "amusement_park_001"}, "")
	if len(state.Enemies) != 2 || state.Enemies[1].EnemyID != "midnight_ticket_collector_001" {
		t.Fatalf("wave 25 phantom did not append exactly one ticket inspector: %#v", state.Enemies)
	}
	item = state.Players[0].BattleItems[0]
	if item.Quantity != 0 || item.LastSpawnWave != 25 {
		t.Fatalf("wave 25 phantom trigger was not authoritatively consumed: %#v", item)
	}
}
