package httpapi

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSelectArenaRecommendedTargetsUsesSevenFourOneDistribution(t *testing.T) {
	rows := make([]store.ArenaTarget, 20)
	for index := range rows {
		rows[index].ArenaDefense.UserID = int64(index + 1)
	}
	cfg := config.ArenaSettings{TargetCount: 12, NearbyTargetCount: 7, RelaxedTargetCount: 4}
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	selected := selectArenaRecommendedTargets(rows, cfg, 99, now, nil)
	if len(selected) != 12 {
		t.Fatalf("selected=%d, want 12", len(selected))
	}
	for index := 0; index < 7; index++ {
		if selected[index].Target.UserID != int64(index+1) || selected[index].Tier != arenaRecommendationTierNearby {
			t.Fatalf("nearby[%d]=%d/%s", index, selected[index].Target.UserID, selected[index].Tier)
		}
	}
	for index := 7; index < 11; index++ {
		if selected[index].Target.UserID != int64(index+1) || selected[index].Tier != arenaRecommendationTierRelaxed {
			t.Fatalf("relaxed[%d]=%d/%s", index, selected[index].Target.UserID, selected[index].Tier)
		}
	}
	if selected[11].Target.UserID < 12 || selected[11].Tier != arenaRecommendationTierRandom {
		t.Fatalf("random=%d/%s, want remaining candidate", selected[11].Target.UserID, selected[11].Tier)
	}
}

func TestArenaMoreTargetScopeAndRotationStayBounded(t *testing.T) {
	if arenaMoreTargetsLimit != 100 {
		t.Fatalf("more target limit=%d, want 100", arenaMoreTargetsLimit)
	}
	if arenaMoreTargetsPageSize != 20 {
		t.Fatalf("more target page size=%d, want 20", arenaMoreTargetsPageSize)
	}
	for raw, want := range map[string]string{
		"more":      "more",
		"MORE":      "more",
		"all":       "more",
		"suggested": "suggested",
		"":          "suggested",
		"unknown":   "suggested",
	} {
		if got := normalizeArenaTargetScope(raw); got != want {
			t.Fatalf("normalize scope %q=%q, want %q", raw, got, want)
		}
	}

	day := time.Date(2026, time.August, 30, 0, 1, 0, 0, time.UTC)
	if got := arenaTargetRotationDay(day); got != "2026-08-30" {
		t.Fatalf("rotation day=%q", got)
	}
	if arenaTargetRotationDay(day.Add(23*time.Hour)) != arenaTargetRotationDay(day) {
		t.Fatal("rotation changed within the same UTC day")
	}
	if arenaTargetRotationDay(day.Add(24*time.Hour)) == arenaTargetRotationDay(day) {
		t.Fatal("rotation did not change on the next UTC day")
	}
}

func TestArenaRecommendationRefreshRotatesWithinTierBands(t *testing.T) {
	rows := make([]store.ArenaTarget, 100)
	for index := range rows {
		rows[index].ArenaDefense.UserID = int64(index + 1)
	}
	cfg := config.ArenaSettings{TargetCount: 12, NearbyTargetCount: 7, RelaxedTargetCount: 4}
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	firstRefresh := now.Add(-10 * time.Minute)
	secondRefresh := now.Add(-5 * time.Minute)
	first := selectArenaRecommendedTargets(rows, cfg, 99, now, &firstRefresh)
	second := selectArenaRecommendedTargets(rows, cfg, 99, now, &secondRefresh)
	if len(first) != 12 || len(second) != 12 {
		t.Fatalf("rotated recommendation sizes=%d/%d, want 12/12", len(first), len(second))
	}
	different := false
	for index := range first {
		if first[index].Target.UserID != second[index].Target.UserID {
			different = true
		}
		for _, selected := range []arenaRecommendedTarget{first[index], second[index]} {
			switch selected.Tier {
			case arenaRecommendationTierNearby:
				if selected.Target.UserID < 1 || selected.Target.UserID > 21 {
					t.Fatalf("nearby refresh selected id=%d outside 1..21", selected.Target.UserID)
				}
			case arenaRecommendationTierRelaxed:
				if selected.Target.UserID < 22 || selected.Target.UserID > 37 {
					t.Fatalf("relaxed refresh selected id=%d outside 22..37", selected.Target.UserID)
				}
			case arenaRecommendationTierRandom:
				if selected.Target.UserID < 38 {
					t.Fatalf("random refresh selected id=%d below 38", selected.Target.UserID)
				}
			default:
				t.Fatalf("unexpected recommendation tier %q", selected.Tier)
			}
		}
	}
	if !different {
		t.Fatal("changing the server refresh timestamp did not rotate recommendations")
	}
}

