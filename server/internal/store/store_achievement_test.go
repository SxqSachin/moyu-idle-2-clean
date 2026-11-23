package store

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAchievementDefinitionsFromRowStagesUnlockFromFirstStage(t *testing.T) {
	defs := achievementDefinitionsFromRow(map[string]any{
		"id":                   "kill_slime",
		"categoryId":           "combat",
		"rarity":               "common",
		"order":                float64(10),
		"stageProgressMode":    "cumulative",
		"prereqAchievementIds": []any{},
		"condition": map[string]any{
			"kind":    "kill_enemy",
			"enemyId": "slime_001",
			"count":   float64(10),
		},
		"stages": []any{
			map[string]any{
				"condition": map[string]any{
					"kind":    "kill_enemy",
					"enemyId": "slime_001",
					"count":   float64(10),
				},
			},
			map[string]any{
				"condition": map[string]any{
					"kind":    "kill_enemy",
					"enemyId": "slime_001",
					"count":   float64(100),
				},
			},
		},
	})
	if len(defs) != 2 {
		t.Fatalf("expected 2 stage definitions, got %d", len(defs))
	}
	if defs[0].ID != "kill_slime__stage_1" {
		t.Fatalf("unexpected first stage id: %s", defs[0].ID)
	}
	if len(defs[0].PrereqAchievementIDs) != 0 {
		t.Fatalf("first stage should inherit no prerequisites, got %#v", defs[0].PrereqAchievementIDs)
	}
	if defs[1].ID != "kill_slime__stage_2" {
		t.Fatalf("unexpected second stage id: %s", defs[1].ID)
	}
	if got := defs[1].PrereqAchievementIDs; len(got) != 1 || got[0] != "kill_slime__stage_1" {
		t.Fatalf("second stage should depend on first stage, got %#v", got)
	}
	if defs[0].StageProgressMode != "cumulative" || defs[1].StageProgressMode != "cumulative" {
		t.Fatalf("stage progress mode should be inherited from base row, got %q and %q", defs[0].StageProgressMode, defs[1].StageProgressMode)
	}
}

func TestCumulativeAchievementStageCarriesActualProgress(t *testing.T) {
	tests := []struct {
		name      string
		condition map[string]any
	}{
		{
			name: "complete action",
			condition: map[string]any{
				"kind":      "complete_action",
				"actionId":  "collect_clear_resin",
				"actorType": "player",
			},
		},
		{
			name: "kill enemy",
			condition: map[string]any{
				"kind":    "kill_enemy",
				"enemyId": "slime_001",
			},
		},
		{
			name: "catch fish",
			condition: map[string]any{
				"kind":      "catch_fish",
				"speciesId": "crucian_carp",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := map[string]any{
				"id":                "multi_stage",
				"categoryId":        "action",
				"rarity":            "common",
				"stageProgressMode": "cumulative",
				"condition":         tc.condition,
				"stages": []any{
					stageCondition(tc.condition, 10),
					stageCondition(tc.condition, 100),
					stageCondition(tc.condition, 1000),
					stageCondition(tc.condition, 10000),
				},
			}
			defs := achievementDefinitionsFromRow(row)
			s := Store{achievementDefs: defs}
			states := map[string]achievementUnlockState{
				defs[0].ID: {ProgressValue: 5000, CompletedAt: timePtrForTest()},
			}

			progress, completed := s.initialAchievementProgressTx(defs[1], states)
			if progress != 5000 || !completed {
				t.Fatalf("stage 2 initial progress/completed = %d/%t, want 5000/true", progress, completed)
			}
			states[defs[1].ID] = achievementUnlockState{ProgressValue: progress, CompletedAt: timePtrForTest()}

			progress, completed = s.initialAchievementProgressTx(defs[2], states)
			if progress != 5000 || !completed {
				t.Fatalf("stage 3 initial progress/completed = %d/%t, want 5000/true", progress, completed)
			}
			states[defs[2].ID] = achievementUnlockState{ProgressValue: progress, CompletedAt: timePtrForTest()}

			progress, completed = s.initialAchievementProgressTx(defs[3], states)
			if progress != 5000 || completed {
				t.Fatalf("stage 4 initial progress/completed = %d/%t, want 5000/false", progress, completed)
			}
		})
	}
}

