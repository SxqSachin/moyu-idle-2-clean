package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"mysecondwebgame/server/internal/store"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dm "github.com/alibabacloud-go/dm-20151123/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

const (
	authRecoveryCodeTTL        = 10 * time.Minute
	authRecoveryCodeCooldown   = 60 * time.Second
	authRecoveryCodeMaxFails   = 5
	authPasswordResetTokenTTL  = 30 * time.Minute
	authRecoveryCodeLength     = 6
	authRecoveryEmailSubjectCN = "MySecondWebGame"
)

type aliyunDirectMailConfig struct {
	AccessKeyID     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	AccountName     string `json:"accountName"`
	RegionID        string `json:"regionId"`
	Endpoint        string `json:"endpoint"`
	FromAlias       string `json:"fromAlias"`
	TemplateID      string `json:"templateId"`
}

type forgotPasswordSendRequest struct {
	Email        string `json:"email"`
	CaptchaToken string `json:"captchaToken"`
}

type forgotPasswordVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type forgotPasswordResetRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type emailVerifySendRequest struct {
	Email        string `json:"email"`
	CaptchaToken string `json:"captchaToken"`
}

type emailVerifyConfirmRequest struct {
	Code string `json:"code"`
}

func (s *Server) handleForgotPasswordSend(c *fiber.Ctx) error {
	var req forgotPasswordSendRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	if !s.requireCaptchaToken(c, req.CaptchaToken) {
		return nil
	}
	// Existing accounts retain password recovery even if their domain is blocked later.
	email := normalizeEmail(req.Email)
	if email == "" || len(email) > maxEmailLength || !isValidEmail(email) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidEmail, "Email is invalid", "")
	}

	_, username, _, err := s.deps.Store.GetUserByEmail(c.Context(), email)
	if err != nil {
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"sent": true})
	}
	code, err := s.issueRecoveryCode(c.Context(), authRecoveryPurposeReset, email, c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthEmailSendFailed, "Failed to send recovery email", err.Error())
	}
	if err := s.sendRecoveryCodeEmail(email, username, "密码重置", code, authRecoveryCodeTTL); err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthEmailSendFailed, "Failed to send recovery email", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"sent": true})
}

func (s *Server) handleForgotPasswordVerify(c *fiber.Ctx) error {
	var req forgotPasswordVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	email := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if email == "" || code == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", "")
	}
	if !isValidEmail(email) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidEmail, "Email is invalid", "")
	}
	userID, _, _, err := s.deps.Store.GetUserByEmail(c.Context(), email)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthVerifyCodeInvalid, "Verification code is invalid", "")
	}
	ok, err := s.consumeRecoveryCode(c.Context(), authRecoveryPurposeReset, email, code, c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthEmailSendFailed, "Recovery backend unavailable", err.Error())
	}
	if !ok {
		if attempts, attemptErr := s.bumpRecoveryCodeAttempt(c.Context(), authRecoveryPurposeReset, email); attemptErr == nil && attempts >= authRecoveryCodeMaxFails {
			return s.writeAPIError(c, fiber.StatusTooManyRequests, codeAuthVerifyCodeTooMany, "Too many failed verification attempts", "")
		}
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthVerifyCodeInvalid, "Verification code is invalid", "")
	}
	resetToken := randomHexLower(64)
	resetHash := recoveryTokenHash(resetToken)
	expiresAt := time.Now().Add(authPasswordResetTokenTTL)
	if err := s.deps.Store.CreatePasswordResetToken(c.Context(), userID, resetHash, expiresAt); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthResetPasswordFailed, "Failed to create reset token", err.Error())
	}
	_ = s.clearRecoveryCodeAttempts(c.Context(), authRecoveryPurposeReset, email)
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"resetToken": resetToken,
		"expiresIn":  int(authPasswordResetTokenTTL.Seconds()),
	})
}

