package store

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

func TestComputeCompletedRuns(t *testing.T) {
	duration := 2 * time.Second
	tests := []struct {
		name      string
		elapsed   time.Duration
		remaining int
		want      int
	}{
		{name: "at least one when due", elapsed: 100 * time.Millisecond, remaining: 5, want: 1},
		{name: "multiple by elapsed", elapsed: 9 * time.Second, remaining: 10, want: 4},
		{name: "capped by remaining", elapsed: 100 * time.Second, remaining: 3, want: 3},
		{name: "zero when no remaining", elapsed: 100 * time.Second, remaining: 0, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeCompletedRuns(tc.elapsed, duration, tc.remaining)
			if got != tc.want {
				t.Fatalf("computeCompletedRuns() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestComputeAdditionalStarts(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		doneAfter int
		total     int
		want      int
	}{
		{name: "queue finished", completed: 3, doneAfter: 10, total: 10, want: 2},
		{name: "queue continues running", completed: 3, doneAfter: 7, total: 10, want: 3},
		{name: "no completed", completed: 0, doneAfter: 0, total: 10, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeAdditionalStarts(tc.completed, tc.doneAfter, tc.total)
			if got != tc.want {
				t.Fatalf("computeAdditionalStarts() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestShouldYieldActionQueueAdvanceBudget(t *testing.T) {
	started := time.Now().Add(-2 * time.Millisecond)
	if !shouldYieldActionQueueAdvanceBudget(started, time.Millisecond, nil) {
		t.Fatal("budget should yield after max exec elapses")
	}
	if shouldYieldActionQueueAdvanceBudget(started, 0, nil) {
		t.Fatal("zero max exec should not yield")
	}
	if shouldYieldActionQueueAdvanceBudget(time.Time{}, time.Millisecond, nil) {
		t.Fatal("zero start should not yield")
	}
	if shouldYieldActionQueueAdvanceBudget(started, time.Millisecond, map[string]time.Time{"player:0": started}) {
		t.Fatal("lane carry should finish before yielding")
	}
}

func TestActionQueueCarryLaneKeysAreStable(t *testing.T) {
	started := time.Now()
	got := actionQueueCarryLaneKeys(map[string]time.Time{
		"kitty:9":  started,
		"player:0": started,
		"kitty:2":  started,
	})
	want := []string{"kitty:2", "kitty:9", "player:0"}
	if len(got) != len(want) {
		t.Fatalf("lane keys=%v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("lane keys=%v, want %v", got, want)
		}
	}
}

func TestFastForwardBudgetIterationsIncludesPhaseProgress(t *testing.T) {
	tests := []struct {
		name   string
		result SettleResult
		want   int
	}{
		{name: "completed actions", result: SettleResult{Completed: 7}, want: 7},
		{name: "fishing phases", result: SettleResult{Progressed: true, BudgetIterations: 23}, want: 23},
		{name: "never undercounts completion", result: SettleResult{Completed: 4, BudgetIterations: 2}, want: 4},
		{name: "empty result", result: SettleResult{}, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fastForwardBudgetIterations(tc.result); got != tc.want {
				t.Fatalf("fastForwardBudgetIterations() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestActionQueueSettleMadeProgressContract(t *testing.T) {
	tests := []struct {
		name                    string
		result                  SettleResult
		requireBudgetIterations bool
		want                    bool
	}{
		{name: "completed action", result: SettleResult{Completed: 1}, want: true},
		{name: "fast-forward state machine phase", result: SettleResult{Progressed: true, BudgetIterations: 1}, requireBudgetIterations: true, want: true},
		{name: "unbudgeted fast-forward state machine phase", result: SettleResult{Progressed: true}, requireBudgetIterations: true, want: false},
		{name: "normal state machine phase", result: SettleResult{Progressed: true}, want: true},
		{name: "explicit cancellation", result: SettleResult{CancelAfterSettle: true}, want: true},
		{name: "empty result stalls", result: SettleResult{}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := actionQueueSettleMadeProgress(tc.result, tc.requireBudgetIterations); got != tc.want {
				t.Fatalf("actionQueueSettleMadeProgress(%+v, %t) = %t, want %t", tc.result, tc.requireBudgetIterations, got, tc.want)
			}
		})
	}
}

func TestShouldApplyOfflineFinalWorkLoadout(t *testing.T) {
	tests := []struct {
		name              string
		hasMorePlayerDue  bool
		stalledPlayerLane bool
		want              bool
	}{
		{name: "player lane caught up", want: true},
		{name: "player lane continues", hasMorePlayerDue: true, want: false},
		{name: "current player action stalled", hasMorePlayerDue: true, stalledPlayerLane: true, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldApplyOfflineFinalWorkLoadout(tc.hasMorePlayerDue, tc.stalledPlayerLane); got != tc.want {
				t.Fatalf("shouldApplyOfflineFinalWorkLoadout(%t, %t) = %t, want %t", tc.hasMorePlayerDue, tc.stalledPlayerLane, got, tc.want)
			}
		})
	}
}

func TestQueueExecutorsDeclareOfflineProjectionPolicy(t *testing.T) {
	if QueueOfflineProjectionImmediate != 0 {
		t.Fatal("immediate projection must remain the zero-value safe default")
	}
	tests := []struct {
		name     string
		executor QueueActionExecutor
		want     QueueOfflineProjectionPolicy
	}{
		{name: "generic", executor: newGenericExecutor(nil), want: QueueOfflineProjectionTerminal},
		{name: "building", executor: newBuildingExecutor(nil), want: QueueOfflineProjectionTerminal},
		{name: "alchemy", executor: newAlchemyExecutor(nil, "alchemy.convert"), want: QueueOfflineProjectionTerminal},
		{name: "enhance", executor: newEnhanceExecutor(nil), want: QueueOfflineProjectionImmediate},
		{name: "fishing", executor: newFishingExecutor(nil), want: QueueOfflineProjectionImmediate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.executor.OfflineProjectionPolicy(); got != tc.want {
				t.Fatalf("projection policy=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestFishingCatchAchievementReconcileEvent(t *testing.T) {
	completedAt := time.UnixMilli(1_725_000_123_456)
	event, ok := fishingCatchAchievementReconcileEvent(42, SettleResult{
		CompletedAt: completedAt,
		FishingCatches: map[string]FishingCatchTally{
			"crucian_carp": {Landed: 2, Kept: 1, Released: 1},
		},
	})
	if !ok {
		t.Fatal("landed fishing catch should emit an achievement reconcile event")
	}
	if event.Kind != AchievementEventStateReconcile || event.StateKind != "catch_fish" {
		t.Fatalf("unexpected fishing achievement event: %+v", event)
	}
	if event.SourceKey != "fishing_catch:42:1725000123456" {
		t.Fatalf("unexpected source key: %q", event.SourceKey)
	}
	if _, ok := fishingCatchAchievementReconcileEvent(42, SettleResult{
		CompletedAt: completedAt,
		FishingCatches: map[string]FishingCatchTally{
			"crucian_carp": {Landed: 0},
		},
	}); ok {
		t.Fatal("zero landed catches must not emit an achievement reconcile event")
	}
	if _, ok := fishingCatchAchievementReconcileEvent(42, SettleResult{
		FishingCatches: map[string]FishingCatchTally{
			"crucian_carp": {Landed: 1},
		},
	}); ok {
		t.Fatal("a catch without a settled timestamp must not emit an unstable event key")
	}
}

func TestActionQueueOfflineFastForwardIndexesExist(t *testing.T) {
	files := []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260629_action_queue_offline_fastforward_indexes.sql",
	}
	required := []string{
		"idx_player_action_queue_running_due_user",
		"idx_player_action_queue_pending_user_order",
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(raw)
		for _, indexName := range required {
			if !strings.Contains(text, indexName) {
				t.Fatalf("%s missing %s", file, indexName)
			}
		}
	}
}

func TestOfflineQueueRunDefersFinalWorkLoadoutInsideTransaction(t *testing.T) {
	raw, err := os.ReadFile("action_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) startQueueRunTx")
	if start < 0 {
		t.Fatal("startQueueRunTx not found")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	executorAt := strings.Index(body, "executor.StartRunTx")
	if executorAt < 0 {
		t.Fatal("queue start executor call not found")
	}
	if !strings.Contains(body, "workLoadoutActivationImmediate") {
		t.Fatal("queue start must make immediate work-loadout activation explicit")
	}

	advanceStart := strings.Index(source, "func (s *Store) advanceActionQueueInternal")
	if advanceStart < 0 {
		t.Fatal("advanceActionQueueInternal not found")
	}
	advanceBody := source[advanceStart:]
	if advanceEnd := strings.Index(advanceBody, "\nfunc shouldYieldActionQueueAdvanceBudget"); advanceEnd >= 0 {
		advanceBody = advanceBody[:advanceEnd]
	}
	deferAt := strings.Index(advanceBody, "workLoadoutActivationDeferred")
	handoffAt := strings.Index(advanceBody, "handoffOnly := offlineLog")
	handoffFilterAt := strings.Index(advanceBody, "ANY($4::text[])")
	playerLaneDueAt := strings.Index(advanceBody, "s.hasPlayerActionQueueDueUntilTx")
	finalApplyAt := strings.Index(advanceBody, "s.applyWorkLoadoutTx(ctx, tx, userID, summary.FinalWorkLoadoutID)")
	clearIntentAt := strings.Index(advanceBody, "s.clearOfflineWorkLoadoutIntentTx(ctx, tx, userID, summary.FinalWorkLoadoutID)")
	commitAt := strings.Index(advanceBody, "tx.Commit(ctx)")
	if deferAt < 0 || handoffAt < 0 || handoffFilterAt < 0 || playerLaneDueAt < 0 || finalApplyAt < 0 || commitAt < 0 || playerLaneDueAt > finalApplyAt || finalApplyAt > commitAt {
		t.Fatal("offline settlement must defer and atomically apply only its final work loadout before commit")
	}
	if clearIntentAt < finalApplyAt || clearIntentAt > commitAt {
		t.Fatal("applied offline work-loadout intent must be cleared in the same transaction so later recovery cannot replay it")
	}
	if strings.Contains(advanceBody[commitAt:], "ApplyWorkLoadout") || strings.Contains(advanceBody[commitAt:], "applyWorkLoadout") {
		t.Fatal("offline settlement must not apply a work loadout after commit")
	}
}

func TestDeferredWorkLoadoutSurvivesRuntimeCheckpointUpdates(t *testing.T) {
	runtimeRaw, err := json.Marshal(genericExecutorRuntime{DurationMs: 2_000})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "collect.herb",
		ExecutorID: "generic",
		Runtime:    runtimeRaw,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload, err = setQueueRuntimeDeferredWorkLoadoutID(payload, "loadout-herbalism")
	if err != nil {
		t.Fatal(err)
	}
	if got := queueRuntimeDeferredWorkLoadoutID(payload); got != "loadout-herbalism" {
		t.Fatalf("deferred loadout=%q, want loadout-herbalism", got)
	}

	payload, err = setQueueRuntimePayload(payload, genericExecutorRuntime{DurationMs: 3_000})
	if err != nil {
		t.Fatal(err)
	}
	if got := queueRuntimeDeferredWorkLoadoutID(payload); got != "loadout-herbalism" {
		t.Fatalf("checkpoint update dropped deferred loadout: %q", got)
	}

	legacyPayload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "collect.herb",
		ExecutorID: "generic",
		Runtime:    runtimeRaw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := queueRuntimeDeferredWorkLoadoutID(legacyPayload); got != "" {
		t.Fatalf("legacy runtime unexpectedly resolved deferred loadout %q", got)
	}
}

func TestDeferredWorkLoadoutSurvivesSpecializedExecutorCheckpoints(t *testing.T) {
	tests := []struct {
		name       string
		executorID string
		update     func([]byte) (json.RawMessage, error)
	}{
		{
			name:       "building",
			executorID: "building_upgrade",
			update: func(payload []byte) (json.RawMessage, error) {
				return setBuildingRuntimeAndContext(payload, buildingExecutorRuntime{BuildingID: "forge"}, buildingRunContext{BuildingID: "forge"})
			},
		},
		{
			name:       "enhance",
			executorID: "equipment_enhance",
			update: func(payload []byte) (json.RawMessage, error) {
				return setEnhanceRunContext(payload, enhanceRunContext{})
			},
		},
		{
			name:       "fishing",
			executorID: "fishing_trip",
			update: func(payload []byte) (json.RawMessage, error) {
				return setFishingExecutorRuntime(payload, fishingExecutorRuntime{TripID: "trip-1"})
			},
		},
		{
			name:       "alchemy",
			executorID: "alchemy.convert",
			update: func(payload []byte) (json.RawMessage, error) {
				return setAlchemyReservedCosts(payload, alchemyConvertArgs{}, map[string]int64{"herb": 1})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtimeRaw, err := json.Marshal(map[string]any{"durationMs": 2_000})
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(queueRuntimeEnvelope{
				ActionID:              tc.executorID,
				ExecutorID:            tc.executorID,
				Runtime:               runtimeRaw,
				DeferredWorkLoadoutID: "loadout-final",
			})
			if err != nil {
				t.Fatal(err)
			}
			updated, err := tc.update(payload)
			if err != nil {
				t.Fatal(err)
			}
			if got := queueRuntimeDeferredWorkLoadoutID(updated); got != "loadout-final" {
				t.Fatalf("checkpoint update dropped deferred loadout: %q", got)
			}
		})
	}
}

func TestOfflineChunkSkipsPostCommitReadModels(t *testing.T) {
	raw, err := os.ReadFile("action_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Store) AdvanceActionQueueOfflineChunk")
	if start < 0 {
		t.Fatal("AdvanceActionQueueOfflineChunk not found")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	for _, required := range []string{"SkipReadModels: true", "SkipQueueRead:  true"} {
		if !strings.Contains(body, required) {
			t.Fatalf("offline chunk missing %q", required)
		}
	}
}

func TestCollectActionEfficiencyAttributeKeys(t *testing.T) {
	keys := collectActionEfficiencyAttributeKeys(
		[]catalog.AttributeDefinition{
			{ID: "collecting_speed", RelatedActionMainType: "collecting", RelatedActionEffect: "actionEfficiency"},
			{ID: "collecting_bonus_legacy", RelatedActionMainType: "collecting", RelatedActionEffect: ""},
			{ID: "collecting_other_effect", RelatedActionMainType: "collecting", RelatedActionEffect: "someOtherEffect"},
			{ID: "mining_speed", RelatedActionMainType: "mining", RelatedActionEffect: "actionEfficiency"},
		},
		"collecting",
	)
	if len(keys) != 2 {
		t.Fatalf("len(keys)=%d, want 2", len(keys))
	}
	if keys[0] != "collecting_speed" || keys[1] != "collecting_bonus_legacy" {
		t.Fatalf("keys=%v, want collecting_speed/collecting_bonus_legacy", keys)
	}
}

func TestCollectActionEfficiencyAttributeKeysIgnoresAlchemyLevel(t *testing.T) {
	keys := collectActionEfficiencyAttributeKeys(
		[]catalog.AttributeDefinition{
			{ID: "alchemy_level", RelatedActionMainType: "", RelatedActionEffect: ""},
			{ID: "alchemy_efficiency", RelatedActionMainType: "alchemy", RelatedActionEffect: "actionEfficiency"},
			{ID: "alchemy_precision", RelatedActionMainType: "alchemy", RelatedActionEffect: "rangeBonus"},
		},
		"alchemy",
	)
	if len(keys) != 1 || keys[0] != "alchemy_efficiency" {
		t.Fatalf("keys=%v, want only alchemy_efficiency", keys)
	}
}

func TestComputeEffectiveActionDuration(t *testing.T) {
	base := 44 * time.Second
	got := computeEffectiveActionDuration(base, 1)
	if got != 22*time.Second {
		t.Fatalf("duration=%v, want 22s", got)
	}
	got = computeEffectiveActionDuration(base, 2)
	if got != 14*time.Second+666*time.Millisecond+666*time.Microsecond+666*time.Nanosecond {
		t.Fatalf("duration=%v, want 14.666666666s", got)
	}
	got = computeEffectiveActionDuration(base, 0.5)
	if got != 29*time.Second+333*time.Millisecond+333*time.Microsecond+333*time.Nanosecond {
		t.Fatalf("duration=%v, want 29.333333333s", got)
	}
	got = computeEffectiveActionDuration(base, -0.5)
	if got != 66*time.Second {
		t.Fatalf("duration=%v, want 66s", got)
	}
	got = computeEffectiveActionDuration(base, -1)
	if got != 88*time.Second {
		t.Fatalf("duration=%v, want 88s", got)
	}
}

func TestComputeEffectiveActionDurationWithFloor(t *testing.T) {
	base := 16 * time.Second
	got := computeEffectiveActionDurationWithFloor(base, -1, 0.5)
	if got != 32*time.Second {
		t.Fatalf("duration=%v, want 32s for -100%% efficiency", got)
	}
	got = computeEffectiveActionDurationWithFloor(base, -0.5, 0.5)
	if got != 24*time.Second {
		t.Fatalf("duration=%v, want 24s for -50%% efficiency", got)
	}
	got = computeEffectiveActionDurationWithFloor(base, 99, 0.5)
	if got != 8*time.Second {
		t.Fatalf("duration=%v, want 8s when kitty speed floor is 0.5", got)
	}
	got = computeEffectiveActionDurationWithFloor(base, 999, 0.05)
	if got != minimumActionQueueRunDuration {
		t.Fatalf("duration=%v, want global queue minimum %v", got, minimumActionQueueRunDuration)
	}
	got = computeEffectiveActionDurationWithFloor(base, 1, 0.5)
	if got != 8*time.Second {
		t.Fatalf("duration=%v, want 8s for kitty 100%% efficiency", got)
	}
	got = computeEffectiveActionDuration(40*time.Second, 30)
	if got != 2*time.Second {
		t.Fatalf("duration=%v, want 2s for 40s action at 3000%% efficiency", got)
	}
}

func TestNormalizeActionQueueRunDuration(t *testing.T) {
	for _, duration := range []time.Duration{0, time.Second, 1999 * time.Millisecond} {
		if got := normalizeActionQueueRunDuration(duration); got != minimumActionQueueRunDuration {
			t.Fatalf("normalize duration=%v got=%v, want=%v", duration, got, minimumActionQueueRunDuration)
		}
	}
	if got := normalizeActionQueueRunDuration(2500 * time.Millisecond); got != 2500*time.Millisecond {
		t.Fatalf("duration above minimum changed to %v", got)
	}
}

func TestResolveActionRewardBatchQuantity(t *testing.T) {
	countReward := int64(2)
	percent := 1.0
	reward := ActionItemRewardSpec{
		Count:   &countReward,
		Percent: &percent,
	}
	got := resolveActionRewardBatchQuantity(reward, 1000, queueRewardRNG(1, 2, 0))
	if got != 2000 {
		t.Fatalf("fixed reward batch = %d, want 2000", got)
	}
}

func TestQueueRewardRNGDeterministic(t *testing.T) {
	r1 := queueRewardRNG(100, 200, 3)
	r2 := queueRewardRNG(100, 200, 3)
	for i := 0; i < 5; i++ {
		a := r1.Int63()
		b := r2.Int63()
		if a != b {
			t.Fatalf("determinism mismatch at %d: %d != %d", i, a, b)
		}
	}
}

func TestAdjustCompletedForAffordableStarts(t *testing.T) {
	tests := []struct {
		name             string
		initialCompleted int
		doneBefore       int
		totalCount       int
		affordableStarts int64
		wantCompleted    int
		wantCancel       bool
		wantStarts       int64
	}{
		{
			name:             "enough cost continue running",
			initialCompleted: 5,
			doneBefore:       10,
			totalCount:       100,
			affordableStarts: 5,
			wantCompleted:    5,
			wantCancel:       false,
			wantStarts:       5,
		},
		{
			name:             "cost limited partial then cancel",
			initialCompleted: 1000,
			doneBefore:       0,
			totalCount:       1000,
			affordableStarts: 299,
			wantCompleted:    300,
			wantCancel:       true,
			wantStarts:       299,
		},
		{
			name:             "cannot start next keeps only current and cancel",
			initialCompleted: 8,
			doneBefore:       10,
			totalCount:       100,
			affordableStarts: 0,
			wantCompleted:    1,
			wantCancel:       true,
			wantStarts:       0,
		},
		{
			name:             "limited but naturally finishes without cancel",
			initialCompleted: 10,
			doneBefore:       8,
			totalCount:       10,
			affordableStarts: 1,
			wantCompleted:    2,
			wantCancel:       false,
			wantStarts:       1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCompleted, gotCancel, gotStarts := adjustCompletedForAffordableStarts(tc.initialCompleted, tc.doneBefore, tc.totalCount, tc.affordableStarts)
			if gotCompleted != tc.wantCompleted || gotCancel != tc.wantCancel || gotStarts != tc.wantStarts {
				t.Fatalf("adjustCompletedForAffordableStarts() = (%d, %v, %d), want (%d, %v, %d)",
					gotCompleted, gotCancel, gotStarts, tc.wantCompleted, tc.wantCancel, tc.wantStarts)
			}
		})
	}
}

func TestOfflineCarryContinuesNextActionAfterCostLimitedCancelWithEfficiency(t *testing.T) {
	startedAt := time.Unix(0, 0)
	targetAt := startedAt.Add(300 * time.Second)
	timePtr := func(value time.Time) *time.Time { return &value }

	chopTiming := resolveQueueRunTiming(QueueRuntimeItem{
		TotalCount: 100,
		DoneCount:  0,
		RunStartAt: timePtr(startedAt),
		RunEndsAt:  timePtr(startedAt.Add(time.Second)),
	}, targetAt, time.Second)
	chopCompleted, chopCompletedAt := completedRunsForQueueTiming(chopTiming, targetAt, 100)
	if chopCompleted != 100 || !chopCompletedAt.Equal(startedAt.Add(199*time.Second)) {
		t.Fatalf("chop completed=%d at=%s, want 100 at 199s", chopCompleted, chopCompletedAt.Sub(startedAt))
	}

	plankStartAt := nextQueueCarryStartAt(targetAt, chopCompletedAt, true, true)
	plankTiming := resolveQueueRunTiming(QueueRuntimeItem{
		TotalCount: 100,
		DoneCount:  0,
		RunStartAt: timePtr(plankStartAt),
		RunEndsAt:  timePtr(plankStartAt.Add(time.Second)),
	}, targetAt, time.Second)
	plankPotential, _ := completedRunsForQueueTiming(plankTiming, targetAt, 100)
	if plankPotential != 51 {
		t.Fatalf("plank potential=%d, want 51 before inventory limit", plankPotential)
	}
	plankCompleted, cancelPlank, startsToCharge := adjustCompletedForAffordableStarts(plankPotential, 0, 100, 29)
	if plankCompleted != 30 || !cancelPlank || startsToCharge != 29 {
		t.Fatalf("plank adjusted=(completed:%d cancel:%v starts:%d), want 30/true/29", plankCompleted, cancelPlank, startsToCharge)
	}
	plankCompletedAt := completedAtForQueueTiming(plankTiming, plankCompleted)
	if !plankCompletedAt.Equal(startedAt.Add(258 * time.Second)) {
		t.Fatalf("plank completed at %s, want 258s", plankCompletedAt.Sub(startedAt))
	}

	miningStartAt := nextQueueCarryStartAt(targetAt, plankCompletedAt, true, true)
	miningDurationWithLoadout := computeEffectiveActionDurationWithFloor(5*time.Second, 0.25, 0.05)
	if miningDurationWithLoadout != 4*time.Second {
		t.Fatalf("mining duration with loadout=%s, want 4s", miningDurationWithLoadout)
	}
	miningTiming := resolveQueueRunTiming(QueueRuntimeItem{
		TotalCount: 100,
		DoneCount:  0,
		RunStartAt: timePtr(miningStartAt),
		RunEndsAt:  timePtr(miningStartAt.Add(miningDurationWithLoadout)),
	}, targetAt, miningDurationWithLoadout)
	miningCompleted, _ := completedRunsForQueueTiming(miningTiming, targetAt, 100)
	if miningCompleted != 10 {
		t.Fatalf("mining completed=%d, want 10 from carried remaining time and loadout efficiency", miningCompleted)
	}

	miningWithoutLoadoutTiming := resolveQueueRunTiming(QueueRuntimeItem{
		TotalCount: 100,
		DoneCount:  0,
		RunStartAt: timePtr(miningStartAt),
		RunEndsAt:  timePtr(miningStartAt.Add(5 * time.Second)),
	}, targetAt, 5*time.Second)
	miningWithoutLoadout, _ := completedRunsForQueueTiming(miningWithoutLoadoutTiming, targetAt, 100)
	if miningWithoutLoadout != 8 {
		t.Fatalf("mining without loadout=%d, want 8 baseline", miningWithoutLoadout)
	}
}

func TestOfflineCarryUsesActualCompletionForSpecialExecutorPartialSettle(t *testing.T) {
	startedAt := time.Unix(0, 0)
	targetAt := startedAt.Add(199 * time.Second)
	timePtr := func(value time.Time) *time.Time { return &value }
	timing := resolveQueueRunTiming(QueueRuntimeItem{
		TotalCount: 100,
		DoneCount:  0,
		RunStartAt: timePtr(startedAt.Add(100 * time.Second)),
		RunEndsAt:  timePtr(startedAt.Add(101 * time.Second)),
	}, targetAt, time.Second)

	theoretical, theoreticalCompletedAt := completedRunsForQueueTiming(timing, targetAt, 100)
	if theoretical != 50 || !theoreticalCompletedAt.Equal(targetAt) {
		t.Fatalf("theoretical completed=%d at=%s, want 50 at target", theoretical, theoreticalCompletedAt.Sub(startedAt))
	}

	tests := []struct {
		name        string
		actionID    string
		actual      int
		wantCarryAt time.Duration
	}{
		{name: "enhance material stop after attempts", actionID: "equipment_enhance", actual: 30, wantCarryAt: 159 * time.Second},
		{name: "alchemy convert affordable cap", actionID: "alchemy.convert", actual: 30, wantCarryAt: 159 * time.Second},
		{name: "alchemy transmute affordable cap", actionID: "alchemy.transmute", actual: 30, wantCarryAt: 159 * time.Second},
		{name: "alchemy bench carrier or material cap", actionID: "alchemy.bench", actual: 30, wantCarryAt: 159 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			executor := (&Store{}).actionHandlerForType(tc.actionID)
			if _, ok := queueDurationRefresherForStatus(executor, "running"); ok {
				t.Fatalf("%s must keep executor-owned timing so actual CompletedAt can carry to the next action", tc.actionID)
			}
			actualCompletedAt := completedAtForQueueTiming(timing, tc.actual)
			if !actualCompletedAt.Equal(startedAt.Add(tc.wantCarryAt)) {
				t.Fatalf("actual completed at %s, want %s", actualCompletedAt.Sub(startedAt), tc.wantCarryAt)
			}
			carryStartAt := nextQueueCarryStartAt(targetAt, actualCompletedAt, true, true)
			if !carryStartAt.Equal(actualCompletedAt) {
				t.Fatalf("carry start at %s, want actual completed at %s", carryStartAt.Sub(startedAt), actualCompletedAt.Sub(startedAt))
			}

			nextTiming := resolveQueueRunTiming(QueueRuntimeItem{
				TotalCount: 100,
				DoneCount:  0,
				RunStartAt: timePtr(carryStartAt),
				RunEndsAt:  timePtr(carryStartAt.Add(time.Second)),
			}, targetAt, time.Second)
			nextCompleted, _ := completedRunsForQueueTiming(nextTiming, targetAt, 100)
			if nextCompleted != 20 {
				t.Fatalf("next action completed=%d, want 20 from actual carry time and minimum duration", nextCompleted)
			}
		})
	}
}

func TestOfflineCapWindowShiftOnlyMovesPostTargetGap(t *testing.T) {
	targetAt := time.Unix(1_000, 0)
	realNow := targetAt.Add(24 * time.Hour)
	if got := offlineCapWindowShift(targetAt, realNow); got != 24*time.Hour {
		t.Fatalf("shift=%s, want 24h", got)
	}
	if got := offlineCapWindowShift(realNow, targetAt); got != 0 {
		t.Fatalf("negative shift=%s, want 0", got)
	}
	if got := offlineCapWindowShift(time.Time{}, realNow); got != 0 {
		t.Fatalf("zero settleAt shift=%s, want 0", got)
	}
}

func TestBuildActionCostEntriesSortedAndScaled(t *testing.T) {
	entries, err := buildActionCostEntries(map[string]int64{
		"wool":                       24,
		"soft_cotton":                12,
		"coarse_linen_gatherer_head": 1,
		"invalid":                    0,
	}, 2)
	if err != nil {
		t.Fatalf("buildActionCostEntries returned err: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries len=%d, want 3", len(entries))
	}
	if entries[0].ItemID != "coarse_linen_gatherer_head" || entries[0].Quantity != 2 {
		t.Fatalf("entry0=%+v, want coarse_linen_gatherer_head x2", entries[0])
	}
	if entries[1].ItemID != "soft_cotton" || entries[1].Quantity != 24 {
		t.Fatalf("entry1=%+v, want soft_cotton x24", entries[1])
	}
	if entries[2].ItemID != "wool" || entries[2].Quantity != 48 {
		t.Fatalf("entry2=%+v, want wool x48", entries[2])
	}
}

func TestAggregateAchievementEventsForOfflineBatch(t *testing.T) {
	events := []AchievementEvent{
		{
			Kind:      AchievementEventCompleteAction,
			ActionID:  "chop_wood",
			ActorType: ActionActorTypePlayer,
			SubjectID: "chop_wood",
			Count:     2,
			SourceKey: "action_queue:1:2",
		},
		{
			Kind:      AchievementEventCompleteAction,
			ActionID:  "chop_wood",
			ActorType: ActionActorTypePlayer,
			SubjectID: "chop_wood",
			Count:     3,
			SourceKey: "action_queue:1:5",
		},
		{
			Kind:         AchievementEventEnhanceCount,
			ItemID:       "bronze_sword",
			SubjectID:    "bronze_sword",
			Count:        1,
			Successful:   true,
			EnhanceLevel: 3,
			SourceKey:    "enhance:9:1",
		},
		{
			Kind:         AchievementEventEnhanceCount,
			ItemID:       "bronze_sword",
			SubjectID:    "bronze_sword",
			Count:        1,
			Successful:   true,
			EnhanceLevel: 3,
			SourceKey:    "enhance:9:2",
		},
		{
			Kind:         AchievementEventEnhanceCount,
			ItemID:       "bronze_sword",
			SubjectID:    "bronze_sword",
			Count:        1,
			Successful:   false,
			EnhanceLevel: 3,
			SourceKey:    "enhance:9:3",
		},
		{
			Kind:      AchievementEventStateReconcile,
			StateKind: "own_item",
			ItemID:    "bronze_sword",
			SubjectID: "bronze_sword",
			SourceKey: "reconcile:1",
		},
		{
			Kind:      AchievementEventStateReconcile,
			StateKind: "own_item",
			ItemID:    "bronze_sword",
			SubjectID: "bronze_sword",
			SourceKey: "reconcile:2",
		},
	}

	aggregated := aggregateAchievementEventsForOfflineBatch("batch-1", events)
	if len(aggregated) != 4 {
		t.Fatalf("len(aggregated)=%d, want 4", len(aggregated))
	}

	byKind := make(map[string][]AchievementEvent)
	for _, event := range aggregated {
		byKind[event.Kind] = append(byKind[event.Kind], event)
		if event.SourceKey == "" {
			t.Fatalf("aggregated event missing source key: %#v", event)
		}
	}

	actionEvents := byKind[AchievementEventCompleteAction]
	if len(actionEvents) != 1 || actionEvents[0].Count != 5 {
		t.Fatalf("complete_action aggregation = %#v, want single count=5", actionEvents)
	}

	enhanceEvents := byKind[AchievementEventEnhanceCount]
	if len(enhanceEvents) != 2 {
		t.Fatalf("enhance_count aggregation len=%d, want 2", len(enhanceEvents))
	}
	var successCount int64
	var failureCount int64
	for _, event := range enhanceEvents {
		if event.Successful {
			successCount = event.Count
		} else {
			failureCount = event.Count
		}
	}
	if successCount != 2 || failureCount != 1 {
		t.Fatalf("enhance_count aggregation = success:%d failure:%d, want 2/1", successCount, failureCount)
	}

	reconcileEvents := byKind[AchievementEventStateReconcile]
	if len(reconcileEvents) != 1 {
		t.Fatalf("state_reconcile dedupe len=%d, want 1", len(reconcileEvents))
	}
}

func TestBuildActionCostEntriesOverflow(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	_, err := buildActionCostEntries(map[string]int64{"wool": maxInt64}, 2)
	if err == nil {
		t.Fatalf("expected overflow error")
	}
}

func TestMaxAffordableStartsByOwned(t *testing.T) {
	entries := []actionCostEntry{
		{ItemID: "soft_cotton", Quantity: 12},
		{ItemID: "wool", Quantity: 24},
	}
	owned := map[string]int64{
		"soft_cotton": 100,
		"wool":        49,
	}
	got := maxAffordableStartsByOwned(entries, owned, 1000)
	if got != 2 {
		t.Fatalf("maxAffordableStartsByOwned()=%d, want 2", got)
	}
}

func TestGenericClassifyStartErrorStatusRequirement(t *testing.T) {
	executor := newGenericExecutor(&Store{})
	cancelQueue, reason := executor.ClassifyStartError(ErrStatusRequireNotMet)
	if !cancelQueue {
		t.Fatalf("expected status requirement error to cancel queue")
	}
	if reason != ActionQueueCancelReasonStatusRequirement {
		t.Fatalf("reason=%q, want %q", reason, ActionQueueCancelReasonStatusRequirement)
	}
}

func TestGenericClassifyStartErrorBuildingRequirement(t *testing.T) {
	executor := newGenericExecutor(&Store{})
	cancelQueue, reason := executor.ClassifyStartError(ErrBuildingRequireNotMet)
	if !cancelQueue {
		t.Fatalf("expected building requirement error to cancel queue")
	}
	if reason != ActionQueueCancelReasonBuildingRequirement {
		t.Fatalf("reason=%q, want %q", reason, ActionQueueCancelReasonBuildingRequirement)
	}
}

func TestAccumulateActionActorSummary(t *testing.T) {
	summaryByLane := map[string]*ActionActorSummary{}
	accumulateActionActorSummary(summaryByLane, ActionActorTypeKitty, 12, SettleResult{
		Completed: 2,
		ItemRewards: map[string]int64{
			"fish": 3,
		},
		ItemCosts: map[string]int64{
			"bait": 1,
		},
		AttrExp: map[string]float64{
			"crafting": 0,
		},
	})
	accumulateActionActorSummary(summaryByLane, ActionActorTypeKitty, 12, SettleResult{
		Completed: 1,
		ItemRewards: map[string]int64{
			"fish": 2,
		},
		ItemCosts: map[string]int64{
			"bait": 2,
		},
		AttrExp: map[string]float64{
			"crafting": 1.5,
		},
	})
	row := summaryByLane["kitty:12"]
	if row == nil {
		t.Fatalf("missing kitty summary")
	}
	if row.CompletedSteps != 3 {
		t.Fatalf("completed=%d, want 3", row.CompletedSteps)
	}
	if row.ItemRewards["fish"] != 5 {
		t.Fatalf("fish reward=%d, want 5", row.ItemRewards["fish"])
	}
	if row.ItemCosts["bait"] != 3 {
		t.Fatalf("bait cost=%d, want 3", row.ItemCosts["bait"])
	}
	if row.AttrExp["crafting"] != 1.5 {
		t.Fatalf("crafting exp=%v, want 1.5", row.AttrExp["crafting"])
	}
}

func TestBuildActionActorSummariesSort(t *testing.T) {
	rows := buildActionActorSummaries(map[string]*ActionActorSummary{
		"kitty:3": {
			ActorType:      ActionActorTypeKitty,
			ActorID:        3,
			CompletedSteps: 1,
		},
		"player:9": {
			ActorType:      ActionActorTypePlayer,
			ActorID:        9,
			CompletedSteps: 2,
		},
		"kitty:1": {
			ActorType:      ActionActorTypeKitty,
			ActorID:        1,
			CompletedSteps: 3,
		},
	})
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	if rows[0].ActorType != ActionActorTypePlayer || rows[0].ActorID != 0 {
		t.Fatalf("row0=(%s,%d), want (player,0)", rows[0].ActorType, rows[0].ActorID)
	}
	if rows[1].ActorType != ActionActorTypeKitty || rows[1].ActorID != 1 {
		t.Fatalf("row1=(%s,%d), want (kitty,1)", rows[1].ActorType, rows[1].ActorID)
	}
	if rows[2].ActorType != ActionActorTypeKitty || rows[2].ActorID != 3 {
		t.Fatalf("row2=(%s,%d), want (kitty,3)", rows[2].ActorType, rows[2].ActorID)
	}
}

func TestMarkActionActorEquipmentChangedMergesWithSettlementSummary(t *testing.T) {
	summaryByLane := map[string]*ActionActorSummary{}
	accumulateActionActorSummary(summaryByLane, ActionActorTypeKitty, 12, SettleResult{
		Completed: 1,
		AttrExp:   map[string]float64{"fishing": 0.5},
	})
	markActionActorEquipmentChanged(summaryByLane, ActionActorTypeKitty, 12)
	markActionActorEquipmentChanged(summaryByLane, ActionActorTypeKitty, 9)

	rows := buildActionActorSummaries(summaryByLane)
	if len(rows) != 2 {
		t.Fatalf("rows=%d, want 2", len(rows))
	}
	if rows[0].ActorID != 9 || !rows[0].EquipmentChanged {
		t.Fatalf("row0=%+v, want kitty 9 equipment changed", rows[0])
	}
	if rows[1].ActorID != 12 || !rows[1].EquipmentChanged || rows[1].AttrExp["fishing"] != 0.5 {
		t.Fatalf("row1=%+v, want merged equipment and fishing exp", rows[1])
	}
}