func TestFishingAchievementCounterKeys(t *testing.T) {
	def := achievementDefinition{Condition: map[string]any{
		"kind":      "catch_fish",
		"speciesId": "crucian_carp",
		"count":     float64(20),
	}}
	if got := achievementConditionCounterKey(def); got != "catch_fish:crucian_carp" {
		t.Fatalf("condition counter key = %q", got)
	}
	if got := achievementEventPrimaryCounterKey(AchievementEvent{
		Kind:      AchievementEventStateReconcile,
		StateKind: "catch_fish",
	}); got != "state_reconcile:catch_fish" {
		t.Fatalf("state reconcile counter key = %q", got)
	}
}

func TestPrepareAchievementEventBatchRowsPreservesIdentityAndCounts(t *testing.T) {
	actionEvent := AchievementEvent{
		Kind:      AchievementEventCompleteAction,
		ActionID:  "collect_clear_resin",
		ActorType: ActionActorTypePlayer,
		Count:     3,
		SourceKey: "action_queue:7:3",
	}
	rows, err := prepareAchievementEventBatchRows([]AchievementEventBatch{
		{
			UserID: 42,
			Events: []AchievementEvent{
				actionEvent,
				actionEvent,
				{
					Kind:      AchievementEventKillEnemy,
					EnemyID:   "slime_001",
					Count:     0,
					SourceKey: "battle_run:9:kill:slime_001",
				},
				{},
			},
		},
		{UserID: 0, Events: []AchievementEvent{actionEvent}},
	})
	if err != nil {
		t.Fatalf("prepare achievement rows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("prepared rows=%d, want 3", len(rows))
	}
	if rows[0].EventID != rows[1].EventID {
		t.Fatalf("duplicate source identity changed: %q != %q", rows[0].EventID, rows[1].EventID)
	}
	if rows[0].CounterKey != "complete_action:player:collect_clear_resin" || rows[0].Delta != 3 {
		t.Fatalf("action row changed: %#v", rows[0])
	}
	if rows[2].EventID != "42:kill_enemy:slime_001:battle_run:9:kill:slime_001" || rows[2].Delta != 1 {
		t.Fatalf("normalized kill row changed: %#v", rows[2])
	}
	var payload AchievementEvent
	if err := json.Unmarshal([]byte(rows[0].Payload), &payload); err != nil {
		t.Fatalf("decode prepared payload: %v", err)
	}
	if payload.Count != actionEvent.Count || payload.SourceKey != actionEvent.SourceKey || payload.ActionID != actionEvent.ActionID {
		t.Fatalf("prepared payload lost event fields: %#v", payload)
	}
}

func TestAchievementEventBatchInsertPreservesConflictAndCommitNotificationOrdering(t *testing.T) {
	raw, err := os.ReadFile("store_achievement.go")
	if err != nil {
		t.Fatalf("read achievement store: %v", err)
	}
	source := string(raw)
	insertBody := achievementFunctionBodyForTest(t, source, "func (s *Store) insertAchievementEventRowsTx")
	insertSQL := normalizeAchievementSQL(insertBody)
	for _, required := range []string{
		"from unnest($1::text[], $2::bigint[], $3::text[], $4::text[], $5::bigint[], $6::text[])",
		"as input(event_id, user_id, kind, counter_key, delta, payload)",
		"on conflict (event_id) do nothing",
	} {
		if !strings.Contains(insertSQL, required) {
			t.Fatalf("batched achievement insert missing %q", required)
		}
	}

	bulkBody := achievementFunctionBodyForTest(t, source, "func (s *Store) BulkEnqueueAchievementEvents")
	prepareAt := strings.Index(bulkBody, "prepareAchievementEventBatchRows")
	transactionAt := strings.Index(bulkBody, "s.withTx")
	insertAt := strings.Index(bulkBody, "s.insertAchievementEventRowsTx")
	notifyAt := strings.Index(bulkBody, "s.achievementNotify(ctx)")
	if prepareAt < 0 || transactionAt <= prepareAt || insertAt <= transactionAt || notifyAt <= insertAt {
		t.Fatal("bulk achievement enqueue no longer prepares once, inserts transactionally, then notifies after commit")
	}
	if strings.Contains(bulkBody, "s.EnqueueAchievementEventsTx") {
		t.Fatal("bulk achievement enqueue regressed to one INSERT call per buffered batch")
	}
}

func TestAchievementEventIndexesMatchCleanupMigration(t *testing.T) {
	initRaw, err := os.ReadFile("../../schema/init.sql")
	if err != nil {
		t.Fatalf("read init schema: %v", err)
	}
	initSQL := normalizeAchievementSQL(string(initRaw))
	for _, required := range []string{
		"create index if not exists idx_achievement_events_pending_claim on achievement_events(status, available_at, created_at)",
		"create index if not exists idx_achievement_events_processing_claim on achievement_events(status, locked_until, created_at)",
		"create index if not exists idx_achievement_events_pending_due on achievement_events(available_at, created_at, event_id) where status = 'pending'",
		"create index if not exists idx_achievement_events_processing_due on achievement_events(locked_until, created_at, event_id) where status = 'processing' and locked_until is not null",
		"create index if not exists idx_achievement_events_processed on achievement_events(status, processed_at)",
	} {
		if !strings.Contains(initSQL, required) {
			t.Fatalf("fresh schema missing achievement event index %q", required)
		}
	}
	for _, obsolete := range []string{
		"create index if not exists idx_achievement_events_claim",
		"create index if not exists idx_achievement_events_user_status",
	} {
		if strings.Contains(initSQL, obsolete) {
			t.Fatalf("fresh schema must not recreate obsolete index: %s", obsolete)
		}
	}

	cleanupRaw, err := os.ReadFile("../../schema/migrations/20260902_achievement_event_index_cleanup.sql")
	if err != nil {
		t.Fatalf("read achievement index cleanup migration: %v", err)
	}
	cleanupSQL := normalizeAchievementSQL(string(cleanupRaw))
	for _, required := range []string{
		"from pg_index",
		"and indisready",
		"and indisvalid",
		"idx_achievement_events_pending_claim",
		"idx_achievement_events_processing_claim",
		"idx_achievement_events_pending_due",
		"idx_achievement_events_processing_due",
		"drop index concurrently if exists idx_achievement_events_claim",
		"drop index concurrently if exists idx_achievement_events_user_status",
	} {
		if !strings.Contains(cleanupSQL, required) {
			t.Fatalf("achievement index cleanup migration missing %q", required)
		}
	}
	if strings.Count(cleanupSQL, "drop index concurrently if exists") != 2 {
		t.Fatal("achievement index cleanup must drop exactly the two obsolete indexes concurrently")
	}
	if strings.Contains(cleanupSQL, "begin;") || strings.Contains(cleanupSQL, "commit;") {
		t.Fatal("concurrent achievement index cleanup must not use an explicit transaction")
	}
}

func stageCondition(base map[string]any, count int64) map[string]any {
	condition := map[string]any{}
	for key, value := range base {
		condition[key] = value
	}
	condition["count"] = float64(count)
	return map[string]any{"condition": condition}
}

func normalizeAchievementSQL(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}

func achievementFunctionBodyForTest(t *testing.T, source string, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("missing function %q", signature)
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	return body
}

func timePtrForTest() *time.Time {
	now := time.Now()
	return &now
}