func (s *Server) handleForgotPasswordReset(c *fiber.Ctx) error {
	var req forgotPasswordResetRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	token := strings.TrimSpace(req.Token)
	nextPassword := req.NewPassword
	if token == "" || len(nextPassword) < minPasswordLength {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", "")
	}
	tokenHash := recoveryTokenHash(token)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(nextPassword), bcrypt.DefaultCost)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthResetPasswordFailed, "Failed to process credentials", err.Error())
	}
	userID, ok, err := s.deps.Store.ConsumePasswordResetTokenAndUpdatePassword(c.Context(), tokenHash, string(passwordHash), time.Now())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthResetPasswordFailed, "Failed to reset password", err.Error())
	}
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthResetTokenInvalid, "Reset token is invalid", "")
	}
	sessionVersion, err := s.deps.Store.RotateSessionVersion(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to rotate session", err.Error())
	}
	s.kickUserSockets(userID, "PASSWORD_RESET")
	_ = sessionVersion
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"userId": userID})
}

func (s *Server) handlePlayerProfileEmailVerifySend(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req emailVerifySendRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	if !s.requireCaptchaToken(c, req.CaptchaToken) {
		return nil
	}
	email := normalizeEmail(req.Email)
	if email == "" || len(email) > maxEmailLength || !isValidEmail(email) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidEmail, "Email is invalid", "")
	}
	if s.isEmailDomainBlocked(email) {
		return s.writeEmailDomainBlocked(c)
	}
	current, err := s.deps.Store.GetUserSettings(c.Context(), userID)
	if err == nil && current.EmailVerified && strings.EqualFold(current.Email, email) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileEmailUnchanged, "Email is unchanged", "")
	}
	code, err := s.issueRecoveryCode(c.Context(), authRecoveryPurposeEmailVerify, strconv.FormatInt(userID, 10), c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthEmailSendFailed, "Failed to send verification email", err.Error())
	}
	if err := s.storePendingEmail(c.Context(), userID, email); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	username := current.Username
	if err := s.sendRecoveryCodeEmail(email, username, "邮箱验证", code, authRecoveryCodeTTL); err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthEmailSendFailed, "Failed to send verification email", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"email": email, "emailVerified": false})
}

func (s *Server) handlePlayerProfileEmailVerifyConfirm(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req emailVerifyConfirmRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", "")
	}
	email, err := s.pendingEmail(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthVerifyCodeExpired, "Verification code expired", err.Error())
	}
	if s.isEmailDomainBlocked(email) {
		return s.writeEmailDomainBlocked(c)
	}
	ok, err := s.consumeRecoveryCode(c.Context(), authRecoveryPurposeEmailVerify, strconv.FormatInt(userID, 10), code, c.IP(), c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthEmailSendFailed, "Recovery backend unavailable", err.Error())
	}
	if !ok {
		if attempts, attemptErr := s.bumpRecoveryCodeAttempt(c.Context(), authRecoveryPurposeEmailVerify, strconv.FormatInt(userID, 10)); attemptErr == nil && attempts >= authRecoveryCodeMaxFails {
			return s.writeAPIError(c, fiber.StatusTooManyRequests, codeAuthVerifyCodeTooMany, "Too many failed verification attempts", "")
		}
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthVerifyCodeInvalid, "Verification code is invalid", "")
	}
	if err := s.deps.Store.MarkUserEmailVerified(c.Context(), userID, email, time.Now()); err != nil {
		if errors.Is(err, store.ErrDuplicateEmail) {
			return s.writeAPIError(c, fiber.StatusConflict, codeProfileEmailExists, "Email already exists", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	_ = s.clearPendingEmail(c.Context(), userID)
	_ = s.clearRecoveryCodeAttempts(c.Context(), authRecoveryPurposeEmailVerify, strconv.FormatInt(userID, 10))
	settings, _ := s.deps.Store.GetUserSettings(c.Context(), userID)
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"email": email, "emailVerified": true, "username": settings.Username})
}

const authRecoveryPurposeReset = "reset"
const authRecoveryPurposeEmailVerify = "email_verify"

func normalizeEmail(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}

func recoveryTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func recoveryCodeKey(purpose, scope string) string {
	return "auth:recovery:code:" + purpose + ":" + strings.ToLower(strings.TrimSpace(scope))
}

func recoveryAttemptsKey(purpose, scope string) string {
	return "auth:recovery:attempts:" + purpose + ":" + strings.ToLower(strings.TrimSpace(scope))
}

func recoveryCooldownKey(purpose, scope string) string {
	return "auth:recovery:cooldown:" + purpose + ":" + strings.ToLower(strings.TrimSpace(scope))
}

