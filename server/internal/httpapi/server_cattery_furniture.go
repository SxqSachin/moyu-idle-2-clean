package httpapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func (s *Server) furnitureError(c *fiber.Ctx, err error) error {
	status, code := fiber.StatusInternalServerError, "FURNITURE_UNAVAILABLE"
	switch {
	case errors.Is(err, store.ErrUserBanned):
		status, code = fiber.StatusForbidden, "FURNITURE_UNAVAILABLE"
	case errors.Is(err, store.ErrFurnitureConflict):
		status, code = fiber.StatusConflict, "FURNITURE_CONFLICT"
	case errors.Is(err, store.ErrInsufficientItem):
		status, code = fiber.StatusConflict, "FURNITURE_MATERIALS"
	case errors.Is(err, store.ErrOperationEffectRecoveryPending):
		status, code = fiber.StatusConflict, "FURNITURE_RECOVERY_PENDING"
	case errors.Is(err, store.ErrFurnitureCapacity):
		status, code = fiber.StatusConflict, "FURNITURE_CAPACITY"
	case errors.Is(err, store.ErrFurnitureHistory):
		status, code = fiber.StatusConflict, "FURNITURE_HISTORY"
	case errors.Is(err, store.ErrFurniturePending):
		status, code = fiber.StatusConflict, "FURNITURE_PENDING"
	case errors.Is(err, store.ErrCatteryUnavailable):
		status, code = fiber.StatusConflict, "FURNITURE_CATTERY_REQUIRED"
	case errors.Is(err, store.ErrFurnitureInvalid):
		status, code = fiber.StatusBadRequest, "FURNITURE_INVALID"
	}
	return s.writeAPIError(c, status, code, "Furniture request could not be completed", "")
}
func (s *Server) handleFurnitureGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, 401, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var projection store.FurnitureProjection
	if c.Query("view") == "summary" {
		projection, err = s.deps.Store.GetFurnitureSummary(c.Context(), userID)
	} else {
		projection, err = s.deps.Store.GetFurniture(c.Context(), userID)
	}
	if err != nil {
		return s.furnitureError(c, err)
	}
	return s.writeAPISuccess(c, 200, projection)
}
func (s *Server) handleFurnitureCommand(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, 401, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var cmd store.FurnitureCommand
	if err = c.BodyParser(&cmd); err != nil {
		return s.furnitureError(c, store.ErrFurnitureInvalid)
	}
	projection, err := s.deps.Store.CommandFurniture(c.Context(), userID, cmd)
	if err != nil {
		return s.furnitureError(c, err)
	}
	if cmd.Action == "install" || cmd.Action == "refill" || cmd.Action == "upgrade" {
		s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	}
	s.pushUserEventClusterOnce(userID, "game.cattery.furniture", projection)
	return s.writeAPISuccess(c, 200, projection)
}

func (s *Server) handleFurnitureReport(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, 401, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	ack := ""
	if c.Method() == fiber.MethodPost {
		var body struct {
			Token string `json:"token"`
		}
		if c.BodyParser(&body) != nil || len(body.Token) != 64 {
			return s.furnitureError(c, store.ErrFurnitureInvalid)
		}
		ack = body.Token
	}
	report, err := s.deps.Store.FurnitureReport(c.Context(), userID, ack)
	if err != nil {
		return s.furnitureError(c, err)
	}
	return s.writeAPISuccess(c, 200, report)
}
