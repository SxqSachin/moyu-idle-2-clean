package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
)

func newCaptchaTestServer(t *testing.T, appEnv string) (*Server, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config: config.Config{
				AppEnv:                 appEnv,
				GeetestID:              "test-geetest-id",
				GeetestKey:             "test-geetest-key",
				GeetestBypassStatusKey: "gt_server_bypass_status",
				GeetestDevBypass:       true,
				GeetestTokenTTLSec:     120,
			},
			Redis: rd,
		},
		geetestBypassStop: make(chan struct{}),
	}
	cleanup := func() {
		rd.Close()
		mr.Close()
	}
	return s, cleanup
}

func decodeAPIError(t *testing.T, resp *http.Response) apiErrorResponse {
	t.Helper()
	defer resp.Body.Close()
	var payload apiErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode api error failed: %v", err)
	}
	return payload
}

func issueTokenForTest(t *testing.T, s *Server, ip string, ua string) string {
	t.Helper()
	token, err := s.issueCaptchaToken(context.Background(), ip, ua)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	return token
}

func issueTokenViaValidateEndpoint(t *testing.T, s *Server, ua string) string {
	t.Helper()
	s.Post("/api/geetest/validate", s.handleGeetestValidate)
	body := `{"geetest_challenge":"c","geetest_validate":"v","geetest_seccode":"s"}`
	req := httptest.NewRequest(http.MethodPost, "/api/geetest/validate", bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, ua)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			CaptchaToken string `json:"captchaToken"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload failed: %v", err)
	}
	if !payload.OK || strings.TrimSpace(payload.Data.CaptchaToken) == "" {
		t.Fatalf("expected captcha token from validate endpoint")
	}
	return payload.Data.CaptchaToken
}

func TestCaptchaTokenConsumeOneTime(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()

	token := issueTokenForTest(t, s, "127.0.0.1", "ua-test")
	ok, err := s.consumeCaptchaToken(context.Background(), token, "127.0.0.1", "ua-test")
	if err != nil {
		t.Fatalf("consume token failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected consume success")
	}
	ok, err = s.consumeCaptchaToken(context.Background(), token, "127.0.0.1", "ua-test")
	if err != nil {
		t.Fatalf("consume token second time failed: %v", err)
	}
	if ok {
		t.Fatalf("expected one-time token to be consumed")
	}
}

func TestCaptchaTokenMismatchDoesNotConsume(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()

	token := issueTokenForTest(t, s, "127.0.0.1", "ua-a")
	ok, err := s.consumeCaptchaToken(context.Background(), token, "127.0.0.1", "ua-b")
	if err != nil {
		t.Fatalf("consume mismatch failed: %v", err)
	}
	if ok {
		t.Fatalf("expected mismatch consume to fail")
	}
	ok, err = s.consumeCaptchaToken(context.Background(), token, "127.0.0.1", "ua-a")
	if err != nil {
		t.Fatalf("consume after mismatch failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected correct consume after mismatch to succeed")
	}
}

func TestLoginCaptchaRequiredAndInvalid(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()
	s.Post("/api/login", s.handleLogin)

	// missing captcha token
	reqBody := `{"username":"a","password":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthCaptchaRequired {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCaptchaRequired)
	}

	// invalid captcha token
	reqBody = `{"username":"a","password":"123456","captchaToken":"bad-token"}`
	req = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, "ua-test")
	resp, err = s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload = decodeAPIError(t, resp)
	if payload.Code != codeAuthCaptchaInvalid {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCaptchaInvalid)
	}
}

func TestLoginCaptchaValidProceedsToCredentialValidation(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()
	s.Post("/api/login", s.handleLogin)

	token := issueTokenViaValidateEndpoint(t, s, "ua-login")
	reqBody := `{"username":"a","password":"123456","captchaToken":"` + token + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, "ua-login")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthCredentialTooShort {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCredentialTooShort)
	}
}

