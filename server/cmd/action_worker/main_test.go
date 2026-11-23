package main

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/store"
)

func TestLoadWorkerConfigKeepsSequentialDefault(t *testing.T) {
	t.Setenv("ACTION_WORKER_CONCURRENCY", "")

	if got := loadWorkerConfig().concurrency; got != 1 {
		t.Fatalf("default concurrency=%d, want 1", got)
	}

	t.Setenv("ACTION_WORKER_CONCURRENCY", "99")
	if got := loadWorkerConfig().concurrency; got != 4 {
		t.Fatalf("capped concurrency=%d, want 4", got)
	}
}

func TestActionWorkerPresenceBackendErrorsDoNotStartOfflineWork(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rd.Close() })
	w := actionWorker{redis: rd}
	ctx := context.Background()
	key := actionruntime.PresenceSessionConnKey(41, 2)
	if active, err := w.activePresenceSession(ctx, 41, 2); err != nil || active {
		t.Fatal(active, err)
	}
	mr.Set(key, "1")
	if active, err := w.activePresenceSession(ctx, 41, 2); err != nil || !active {
		t.Fatal(active, err)
	}
	mr.Del(key)
	if _, err := rd.LPush(ctx, key, "wrong-type").Result(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.activePresenceSession(ctx, 41, 2); err == nil {
		t.Fatal("presence failure was interpreted as disconnection")
	}
}

func TestActionWorkerPollDelayWakesAtKnownDeadline(t *testing.T) {
	now := time.UnixMilli(10_000)
	regularDelay := 200 * time.Millisecond

	if got := actionWorkerPollDelay(now, now.Add(37*time.Millisecond), regularDelay, true); got != 37*time.Millisecond {
		t.Fatalf("near deadline delay=%s, want 37ms", got)
	}
	if got := actionWorkerPollDelay(now, now.Add(time.Second), regularDelay, true); got != regularDelay {
		t.Fatalf("far deadline delay=%s, want %s", got, regularDelay)
	}
	if got := actionWorkerPollDelay(now, time.Time{}, regularDelay, true); got != regularDelay {
		t.Fatalf("missing deadline delay=%s, want %s", got, regularDelay)
	}
	if got := actionWorkerPollDelay(now, now.Add(37*time.Millisecond), regularDelay, false); got != regularDelay {
		t.Fatalf("throttled deadline delay=%s, want %s", got, regularDelay)
	}
}

func TestActionWorkerDueScoreWakeAtGuardsSubMillisecondTimestamp(t *testing.T) {
	if got := actionWorkerDueScoreWakeAt(10_000); !got.Equal(time.UnixMilli(10_001)) {
		t.Fatalf("wake_at=%s, want %s", got, time.UnixMilli(10_001))
	}
}

func TestActionWorkerPollDelayOnlyRetriesOverdueWorkAfterEmptyPoll(t *testing.T) {
	now := time.UnixMilli(10_000)
	regularDelay := 200 * time.Millisecond
	overdue := now.Add(-time.Millisecond)

	if got := actionWorkerPollDelay(now, overdue, regularDelay, false); got != regularDelay {
		t.Fatalf("throttled overdue delay=%s, want %s", got, regularDelay)
	}
	if got := actionWorkerPollDelay(now, overdue, regularDelay, true); got != 0 {
		t.Fatalf("empty-poll overdue delay=%s, want immediate retry", got)
	}
	if got := actionWorkerPollDelay(now, now.Add(time.Second), 0, false); got != 0 {
		t.Fatalf("elapsed regular poll delay=%s, want immediate", got)
	}
}

func TestProcessActionUsersUsesBoundedConcurrency(t *testing.T) {
	userIDs := make([]int64, 12)
	for i := range userIDs {
		userIDs[i] = int64(i + 1)
	}
	var active atomic.Int64
	var maxActive atomic.Int64
	results := processActionUsers(context.Background(), userIDs, 3, func(_ context.Context, _ int64) (bool, error) {
		current := active.Add(1)
		for {
			seen := maxActive.Load()
			if current <= seen || maxActive.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return true, nil
	})

	if len(results) != len(userIDs) {
		t.Fatalf("results=%d, want %d", len(results), len(userIDs))
	}
	if got := maxActive.Load(); got < 2 || got > 3 {
		t.Fatalf("max concurrency=%d, want 2..3", got)
	}
	for _, result := range results {
		if result.err != nil || !result.advanced {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
}

func TestRecordUserFailureKeepsDeadUserScheduledForProbe(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rd.Close() })

	worker := &actionWorker{
		redis: rd,
		settings: workerConfig{
			retryBaseDelay:   time.Second,
			retryMaxDelay:    30 * time.Second,
			failureTTL:       time.Hour,
			deadAfterFailure: 2,
			deadRetryDelay:   5 * time.Minute,
			deadRetryMax:     time.Hour,
		},
	}
	ctx := context.Background()
	userID := int64(42)
	worker.recordUserFailure(ctx, userID, errors.New("temporary database timeout"))
	beforeSecondFailure := time.Now()
	worker.recordUserFailure(ctx, userID, errors.New("temporary database timeout"))

	member := strconv.FormatInt(userID, 10)
	dueScore, err := rd.ZScore(ctx, actionruntime.QueueDueKey(), member).Result()
	if err != nil {
		t.Fatalf("read due score: %v", err)
	}
	minRetryAt := beforeSecondFailure.Add(worker.settings.deadRetryDelay - time.Second).UnixMilli()
	if int64(dueScore) < minRetryAt {
		t.Fatalf("due score=%d, want >= %d", int64(dueScore), minRetryAt)
	}
	if _, err := rd.ZScore(ctx, actionruntime.DeadUsersKey(), member).Result(); err != nil {
		t.Fatalf("dead user not recorded: %v", err)
	}
	nextRetryRaw, err := rd.HGet(ctx, actionruntime.DeadInfoKey(userID), "next_retry_ms").Result()
	if err != nil || nextRetryRaw == "" {
		t.Fatalf("dead retry metadata missing value=%q err=%v", nextRetryRaw, err)
	}

	worker.clearUserFailure(ctx, userID)
	if got, err := rd.ZScore(ctx, actionruntime.DeadUsersKey(), member).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("dead user was not cleared score=%v err=%v", got, err)
	}
}

