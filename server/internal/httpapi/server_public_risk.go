package httpapi

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/riskinsight"
	"mysecondwebgame/server/internal/store"
)

const (
	publicRiskMinCatVillageLevel = 20
	publicRiskCooldown           = 30 * time.Second
)

func (s *Server) handlePublicRiskNoticeboard(c *fiber.Ctx) error {
	if s.deps.RiskInsight == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "PUBLIC_RISK_UNAVAILABLE", "public risk service unavailable", "")
	}
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	category, err := riskinsight.NormalizeCategory(c.Query("category"))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PUBLIC_RISK_INVALID_CATEGORY", "invalid public risk category", "")
	}
	rep, err := s.deps.Store.GetCatVillageReputation(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PUBLIC_RISK_REPUTATION_FAILED", "get reputation failed", err.Error())
	}
	repState := store.ComputeRepLevel(rep, s.deps.Config.CatVillageSettings.RepLevels)
	if repState.Level < publicRiskMinCatVillageLevel {
		return s.writeAPIError(c, fiber.StatusForbidden, "PUBLIC_RISK_REPUTATION_TOO_LOW", "cat village reputation level is too low", "")
	}
	if retryAfter := s.checkPublicRiskCooldown(c, userID); retryAfter > 0 {
		return s.writeAPIError(c, fiber.StatusTooManyRequests, "PUBLIC_RISK_RATE_LIMITED", "please wait before refreshing", fmt.Sprintf("retryAfterSeconds=%d", int(retryAfter.Seconds())+1))
	}
	snapshot, err := s.deps.RiskInsight.GetPublicNoticeboard(c.Context(), category)
	if err != nil {
		if errors.Is(err, riskinsight.ErrInvalidCategory) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "PUBLIC_RISK_INVALID_CATEGORY", "invalid public risk category", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PUBLIC_RISK_LOAD_FAILED", "load public risk noticeboard failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, snapshot)
}

func (s *Server) checkPublicRiskCooldown(c *fiber.Ctx, userID int64) time.Duration {
	if userID <= 0 {
		return 0
	}
	if s.deps.Redis != nil {
		key := "risk:public_noticeboard:cooldown:" + strconv.FormatInt(userID, 10)
		ok, err := s.deps.Redis.SetNX(c.Context(), key, "1", publicRiskCooldown).Result()
		if err == nil && ok {
			return 0
		}
		if err == nil && !ok {
			ttl, ttlErr := s.deps.Redis.TTL(c.Context(), key).Result()
			if ttlErr == nil && ttl > 0 {
				return ttl
			}
			return publicRiskCooldown
		}
		if err != nil && !errors.Is(err, redis.Nil) {
			return s.checkPublicRiskCooldownFallback(userID)
		}
	}
	return s.checkPublicRiskCooldownFallback(userID)
}

func (s *Server) checkPublicRiskCooldownFallback(userID int64) time.Duration {
	now := time.Now()
	s.publicRiskCooldownMu.Lock()
	defer s.publicRiskCooldownMu.Unlock()
	until := s.publicRiskCooldownUntil[userID]
	if until.After(now) {
		return until.Sub(now)
	}
	for id, ts := range s.publicRiskCooldownUntil {
		if ts.Before(now) {
			delete(s.publicRiskCooldownUntil, id)
		}
	}
	s.publicRiskCooldownUntil[userID] = now.Add(publicRiskCooldown)
	return 0
}
