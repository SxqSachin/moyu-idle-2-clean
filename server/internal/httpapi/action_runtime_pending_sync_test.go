package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/app"
)

func TestActionRuntimePendingSyncClearIsRecordScoped(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	ctx := context.Background()
	s := &Server{deps: app.Dependencies{Redis: rd}}
	nowMS := time.Now().UnixMilli()
	firstID := actionruntime.PendingSyncID(42, 1001)
	secondID := actionruntime.PendingSyncID(42, 1002)

	if err := rd.ZAdd(ctx, actionruntime.PendingSyncUsersKey(),
		redis.Z{Score: float64(nowMS), Member: firstID},
		redis.Z{Score: float64(nowMS + 1), Member: secondID},
	).Err(); err != nil {
		t.Fatalf("seed pending zset: %v", err)
	}
	if err := rd.HSet(ctx, actionruntime.PendingSyncInfoKey(firstID), "user_id", 42).Err(); err != nil {
		t.Fatalf("seed first info: %v", err)
	}
	if err := rd.HSet(ctx, actionruntime.PendingSyncInfoKey(secondID), "user_id", 42).Err(); err != nil {
		t.Fatalf("seed second info: %v", err)
	}

	s.clearActionRuntimePendingSync(ctx, secondID, nowMS)
	if _, err := rd.ZScore(ctx, actionruntime.PendingSyncUsersKey(), secondID).Result(); err != nil {
		t.Fatalf("wrong-score clear removed second pending record: %v", err)
	}

	s.clearActionRuntimePendingSync(ctx, firstID, nowMS)
	if _, err := rd.ZScore(ctx, actionruntime.PendingSyncUsersKey(), firstID).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("first pending record still exists err=%v", err)
	}
	if exists := rd.Exists(ctx, actionruntime.PendingSyncInfoKey(firstID)).Val(); exists != 0 {
		t.Fatalf("first pending info still exists")
	}
	if _, err := rd.ZScore(ctx, actionruntime.PendingSyncUsersKey(), secondID).Result(); err != nil {
		t.Fatalf("second pending record was removed: %v", err)
	}
	if exists := rd.Exists(ctx, actionruntime.PendingSyncInfoKey(secondID)).Val(); exists != 1 {
		t.Fatalf("second pending info missing")
	}
}

func TestActionRuntimePendingSyncClaimIsRecordScoped(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	ctx := context.Background()
	s := &Server{deps: app.Dependencies{Redis: rd}, wsNodeAdvertise: "server-a"}
	pendingID := actionruntime.PendingSyncID(42, 2001)
	nowMS := time.Now().UnixMilli()

	claimed, err := s.claimActionRuntimePendingSync(ctx, pendingID)
	if err != nil {
		t.Fatalf("claim missing pending sync: %v", err)
	}
	if claimed {
		t.Fatalf("missing pending record should not be claimable")
	}

	if err := rd.ZAdd(ctx, actionruntime.PendingSyncUsersKey(), redis.Z{Score: float64(nowMS), Member: pendingID}).Err(); err != nil {
		t.Fatalf("seed pending zset: %v", err)
	}
	if err := rd.HSet(ctx, actionruntime.PendingSyncInfoKey(pendingID), "user_id", 42).Err(); err != nil {
		t.Fatalf("seed pending info: %v", err)
	}

	claimed, err = s.claimActionRuntimePendingSync(ctx, pendingID)
	if err != nil {
		t.Fatalf("claim pending sync: %v", err)
	}
	if !claimed {
		t.Fatalf("first claim should win")
	}
	claimed, err = s.claimActionRuntimePendingSync(ctx, pendingID)
	if err != nil {
		t.Fatalf("claim pending sync again: %v", err)
	}
	if claimed {
		t.Fatalf("second claim should not win before ttl expires")
	}

	s.clearActionRuntimePendingSync(ctx, pendingID, nowMS+1)
	if exists := rd.Exists(ctx, actionruntime.PendingSyncClaimKey(pendingID)).Val(); exists != 1 {
		t.Fatalf("wrong-score clear should keep claim")
	}

	s.clearActionRuntimePendingSync(ctx, pendingID, nowMS)
	if exists := rd.Exists(ctx, actionruntime.PendingSyncClaimKey(pendingID)).Val(); exists != 0 {
		t.Fatalf("correct clear should remove claim")
	}
	claimed, err = s.claimActionRuntimePendingSync(ctx, pendingID)
	if err != nil {
		t.Fatalf("claim cleared pending sync: %v", err)
	}
	if claimed {
		t.Fatalf("cleared pending record should not be claimable")
	}
}
