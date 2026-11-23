package httpapi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestHandleBattleRunStartedClearsReadyWaitAndQueuesCompile(t *testing.T) {
	t.Parallel()

	runID := int64(42)
	roomID := int64(7)
	now := time.UnixMilli(1_000)
	s := &Server{
		battleCompileQueue: make(chan battleCompileKickoffTask, 1),
		battleCompileRuns:  make(map[int64]struct{}),
		battleNextReadyMs:  map[int64]int64{roomID: now.Add(-120 * time.Millisecond).UnixMilli()},
		battleLastFinishMs: make(map[int64]int64),
	}
	detail := store.BattleRoomDetail{
		Room: store.BattleRoom{
			ID:           roomID,
			CurrentRunID: &runID,
		},
	}

	s.handleBattleRunStarted(detail, 99, now, battleCompileSourcePrepare, battleCompileSourcePrepare, true, context.Background())

	if _, ok := s.battleNextReadyMs[roomID]; ok {
		t.Fatalf("expected ready wait marker to be consumed for room %d", roomID)
	}
	if got := len(s.battleCompileQueue); got != 1 {
		t.Fatalf("compile queue len=%d, want 1", got)
	}
	if _, ok := s.battleCompileRuns[runID]; !ok {
		t.Fatalf("expected run %d to be marked inflight after successful kickoff", runID)
	}
}

func TestHandleBattleRunStartedQueueFullDoesNotLeaveRunStuckInflight(t *testing.T) {
	t.Parallel()

	runID := int64(84)
	roomID := int64(8)
	s := &Server{
		battleCompileSlots: make(chan struct{}, 4),
		battleCompileQueue: make(chan battleCompileKickoffTask, 1),
		battleCompileRuns:  make(map[int64]struct{}),
		battleNextReadyMs:  make(map[int64]int64),
		battleLastFinishMs: make(map[int64]int64),
	}
	s.battleCompileQueue <- battleCompileKickoffTask{RoomID: 1, RunID: 1}
	detail := store.BattleRoomDetail{
		Room: store.BattleRoom{
			ID:           roomID,
			CurrentRunID: &runID,
		},
	}

	s.handleBattleRunStarted(detail, 101, time.Now(), battleCompileSourcePrepare, battleCompileSourcePrepare, false, context.Background())

	if _, ok := s.battleCompileRuns[runID]; ok {
		t.Fatalf("queue-full kickoff must release inflight marker for run %d", runID)
	}
	if got := len(s.battleCompileQueue); got != 1 {
		t.Fatalf("compile queue len=%d, want 1", got)
	}
}

func TestTryKickoffBattleCompileTickRetryDefersWhenPressured(t *testing.T) {
	t.Parallel()

	runID := int64(96)
	roomID := int64(9)
	s := &Server{
		battleCompileSlots: make(chan struct{}, 4),
		battleCompileQueue: make(chan battleCompileKickoffTask, 4),
		battleCompileRuns:  make(map[int64]struct{}),
	}
	s.battleCompileQueue <- battleCompileKickoffTask{RoomID: 1, RunID: 1}
	s.battleCompileQueue <- battleCompileKickoffTask{RoomID: 2, RunID: 2}
	detail := store.BattleRoomDetail{
		Room: store.BattleRoom{
			ID:           roomID,
			CurrentRunID: &runID,
		},
	}

	if s.tryKickoffBattleCompile(detail, "", battleCompileSourceTickRetry, time.Now().UnixMilli()) {
		t.Fatal("tick retry compile kickoff must defer while queue is pressured")
	}
	if _, ok := s.battleCompileRuns[runID]; ok {
		t.Fatalf("deferred recovery kickoff must not mark run %d inflight", runID)
	}
	if got := len(s.battleCompileQueue); got != 2 {
		t.Fatalf("compile queue len=%d, want 2", got)
	}
}

