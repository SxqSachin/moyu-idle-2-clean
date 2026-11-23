package httpapi

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/store"
)

func TestComputeActionQueueNextDueIgnoresBlockedKittyRunningLane(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	queue := []store.ActionQueueItem{
		{
			ID:        1,
			ActorType: store.ActionActorTypeKitty,
			ActorID:   7,
			Status:    "running",
			RunEndsAt: now.Add(1 * time.Minute),
		},
		{
			ID:        2,
			ActorType: store.ActionActorTypePlayer,
			Status:    "running",
			RunEndsAt: now.Add(2 * time.Minute),
		},
	}
	next, ok := computeActionQueueNextDue(queue, map[int64]struct{}{7: {}}, now)
	if !ok {
		t.Fatalf("expected schedulable due")
	}
	if !next.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("next=%v, want %v", next, now.Add(2*time.Minute))
	}
}

func TestComputeActionQueueNextDueRemovesUserWhenOnlyBlockedKittyLanesRemain(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	queue := []store.ActionQueueItem{
		{
			ID:        1,
			ActorType: store.ActionActorTypeKitty,
			ActorID:   11,
			Status:    "running",
			RunEndsAt: now.Add(1 * time.Minute),
		},
		{
			ID:        2,
			ActorType: store.ActionActorTypeKitty,
			ActorID:   11,
			Status:    "pending",
		},
	}
	if _, ok := computeActionQueueNextDue(queue, map[int64]struct{}{11: {}}, now); ok {
		t.Fatalf("expected no schedulable due when only blocked kitty lanes remain")
	}
}

func TestScheduleActionPushRestoresDueAfterKittyUnblocked(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	server := &Server{
		deps:              app.Dependencies{Redis: rd},
		mutationSyncQueue: make(chan mutationSyncScope, 1),
	}
	userID := int64(42)
	kittyID := int64(9)
	queue := []store.ActionQueueItem{{
		ID:        1,
		ActorType: store.ActionActorTypeKitty,
		ActorID:   kittyID,
		Status:    "running",
		RunEndsAt: time.Now().Add(-1 * time.Second),
	}}

	if err := server.syncActionQueueKittyCombatProjection(context.Background(), userID, []store.PlayerKitty{{
		ID:     kittyID,
		Status: store.KittyStatusInCombat,
	}}); err != nil {
		t.Fatalf("sync projection blocked: %v", err)
	}
	server.scheduleActionPush(userID, nil, queue)
	if got, err := rd.ZCard(context.Background(), actionQueueDueKey()).Result(); err != nil || got != 0 {
		t.Fatalf("zcard err=%v got=%d, want 0", err, got)
	}

	server.NotifyKittyStatusChanged(userID, kittyID, store.KittyStatusIdle)
	server.scheduleActionPush(userID, nil, queue)
	score, err := rd.ZScore(context.Background(), actionQueueDueKey(), strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		t.Fatalf("zscore err=%v", err)
	}
	if score == 0 {
		t.Fatalf("expected due user after kitty unblocked")
	}
	if score > float64(time.Now().UnixMilli()) {
		t.Fatalf("score=%f, want immediate due after unblocked", score)
	}
}

func TestApplyKittyStatusChangesProjectionUpdatesRedisSet(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	server := &Server{
		deps:              app.Dependencies{Redis: rd},
		mutationSyncQueue: make(chan mutationSyncScope, 1),
	}
	userID := int64(88)
	kittyID := int64(3)

	server.applyKittyStatusChangesProjection(context.Background(), []KittyStatusChange{{
		UserID:  userID,
		KittyID: kittyID,
		Status:  store.KittyStatusInCombat,
	}})
	isMember, err := mr.SIsMember(actionQueueKittyCombatBlockedKey(userID), strconv.FormatInt(kittyID, 10))
	if err != nil {
		t.Fatalf("sismember blocked err=%v", err)
	}
	if !isMember {
		t.Fatalf("expected kitty in blocked projection set")
	}

	server.applyKittyStatusChangesProjection(context.Background(), []KittyStatusChange{{
		UserID:  userID,
		KittyID: kittyID,
		Status:  store.KittyStatusIdle,
	}})
	isMember, err = mr.SIsMember(actionQueueKittyCombatBlockedKey(userID), strconv.FormatInt(kittyID, 10))
	if err != nil && err.Error() != "ERR no such key" {
		t.Fatalf("sismember unblocked err=%v", err)
	}
	if isMember {
		t.Fatalf("expected kitty removed from blocked projection set")
	}
}
