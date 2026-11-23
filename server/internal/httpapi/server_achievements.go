package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) handleAchievementsList(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	state, err := s.deps.Store.ListPlayerAchievements(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ACHIEVEMENTS_LIST_FAILED", "list achievements failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, state)
}

func (s *Server) handleAchievementClaim(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	achievementID := strings.TrimSpace(c.Params("achievementID"))
	if achievementID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ACHIEVEMENT_ID_REQUIRED", "achievement id is required", "")
	}
	result, err := s.deps.Store.ClaimAchievementReward(c.Context(), userID, achievementID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrAchievementNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ACHIEVEMENT_NOT_FOUND", "achievement not found", "")
		case errors.Is(err, store.ErrAchievementNotCompleted):
			return s.writeAPIError(c, fiber.StatusConflict, "ACHIEVEMENT_NOT_COMPLETED", "achievement not completed", "")
		case errors.Is(err, store.ErrAchievementRewardClaimed):
			return s.writeAPIError(c, fiber.StatusConflict, "ACHIEVEMENT_REWARD_CLAIMED", "achievement reward already claimed", "")
		case errors.Is(err, store.ErrAchievementRewardInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ACHIEVEMENT_REWARD_INVALID", "achievement reward invalid", err.Error())
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ACHIEVEMENT_CLAIM_FAILED", "claim achievement failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:      []int64{userID},
		Inventory:    result.InventoryChanged,
		Wardrobe:     result.WardrobeChanged,
		Achievements: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "result": result})
}

func (s *Server) pushAchievements(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Use the read-only snapshot: the caller (e.g. ClaimAchievementReward) already ran a
	// full reconcile inside its own transaction, so the DB is already fresh. Re-running
	// reconcile here would duplicate all the state COUNT queries for no benefit.
	state, err := s.deps.Store.ListPlayerAchievementsCurrentStates(ctx, userID)
	if err != nil {
		return
	}
	s.pushUserEventClusterOnce(userID, "game.achievements.patch", state)
}

func (s *Server) NotifyAchievementUsersProcessed(userIDs []int64) {
	seen := make(map[int64]struct{}, len(userIDs))
	unique := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		unique = append(unique, userID)
	}
	if len(unique) == 0 {
		return
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:      unique,
		Achievements: true,
	})
}
