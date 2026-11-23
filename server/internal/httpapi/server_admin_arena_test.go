package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func newAdminArenaInitializeTestServer() *Server {
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{Config: config.Config{
			AppEnv:               "development",
			ReleaseAnnounceToken: "admin-token",
			GlobalSettings: config.GlobalSettings{
				Arena: config.ArenaSettings{Enabled: true},
			},
		}},
	}
	s.Post("/api/admin/arena/defenses/initialize", s.releaseAdminMiddleware(), s.handleAdminArenaDefensesInitialize)
	return s
}

func TestAdminArenaDefensesInitializeRequiresReleaseToken(t *testing.T) {
	s := newAdminArenaInitializeTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/arena/defenses/initialize", nil)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminArenaDefensesInitializeRunsWithoutRequestBody(t *testing.T) {
	s := newAdminArenaInitializeTestServer()
	original := adminArenaDefenseInitializeExecutor
	t.Cleanup(func() { adminArenaDefenseInitializeExecutor = original })
	called := false
	activeSince := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	adminArenaDefenseInitializeExecutor = func(_ *Server, _ context.Context) (adminArenaDefenseInitializeResult, error) {
		called = true
		return adminArenaDefenseInitializeResult{
			ActiveSince:      activeSince,
			ActiveWithinDays: 14,
			BatchSize:        adminArenaDefenseInitializationBatchSize,
			Batches:          2,
			Scanned:          10,
			Initialized:      9,
			SkippedExisting:  1,
		}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/arena/defenses/initialize", nil)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !called {
		t.Fatal("initializer was not called")
	}
	payload := decodeAPIResponse[apiSuccessResponse](t, resp)
	resultRaw, err := json.Marshal(payload.Data)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result adminArenaDefenseInitializeResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Batches != 2 || result.Initialized != 9 || result.SkippedExisting != 1 {
		t.Fatalf("result=%+v", result)
	}
}

func newAdminArenaLeaderboardResetTestServer() *Server {
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{Config: config.Config{
			AppEnv:               "development",
			ReleaseAnnounceToken: "admin-token",
			GlobalSettings: config.GlobalSettings{
				Arena: config.ArenaSettings{Enabled: true, DefaultRating: 1200},
			},
		}},
	}
	s.Post("/api/admin/arena/leaderboard/reset", s.releaseAdminMiddleware(), s.handleAdminArenaLeaderboardReset)
	return s
}

