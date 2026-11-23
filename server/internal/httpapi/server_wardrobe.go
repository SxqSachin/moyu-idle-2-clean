package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

// --- Request types ---

type wardrobeEquipRequest struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}

type wardrobeUnequipRequest struct {
	SlotID string `json:"slotId"`
}

type adminWardrobeGrantRequest struct {
	UserID   int64  `json:"userId"`
	Category string `json:"category"`
	ItemID   string `json:"itemId"`
	Source   string `json:"source"`
}

// --- Player wardrobe handlers ---

func (s *Server) handleWardrobeGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	state, err := s.deps.Store.GetWardrobe(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "WARDROBE_GET_FAILED", "get wardrobe failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, state)
}

func (s *Server) handleWardrobeEquip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req wardrobeEquipRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_EQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.SlotID = strings.TrimSpace(req.SlotID)
	req.ItemID = strings.TrimSpace(req.ItemID)
	if req.SlotID == "" || req.ItemID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_EQUIP_INVALID_REQUEST", "slotId and itemId are required", "")
	}
	if err := s.deps.Store.EquipCosmetic(c.Context(), userID, req.SlotID, req.ItemID); err != nil {
		return s.writeWardrobeError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:  []int64{userID},
		Wardrobe: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleWardrobeUnequip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req wardrobeUnequipRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_UNEQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.SlotID = strings.TrimSpace(req.SlotID)
	if req.SlotID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_UNEQUIP_INVALID_REQUEST", "slotId is required", "")
	}
	if err := s.deps.Store.UnequipCosmetic(c.Context(), userID, req.SlotID); err != nil {
		return s.writeWardrobeError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:  []int64{userID},
		Wardrobe: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

// --- Kitty wardrobe handlers ---

func (s *Server) handleKittyWardrobeEquip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, parseErr := s.parseWardrobeKittyID(c)
	if parseErr != nil {
		return parseErr
	}
	var req wardrobeEquipRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_KITTY_EQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.SlotID = strings.TrimSpace(req.SlotID)
	req.ItemID = strings.TrimSpace(req.ItemID)
	if req.SlotID == "" || req.ItemID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_KITTY_EQUIP_INVALID_REQUEST", "slotId and itemId are required", "")
	}
	if err := s.deps.Store.EquipKittyCosmetic(c.Context(), userID, kittyID, req.SlotID, req.ItemID); err != nil {
		return s.writeWardrobeError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:  []int64{userID},
		Wardrobe: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "kittyId": kittyID})
}

func (s *Server) handleKittyWardrobeUnequip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, parseErr := s.parseWardrobeKittyID(c)
	if parseErr != nil {
		return parseErr
	}
	var req wardrobeUnequipRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_KITTY_UNEQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.SlotID = strings.TrimSpace(req.SlotID)
	if req.SlotID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_KITTY_UNEQUIP_INVALID_REQUEST", "slotId is required", "")
	}
	if err := s.deps.Store.UnequipKittyCosmetic(c.Context(), userID, kittyID, req.SlotID); err != nil {
		return s.writeWardrobeError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:  []int64{userID},
		Wardrobe: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "kittyId": kittyID})
}

// --- Admin handlers ---

func (s *Server) handleAdminWardrobeGrant(c *fiber.Ctx) error {
	var req adminWardrobeGrantRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_GRANT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.UserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_GRANT_INVALID_USER", "userId is required", "")
	}
	req.Category = strings.TrimSpace(req.Category)
	req.ItemID = strings.TrimSpace(req.ItemID)
	if req.Category == "" || req.ItemID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_GRANT_INVALID_REQUEST", "category and itemId are required", "")
	}
	alreadyOwned, err := s.deps.Store.GrantCosmetic(c.Context(), req.UserID, store.CosmeticGrant{
		Category: req.Category,
		ItemID:   req.ItemID,
		Source:   req.Source,
	})
	if err != nil {
		return s.writeWardrobeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":           true,
		"alreadyOwned": alreadyOwned,
	})
}

type adminWardrobeBatchGrantRequest struct {
	UserID int64             `json:"userId"`
	Items  []adminGrantItem  `json:"items"`
	Source string            `json:"source"`
}

type adminGrantItem struct {
	Category string `json:"category"`
	ItemID   string `json:"itemId"`
}

type adminWardrobeRevokeRequest struct {
	UserID   int64  `json:"userId"`
	Category string `json:"category"`
	ItemID   string `json:"itemId"`
}

func (s *Server) handleAdminWardrobeList(c *fiber.Ctx) error {
	userIDStr := strings.TrimSpace(c.Query("userId"))
	if userIDStr == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_LIST_INVALID_USER", "userId query param is required", "")
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_LIST_INVALID_USER", "invalid userId", "")
	}
	state, err := s.deps.Store.GetWardrobe(c.Context(), userID)
	if err != nil {
		return s.writeWardrobeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"userId": userID,
		"owned":  state.Owned,
		"equipped": state.Equipped,
	})
}

