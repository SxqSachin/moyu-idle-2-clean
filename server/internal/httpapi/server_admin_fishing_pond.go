package httpapi

import (
	"context"
	"errors"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

var errAdminFishingPondUnavailable = errors.New("admin fishing pond unavailable")

var adminFishingPondFishAddExecutor = func(s *Server, ctx context.Context, input store.AdminFishingPondFishInput) (store.AdminFishingPondFishResult, error) {
	if s.deps.Store == nil {
		return store.AdminFishingPondFishResult{}, errAdminFishingPondUnavailable
	}
	return s.deps.Store.AdminAddFishingPondFish(ctx, input)
}

func (s *Server) handleAdminFishingPondFishAdd(c *fiber.Ctx) error {
	var input store.AdminFishingPondFishInput
	if err := c.BodyParser(&input); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_FISHING_POND_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if input.UserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_FISHING_POND_INVALID_USER_ID", "invalid user id", "")
	}
	result, err := adminFishingPondFishAddExecutor(s, c.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, errAdminFishingPondUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_FISHING_POND_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrAdminFishingPondFishInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_FISHING_POND_INVALID_FISH", "invalid fish overrides", err.Error())
		case errors.Is(err, store.ErrFishingPondNotBuilt):
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_FISHING_POND_NOT_BUILT", "fishing pond is not built", err.Error())
		case errors.Is(err, store.ErrFishingPondFull):
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_FISHING_POND_FULL", "fishing pond capacity is full", err.Error())
		case errors.Is(err, store.ErrFishingPondBiomassFull):
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_FISHING_POND_BIOMASS_FULL", "fishing pond biomass capacity is full", err.Error())
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_FISHING_POND_FAILED", "add fishing pond fish failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{input.UserID}, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":         true,
		"userId":     input.UserID,
		"instanceId": result.InstanceID,
		"customData": result.CustomData,
		"fish":       result.Fish,
	})
}
