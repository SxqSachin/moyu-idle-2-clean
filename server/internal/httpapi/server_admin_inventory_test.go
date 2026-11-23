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

func newAdminInventoryGrantTestServer() *Server {
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{
				AppEnv:               "development",
				ReleaseAnnounceToken: "admin-token",
			},
		},
		mutationSyncQueue: make(chan mutationSyncScope, 8),
	}
	s.Post("/api/admin/inventory/grant", s.releaseAdminMiddleware(), s.handleAdminInventoryGrant)
	return s
}

func decodeAPIResponse[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var payload T
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func TestAdminInventoryGrantRequiresReleaseToken(t *testing.T) {
	s := newAdminInventoryGrantTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/grant", bytes.NewBufferString(`{"userId":1,"itemId":"gold_coin","quantity":1}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	payload := decodeAPIResponse[apiErrorResponse](t, resp)
	if payload.Code != "RELEASE_ANNOUNCE_UNAUTHORIZED" {
		t.Fatalf("code=%s, want RELEASE_ANNOUNCE_UNAUTHORIZED", payload.Code)
	}
}

func TestAdminInventoryGrantRejectsInvalidGrantMode(t *testing.T) {
	s := newAdminInventoryGrantTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/grant", bytes.NewBufferString(`{"userId":1,"itemId":"gold_coin","quantity":1,"grantMode":"instance"}`))
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
	if payload.Code != "ADMIN_INVENTORY_GRANT_INVALID_GRANT_MODE" {
		t.Fatalf("code=%s, want ADMIN_INVENTORY_GRANT_INVALID_GRANT_MODE", payload.Code)
	}
}

func TestAdminInventoryGrantSuccessWithEnhancedMode(t *testing.T) {
	s := newAdminInventoryGrantTestServer()
	originalExecutor := adminInventoryGrantExecutor
	t.Cleanup(func() { adminInventoryGrantExecutor = originalExecutor })
	var captured adminInventoryGrantCommand
	adminInventoryGrantExecutor = func(_ *Server, _ context.Context, cmd adminInventoryGrantCommand) error {
		captured = cmd
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/inventory/grant", bytes.NewBufferString(`{"userId":7,"itemId":"wood_sword","quantity":2,"grantMode":"enhanced","enhanceLevel":4}`))
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
	if captured.UserID != 7 || captured.ItemID != "wood_sword" || captured.Quantity != 2 || captured.GrantMode != "enhanced" || captured.EnhanceLevel != 4 {
		t.Fatalf("captured=%+v", captured)
	}
}
