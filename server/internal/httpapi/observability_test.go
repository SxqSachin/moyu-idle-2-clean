package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/store"
)

func captureLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prevWriter := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
	}()
	fn()
	return buf.String()
}

func installLoginHookDefaults(t *testing.T) {
	t.Helper()
	passwordHash := testPasswordHash(t)
	origGetUserAuthState := loginGetUserAuthState
	origGetUserSettings := loginGetUserSettings
	origCheckIPTrust := loginCheckIPTrust
	origIssueRecoveryCode := loginIssueRecoveryCode
	origSendRecoveryCodeEmail := loginSendRecoveryCodeEmail
	origClearRecoveryAttempts := loginClearRecoveryAttempts
	origStorePendingIPLogin := loginStorePendingIPLogin
	origPlayerAttributeSeeds := loginPlayerAttributeSeeds
	origEnsurePlayerAttributes := loginEnsurePlayerAttributes
	origRecomputeDerivedStats := loginRecomputeDerivedStats
	origGetPresence := loginGetPresence
	origRotateSessionVersion := loginRotateSessionVersion
	origSignToken := loginSignToken
	origRecordIPLogin := loginRecordIPLogin
	origIsTrustedDevice := loginIsTrustedDevice
	origTrustDevice := loginTrustDevice
	t.Cleanup(func() {
		loginGetUserAuthState = origGetUserAuthState
		loginGetUserSettings = origGetUserSettings
		loginCheckIPTrust = origCheckIPTrust
		loginIssueRecoveryCode = origIssueRecoveryCode
		loginSendRecoveryCodeEmail = origSendRecoveryCodeEmail
		loginClearRecoveryAttempts = origClearRecoveryAttempts
		loginStorePendingIPLogin = origStorePendingIPLogin
		loginPlayerAttributeSeeds = origPlayerAttributeSeeds
		loginEnsurePlayerAttributes = origEnsurePlayerAttributes
		loginRecomputeDerivedStats = origRecomputeDerivedStats
		loginGetPresence = origGetPresence
		loginRotateSessionVersion = origRotateSessionVersion
		loginSignToken = origSignToken
		loginRecordIPLogin = origRecordIPLogin
		loginIsTrustedDevice = origIsTrustedDevice
		loginTrustDevice = origTrustDevice
	})
	loginGetUserAuthState = func(_ *Server, _ context.Context, _ string) (store.UserAuthState, error) {
		return store.UserAuthState{
			UserID:        42,
			PasswordHash:  string(passwordHash),
			EmailVerified: false,
		}, nil
	}
	loginGetUserSettings = func(_ *Server, _ context.Context, userID int64) (store.UserSettings, error) {
		return store.UserSettings{UserID: userID}, nil
	}
	loginCheckIPTrust = func(_ *Server, _ context.Context, _ int64, _ string) (int, bool, error) {
		return 1, true, nil
	}
	loginIssueRecoveryCode = func(_ *Server, _ context.Context, _, _, _, _ string) (string, error) {
		return "123456", nil
	}
	loginSendRecoveryCodeEmail = func(_ *Server, _, _, _, _ string, _ time.Duration) error { return nil }
	loginClearRecoveryAttempts = func(_ *Server, _ context.Context, _, _ string) error { return nil }
	loginStorePendingIPLogin = func(_ *Server, _ context.Context, _ int64, _, _ string) (string, error) {
		return "pending-token", nil
	}
	loginPlayerAttributeSeeds = func(_ *Server) ([]store.PlayerAttributeSeed, error) {
		return []store.PlayerAttributeSeed{{AttrKey: "hp", BaseDefault: 10}}, nil
	}
	loginEnsurePlayerAttributes = func(_ *Server, _ context.Context, _ int64, _ []store.PlayerAttributeSeed) error { return nil }
	loginRecomputeDerivedStats = func(_ *Server, _ context.Context, _ int64) error { return nil }
	loginGetPresence = func(_ *Server, _ context.Context, userID int64) (store.PresenceState, error) {
		return store.PresenceState{UserID: userID, SessionVersion: 1}, nil
	}
	loginRotateSessionVersion = func(_ *Server, _ context.Context, _ int64) (int64, error) { return 2, nil }
	loginSignToken = func(_ *Server, _ int64, _ int64) (string, error) { return "signed-token", nil }
	loginRecordIPLogin = func(_ *Server, _ context.Context, _ int64, _ string) error { return nil }
	loginIsTrustedDevice = func(_ *Server, _ context.Context, _ int64, _ string) (bool, error) { return false, nil }
	loginTrustDevice = func(_ *Server, _ context.Context, _ int64, _ string, _ time.Duration) error { return nil }
}

