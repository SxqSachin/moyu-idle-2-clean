package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mysecondwebgame/server/internal/risk"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	minUsernameLength = 2
	maxUsernameLength = 24
	minPasswordLength = 6
	maxEmailLength    = 128
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]+$`)
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

var (
	loginGetUserAuthState = func(s *Server, ctx context.Context, username string) (store.UserAuthState, error) {
		return s.deps.Store.GetUserAuthState(ctx, username)
	}
	loginGetUserSettings = func(s *Server, ctx context.Context, userID int64) (store.UserSettings, error) {
		return s.deps.Store.GetUserSettings(ctx, userID)
	}
	loginCheckIPTrust = func(s *Server, ctx context.Context, userID int64, ip string) (int, bool, error) {
		return s.deps.Store.CheckIPTrust(ctx, userID, ip)
	}
	loginIssueRecoveryCode = func(s *Server, ctx context.Context, purpose, scope, ip, ua string) (string, error) {
		return s.issueRecoveryCode(ctx, purpose, scope, ip, ua)
	}
	loginSendRecoveryCodeEmail = func(s *Server, to, username, subject, code string, ttl time.Duration) error {
		return s.sendRecoveryCodeEmail(to, username, subject, code, ttl)
	}
	loginClearRecoveryAttempts = func(s *Server, ctx context.Context, purpose, scope string) error {
		return s.clearRecoveryCodeAttempts(ctx, purpose, scope)
	}
	loginStorePendingIPLogin = func(s *Server, ctx context.Context, userID int64, ip, deviceHash string) (string, error) {
		return s.storePendingIPLogin(ctx, userID, ip, deviceHash)
	}
	loginPlayerAttributeSeeds   = func(s *Server) ([]store.PlayerAttributeSeed, error) { return s.playerAttributeSeeds() }
	loginEnsurePlayerAttributes = func(s *Server, ctx context.Context, userID int64, seeds []store.PlayerAttributeSeed) error {
		return s.deps.Store.EnsurePlayerAttributes(ctx, userID, seeds)
	}
	loginRecomputeDerivedStats = func(s *Server, ctx context.Context, userID int64) error {
		return s.deps.Store.RecomputeDerivedStats(ctx, userID)
	}
	loginGetPresence = func(s *Server, ctx context.Context, userID int64) (store.PresenceState, error) {
		return s.deps.Store.GetPresence(ctx, userID)
	}
	loginRotateSessionVersion = func(s *Server, ctx context.Context, userID int64) (int64, error) {
		return s.deps.Store.RotateSessionVersion(ctx, userID)
	}
	loginSignToken = func(s *Server, userID int64, sessionVersion int64) (string, error) {
		return s.deps.Auth.Sign(userID, sessionVersion)
	}
	loginRecordIPLogin = func(s *Server, ctx context.Context, userID int64, ip string) error {
		return s.deps.Store.RecordIPLogin(ctx, userID, ip)
	}
	loginIsTrustedDevice = func(s *Server, ctx context.Context, userID int64, deviceHash string) (bool, error) {
		return s.deps.Store.IsTrustedDevice(ctx, userID, deviceHash)
	}
	loginTrustDevice = func(s *Server, ctx context.Context, userID int64, deviceHash string, ttl time.Duration) error {
		return s.deps.Store.TrustDevice(ctx, userID, deviceHash, ttl)
	}
)

const trustedDeviceTTL = 30 * 24 * time.Hour

// computeDeviceHash derives a stable device identifier from the device profile.
// It combines deviceFamilyHash (hardware fingerprint) with localDeviceId (browser-unique secret).
// Returns "" if both are empty, which disables trusted device bypass.
func computeDeviceHash(profile risk.DeviceProfile) string {
	dfh := strings.TrimSpace(profile.DeviceFamilyHash)
	lid := strings.TrimSpace(profile.LocalDeviceID)
	if dfh == "" && lid == "" {
		return ""
	}
	h := sha256.Sum256([]byte(dfh + ":" + lid))
	return hex.EncodeToString(h[:])
}

func (s *Server) handleRegister(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	if !s.requireCaptchaToken(c, req.CaptchaToken) {
		return nil
	}
	normalizedUsername := strings.TrimSpace(req.Username)
	normalizedEmail := strings.TrimSpace(strings.ToLower(req.Email))
	if usernameLength(normalizedUsername) < minUsernameLength || len(req.Password) < minPasswordLength {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthCredentialTooShort, "Username or password is too short", "")
	}
	if normalizedEmail == "" || len(normalizedEmail) > maxEmailLength || !isValidEmail(normalizedEmail) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidEmail, "Email is invalid", "")
	}
	if s.isEmailDomainBlocked(normalizedEmail) {
		return s.writeEmailDomainBlocked(c)
	}
	policyVersion := normalizePolicyVersion(req.AcceptedTermsVersion)
	if !req.AcceptedTerms || policyVersion == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthTermsRequired, "Terms agreement is required for registration", "")
	}
	if usernameLength(normalizedUsername) > maxUsernameLength {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameTooLong, "Username is too long", "")
	}
	if !isValidUsername(normalizedUsername) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameFormat, "Username format is invalid", "")
	}
	if err := s.rejectModeratedAPI(c, moderationRuleUsername, normalizedUsername, 0); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthHashPasswordFailed, "Failed to process credentials", err.Error())
	}
	userID, err := s.deps.Store.CreateUser(c.Context(), normalizedUsername, normalizedEmail, string(hash), policyVersion)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateUser) {
			return s.writeAPIError(c, fiber.StatusConflict, codeAuthUsernameExists, "Username already exists", "")
		}
		if errors.Is(err, store.ErrDuplicateEmail) {
			return s.writeAPIError(c, fiber.StatusConflict, codeAuthEmailExists, "Email already exists", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthCreateUserFailed, "Failed to create user", err.Error())
	}
	token, err := s.deps.Auth.Sign(userID, 1)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
	}
	s.recordRiskSignal(c, userID, "register", req.DeviceProfile, req.Password, map[string]any{
		"username": normalizedUsername,
		"email":    normalizedEmail,
	})
	return s.writeAPISuccess(c, fiber.StatusCreated, fiber.Map{"userId": userID, "token": token})
}

func (s *Server) handleLogin(c *fiber.Ctx) error {
	logState := newLoginRequestLogState(c.Path(), c.IP(), "")
	defer func() {
		s.emitLoginRequestLog(logState, c)
	}()
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		logState.finalStatus = fiber.StatusBadRequest
		logState.finalCode = codeAuthInvalidPayload
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	logState.usernameHash = shortHash(strings.ToLower(strings.TrimSpace(req.Username)))
	if !s.requireCaptchaToken(c, req.CaptchaToken) {
		logState.finalStatus = c.Response().StatusCode()
		switch strings.TrimSpace(req.CaptchaToken) {
		case "":
			logState.finalCode = codeAuthCaptchaRequired
		default:
			logState.finalCode = codeAuthCaptchaInvalid
			if c.Response().StatusCode() == fiber.StatusServiceUnavailable {
				logState.finalCode = codeAuthCaptchaUnavailable
			}
		}
		return nil
	}
	logState.captchaAccepted = true
	logState.markStage(loginStageCaptchaOK)
	if usernameLength(strings.TrimSpace(req.Username)) < minUsernameLength || len(req.Password) < minPasswordLength {
		logState.finalStatus = fiber.StatusBadRequest
		logState.finalCode = codeAuthCredentialTooShort
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthCredentialTooShort, "Username or password is too short", "")
	}
	usernameKey := strings.ToLower(strings.TrimSpace(req.Username))
	locked, retryAfter, guardErr := s.loginGuard.IsLocked(c.IP(), usernameKey, time.Now())
	if guardErr != nil {
		logState.finalStatus = fiber.StatusServiceUnavailable
		logState.finalCode = codeAuthLoginGuardUnavailable
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthLoginGuardUnavailable, "login guard backend unavailable", guardErr.Error())
	}
	logState.markStage(loginStageLoginGuardChecked)

	if locked {
		c.Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		logState.finalStatus = fiber.StatusTooManyRequests
		logState.finalCode = codeAuthLoginLocked
		return s.writeAPIError(c, fiber.StatusTooManyRequests, codeAuthLoginLocked, "Too many failed login attempts, please retry later", "")
	}
	authState, err := loginGetUserAuthState(s, c.Context(), req.Username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if guardErr := s.loginGuard.RecordFailure(c.IP(), usernameKey, time.Now()); guardErr != nil {
				logState.finalStatus = fiber.StatusServiceUnavailable
				logState.finalCode = codeAuthLoginGuardUnavailable
				return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthLoginGuardUnavailable, "login guard backend unavailable", guardErr.Error())
			}
			time.Sleep(150 * time.Millisecond)
			logState.finalStatus = fiber.StatusUnauthorized
			logState.finalCode = codeAuthInvalidCredentials
			return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidCredentials, "Invalid username or password", "")
		}
		logState.finalStatus = fiber.StatusInternalServerError
		logState.finalCode = codeAuthLookupUserFailed
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthLookupUserFailed, "Failed to validate credentials", err.Error())
	}
	logState.userID = authState.UserID
	logState.markStage(loginStageAuthStateLoaded)
	if err := bcrypt.CompareHashAndPassword([]byte(authState.PasswordHash), []byte(req.Password)); err != nil {
		if guardErr := s.loginGuard.RecordFailure(c.IP(), usernameKey, time.Now()); guardErr != nil {
			logState.finalStatus = fiber.StatusServiceUnavailable
			logState.finalCode = codeAuthLoginGuardUnavailable
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthLoginGuardUnavailable, "login guard backend unavailable", guardErr.Error())
		}
		time.Sleep(150 * time.Millisecond)
		logState.finalStatus = fiber.StatusUnauthorized
		logState.finalCode = codeAuthInvalidCredentials
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidCredentials, "Invalid username or password", "")
	}
	logState.markStage(loginStagePasswordVerified)
	if reason, bannedUntil := activeBanState(authState, time.Now()); reason != "" {
		detail := reason
		if !bannedUntil.IsZero() {
			detail = reason + "|" + bannedUntil.UTC().Format(time.RFC3339)
		}
		logState.finalStatus = fiber.StatusForbidden
		logState.finalCode = codeAuthAccountBanned
		return s.writeAPIError(c, fiber.StatusForbidden, codeAuthAccountBanned, "Account has been banned", "", detail)
	}
	if guardErr := s.loginGuard.Reset(c.IP(), usernameKey); guardErr != nil {
		logState.finalStatus = fiber.StatusServiceUnavailable
		logState.finalCode = codeAuthLoginGuardUnavailable
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthLoginGuardUnavailable, "login guard backend unavailable", guardErr.Error())
	}
	deviceHash := computeDeviceHash(req.DeviceProfile)
	if authState.EmailVerified {
		skipIPVerify := false
		if deviceHash != "" {
			if trusted, trustErr := loginIsTrustedDevice(s, c.Context(), authState.UserID, deviceHash); trustErr == nil && trusted {
				skipIPVerify = true
			}
		}
		if !skipIPVerify {
			disableIPVerify := false
			if userSettings, settingsErr := loginGetUserSettings(s, c.Context(), authState.UserID); settingsErr == nil {
				disableIPVerify = userSettings.DisableIPLoginVerify
				logState.markStage(loginStageUserSettingsLoaded)
			}
			if !disableIPVerify {
				ipCount, hasAnyIP, ipErr := loginCheckIPTrust(s, c.Context(), authState.UserID, c.IP())
				if ipErr == nil && hasAnyIP && ipCount == 0 {
					scopeKey := strconv.FormatInt(authState.UserID, 10)
					logState.markStage(loginStageIPTrustChecked)
					code, issueErr := loginIssueRecoveryCode(s, c.Context(), "ip_login_verify", scopeKey, c.IP(), c.Get(fiber.HeaderUserAgent))
					if issueErr == nil {
						_ = loginSendRecoveryCodeEmail(s, authState.Email, strings.TrimSpace(req.Username), "异地登录验证", code, authRecoveryCodeTTL)
						_ = loginClearRecoveryAttempts(s, c.Context(), "ip_login_verify", scopeKey)
					}
					pendingToken, storeErr := loginStorePendingIPLogin(s, c.Context(), authState.UserID, c.IP(), deviceHash)
					if storeErr != nil {
						logState.finalStatus = fiber.StatusInternalServerError
						logState.finalCode = codeAuthSignTokenFailed
						return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", storeErr.Error())
					}
					logState.ipVerifyRequired = true
					logState.finalStatus = fiber.StatusForbidden
					logState.finalCode = codeLoginIPVerifyRequired
					return s.writeAPIError(c, fiber.StatusForbidden, codeLoginIPVerifyRequired, "Login from new location requires email verification", "", pendingToken)
				}
				if ipErr == nil {
					logState.markStage(loginStageIPTrustChecked)
				}
			}
		}
	}
	seeds, err := loginPlayerAttributeSeeds(s)
	if err != nil {
		logState.finalStatus = fiber.StatusInternalServerError
		logState.finalCode = "PLAYER_ATTRIBUTE_SEEDS_INVALID"
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTE_SEEDS_INVALID", "player attribute seeds invalid", err.Error())
	}
	if err := loginEnsurePlayerAttributes(s, c.Context(), authState.UserID, seeds); err != nil {
		logState.finalStatus = fiber.StatusInternalServerError
		logState.finalCode = "PLAYER_ATTRIBUTE_INIT_FAILED"
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTE_INIT_FAILED", "initialize player attributes failed", err.Error())
	}
	logState.markStage(loginStageAttributesEnsured)
	if err := loginRecomputeDerivedStats(s, c.Context(), authState.UserID); err != nil {
		logState.finalStatus = fiber.StatusInternalServerError
		logState.finalCode = "PLAYER_STATS_RECOMPUTE_FAILED"
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_STATS_RECOMPUTE_FAILED", "recompute stats failed", err.Error())
	}
	logState.markStage(loginStageStatsRecomputed)
	presence, err := loginGetPresence(s, c.Context(), authState.UserID)
	if err != nil {
		logState.finalStatus = fiber.StatusInternalServerError
		logState.finalCode = codeAuthSignTokenFailed
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to read session state", err.Error())
	}
	logState.markStage(loginStagePresenceLoaded)
	var sessionVersion int64
	if presence.SessionVersion <= 0 {
		sessionVersion, err = loginRotateSessionVersion(s, c.Context(), authState.UserID)
		if err != nil {
			logState.finalStatus = fiber.StatusInternalServerError
			logState.finalCode = codeAuthSignTokenFailed
			return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
		}
		logState.sessionRotated = true
	} else {
		sessionVersion = presence.SessionVersion
	}
	logState.markStage(loginStageSessionReady)
	token, err := loginSignToken(s, authState.UserID, sessionVersion)
	if err != nil {
		logState.finalStatus = fiber.StatusInternalServerError
		logState.finalCode = codeAuthSignTokenFailed
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
	}
	logState.markStage(loginStageTokenSigned)
	_ = loginRecordIPLogin(s, c.Context(), authState.UserID, c.IP())
	if deviceHash != "" {
		_ = loginTrustDevice(s, c.Context(), authState.UserID, deviceHash, trustedDeviceTTL)
	}
	logState.markStage(loginStageIPLoginRecorded)
	s.recordRiskSignal(c, authState.UserID, "login", req.DeviceProfile, req.Password, map[string]any{
		"username": strings.TrimSpace(req.Username),
	})
	logState.finalStatus = fiber.StatusOK
	logState.finalCode = ""
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"userId": authState.UserID, "token": token})
}

func isValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func usernameLength(username string) int {
	return utf8.RuneCountInString(username)
}

func isValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

func normalizePolicyVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return trimmed
}

func activeBanState(authState store.UserAuthState, now time.Time) (string, time.Time) {
	reason := strings.TrimSpace(authState.BanReason)
	if reason == "" {
		return "", time.Time{}
	}
	if authState.BanIsPermanent {
		return reason, time.Time{}
	}
	if authState.BanUntil != nil && authState.BanUntil.After(now) {
		return reason, authState.BanUntil.UTC()
	}
	return "", time.Time{}
}

const pendingIPLoginTTL = 10 * time.Minute

type pendingIPLoginState struct {
	UserID     int64  `json:"userId"`
	IP         string `json:"ip"`
	DeviceHash string `json:"deviceHash,omitempty"`
}

func pendingIPLoginKey(token string) string {
	return "auth:login:ip-pending:" + strings.TrimSpace(token)
}

func (s *Server) storePendingIPLogin(ctx context.Context, userID int64, ip, deviceHash string) (string, error) {
	if s.deps.Redis == nil {
		return "", errors.New("redis unavailable")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	state := pendingIPLoginState{UserID: userID, IP: ip, DeviceHash: deviceHash}
	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	if err := s.deps.Redis.Set(ctx, pendingIPLoginKey(token), string(data), pendingIPLoginTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) lookupPendingIPLogin(ctx context.Context, token string) (pendingIPLoginState, error) {
	if s.deps.Redis == nil {
		return pendingIPLoginState{}, errors.New("redis unavailable")
	}
	raw, err := s.deps.Redis.Get(ctx, pendingIPLoginKey(token)).Result()
	if err != nil {
		return pendingIPLoginState{}, err
	}
	var state pendingIPLoginState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return pendingIPLoginState{}, err
	}
	return state, nil
}

func (s *Server) deletePendingIPLogin(ctx context.Context, token string) {
	if s.deps.Redis != nil {
		_ = s.deps.Redis.Del(ctx, pendingIPLoginKey(token)).Err()
	}
}

type loginIPVerifyRequest struct {
	PendingToken string `json:"pendingToken"`
	Code         string `json:"code"`
}

func (s *Server) handleLoginIPVerify(c *fiber.Ctx) error {
	var req loginIPVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Invalid request payload", err.Error())
	}
	if strings.TrimSpace(req.PendingToken) == "" || strings.TrimSpace(req.Code) == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeAuthInvalidPayload, "Missing pending token or code", "")
	}
	pending, err := s.lookupPendingIPLogin(c.Context(), req.PendingToken)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusForbidden, codeLoginIPPendingExpired, "Login session expired, please log in again", "")
	}
	scopeKey := strconv.FormatInt(pending.UserID, 10)
	attempts, bumpErr := s.bumpRecoveryCodeAttempt(c.Context(), "ip_login_verify", scopeKey)
	if bumpErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to verify code", bumpErr.Error())
	}
	if attempts > int64(authRecoveryCodeMaxFails) {
		s.deletePendingIPLogin(c.Context(), req.PendingToken)
		return s.writeAPIError(c, fiber.StatusForbidden, codeAuthVerifyCodeTooMany, "Too many failed attempts", "")
	}
	ok, consumeErr := s.consumeRecoveryCode(c.Context(), "ip_login_verify", scopeKey, req.Code, c.IP(), c.Get(fiber.HeaderUserAgent))
	if consumeErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to verify code", consumeErr.Error())
	}
	if !ok {
		return s.writeAPIError(c, fiber.StatusForbidden, codeAuthVerifyCodeInvalid, "Invalid verification code", "")
	}
	s.deletePendingIPLogin(c.Context(), req.PendingToken)
	_ = s.clearRecoveryCodeAttempts(c.Context(), "ip_login_verify", scopeKey)
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTE_SEEDS_INVALID", "player attribute seeds invalid", err.Error())
	}
	if err := s.deps.Store.EnsurePlayerAttributes(c.Context(), pending.UserID, seeds); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTE_INIT_FAILED", "initialize player attributes failed", err.Error())
	}
	if err := s.deps.Store.RecomputeDerivedStats(c.Context(), pending.UserID); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_STATS_RECOMPUTE_FAILED", "recompute stats failed", err.Error())
	}
	presence, err := s.deps.Store.GetPresence(c.Context(), pending.UserID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to read session state", err.Error())
	}
	var sessionVersion int64
	if presence.SessionVersion <= 0 {
		sessionVersion, err = s.deps.Store.RotateSessionVersion(c.Context(), pending.UserID)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
		}
	} else {
		sessionVersion = presence.SessionVersion
	}
	token, err := s.deps.Auth.Sign(pending.UserID, sessionVersion)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
	}
	_ = s.deps.Store.RecordIPLogin(c.Context(), pending.UserID, pending.IP)
	if pending.DeviceHash != "" {
		_ = s.deps.Store.TrustDevice(c.Context(), pending.UserID, pending.DeviceHash, trustedDeviceTTL)
	}
	s.recordRiskSignal(c, pending.UserID, "login", risk.DeviceProfile{}, "", map[string]any{
		"ip_verify": true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"userId": pending.UserID, "token": token})
}

type marketListRequest struct {
	ItemID   string `json:"itemId"`
	Price    int64  `json:"price"`
	Quantity int64  `json:"quantity"`
}

type marketBuyOrderRequest struct {
	ItemID   string `json:"itemId"`
	Price    int64  `json:"price"`
	Quantity int64  `json:"quantity"`
}

type marketFillBuyOrderRequest struct {
	OrderID  int64 `json:"orderId"`
	Quantity int64 `json:"quantity"`
}

type marketCancelBuyOrderRequest struct {
	OrderID int64 `json:"orderId"`
}
