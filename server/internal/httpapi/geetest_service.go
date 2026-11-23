package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const (
	geetestBypassStatusSuccess = "success"
	geetestBypassStatusFail    = "fail"
)

var consumeCaptchaTokenScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current then
	return 0
end
if current == ARGV[1] then
	redis.call("DEL", KEYS[1])
	return 1
end
return -1
`)

type geetestValidateRequest struct {
	Challenge string `json:"geetest_challenge"`
	Validate  string `json:"geetest_validate"`
	Seccode   string `json:"geetest_seccode"`
}

func (s *Server) startGeetestBypassPoller() {
	if s.deps.Redis == nil {
		return
	}
	geetestID := strings.TrimSpace(s.deps.Config.GeetestID)
	if geetestID == "" {
		return
	}
	cycle := time.Duration(s.deps.Config.GeetestBypassCycleSec) * time.Second
	if cycle <= 0 {
		cycle = 10 * time.Second
	}
	go func() {
		s.refreshGeetestBypassStatus(context.Background())
		ticker := time.NewTicker(cycle)
		defer ticker.Stop()
		for {
			select {
			case <-s.geetestBypassStop:
				return
			case <-ticker.C:
				s.refreshGeetestBypassStatus(context.Background())
			}
		}
	}()
}

func (s *Server) refreshGeetestBypassStatus(ctx context.Context) {
	status := geetestBypassStatusFail
	fetched, err := fetchGeetestBypassStatus(s.deps.Config.GeetestBypassURL, s.deps.Config.GeetestID)
	if err == nil {
		status = fetched
	}
	if s.deps.Redis == nil {
		return
	}
	if err := s.deps.Redis.Set(ctx, s.geetestBypassStatusKey(), status, 0).Err(); err != nil {
		log.Printf("geetest bypass status update failed status=%s err=%v", status, err)
		return
	}
}

func fetchGeetestBypassStatus(rawURL string, geetestID string) (string, error) {
	target := strings.TrimSpace(rawURL)
	if target == "" {
		target = "http://bypass.geetest.com/v1/bypass_status.php"
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return geetestBypassStatusFail, err
	}
	query := url.Values{}
	query.Set("gt", strings.TrimSpace(geetestID))
	req.URL.RawQuery = query.Encode()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return geetestBypassStatusFail, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return geetestBypassStatusFail, errors.New("bypass status http not ok")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return geetestBypassStatusFail, err
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return geetestBypassStatusFail, err
	}
	if strings.EqualFold(strings.TrimSpace(payload.Status), geetestBypassStatusSuccess) {
		return geetestBypassStatusSuccess, nil
	}
	return geetestBypassStatusFail, nil
}

func (s *Server) geetestBypassStatusKey() string {
	key := strings.TrimSpace(s.deps.Config.GeetestBypassStatusKey)
	if key == "" {
		return "gt_server_bypass_status"
	}
	return key
}

func (s *Server) geetestBypassStatus(ctx context.Context) string {
	if s.deps.Redis == nil {
		return geetestBypassStatusFail
	}
	status, err := s.deps.Redis.Get(ctx, s.geetestBypassStatusKey()).Result()
	if err != nil {
		return geetestBypassStatusFail
	}
	if strings.EqualFold(strings.TrimSpace(status), geetestBypassStatusSuccess) {
		return geetestBypassStatusSuccess
	}
	return geetestBypassStatusFail
}

func (s *Server) geetestFailModeAllowed() bool {
	return s.isDevEnv() && s.deps.Config.GeetestDevBypass
}

func (s *Server) handleGeetestRegister(c *fiber.Ctx) error {
	gt := newGeetestLib(s.deps.Config.GeetestID, s.deps.Config.GeetestKey)
	digestmod := "md5"
	params := map[string]string{
		"digestmod":   digestmod,
		"user_id":     geetestUserID(c.IP()),
		"client_type": "web",
		"ip_address":  strings.TrimSpace(c.IP()),
	}
	bypassStatus := s.geetestBypassStatus(c.Context())
	result := gt.localRegister()
	if bypassStatus == geetestBypassStatusSuccess {
		result = gt.register(digestmod, params)
	}
	c.Set(fiber.HeaderContentType, "application/json;charset=UTF-8")
	return c.SendString(result.Data)
}

func geetestUserID(ip string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ip)))
	return hex.EncodeToString(sum[:8])
}

func (s *Server) handleGeetestValidate(c *fiber.Ctx) error {
	var req geetestValidateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	gt := newGeetestLib(s.deps.Config.GeetestID, s.deps.Config.GeetestKey)
	params := map[string]string{
		"user_id":     geetestUserID(c.IP()),
		"client_type": "web",
		"ip_address":  strings.TrimSpace(c.IP()),
	}

	bypassStatus := s.geetestBypassStatus(c.Context())
	var result *geetestLibResult
	switch bypassStatus {
	case geetestBypassStatusSuccess:
		result = gt.successValidate(req.Challenge, req.Validate, req.Seccode, params)
	default:
		if !s.geetestFailModeAllowed() {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthCaptchaUnavailable, "Captcha service unavailable", "geetest bypass status fail")
		}
		result = gt.failValidate(req.Challenge, req.Validate, req.Seccode)
	}
	if result.Status != 1 {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthCaptchaInvalid, "Captcha verification failed", result.Msg)
	}
	captchaToken, err := s.issueCaptchaToken(c.Context(), c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthCaptchaUnavailable, "Captcha service unavailable", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"captchaToken": captchaToken})
}

func (s *Server) issueCaptchaToken(ctx context.Context, ip string, userAgent string) (string, error) {
	if s.deps.Redis == nil {
		return "", errors.New("captcha backend unavailable: redis is nil")
	}
	token := randomHexLower(64)
	key := s.captchaTokenKey(token)
	if key == "" {
		return "", errors.New("captcha token key is empty")
	}
	ttl := time.Duration(s.deps.Config.GeetestTokenTTLSec) * time.Second
	if ttl <= 0 {
		ttl = 120 * time.Second
	}
	binding := captchaBindingDigest(ip, userAgent)
	if err := s.deps.Redis.Set(ctx, key, binding, ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) captchaTokenKey(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	return "captcha:token:" + trimmed
}

func captchaBindingDigest(ip string, userAgent string) string {
	payload := strings.TrimSpace(ip) + "\n" + strings.TrimSpace(userAgent)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (s *Server) consumeCaptchaToken(ctx context.Context, token string, ip string, userAgent string) (bool, error) {
	if s.deps.Redis == nil {
		return false, errors.New("captcha backend unavailable: redis is nil")
	}
	key := s.captchaTokenKey(token)
	if key == "" {
		return false, nil
	}
	binding := captchaBindingDigest(ip, userAgent)
	raw, err := consumeCaptchaTokenScript.Run(ctx, s.deps.Redis, []string{key}, binding).Result()
	if err != nil {
		return false, err
	}
	code, err := toInt64(raw)
	if err != nil {
		return false, fmt.Errorf("captcha consume result parse failed: %w", err)
	}
	return code == 1, nil
}

func (s *Server) requireCaptchaToken(c *fiber.Ctx, token string) bool {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		_ = s.writeAPIError(c, fiber.StatusBadRequest, codeAuthCaptchaRequired, "Captcha token is required", "")
		return false
	}
	ok, err := s.consumeCaptchaToken(c.Context(), trimmed, c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		_ = s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthCaptchaUnavailable, "Captcha service unavailable", err.Error())
		return false
	}
	if !ok {
		_ = s.writeAPIError(c, fiber.StatusBadRequest, codeAuthCaptchaInvalid, "Captcha token is invalid", "")
		return false
	}
	return true
}
