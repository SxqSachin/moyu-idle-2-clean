package httpapi

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

const (
	adminInventoryGrantModeTemplate = "template"
	adminInventoryGrantModeEnhanced = "enhanced"
)

type adminInventoryGrantRequest struct {
	UserID       int64  `json:"userId"`
	ItemID       string `json:"itemId"`
	Quantity     int64  `json:"quantity"`
	GrantMode    string `json:"grantMode"`
	EnhanceLevel int    `json:"enhanceLevel"`
}

type adminInventoryGrantCommand struct {
	UserID       int64
	ItemID       string
	Quantity     int64
	GrantMode    string
	EnhanceLevel int
}

var errAdminInventoryGrantUnavailable = errors.New("admin inventory grant unavailable")

var adminInventoryGrantExecutor = func(s *Server, ctx context.Context, cmd adminInventoryGrantCommand) error {
	if s.deps.Store == nil {
		return errAdminInventoryGrantUnavailable
	}
	return s.deps.Store.AdminGrantInventory(ctx, store.AdminInventoryGrantInput{
		UserID:       cmd.UserID,
		ItemID:       cmd.ItemID,
		Quantity:     cmd.Quantity,
		GrantMode:    cmd.GrantMode,
		EnhanceLevel: cmd.EnhanceLevel,
	})
}

func normalizeAdminInventoryGrantMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", adminInventoryGrantModeTemplate:
		return adminInventoryGrantModeTemplate
	case adminInventoryGrantModeEnhanced:
		return adminInventoryGrantModeEnhanced
	default:
		return ""
	}
}

func (s *Server) handleAdminInventoryGrant(c *fiber.Ctx) error {
	var req adminInventoryGrantRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if req.UserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_USER_ID", "invalid user id", "")
	}
	itemID := strings.TrimSpace(req.ItemID)
	if itemID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_ITEM_ID", "invalid item id", "")
	}
	quantity := req.Quantity
	if quantity == 0 {
		quantity = 1
	}
	if quantity < 0 || quantity > 1_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_QUANTITY", "invalid quantity", "")
	}
	grantMode := normalizeAdminInventoryGrantMode(req.GrantMode)
	if grantMode == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_GRANT_MODE", "invalid grant mode", "")
	}
	if grantMode == adminInventoryGrantModeEnhanced && req.EnhanceLevel <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_ENHANCE_LEVEL", "invalid enhance level", "")
	}
	if err := adminInventoryGrantExecutor(s, c.Context(), adminInventoryGrantCommand{
		UserID:       req.UserID,
		ItemID:       itemID,
		Quantity:     quantity,
		GrantMode:    grantMode,
		EnhanceLevel: req.EnhanceLevel,
	}); err != nil {
		switch {
		case errors.Is(err, errAdminInventoryGrantUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_INVENTORY_GRANT_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrBadQuantity):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_QUANTITY", "invalid quantity", "")
		case errors.Is(err, store.ErrInvalidItemID):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_ITEM_ID", "invalid item id", "")
		case errors.Is(err, store.ErrAdminInventoryGrantModeInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_GRANT_MODE", "invalid grant mode", "")
		case errors.Is(err, store.ErrAdminInventoryGrantEnhanceLevelInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_INVENTORY_GRANT_INVALID_ENHANCE_LEVEL", "invalid enhance level", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_INVENTORY_GRANT_FAILED", "admin inventory grant failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:   []int64{req.UserID},
		Inventory: true,
	})
	resp := fiber.Map{
		"ok":        true,
		"userId":    req.UserID,
		"itemId":    itemID,
		"quantity":  quantity,
		"grantMode": grantMode,
	}
	if grantMode == adminInventoryGrantModeEnhanced {
		resp["enhanceLevel"] = req.EnhanceLevel
	}
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}
