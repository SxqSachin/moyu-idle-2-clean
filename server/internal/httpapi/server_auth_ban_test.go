package httpapi

import (
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"
)

func TestActiveBanStatePermanent(t *testing.T) {
	reason, until := activeBanState(store.UserAuthState{
		BanReason:      "abnormal_trade",
		BanIsPermanent: true,
	}, time.Now())
	if reason != "abnormal_trade" {
		t.Fatalf("reason=%q, want abnormal_trade", reason)
	}
	if !until.IsZero() {
		t.Fatalf("until should be zero for permanent ban")
	}
}

func TestActiveBanStateExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	reason, _ := activeBanState(store.UserAuthState{
		BanReason: "abnormal_trade",
		BanUntil:  &past,
	}, time.Now())
	if reason != "" {
		t.Fatalf("reason=%q, want empty", reason)
	}
}