func (s *Server) handleAdminWardrobeBatchGrant(c *fiber.Ctx) error {
	var req adminWardrobeBatchGrantRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_BATCH_GRANT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.UserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_BATCH_GRANT_INVALID_USER", "userId is required", "")
	}
	if len(req.Items) == 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_BATCH_GRANT_EMPTY", "items array is empty", "")
	}
	if len(req.Items) > 100 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_BATCH_GRANT_TOO_LARGE", "items array exceeds limit of 100", "")
	}
	grants := make([]store.CosmeticGrant, 0, len(req.Items))
	for i, item := range req.Items {
		cat := strings.TrimSpace(item.Category)
		id := strings.TrimSpace(item.ItemID)
		if cat == "" || id == "" {
			return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_BATCH_GRANT_INVALID_ITEM",
				fmt.Sprintf("item at index %d: category and itemId are required", i), "")
		}
		grants = append(grants, store.CosmeticGrant{
			Category: cat,
			ItemID:   id,
			Source:   req.Source,
		})
	}
	newlyGranted, err := s.deps.Store.GrantCosmetics(c.Context(), req.UserID, grants)
	if err != nil {
		return s.writeWardrobeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":           true,
		"grantedCount": len(newlyGranted),
		"skippedCount": len(grants) - len(newlyGranted),
		"granted":      newlyGranted,
	})
}

func (s *Server) handleAdminWardrobeRevoke(c *fiber.Ctx) error {
	var req adminWardrobeRevokeRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_REVOKE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.UserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_REVOKE_INVALID_USER", "userId is required", "")
	}
	req.Category = strings.TrimSpace(req.Category)
	req.ItemID = strings.TrimSpace(req.ItemID)
	if req.Category == "" || req.ItemID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_ADMIN_REVOKE_INVALID_REQUEST", "category and itemId are required", "")
	}
	if err := s.deps.Store.RevokeCosmetic(c.Context(), req.UserID, req.Category, req.ItemID); err != nil {
		return s.writeWardrobeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

// --- Helpers ---

func (s *Server) parseWardrobeKittyID(c *fiber.Ctx) (int64, error) {
	raw := strings.TrimSpace(c.Params("kittyID"))
	id, err := strconv.ParseInt(raw, 10, 64)
	if raw == "" || err != nil || id <= 0 {
		return 0, s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_INVALID_KITTY_ID", "invalid kitty id", "")
	}
	return id, nil
}

func (s *Server) writeWardrobeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, store.ErrCosmeticNotOwned):
		return s.writeAPIError(c, fiber.StatusConflict, "WARDROBE_COSMETIC_NOT_OWNED", "cosmetic not owned", "")
	case errors.Is(err, store.ErrWardrobeSlotInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_SLOT_INVALID", "invalid wardrobe slot", "")
	case errors.Is(err, store.ErrCosmeticItemNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "WARDROBE_COSMETIC_NOT_FOUND", "cosmetic item not found", "")
	case errors.Is(err, store.ErrCosmeticCategoryInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "WARDROBE_CATEGORY_INVALID", "invalid cosmetic category", "")
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "WARDROBE_KITTY_NOT_FOUND", "kitty not found", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, "WARDROBE_OPERATION_FAILED", "wardrobe operation failed", err.Error())
	}
}

// pushWardrobe pushes the current wardrobe state to the user via WebSocket.
func (s *Server) pushWardrobe(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state, err := s.deps.Store.GetWardrobe(ctx, userID)
	if err != nil {
		return
	}
	s.pushUserEventClusterOnce(userID, "game.wardrobe.patch", state)
}

// resolveChatCosmeticMeta resolves a player's equipped cosmetics for chat display.
// Returns the raw item IDs so the frontend can translate via i18n.
// Returns empty meta on any error (graceful degradation — never blocks chat).
func (s *Server) resolveChatCosmeticMeta(ctx context.Context, userID int64) store.ChatCosmeticMeta {
	prefixID, suffixID, err := s.deps.Store.GetPlayerTitle(ctx, userID)
	if err != nil {
		return store.ChatCosmeticMeta{}
	}
	// Validate against catalog — skip disabled/missing entries.
	if prefixID != "" {
		if entry := s.deps.GameData.TitlePrefixByID(prefixID); entry == nil || entry.Disabled {
			prefixID = ""
		}
	}
	if suffixID != "" {
		if entry := s.deps.GameData.TitleSuffixByID(suffixID); entry == nil || entry.Disabled {
			suffixID = ""
		}
	}
	return store.ChatCosmeticMeta{
		TitlePrefixID: prefixID,
		TitleSuffixID: suffixID,
		DisplayMeta:   map[string]any{},
	}
}
