package httpapi

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestBattleCompileDispatchOriginMs(t *testing.T) {
	t.Parallel()

	roomUpdatedAt := time.UnixMilli(1_234)
	fallback := time.UnixMilli(9_876)
	if got := battleCompileDispatchOriginMs(store.BattleRoom{UpdatedAt: roomUpdatedAt}, fallback); got != roomUpdatedAt.UnixMilli() {
		t.Fatalf("compile dispatch origin=%d, want room update %d", got, roomUpdatedAt.UnixMilli())
	}
	if got := battleCompileDispatchOriginMs(store.BattleRoom{}, fallback); got != fallback.UnixMilli() {
		t.Fatalf("compile dispatch fallback=%d, want %d", got, fallback.UnixMilli())
	}
}

func TestBattleReadyDispatchNextSleep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pressure battleCompileQueuePressure
		started  int
		want     time.Duration
	}{
		{
			name:     "normal work keeps busy cadence",
			pressure: battleCompileQueuePressure{Slots: 4},
			started:  1,
			want:     battleV2ReadyDispatchBusySleep,
		},
		{
			name:     "pressured work keeps busy cadence",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 2},
			started:  1,
			want:     battleV2ReadyDispatchBusySleep,
		},
		{
			name:     "pending backlog work backs off",
			pressure: battleCompileQueuePressure{Slots: 4, Pending: 4},
			started:  1,
			want:     battleV2ReadyDispatchSleep,
		},
		{
			name:     "outstanding backlog work backs off",
			pressure: battleCompileQueuePressure{Slots: 4, InFlight: 8},
			started:  1,
			want:     battleV2ReadyDispatchSleep,
		},
		{
			name:     "idle normal loop uses regular cadence",
			pressure: battleCompileQueuePressure{Slots: 4},
			want:     battleV2ReadyDispatchSleep,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := battleReadyDispatchNextSleep(tc.pressure, tc.started); got != tc.want {
				t.Fatalf("ready dispatch sleep=%s, want %s", got, tc.want)
			}
		})
	}
}

func TestBattleReadyDispatchWorkerCountIsBounded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		roomCount int
		want      int
	}{
		{roomCount: 0, want: 0},
		{roomCount: 1, want: 1},
		{roomCount: 2, want: 2},
		{roomCount: 4, want: 2},
		{roomCount: 64, want: 2},
	}
	for _, tc := range tests {
		if got := battleReadyDispatchWorkerCount(tc.roomCount); got != tc.want {
			t.Fatalf("battleReadyDispatchWorkerCount(%d)=%d, want %d", tc.roomCount, got, tc.want)
		}
	}
}

func TestRunBattleReadyDispatchWorkersBoundsConcurrencyAndCountsSuccess(t *testing.T) {
	t.Parallel()

	rooms := []store.BattleRoomV2Claim{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}
	expectedWorkers := battleReadyDispatchWorkerCount(len(rooms))
	entered := make(chan struct{}, expectedWorkers)
	release := make(chan struct{})
	done := make(chan int, 1)
	var active atomic.Int32
	var maximum atomic.Int32
	calls := make([]atomic.Int32, len(rooms)+1)
	go func() {
		done <- runBattleReadyDispatchWorkers(rooms, func(room store.BattleRoomV2Claim) bool {
			calls[int(room.ID)].Add(1)
			current := active.Add(1)
			for {
				observed := maximum.Load()
				if current <= observed || maximum.CompareAndSwap(observed, current) {
					break
				}
			}
			if room.ID <= int64(expectedWorkers) {
				entered <- struct{}{}
				<-release
			}
			active.Add(-1)
			return room.ID != 3
		})
	}()
	timeout := time.After(2 * time.Second)
	for index := 0; index < expectedWorkers; index++ {
		select {
		case <-entered:
		case <-timeout:
			close(release)
			t.Fatal("workers did not reach the expected concurrency")
		}
	}
	close(release)
	select {
	case got := <-done:
		if got != 4 {
			t.Fatalf("started=%d, want 4", got)
		}
	case <-timeout:
		t.Fatal("ready-dispatch workers did not complete")
	}
	if got := maximum.Load(); got != int32(expectedWorkers) {
		t.Fatalf("maximum concurrency=%d, want %d", got, expectedWorkers)
	}
	for _, room := range rooms {
		if got := calls[int(room.ID)].Load(); got != 1 {
			t.Fatalf("room_id=%d calls=%d, want 1", room.ID, got)
		}
	}
}

func TestBattlePlaybackDispatchRecoveryUsesLiteReads(t *testing.T) {
	raw, err := os.ReadFile("battle_loop_v2.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Server) dispatchBattlePlaybackRoomV2")
	if start < 0 {
		t.Fatal("playback dispatch helper missing")
	}
	body = body[start:]
	if next := strings.Index(body, "\nfunc "); next >= 0 {
		body = body[:next]
	}
	for _, expected := range []string{"loadBattleRoomMemberIDsCached", "GetBattleRunByRoomLite", "HandoffBattleRoomPlaybackLease"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("playback recovery must use %s", expected)
		}
	}
	for _, forbidden := range []string{"loadBattleRoomDetailFresh", "GetBattleRunByID("} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("playback recovery must not use cold read %s", forbidden)
		}
	}
}
