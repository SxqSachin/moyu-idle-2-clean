package httpapi

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestPlayerProfileCacheEpochRejectsStaleRefill(t *testing.T) {
	const userID int64 = 42
	now := time.Date(2026, time.August, 27, 16, 0, 0, 0, time.UTC)
	s := &Server{
		playerProfileMem:   make(map[int64]playerProfileCacheEntry),
		playerProfileEpoch: make(map[int64]uint64),
		playerPublicMem:    make(map[int64]playerProfileCacheEntry),
		playerPublicEpoch:  make(map[int64]uint64),
	}

	_, profileEpoch, profileHit := s.lookupPlayerProfileCache(userID, now)
	_, publicEpoch, publicHit := s.lookupPlayerPublicCache(userID, now)
	if profileHit || publicHit {
		t.Fatal("empty caches unexpectedly hit")
	}

	s.invalidatePlayerProfileCaches(userID)
	stale := playerProfileCacheEntry{
		Payload:   fiber.Map{"username": "old-name"},
		ExpiresAt: now.Add(playerProfileCacheTTL),
		CachedAt:  now,
	}
	if s.cachePlayerProfileIfCurrent(userID, profileEpoch, stale) {
		t.Fatal("stale detailed profile refill was accepted after invalidation")
	}
	if s.cachePlayerPublicIfCurrent(userID, publicEpoch, stale) {
		t.Fatal("stale public profile refill was accepted after invalidation")
	}
	if _, exists := s.playerProfileMem[userID]; exists {
		t.Fatal("stale detailed profile payload was stored")
	}
	if _, exists := s.playerPublicMem[userID]; exists {
		t.Fatal("stale public profile payload was stored")
	}

	_, currentProfileEpoch, _ := s.lookupPlayerProfileCache(userID, now)
	_, currentPublicEpoch, _ := s.lookupPlayerPublicCache(userID, now)
	if currentProfileEpoch == profileEpoch || currentPublicEpoch == publicEpoch {
		t.Fatal("cache invalidation did not advance both per-user epochs")
	}
	if !s.cachePlayerProfileIfCurrent(userID, currentProfileEpoch, stale) {
		t.Fatal("current detailed profile refill was rejected")
	}
	if !s.cachePlayerPublicIfCurrent(userID, currentPublicEpoch, stale) {
		t.Fatal("current public profile refill was rejected")
	}
}
