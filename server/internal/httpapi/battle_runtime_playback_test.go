package httpapi

import (
	"testing"
	"time"
)

func TestNormalizeBattlePlaybackNextTickAtClampsFutureSchedule(t *testing.T) {
	server := &Server{}
	now := time.Unix(100, 0)
	got := server.normalizeBattlePlaybackNextTickAt(7, 9, now, now.Add(30*time.Second).UnixMilli())
	want := now.Add(battlePlaybackMaxFutureSleep)
	if !got.Equal(want) {
		t.Fatalf("normalizeBattlePlaybackNextTickAt()=%v, want %v", got, want)
	}
}
