package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func (s *Server) handleWarehouseChangeLogs(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	filter := store.WarehouseChangeLogFilter{
		Category: store.InventoryAuditCategory(c.Query("category")),
	}
	items, effectiveSince, err := s.deps.Store.ListWarehouseChangeLogs(c.Context(), userID, filter)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "WAREHOUSE_LOG_LIST_FAILED", "list warehouse logs failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"since": effectiveSince,
	})
}