func TestArenaRecommendationRefreshStateUsesRedisServerTime(t *testing.T) {
	serverTime := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	ready := buildArenaRecommendationRefreshState(serverTime, nil, 5*time.Minute)
	if ready.ServerTime != serverTime || ready.RefreshedAt != nil || ready.AvailableAt != nil {
		t.Fatalf("initial refresh state=%+v, want immediately available", ready)
	}
	refreshedAt := serverTime.Add(-2 * time.Minute)
	cooling := buildArenaRecommendationRefreshState(serverTime, &refreshedAt, 5*time.Minute)
	if cooling.RefreshedAt == nil || !cooling.RefreshedAt.Equal(refreshedAt) || cooling.AvailableAt == nil || !cooling.AvailableAt.Equal(refreshedAt.Add(5*time.Minute)) {
		t.Fatalf("cooling refresh state=%+v", cooling)
	}

	server := &Server{}
	if _, err := server.getArenaRecommendationRefreshState(context.Background(), 0, 5*time.Minute); !errors.Is(err, store.ErrBattleRoomConfigInvalid) {
		t.Fatalf("invalid refresh state error=%v, want %v", err, store.ErrBattleRoomConfigInvalid)
	}
	if _, err := server.getArenaRecommendationRefreshState(context.Background(), 1, 5*time.Minute); !errors.Is(err, errArenaRecommendationRefreshUnavailable) {
		t.Fatalf("missing redis error=%v, want %v", err, errArenaRecommendationRefreshUnavailable)
	}
}

func TestArenaRecommendationRefreshUsesAtomicRedisCooldown(t *testing.T) {
	mr := miniredis.RunT(t)
	serverTime := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	mr.SetTime(serverTime)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rd.Close() })
	server := &Server{deps: app.Dependencies{Redis: rd}}
	ctx := context.Background()
	cooldown := 5 * time.Minute

	initial, err := server.getArenaRecommendationRefreshState(ctx, 42, cooldown)
	if err != nil {
		t.Fatalf("get initial refresh state: %v", err)
	}
	if initial.ServerTime != serverTime || initial.RefreshedAt != nil || initial.AvailableAt != nil {
		t.Fatalf("initial redis refresh state=%+v", initial)
	}

	const requests = 12
	var allowed atomic.Int32
	var cooling atomic.Int32
	var wait sync.WaitGroup
	unhandled := make(chan error, requests)
	wait.Add(requests)
	for range requests {
		go func() {
			defer wait.Done()
			_, refreshErr := server.refreshArenaRecommendations(ctx, 42, cooldown)
			switch {
			case refreshErr == nil:
				allowed.Add(1)
			case errors.Is(refreshErr, errArenaRecommendationRefreshCooldown):
				cooling.Add(1)
			default:
				unhandled <- refreshErr
			}
		}()
	}
	wait.Wait()
	close(unhandled)
	for refreshErr := range unhandled {
		t.Fatalf("concurrent refresh returned unexpected error: %v", refreshErr)
	}
	if allowed.Load() != 1 || cooling.Load() != requests-1 {
		t.Fatalf("concurrent refresh allowed/cooling=%d/%d, want 1/%d", allowed.Load(), cooling.Load(), requests-1)
	}

	stored, err := server.getArenaRecommendationRefreshState(ctx, 42, cooldown)
	if err != nil {
		t.Fatalf("get stored refresh state: %v", err)
	}
	if stored.RefreshedAt == nil || !stored.RefreshedAt.Equal(serverTime) || stored.AvailableAt == nil || !stored.AvailableAt.Equal(serverTime.Add(cooldown)) {
		t.Fatalf("stored redis refresh state=%+v", stored)
	}
	ttl, err := rd.PTTL(ctx, arenaRecommendationRefreshKey(42)).Result()
	if err != nil {
		t.Fatalf("read refresh state ttl: %v", err)
	}
	if ttl != arenaRecommendationRefreshStateBaseTTL+cooldown {
		t.Fatalf("refresh state ttl=%s, want %s", ttl, arenaRecommendationRefreshStateBaseTTL+cooldown)
	}

	mr.SetTime(serverTime.Add(cooldown))
	next, err := server.refreshArenaRecommendations(ctx, 42, cooldown)
	if err != nil {
		t.Fatalf("refresh at cooldown boundary: %v", err)
	}
	if next.RefreshedAt == nil || !next.RefreshedAt.Equal(serverTime.Add(cooldown)) {
		t.Fatalf("next refresh state=%+v", next)
	}

	mr.FastForward(arenaRecommendationRefreshStateBaseTTL + cooldown)
	expired, err := server.getArenaRecommendationRefreshState(ctx, 42, cooldown)
	if err != nil {
		t.Fatalf("get expired refresh state: %v", err)
	}
	if expired.RefreshedAt != nil || expired.AvailableAt != nil {
		t.Fatalf("expired redis refresh state=%+v, want default recommendations", expired)
	}
}

