package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func newAdminUserBanTestServer() *Server {
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{
				AppEnv:               "development",
				ReleaseAnnounceToken: "admin-token",
				GlobalSettings:       config.GlobalSettings{Admin: config.AdminSettings{UserBanReasons: []config.AdminBanReason{{Key: "abnormal_trade", NameKey: "admin.banReason.abnormalTrade"}}}},
			},
		},
	}
	s.Post("/api/admin/users/ban", s.releaseAdminMiddleware(), s.handleAdminUserBan)
	s.Post("/api/admin/users/unban", s.releaseAdminMiddleware(), s.handleAdminUserUnban)
	s.Get("/api/admin/users/ban-reasons", s.releaseAdminMiddleware(), s.handleAdminUserBanReasons)
	return s
}

func TestAdminUserBanRequiresReleaseToken(t *testing.T) {
	s := newAdminUserBanTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/ban", bytes.NewBufferString(`{"userId":1,"reason":"abnormal_trade","isPermanent":true}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminUserBanRejectsInvalidReason(t *testing.T) {
	s := newAdminUserBanTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/ban", bytes.NewBufferString(`{"userId":1,"reason":"other","isPermanent":true}`))
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
	if payload.Code != "ADMIN_USER_BAN_INVALID_REASON" {
		t.Fatalf("code=%s, want ADMIN_USER_BAN_INVALID_REASON", payload.Code)
	}
}

func TestAdminUserBanSuccess(t *testing.T) {
	s := newAdminUserBanTestServer()
	origBan := adminUserBanExecutor
	origKick := adminUserKickExecutor
	origResolve := adminUserResolveTargetExecutor
	t.Cleanup(func() {
		adminUserBanExecutor = origBan
		adminUserKickExecutor = origKick
		adminUserResolveTargetExecutor = origResolve
	})
	var captured adminUserBanCommand
	var kickedUserID int64
	adminUserResolveTargetExecutor = func(_ context.Context, _ *store.Store, _ int64, _ string) (int64, error) {
		return 7, nil
	}
	adminUserKickExecutor = func(_ *Server, userID int64) {
		kickedUserID = userID
	}
	adminUserBanExecutor = func(_ *Server, _ context.Context, cmd adminUserBanCommand) error {
		captured = cmd
		return nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/ban", bytes.NewBufferString(`{"username":"alice","reason":"abnormal_trade","banUntil":"2030-01-01T00:00:00Z","isPermanent":false}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	if kickedUserID != 7 {
		t.Fatalf("kickedUserID=%d, want 7", kickedUserID)
	}
	if captured.UserID != 7 || captured.Reason != "abnormal_trade" || captured.BanUntil == nil || captured.IsPermanent {
		t.Fatalf("captured=%+v", captured)
	}
	payload := decodeAPIResponse[apiSuccessResponse](t, resp)
	if !payload.OK {
		t.Fatalf("expected ok=true")
	}
}

func TestAdminUserUnbanSuccess(t *testing.T) {
	s := newAdminUserBanTestServer()
	origUnban := adminUserUnbanExecutor
	origResolve := adminUserResolveTargetExecutor
	t.Cleanup(func() {
		adminUserUnbanExecutor = origUnban
		adminUserResolveTargetExecutor = origResolve
	})
	var unbannedUserID int64
	adminUserResolveTargetExecutor = func(_ context.Context, _ *store.Store, _ int64, _ string) (int64, error) {
		return 9, nil
	}
	adminUserUnbanExecutor = func(_ *Server, _ context.Context, userID int64) error {
		unbannedUserID = userID
		return nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/unban", bytes.NewBufferString(`{"username":"alice"}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	if unbannedUserID != 9 {
		t.Fatalf("unbannedUserID=%d, want 9", unbannedUserID)
	}
}

func TestParseAdminBanUntil(t *testing.T) {
	parsed, err := parseAdminBanUntil("2030-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("parseAdminBanUntil failed: %v", err)
	}
	want := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Fatalf("parsed=%s, want=%s", parsed.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestAdminUserBanReasonsEndpoint(t *testing.T) {
	s := newAdminUserBanTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users/ban-reasons", nil)
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
}
