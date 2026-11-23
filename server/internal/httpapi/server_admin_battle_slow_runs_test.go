package httpapi

import (
	"bytes"
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

func newAdminBattleSlowRunsTestServer() *Server {
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{
				AppEnv:               "development",
				ReleaseAnnounceToken: "admin-token",
			},
		},
	}
	s.Get("/api/admin/battle/slow-runs", s.releaseAdminMiddleware(), s.handleAdminBattleSlowRunsList)
	s.Get("/api/admin/battle/slow-runs/:archiveID", s.releaseAdminMiddleware(), s.handleAdminBattleSlowRunGet)
	s.Post("/api/admin/battle/slow-runs/cleanup", s.releaseAdminMiddleware(), s.handleAdminBattleSlowRunsCleanup)
	return s
}

func TestAdminBattleSlowRunsListRejectsInvalidLimit(t *testing.T) {
	s := newAdminBattleSlowRunsTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/battle/slow-runs?limit=oops", nil)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	payload := decodeAPIResponse[apiErrorResponse](t, resp)
	if payload.Code != "ADMIN_BATTLE_SLOW_RUNS_INVALID_LIMIT" {
		t.Fatalf("code=%s, want ADMIN_BATTLE_SLOW_RUNS_INVALID_LIMIT", payload.Code)
	}
}

func TestAdminBattleSlowRunsListSuccess(t *testing.T) {
	s := newAdminBattleSlowRunsTestServer()
	original := adminBattleSlowRunsListExecutor
	t.Cleanup(func() { adminBattleSlowRunsListExecutor = original })
	now := time.Unix(100, 0).UTC()
	adminBattleSlowRunsListExecutor = func(_ *Server, _ context.Context, filter store.BattleSlowRunArchiveFilter) ([]store.BattleSlowRunArchiveSummary, error) {
		if filter.RoomID != 12 || filter.RunID != 34 || filter.BeforeID != 56 || filter.Limit != 10 {
			t.Fatalf("unexpected filter: %+v", filter)
		}
		return []store.BattleSlowRunArchiveSummary{
			{
				ID:                88,
				RoomID:            12,
				RunID:             34,
				DungeonID:         "base_2",
				WorkerID:          "worker-1",
				Source:            "ready_dispatch",
				Wave:              7,
				Outcome:           "failed",
				QueueSinceStartMs: 1200,
				CompileElapsedMs:  800,
				TotalMs:           1600,
				TotalSteps:        100,
				TotalTurns:        90,
				TimelineEvents:    101,
				FinalSeq:          101,
				PlayerUnits:       1,
				GuardStepLimited:  true,
				TriggerFlags:      map[string]any{"queueSlow": true, "compileSlow": true, "guardStepLimited": true},
				TurnProfile:       map[string]any{"preActionMs": int64(5), "executeMs": int64(12), "postActionMs": int64(3)},
				RunStartedAt:      &now,
				CapturedAt:        now,
			},
		}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/battle/slow-runs?roomId=12&runId=34&beforeId=56&limit=10", nil)
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
	var result struct {
		Items []struct {
			ID       int64 `json:"id"`
			Analysis struct {
				PrimaryCause string `json:"primaryCause"`
			} `json:"analysis"`
		} `json:"items"`
		NextBeforeID int64 `json:"nextBeforeId"`
	}
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 88 {
		t.Fatalf("unexpected items: %+v", result.Items)
	}
	if result.Items[0].Analysis.PrimaryCause != "mixed" {
		t.Fatalf("primaryCause=%q, want mixed", result.Items[0].Analysis.PrimaryCause)
	}
	if result.NextBeforeID != 88 {
		t.Fatalf("nextBeforeId=%d, want 88", result.NextBeforeID)
	}
}

func TestAdminBattleSlowRunGetNotFound(t *testing.T) {
	s := newAdminBattleSlowRunsTestServer()
	original := adminBattleSlowRunsGetExecutor
	t.Cleanup(func() { adminBattleSlowRunsGetExecutor = original })
	adminBattleSlowRunsGetExecutor = func(_ *Server, _ context.Context, _ int64) (store.BattleSlowRunArchive, error) {
		return store.BattleSlowRunArchive{}, store.ErrBattleSlowRunArchiveNotFound
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/battle/slow-runs/77", nil)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAdminBattleSlowRunsCleanupSuccess(t *testing.T) {
	s := newAdminBattleSlowRunsTestServer()
	original := adminBattleSlowRunsCleanupExecutor
	t.Cleanup(func() { adminBattleSlowRunsCleanupExecutor = original })
	var captured adminBattleSlowRunsCleanupCommand
	adminBattleSlowRunsCleanupExecutor = func(_ *Server, _ context.Context, cmd adminBattleSlowRunsCleanupCommand) (int64, int, error) {
		captured = cmd
		return 9, 2, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/slow-runs/cleanup", bytes.NewBufferString(`{"keepDays":30,"batchSize":2000,"rounds":3}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	if captured.KeepDays != 30 || captured.BatchSize != 2000 || captured.Rounds != 3 {
		t.Fatalf("captured=%+v", captured)
	}
}
