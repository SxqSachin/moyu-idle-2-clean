package httpapi

import (
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestBattleRoomDetailCacheBypass(t *testing.T) {
	runID := int64(42)
	cases := []struct {
		name   string
		detail store.BattleRoomDetail
		want   bool
	}{
		{
			name: "idle room can use cache",
			detail: store.BattleRoomDetail{
				Room: store.BattleRoom{Status: store.BattleRoomStatusIdle},
			},
			want: false,
		},
		{
			name: "playing room bypasses cache",
			detail: store.BattleRoomDetail{
				Room: store.BattleRoom{Status: store.BattleRoomStatusPlaying},
			},
			want: true,
		},
		{
			name: "entering room bypasses cache",
			detail: store.BattleRoomDetail{
				Room: store.BattleRoom{Status: store.BattleRoomStatusEntering},
			},
			want: true,
		},
		{
			name: "recovering room bypasses cache",
			detail: store.BattleRoomDetail{
				Room: store.BattleRoom{Status: store.BattleRoomStatusRecovering},
			},
			want: true,
		},
		{
			name: "room with active run bypasses cache",
			detail: store.BattleRoomDetail{
				Room: store.BattleRoom{Status: store.BattleRoomStatusIdle, CurrentRunID: &runID},
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := battleRoomDetailCacheBypass(tc.detail); got != tc.want {
				t.Fatalf("battleRoomDetailCacheBypass()=%t want %t", got, tc.want)
			}
		})
	}
}