func pendingEmailKey(userID int64) string {
	return "auth:recovery:pending-email:" + strconv.FormatInt(userID, 10)
}

func (s *Server) issueRecoveryCode(ctx context.Context, purpose, scope, ip, userAgent string) (string, error) {
	if s.deps.Redis == nil {
		return "", errors.New("recovery backend unavailable")
	}
	scopeKey := strings.ToLower(strings.TrimSpace(scope))
	if scopeKey == "" {
		return "", errors.New("recovery scope is empty")
	}
	cooldownKey := recoveryCooldownKey(purpose, scopeKey)
	if ttl, err := s.deps.Redis.TTL(ctx, cooldownKey).Result(); err == nil && ttl > 0 {
		return "", fmt.Errorf("retry after %d seconds", int(ttl.Seconds()))
	}
	limitKey := "auth:recovery:limit:" + purpose + ":" + scopeKey
	count, err := s.deps.Redis.Incr(ctx, limitKey).Result()
	if err != nil {
		return "", err
	}
	if count == 1 {
		_ = s.deps.Redis.Expire(ctx, limitKey, time.Hour).Err()
	}
	if count > 8 {
		return "", errors.New("rate limited")
	}
	code := recoveryNumericCode(authRecoveryCodeLength)
	codeHash := recoveryCodeDigest(purpose, scopeKey, code)
	if err := s.deps.Redis.Set(ctx, recoveryCodeKey(purpose, scopeKey), codeHash, authRecoveryCodeTTL).Err(); err != nil {
		return "", err
	}
	_ = s.deps.Redis.Set(ctx, cooldownKey, "1", authRecoveryCodeCooldown).Err()
	_ = ip
	_ = userAgent
	return code, nil
}

func (s *Server) consumeRecoveryCode(ctx context.Context, purpose, scope, providedCode, ip, userAgent string) (bool, error) {
	if s.deps.Redis == nil {
		return false, errors.New("recovery backend unavailable")
	}
	scopeKey := strings.ToLower(strings.TrimSpace(scope))
	if scopeKey == "" {
		return false, nil
	}
	codeHash := recoveryCodeDigest(purpose, scopeKey, providedCode)
	raw, err := consumeCaptchaTokenScript.Run(ctx, s.deps.Redis, []string{recoveryCodeKey(purpose, scopeKey)}, codeHash).Result()
	if err != nil {
		return false, err
	}
	code, err := toInt64(raw)
	if err != nil {
		return false, err
	}
	_ = ip
	_ = userAgent
	return code == 1, nil
}

func (s *Server) bumpRecoveryCodeAttempt(ctx context.Context, purpose, scope string) (int64, error) {
	if s.deps.Redis == nil {
		return 0, errors.New("recovery backend unavailable")
	}
	n, err := s.deps.Redis.Incr(ctx, recoveryAttemptsKey(purpose, scope)).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = s.deps.Redis.Expire(ctx, recoveryAttemptsKey(purpose, scope), authRecoveryCodeTTL*6).Err()
	}
	return n, nil
}

func (s *Server) clearRecoveryCodeAttempts(ctx context.Context, purpose, scope string) error {
	if s.deps.Redis == nil {
		return errors.New("recovery backend unavailable")
	}
	return s.deps.Redis.Del(ctx, recoveryAttemptsKey(purpose, scope)).Err()
}

func recoveryCodeDigest(purpose, scope, code string) string {
	mac := hmac.New(sha256.New, []byte("mysecondwebgame-recovery-code"))
	_, _ = mac.Write([]byte(purpose + ":" + scope + ":" + strings.TrimSpace(code)))
	return hex.EncodeToString(mac.Sum(nil))
}

func recoveryNumericCode(length int) string {
	if length <= 0 {
		return ""
	}
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return strings.Repeat("0", length)
	}
	return fmt.Sprintf("%0*d", length, n.Int64())
}

func (s *Server) storePendingEmail(ctx context.Context, userID int64, email string) error {
	if s.deps.Redis == nil {
		return errors.New("recovery backend unavailable")
	}
	return s.deps.Redis.Set(ctx, pendingEmailKey(userID), normalizeEmail(email), authRecoveryCodeTTL).Err()
}

