package httpapi

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

const alchemyInheritImmediateCooldown = 10 * time.Second

type fishingInfusionPreviewRequest struct {
	FishInstanceID int64 `json:"fishInstanceId"`
}

func (s *Server) handleAlchemyDiscoveries(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return err
	}
	discoveries, err := s.deps.Store.GetAlchemyDiscoveries(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ALCHEMY_DISCOVERIES_FAILED", "failed to load alchemy discoveries", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, discoveries)
}

func (s *Server) handleFishingInfusionPreview(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingInfusionPreviewRequest
	if err := c.BodyParser(&request); err != nil || request.FishInstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ALCHEMY_FISH_INVALID", "choose a stored fish", "")
	}
	preview, err := s.deps.Store.FishingInfusionPreview(c.Context(), userID, request.FishInstanceID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidInstanceState), errors.Is(err, store.ErrInstanceNotOwned):
			return s.writeAPIError(c, fiber.StatusConflict, "ALCHEMY_FISH_UNAVAILABLE", "that fish is no longer available", err.Error())
		case errors.Is(err, store.ErrAlchemyArgsInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ALCHEMY_FISH_INVALID", "this fish cannot be infused", err.Error())
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ALCHEMY_FISH_PREVIEW_FAILED", "fish infusion preview failed", err.Error())
		}
	}
	return s.writeAPISuccess(c, fiber.StatusOK, preview)
}

func (s *Server) handleAlchemyInheritImmediate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if retryAfter := s.checkAlchemyInheritImmediateCooldown(c, userID); retryAfter > 0 {
		retryAfterSeconds := ceilDurationSeconds(retryAfter)
		c.Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		return s.writeAPIError(c, fiber.StatusTooManyRequests, "ALCHEMY_INHERIT_COOLDOWN", "fusion is cooling down", fmt.Sprintf("retryAfterSeconds=%d", retryAfterSeconds))
	}
	result, err := s.deps.Store.AlchemyInheritImmediate(c.Context(), userID, c.BodyRaw())
	if err != nil {
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "ALCHEMY_INHERIT_INSUFFICIENT_ITEM", "not enough items for inheritance", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "ALCHEMY_INHERIT_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "ALCHEMY_INHERIT_INSTANCE_INVALID_STATE", "instance state must be inventory", "")
		}
		if errors.Is(err, store.ErrAlchemyKindMismatch) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ALCHEMY_INHERIT_KIND_MISMATCH", "source affix kinds do not match", "")
		}
		if errors.Is(err, store.ErrAlchemyCarrierOccupied) || errors.Is(err, store.ErrAffixInvalidCarrier) || errors.Is(err, store.ErrAffixRarityNotAllowed) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ALCHEMY_INHERIT_INVALID_CARRIER", "invalid inheritance carrier", "")
		}
		if errors.Is(err, store.ErrAlchemyMediumUnknown) || errors.Is(err, store.ErrAlchemyArgsInvalid) || errors.Is(err, store.ErrAffixSlotNotFound) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ALCHEMY_INHERIT_INVALID_REQUEST", "invalid inheritance request", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ALCHEMY_INHERIT_FAILED", "inheritance failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) checkAlchemyInheritImmediateCooldown(c *fiber.Ctx, userID int64) time.Duration {
	if userID <= 0 {
		return 0
	}
	if s.deps.Redis != nil {
		key := "alchemy:inherit:cooldown:" + strconv.FormatInt(userID, 10)
		ok, err := s.deps.Redis.SetNX(c.Context(), key, "1", alchemyInheritImmediateCooldown).Result()
		if err == nil && ok {
			return 0
		}
		if err == nil && !ok {
			ttl, ttlErr := s.deps.Redis.TTL(c.Context(), key).Result()
			if ttlErr == nil && ttl > 0 {
				return ttl
			}
			return alchemyInheritImmediateCooldown
		}
	}
	return s.checkAlchemyInheritImmediateCooldownFallback(userID)
}

func (s *Server) checkAlchemyInheritImmediateCooldownFallback(userID int64) time.Duration {
	now := time.Now()
	s.alchemyInheritCooldownMu.Lock()
	defer s.alchemyInheritCooldownMu.Unlock()
	if s.alchemyInheritCooldownUntil == nil {
		s.alchemyInheritCooldownUntil = make(map[int64]time.Time)
	}
	until := s.alchemyInheritCooldownUntil[userID]
	if until.After(now) {
		return until.Sub(now)
	}
	for id, ts := range s.alchemyInheritCooldownUntil {
		if ts.Before(now) {
			delete(s.alchemyInheritCooldownUntil, id)
		}
	}
	s.alchemyInheritCooldownUntil[userID] = now.Add(alchemyInheritImmediateCooldown)
	return 0
}

func ceilDurationSeconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int((value + time.Second - time.Nanosecond) / time.Second)
}