func TestRegisterCaptchaRequiredAndInvalid(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()
	s.Post("/api/register", s.handleRegister)

	reqBody := `{"username":"a","email":"u@example.com","password":"123456","acceptedTerms":true,"acceptedTermsVersion":"v1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthCaptchaRequired {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCaptchaRequired)
	}

	reqBody = `{"username":"a","email":"u@example.com","password":"123456","acceptedTerms":true,"acceptedTermsVersion":"v1","captchaToken":"bad-token"}`
	req = httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, "ua-register")
	resp, err = s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload = decodeAPIError(t, resp)
	if payload.Code != codeAuthCaptchaInvalid {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCaptchaInvalid)
	}
}

func TestRegisterCaptchaValidProceedsToCredentialValidation(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()
	s.Post("/api/register", s.handleRegister)

	token := issueTokenViaValidateEndpoint(t, s, "ua-register-2")
	reqBody := `{"username":"a","email":"u@example.com","password":"123456","acceptedTerms":true,"acceptedTermsVersion":"v1","captchaToken":"` + token + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, "ua-register-2")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthCredentialTooShort {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCredentialTooShort)
	}
}

func TestGeetestValidateBypassPolicy(t *testing.T) {
	// production + bypass fail should reject
	sProd, cleanupProd := newCaptchaTestServer(t, "production")
	defer cleanupProd()
	sProd.deps.Config.GeetestDevBypass = false
	sProd.Post("/api/geetest/validate", sProd.handleGeetestValidate)
	body := `{"geetest_challenge":"c","geetest_validate":"v","geetest_seccode":"s"}`
	req := httptest.NewRequest(http.MethodPost, "/api/geetest/validate", bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := sProd.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthCaptchaUnavailable {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthCaptchaUnavailable)
	}

	// development + bypass fail + non-empty payload should pass fail-mode and issue token
	sDev, cleanupDev := newCaptchaTestServer(t, "development")
	defer cleanupDev()
	sDev.Post("/api/geetest/validate", sDev.handleGeetestValidate)
	req = httptest.NewRequest(http.MethodPost, "/api/geetest/validate", bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err = sDev.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var successPayload apiSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&successPayload); err != nil {
		t.Fatalf("decode success payload failed: %v", err)
	}
	if !successPayload.OK {
		t.Fatalf("expected success response")
	}
	data, ok := successPayload.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", successPayload.Data)
	}
	if strings.TrimSpace(fmt.Sprintf("%v", data["captchaToken"])) == "" {
		t.Fatalf("expected captchaToken in response")
	}
}

func TestRefreshGeetestBypassStatusUpdatesRedis(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()
	s.deps.Config.GeetestID = "test-geetest-id"

	successAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer successAPI.Close()

	s.deps.Config.GeetestBypassURL = successAPI.URL
	s.refreshGeetestBypassStatus(context.Background())
	status, err := s.deps.Redis.Get(context.Background(), s.geetestBypassStatusKey()).Result()
	if err != nil {
		t.Fatalf("read bypass status failed: %v", err)
	}
	if status != geetestBypassStatusSuccess {
		t.Fatalf("status=%s, want %s", status, geetestBypassStatusSuccess)
	}

	failAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failAPI.Close()

	s.deps.Config.GeetestBypassURL = failAPI.URL
	s.refreshGeetestBypassStatus(context.Background())
	status, err = s.deps.Redis.Get(context.Background(), s.geetestBypassStatusKey()).Result()
	if err != nil {
		t.Fatalf("read bypass status failed: %v", err)
	}
	if status != geetestBypassStatusFail {
		t.Fatalf("status=%s, want %s", status, geetestBypassStatusFail)
	}

	s.deps.Config.GeetestBypassURL = "http://127.0.0.1:1/unreachable"
	s.refreshGeetestBypassStatus(context.Background())
	status, err = s.deps.Redis.Get(context.Background(), s.geetestBypassStatusKey()).Result()
	if err != nil {
		t.Fatalf("read bypass status failed: %v", err)
	}
	if status != geetestBypassStatusFail {
		t.Fatalf("status=%s, want %s", status, geetestBypassStatusFail)
	}
}
