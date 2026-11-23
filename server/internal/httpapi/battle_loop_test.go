package httpapi

import (
	"os"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestBattleLoopStartsV2Only(t *testing.T) {
	raw, err := os.ReadFile("battle_loop.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Server) startBattleLoop()")
	if start < 0 {
		t.Fatal("battle loop entrypoint missing")
	}
	body = body[start:]
	if next := strings.Index(body, "\nfunc (s *Server) startBattleWorkerHeartbeatLoop"); next >= 0 {
		body = body[:next]
	}
	if !strings.Contains(body, "s.startBattleV2Loops(workerID)") {
		t.Fatal("battle worker no longer starts V2 dispatch")
	}
	if strings.Contains(body, "startBattleLegacyLoops") || strings.Contains(body, "HasBattleRoomsV1") {
		t.Fatal("battle worker entrypoint must not start or probe V1 dispatch")
	}
}

func TestBattleRunStartSlowLogUsesLatenessBeyondExpectedDelay(t *testing.T) {
	recoveryExpected := expectedBattleRunStartGapMs(battleCompileSourceRecoveryAutoReenter, 3_000, 10_000)
	if recoveryExpected != 10_000 {
		t.Fatalf("recovery expected gap=%d, want 10000", recoveryExpected)
	}
	if late := battleRunStartLateMs(10_250, recoveryExpected); late != 250 || shouldLogBattleRunStart(late, -1) {
		t.Fatalf("normal recovery late=%d should not log as slow", late)
	}
	if late := battleRunStartLateMs(15_000, recoveryExpected); late != 5_000 || !shouldLogBattleRunStart(late, -1) {
		t.Fatalf("delayed recovery late=%d should log as slow", late)
	}

	readyExpected := expectedBattleRunStartGapMs(battleCompileSourceReadyDispatch, 3_000, 10_000)
	if readyExpected != 3_000 {
		t.Fatalf("ready expected gap=%d, want 3000", readyExpected)
	}
	if !shouldLogBattleRunStart(0, 4_500) {
		t.Fatal("ready wait at threshold should log independently of finish gap")
	}
}

func TestBattleRoomUsesLegacyDueQueue(t *testing.T) {
	if battleRoomUsesLegacyDueQueue(store.BattleRoom{SchemaVersion: BattleRoomSchemaV2}) {
		t.Fatal("v2 room must not use the legacy Redis due index")
	}
	if !battleRoomUsesLegacyDueQueue(store.BattleRoom{SchemaVersion: BattleRoomSchemaV1}) {
		t.Fatal("v1 room must keep using the legacy Redis due index")
	}
	if !battleRoomUsesLegacyDueQueue(store.BattleRoom{}) {
		t.Fatal("unknown room version must fail safe to the legacy Redis due index")
	}
}

func TestBattleTickShardWorkerCount(t *testing.T) {
	tests := []struct {
		name          string
		roomCount     int
		configuredMax int
		want          int
	}{
		{name: "empty rooms", roomCount: 0, configuredMax: 8, want: 1},
		{name: "uses configured max", roomCount: 40, configuredMax: 12, want: 12},
		{name: "caps by room count", roomCount: 3, configuredMax: 12, want: 3},
		{name: "falls back to default when config invalid", roomCount: 20, configuredMax: 0, want: 8},
		{name: "fallback still caps by room count", roomCount: 5, configuredMax: -1, want: 5},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := battleTickShardWorkerCount(tc.roomCount, tc.configuredMax)
			if got != tc.want {
				t.Fatalf("battleTickShardWorkerCount(%d, %d) = %d, want %d", tc.roomCount, tc.configuredMax, got, tc.want)
			}
		})
	}
}

func TestBattlePlaybackLeaseFallbackWindow(t *testing.T) {
	if battlePlaybackLeaseTTL > 4*time.Second {
		t.Fatalf("battlePlaybackLeaseTTL=%s, want <=4s so playback fallback does not stall rooms too long", battlePlaybackLeaseTTL)
	}
	if battlePlaybackLeaseRenewThreshold >= battlePlaybackLeaseTTL {
		t.Fatalf("battlePlaybackLeaseRenewThreshold=%s must be lower than ttl=%s", battlePlaybackLeaseRenewThreshold, battlePlaybackLeaseTTL)
	}
}

func TestBattleTickClaimRoomLimit(t *testing.T) {
	policy := battleTickClaimPolicy{
		MinRooms:           32,
		MaxRooms:           300,
		TargetLeaseUtilPct: 60,
	}
	lease := 3 * time.Second
	tests := []struct {
		name           string
		execPerRoom    time.Duration
		wantClaimLimit int
	}{
		{name: "cold start uses max", execPerRoom: 0, wantClaimLimit: 300},
		{name: "high load shrinks to min", execPerRoom: 200 * time.Millisecond, wantClaimLimit: 32},
		{name: "mid load computes dynamic", execPerRoom: 20 * time.Millisecond, wantClaimLimit: 90},
		{name: "low load caps at max", execPerRoom: 2 * time.Millisecond, wantClaimLimit: 300},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := battleTickClaimRoomLimit(policy, lease, tc.execPerRoom)
			if got != tc.wantClaimLimit {
				t.Fatalf("battleTickClaimRoomLimit(...) = %d, want %d", got, tc.wantClaimLimit)
			}
		})
	}
}

func TestBattleTickUpdateExecPerRoomEWMA(t *testing.T) {
	tests := []struct {
		name        string
		prev        time.Duration
		execElapsed time.Duration
		rooms       int
		want        time.Duration
	}{
		{name: "cold start uses sample", prev: 0, execElapsed: 200 * time.Millisecond, rooms: 10, want: 20 * time.Millisecond},
		{name: "updates by alpha", prev: 20 * time.Millisecond, execElapsed: 400 * time.Millisecond, rooms: 10, want: 26 * time.Millisecond},
		{name: "invalid sample keeps prev", prev: 15 * time.Millisecond, execElapsed: 0, rooms: 10, want: 15 * time.Millisecond},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := battleTickUpdateExecPerRoomEWMA(tc.prev, tc.execElapsed, tc.rooms)
			if got != tc.want {
				t.Fatalf("battleTickUpdateExecPerRoomEWMA(%s, %s, %d) = %s, want %s", tc.prev, tc.execElapsed, tc.rooms, got, tc.want)
			}
		})
	}
}
