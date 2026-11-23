package httpapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/store"
)

func (s *Server) handleAdminUserDetail(c *fiber.Ctx) error {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Params("userID")), 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_INVALID_USER_ID", "invalid user id", "")
	}
	item, err := s.deps.Store.GetAdminUserDetail(c.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_USER_DETAIL_NOT_FOUND", "user not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_USER_DETAIL_FAILED", "admin user detail failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, item)
}

func (s *Server) handleAdminBannedUsers(c *fiber.Ctx) error {
	limit := 50
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_BANNED_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	items, err := s.deps.Store.ListAdminBannedUsers(c.Context(), store.AdminBannedUserFilter{
		Query: strings.TrimSpace(c.Query("q")),
		Limit: limit,
	})
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_USER_BANNED_LIST_FAILED", "admin banned users list failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": items})
}
