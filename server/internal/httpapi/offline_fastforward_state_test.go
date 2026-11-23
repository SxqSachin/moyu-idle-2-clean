package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func TestOfflineFastForwardStateCapsTargetAtMaxDuration(t *testing.T) {
	startedAt := time.Unix(1_000, 0)
	now := startedAt.Add(48 * time.Hour)

	state, ok := newOfflineFastForwardState(startedAt, int64((48*time.Hour)/time.Second), now, 24*time.Hour)
	if !ok {
		t.Fatalf("expected state")
	}
	if got, want := state.OfflineStartedAtMS, startedAt.UnixMilli(); got != want {
		t.Fatalf("startedAtMS=%d, want %d", got, want)
	}
	if got, want := state.OfflineTargetAtMS, startedAt.Add(24*time.Hour).UnixMilli(); got != want {
		t.Fatalf("targetAtMS=%d, want %d", got, want)
	}
	if got, want := state.OfflineDurationSeconds, int64((24*time.Hour)/time.Second); got != want {
		t.Fatalf("duration=%d, want %d", got, want)
	}
}

func TestOfflineFastForwardStateParsesLegacyDuration(t *testing.T) {
	now := time.Unix(10_000, 0)
	state, ok := parseOfflineFastForwardState("7200", now, time.Hour)
	if !ok {
		t.Fatalf("expected legacy state")
	}
	if got, want := state.OfflineDurationSeconds, int64(3600); got != want {
		t.Fatalf("duration=%d, want %d", got, want)
	}
	if got, want := state.OfflineTargetAtMS, now.Add(-time.Hour).UnixMilli(); got != want {
		t.Fatalf("targetAtMS=%d, want %d", got, want)
	}
}

func TestOfflineFastForwardStateExpiryMatchesReadyTTL(t *testing.T) {
	now := time.Unix(15_000, 0)
	state := offlineFastForwardState{CreatedAtMS: now.Add(-offlineFastForwardReadyTTL + time.Millisecond).UnixMilli()}
	if offlineFastForwardStateExpired(state, now) {
		t.Fatalf("state expired before TTL")
	}
	state.CreatedAtMS = now.Add(-offlineFastForwardReadyTTL).UnixMilli()
	if !offlineFastForwardStateExpired(state, now) {
		t.Fatalf("state did not expire at TTL")
	}
	if offlineFastForwardStateExpired(offlineFastForwardState{}, now) {
		t.Fatalf("legacy state without createdAtMS should not expire")
	}
}

func TestOfflineFastForwardFallbackPrunesExpiredState(t *testing.T) {
	now := time.Unix(16_000, 0)
	s := &Server{offlineFFReady: map[int64]offlineFastForwardState{
		1: {
			OfflineTargetAtMS:      now.UnixMilli(),
			OfflineDurationSeconds: 60,
			CreatedAtMS:            now.Add(-offlineFastForwardReadyTTL).UnixMilli(),
		},
		2: {
			OfflineTargetAtMS:      now.UnixMilli(),
			OfflineDurationSeconds: 60,
			CreatedAtMS:            now.Add(-time.Minute).UnixMilli(),
		},
	}}

	if got := s.pruneExpiredOfflineFastForwardFallback(now); got != 1 {
		t.Fatalf("pruned=%d, want 1", got)
	}
	if _, ok := s.offlineFFReady[1]; ok {
		t.Fatalf("expired state remains")
	}
	if _, ok := s.offlineFFReady[2]; !ok {
		t.Fatalf("fresh state was pruned")
	}
	if got := s.memoryMapSizeSnapshot()["offlineFFReady"]; got != 1 {
		t.Fatalf("offlineFFReady size=%d, want 1", got)
	}
}

func TestOfflineFastForwardFallbackSweepPrunesWithoutStaleUserCaches(t *testing.T) {
	now := time.Unix(16_500, 0)
	s := &Server{offlineFFReady: map[int64]offlineFastForwardState{
		1: {
			OfflineTargetAtMS:      now.UnixMilli(),
			OfflineDurationSeconds: 60,
			CreatedAtMS:            now.Add(-offlineFastForwardReadyTTL).UnixMilli(),
		},
	}}

	s.sweepPresenceAndPatchCaches(now)

	if got := len(s.offlineFFReady); got != 0 {
		t.Fatalf("offlineFFReady size=%d, want 0", got)
	}
}

