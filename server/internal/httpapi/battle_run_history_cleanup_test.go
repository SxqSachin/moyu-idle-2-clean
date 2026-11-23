package httpapi

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBattleRunHistoryCleanupQueueDeduplicatesAndBounds(t *testing.T) {
	queue := newBattleRunHistoryCleanupQueue(3)
	for _, roomID := range []int64{3, 1, 2, 1} {
		if !queue.enqueue(roomID) {
			t.Fatalf("enqueue room %d failed", roomID)
		}
	}
	if queue.enqueue(4) {
		t.Fatal("queue accepted a fourth distinct room beyond its capacity")
	}

	first, dropped := queue.take(2)
	if len(first) != 2 || dropped != 1 {
		t.Fatalf("first batch=%v dropped=%d, want two rooms and one dropped enqueue", first, dropped)
	}
	second, dropped := queue.take(2)
	if len(second) != 1 || dropped != 0 {
		t.Fatalf("second batch=%v dropped=%d, want remaining room", second, dropped)
	}
	all := append(append([]int64(nil), first...), second...)
	slices.Sort(all)
	if !slices.Equal(all, []int64{1, 2, 3}) {
		t.Fatalf("deduplicated rooms=%v, want [1 2 3]", all)
	}

	queue.requeue([]int64{4, 4, 2})
	requeued, dropped := queue.take(10)
	if dropped != 0 || !slices.Equal(requeued, []int64{2, 4}) {
		t.Fatalf("requeued rooms=%v dropped=%d, want [2 4]", requeued, dropped)
	}
}

func TestBattleRunHistoryCleanupQueueConcurrentEnqueue(t *testing.T) {
	queue := newBattleRunHistoryCleanupQueue(128)
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for roomID := int64(1); roomID <= 64; roomID++ {
				queue.enqueue(roomID)
			}
		}()
	}
	workers.Wait()
	roomIDs, dropped := queue.take(128)
	if dropped != 0 || len(roomIDs) != 64 {
		t.Fatalf("concurrent queue rooms=%d dropped=%d, want 64 unique rooms", len(roomIDs), dropped)
	}
	for index, roomID := range roomIDs {
		if roomID != int64(index+1) {
			t.Fatalf("room ids=%v, want sorted ids 1 through 64", roomIDs)
		}
	}
}

func TestScheduleBattleRunHistoryCleanupStopsWithWorker(t *testing.T) {
	server := &Server{
		battleLoopEnabled:       true,
		battleStopCh:            make(chan struct{}),
		battleRunHistoryCleanup: newBattleRunHistoryCleanupQueue(2),
	}
	if !server.scheduleBattleRunHistoryCleanup(7) {
		t.Fatal("active battle worker rejected cleanup")
	}
	close(server.battleStopCh)
	if server.scheduleBattleRunHistoryCleanup(8) {
		t.Fatal("stopped battle worker accepted cleanup")
	}
	roomIDs, _ := server.battleRunHistoryCleanup.take(10)
	if !slices.Equal(roomIDs, []int64{7}) {
		t.Fatalf("queued rooms=%v, want only the pre-shutdown room", roomIDs)
	}
}

func TestBattleRunHistoryCleanupLoopUsesBattleWorkerLifecycle(t *testing.T) {
	server := &Server{
		battleLoopEnabled: true,
		battleStopCh:      make(chan struct{}),
	}
	server.startBattleRunHistoryCleanupLoop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.ShutdownBattleWorker(ctx); err != nil {
		t.Fatalf("shutdown cleanup worker: %v", err)
	}
}

func TestBattleRunHistoryCleanupIsDungeonOnlyAndNotPerFinalizeGoroutine(t *testing.T) {
	cleanupRaw, err := os.ReadFile("battle_run_history_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := strings.ToLower(string(cleanupRaw))
	for _, forbidden := range []string{
		"arena_challenges",
		"arena_challenge_payloads",
		"combat_trial_runs",
		"combat_trial_run_payloads",
		"prunearenachallengehistory",
		"prunecombattrialruns",
	} {
		if strings.Contains(cleanup, forbidden) {
			t.Fatalf("ordinary battle cleanup crossed into another battle domain: %q", forbidden)
		}
	}

	finalizeRaw, err := os.ReadFile("battle_runtime_finalize.go")
	if err != nil {
		t.Fatal(err)
	}
	finalize := string(finalizeRaw)
	if !strings.Contains(finalize, "s.scheduleBattleRunHistoryCleanup(detail.Room.ID)") {
		t.Fatal("battle finalize must enqueue bounded history cleanup")
	}
	if strings.Contains(finalize, "compactBattleRunHistoryAsync") || strings.Contains(finalize, "launchBattleTaskOrRun(func() { s.compactBattleRunHistory") {
		t.Fatal("battle finalize must not launch one cleanup goroutine per run")
	}

	loopRaw, err := os.ReadFile("battle_loop.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loopRaw), "s.startBattleRunHistoryCleanupLoop()") {
		t.Fatal("battle worker must own the cleanup loop lifecycle")
	}
}
