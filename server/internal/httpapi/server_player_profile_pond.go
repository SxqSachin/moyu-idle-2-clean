package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

type publicFishingPondReader interface {
	GetPublicFishingPond(context.Context, int64) (store.FishingPondPublicOverview, error)
}

func (s *Server) handlePlayerProfilePondGet(c *fiber.Ctx) error {
	return s.servePlayerProfilePondGet(c, s.deps.Store)
}

func (s *Server) servePlayerProfilePondGet(c *fiber.Ctx, reader publicFishingPondReader) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}

	userID, err := strconv.ParseInt(strings.TrimSpace(c.Params("userID")), 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_PROFILE_POND_INVALID_USER_ID", "invalid user id", "")
	}

	pond, err := reader.GetPublicFishingPond(c.Context(), userID)
	if errors.Is(err, store.ErrUserNotFound) {
		return s.writeAPIError(c, fiber.StatusNotFound, "PLAYER_PROFILE_POND_NOT_FOUND", "player not found", "")
	}
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_POND_GET_FAILED", "get player pond failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, pond)
}