func TestDeadRetryDelayGrowsAndCaps(t *testing.T) {
	worker := &actionWorker{settings: workerConfig{
		deadAfterFailure: 10,
		deadRetryDelay:   5 * time.Minute,
		deadRetryMax:     time.Hour,
	}}

	tests := []struct {
		failures int64
		want     time.Duration
	}{
		{failures: 10, want: 5 * time.Minute},
		{failures: 11, want: 10 * time.Minute},
		{failures: 13, want: 40 * time.Minute},
		{failures: 14, want: time.Hour},
		{failures: 50, want: time.Hour},
	}
	for _, test := range tests {
		if got := worker.deadRetryDelayFor(test.failures); got != test.want {
			t.Fatalf("failures=%d delay=%s, want %s", test.failures, got, test.want)
		}
	}
}

func TestRecoverDeadUsersUsesRedisOnlyAndPreservesRetryBackoff(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rd.Close() })
	ctx := context.Background()

	existingRetry := time.Now().Add(5 * time.Minute)
	pipe := rd.TxPipeline()
	pipe.ZAdd(ctx, actionruntime.QueueDueKey(), redis.Z{Score: float64(existingRetry.UnixMilli()), Member: "7"})
	pipe.ZAdd(ctx, actionruntime.DeadUsersKey(),
		redis.Z{Score: 1, Member: "7"},
		redis.Z{Score: 2, Member: "8"},
		redis.Z{Score: 3, Member: "9"},
		redis.Z{Score: 4, Member: "invalid"},
	)
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("seed dead users: %v", err)
	}

	worker := &actionWorker{redis: rd, settings: workerConfig{
		deadRetryDelay: 5 * time.Minute,
		deadRetryMax:   time.Hour,
	}}
	recovered, err := worker.recoverDeadUsers(ctx)
	if err != nil {
		t.Fatalf("recover dead users: %v", err)
	}
	if recovered != 2 {
		t.Fatalf("recovered=%d, want 2", recovered)
	}
	existingScore, err := rd.ZScore(ctx, actionruntime.QueueDueKey(), "7").Result()
	if err != nil {
		t.Fatalf("read existing score: %v", err)
	}
	if int64(existingScore) != existingRetry.UnixMilli() {
		t.Fatalf("existing retry overwritten score=%d, want %d", int64(existingScore), existingRetry.UnixMilli())
	}
	recoveredFirst, err := rd.ZScore(ctx, actionruntime.QueueDueKey(), "8").Result()
	if err != nil {
		t.Fatalf("legacy dead user was not recovered: %v", err)
	}
	recoveredSecond, err := rd.ZScore(ctx, actionruntime.QueueDueKey(), "9").Result()
	if err != nil {
		t.Fatalf("second legacy dead user was not recovered: %v", err)
	}
	if recoveredSecond <= recoveredFirst {
		t.Fatalf("legacy recovery was not staggered first=%f second=%f", recoveredFirst, recoveredSecond)
	}
	if _, err := rd.ZScore(ctx, actionruntime.QueueDueKey(), "invalid").Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("invalid dead user should be skipped err=%v", err)
	}
}

func TestActionScheduleHasKitty(t *testing.T) {
	playerOnly := []store.ActionQueueScheduleItem{{ActorType: store.ActionActorTypePlayer}}
	if actionScheduleHasKitty(playerOnly) {
		t.Fatal("player-only schedule should not read kitty combat projection")
	}
	withKitty := append(playerOnly, store.ActionQueueScheduleItem{ActorType: store.ActionActorTypeKitty, ActorID: 9})
	if !actionScheduleHasKitty(withKitty) {
		t.Fatal("kitty schedule should read kitty combat projection")
	}
}
