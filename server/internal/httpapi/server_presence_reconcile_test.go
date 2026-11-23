package httpapi

import (
	"context"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestReconcileOnlinePresenceContinuesPastActiveFirstBatch(t *testing.T) {
	allRows := make([]store.OnlinePresenceRow, 0, 101)
	lastSeenAt := time.Unix(10_000, 0)
	for userID := int64(1); userID <= 101; userID++ {
		allRows = append(allRows, store.OnlinePresenceRow{
			UserID:         userID,
			SessionVersion: 1,
			LastSeenAt:     lastSeenAt.Add(time.Duration(userID) * time.Second),
		})
	}

	var cursors []int64
	listOnline := func(_ context.Context, limit int, afterUserID int64) ([]store.OnlinePresenceRow, error) {
		cursors = append(cursors, afterUserID)
		out := make([]store.OnlinePresenceRow, 0, limit)
		for _, row := range allRows {
			if row.UserID <= afterUserID {
				continue
			}
			out = append(out, row)
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	}

	var offlineUsers []int64
	var offlineStartedAt []time.Time
	cachedOffline := map[int64]string{}
	result, err := reconcileOnlinePresence(
		context.Background(),
		100,
		2,
		0,
		listOnline,
		func(_ context.Context, userID, _ int64) bool {
			return userID <= 100
		},
		func(_ context.Context, row store.OnlinePresenceRow) error {
			offlineUsers = append(offlineUsers, row.UserID)
			offlineStartedAt = append(offlineStartedAt, row.LastSeenAt)
			return nil
		},
		func(userID int64, status string) {
			cachedOffline[userID] = status
		},
	)
	if err != nil {
		t.Fatalf("reconcileOnlinePresence err=%v", err)
	}
	if result.scanned != 101 || result.corrected != 1 || result.batches != 2 || result.cursorUserID != 101 || result.limited {
		t.Fatalf("result=%+v, want scanned=101 corrected=1 batches=2 cursor=101 limited=false", result)
	}
	if len(cursors) != 2 || cursors[0] != 0 || cursors[1] != 100 {
		t.Fatalf("cursors=%v, want [0 100]", cursors)
	}
	if len(offlineUsers) != 1 || offlineUsers[0] != 101 {
		t.Fatalf("offlineUsers=%v, want [101]", offlineUsers)
	}
	wantLastSeenAt := lastSeenAt.Add(101 * time.Second)
	if len(offlineStartedAt) != 1 || !offlineStartedAt[0].Equal(wantLastSeenAt) {
		t.Fatalf("offlineStartedAt=%v, want %v", offlineStartedAt, wantLastSeenAt)
	}
	if got := cachedOffline[101]; got != "offline" {
		t.Fatalf("cachedOffline[101]=%q, want offline", got)
	}
}

func TestReconcileOnlinePresenceMarksLimitedWhenMaxBatchesReached(t *testing.T) {
	listOnline := func(_ context.Context, limit int, afterUserID int64) ([]store.OnlinePresenceRow, error) {
		out := make([]store.OnlinePresenceRow, 0, limit)
		for i := 1; i <= limit; i++ {
			userID := afterUserID + int64(i)
			out = append(out, store.OnlinePresenceRow{UserID: userID, SessionVersion: 1})
		}
		return out, nil
	}

	result, err := reconcileOnlinePresence(
		context.Background(),
		100,
		2,
		0,
		listOnline,
		func(context.Context, int64, int64) bool { return true },
		func(context.Context, store.OnlinePresenceRow) error { return nil },
		func(int64, string) {},
	)
	if err != nil {
		t.Fatalf("reconcileOnlinePresence err=%v", err)
	}
	if !result.limited || result.scanned != 200 || result.batches != 2 || result.cursorUserID != 200 {
		t.Fatalf("result=%+v, want limited scanned=200 batches=2 cursor=200", result)
	}
}

func TestReconcileOnlinePresenceStartsAfterPersistedCursor(t *testing.T) {
	var cursors []int64
	listOnline := func(_ context.Context, limit int, afterUserID int64) ([]store.OnlinePresenceRow, error) {
		cursors = append(cursors, afterUserID)
		return []store.OnlinePresenceRow{
			{UserID: afterUserID + 1, SessionVersion: 1},
		}, nil
	}

	result, err := reconcileOnlinePresence(
		context.Background(),
		100,
		10,
		200,
		listOnline,
		func(context.Context, int64, int64) bool { return true },
		func(context.Context, store.OnlinePresenceRow) error { return nil },
		func(int64, string) {},
	)
	if err != nil {
		t.Fatalf("reconcileOnlinePresence err=%v", err)
	}
	if result.scanned != 1 || result.cursorUserID != 201 || result.limited {
		t.Fatalf("result=%+v, want scanned=1 cursor=201 limited=false", result)
	}
	if len(cursors) != 1 || cursors[0] != 200 {
		t.Fatalf("cursors=%v, want [200]", cursors)
	}
}