func TestAdminArenaLeaderboardResetRequiresReleaseToken(t *testing.T) {
	s := newAdminArenaLeaderboardResetTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/arena/leaderboard/reset", nil)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminArenaLeaderboardResetUsesConfiguredDefaultRating(t *testing.T) {
	s := newAdminArenaLeaderboardResetTestServer()
	original := adminArenaLeaderboardResetExecutor
	t.Cleanup(func() { adminArenaLeaderboardResetExecutor = original })
	resetAt := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	adminArenaLeaderboardResetExecutor = func(_ *Server, _ context.Context, defaultRating int64) (store.ArenaLeaderboardResetResult, error) {
		if defaultRating != 1200 {
			t.Fatalf("defaultRating=%d, want 1200", defaultRating)
		}
		return store.ArenaLeaderboardResetResult{
			ResetPlayers:  37,
			DefaultRating: defaultRating,
			ResetAt:       resetAt,
		}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/arena/leaderboard/reset", nil)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	payload := decodeAPIResponse[apiSuccessResponse](t, resp)
	resultRaw, err := json.Marshal(payload.Data)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result store.ArenaLeaderboardResetResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ResetPlayers != 37 || result.DefaultRating != 1200 || !result.ResetAt.Equal(resetAt) {
		t.Fatalf("result=%+v", result)
	}
}

func TestRunAdminArenaDefenseInitializationProcessesEveryInternalPage(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	var cursors []int64
	listCandidates := func(_ context.Context, activeSince time.Time, afterUserID int64, limit int) ([]int64, bool, error) {
		if want := now.Add(-adminArenaDefenseInitializationWindow); !activeSince.Equal(want) {
			t.Fatalf("activeSince=%s, want %s", activeSince, want)
		}
		if limit != adminArenaDefenseInitializationBatchSize {
			t.Fatalf("limit=%d, want %d", limit, adminArenaDefenseInitializationBatchSize)
		}
		cursors = append(cursors, afterUserID)
		switch afterUserID {
		case 0:
			return []int64{11, 22}, true, nil
		case 22:
			return []int64{33}, false, nil
		default:
			t.Fatalf("unexpected cursor %d", afterUserID)
			return nil, false, nil
		}
	}
	var initializedUserIDs []int64
	initializeUser := func(_ context.Context, userID int64, initializedAt time.Time) (bool, error) {
		if !initializedAt.Equal(now) {
			t.Fatalf("initializedAt=%s, want %s", initializedAt, now)
		}
		initializedUserIDs = append(initializedUserIDs, userID)
		return userID != 22, nil
	}

	result, err := runAdminArenaDefenseInitialization(context.Background(), now, listCandidates, initializeUser)
	if err != nil {
		t.Fatalf("run initialization: %v", err)
	}
	if len(cursors) != 2 || cursors[0] != 0 || cursors[1] != 22 {
		t.Fatalf("cursors=%v", cursors)
	}
	if len(initializedUserIDs) != 3 || initializedUserIDs[0] != 11 || initializedUserIDs[1] != 22 || initializedUserIDs[2] != 33 {
		t.Fatalf("initializedUserIDs=%v", initializedUserIDs)
	}
	if result.Batches != 2 || result.Scanned != 3 || result.Initialized != 2 || result.SkippedExisting != 1 {
		t.Fatalf("result=%+v", result)
	}
}

func TestDefaultArenaLineupSelectionUsesTwoUnlockedKitties(t *testing.T) {
	selection := defaultArenaLineupSelection([]arenaDefaultKittyCandidate{{KittyID: 1}, {KittyID: 2}})
	if selection.PlayerPosition != "vanguard" || len(selection.Kitties) != 2 {
		t.Fatalf("selection=%+v", selection)
	}
	if selection.Kitties[0].Position != "midguard" || selection.Kitties[1].Position != "rearguard" {
		t.Fatalf("selection=%+v", selection)
	}
}

func TestDefaultArenaLineupSelectionAllowsPlayerOnly(t *testing.T) {
	selection := defaultArenaLineupSelection(nil)
	if selection.PlayerPosition != "vanguard" || len(selection.Kitties) != 0 {
		t.Fatalf("selection=%+v", selection)
	}
}

func TestDefaultArenaLineupSelectionKeepsSameRowLockedPair(t *testing.T) {
	selection := defaultArenaLineupSelection([]arenaDefaultKittyCandidate{
		{KittyID: 1, PositionLocked: "midguard"},
		{KittyID: 2, PositionLocked: "midguard"},
		{KittyID: 3, PositionLocked: "rearguard"},
	})
	if len(selection.Kitties) != 2 || selection.Kitties[0].KittyID != 1 || selection.Kitties[1].KittyID != 2 {
		t.Fatalf("selection=%+v", selection)
	}
	if selection.Kitties[0].Position != "midguard" || selection.Kitties[1].Position != "midguard" {
		t.Fatalf("selection did not preserve same-row locks: %+v", selection)
	}
}

func TestDefaultArenaLineupSelectionAllowsPlayerAndKittiesInSameRow(t *testing.T) {
	selection := defaultArenaLineupSelection([]arenaDefaultKittyCandidate{
		{KittyID: 1, PositionLocked: "vanguard"},
		{KittyID: 2, PositionLocked: "vanguard"},
	})
	if len(selection.Kitties) != 2 || selection.Kitties[0].KittyID != 1 || selection.Kitties[1].KittyID != 2 {
		t.Fatalf("selection=%+v", selection)
	}
	if selection.PlayerPosition != "vanguard" || selection.Kitties[0].Position != "vanguard" || selection.Kitties[1].Position != "vanguard" {
		t.Fatalf("selection=%+v", selection)
	}
}
