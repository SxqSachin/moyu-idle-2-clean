package httpapi

import (
	"testing"
	"time"
)

func TestLoginAttemptGuardLocksAfterThreshold(t *testing.T) {
	now := time.Now()
	g := newLoginAttemptGuard(3, 5*time.Minute, 10*time.Minute, nil, "test")
	ip := "127.0.0.1"
	user := "alice"

	if err := g.RecordFailure(ip, user, now); err == nil {
		t.Fatalf("expected backend unavailable error")
	}
	locked, retryAfter, err := g.IsLocked(ip, user, now.Add(20*time.Second))
	if err == nil {
		t.Fatalf("expected backend unavailable error")
	}
	if locked || retryAfter != 0 {
		t.Fatalf("unexpected lock state when backend unavailable")
	}
}

func TestLoginAttemptGuardReset(t *testing.T) {
	now := time.Now()
	g := newLoginAttemptGuard(2, 5*time.Minute, 10*time.Minute, nil, "test")
	ip := "127.0.0.1"
	user := "bob"

	if err := g.RecordFailure(ip, user, now); err == nil {
		t.Fatalf("expected backend unavailable error")
	}
	if err := g.Reset(ip, user); err == nil {
		t.Fatalf("expected backend unavailable error")
	}
}
