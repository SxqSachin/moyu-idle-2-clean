package store

import (
	"testing"
	"time"
)

func TestQueueRunTimingRequiresFullCurrentWindow(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Second)
	ends := start.Add(30 * time.Second)
	item := QueueRuntimeItem{
		TotalCount: 5,
		DoneCount:  0,
		RunStartAt: &start,
		RunEndsAt:  &ends,
	}
	timing := resolveQueueRunTiming(item, now, 30*time.Second)
	completed, completedAt := completedRunsForQueueTiming(timing, now, item.TotalCount-item.DoneCount)
	if completed != 0 {
		t.Fatalf("completed=%d, want 0 before full duration", completed)
	}
	if !completedAt.Equal(start) {
		t.Fatalf("completedAt=%v, want start %v", completedAt, start)
	}

	completed, completedAt = completedRunsForQueueTiming(timing, ends, item.TotalCount-item.DoneCount)
	if completed != 1 {
		t.Fatalf("completed=%d, want 1 after full duration", completed)
	}
	if !completedAt.Equal(ends) {
		t.Fatalf("completedAt=%v, want ends %v", completedAt, ends)
	}
}

func TestQueueRunTimingRecoversStandardDurationFromCurrentWindow(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	start := now.Add(-32 * time.Second)
	ends := start.Add(32 * time.Second)
	item := QueueRuntimeItem{
		TotalCount: 40,
		DoneCount:  1,
		RunStartAt: &start,
		RunEndsAt:  &ends,
	}

	timing := resolveQueueRunTiming(item, now, 0)
	if timing.CurrentDuration != 32*time.Second {
		t.Fatalf("current duration=%v, want 32s", timing.CurrentDuration)
	}
	if timing.NextDuration != 32*time.Second {
		t.Fatalf("next duration=%v, want 32s", timing.NextDuration)
	}
	completed, completedAt := completedRunsForQueueTiming(timing, now, item.TotalCount-item.DoneCount)
	if completed != 1 {
		t.Fatalf("completed=%d, want 1", completed)
	}
	if !completedAt.Equal(ends) {
		t.Fatalf("completedAt=%v, want %v", completedAt, ends)
	}
	if got := completedAt.Add(timing.NextDuration); !got.Equal(ends.Add(32 * time.Second)) {
		t.Fatalf("next run ends at=%v, want %v", got, ends.Add(32*time.Second))
	}
}

func TestOnlineQueueWindowDoesNotCarryTimeDebt(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 10, 0, 0, time.UTC)
	theoreticalCompletedAt := now.Add(-7 * time.Minute)
	result := SettleResult{
		NextRunStartAt: theoreticalCompletedAt,
		NextRunEndsAt:  theoreticalCompletedAt.Add(14 * time.Second),
	}

	nextStart, nextEnds := nextQueueRunWindow(now, result, nil, false, false)
	if !nextStart.Equal(now) {
		t.Fatalf("online next start=%v, want now %v", nextStart, now)
	}
	if !nextEnds.Equal(now.Add(14 * time.Second)) {
		t.Fatalf("online next ends=%v, want %v", nextEnds, now.Add(14*time.Second))
	}
	if got := nextQueueCarryStartAt(now, theoreticalCompletedAt, false, false); !got.Equal(now) {
		t.Fatalf("online carry start=%v, want now %v", got, now)
	}
}

func TestOfflineQueueWindowCarriesTheoreticalTime(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 10, 0, 0, time.UTC)
	theoreticalCompletedAt := now.Add(-7 * time.Minute)
	result := SettleResult{
		NextRunStartAt: theoreticalCompletedAt,
		NextRunEndsAt:  theoreticalCompletedAt.Add(14 * time.Second),
	}

	nextStart, nextEnds := nextQueueRunWindow(now, result, nil, true, true)
	if !nextStart.Equal(theoreticalCompletedAt) {
		t.Fatalf("offline next start=%v, want %v", nextStart, theoreticalCompletedAt)
	}
	if !nextEnds.Equal(theoreticalCompletedAt.Add(14 * time.Second)) {
		t.Fatalf("offline next ends=%v, want %v", nextEnds, theoreticalCompletedAt.Add(14*time.Second))
	}
	if got := nextQueueCarryStartAt(now, theoreticalCompletedAt, true, true); !got.Equal(theoreticalCompletedAt) {
		t.Fatalf("offline carry start=%v, want %v", got, theoreticalCompletedAt)
	}
}

func TestQueueTimingEnforcesMinimumForFutureRuns(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	timing := resolveQueueRunTiming(QueueRuntimeItem{}, now, time.Second)
	if timing.NextDuration != minimumActionQueueRunDuration {
		t.Fatalf("next duration=%v, want %v", timing.NextDuration, minimumActionQueueRunDuration)
	}

	result := SettleResult{
		NextRunStartAt: now.Add(-time.Minute),
		NextRunEndsAt:  now.Add(-time.Minute).Add(time.Second),
	}
	onlineStart, onlineEnds := nextQueueRunWindow(now, result, nil, false, false)
	if !onlineStart.Equal(now) || !onlineEnds.Equal(now.Add(minimumActionQueueRunDuration)) {
		t.Fatalf("online window=(%v,%v), want (%v,%v)", onlineStart, onlineEnds, now, now.Add(minimumActionQueueRunDuration))
	}
	offlineStart, offlineEnds := nextQueueRunWindow(now, result, nil, true, true)
	if !offlineStart.Equal(result.NextRunStartAt) || !offlineEnds.Equal(result.NextRunStartAt.Add(minimumActionQueueRunDuration)) {
		t.Fatalf("offline window=(%v,%v), want (%v,%v)", offlineStart, offlineEnds, result.NextRunStartAt, result.NextRunStartAt.Add(minimumActionQueueRunDuration))
	}
}

