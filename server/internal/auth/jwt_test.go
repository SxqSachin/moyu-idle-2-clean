package auth

import (
	"errors"
	"testing"
	"time"
)

func TestSignAndParse(t *testing.T) {
	svc := New("test-secret", time.Hour)

	token, err := svc.Sign(42, 3)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	userID, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if userID != 42 {
		t.Fatalf("expected userID 42, got %d", userID)
	}

	claims, err := svc.ParseWithSession(token)
	if err != nil {
		t.Fatalf("parse token with session: %v", err)
	}
	if claims.SessionVersion != 3 {
		t.Fatalf("expected session version 3, got %d", claims.SessionVersion)
	}
}

func TestParseRejectsInvalidToken(t *testing.T) {
	svc := New("test-secret", time.Hour)

	_, err := svc.Parse("bad-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
