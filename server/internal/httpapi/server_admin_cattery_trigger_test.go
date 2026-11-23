package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
)

func newAdminCatteryTriggerTestServer() *Server {
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
	s.Post("/api/admin/cattery/trigger", s.releaseAdminMiddleware(), s.handleAdminCatteryTrigger)
	return s
}

func TestAdminCatteryTriggerRequiresReleaseToken(t *testing.T) {
	s := newAdminCatteryTriggerTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cattery/trigger", bytes.NewBufferString(`{"userId":1,"kittyId":1}`))
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

func TestAdminCatteryTriggerRejectsInvalidType(t *testing.T) {
	s := newAdminCatteryTriggerTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cattery/trigger", bytes.NewBufferString(`{"userId":1,"kittyId":1,"triggerType":"foo"}`))
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
	if payload.Code != "ADMIN_CATTERY_TRIGGER_INVALID_TYPE" {
		t.Fatalf("code=%s, want ADMIN_CATTERY_TRIGGER_INVALID_TYPE", payload.Code)
	}
}

func TestAdminCatteryTriggerSuccess(t *testing.T) {
	s := newAdminCatteryTriggerTestServer()
	original := adminCatteryTriggerExecutor
	t.Cleanup(func() { adminCatteryTriggerExecutor = original })
	var captured adminCatteryTriggerCommand
	adminCatteryTriggerExecutor = func(_ *Server, _ context.Context, cmd adminCatteryTriggerCommand) (adminCatteryTriggerResult, error) {
		captured = cmd
		return adminCatteryTriggerResult{
			TriggerType: "encounter",
			UserID:      cmd.UserID,
			KittyID:     cmd.KittyID,
		}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cattery/trigger", bytes.NewBufferString(`{"userId":7,"kittyId":11,"triggerType":"encounter"}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	if captured.UserID != 7 || captured.KittyID != 11 || captured.TriggerType != "encounter" {
		t.Fatalf("captured=%+v", captured)
	}
	payload := decodeAPIResponse[apiSuccessResponse](t, resp)
	if !payload.OK {
		t.Fatalf("expected ok=true")
	}
}
