package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/auth"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func TestAuthUserWithSessionReturnsAccountBanned(t *testing.T) {
	future := time.Now().Add(time.Hour)
	s := &Server{App: fiber.New(), deps: app.Dependencies{Config: config.Config{AuthDBTimeoutMS: 800}}}
	origParse := parseAuthClaimsWithSession
	origAuthGate := loadAuthGateState
	t.Cleanup(func() {
		parseAuthClaimsWithSession = origParse
		loadAuthGateState = origAuthGate
	})
	parseAuthClaimsWithSession = func(_ *Server, _ string) (auth.Claims, error) {
		return auth.Claims{UserID: 42, SessionVersion: 7}, nil
	}
	loadAuthGateState = func(_ *Server, _ context.Context, userID int64) (store.AuthGateState, error) {
		return store.AuthGateState{
			UserID:         userID,
			SessionVersion: 7,
			BanReason:      "abnormal_trade",
			BanUntil:       &future,
		}, nil
	}

	_, _, err := s.authUserWithSession(context.Background(), "token")
	if !errors.Is(err, errAuthAccountBanned) {
		t.Fatalf("err=%v, want account banned", err)
	}
}

func TestAuthUserWithSessionReturnsSessionRevoked(t *testing.T) {
	s := &Server{App: fiber.New(), deps: app.Dependencies{Config: config.Config{AuthDBTimeoutMS: 800}}}
	origParse := parseAuthClaimsWithSession
	origAuthGate := loadAuthGateState
	t.Cleanup(func() {
		parseAuthClaimsWithSession = origParse
		loadAuthGateState = origAuthGate
	})
	parseAuthClaimsWithSession = func(_ *Server, _ string) (auth.Claims, error) {
		return auth.Claims{UserID: 7, SessionVersion: 3}, nil
	}
	loadAuthGateState = func(_ *Server, _ context.Context, userID int64) (store.AuthGateState, error) {
		return store.AuthGateState{UserID: userID, SessionVersion: 4}, nil
	}

	_, _, err := s.authUserWithSession(context.Background(), "token")
	if !errors.Is(err, errAuthSessionRevoked) {
		t.Fatalf("err=%v, want session revoked", err)
	}
}

func TestAuthUserWithSessionReturnsBackendBusyOnAuthGateTimeout(t *testing.T) {
	s := &Server{App: fiber.New(), deps: app.Dependencies{Config: config.Config{AuthDBTimeoutMS: 1}}}
	origParse := parseAuthClaimsWithSession
	origAuthGate := loadAuthGateState
	t.Cleanup(func() {
		parseAuthClaimsWithSession = origParse
		loadAuthGateState = origAuthGate
	})
	parseAuthClaimsWithSession = func(_ *Server, _ string) (auth.Claims, error) {
		return auth.Claims{UserID: 7, SessionVersion: 3}, nil
	}
	loadAuthGateState = func(_ *Server, ctx context.Context, _ int64) (store.AuthGateState, error) {
		<-ctx.Done()
		return store.AuthGateState{}, ctx.Err()
	}

	_, _, err := s.authUserWithSession(context.Background(), "token")
	if !errors.Is(err, errAuthBackendBusy) {
		t.Fatalf("err=%v, want auth backend busy", err)
	}
}

func TestShouldDeduplicatePreauthenticatedWSBindRequiresFirstCompleteMatch(t *testing.T) {
	tests := []struct {
		name                                                         string
		preauthenticated                                             bool
		clientUserID, clientVersion, presenceUserID, presenceVersion int64
		nextUserID, nextVersion                                      int64
		want                                                         bool
	}{
		{name: "first matching bind after preauthentication", preauthenticated: true, clientUserID: 7, clientVersion: 3, presenceUserID: 7, presenceVersion: 3, nextUserID: 7, nextVersion: 3, want: true},
		{name: "later matching rebind", clientUserID: 7, clientVersion: 3, presenceUserID: 7, presenceVersion: 3, nextUserID: 7, nextVersion: 3},
		{name: "client not initialized", preauthenticated: true, presenceUserID: 7, presenceVersion: 3, nextUserID: 7, nextVersion: 3},
		{name: "presence not initialized", preauthenticated: true, clientUserID: 7, clientVersion: 3, nextUserID: 7, nextVersion: 3},
		{name: "different user", preauthenticated: true, clientUserID: 7, clientVersion: 3, presenceUserID: 7, presenceVersion: 3, nextUserID: 8, nextVersion: 3},
		{name: "different session", preauthenticated: true, clientUserID: 7, clientVersion: 3, presenceUserID: 7, presenceVersion: 3, nextUserID: 7, nextVersion: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldDeduplicatePreauthenticatedWSBind(tc.preauthenticated, tc.clientUserID, tc.clientVersion, tc.presenceUserID, tc.presenceVersion, tc.nextUserID, tc.nextVersion)
			if got != tc.want {
				t.Fatalf("shouldDeduplicatePreauthenticatedWSBind() = %t, want %t", got, tc.want)
			}
		})
	}
}
