package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
)

func newAdminBattleRoomsReinitializeTestServer(t *testing.T) *Server {
	t.Helper()
	miniRedis := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{
				AppEnv:               "development",
				ReleaseAnnounceToken: "admin-token",
			},
			GameData: catalog.Data{SpecializationTrees: []catalog.SpecializationTree{{
				ID: "player_combat_specialization", SchemaVersion: 2, RulesetVersion: "specialization-v5", BalanceRevision: 7, OwnerType: "player",
			}}},
			Redis: redisClient,
		},
	}
	s.publishBattleWorkerHeartbeat("worker-1", time.Now())
	s.Post("/api/admin/battle/rooms/reinitialize", s.releaseAdminMiddleware(), s.handleAdminBattleRoomsReinitialize)
	return s
}

func TestAdminBattleRoomsReinitializeRejectsStaleSpecializationRevision(t *testing.T) {
	s := newAdminBattleRoomsReinitializeTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/rooms/reinitialize", bytes.NewBufferString(`{
		"expectedSpecialization":{"treeId":"player_combat_specialization","rulesetVersion":"specialization-v5","balanceRevision":6}
	}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusConflict)
	}
	payload := decodeAPIResponse[apiErrorResponse](t, resp)
	if payload.Code != "ADMIN_BATTLE_REINIT_SPECIALIZATION_CONFLICT" {
		t.Fatalf("code=%s, want ADMIN_BATTLE_REINIT_SPECIALIZATION_CONFLICT", payload.Code)
	}
}

func TestAdminBattleRoomsReinitializeRequiresReleaseToken(t *testing.T) {
	s := newAdminBattleRoomsReinitializeTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/rooms/reinitialize", bytes.NewBufferString(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminBattleRoomsReinitializeRejectsInvalidStatuses(t *testing.T) {
	s := newAdminBattleRoomsReinitializeTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/rooms/reinitialize", bytes.NewBufferString(`{"statuses":["playing","idle"]}`))
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
	if payload.Code != "ADMIN_BATTLE_REINIT_INVALID_STATUSES" {
		t.Fatalf("code=%s, want ADMIN_BATTLE_REINIT_INVALID_STATUSES", payload.Code)
	}
}

func TestAdminBattleRoomsReinitializeSuccess(t *testing.T) {
	s := newAdminBattleRoomsReinitializeTestServer(t)
	original := adminBattleRoomsReinitializeExecutor
	t.Cleanup(func() { adminBattleRoomsReinitializeExecutor = original })
	var captured adminBattleRoomsReinitializeCommand
	adminBattleRoomsReinitializeExecutor = func(_ *Server, _ context.Context, cmd adminBattleRoomsReinitializeCommand) (int64, int64, error) {
		captured = cmd
		return 7, 5, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/rooms/reinitialize", bytes.NewBufferString(`{
		"statuses":["playing","recovering"],
		"expectedSpecialization":{"treeId":"player_combat_specialization","rulesetVersion":"specialization-v5","balanceRevision":7}
	}`))
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
		ReinitRooms     int64                              `json:"reinitRooms"`
		InterruptedRuns int64                              `json:"interruptedRuns"`
		RunStatus       string                             `json:"runStatus"`
		Statuses        []string                           `json:"statuses"`
		Specialization  specializationRuntimeVersion       `json:"specialization"`
		BattleWorkers   battleWorkerSpecializationSnapshot `json:"battleWorkers"`
	}
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ReinitRooms != 7 || result.InterruptedRuns != 5 {
		t.Fatalf("result=%+v", result)
	}
	if result.RunStatus != "interrupted" || result.Specialization.BalanceRevision != 7 || !result.BattleWorkers.Ready || result.BattleWorkers.MatchingWorkers != 1 {
		t.Fatalf("operational metadata=%+v", result)
	}
	if len(captured.Statuses) != 2 || captured.Statuses[0] != "playing" || captured.Statuses[1] != "recovering" {
		t.Fatalf("captured=%+v", captured)
	}
}

func TestAdminBattleRoomsReinitializeRejectsMismatchedBattleWorker(t *testing.T) {
	s := newAdminBattleRoomsReinitializeTestServer(t)
	mismatched := `{"workerId":"worker-2","specialization":{"treeId":"player_combat_specialization","rulesetVersion":"specialization-v5","balanceRevision":6}}`
	if err := s.deps.Redis.Set(context.Background(), s.battleWorkerHeartbeatKey("worker-2"), mismatched, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/battle/rooms/reinitialize", bytes.NewBufferString(`{
		"expectedSpecialization":{"treeId":"player_combat_specialization","rulesetVersion":"specialization-v5","balanceRevision":7}
	}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusConflict)
	}
	payload := decodeAPIResponse[apiErrorResponse](t, resp)
	if payload.Code != "ADMIN_BATTLE_REINIT_WORKERS_CONFLICT" {
		t.Fatalf("code=%s, want ADMIN_BATTLE_REINIT_WORKERS_CONFLICT", payload.Code)
	}
}
