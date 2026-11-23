package httpapi

import (
	"errors"
	"strings"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type useOperationEffectRequest struct {
	InstanceID    int64  `json:"instanceId"`
	TargetKind    string `json:"targetKind"`
	TargetActorID int64  `json:"targetActorId"`
	RequestID     string `json:"requestId"`
	Quantity      *int64 `json:"quantity"`
}

func (s *Server) handleTransmuteFoodEligibility(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	eligible, err := s.deps.Store.TransmuteFoodEligible(c.Context(), strings.TrimSpace(c.Params("itemId")))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "TRANSMUTE_FOOD_PREVIEW_FAILED", "eligibility unavailable", "")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"eligible": eligible})
}

func (s *Server) handleOperationEffectsGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	projection, err := s.deps.Store.ListOperationEffects(c.Context(), userID, time.Now())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "OPERATION_EFFECTS_LIST_FAILED", "load operation effects failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, projection)
}

func (s *Server) handleUseOperationEffect(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req useOperationEffectRequest
	if err := c.BodyParser(&req); err != nil || req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OPERATION_EFFECT_USE_INVALID_REQUEST", "invalid request", "")
	}
	quantity := int64(1)
	if req.Quantity != nil {
		quantity = *req.Quantity
	}
	targetKind := operationeffect.TargetKind(strings.TrimSpace(req.TargetKind))
	target := operationeffect.Target{Kind: targetKind, ActorID: req.TargetActorID}
	requestID := strings.TrimSpace(req.RequestID)
	headerRequestID := strings.TrimSpace(c.Get("X-Idempotency-Key"))
	if requestID != "" && headerRequestID != "" && requestID != headerRequestID {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OPERATION_EFFECT_USE_INVALID_REQUEST", "idempotency keys do not match", "")
	}
	if requestID == "" {
		requestID = headerRequestID
	}
	projection, err := s.deps.Store.ActivateOperationEffectQuantity(
		c.Context(), userID, req.InstanceID, target, requestID, time.Now(), quantity,
	)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrOperationEffectRequestInvalid), errors.Is(err, store.ErrOperationEffectTargetInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "OPERATION_EFFECT_USE_INVALID_REQUEST", "invalid operation effect request", err.Error())
		case errors.Is(err, store.ErrOperationEffectTargetNotOwned):
			return s.writeAPIError(c, fiber.StatusForbidden, "OPERATION_EFFECT_TARGET_NOT_OWNED", "target kitty is not owned", "")
		case errors.Is(err, store.ErrOperationEffectNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "OPERATION_EFFECT_DEFINITION_NOT_FOUND", "operation effect definition not found", "")
		case errors.Is(err, store.ErrOperationEffectAtCap):
			return s.writeAPIError(c, fiber.StatusConflict, "OPERATION_EFFECT_AT_CAP", "operation effect is already at its cap", "")
		case errors.Is(err, store.ErrOperationEffectRecoveryPending):
			return s.writeAPIError(c, fiber.StatusConflict, "OPERATION_EFFECT_RECOVERY_PENDING", "finish queue recovery before adding charges", "")
		case errors.Is(err, store.ErrOperationEffectStrengthConflict):
			return s.writeAPIError(c, fiber.StatusConflict, "OPERATION_EFFECT_STRENGTH_CONFLICT", "a different strength of this effect is already active", "")
		case errors.Is(err, store.ErrOperationEffectIdempotencyConflict):
			return s.writeAPIError(c, fiber.StatusConflict, "OPERATION_EFFECT_REQUEST_CONFLICT", "request id was already used", "")
		case errors.Is(err, store.ErrOperationEffectItemInvalid), errors.Is(err, store.ErrInvalidInstanceState), errors.Is(err, store.ErrInsufficientItem), errors.Is(err, pgx.ErrNoRows):
			return s.writeAPIError(c, fiber.StatusConflict, "OPERATION_EFFECT_ITEM_UNAVAILABLE", "item is no longer available", "")
		case errors.Is(err, store.ErrInstanceNotOwned):
			return s.writeAPIError(c, fiber.StatusForbidden, "OPERATION_EFFECT_ITEM_NOT_OWNED", "item is not owned", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "OPERATION_EFFECT_USE_FAILED", "use operation effect item failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:          []int64{userID},
		Inventory:        true,
		OperationEffects: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, projection)
}
