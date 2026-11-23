package httpapi

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestBattlePlaybackWorkerReservationFencesStaleMutationAndRelease(t *testing.T) {
	server := &Server{}
	detail := store.BattleRoomDetail{Room: store.BattleRoom{ID: 41, SchemaVersion: BattleRoomSchemaV2}}
	stale, ok := server.newBattlePlaybackWorkerReservation(detail.Room.ID, 73, "worker-a")
	if !ok {
		t.Fatal("create stale reservation")
	}
	current, ok := server.newBattlePlaybackWorkerReservation(detail.Room.ID, 73, "worker-a")
	if !ok {
		t.Fatal("create current reservation")
	}
	if stale.OwnerToken == current.OwnerToken || stale.Generation >= current.Generation {
		t.Fatalf("reservations are not uniquely ordered: stale=%+v current=%+v", stale, current)
	}
	if !server.activateBattlePlaybackWorkerForRun(detail, stale, time.Now()) {
		t.Fatal("activate stale reservation")
	}
	if !server.activateBattlePlaybackWorkerForRun(detail, current, time.Now()) {
		t.Fatal("activate current reservation")
	}
	if !server.updateBattlePlaybackWorkerState(current, func(state *battlePlaybackWorkerState) {
		state.LastPushedSeq = 77
	}) {
		t.Fatal("update current reservation")
	}

	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			server.updateBattlePlaybackWorkerState(stale, func(state *battlePlaybackWorkerState) {
				state.LastPushedSeq = -1
			})
			server.releaseBattlePlaybackWorker(stale)
		}()
	}
	workers.Wait()

	state, exists := server.battlePlaybackWorkerState(current)
	if !exists {
		t.Fatal("stale release removed the current reservation")
	}
	if state.LastPushedSeq != 77 {
		t.Fatalf("stale update changed current state: %+v", state)
	}
	if server.activateBattlePlaybackWorkerForRun(detail, stale, time.Now()) {
		t.Fatal("older reservation replaced the current reservation")
	}
	if !server.releaseBattlePlaybackWorker(current) {
		t.Fatal("current reservation could not release itself")
	}
}

func TestBattlePlaybackLeaseRenewalFailsClosedWhenOwnerTokenIsLost(t *testing.T) {
	if err := battlePlaybackLeaseRenewalError(true); err != nil {
		t.Fatalf("renewed lease returned error: %v", err)
	}
	if err := battlePlaybackLeaseRenewalError(false); !errors.Is(err, errBattlePlaybackLeaseLost) {
		t.Fatalf("lost lease error=%v, want %v", err, errBattlePlaybackLeaseLost)
	}
}

func TestBattlePlaybackWorkerWakeAtRenewsBeforeLongEventSleep(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	leaseUntil := now.Add(battlePlaybackLeaseTTL)
	longNextTick := now.Add(battlePlaybackMaxFutureSleep)
	wantRenewAt := leaseUntil.Add(-battlePlaybackLeaseRenewThreshold)
	if got := battlePlaybackWorkerWakeAt(longNextTick, leaseUntil, time.Time{}); !got.Equal(wantRenewAt) {
		t.Fatalf("long sleep wake=%s, want lease renewal at %s", got, wantRenewAt)
	}
	if !battlePlaybackLeaseNeedsRenewal(now, longNextTick, leaseUntil) {
		t.Fatal("lease that cannot cover the next tick must be renewed before worker launch")
	}

	shortNextTick := now.Add(time.Second)
	if got := battlePlaybackWorkerWakeAt(shortNextTick, leaseUntil, time.Time{}); !got.Equal(shortNextTick) {
		t.Fatalf("short sleep wake=%s, want next tick at %s", got, shortNextTick)
	}
	if battlePlaybackLeaseNeedsRenewal(now, shortNextTick, leaseUntil) {
		t.Fatal("lease with enough margin should not be renewed early")
	}

	if got := battlePlaybackWorkerWakeAt(longNextTick, time.Time{}, time.Time{}); !got.Equal(longNextTick) {
		t.Fatalf("missing lease wake=%s, want next tick at %s", got, longNextTick)
	}
}

func TestBattlePlaybackWorkerWakeAtHonorsRenewRetryBackoff(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	expiredLease := now.Add(-time.Second)
	nextTick := now.Add(5 * time.Second)
	retryNotBefore := now.Add(battlePlaybackLeaseRenewRetryDelay)
	if got := battlePlaybackWorkerWakeAt(nextTick, expiredLease, retryNotBefore); !got.Equal(retryNotBefore) {
		t.Fatalf("fast renew failure wake=%s, want retry backoff at %s", got, retryNotBefore)
	}
}

func TestBattlePlaybackLeaseCoverageIncludesMaximumTickWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	if battlePlaybackLeaseCoversTick(nil, now, now.Add(battlePlaybackTickOperationTimeout)) {
		t.Fatal("lease ending at the tick timeout has no safety margin")
	}
	if !battlePlaybackLeaseCoversTick(nil, now, now.Add(battlePlaybackTickOperationTimeout+time.Millisecond)) {
		t.Fatal("lease extending beyond the tick timeout should cover the tick")
	}
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(4*time.Second))
	defer cancel()
	if battlePlaybackLeaseCoversTick(ctx, now, now.Add(4*time.Second)) {
		t.Fatal("lease ending at the operation context deadline must be renewed")
	}
}

func TestPlaybackBattleRunTickRejectsExpiredOperationContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &Server{}
	_, _, err := server.playbackBattleRunTick(
		ctx,
		store.BattleRoomDetail{Room: store.BattleRoom{ID: 41}},
		store.BattleRun{ID: 73},
		time.Now(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expired tick context error=%v, want %v", err, context.Canceled)
	}
}