func (s *Server) pendingEmail(ctx context.Context, userID int64) (string, error) {
	if s.deps.Redis == nil {
		return "", errors.New("recovery backend unavailable")
	}
	email, err := s.deps.Redis.Get(ctx, pendingEmailKey(userID)).Result()
	if err != nil {
		return "", err
	}
	return normalizeEmail(email), nil
}

func (s *Server) clearPendingEmail(ctx context.Context, userID int64) error {
	if s.deps.Redis == nil {
		return errors.New("recovery backend unavailable")
	}
	return s.deps.Redis.Del(ctx, pendingEmailKey(userID)).Err()
}

func (s *Server) sendRecoveryCodeEmail(to, username, title, code string, ttl time.Duration) error {
	cfg, err := loadAliyunDirectMailConfig()
	log.Printf("loaded aliyun direct mail config: account=%s region=%s", cfg.AccountName, cfg.RegionID)
	if err != nil {
		return err
	}
	return sendRecoveryCodeEmailViaAliyun(cfg, to, username, title, code, ttl)
}

func loadAliyunDirectMailConfig() (*aliyunDirectMailConfig, error) {
	candidatePaths := []string{
		"email-aliyun.json",
		"server/email-aliyun.json",
		"config/email-aliyun.json",
	}
	if exe, err := os.Executable(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(filepath.Dir(exe), "email-aliyun.json"))
	}
	for _, path := range candidatePaths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg aliyunDirectMailConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("invalid aliyun email config %s: %w", path, err)
		}
		cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
		cfg.AccessKeySecret = strings.TrimSpace(cfg.AccessKeySecret)
		cfg.AccountName = strings.TrimSpace(cfg.AccountName)
		cfg.RegionID = strings.TrimSpace(cfg.RegionID)
		cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
		cfg.FromAlias = strings.TrimSpace(cfg.FromAlias)
		cfg.TemplateID = strings.TrimSpace(cfg.TemplateID)
		if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" || cfg.AccountName == "" {
			return nil, fmt.Errorf("aliyun email config %s is incomplete", path)
		}
		if cfg.RegionID == "" {
			cfg.RegionID = "cn-hangzhou"
		}
		if cfg.Endpoint == "" {
			cfg.Endpoint = "dm.aliyuncs.com"
		}

		return &cfg, nil
	}
	return nil, errors.New("aliyun email config not found")
}

func sendRecoveryCodeEmailViaAliyun(cfg *aliyunDirectMailConfig, to, username, title, code string, ttl time.Duration) error {
	clientCfg := &openapi.Config{
		AccessKeyId:     tea.String(cfg.AccessKeyID),
		AccessKeySecret: tea.String(cfg.AccessKeySecret),
		RegionId:        tea.String(cfg.RegionID),
		Endpoint:        tea.String(cfg.Endpoint),
	}
	client, err := dm.NewClient(clientCfg)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("%s - %s", authRecoveryEmailSubjectCN, title)
	body := fmt.Sprintf(`<html><body><h2>%s</h2><p>尊敬的 %s，您好。</p><p>验证码：<b>%s</b></p><p>有效期：%d 分钟</p></body></html>`, title, username, code, int(ttl.Minutes()))
	req := &dm.SingleSendMailRequest{
		AccountName:    tea.String(cfg.AccountName),
		AddressType:    tea.Int32(1),
		ToAddress:      tea.String(to),
		Subject:        tea.String(subject),
		HtmlBody:       tea.String(body),
		TextBody:       tea.String("1"),
		ReplyToAddress: tea.Bool(true),
	}
	if cfg.FromAlias != "" {
		req.FromAlias = tea.String(cfg.FromAlias)
	}
	if cfg.TemplateID != "" {
		req.Template = &dm.SingleSendMailRequestTemplate{
			TemplateId: tea.String(cfg.TemplateID),
			TemplateData: map[string]*string{
				"name":           tea.String(username),
				"code":           tea.String(code),
				"title":          tea.String(title),
				"expiry_minutes": tea.String(strconv.Itoa(int(ttl.Minutes()))),
			},
		}
	}
	runtime := &util.RuntimeOptions{}
	resp, err := client.SingleSendMailWithOptions(req, runtime)
	if err != nil {
		return err
	}
	log.Printf("aliyun dm email sent to=%s requestId=%s", to, tea.StringValue(resp.Body.RequestId))
	return nil
}