func TestQueueTimingPreservesCurrentLegacyWindowAndClampsNextRun(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-time.Second)
	endsAt := now
	timing := resolveQueueRunTiming(QueueRuntimeItem{
		RunStartAt: &startedAt,
		RunEndsAt:  &endsAt,
	}, now, time.Second)
	if timing.CurrentDuration != time.Second {
		t.Fatalf("current duration=%v, want legacy window %v", timing.CurrentDuration, time.Second)
	}
	if timing.NextDuration != minimumActionQueueRunDuration {
		t.Fatalf("next duration=%v, want %v", timing.NextDuration, minimumActionQueueRunDuration)
	}
	completed, completedAt := completedRunsForQueueTiming(timing, now, 10)
	if completed != 1 || !completedAt.Equal(now) {
		t.Fatalf("completed=%d at=%v, want one legacy run at %v", completed, completedAt, now)
	}
}

func TestQueueDurationRefreshCapabilityIsOwnedByExecutor(t *testing.T) {
	store := &Store{}
	tests := []struct {
		actionID    string
		wantPending bool
		wantRunning bool
	}{
		{actionID: "work_mining", wantPending: true, wantRunning: true},
		{actionID: "future_generic_action", wantPending: true, wantRunning: true},
		{actionID: "equipment_enhance", wantPending: true},
		{actionID: "building_upgrade", wantPending: true},
		{actionID: "alchemy.convert", wantPending: true},
		{actionID: "alchemy.bench", wantPending: true},
		{actionID: "alchemy.transmute", wantPending: true},
		{actionID: "alchemy.smelt"},
		{actionID: "alchemy.inherit", wantPending: true},
		{actionID: FishingTripActionID},
	}
	for _, tc := range tests {
		t.Run(tc.actionID, func(t *testing.T) {
			executor := store.actionHandlerForType(tc.actionID)
			_, gotPending := queueDurationRefresherForStatus(executor, "pending")
			_, gotRunning := queueDurationRefresherForStatus(executor, "running")
			if gotPending != tc.wantPending || gotRunning != tc.wantRunning {
				t.Fatalf(
					"duration refresh capability pending=%v running=%v, want pending=%v running=%v",
					gotPending, gotRunning, tc.wantPending, tc.wantRunning,
				)
			}
		})
	}
}

func TestResolveQueueInsertCountKeepsAlchemyCommandCount(t *testing.T) {
	convertCount, err := resolveQueueInsertCount("alchemy.convert", 100, alchemyConvertArgs{
		SourceItemID: "wood",
		GroupID:      "group.life_primary_common",
		SourceCount:  1,
	})
	if err != nil {
		t.Fatalf("resolve convert count: %v", err)
	}
	if convertCount != 100 {
		t.Fatalf("convert insert count=%d, want 100", convertCount)
	}

	transmuteCount, err := resolveQueueInsertCount("alchemy.transmute", 100, alchemyTransmuteArgs{
		ItemID:     "wood",
		TotalCount: 1,
	})
	if err != nil {
		t.Fatalf("resolve transmute count: %v", err)
	}
	if transmuteCount != 100 {
		t.Fatalf("transmute insert count=%d, want 100", transmuteCount)
	}
}

func TestResolveQueueInsertCountRejectsDatabaseOverflow(t *testing.T) {
	if _, err := resolveQueueInsertCount("alchemy.convert", maxActionQueueTotalCount+1, alchemyConvertArgs{
		SourceItemID: "wood",
		GroupID:      "group.life_primary_common",
		SourceCount:  1,
	}); err != ErrInvalidActionCount {
		t.Fatalf("alchemy overflow err=%v, want %v", err, ErrInvalidActionCount)
	}

	if _, err := resolveQueueInsertCount("equipment_enhance", 1, enhanceExecutorArgs{
		EquipmentInstanceID: 42,
		MaxAttempts:         maxActionQueueTotalCount + 1,
	}); err != ErrInvalidActionCount {
		t.Fatalf("enhance override overflow err=%v, want %v", err, ErrInvalidActionCount)
	}
}

func TestResolveQueueInsertCountAllowsRegisteredArgOverride(t *testing.T) {
	got, err := resolveQueueInsertCount("equipment_enhance", 1, enhanceExecutorArgs{
		EquipmentInstanceID: 42,
		MaxAttempts:         7,
	})
	if err != nil {
		t.Fatalf("resolve enhance count: %v", err)
	}
	if got != 7 {
		t.Fatalf("enhance insert count=%d, want 7", got)
	}
}