func TestTryKickoffBattleCompileRoutesFirstSnapshotAheadOfRegularBacklog(t *testing.T) {
	t.Parallel()

	runID := int64(97)
	s := &Server{
		battleCompileSlots:  make(chan struct{}, 4),
		battleCompileFirstQ: make(chan battleCompileKickoffTask, 4),
		battleCompileQueue:  make(chan battleCompileKickoffTask, 4),
		battleCompileRuns:   make(map[int64]struct{}),
	}
	s.battleCompileQueue <- battleCompileKickoffTask{RoomID: 1, RunID: 1}
	s.battleCompileQueue <- battleCompileKickoffTask{RoomID: 2, RunID: 2}
	detail := store.BattleRoomDetail{Room: store.BattleRoom{ID: 10, CurrentRunID: &runID}}

	if !s.tryKickoffBattleCompile(detail, "", battleCompileSourceReadyDispatch, time.Now().UnixMilli()) {
		t.Fatal("new run first snapshot must enter its bounded priority queue under regular pressure")
	}
	if got := len(s.battleCompileFirstQ); got != 1 {
		t.Fatalf("first snapshot queue len=%d, want 1", got)
	}
	if got := len(s.battleCompileQueue); got != 2 {
		t.Fatalf("regular queue len=%d, want 2", got)
	}
}

func TestTryKickoffBattleCompileFirstQueueFullReleasesDedup(t *testing.T) {
	t.Parallel()

	runID := int64(98)
	s := &Server{
		battleCompileSlots:  make(chan struct{}, 4),
		battleCompileFirstQ: make(chan battleCompileKickoffTask, 1),
		battleCompileQueue:  make(chan battleCompileKickoffTask, 4),
		battleCompileRuns:   make(map[int64]struct{}),
	}
	s.battleCompileFirstQ <- battleCompileKickoffTask{RoomID: 1, RunID: 1}
	detail := store.BattleRoomDetail{Room: store.BattleRoom{ID: 11, CurrentRunID: &runID}}

	if s.tryKickoffBattleCompile(detail, "", battleCompileSourcePrepare, time.Now().UnixMilli()) {
		t.Fatal("full first snapshot queue must fall back to durable dispatch")
	}
	if _, exists := s.battleCompileRuns[runID]; exists {
		t.Fatal("full first snapshot queue left a stale dedup marker")
	}
}

func TestTakeBattleCompileTaskHonorsPreferenceAndAllowsBorrowing(t *testing.T) {
	t.Parallel()

	s := &Server{
		battleStopCh:        make(chan struct{}),
		battleCompileFirstQ: make(chan battleCompileKickoffTask, 2),
		battleCompileQueue:  make(chan battleCompileKickoffTask, 2),
	}
	s.battleCompileFirstQ <- battleCompileKickoffTask{RunID: 1}
	s.battleCompileQueue <- battleCompileKickoffTask{RunID: 2}
	task, fromFirst, ok := s.takeBattleCompileTask(true)
	if !ok || !fromFirst || task.RunID != 1 {
		t.Fatalf("first-preferred dequeue=(%+v,%t,%t), want first run 1", task, fromFirst, ok)
	}
	task, fromFirst, ok = s.takeBattleCompileTask(true)
	if !ok || fromFirst || task.RunID != 2 {
		t.Fatalf("empty first queue must borrow regular work, got (%+v,%t,%t)", task, fromFirst, ok)
	}

	s.battleCompileFirstQ <- battleCompileKickoffTask{RunID: 3}
	s.battleCompileQueue <- battleCompileKickoffTask{RunID: 4}
	task, fromFirst, ok = s.takeBattleCompileTask(false)
	if !ok || fromFirst || task.RunID != 4 {
		t.Fatalf("regular-preferred dequeue=(%+v,%t,%t), want regular run 4", task, fromFirst, ok)
	}
}

func TestBattleCompileWorkersReserveOneRegularLane(t *testing.T) {
	t.Parallel()

	for workerIndex, wantFirst := range []bool{true, true, true, false} {
		if got := battleCompileWorkerPrefersFirst(workerIndex, 4); got != wantFirst {
			t.Fatalf("worker %d first preference=%t, want %t", workerIndex, got, wantFirst)
		}
	}
	if !battleCompileWorkerPrefersFirst(0, 1) {
		t.Fatal("single compile worker must start with first-snapshot preference before alternating")
	}
}