func TestOfflineFastForwardFallbackRejectsExpiredStateOnAccess(t *testing.T) {
	now := time.Now()
	expired := offlineFastForwardState{
		OfflineTargetAtMS:      now.UnixMilli(),
		OfflineDurationSeconds: 60,
		CreatedAtMS:            now.Add(-offlineFastForwardReadyTTL).UnixMilli(),
	}
	s := &Server{offlineFFReady: map[int64]offlineFastForwardState{1: expired, 2: expired, 3: expired}}

	if s.hasOfflineFastForwardReady(1) {
		t.Fatalf("has returned true for expired state")
	}
	if ready, _ := s.peekOfflineFastForward(2); ready {
		t.Fatalf("peek returned expired state")
	}
	if consumed, _ := s.consumeOfflineFastForwardIfUnchanged(3, expired); consumed {
		t.Fatalf("consume returned expired state")
	}
	if got := len(s.offlineFFReady); got != 0 {
		t.Fatalf("offlineFFReady size=%d, want 0", got)
	}
}

func TestOfflineFastForwardPeekConsumeRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	s := &Server{
		deps: app.Dependencies{
			Redis: rd,
			Config: config.Config{GlobalSettings: config.GlobalSettings{
				Activity: config.ActivitySettings{OfflineProgressMaxDurationSeconds: 3600},
			}},
		},
	}
	now := time.Unix(20_000, 0)
	startedAt := now.Add(-2 * time.Hour)
	s.markOfflineFastForward(42, startedAt, int64((2*time.Hour)/time.Second), now)

	ready, state := s.peekOfflineFastForward(42)
	if !ready {
		t.Fatalf("expected ready state")
	}
	if got, want := state.OfflineDurationSeconds, int64(3600); got != want {
		t.Fatalf("peek duration=%d, want %d", got, want)
	}
	consumed, consumedState := s.consumeOfflineFastForwardIfUnchanged(42, state)
	if !consumed {
		t.Fatalf("expected consumed state")
	}
	if consumedState.OfflineTargetAtMS != state.OfflineTargetAtMS {
		t.Fatalf("consume target=%d, want %d", consumedState.OfflineTargetAtMS, state.OfflineTargetAtMS)
	}
	if exists := rd.Exists(context.Background(), offlineFastForwardReadyKey(42)).Val(); exists != 0 {
		t.Fatalf("offline marker still exists")
	}
}

func TestOfflineFastForwardConditionalConsumePreservesNewerTarget(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	s := &Server{
		deps: app.Dependencies{
			Redis: rd,
			Config: config.Config{GlobalSettings: config.GlobalSettings{
				Activity: config.ActivitySettings{OfflineProgressMaxDurationSeconds: 10_800},
			}},
		},
	}
	now := time.Unix(25_000, 0)
	s.markOfflineFastForward(42, now.Add(-time.Hour), 3600, now)
	ready, oldState := s.peekOfflineFastForward(42)
	if !ready {
		t.Fatal("expected ready state")
	}
	s.markOfflineFastForward(42, now.Add(-2*time.Hour), 7201, now.Add(time.Second))

	if consumed, _ := s.consumeOfflineFastForwardIfUnchanged(42, oldState); consumed {
		t.Fatal("stale marker must not consume a newer marker")
	}
	if exists := rd.Exists(context.Background(), offlineFastForwardReadyKey(42)).Val(); exists != 1 {
		t.Fatal("conditional consume removed a newer marker")
	}
	ready, newState := s.peekOfflineFastForward(42)
	if !ready {
		t.Fatal("expected newer ready state")
	}
	if consumed, got := s.consumeOfflineFastForwardIfUnchanged(42, newState); !consumed || got.OfflineTargetAtMS != newState.OfflineTargetAtMS {
		t.Fatalf("matching marker consume = (%t, %d), want true/%d", consumed, got.OfflineTargetAtMS, newState.OfflineTargetAtMS)
	}
}

func TestOfflineFastForwardConditionalConsumeSupportsLegacyMarker(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	s := &Server{deps: app.Dependencies{Redis: rd}}
	key := offlineFastForwardReadyKey(42)
	if err := rd.Set(context.Background(), key, "3600", offlineFastForwardReadyTTL).Err(); err != nil {
		t.Fatal(err)
	}
	ready, state := s.peekOfflineFastForward(42)
	if !ready {
		t.Fatal("expected legacy ready state")
	}
	if consumed, _ := s.consumeOfflineFastForwardIfUnchanged(42, state); !consumed {
		t.Fatal("expected legacy marker to be conditionally consumed")
	}
	if exists := rd.Exists(context.Background(), key).Val(); exists != 0 {
		t.Fatal("legacy marker still exists")
	}
}

