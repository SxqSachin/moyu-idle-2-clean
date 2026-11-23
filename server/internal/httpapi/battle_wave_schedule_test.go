package httpapi

import "testing"

func TestBattleWaveSchedulesUseHighestPriorityMatch(t *testing.T) {
	s := &Server{}
	cfg := battleWorldConfig{WaveSchedules: []battleWaveSchedule{
		{
			ID: "parade_king", Every: 8, Priority: 20, MinCount: 1, MaxCount: 1,
			EnemyChoices: []battleWaveScheduleEnemyChoice{{EnemyID: "parade_king_boss_001", Weight: 1}},
		},
		{
			ID: "park_elites", Every: 4, Priority: 10, MinCount: 1, MaxCount: 3,
			EnemyChoices: []battleWaveScheduleEnemyChoice{
				{EnemyID: "candy_clown_001", Weight: 40},
				{EnemyID: "carousel_cat_001", Weight: 30},
				{EnemyID: "ghostly_rider_001", Weight: 30},
			},
		},
	}}

	for _, wave := range []int{8, 16} {
		state := &battleRuntimeState{Seed: 91, Wave: wave}
		ids := s.resolveWaveScheduleEnemyIDs(state, cfg)
		if len(ids) != 1 || ids[0] != "parade_king_boss_001" {
			t.Fatalf("wave %d should choose only the higher-priority parade boss, got %#v", wave, ids)
		}
		if state.RNGIndex != 0 {
			t.Fatalf("fixed-count single-choice schedule should not consume RNG, got index %d", state.RNGIndex)
		}
	}

	allowed := map[string]bool{
		"candy_clown_001":   true,
		"carousel_cat_001":  true,
		"ghostly_rider_001": true,
	}
	for _, wave := range []int{4, 12} {
		state := &battleRuntimeState{Seed: 91, Wave: wave}
		ids := s.resolveWaveScheduleEnemyIDs(state, cfg)
		if len(ids) < 1 || len(ids) > 3 {
			t.Fatalf("wave %d elite count out of range: %#v", wave, ids)
		}
		for _, id := range ids {
			if !allowed[id] {
				t.Fatalf("wave %d selected enemy outside elite pool: %q", wave, id)
			}
		}
	}
}

func TestFogPhantomWaveAppendsOnlyHighestPrioritySchedule(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{
		Seed: 91,
		Wave: 8,
		Enemies: []battleUnitState{{
			UnitID: "fog:phantom:1", EnemyType: "fog_player", Side: "enemy", Alive: true, HP: 100, MaxHP: 100,
		}},
	}
	worldCfg := battleWorldConfig{ID: "amusement_park_001", WaveSchedules: []battleWaveSchedule{
		{
			ID: "parade_king", Every: 8, Priority: 20, MinCount: 1, MaxCount: 1,
			EnemyChoices: []battleWaveScheduleEnemyChoice{{EnemyID: "parade_king_boss_001", Weight: 1}},
		},
		{
			ID: "park_elites", Every: 4, Priority: 10, MinCount: 1, MaxCount: 3,
			EnemyChoices: []battleWaveScheduleEnemyChoice{{EnemyID: "candy_clown_001", Weight: 1}},
		},
	}}
	enemyCatalog := map[string]battleEnemyConfig{
		"parade_king_boss_001": {
			ID: "parade_king_boss_001", Name: "Parade King", Type: "boss", Position: "vanguard",
			Stats: map[string]float64{"hp": 22000, "mp": 0, "speed": 70},
		},
		"candy_clown_001": {
			ID: "candy_clown_001", Name: "Candy Clown", Type: "elite", Position: "middle",
			Stats: map[string]float64{"hp": 4500, "mp": 0, "speed": 80},
		},
	}

	s.appendFogPhantomAuthoredEnemies(state, enemyCatalog, worldCfg, "")

	paradeKings := 0
	clowns := 0
	for _, enemy := range state.Enemies {
		switch enemy.EnemyID {
		case "parade_king_boss_001":
			paradeKings++
		case "candy_clown_001":
			clowns++
		}
	}
	if paradeKings != 1 || clowns != 0 || len(state.Enemies) != 2 {
		t.Fatalf("wave 8 phantom should keep itself and append exactly one parade king, got %#v", state.Enemies)
	}
	if state.RNGIndex != 0 {
		t.Fatalf("fixed-count single-choice phantom schedule should not consume RNG, got index %d", state.RNGIndex)
	}
}

func TestBattleWaveSchedulesDoNotAffectLegacyWorldRNG(t *testing.T) {
	s := &Server{}
	state := &battleRuntimeState{Seed: 7, Wave: 4}
	if ids := s.resolveWaveScheduleEnemyIDs(state, battleWorldConfig{}); len(ids) != 0 {
		t.Fatalf("unexpected schedule enemies: %#v", ids)
	}
	if state.RNGIndex != 0 {
		t.Fatalf("world without schedules consumed RNG: %d", state.RNGIndex)
	}
}

func TestParseBattleWaveSchedulesFiltersDisabledEnemies(t *testing.T) {
	raw := []any{map[string]any{
		"id": "park_elites", "every": 4, "priority": 10, "minCount": 1, "maxCount": 3,
		"enemyChoices": []any{
			map[string]any{"enemyId": "enabled", "weight": 40.0},
			map[string]any{"enemyId": "disabled", "weight": 60.0},
		},
	}}
	parsed := parseBattleWaveSchedules(raw, map[string]struct{}{"enabled": {}})
	if len(parsed) != 1 || len(parsed[0].EnemyChoices) != 1 || parsed[0].EnemyChoices[0].EnemyID != "enabled" {
		t.Fatalf("unexpected normalized schedules: %#v", parsed)
	}
}
