package httpapi

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) playerSpecializationBattleContribution(ctx context.Context, userID int64) (store.SpecializationBattleContribution, error) {
	return s.deps.Store.GetPlayerSpecializationBattleContribution(ctx, userID)
}

type playerSpecializationAllocateRequest struct {
	NodeID                 string `json:"nodeId"`
	ExpectedRevision       int64  `json:"expectedRevision"`
	ExpectedRulesetVersion string `json:"expectedRulesetVersion"`
	ImpactDigest           string `json:"impactDigest"`
}

type playerSpecializationResetRequest struct {
	ExpectedRevision       int64  `json:"expectedRevision"`
	ExpectedRulesetVersion string `json:"expectedRulesetVersion"`
	ImpactDigest           string `json:"impactDigest"`
}

func (s *Server) handlePlayerSpecializationGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	treeID := strings.TrimSpace(c.Params("treeId"))
	if treeID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_TREE_ID_REQUIRED", "specialization tree id required", "")
	}
	view, err := s.deps.Store.GetPlayerSpecialization(c.Context(), userID, treeID)
	if err != nil {
		log.Printf("player specialization get failed user_id=%d tree_id=%s err=%v", userID, treeID, err)
		return s.writePlayerSpecializationError(c, err)
	}
	if view.RulesetReset != nil {
		s.syncAfterSpecializationMutation(userID)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, view)
}

func (s *Server) handlePlayerSpecializationAllocate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	treeID := strings.TrimSpace(c.Params("treeId"))
	if treeID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_TREE_ID_REQUIRED", "specialization tree id required", "")
	}
	idempotencyKey, err := specializationIdempotencyKey(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_IDEMPOTENCY_KEY_REQUIRED", "X-Idempotency-Key header required", "")
	}
	var req playerSpecializationAllocateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_INVALID_PAYLOAD", "invalid specialization allocation payload", err.Error())
	}
	if strings.TrimSpace(req.NodeID) == "" || req.ExpectedRevision < 0 || strings.TrimSpace(req.ExpectedRulesetVersion) == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_INVALID_REQUEST", "invalid specialization allocation request", "")
	}
	result, err := s.deps.Store.AllocatePlayerSpecialization(c.Context(), userID, store.SpecializationAllocateCommand{
		TreeID:                 treeID,
		NodeID:                 strings.TrimSpace(req.NodeID),
		ExpectedRevision:       req.ExpectedRevision,
		ExpectedRulesetVersion: strings.TrimSpace(req.ExpectedRulesetVersion),
		ImpactDigest:           strings.TrimSpace(req.ImpactDigest),
		IdempotencyKey:         idempotencyKey,
	})
	if err != nil {
		log.Printf("player specialization allocate failed user_id=%d tree_id=%s node_id=%s err=%v", userID, treeID, strings.TrimSpace(req.NodeID), err)
		return s.writePlayerSpecializationError(c, err)
	}
	if result.RequiresConfirm && !result.Committed {
		return s.writeSpecializationImpactConfirmation(c, result)
	}
	if result.Committed {
		s.syncAfterSpecializationMutation(userID)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handlePlayerSpecializationReset(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	treeID := strings.TrimSpace(c.Params("treeId"))
	if treeID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_TREE_ID_REQUIRED", "specialization tree id required", "")
	}
	idempotencyKey, err := specializationIdempotencyKey(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_IDEMPOTENCY_KEY_REQUIRED", "X-Idempotency-Key header required", "")
	}
	var req playerSpecializationResetRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_INVALID_PAYLOAD", "invalid specialization reset payload", err.Error())
	}
	if req.ExpectedRevision < 0 || strings.TrimSpace(req.ExpectedRulesetVersion) == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_INVALID_REQUEST", "invalid specialization reset request", "")
	}
	result, err := s.deps.Store.ResetPlayerSpecialization(c.Context(), userID, store.SpecializationResetCommand{
		TreeID:                 treeID,
		ExpectedRevision:       req.ExpectedRevision,
		ExpectedRulesetVersion: strings.TrimSpace(req.ExpectedRulesetVersion),
		ImpactDigest:           strings.TrimSpace(req.ImpactDigest),
		IdempotencyKey:         idempotencyKey,
	})
	if err != nil {
		log.Printf("player specialization reset failed user_id=%d tree_id=%s err=%v", userID, treeID, err)
		return s.writePlayerSpecializationError(c, err)
	}
	if result.RequiresConfirm && !result.Committed {
		return s.writeSpecializationImpactConfirmation(c, result)
	}
	if result.Committed {
		s.syncAfterSpecializationMutation(userID)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) syncAfterSpecializationMutation(userID int64) {
	s.invalidatePlayerProfileCaches(userID)
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:      []int64{userID},
		PlayerState:  true,
		Inventory:    true,
		Equipment:    true,
		Attributes:   true,
		Queue:        true,
		Achievements: true,
	})
}

