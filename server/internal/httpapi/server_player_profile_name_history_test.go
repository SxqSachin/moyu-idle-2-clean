package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

type playerUsernameProfileSnapshotReaderFunc func(context.Context, int64, int) (store.PlayerUsernameProfileSnapshot, error)

func (fn playerUsernameProfileSnapshotReaderFunc) GetPlayerUsernameProfileSnapshot(ctx context.Context, userID int64, limit int) (store.PlayerUsernameProfileSnapshot, error) {
	return fn(ctx, userID, limit)
}

func TestLoadPlayerProfileNameHistorySanitizesPublicPayload(t *testing.T) {
	changedAt := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.UTC)
	changedBy := int64(7)
	var receivedUserID int64
	var receivedLimit int
	reader := playerUsernameProfileSnapshotReaderFunc(func(_ context.Context, userID int64, limit int) (store.PlayerUsernameProfileSnapshot, error) {
		receivedUserID = userID
		receivedLimit = limit
		return store.PlayerUsernameProfileSnapshot{
			PublicUsernameSnapshot: store.PublicUsernameSnapshot{Username: "CurrentName", ChangeID: 99},
			History: []store.PlayerUsernameHistoryEntry{{
				ID:              99,
				UserID:          userID,
				OldUsername:     "  OldName  ",
				NewUsername:     "PrivateCurrentName",
				ChangedByUserID: &changedBy,
				ChangeSource:    "self_service",
				CostItemID:      "rename_ticket",
				CostItemCount:   1,
				ChangedAt:       changedAt,
			}},
		}, nil
	})

	usernameSnapshot, history, hasMore, err := loadPlayerProfileUsernameSnapshot(context.Background(), reader, 7)
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if receivedUserID != 7 || receivedLimit != playerProfileNameHistoryLimit+1 {
		t.Fatalf("query args userID=%d limit=%d", receivedUserID, receivedLimit)
	}
	if usernameSnapshot.Username != "CurrentName" || usernameSnapshot.ChangeID != 99 {
		t.Fatalf("unexpected username snapshot: %#v", usernameSnapshot)
	}
	if hasMore {
		t.Fatal("single history entry unexpectedly reported more results")
	}
	if len(history) != 1 || history[0].OldName != "OldName" {
		t.Fatalf("unexpected history: %#v", history)
	}

	raw, err := json.Marshal(history[0])
	if err != nil {
		t.Fatalf("marshal history: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(fields) != 1 || fields["oldName"] != "OldName" {
		t.Fatalf("public history leaked private fields: %#v", fields)
	}
	if _, exists := fields["newUsername"]; exists {
		t.Fatalf("new username leaked: %#v", fields)
	}
	for _, privateField := range []string{"changedAt", "newName", "changedByUserId", "changeSource", "costItemId", "costItemCount"} {
		if _, exists := fields[privateField]; exists {
			t.Fatalf("private field %q leaked: %#v", privateField, fields)
		}
	}
}

func TestLoadPlayerProfileNameHistoryReturnsNonNilEmptyList(t *testing.T) {
	reader := playerUsernameProfileSnapshotReaderFunc(func(context.Context, int64, int) (store.PlayerUsernameProfileSnapshot, error) {
		return store.PlayerUsernameProfileSnapshot{
			PublicUsernameSnapshot: store.PublicUsernameSnapshot{Username: "CurrentName"},
		}, nil
	})

	_, history, hasMore, err := loadPlayerProfileUsernameSnapshot(context.Background(), reader, 7)
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if hasMore || history == nil || len(history) != 0 {
		t.Fatalf("history=%#v, want non-nil empty list", history)
	}
}

func TestLoadPlayerProfileNameHistoryReportsTruncation(t *testing.T) {
	entries := make([]store.PlayerUsernameHistoryEntry, 0, playerProfileNameHistoryLimit+1)
	for i := 0; i < playerProfileNameHistoryLimit+1; i++ {
		entries = append(entries, store.PlayerUsernameHistoryEntry{OldUsername: "old-name"})
	}
	reader := playerUsernameProfileSnapshotReaderFunc(func(context.Context, int64, int) (store.PlayerUsernameProfileSnapshot, error) {
		return store.PlayerUsernameProfileSnapshot{
			PublicUsernameSnapshot: store.PublicUsernameSnapshot{Username: "CurrentName", ChangeID: 51},
			History:                entries,
		}, nil
	})

	_, history, hasMore, err := loadPlayerProfileUsernameSnapshot(context.Background(), reader, 7)
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if !hasMore || len(history) != playerProfileNameHistoryLimit {
		t.Fatalf("history len=%d hasMore=%v, want len=%d hasMore=true", len(history), hasMore, playerProfileNameHistoryLimit)
	}
}
