package httpapi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

func TestBattleKillAchievementEventsFromTimeline(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"eventType": "battle.turn.action",
		"data": map[string]any{
			"actionLog": []map[string]any{
				{
					"kind":        "damage",
					"targetId":    "enemy:1:1:slime_001",
					"beforeAlive": true,
					"afterAlive":  false,
				},
				{
					"kind":        "damage",
					"targetId":    "enemy:1:2:slime_001",
					"beforeAlive": true,
					"afterAlive":  false,
				},
				{
					"kind":        "damage",
					"targetId":    "enemy:1:3:wolf_001",
					"beforeAlive": true,
					"afterAlive":  true,
				},
			},
		},
	})

	events := battleKillAchievementEventsFromResult(99, map[string]any{
		"outcome":    store.BattleRunStatusEnded,
		"finalState": map[string]any{"players": []map[string]any{}},
	}, []wsbus.BattleTimelineEvent{{Seq: 1, Payload: payload}})

	if len(events) != 1 {
		t.Fatalf("event count=%d, want 1: %#v", len(events), events)
	}
	if events[0].Kind != store.AchievementEventKillEnemy || events[0].EnemyID != "slime_001" || events[0].Count != 2 {
		t.Fatalf("unexpected kill event: %#v", events[0])
	}
}

func TestBattleKillAchievementEventsPreferAchievementStats(t *testing.T) {
	events := battleKillAchievementEventsFromResult(98, map[string]any{
		"achievementStats": map[string]any{
			"enemyKills": map[string]any{
				"slime_001": float64(3),
			},
		},
	}, nil)

	if len(events) != 1 {
		t.Fatalf("event count=%d, want 1: %#v", len(events), events)
	}
	if events[0].EnemyID != "slime_001" || events[0].Count != 3 {
		t.Fatalf("unexpected stats kill event: %#v", events[0])
	}
}

func TestBattleKillAchievementEventsPreferBattleStats(t *testing.T) {
	events := battleKillAchievementEventsFromResult(97, map[string]any{
		"battleStats": map[string]any{
			"enemyKills": map[string]any{
				"wolf_001": float64(4),
			},
		},
		"achievementStats": map[string]any{
			"enemyKills": map[string]any{
				"slime_001": float64(2),
			},
		},
	}, nil)

	if len(events) != 1 {
		t.Fatalf("event count=%d, want 1: %#v", len(events), events)
	}
	if events[0].EnemyID != "wolf_001" || events[0].Count != 4 {
		t.Fatalf("unexpected battle stats kill event: %#v", events[0])
	}
}

func TestBattleKillAchievementEventsFallsBackWhenBattleStatsEmpty(t *testing.T) {
	events := battleKillAchievementEventsFromResult(96, map[string]any{
		"battleStats": map[string]any{},
		"achievementStats": map[string]any{
			"enemyKills": map[string]any{
				"slime_001": float64(2),
			},
		},
	}, nil)

	if len(events) != 1 {
		t.Fatalf("event count=%d, want 1: %#v", len(events), events)
	}
	if events[0].EnemyID != "slime_001" || events[0].Count != 2 {
		t.Fatalf("unexpected fallback stats kill event: %#v", events[0])
	}
}

func TestBattleAchievementStatsCorrectsInferredKillEnemyID(t *testing.T) {
	stats := battleAchievementStatsFromState(battleRuntimeState{
		AchievementStats: battleAchievementStats{
			EnemyKills: map[string]int64{
				"1": 1,
			},
			KilledEnemyUnits: map[string]bool{
				"enemy:1": true,
			},
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", EnemyID: "slime_001", Alive: false},
		},
	})

	kills := normalizeAnyMap(stats["enemyKills"])
	if got := toInt64Loose(kills["slime_001"], 0); got != 1 {
		t.Fatalf("slime_001 kills=%d, want 1: %#v", got, kills)
	}
	if got := toInt64Loose(kills["1"], 0); got != 0 {
		t.Fatalf("inferred enemy id should be corrected, got %d: %#v", got, kills)
	}
}

func TestBattleKillAchievementEventsFromTimelineDedupesDeadUpdates(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"eventType": "battle.timeline.node",
		"data": map[string]any{
			"p": []map[string]any{
				{"k": "d", "t": []map[string]any{{"u": "enemy:1:1:slime_001", "l": false}}},
				{"k": "r", "t": []map[string]any{{"u": "enemy:1:1:slime_001", "l": false}}},
			},
		},
	})

	events := battleKillAchievementEventsFromResult(100, nil, []wsbus.BattleTimelineEvent{{Seq: 1, Payload: payload}})

	if len(events) != 1 {
		t.Fatalf("event count=%d, want 1: %#v", len(events), events)
	}
	if events[0].EnemyID != "slime_001" || events[0].Count != 1 {
		t.Fatalf("unexpected deduped kill event: %#v", events[0])
	}
}

func TestBattleKillAchievementEventBatchesPreserveRewardUsersAndSourceKeys(t *testing.T) {
	batches := battleKillAchievementEventBatches(77, map[int64]store.BattleWaveReward{
		12: {},
		4:  {},
		0:  {},
		-5: {},
	}, map[string]any{
		"battleStats": map[string]any{
			"enemyKills": map[string]any{"wolf_001": float64(2)},
		},
	}, nil)

	if len(batches) != 2 || batches[0].UserID != 4 || batches[1].UserID != 12 {
		t.Fatalf("achievement users=%v, want sorted positive reward users 4 and 12", batches)
	}
	for _, batch := range batches {
		if len(batch.Events) != 1 {
			t.Fatalf("user %d event count=%d, want 1", batch.UserID, len(batch.Events))
		}
		event := batch.Events[0]
		if event.SourceKey != "battle_run:77:kill:wolf_001" || event.EnemyID != "wolf_001" || event.Count != 2 {
			t.Fatalf("user %d kill event changed: %#v", batch.UserID, event)
		}
	}
}

func TestBattleKillAchievementBatchesAreFrozenBeforeCoreFinalization(t *testing.T) {
	raw, err := os.ReadFile("battle_runtime_finalize.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) settleBattleRunPlayback")
	if start < 0 {
		t.Fatal("settleBattleRunPlayback missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	batchesAt := strings.Index(body, "killAchievementBatches = battleKillAchievementEventBatches")
	finalizeAt := strings.Index(body, "CompleteBattleRunAndMaybeContinue(ctx")
	if batchesAt < 0 || finalizeAt <= batchesAt {
		t.Fatal("kill achievement batches must be frozen before the core finalization transaction")
	}
	for _, expected := range []string{
		"skillExpJobPayload, killAchievementBatches, now, interBattleDelay",
		"skillExpJobPayload, killAchievementBatches, now, recoveryDuration",
		"skillExpJobPayload, killAchievementBatches, now)",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("run-fenced kill achievement settlement missing %q", expected)
		}
	}
	if strings.Contains(body, "applyBattleKillAchievementEvents(ctx") {
		t.Fatal("new finalizations must not re-enqueue kill achievements after the core transaction")
	}
	repairRaw, err := os.ReadFile("battle_settlement_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repairRaw), "func (s *Server) reconcileBattleSettlementPostFinalize") ||
		!strings.Contains(string(repairRaw), "applyBattleKillAchievementEvents") ||
		!strings.Contains(source, "func (s *Server) applyBattleKillAchievementEvents") {
		t.Fatal("idempotent terminal achievement repair path must remain available")
	}
}