func TestOfflineFastForwardFallbackConditionalConsumePreservesNewerTarget(t *testing.T) {
	now := time.Now()
	state := offlineFastForwardState{
		OfflineStartedAtMS:     now.Add(-time.Hour).UnixMilli(),
		OfflineTargetAtMS:      now.UnixMilli(),
		OfflineDurationSeconds: 3600,
		CreatedAtMS:            now.UnixMilli(),
	}
	s := &Server{offlineFFReady: map[int64]offlineFastForwardState{42: state}}
	oldState := state
	state.OfflineStartedAtMS = now.Add(-2 * time.Hour).UnixMilli()
	state.OfflineDurationSeconds = 7200
	state.OfflineTargetAtMS = now.Add(time.Second).UnixMilli()
	state.CreatedAtMS = now.Add(time.Second).UnixMilli()
	s.offlineFFReady[42] = state

	if consumed, _ := s.consumeOfflineFastForwardIfUnchanged(42, oldState); consumed {
		t.Fatal("stale marker must not consume a newer fallback marker")
	}
	if got, ok := s.offlineFFReady[42]; !ok || got.OfflineTargetAtMS != state.OfflineTargetAtMS {
		t.Fatal("conditional consume removed the fallback marker")
	}
	if consumed, got := s.consumeOfflineFastForwardIfUnchanged(42, state); !consumed || got.OfflineTargetAtMS != state.OfflineTargetAtMS {
		t.Fatalf("matching fallback consume = (%t, %d), want true/%d", consumed, got.OfflineTargetAtMS, state.OfflineTargetAtMS)
	}
	if _, ok := s.offlineFFReady[42]; ok {
		t.Fatal("matching conditional consume left the fallback marker behind")
	}
}

func TestOfflineFastForwardWindowForOfflinePresence(t *testing.T) {
	now := time.Unix(30_000, 0)
	startedAt := now.Add(-10 * time.Minute)
	s := &Server{deps: app.Dependencies{Config: config.Config{
		SharedGlobalSettings: config.DefaultSharedGlobalSettings(),
	}}}

	gotStartedAt, gotDurationS := s.offlineFastForwardWindow(store.PresenceState{
		Status:     "offline",
		LastSeenAt: startedAt,
	}, now)
	if !gotStartedAt.Equal(startedAt) {
		t.Fatalf("startedAt=%v, want %v", gotStartedAt, startedAt)
	}
	if gotDurationS != int64((10*time.Minute)/time.Second) {
		t.Fatalf("duration=%d, want 600", gotDurationS)
	}
}

func TestOfflineFastForwardWindowForStaleOnlinePresenceSkipsVisibilityGrace(t *testing.T) {
	now := time.Unix(40_000, 0)
	lastSeenAt := now.Add(-10 * time.Minute)
	settings := config.DefaultSharedGlobalSettings()
	settings.Presence.VisibilityOfflineDelaySeconds = 120
	s := &Server{deps: app.Dependencies{Config: config.Config{
		SharedGlobalSettings: settings,
	}}}

	gotStartedAt, gotDurationS := s.offlineFastForwardWindow(store.PresenceState{
		Status:     "online",
		LastSeenAt: lastSeenAt,
	}, now)
	wantStartedAt := lastSeenAt.Add(120 * time.Second)
	if !gotStartedAt.Equal(wantStartedAt) {
		t.Fatalf("startedAt=%v, want %v", gotStartedAt, wantStartedAt)
	}
	if gotDurationS != int64((8*time.Minute)/time.Second) {
		t.Fatalf("duration=%d, want 480", gotDurationS)
	}
}

func TestOfflineFastForwardWindowForFreshOnlinePresence(t *testing.T) {
	now := time.Unix(50_000, 0)
	settings := config.DefaultSharedGlobalSettings()
	settings.Presence.VisibilityOfflineDelaySeconds = 120
	s := &Server{deps: app.Dependencies{Config: config.Config{
		SharedGlobalSettings: settings,
	}}}

	gotStartedAt, gotDurationS := s.offlineFastForwardWindow(store.PresenceState{
		Status:     "online",
		LastSeenAt: now.Add(-119 * time.Second),
	}, now)
	if !gotStartedAt.IsZero() {
		t.Fatalf("startedAt=%v, want zero", gotStartedAt)
	}
	if gotDurationS != 0 {
		t.Fatalf("duration=%d, want 0", gotDurationS)
	}
}

