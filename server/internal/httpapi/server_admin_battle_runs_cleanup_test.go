package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
)

func newAdminBattleRunsCleanupTestServer() *Server {
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{
				AppEnv:               "development",
				ReleaseAnnounceToken: "admin-token",
				GlobalSettings: config.GlobalSettings{
					Battle: config.BattleSettings{
						RunHistoryKeepCount: 10,
					},
				},
			},
		},
	}
	s.Post("/api/admin/battle/runs/cleanup", s.releaseAdminMiddleware(), s.handleAdminBattleRunsCleanup)
	return s
}

func TestAdminBattleRunsCleanupRequiresReleaseToken(t *testing.T) {
	s := newAdminBattleRunsCleanupTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/runs/cleanup", bytes.NewBufferString(`{"rounds":1}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminBattleRunsCleanupRejectsInvalidBatchSize(t *testing.T) {
	s := newAdminBattleRunsCleanupTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/runs/cleanup", bytes.NewBufferString(`{"batchSize":30000}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	payload := decodeAPIResponse[apiErrorResponse](t, resp)
	if payload.Code != "ADMIN_BATTLE_RUNS_CLEANUP_INVALID_BATCH_SIZE" {
		t.Fatalf("code=%s, want ADMIN_BATTLE_RUNS_CLEANUP_INVALID_BATCH_SIZE", payload.Code)
	}
}

func TestAdminBattleRunsCleanupSuccess(t *testing.T) {
	s := newAdminBattleRunsCleanupTestServer()
	original := adminBattleRunsCleanupExecutor
	t.Cleanup(func() { adminBattleRunsCleanupExecutor = original })
	var captured adminBattleRunsCleanupCommand
	adminBattleRunsCleanupExecutor = func(_ *Server, _ context.Context, cmd adminBattleRunsCleanupCommand) (int64, int, error) {
		captured = cmd
		return 1234, 3, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/runs/cleanup", bytes.NewBufferString(`{"keepCount":10,"batchSize":2000,"rounds":5}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	payload := decodeAPIResponse[apiSuccessResponse](t, resp)
	if !payload.OK {
		t.Fatalf("expected ok=true")
	}
	resultRaw, err := json.Marshal(payload.Data)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Deleted != 1234 {
		t.Fatalf("deleted=%d, want 1234", result.Deleted)
	}
	if captured.KeepCount != 10 || captured.BatchSize != 2000 || captured.Rounds != 5 {
		t.Fatalf("captured=%+v", captured)
	}
}