func newLoginObservabilityServer(t *testing.T) *Server {
	t.Helper()
	s, cleanup := newCaptchaTestServer(t, "development")
	t.Cleanup(cleanup)
	s.loginGuard = newLoginAttemptGuard(5, time.Minute, time.Minute, s.deps.Redis, "test-login")
	s.Post("/api/login", s.requestMetricsMiddleware(), s.handleLogin)
	return s
}

func issueLoginCaptchaToken(t *testing.T, s *Server) string {
	t.Helper()
	return issueTokenViaValidateEndpoint(t, s, "ua-login-observability")
}

func testPasswordHash(t *testing.T) string {
	t.Helper()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	return string(passwordHash)
}

func TestLoginObservabilityLogsDBFailure(t *testing.T) {
	s := newLoginObservabilityServer(t)
	installLoginHookDefaults(t)
	loginGetUserAuthState = func(_ *Server, _ context.Context, _ string) (store.UserAuthState, error) {
		return store.UserAuthState{}, errors.New("db unavailable")
	}

	body := `{"username":"alice","password":"password123","captchaToken":"` + issueLoginCaptchaToken(t, s) + `"}`
	logs := captureLogs(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set(fiber.HeaderUserAgent, "ua-login-observability")
		resp, err := s.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Fatalf("status=%d, want %d", resp.StatusCode, fiber.StatusInternalServerError)
		}
	})

	if !strings.Contains(logs, "login request failed") || !strings.Contains(logs, "code=AUTH_LOOKUP_USER_FAILED") {
		t.Fatalf("expected db failure login log, got %q", logs)
	}
}

func TestLoginObservabilityLogsSlowAttributeInit(t *testing.T) {
	s := newLoginObservabilityServer(t)
	installLoginHookDefaults(t)
	loginEnsurePlayerAttributes = func(_ *Server, _ context.Context, _ int64, _ []store.PlayerAttributeSeed) error {
		time.Sleep(slowLoginThreshold + 50*time.Millisecond)
		return nil
	}

	body := `{"username":"alice","password":"password123","captchaToken":"` + issueLoginCaptchaToken(t, s) + `"}`
	logs := captureLogs(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set(fiber.HeaderUserAgent, "ua-login-observability")
		resp, err := s.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("status=%d, want %d", resp.StatusCode, fiber.StatusOK)
		}
	})

	if !strings.Contains(logs, "login request slow") || !strings.Contains(logs, loginStageAttributesEnsured) {
		t.Fatalf("expected slow attribute login log, got %q", logs)
	}
}

func TestLoginObservabilityLogsIPVerifyBranch(t *testing.T) {
	s := newLoginObservabilityServer(t)
	installLoginHookDefaults(t)
	loginGetUserAuthState = func(_ *Server, _ context.Context, _ string) (store.UserAuthState, error) {
		return store.UserAuthState{
			UserID:        42,
			PasswordHash:  testPasswordHash(t),
			EmailVerified: true,
			Email:         "a@example.com",
		}, nil
	}
	loginGetUserSettings = func(_ *Server, _ context.Context, userID int64) (store.UserSettings, error) {
		return store.UserSettings{UserID: userID, DisableIPLoginVerify: false}, nil
	}
	loginCheckIPTrust = func(_ *Server, _ context.Context, _ int64, _ string) (int, bool, error) {
		return 0, true, nil
	}

	body := `{"username":"alice","password":"password123","captchaToken":"` + issueLoginCaptchaToken(t, s) + `"}`
	logs := captureLogs(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set(fiber.HeaderUserAgent, "ua-login-observability")
		resp, err := s.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusForbidden {
			t.Fatalf("status=%d, want %d", resp.StatusCode, fiber.StatusForbidden)
		}
	})

	if !strings.Contains(logs, "code=LOGIN_IP_VERIFY_REQUIRED") || !strings.Contains(logs, "ip_verify_required=true") {
		t.Fatalf("expected ip verify login log, got %q", logs)
	}
}

func TestLoginObservabilityLogsSuccessfulNormalLogin(t *testing.T) {
	s := newLoginObservabilityServer(t)
	installLoginHookDefaults(t)
	loginSignToken = func(_ *Server, _ int64, _ int64) (string, error) {
		time.Sleep(slowLoginThreshold + 50*time.Millisecond)
		return "signed-token", nil
	}

	body := `{"username":"alice","password":"password123","captchaToken":"` + issueLoginCaptchaToken(t, s) + `"}`
	logs := captureLogs(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set(fiber.HeaderUserAgent, "ua-login-observability")
		resp, err := s.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("status=%d, want %d", resp.StatusCode, fiber.StatusOK)
		}
	})

	if !strings.Contains(logs, "login request slow") || !strings.Contains(logs, "status=200") || !strings.Contains(logs, loginStageTokenSigned) {
		t.Fatalf("expected successful login slow log, got %q", logs)
	}
}