func TestOfflineStartedAtForStaleOnlineUsesVisibilityGrace(t *testing.T) {
	now := time.Unix(55_000, 0)
	lastSeenAt := now.Add(-10 * time.Minute)
	settings := config.DefaultSharedGlobalSettings()
	settings.Presence.VisibilityOfflineDelaySeconds = 120
	s := &Server{deps: app.Dependencies{Config: config.Config{
		SharedGlobalSettings: settings,
	}}}

	gotStartedAt := s.offlineStartedAtForStaleOnline(lastSeenAt, now)
	wantStartedAt := lastSeenAt.Add(120 * time.Second)
	if !gotStartedAt.Equal(wantStartedAt) {
		t.Fatalf("startedAt=%v, want %v", gotStartedAt, wantStartedAt)
	}
}

func TestPresenceStatusTransitionWriteAtIgnoresFreshOnlineOffline(t *testing.T) {
	now := time.Unix(56_000, 0)
	settings := config.DefaultSharedGlobalSettings()
	settings.Presence.VisibilityOfflineDelaySeconds = 120
	s := &Server{deps: app.Dependencies{Config: config.Config{
		SharedGlobalSettings: settings,
	}}}

	gotAt, shouldWrite := s.presenceStatusTransitionWriteAt(store.PresenceState{
		Status:     "online",
		LastSeenAt: now.Add(-30 * time.Second),
	}, "offline", now)
	if shouldWrite {
		t.Fatalf("shouldWrite=true, want false")
	}
	if !gotAt.IsZero() {
		t.Fatalf("writeAt=%v, want zero", gotAt)
	}
}

func TestPresenceStatusTransitionWriteAtUsesGraceStartForStaleOnlineOffline(t *testing.T) {
	now := time.Unix(57_000, 0)
	lastSeenAt := now.Add(-10 * time.Minute)
	settings := config.DefaultSharedGlobalSettings()
	settings.Presence.VisibilityOfflineDelaySeconds = 120
	s := &Server{deps: app.Dependencies{Config: config.Config{
		SharedGlobalSettings: settings,
	}}}

	gotAt, shouldWrite := s.presenceStatusTransitionWriteAt(store.PresenceState{
		Status:     "online",
		LastSeenAt: lastSeenAt,
	}, "offline", now)
	wantAt := lastSeenAt.Add(120 * time.Second)
	if !shouldWrite {
		t.Fatalf("shouldWrite=false, want true")
	}
	if !gotAt.Equal(wantAt) {
		t.Fatalf("writeAt=%v, want %v", gotAt, wantAt)
	}
}

func TestMarkOfflineFastForwardFromPresenceUsesComputedWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	now := time.Unix(60_000, 0)
	lastSeenAt := now.Add(-10 * time.Minute)
	settings := config.DefaultSharedGlobalSettings()
	settings.Presence.VisibilityOfflineDelaySeconds = 120
	s := &Server{
		deps: app.Dependencies{
			Redis: rd,
			Config: config.Config{
				SharedGlobalSettings: settings,
				GlobalSettings: config.GlobalSettings{
					Activity: config.ActivitySettings{OfflineProgressMaxDurationSeconds: 3600},
				},
			},
		},
	}

	gotDurationS, err := s.markOfflineFastForwardFromPresence(42, store.PresenceState{
		Status:     "online",
		LastSeenAt: lastSeenAt,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if gotDurationS != int64((8*time.Minute)/time.Second) {
		t.Fatalf("duration=%d, want 480", gotDurationS)
	}
	ready, state := s.peekOfflineFastForward(42)
	if !ready {
		t.Fatalf("expected ready state")
	}
	wantStartedAt := lastSeenAt.Add(120 * time.Second)
	if got, want := state.OfflineStartedAtMS, wantStartedAt.UnixMilli(); got != want {
		t.Fatalf("startedAtMS=%d, want %d", got, want)
	}
	if got, want := state.OfflineDurationSeconds, gotDurationS; got != want {
		t.Fatalf("duration=%d, want %d", got, want)
	}
}