func TestBattleCompileWorkerKeepsPostDequeueStopFence(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("battle_runtime_engine.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) startBattleCompileKickoffWorkers")
	if start < 0 {
		t.Fatal("startBattleCompileKickoffWorkers missing")
	}
	body := source[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	dequeueAt := strings.Index(body, "s.takeBattleCompileTask")
	if dequeueAt < 0 {
		t.Fatal("compile worker dequeue missing")
	}
	stopFenceAt := dequeueAt + strings.Index(body[dequeueAt:], "case <-s.battleStopCh")
	executeAt := strings.Index(body, "s.executeBattleCompileKickoff")
	if stopFenceAt < dequeueAt || executeAt <= stopFenceAt {
		t.Fatal("compile worker must re-check shutdown after dequeue and before execution")
	}
}

func TestShouldLogBattleCompileTail(t *testing.T) {
	t.Parallel()

	if shouldLogBattleCompileTail(99, 99, 99, 199, 499) {
		t.Fatal("expected sub-threshold compile tail metrics to stay quiet")
	}
	if !shouldLogBattleCompileTail(100, 0, 0, 0, 0) {
		t.Fatal("expected save stage at threshold to log")
	}
	if !shouldLogBattleCompileTail(0, 0, 0, 200, 0) {
		t.Fatal("expected post-compile tail at threshold to log")
	}
	if !shouldLogBattleCompileTail(0, 0, 0, 0, 500) {
		t.Fatal("expected total compile tail at threshold to log")
	}
}

func TestBattleCompileRetryDelayBacksOffAndCaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: time.Second},
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 5, want: 16 * time.Second},
		{attempt: 6, want: battleCompileRetryMaxDelay},
		{attempt: 100, want: battleCompileRetryMaxDelay},
	}
	for _, tc := range tests {
		if got := battleCompileRetryDelay(tc.attempt); got != tc.want {
			t.Fatalf("battleCompileRetryDelay(%d)=%s, want %s", tc.attempt, got, tc.want)
		}
	}
	invalidCarry := fmt.Errorf("wrapped: %w", errBattlePreviousWaveCarryInvalid)
	if got := battleCompileRetryDelayForError(1, invalidCarry); got != 30*time.Second {
		t.Fatalf("first invalid-carry retry delay=%s, want 30s", got)
	}
	if got := battleCompileRetryDelayForError(100, invalidCarry); got != battleCompileInvalidCarryRetryMaxDelay {
		t.Fatalf("invalid-carry retry cap=%s, want %s", got, battleCompileInvalidCarryRetryMaxDelay)
	}
}

func TestBattleCompileRetryHasLocalNotBeforeFallback(t *testing.T) {
	t.Parallel()

	s := &Server{battleCompileRetries: make(map[int64]battleCompileRetryState)}
	if !s.battleCompileRetryReady(91, time.Now()) {
		t.Fatal("new run must be eligible for compilation")
	}
	delay := s.nextBattleCompileRetryDelay(91, nil)
	if delay != time.Second {
		t.Fatalf("first retry delay=%s, want 1s", delay)
	}
	s.battleCompileRetryMu.Lock()
	notBefore := s.battleCompileRetries[91].NotBefore
	s.battleCompileRetryMu.Unlock()
	if s.battleCompileRetryReady(91, notBefore.Add(-time.Millisecond)) {
		t.Fatal("failed compile became locally eligible before its persisted retry update could take effect")
	}
	if !s.battleCompileRetryReady(91, notBefore) {
		t.Fatal("failed compile did not become eligible after its local retry delay")
	}
	s.clearBattleCompileRetry(91)
	if !s.battleCompileRetryReady(91, time.Now()) {
		t.Fatal("successful or stale compile did not clear local retry state")
	}
}

func TestBattleCompileRetryStateIsPrunedAndBounded(t *testing.T) {
	t.Parallel()

	staleAt := time.Now().Add(-battleCompileRetryStateTTL - time.Minute)
	s := &Server{battleCompileRetries: map[int64]battleCompileRetryState{
		1: {Attempts: 9, NotBefore: staleAt},
	}}
	s.nextBattleCompileRetryDelay(2, nil)
	s.battleCompileRetryMu.Lock()
	_, staleExists := s.battleCompileRetries[1]
	s.battleCompileRetryMu.Unlock()
	if staleExists {
		t.Fatal("stale compile retry state was not pruned")
	}

	full := &Server{battleCompileRetries: make(map[int64]battleCompileRetryState, battleCompileRetryStateMaxEntries)}
	for runID := int64(1); runID <= battleCompileRetryStateMaxEntries; runID++ {
		full.battleCompileRetries[runID] = battleCompileRetryState{Attempts: 1, NotBefore: time.Now().Add(time.Duration(runID) * time.Millisecond)}
	}
	full.nextBattleCompileRetryDelay(battleCompileRetryStateMaxEntries+1, nil)
	full.battleCompileRetryMu.Lock()
	size := len(full.battleCompileRetries)
	full.battleCompileRetryMu.Unlock()
	if size > battleCompileRetryStateMaxEntries {
		t.Fatalf("compile retry state grew to %d entries, max %d", size, battleCompileRetryStateMaxEntries)
	}
}
