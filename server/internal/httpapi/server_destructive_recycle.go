package httpapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/store"
)

type destructiveRecycleRequest struct {
	Items []store.DestructiveRecycleInput `json:"items"`
}

func (s *Server) handleDestructiveRecycleQuote(c *fiber.Ctx) error {
	return s.handleDestructiveRecycleRequest(c, false)
}

func (s *Server) handleDestructiveRecycle(c *fiber.Ctx) error {
	return s.handleDestructiveRecycleRequest(c, true)
}

func (s *Server) handleDestructiveRecycleRequest(c *fiber.Ctx, confirm bool) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req destructiveRecycleRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil || len(req.Items) == 0 || len(req.Items) > store.RecycleBatchItemLimit {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_INVALID_REQUEST", "invalid request", "")
	}
	var result store.DestructiveRecycleQuote
	if confirm {
		result, err = s.deps.Store.DestructiveRecycle(c.Context(), userID, req.Items)
	} else {
		result, err = s.deps.Store.QuoteDestructiveRecycle(c.Context(), userID, req.Items)
	}
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInstanceNotOwned):
			return s.writeAPIError(c, fiber.StatusForbidden, "ITEM_RECYCLE_INSTANCE_NOT_OWNED", "instance not owned", "")
		case errors.Is(err, store.ErrDestructiveRecycleChanged), errors.Is(err, pgx.ErrNoRows), errors.Is(err, store.ErrInvalidInstanceState), errors.Is(err, store.ErrInsufficientItem), errors.Is(err, store.ErrFishingTackleInUse):
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_DESTRUCTIVE_RECYCLE_CHANGED", "equipment changed; preview again", "")
		case errors.Is(err, store.ErrBadQuantity), errors.Is(err, store.ErrItemNotRecyclable), errors.Is(err, store.ErrRecyclePriceInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_DESTRUCTIVE_RECYCLE_INELIGIBLE", "equipment not eligible for destructive recycling", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_RECYCLE_FAILED", "recycle failed", err.Error())
		}
	}
	if confirm {
		s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: true})
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}
