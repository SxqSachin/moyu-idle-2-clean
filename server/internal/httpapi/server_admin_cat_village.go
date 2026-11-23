package httpapi

import (
	"github.com/gofiber/fiber/v2"
)

type adminCatVillageGenerateRequest struct {
	UserID       int64  `json:"userId"`
	Count        int    `json:"count"`
	CommissionID string `json:"commissionId"`
}

func (s *Server) handleAdminCatVillageGenerate(c *fiber.Ctx) error {
	var req adminCatVillageGenerateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_CAT_VILLAGE_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if req.UserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_CAT_VILLAGE_INVALID_USER", "userId required", "")
	}
	if req.CommissionID != "" {
		if err := s.deps.Store.ForceInsertSpecificCommission(c.Context(), req.UserID, req.CommissionID, s.deps.GameData.CatVillageCommissions); err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_CAT_VILLAGE_INSERT_FAILED", "insert commission failed", err.Error())
		}
		s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{req.UserID}, PlayerState: true})
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"inserted": req.CommissionID, "userId": req.UserID})
	}
	count := req.Count
	if count <= 0 {
		count = 3
	}
	generated, err := s.deps.Store.ForceGenerateCatVillageCommissions(c.Context(), req.UserID, count, s.deps.GameData.CatVillageCommissions)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_CAT_VILLAGE_GENERATE_FAILED", "generate commissions failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{req.UserID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"generated": generated, "userId": req.UserID})
}