func TestRequestMetricsMiddlewareLogsSlowAPIRequests(t *testing.T) {
	s := &Server{App: fiber.New()}
	s.Get("/api/slow", s.requestMetricsMiddleware(), func(c *fiber.Ctx) error {
		time.Sleep(slowAPIRequestThreshold + 50*time.Millisecond)
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
	})

	logs := captureLogs(t, func() {
		resp, err := s.Test(httptest.NewRequest(http.MethodGet, "/api/slow", nil), int((slowAPIRequestThreshold+500*time.Millisecond)/time.Millisecond))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("status=%d, want %d", resp.StatusCode, fiber.StatusOK)
		}
	})

	if !strings.Contains(logs, "api request slow") || !strings.Contains(logs, "path=/api/slow") {
		t.Fatalf("expected slow api log, got %q", logs)
	}
	for _, field := range []string{"auth_ms=", "auth_total_ms=", "auth_parse_ms=", "auth_db_ms=", "pg_acquire_ms=", "pool_acquired=", "pool_idle=", "pool_max="} {
		if !strings.Contains(logs, field) {
			t.Fatalf("slow api log missing %s: %q", field, logs)
		}
	}
}

func TestReadinessDegradedReasonsIncludesStoreFailure(t *testing.T) {
	origStoreHealth := readinessStoreHealth
	origRedisPing := readinessRedisPing
	t.Cleanup(func() {
		readinessStoreHealth = origStoreHealth
		readinessRedisPing = origRedisPing
	})
	readinessStoreHealth = func(_ *Server, _ context.Context) error { return errors.New("db down") }
	readinessRedisPing = func(_ *Server, _ context.Context) error { return nil }
	s := &Server{deps: app.Dependencies{Store: &store.Store{}, Redis: redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})}}
	reasons := s.readinessDegradedReasons(time.Now())
	if !containsReason(reasons, "store_unhealthy") {
		t.Fatalf("reasons=%v, want store_unhealthy", reasons)
	}
}

func TestReadinessDegradedReasonsIncludesRedisFailure(t *testing.T) {
	origStoreHealth := readinessStoreHealth
	origRedisPing := readinessRedisPing
	t.Cleanup(func() {
		readinessStoreHealth = origStoreHealth
		readinessRedisPing = origRedisPing
	})
	readinessStoreHealth = func(_ *Server, _ context.Context) error { return nil }
	readinessRedisPing = func(_ *Server, _ context.Context) error { return errors.New("redis down") }
	s := &Server{deps: app.Dependencies{Store: &store.Store{}, Redis: redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})}}
	reasons := s.readinessDegradedReasons(time.Now())
	if !containsReason(reasons, "redis_unhealthy") {
		t.Fatalf("reasons=%v, want redis_unhealthy", reasons)
	}
}

func TestReadinessDegradedReasonsIncludesNATSFailure(t *testing.T) {
	s := &Server{}
	reasons := s.readinessDegradedReasons(time.Now())
	if !containsReason(reasons, "nats_unavailable") {
		t.Fatalf("reasons=%v, want nats_unavailable", reasons)
	}
}

func TestReadinessDegradedReasonsIncludesPressureWindow(t *testing.T) {
	origStoreHealth := readinessStoreHealth
	origRedisPing := readinessRedisPing
	t.Cleanup(func() {
		readinessStoreHealth = origStoreHealth
		readinessRedisPing = origRedisPing
	})
	readinessStoreHealth = func(_ *Server, _ context.Context) error { return nil }
	readinessRedisPing = func(_ *Server, _ context.Context) error { return nil }
	s := &Server{
		deps:   app.Dependencies{Store: &store.Store{}, Redis: redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})},
		wsNATS: &nats.Conn{},
	}
	now := time.Now()
	s.wsSlowConsumerAt = []time.Time{
		now.Add(-time.Second),
		now.Add(-2 * time.Second),
		now.Add(-3 * time.Second),
	}
	reasons := s.readinessDegradedReasons(now)
	if !containsReason(reasons, "ws_pressure") {
		t.Fatalf("reasons=%v, want ws_pressure", reasons)
	}
}

func containsReason(reasons []string, target string) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}
