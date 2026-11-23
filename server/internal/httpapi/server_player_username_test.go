package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/ws"
)

type playerUsernameRenamerFunc func(context.Context, int64, string, string) (store.PlayerUsernameRenameResult, error)

func (fn playerUsernameRenamerFunc) RenamePlayerUsername(ctx context.Context, userID int64, username string, expectedCurrentUsername string) (store.PlayerUsernameRenameResult, error) {
	return fn(ctx, userID, username, expectedCurrentUsername)
}

type playerUsernameHTTPResponse struct {
	OK    bool `json:"ok"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
	Data struct {
		Username         string    `json:"username"`
		PreviousUsername string    `json:"previousUsername"`
		ChangedAt        time.Time `json:"changedAt"`
		ChangeID         int64     `json:"changeId"`
	} `json:"data"`
}

func newPlayerUsernameTestServer(renamer playerUsernameRenamer) *Server {
	const userID int64 = 7
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{AppEnv: "development"},
			Hub:    ws.NewHub(),
		},
		wsSubject:         "test.ws.broadcast",
		mutationSyncQueue: make(chan mutationSyncScope, 1),
		playerProfileMem:  map[int64]playerProfileCacheEntry{userID: {}},
		playerPublicMem:   map[int64]playerProfileCacheEntry{userID: {}},
	}
	s.Post("/api/player/profile/username", func(c *fiber.Ctx) error {
		c.Locals("authUserID", userID)
		return s.handlePlayerProfileUsernameWithStore(c, renamer)
	})
	return s
}

func performPlayerUsernameRequest(t *testing.T, s *Server, body string) (*http.Response, playerUsernameHTTPResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/player/profile/username", bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	var payload playerUsernameHTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp, payload
}

func TestHandlePlayerProfileUsernameSuccessSynchronizesCachesAndInventory(t *testing.T) {
	changedAt := time.Date(2026, time.August, 27, 14, 0, 0, 0, time.UTC)
	var capturedUserID int64
	var capturedUsername string
	var capturedExpectedCurrentUsername string
	renamer := playerUsernameRenamerFunc(func(_ context.Context, userID int64, username string, expectedCurrentUsername string) (store.PlayerUsernameRenameResult, error) {
		capturedUserID = userID
		capturedUsername = username
		capturedExpectedCurrentUsername = expectedCurrentUsername
		return store.PlayerUsernameRenameResult{Username: username, PreviousUsername: "旧名字", ChangedAt: changedAt, HistoryID: 123}, nil
	})
	s := newPlayerUsernameTestServer(renamer)

	resp, payload := performPlayerUsernameRequest(t, s, `{"username":"  新名字  ","expectedCurrentUsername":"  旧名字  "}`)

	if resp.StatusCode != http.StatusOK || !payload.OK {
		t.Fatalf("status=%d payload=%#v", resp.StatusCode, payload)
	}
	if capturedUserID != 7 || capturedUsername != "新名字" {
		t.Fatalf("rename args userID=%d username=%q", capturedUserID, capturedUsername)
	}
	if capturedExpectedCurrentUsername != "旧名字" {
		t.Fatalf("expected current username=%v, want 旧名字", capturedExpectedCurrentUsername)
	}
	if payload.Data.Username != "新名字" || payload.Data.PreviousUsername != "旧名字" || !payload.Data.ChangedAt.Equal(changedAt) || payload.Data.ChangeID != 123 {
		t.Fatalf("unexpected response data: %#v", payload.Data)
	}
	if _, ok := s.playerProfileMem[7]; ok {
		t.Fatal("profile cache was not invalidated")
	}
	if _, ok := s.playerPublicMem[7]; ok {
		t.Fatal("public profile cache was not invalidated")
	}
	select {
	case scope := <-s.mutationSyncQueue:
		if len(scope.UserIDs) != 1 || scope.UserIDs[0] != 7 || !scope.Inventory {
			t.Fatalf("unexpected mutation sync scope: %#v", scope)
		}
	default:
		t.Fatal("inventory mutation sync was not queued")
	}
}

func TestHandlePlayerProfileUsernameMapsStoreErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "current username conflict", err: store.ErrPlayerUsernameConflict, wantStatus: http.StatusConflict, wantCode: codeProfileUsernameCurrentConflict},
		{name: "unchanged", err: store.ErrPlayerUsernameUnchanged, wantStatus: http.StatusBadRequest, wantCode: codeProfileUsernameSame},
		{name: "duplicate", err: store.ErrDuplicateUser, wantStatus: http.StatusConflict, wantCode: codeProfileUsernameExists},
		{name: "insufficient item", err: store.ErrInsufficientItem, wantStatus: http.StatusConflict, wantCode: codeProfileUsernameInsufficientItem},
		{name: "invalid store boundary", err: store.ErrPlayerUsernameInvalid, wantStatus: http.StatusBadRequest, wantCode: codeProfileUsernameFormat},
		{name: "invalid rename config", err: store.ErrPlayerUsernameRenameCostInvalid, wantStatus: http.StatusInternalServerError, wantCode: codeProfileUsernameConfigInvalid},
		{name: "missing player", err: store.ErrUserNotFound, wantStatus: http.StatusNotFound, wantCode: codeProfileUsernameUserNotFound},
		{name: "unexpected", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: codeProfileUpdateFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renamer := playerUsernameRenamerFunc(func(context.Context, int64, string, string) (store.PlayerUsernameRenameResult, error) {
				return store.PlayerUsernameRenameResult{}, test.err
			})
			s := newPlayerUsernameTestServer(renamer)

			resp, payload := performPlayerUsernameRequest(t, s, `{"username":"ValidName","expectedCurrentUsername":"CurrentName"}`)

			if resp.StatusCode != test.wantStatus || payload.Error.Code != test.wantCode {
				t.Fatalf("status=%d code=%q, want status=%d code=%q", resp.StatusCode, payload.Error.Code, test.wantStatus, test.wantCode)
			}
			if len(s.mutationSyncQueue) != 0 {
				t.Fatal("failed rename unexpectedly queued inventory sync")
			}
		})
	}
}

func TestHandlePlayerProfileUsernameValidatesBeforeStore(t *testing.T) {
	called := false
	renamer := playerUsernameRenamerFunc(func(context.Context, int64, string, string) (store.PlayerUsernameRenameResult, error) {
		called = true
		return store.PlayerUsernameRenameResult{}, nil
	})
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "malformed json", body: `{"username":`, wantStatus: http.StatusBadRequest, wantCode: codeProfileInvalidPayload},
		{name: "too short", body: `{"username":"a"}`, wantStatus: http.StatusBadRequest, wantCode: codeProfileUsernameTooShort},
		{name: "too long", body: `{"username":"1234567890123456789012345"}`, wantStatus: http.StatusBadRequest, wantCode: codeProfileUsernameTooLong},
		{name: "invalid format", body: `{"username":"name-with-dash"}`, wantStatus: http.StatusBadRequest, wantCode: codeProfileUsernameFormat},
		{name: "missing expected current username", body: `{"username":"ValidName"}`, wantStatus: http.StatusBadRequest, wantCode: codeProfileInvalidPayload},
		{name: "empty expected current username", body: `{"username":"ValidName","expectedCurrentUsername":"  "}`, wantStatus: http.StatusBadRequest, wantCode: codeProfileInvalidPayload},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called = false
			s := newPlayerUsernameTestServer(renamer)
			resp, payload := performPlayerUsernameRequest(t, s, test.body)
			if resp.StatusCode != test.wantStatus || payload.Error.Code != test.wantCode {
				t.Fatalf("status=%d code=%q, want status=%d code=%q", resp.StatusCode, payload.Error.Code, test.wantStatus, test.wantCode)
			}
			if called {
				t.Fatal("invalid request reached the store")
			}
		})
	}
}

func TestPlayerUsernameRevisionIsExposedByAuthoritativeSnapshots(t *testing.T) {
	checks := []struct {
		path   string
		needle string
	}{
		{path: "server_player_settings.go", needle: `"usernameChangeId":     settings.UsernameChangeID`},
		{path: "server_bootstrap.go", needle: `"usernameChangeId": settings.UsernameChangeID`},
		{path: "server_player_public.go", needle: `"usernameChangeId":  usernameSnapshot.ChangeID`},
		{path: "server_player_profile.go", needle: `"usernameChangeId":      usernameSnapshot.ChangeID`},
	}
	for _, check := range checks {
		raw, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		if !strings.Contains(string(raw), check.needle) {
			t.Fatalf("%s is missing %q", check.path, check.needle)
		}
	}
}