func TestArenaMoreTargetsPaginationStopsAtDailySampleBoundary(t *testing.T) {
	rows := make([]store.ArenaTarget, 55)
	for index := range rows {
		rows[index].ArenaDefense.UserID = int64(index + 1)
	}
	first, next, hasMore := paginateArenaMoreTargets(rows, 0)
	if len(first) != 20 || next != 20 || !hasMore {
		t.Fatalf("first page len/next/hasMore=%d/%d/%t", len(first), next, hasMore)
	}
	last, next, hasMore := paginateArenaMoreTargets(rows, 40)
	if len(last) != 15 || next != 55 || hasMore {
		t.Fatalf("last page len/next/hasMore=%d/%d/%t", len(last), next, hasMore)
	}
	beyond, next, hasMore := paginateArenaMoreTargets(rows, arenaMoreTargetsLimit)
	if len(beyond) != 0 || next != arenaMoreTargetsLimit || hasMore {
		t.Fatalf("beyond page len/next/hasMore=%d/%d/%t", len(beyond), next, hasMore)
	}
	for raw, wantError := range map[string]bool{"": false, "0": false, "100": false, "-1": true, "101": true, "bad": true} {
		_, err := arenaMoreTargetsOffset(raw)
		if (err != nil) != wantError {
			t.Fatalf("offset %q error=%v, wantError=%t", raw, err, wantError)
		}
	}
}

func TestArenaRecommendationTokenBindsPlayerTargetRevisionAndExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	token := arenaRecommendationToken("test-secret", 11, 22, 3, now)
	if !validArenaRecommendationToken("test-secret", token, 11, 22, 3, now.Add(29*time.Minute)) {
		t.Fatal("fresh recommendation token should be valid")
	}
	for _, test := range []struct {
		name                        string
		secret, token               string
		requester, target, revision int64
		now                         time.Time
	}{
		{name: "wrong secret", secret: "other", token: token, requester: 11, target: 22, revision: 3, now: now},
		{name: "wrong player", secret: "test-secret", token: token, requester: 12, target: 22, revision: 3, now: now},
		{name: "wrong target", secret: "test-secret", token: token, requester: 11, target: 23, revision: 3, now: now},
		{name: "stale revision", secret: "test-secret", token: token, requester: 11, target: 22, revision: 4, now: now},
		{name: "expired", secret: "test-secret", token: token, requester: 11, target: 22, revision: 3, now: now.Add(31 * time.Minute)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if validArenaRecommendationToken(test.secret, test.token, test.requester, test.target, test.revision, test.now) {
				t.Fatal("invalid recommendation token was accepted")
			}
		})
	}
}