func specializationIdempotencyKey(c *fiber.Ctx) (string, error) {
	key := strings.TrimSpace(c.Get("X-Idempotency-Key"))
	if key == "" || len(key) > 128 {
		return "", errors.New("invalid idempotency key")
	}
	return key, nil
}

func (s *Server) writeSpecializationImpactConfirmation(c *fiber.Ctx, result store.SpecializationMutationResult) error {
	now := time.Now()
	body := apiErrorBody{
		Code:    "SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED",
		Message: "specialization change requires equipment impact confirmation",
	}
	return c.Status(fiber.StatusConflict).JSON(apiErrorResponse{
		OK:      false,
		Error:   body,
		Data:    result,
		TS:      now.UnixMilli(),
		Cost:    requestCostSnapshot(c, now),
		Code:    body.Code,
		Message: body.Message,
	})
}

func (s *Server) writePlayerSpecializationError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, store.ErrSpecializationTreeNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "SPECIALIZATION_TREE_NOT_FOUND", "specialization tree not found", "")
	case errors.Is(err, store.ErrSpecializationTreeLocked):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_TREE_LOCKED", "specialization tree is locked", "")
	case errors.Is(err, store.ErrSpecializationNodeNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "SPECIALIZATION_NODE_NOT_FOUND", "specialization node not found", "")
	case errors.Is(err, store.ErrSpecializationNodeMaxRank):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_NODE_MAX_RANK", "specialization node is already at max rank", "")
	case errors.Is(err, store.ErrSpecializationRequirementsUnmet):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_REQUIREMENTS_UNMET", "specialization requirements not met", "")
	case errors.Is(err, store.ErrSpecializationActiveRootLimit):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_ACTIVE_ROOT_LIMIT", "specialization active root limit reached", "")
	case errors.Is(err, store.ErrSpecializationExclusiveConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_EXCLUSIVE_CONFLICT", "specialization exclusive group conflict", "")
	case errors.Is(err, store.ErrSpecializationInsufficientPoints):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_POINTS_INSUFFICIENT", "specialization points insufficient", "")
	case errors.Is(err, store.ErrSpecializationRevisionConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_REVISION_CONFLICT", "specialization revision conflict", "")
	case errors.Is(err, store.ErrSpecializationRulesetConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_RULESET_CONFLICT", "specialization ruleset conflict", "")
	case errors.Is(err, store.ErrSpecializationImpactConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_IMPACT_CONFLICT", "specialization equipment impact changed", "")
	case errors.Is(err, store.ErrSpecializationIdempotencyKeyRequired):
		return s.writeAPIError(c, fiber.StatusBadRequest, "SPECIALIZATION_IDEMPOTENCY_KEY_REQUIRED", "X-Idempotency-Key header required", "")
	case errors.Is(err, store.ErrSpecializationIdempotencyConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_IDEMPOTENCY_CONFLICT", "specialization idempotency key conflict", "")
	case errors.Is(err, store.ErrSpecializationDefinitionInvalid), errors.Is(err, store.ErrSpecializationEquipmentTopologyInvalid):
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SPECIALIZATION_DEFINITION_INVALID", "specialization definition invalid", err.Error())
	case errors.Is(err, store.ErrInsufficientItem):
		return s.writeAPIError(c, fiber.StatusConflict, "SPECIALIZATION_RESET_COST_INSUFFICIENT", "specialization reset cost insufficient", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SPECIALIZATION_MUTATION_FAILED", "specialization operation failed", "")
	}
}
