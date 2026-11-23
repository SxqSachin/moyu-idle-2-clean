package httpapi

import (
	"context"
	"encoding/json"
	"testing"
)

func TestPlayerProfileChangedBroadcastInvalidatesProfileCaches(t *testing.T) {
	const userID int64 = 42
	s := &Server{
		wsSubject:        "test.ws.broadcast",
		playerProfileMem: map[int64]playerProfileCacheEntry{userID: {}},
		playerPublicMem:  map[int64]playerProfileCacheEntry{userID: {}},
	}

	s.broadcastPlayerProfileChanged(context.Background(), userID, "  NewName  ", 101)

	if _, ok := s.playerProfileMem[userID]; ok {
		t.Fatal("profile cache was not invalidated")
	}
	if _, ok := s.playerPublicMem[userID]; ok {
		t.Fatal("public profile cache was not invalidated")
	}
}

func TestPlayerProfileChangedPayloadIsPublicAndMinimal(t *testing.T) {
	payload, ok := newPlayerProfileChangedPayload(7, "  新名字  ", 123)
	if !ok {
		t.Fatal("expected valid profile change payload")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(fields) != 3 || fields["userId"] != float64(7) || fields["username"] != "新名字" || fields["changeId"] != float64(123) {
		t.Fatalf("unexpected payload: %#v", fields)
	}
}

func TestPlayerProfileChangedEventRejectsInvalidPayload(t *testing.T) {
	const userID int64 = 9
	s := &Server{
		playerProfileMem: map[int64]playerProfileCacheEntry{userID: {}},
		playerPublicMem:  map[int64]playerProfileCacheEntry{userID: {}},
	}

	s.applyPlayerProfileChangedEvent(json.RawMessage(`{"userId":0,"username":"ignored"}`))

	if _, ok := s.playerProfileMem[userID]; !ok {
		t.Fatal("invalid event unexpectedly invalidated profile cache")
	}
	if _, ok := s.playerPublicMem[userID]; !ok {
		t.Fatal("invalid event unexpectedly invalidated public profile cache")
	}
}
