package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/market"
	"mysecondwebgame/server/internal/rules"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type playerBattleLoadoutUpsertRequest struct {
	Loadout store.BattleLoadout `json:"loadout"`
}

type playerWorkLoadoutUpsertRequest struct {
	Loadout store.WorkLoadout `json:"loadout"`
}

type playerLoadoutApplyRequest struct {
	LoadoutID string `json:"loadoutId"`
}

type playerWorkBindingsRequest struct {
	Bindings map[string]string `json:"bindings"`
}

type playerLoadoutDeleteRequest struct {
	LoadoutID string `json:"loadoutId"`
}

func (s *Server) handleMarketBuy(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketBuyRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.Quantity <= 0 || req.Quantity > 1_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_QUANTITY", "invalid quantity", "")
	}
	idempotencyKey := c.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_MISSING_IDEMPOTENCY_KEY", "missing X-Idempotency-Key", "")
	}
	result, err := s.deps.Market.BuyWithIdempotency(c.Context(), userID, req.ListingID, req.Quantity, idempotencyKey)
	if err != nil {
		if errors.Is(err, store.ErrInsufficientGold) || errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSUFFICIENT_GOLD", "insufficient gold", "")
		}
		if errors.Is(err, store.ErrListingNotEnough) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSUFFICIENT_LISTING_QUANTITY", "insufficient listing quantity", "")
		}
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_QUANTITY", "invalid quantity", "")
		}
		if errors.Is(err, store.ErrDuplicateIdempotency) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_DUPLICATE_IDEMPOTENCY_KEY", "duplicate idempotency key", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSTANCE_STATE_INVALID", "instance state invalid for listing", "")
		}
		if errors.Is(err, store.ErrMarketItemMustAuction) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSTANCE_REQUIRE_AUCTION", "instance listings are disabled in market", "")
		}
		if errors.Is(err, store.ErrMarketItemNotListable) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_NOT_LISTABLE", "item cannot be traded on market", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_BUY_FAILED", "buy failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID, result.SellerID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":           true,
		"orderId":      result.OrderID,
		"created":      result.Created,
		"feeRateBps":   result.FeeRateBps,
		"feeAmount":    result.FeeAmount,
		"sellerAmount": result.SellerAmount,
	})
}

func (s *Server) handleShopOffers(c *fiber.Ctx) error {
	offers, err := s.deps.Store.ListShopOffers(c.Context())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SHOP_OFFER_LIST_FAILED", "list shop offers failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"offers": offers})
}

func (s *Server) handleShopBuy(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req shopTradeRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SHOP_BUY_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.OfferID <= 0 || req.Quantity <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SHOP_BUY_INVALID_REQUEST", "invalid shop buy request", "")
	}
	if err = s.deps.Store.BuyFromShop(c.Context(), userID, req.OfferID, req.Quantity); err != nil {
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "SHOP_BUY_INVALID_QUANTITY", "invalid quantity", "")
		}
		if errors.Is(err, store.ErrShopOfferNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "SHOP_BUY_OFFER_NOT_FOUND", "shop offer not found", "")
		}
		if errors.Is(err, store.ErrInsufficientGold) || errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "SHOP_BUY_INSUFFICIENT_GOLD", "insufficient gold", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SHOP_BUY_FAILED", "shop buy failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleShopSell(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req shopTradeRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SHOP_SELL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.ItemID == "" || req.Quantity <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SHOP_SELL_INVALID_REQUEST", "invalid shop sell request", "")
	}
	if err = s.deps.Store.SellToShop(c.Context(), userID, req.ItemID, req.Quantity); err != nil {
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "SHOP_SELL_INVALID_QUANTITY", "invalid quantity", "")
		}
		if errors.Is(err, store.ErrShopOfferNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "SHOP_SELL_OFFER_NOT_FOUND", "shop offer not found", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "SHOP_SELL_INSUFFICIENT_ITEMS", "insufficient item quantity", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SHOP_SELL_FAILED", "shop sell failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleMarketFeeSummary(c *fiber.Ctx) error {
	summary, err := s.deps.Store.MarketFeeSummary(c.Context())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_FEE_SUMMARY_FAILED", "query market fee summary failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, summary)
}

func (s *Server) handleRuleEvaluate(c *fiber.Ctx) error {
	var req ruleEvaluateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_EVALUATE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	result := rules.Evaluate(req.Groups, req.Context)
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"passed": result})
}

func (s *Server) handleRuleConfigList(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	ruleKind := strings.TrimSpace(c.Query("kind"))
	if ruleKind != "" && ruleKind != "skill" && ruleKind != "item" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_CONFIG_INVALID_KIND", "invalid rule kind", "")
	}
	rows, err := s.deps.Store.ListRuleSets(c.Context(), userID, ruleKind)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "RULE_CONFIG_LIST_FAILED", "list rule config failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows})
}

func (s *Server) handleRuleConfigUpsert(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req ruleConfigUpsertRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_CONFIG_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.RuleKind != "skill" && req.RuleKind != "item" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_CONFIG_INVALID_KIND", "invalid rule kind", "")
	}
	req.SlotKey = strings.TrimSpace(req.SlotKey)
	if req.SlotKey == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_CONFIG_INVALID_SLOT", "invalid slot key", "")
	}
	maxGroups := s.deps.Config.GlobalSettings.Rule.MaxGroups
	maxConditions := s.deps.Config.GlobalSettings.Rule.MaxConditionsPerGroup
	if len(req.Groups) > maxGroups {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_CONFIG_TOO_MANY_GROUPS", "too many groups", "")
	}
	for _, g := range req.Groups {
		if len(g.Conditions) > maxConditions {
			return s.writeAPIError(c, fiber.StatusBadRequest, "RULE_CONFIG_TOO_MANY_CONDITIONS", "too many conditions in group", "")
		}
	}
	if err = s.deps.Store.UpsertRuleSet(c.Context(), userID, req.RuleKind, req.SlotKey, req.Enabled, req.Groups); err != nil {
		if errors.Is(err, store.ErrSkillForgeSameSource) {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_LINEAGE_CONFLICT", "source skill and derived skill are mutually exclusive", err.Error())
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "RULE_CONFIG_UPSERT_FAILED", "save rule config failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handlePlayerLoadoutsGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	loadouts, err := s.deps.Store.GetPlayerLoadouts(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_LOADOUTS_LOAD_FAILED", "load player loadouts failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, loadouts)
}

func (s *Server) handlePlayerBattleLoadoutUpsert(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerBattleLoadoutUpsertRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_LOADOUT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if err := s.rejectModeratedAPI(c, moderationRuleLoadoutName, req.Loadout.Name, userID); err != nil {
		return err
	}
	loadouts, err := s.deps.Store.UpsertBattleLoadout(c.Context(), userID, req.Loadout)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_BATTLE_LOADOUT_UPSERT_FAILED", "save battle loadout failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, loadouts)
}

func (s *Server) handlePlayerWorkLoadoutUpsert(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerWorkLoadoutUpsertRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_LOADOUT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if err := s.rejectModeratedAPI(c, moderationRuleLoadoutName, req.Loadout.Name, userID); err != nil {
		return err
	}
	loadouts, err := s.deps.Store.UpsertWorkLoadout(c.Context(), userID, req.Loadout)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_WORK_LOADOUT_UPSERT_FAILED", "save work loadout failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, loadouts)
}

func (s *Server) handlePlayerLoadoutDelete(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerLoadoutDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_LOADOUT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	loadouts, err := s.deps.Store.DeletePlayerLoadout(c.Context(), userID, c.Params("type"), req.LoadoutID)
	if err != nil {
		if errors.Is(err, store.ErrLoadoutNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "PLAYER_LOADOUT_NOT_FOUND", "loadout not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_LOADOUT_DELETE_FAILED", "delete loadout failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, loadouts)
}

func (s *Server) handlePlayerWorkLoadoutBindingsUpdate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerWorkBindingsRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_LOADOUT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	loadouts, err := s.deps.Store.UpdateWorkActionBindings(c.Context(), userID, req.Bindings)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_WORK_LOADOUT_BINDINGS_FAILED", "update work loadout bindings failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, loadouts)
}

func (s *Server) handlePlayerBattleLoadoutApply(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerLoadoutApplyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_LOADOUT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	result, err := s.deps.Store.ApplyBattleLoadout(
		c.Context(),
		userID,
		req.LoadoutID,
	)
	if err != nil {
		if errors.Is(err, store.ErrLoadoutNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "PLAYER_LOADOUT_NOT_FOUND", "loadout not found", "")
		}
		var valErr *store.LoadoutValidationError
		if errors.As(err, &valErr) {
			return s.writeAPIError(c, fiber.StatusConflict, "LOADOUT_EQUIP_"+valErr.Reason, "loadout equipment validation failed", valErr.Error(), valErr.ItemID)
		}
		return s.writeAPIError(c, fiber.StatusConflict, "PLAYER_BATTLE_LOADOUT_APPLY_FAILED", "apply battle loadout failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: true, Equipment: true, Attributes: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":            true,
		"skippedSkills": result.SkippedSkills,
	})
}

func (s *Server) handlePlayerWorkLoadoutApply(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerLoadoutApplyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_LOADOUT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if err := s.deps.Store.ApplyWorkLoadout(c.Context(), userID, req.LoadoutID); err != nil {
		if errors.Is(err, store.ErrLoadoutNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "PLAYER_LOADOUT_NOT_FOUND", "loadout not found", "")
		}
		var valErr *store.LoadoutValidationError
		if errors.As(err, &valErr) {
			return s.writeAPIError(c, fiber.StatusConflict, "LOADOUT_EQUIP_"+valErr.Reason, "loadout equipment validation failed", valErr.Error(), valErr.ItemID)
		}
		return s.writeAPIError(c, fiber.StatusConflict, "PLAYER_WORK_LOADOUT_APPLY_FAILED", "apply work loadout failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: true, Equipment: true, Attributes: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleBattleItemBagGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	items, err := s.deps.Store.ListBattleItemBag(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "WS_BATTLE_ITEM_BAG_LIST_FAILED", "load battle item bag failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"limits": fiber.Map{
			"maxKinds":         s.deps.Config.SharedGlobalSettings.BattleItemBagMaxKinds,
			"maxTotalQuantity": s.deps.Config.SharedGlobalSettings.BattleItemBagMaxTotalQuantity,
		},
	})
}

func (s *Server) handleBattleItemBagAdd(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req battleItemBagMutationRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	slotNo := req.SlotNo
	itemID := strings.TrimSpace(req.ItemID)
	if itemID == "" || req.Quantity <= 0 || slotNo <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_INVALID_REQUEST", "invalid slot no, item id or quantity", "")
	}
	item := s.findCatalogItem(itemID)
	if item == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "WS_BATTLE_ITEM_BAG_ITEM_NOT_FOUND", "item not found", "")
	}
	if !item.CarryableInCombat {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_ITEM_NOT_CARRYABLE", "item cannot be carried in combat", "")
	}
	if strings.TrimSpace(item.BattleUsableItemID) == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_ITEM_NOT_USABLE", "item has no battle usable mapping", "")
	}
	err = s.deps.Store.AddBattleItemBagQuantity(
		c.Context(),
		userID,
		slotNo,
		itemID,
		req.Quantity,
		s.deps.Config.SharedGlobalSettings.BattleItemBagMaxKinds,
		s.deps.Config.SharedGlobalSettings.BattleItemBagMaxTotalQuantity,
	)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidItemID), errors.Is(err, store.ErrBadQuantity):
			return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_INVALID_REQUEST", "invalid slot no, item id or quantity", "")
		case errors.Is(err, store.ErrBattleItemBagSlotInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_SLOT_INVALID", "invalid battle item bag slot", "")
		case errors.Is(err, store.ErrBattleItemBagSlotOccupied):
			return s.writeAPIError(c, fiber.StatusConflict, "WS_BATTLE_ITEM_BAG_SLOT_OCCUPIED", "battle item bag slot occupied", "")
		case errors.Is(err, store.ErrBattleItemBagKindsLimitExceeded):
			return s.writeAPIError(c, fiber.StatusConflict, "WS_BATTLE_ITEM_BAG_KINDS_LIMIT_EXCEEDED", "battle item bag kinds limit exceeded", "")
		case errors.Is(err, store.ErrBattleItemBagTotalLimitExceeded):
			return s.writeAPIError(c, fiber.StatusConflict, "WS_BATTLE_ITEM_BAG_TOTAL_LIMIT_EXCEEDED", "battle item bag total quantity limit exceeded", "")
		case errors.Is(err, store.ErrInsufficientItem):
			return s.writeAPIError(c, fiber.StatusConflict, "WS_BATTLE_ITEM_BAG_INSUFFICIENT_INVENTORY", "insufficient inventory quantity", "")
		case errors.Is(err, store.ErrBattleItemBagLocked):
			return s.writeAPIError(c, fiber.StatusConflict, "WS_BATTLE_ITEM_BAG_LOCKED_IN_BATTLE", "battle item bag cannot be modified during battle", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "WS_BATTLE_ITEM_BAG_ADD_FAILED", "add battle item bag quantity failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleBattleItemBagTakeBack(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req battleItemBagTakeBackRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.SlotNo <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_INVALID_REQUEST", "invalid slot no", "")
	}
	err = s.deps.Store.TakeBackBattleItemBagSlot(c.Context(), userID, req.SlotNo, s.deps.Config.SharedGlobalSettings.BattleItemBagMaxKinds)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBattleItemBagSlotInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "WS_BATTLE_ITEM_BAG_SLOT_INVALID", "invalid battle item bag slot", "")
		case errors.Is(err, store.ErrBattleItemBagSlotNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "WS_BATTLE_ITEM_BAG_SLOT_NOT_FOUND", "battle item bag slot not found", "")
		case errors.Is(err, store.ErrBattleItemBagLocked):
			return s.writeAPIError(c, fiber.StatusConflict, "WS_BATTLE_ITEM_BAG_LOCKED_IN_BATTLE", "battle item bag cannot be modified during battle", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "WS_BATTLE_ITEM_BAG_TAKEBACK_FAILED", "take back battle item bag slot failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleBattleSkillConfigGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	ownedSkills, err := s.deps.Store.ListOwnedSkills(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_OWNED_SKILLS_LOAD_FAILED", "load owned skills failed", err.Error())
	}
	customDefinitions, err := s.deps.Store.ListAcceptedCustomSkillDefinitions(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_CUSTOM_SKILLS_LOAD_FAILED", "load custom skills failed", err.Error())
	}
	filterUsableOwnedSkills := func(items []store.OwnedSkill) []store.OwnedSkill {
		filtered := make([]store.OwnedSkill, 0, len(items))
		for _, item := range items {
			key := strings.ToLower(strings.TrimSpace(item.SkillID))
			if strings.HasPrefix(key, "custom:") {
				if _, usable := customDefinitions[key]; !usable {
					continue
				}
			}
			filtered = append(filtered, item)
		}
		return filtered
	}
	ownedSkills = filterUsableOwnedSkills(ownedSkills)
	ownedSkillMap := make(map[string]string, len(ownedSkills)+1)
	for _, item := range ownedSkills {
		id := strings.TrimSpace(item.SkillID)
		if id == "" {
			continue
		}
		ownedSkillMap[strings.ToLower(id)] = id
	}
	if defaultBasicSkillID != "" {
		if _, exists := ownedSkillMap[strings.ToLower(defaultBasicSkillID)]; !exists {
			if err := s.deps.Store.GrantOwnedSkill(c.Context(), userID, defaultBasicSkillID); err != nil {
				return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_OWNED_SKILLS_INIT_FAILED", "init basic skill ownership failed", err.Error())
			}
			ownedSkills, err = s.deps.Store.ListOwnedSkills(c.Context(), userID)
			if err != nil {
				return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_OWNED_SKILLS_LOAD_FAILED", "load owned skills failed", err.Error())
			}
			ownedSkills = filterUsableOwnedSkills(ownedSkills)
			ownedSkillMap = make(map[string]string, len(ownedSkills)+1)
			for _, item := range ownedSkills {
				id := strings.TrimSpace(item.SkillID)
				if id == "" {
					continue
				}
				ownedSkillMap[strings.ToLower(id)] = id
			}
		}
	}
	if defaultBasicSkillID == "" && len(ownedSkills) > 0 {
		for _, item := range ownedSkills {
			id := strings.TrimSpace(item.SkillID)
			if _, usable := ownedSkillMap[strings.ToLower(id)]; usable {
				defaultBasicSkillID = id
				break
			}
		}
	}
	config, cfgErr := s.loadCombatConfigOrDefault(c.Context(), userID)
	if cfgErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_GET_FAILED", "load combat config failed", cfgErr.Error())
	}
	enableCtx, evalErr := s.loadBattleSkillEnableContext(c.Context(), userID)
	if evalErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_ELIGIBILITY_FAILED", "load skill eligibility failed", evalErr.Error())
	}
	currentSlots := battleAvailableSkillSlots(enableCtx.Attributes["intelligence"], s.battleSkillSlotRule())
	config.SkillSlots = normalizeCombatSkillSlots(config.SkillSlots, ownedSkillMap, currentSlots, defaultBasicSkillID)
	reconciledSlots, _, reconcileErr := s.reconcileCombatSkillConfigForRequirementsWithContext(
		c.Context(),
		userID,
		defaultBasicSkillID,
		ownedSkillMap,
		currentSlots,
		config,
		enableCtx,
	)
	if reconcileErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_RECONCILE_FAILED", "reconcile combat config failed", reconcileErr.Error())
	}
	config.SkillSlots = reconciledSlots
	rulesList, err := s.deps.Store.ListRuleSets(c.Context(), userID, "skill")
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "RULE_CONFIG_LIST_FAILED", "list rule config failed", err.Error())
	}
	skillEligibility := s.battleSkillEligibilityMapFromContext(enableCtx)
	for id, raw := range customDefinitions {
		skillEligibility[strings.ToLower(id)] = evaluateBattleSkillRequirement(raw, enableCtx)
	}
	catalog := s.battleSkillCatalogRowsWithCustom(skillEligibility, customDefinitions)
	specializationContribution, err := s.playerSpecializationBattleContribution(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_SPECIALIZATION_PREVIEW_FAILED", "load specialization skill preview failed", err.Error())
	}
	previewStats := cloneFloatMap(enableCtx.Attributes)
	if err := applyBattleSpecializationStatModifiers(previewStats, specializationContribution.StatModifiers); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_SPECIALIZATION_PREVIEW_FAILED", "apply specialization skill preview stats failed", err.Error())
	}
	specializationMutations, err := s.resolveBattleSpecializationEffectMutations(specializationContribution)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_SPECIALIZATION_PREVIEW_FAILED", "resolve specialization skill preview failed", err.Error())
	}
	s.applyBattleSkillCatalogAttackStatPreview(catalog, battleUnitState{
		Stats:                         previewStats,
		WeaponClasses:                 battleWeaponClassListFromSet(enableCtx.WeaponClasses),
		SpecializationEffectMutations: specializationMutations,
	}, ownedSkills, customDefinitions, defaultBasicSkillID)
	sort.SliceStable(catalog, func(i, j int) bool {
		left, _ := catalog[i]["id"].(string)
		right, _ := catalog[j]["id"].(string)
		return left < right
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"config":              config,
		"equipmentSkills":     s.equipmentSkillPreview(enableCtx, config, ownedSkills, customDefinitions),
		"defaultBasicSkillId": defaultBasicSkillID,
		"ownedSkills":         ownedSkills,
		"rules":               rulesList,
		"skillCatalog":        catalog,
		"slotRule": fiber.Map{
			"baseSkillSlots": s.battleSkillSlotRule().BaseSkillSlots,
			"intPerExtra":    s.battleSkillSlotRule().IntPerExtra,
			"minSkillSlots":  s.battleSkillSlotRule().MinSkillSlots,
			"maxSkillSlots":  s.battleSkillSlotRule().MaxSkillSlots,
			"current":        currentSlots,
		},
		"ruleLimits": fiber.Map{
			"maxGroups":             s.deps.Config.GlobalSettings.Rule.MaxGroups,
			"maxConditionsPerGroup": s.deps.Config.GlobalSettings.Rule.MaxConditionsPerGroup,
		},
	})
}

func (s *Server) handleBattleSkillConfigUpdate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	var req combatConfigUpdateRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_CONFIG_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	ownedSkills, err := s.deps.Store.ListOwnedSkills(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_OWNED_SKILLS_LOAD_FAILED", "load owned skills failed", err.Error())
	}
	customDefinitions, err := s.deps.Store.ListAcceptedCustomSkillDefinitions(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_CUSTOM_SKILLS_LOAD_FAILED", "load custom skills failed", err.Error())
	}
	ownedSkillMap := make(map[string]string, len(ownedSkills)+1)
	for _, item := range ownedSkills {
		id := strings.TrimSpace(item.SkillID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if strings.HasPrefix(key, "custom:") {
			if _, usable := customDefinitions[key]; !usable {
				continue
			}
		}
		ownedSkillMap[key] = id
	}
	if defaultBasicSkillID == "" && len(ownedSkills) > 0 {
		for _, item := range ownedSkills {
			id := strings.TrimSpace(item.SkillID)
			if _, usable := ownedSkillMap[strings.ToLower(id)]; usable {
				defaultBasicSkillID = id
				break
			}
		}
	}
	enableCtx, evalErr := s.loadBattleSkillEnableContext(c.Context(), userID)
	if evalErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_ELIGIBILITY_FAILED", "load skill eligibility failed", evalErr.Error())
	}
	currentSlots := battleAvailableSkillSlots(enableCtx.Attributes["intelligence"], s.battleSkillSlotRule())
	seen := make(map[string]struct{}, len(req.SkillSlots))
	for _, skillID := range req.SkillSlots {
		key := strings.ToLower(strings.TrimSpace(skillID))
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_CONFIG_DUPLICATE_SKILL", "duplicate skill in slots", "")
		}
		seen[key] = struct{}{}
		if _, ok := ownedSkillMap[key]; !ok {
			return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_CONFIG_INVALID_SKILL", "skill is not owned", "")
		}
	}
	normalizedSlots := normalizeCombatSkillSlots(req.SkillSlots, ownedSkillMap, 0, defaultBasicSkillID)
	if len(normalizedSlots) > currentSlots {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_CONFIG_SLOT_LIMIT", "skill slot exceeds current capacity", "")
	}
	for _, skillID := range normalizedSlots {
		if strings.EqualFold(skillID, defaultBasicSkillID) {
			continue
		}
		raw, ok := s.battleSkillRawByIDWithCustom(skillID, customDefinitions)
		if !ok {
			return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_CONFIG_INVALID_SKILL", "skill is not owned", "")
		}
		eligible := evaluateBattleSkillRequirement(raw, enableCtx)
		if len(eligible.UnmetStatus) > 0 {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_STATUS_REQUIREMENT_NOT_MET", "skill status requirements not met", "")
		}
		if len(eligible.MissingWeaponClasses) > 0 {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_WEAPON_REQUIREMENT_NOT_MET", "skill weapon requirements not met", "")
		}
	}
	lineages := make(map[string]string, len(normalizedSlots))
	customSkills, err := s.deps.Store.ListPlayerCustomSkills(c.Context(), userID, false)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_CUSTOM_SKILLS_LOAD_FAILED", "load custom skill lineage failed", err.Error())
	}
	lineageByID := make(map[string]string, len(customSkills))
	for _, custom := range customSkills {
		lineage := strings.ToLower(strings.TrimSpace(custom.SourceSkillID))
		if lineage == "" {
			lineage = strings.ToLower(custom.ID)
		}
		lineageByID[strings.ToLower(custom.ID)] = lineage
	}
	for _, skillID := range normalizedSlots {
		key := strings.ToLower(strings.TrimSpace(skillID))
		lineage := key
		if customLineage, exists := lineageByID[key]; exists {
			lineage = customLineage
		}
		if prior, exists := lineages[lineage]; exists && !strings.EqualFold(prior, skillID) {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_LINEAGE_CONFLICT", "source skill and derived skill are mutually exclusive", "")
		}
		lineages[lineage] = skillID
	}
	if err := s.deps.Store.UpsertCombatConfig(c.Context(), userID, store.CombatConfig{
		SkillSlots: normalizedSlots,
	}); err != nil {
		if errors.Is(err, store.ErrSkillForgeSameSource) {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_LINEAGE_CONFLICT", "source skill and derived skill are mutually exclusive", err.Error())
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_CONFIG_SAVE_FAILED", "save combat config failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok": true,
		"config": fiber.Map{
			"skillSlots": normalizedSlots,
		},
	})
}

func (s *Server) handleBattleOwnedSkillGrant(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req ownedSkillMutationRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	skillID := strings.TrimSpace(req.SkillID)
	if skillID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_INVALID_SKILL", "invalid skill id", "")
	}
	if !s.battleSkillExists(skillID) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_UNKNOWN_SKILL", "unknown skill id", "")
	}
	if err := s.deps.Store.GrantOwnedSkill(c.Context(), userID, skillID); err != nil {
		if errors.Is(err, store.ErrInvalidSkillID) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_INVALID_SKILL", "invalid skill id", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "OWNED_SKILL_GRANT_FAILED", "grant skill failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleBattleOwnedSkillRevoke(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req ownedSkillMutationRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	skillID := strings.TrimSpace(req.SkillID)
	if skillID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_INVALID_SKILL", "invalid skill id", "")
	}
	if err := s.deps.Store.RevokeOwnedSkill(c.Context(), userID, skillID); err != nil {
		if errors.Is(err, store.ErrInvalidSkillID) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_INVALID_SKILL", "invalid skill id", "")
		}
		if errors.Is(err, store.ErrBasicSkillLocked) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "OWNED_SKILL_REVOKE_FORBIDDEN", "basic skill cannot be revoked", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "OWNED_SKILL_REVOKE_FAILED", "revoke skill failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleRepairEquip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req repairRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "REPAIR_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "REPAIR_INVALID_INSTANCE", "invalid instance id", "")
	}
	instance, _, getErr := s.deps.Store.GetItemInstance(c.Context(), userID, req.InstanceID)
	if getErr != nil {
		if errors.Is(getErr, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "REPAIR_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "REPAIR_INSTANCE_LOAD_FAILED", "load item instance failed", getErr.Error())
	}
	itemDef := s.findCatalogItem(instance.TemplateItemID)
	if itemDef == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "REPAIR_ITEM_NOT_FOUND", "item not found", "")
	}
	template := s.findEquipmentTemplate(itemDef.ID)
	if !isEquipmentItem(itemDef, template) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIPMENT_ITEM_NOT_EQUIPMENT", "item is not equipment", "")
	}
	if err = s.deps.Store.RepairItem(c.Context(), userID, req.InstanceID); err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "REPAIR_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "REPAIR_FAILED", "repair failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleDetachEquipmentAffix(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req detachAffixRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_DETACH_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 || req.SlotIndex < 0 || strings.TrimSpace(req.ToolItemID) == "" || req.CarrierInstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_DETACH_INVALID_REQUEST", "invalid detach request", "")
	}
	result, err := s.deps.Store.DetachEquipmentAffix(c.Context(), userID, req.InstanceID, req.SlotIndex, strings.TrimSpace(req.ToolItemID), req.CarrierInstanceID)
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AFFIX_DETACH_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_DETACH_INSTANCE_INVALID_STATE", "instance state must be inventory", "")
		}
		if errors.Is(err, store.ErrAffixSlotNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "AFFIX_DETACH_SLOT_NOT_FOUND", "affix slot not found", "")
		}
		if errors.Is(err, store.ErrAffixSlotOccupied) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_DETACH_CARRIER_OCCUPIED", "affix carrier already occupied", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_DETACH_INSUFFICIENT_ITEM", "insufficient detach tool", "")
		}
		if errors.Is(err, store.ErrAffixRarityNotAllowed) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_DETACH_CARRIER_RARITY_NOT_ALLOWED", "carrier rarity rules mismatch", "")
		}
		if errors.Is(err, store.ErrAffixInvalidCarrier) || errors.Is(err, store.ErrAffixCarrierNotFound) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_DETACH_INVALID_ITEM", "invalid affix detach item", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AFFIX_DETACH_FAILED", "detach affix failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleAttachEquipmentAffix(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req attachAffixRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ATTACH_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.EquipmentInstanceID <= 0 || req.AffixInstanceID <= 0 || req.SlotIndex < 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ATTACH_INVALID_REQUEST", "invalid attach request", "")
	}
	result, err := s.deps.Store.AttachEquipmentAffix(c.Context(), userID, req.EquipmentInstanceID, req.AffixInstanceID, req.SlotIndex)
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AFFIX_ATTACH_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ATTACH_INSTANCE_INVALID_STATE", "instance state must be inventory", "")
		}
		if errors.Is(err, store.ErrAffixSlotNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "AFFIX_ATTACH_SLOT_NOT_FOUND", "affix slot not found", "")
		}
		if errors.Is(err, store.ErrAffixSlotOccupied) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ATTACH_SLOT_OCCUPIED", "affix slot occupied", "")
		}
		if errors.Is(err, store.ErrAffixNoEmptySlot) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ATTACH_NO_EMPTY_SLOT", "no empty affix slot", "")
		}
		if errors.Is(err, store.ErrAffixRarityNotAllowed) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ATTACH_CARRIER_RARITY_NOT_ALLOWED", "carrier rarity rules mismatch", "")
		}
		if errors.Is(err, store.ErrAffixNotAllowedByEquipment) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ATTACH_AFFIX_NOT_ALLOWED", "affix not allowed by equipment", "")
		}
		if errors.Is(err, store.ErrAffixEquipmentMaxEffectiveExceeded) {
			affixID := ""
			var limitErr *store.AffixLimitExceededError
			if errors.As(err, &limitErr) {
				affixID = strings.TrimSpace(limitErr.AffixID)
			}
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ATTACH_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED", "affix exceeds equipment max effective count", "", affixID)
		}
		if errors.Is(err, store.ErrAffixInvalidCarrier) || errors.Is(err, store.ErrAffixCarrierNotFound) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ATTACH_INVALID_ITEM", "invalid affix item", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AFFIX_ATTACH_FAILED", "attach affix failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleEnchantEquipmentAffix(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req enchantAffixRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ENCHANT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.EquipmentInstanceID <= 0 || req.SlotIndex < 0 || strings.TrimSpace(req.ToolItemID) == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ENCHANT_INVALID_REQUEST", "invalid enchant request", "")
	}
	result, err := s.deps.Store.EnchantEquipmentAffix(c.Context(), userID, req.EquipmentInstanceID, req.SlotIndex, strings.TrimSpace(req.ToolItemID))
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AFFIX_ENCHANT_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ENCHANT_INSTANCE_INVALID_STATE", "instance state must be inventory", "")
		}
		if errors.Is(err, store.ErrAffixSlotNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "AFFIX_ENCHANT_SLOT_NOT_FOUND", "affix slot not found", "")
		}
		if errors.Is(err, store.ErrAffixSlotOccupied) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ENCHANT_SLOT_OCCUPIED", "affix slot occupied", "")
		}
		if errors.Is(err, store.ErrAffixNoEmptySlot) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ENCHANT_NO_EMPTY_SLOT", "no empty affix slot", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ENCHANT_INSUFFICIENT_ITEM", "insufficient enchant tool", "")
		}
		if errors.Is(err, store.ErrAffixEnchantNoRarityCandidate) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ENCHANT_NO_RARITY_CANDIDATE", "no rarity candidate available", "")
		}
		if errors.Is(err, store.ErrAffixEquipmentMaxEffectiveExceeded) {
			affixID := ""
			var limitErr *store.AffixLimitExceededError
			if errors.As(err, &limitErr) {
				affixID = strings.TrimSpace(limitErr.AffixID)
			}
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_ENCHANT_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED", "affix exceeds equipment max effective count", "", affixID)
		}
		if errors.Is(err, store.ErrAffixInvalidCarrier) || errors.Is(err, store.ErrAffixCarrierNotFound) || errors.Is(err, store.ErrAffixEnchantRuleNotFound) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_ENCHANT_INVALID_ITEM", "invalid enchant item", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AFFIX_ENCHANT_FAILED", "enchant affix failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:          []int64{userID},
		PlayerState:      true,
		OperationEffects: true,
		Inventory:        true,
		Equipment:        true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleSmeltCarrierAffixes(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req smeltAffixRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_SMELT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if len(req.CarrierInstanceIDs) == 0 || len(req.CarrierInstanceIDs) > 20 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_SMELT_INVALID_REQUEST", "invalid smelt request: need 1-20 carriers", "")
	}
	result, err := s.deps.Store.SmeltCarrierAffixes(c.Context(), userID, req.CarrierInstanceIDs)
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AFFIX_SMELT_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AFFIX_SMELT_INSTANCE_INVALID_STATE", "instance state must be inventory", "")
		}
		if errors.Is(err, store.ErrAffixInvalidCarrier) || errors.Is(err, store.ErrAffixCarrierNotFound) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AFFIX_SMELT_INVALID_CARRIER", "invalid carrier for smelting", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AFFIX_SMELT_FAILED", "smelt failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:   []int64{userID},
		Inventory: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleInventoryInstances(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	items, err := s.listInventoryInstances(c.Context(), userID, c.Query("fresh")!="1")
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "INVENTORY_LIST_FAILED", "list inventory instances failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": items})
}

func (s *Server) handleEquipmentInstances(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	items, err := s.deps.Store.ListEquippedItems(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "EQUIPMENT_LIST_FAILED", "list equipped items failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": items})
}

type inventoryInstancesArchiveRequest struct {
	InstanceIDs []int64 `json:"instanceIds"`
	Archived    bool    `json:"archived"`
}

func (s *Server) handleInventoryInstancesArchive(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req inventoryInstancesArchiveRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "INVENTORY_ARCHIVE_INVALID_PAYLOAD", "invalid archive payload", err.Error())
	}
	result, err := s.deps.Store.SetItemInstancesArchived(c.Context(), userID, req.InstanceIDs, req.Archived)
	if err != nil {
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "INVENTORY_ARCHIVE_INVALID_REQUEST", "invalid archive request: need 1-"+strconv.Itoa(store.InventoryArchiveBatchLimit)+" instances", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusNotFound, "INVENTORY_ARCHIVE_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "INVENTORY_ARCHIVE_INSTANCE_INVALID_STATE", "instance state must be inventory", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "INVENTORY_ARCHIVE_FAILED", "archive inventory instances failed", err.Error())
	}
	if result.UpdatedCount > 0 {
		s.syncAfterMutationNow(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"archived":     req.Archived,
		"updatedCount": result.UpdatedCount,
	})
}

func (s *Server) handlePlayerPresence(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	state, err := s.reconcilePresenceState(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PRESENCE_GET_FAILED", "get player presence failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, state)
}

type playerPresenceUpdateRequest struct {
	Status string `json:"status"`
}

func (s *Server) handlePlayerPresenceUpdate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req playerPresenceUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PRESENCE_STATUS_INVALID_PAYLOAD", "invalid presence payload", err.Error())
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	if status != "online" && status != "offline" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PRESENCE_STATUS_INVALID", "presence status must be online or offline", "")
	}
	state, offlineDurationS, err := s.applyPresenceStatusTransition(c.Context(), userID, status, time.Now())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PRESENCE_STATUS_UPDATE_FAILED", "update player presence failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"userId":                 state.UserID,
		"status":                 state.Status,
		"lastSeenAt":             state.LastSeenAt,
		"sessionVersion":         state.SessionVersion,
		"updatedAt":              state.UpdatedAt,
		"offlineDurationSeconds": offlineDurationS,
	})
}

func (s *Server) handlePlayerStats(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTE_SEEDS_INVALID", "player attribute seeds invalid", err.Error())
	}
	if err := s.ensurePlayerStatsSnapshot(c.Context(), userID, seeds); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_STATS_PREPARE_FAILED", "prepare player stats failed", err.Error())
	}
	stats, err := s.deps.Store.GetPlayerStats(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_STATS_GET_FAILED", "get stats failed", err.Error())
	}
	attributeStates, err := s.deps.Store.ListPlayerAttributes(c.Context(), userID, seeds)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTES_GET_FAILED", "get player attributes failed", err.Error())
	}
	progress, err := s.attributeProgressMap(attributeStates)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_ATTRIBUTE_PROGRESS_FAILED", "build attribute progress failed", err.Error())
	}
	attributes := s.attributeMapFromStates(attributeStates)
	battlePointState, _ := s.deps.Store.GetBattlePointState(c.Context(), userID)
	availableBattlePoints := battlePointState.TotalPoints - battlePointState.SpentPoints
	if availableBattlePoints < 0 {
		availableBattlePoints = 0
	}
	buildings, err := s.deps.Store.ListPlayerBuildings(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_BUILDINGS_GET_FAILED", "get player buildings failed", err.Error())
	}
	kitties, err := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_KITTIES_GET_FAILED", "get player kitties failed", err.Error())
	}
	basicAttack, err := s.playerBasicAttackPreview(c.Context(), userID, stats.Derived, attributeStates)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_BASIC_ATTACK_PREVIEW_FAILED", "build basic attack preview failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"derived":           stats.Derived,
		"cacheVersion":      stats.CacheVer,
		"updatedAt":         stats.UpdatedAt,
		"attributeProgress": progress,
		"attributes":        attributes,
		"buildings":         buildings,
		"kitties":           kitties,
		"basicAttack":       basicAttack,
		"battlePoints": fiber.Map{
			"totalPoints": battlePointState.TotalPoints,
			"spentPoints": battlePointState.SpentPoints,
			"available":   availableBattlePoints,
		},
		"formula": s.playerFormulaSnapshot(),
	})
}

func (s *Server) playerBasicAttackPreview(ctx context.Context, userID int64, derived map[string]float64, attributeStates []store.PlayerAttributeState) (*battleBasicAttackProfile, error) {
	equipped, err := s.deps.Store.ListEquippedItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(equipped) == 0 {
		return nil, nil
	}
	profile := s.resolveBattleBasicAttackProfile(equipped)
	if profile == nil {
		return nil, nil
	}
	ensureBasicAttackPreviewEffects(profile)
	if len(profile.Effects) == 0 {
		return profile, nil
	}
	stats := cloneFloatMap(derived)
	for _, row := range attributeStates {
		key := strings.TrimSpace(row.AttrKey)
		if key == "" {
			continue
		}
		stats[key] = row.FinalValue
	}
	contribution, err := s.playerSpecializationBattleContribution(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := applyBattleSpecializationStatModifiers(stats, contribution.StatModifiers); err != nil {
		return nil, err
	}
	specializationMutations, err := s.resolveBattleSpecializationEffectMutations(contribution)
	if err != nil {
		return nil, err
	}
	actor := &battleUnitState{
		Stats:                         stats,
		EffectMutations:               s.resolveBattleAffixEffectMutations(equipped),
		SpecializationEffectMutations: specializationMutations,
	}
	s.applyPlayerBasicAttackPreviewMutations(profile, actor)
	return profile, nil
}

func (s *Server) applyPlayerBasicAttackPreviewMutations(profile *battleBasicAttackProfile, actor *battleUnitState) {
	if s == nil || profile == nil || actor == nil || len(profile.Effects) == 0 {
		return
	}
	fallbackTargeting := profile.Effects[0].Targeting
	mutated := s.applyBattleAffixEffectMutations(actor, "basic_attack", s.battleDefaultBasicSkillID(), 1, fallbackTargeting, profile.Effects)
	profile.Effects = mutated
	if len(mutated) == 0 {
		ensureBasicAttackPreviewEffects(profile)
		return
	}
	profile.TargetDirection = firstNonEmptyTargetPosition(mutated[0].Targeting.TargetPositions)
	profile.TargetSeekStrategy = normalizeBattleSecondarySeek(mutated[0].Targeting.SecondarySeek, profile.TargetSeekStrategy)
	if strings.TrimSpace(mutated[0].DamageType) != "" {
		profile.DamageType = normalizeBattleDamageType(mutated[0].DamageType)
	}
	if terms := battleExtractBasicAttackFormulaTermsFromEffects(mutated); len(terms) > 0 {
		profile.Formula = terms
	}
	if spec, ok := battleExtractBasicAttackDamageSpecFromEffects(mutated, profile.DamageType, profile.Formula); ok {
		profile.DamageSpec = spec
	}
}

func ensureBasicAttackPreviewEffects(profile *battleBasicAttackProfile) {
	if profile == nil || len(profile.Effects) > 0 {
		return
	}
	legacyAmount := battleLegacyAmountFormulaFromBasicAttack(profile)
	if len(legacyAmount.Terms) == 0 && math.Abs(legacyAmount.BaseAdd) <= 1e-9 {
		return
	}
	targetDirection := normalizeBattleTargetPosition(profile.TargetDirection)
	if targetDirection == "" {
		targetDirection = "vanguard"
	}
	seekStrategy := normalizeBattleSecondarySeek(profile.TargetSeekStrategy, "front_to_rear")
	damageType := normalizeBattleDamageType(profile.DamageType)
	damageSpec := normalizeBattleDamageSpecForRuntime(profile.DamageSpec, damageType, legacyAmount)
	profile.DamageType = damageType
	profile.TargetDirection = targetDirection
	profile.TargetSeekStrategy = seekStrategy
	profile.DamageSpec = damageSpec
	profile.Effects = []battleSkillEffect{
		{
			Kind:       "damage",
			DamageType: damageType,
			Targeting: battleSkillTargeting{
				TargetPositions: []string{targetDirection},
				SecondarySeek:   seekStrategy,
				TargetScope:     "enemy",
				SelectRule:      "random",
				TargetCount: battleSkillTargetCount{
					Base: 1,
				},
			},
			AmountFormula: legacyAmount,
			DamageSpec:    damageSpec,
		},
	}
}

func (s *Server) ensurePlayerStatsSnapshot(ctx context.Context, userID int64, seeds []store.PlayerAttributeSeed) error {
	if err := s.deps.Store.EnsurePlayerAttributes(ctx, userID, seeds); err != nil {
		return err
	}
	_, err := s.deps.Store.GetPlayerStats(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("player stats cache missing")
	}
	return err
}

func (s *Server) attributeMapFromStates(states []store.PlayerAttributeState) fiber.Map {
	out := fiber.Map{}
	for _, state := range states {
		key := strings.TrimSpace(state.AttrKey)
		if key == "" {
			continue
		}
		out[key] = fiber.Map{
			"value":                   state.FinalValue,
			"baseValue":               state.BaseDefault + float64(state.BaseLevel-1) + float64(state.SkillPointLevel),
			"skillPointLevel":         state.SkillPointLevel,
			"unit":                    state.Unit,
			"category":                state.Category,
			"growthType":              state.GrowthType,
			"allowBattlePointLevelUp": state.AllowBattlePointLevelUp,
			"order":                   state.Order,
			"nameKey":                 state.NameKey,
			"descKey":                 state.DescKey,
			"iconId":                  state.IconID,
			"emojiIcon":               state.EmojiIcon,
			"sources":                 state.SourceMods,
		}
	}
	return out
}

func (s *Server) attributeProgressMap(states []store.PlayerAttributeState) (fiber.Map, error) {
	out := fiber.Map{}
	for _, state := range states {
		if state.GrowthType != "levelable" {
			continue
		}
		key := strings.TrimSpace(state.AttrKey)
		if key == "" {
			continue
		}
		progress, err := s.deps.Store.ResolveAttributeProgress(key, state.AttrExp)
		if err != nil {
			return nil, err
		}
		out[key] = toAttributeProgress(state.FinalValue, progress)
	}
	return out, nil
}

func (s *Server) playerAttributeSeeds() ([]store.PlayerAttributeSeed, error) {
	if len(s.deps.GameData.Attributes) == 0 {
		return nil, errors.New("game-data.attributes is empty")
	}
	out := make([]store.PlayerAttributeSeed, 0, len(s.deps.GameData.Attributes))
	for _, row := range s.deps.GameData.Attributes {
		key := strings.TrimSpace(row.ID)
		if key == "" {
			continue
		}
		out = append(out, store.PlayerAttributeSeed{
			AttrKey:                 key,
			BaseDefault:             row.BaseDefault,
			GrowthType:              row.GrowthType,
			Unit:                    row.Unit,
			Category:                row.Category,
			Order:                   row.Order,
			NameKey:                 row.NameKey,
			DescKey:                 row.DescKey,
			IconID:                  row.IconID,
			EmojiIcon:               row.EmojiIcon,
			AllowBattlePointLevelUp: row.AllowBattlePointLevelUp,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("game-data.attributes has no valid id")
	}
	return out, nil
}

func (s *Server) playerFormulaSnapshot() fiber.Map {
	derived := fiber.Map{}
	for _, formula := range s.deps.Config.GlobalSettings.DerivedStats.Formulas {
		targetAttr := strings.TrimSpace(formula.TargetAttr)
		if targetAttr == "" {
			continue
		}
		expression := config.BuildDerivedFormulaExpression(formula.Terms)
		if expression == "" {
			continue
		}
		derived[targetAttr] = expression
	}
	operators := append([]string(nil), s.deps.Config.GlobalSettings.DerivedStats.Operators...)
	return fiber.Map{
		"version":   s.deps.Config.GlobalSettings.DerivedStats.Version,
		"operators": operators,
		"derived":   derived,
	}
}

func (s *Server) currentBattleSkillSlots(ctx context.Context, userID int64) (int, error) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return 0, err
	}
	if err := s.ensurePlayerStatsSnapshot(ctx, userID, seeds); err != nil {
		return 0, err
	}
	attrs, err := s.deps.Store.ListPlayerAttributes(ctx, userID, seeds)
	if err != nil {
		return 0, err
	}
	stats := map[string]float64{}
	for _, row := range attrs {
		key := strings.TrimSpace(row.AttrKey)
		if key == "" {
			continue
		}
		stats[key] = row.FinalValue
	}
	return battleAvailableSkillSlots(stats["intelligence"], s.battleSkillSlotRule()), nil
}

func (s *Server) battleSkillCatalogRows(skillEligibility map[string]battleSkillEnableEvaluation) []fiber.Map {
	return s.battleSkillCatalogRowsWithCustom(skillEligibility, nil)
}

func (s *Server) battleSkillCatalogRowsWithCustom(skillEligibility map[string]battleSkillEnableEvaluation, custom map[string]map[string]any) []fiber.Map {
	skillTypeMap := make(map[string]fiber.Map, len(s.deps.GameData.SkillTypes))
	for _, rawType := range s.deps.GameData.SkillTypes {
		id, _ := stringFromAny(rawType["id"])
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		nameKey, _ := stringFromAny(rawType["nameKey"])
		descKey, _ := stringFromAny(rawType["descKey"])
		color, _ := stringFromAny(rawType["color"])
		skillTypeMap[strings.ToLower(id)] = fiber.Map{
			"id":      id,
			"nameKey": strings.TrimSpace(nameKey),
			"descKey": strings.TrimSpace(descKey),
			"color":   strings.TrimSpace(color),
		}
	}

	allSkills := append([]map[string]any(nil), s.deps.GameData.Skills...)
	customIDs := make([]string, 0, len(custom))
	for id := range custom {
		customIDs = append(customIDs, id)
	}
	sort.Strings(customIDs)
	for _, id := range customIDs {
		allSkills = append(allSkills, custom[id])
	}
	rows := make([]fiber.Map, 0, len(allSkills))
	seen := make(map[string]struct{}, len(allSkills))
	for _, raw := range allSkills {
		id, _ := stringFromAny(raw["id"])
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		name, _ := stringFromAny(raw["name"])
		nameKey, _ := stringFromAny(raw["nameKey"])
		descKey, _ := stringFromAny(raw["descKey"])
		category, _ := stringFromAny(raw["category"])
		targetMode, _ := stringFromAny(raw["targetMode"])
		targetDirection, _ := stringFromAny(raw["targetDirection"])
		statusRequirement := normalizeSkillStatusRequirement(raw)
		weaponRequirement := normalizeSkillWeaponRequirement(raw)
		if strings.TrimSpace(targetMode) == "" {
			targetMode = "single"
		}
		if strings.TrimSpace(targetDirection) == "" {
			targetDirection = "vanguard"
		}
		categoryInfo := fiber.Map{}
		if category != "" {
			if row, exists := skillTypeMap[strings.ToLower(strings.TrimSpace(category))]; exists {
				categoryInfo = row
			}
		}
		eligibility, exists := skillEligibility[key]
		if !exists {
			eligibility = battleSkillEnableEvaluation{Eligible: true}
		}
		damageType, _ := stringFromAny(raw["damageType"])
		unmetStatus := make([]fiber.Map, 0, len(eligibility.UnmetStatus))
		for _, row := range eligibility.UnmetStatus {
			unmetStatus = append(unmetStatus, fiber.Map{
				"attrId":   row.AttrID,
				"required": row.Required,
				"current":  row.Current,
			})
		}
		rows = append(rows, fiber.Map{
			"id":                      id,
			"comboEligible":           store.EquipmentComboEligible(raw),
			"dominationEligible":      store.EquipmentDominationEligible(raw),
			"name":                    strings.TrimSpace(name),
			"nameKey":                 strings.TrimSpace(nameKey),
			"descKey":                 strings.TrimSpace(descKey),
			"icon":                    raw["icon"],
			"category":                strings.TrimSpace(category),
			"categoryInfo":            categoryInfo,
			"hpCost":                  maxInt64(0, toInt64Loose(raw["hpCost"], 0)),
			"mpCost":                  maxInt64(0, toInt64Loose(raw["mpCost"], 0)),
			"cooldown":                maxInt64(0, toInt64Loose(raw["cooldown"], 0)),
			"maxLevel":                maxInt64(1, toInt64Loose(raw["maxLevel"], 200)),
			"targetMode":              strings.TrimSpace(strings.ToLower(targetMode)),
			"targetDirection":         strings.TrimSpace(strings.ToLower(targetDirection)),
			"targetBaseCount":         maxInt64(1, toInt64Loose(raw["targetBaseCount"], 1)),
			"targetFormula":           parseBattleAttackTerms(raw["targetFormula"]),
			"targeting":               raw["targeting"],
			"damageType":              strings.TrimSpace(strings.ToLower(damageType)),
			"effects":                 raw["effects"],
			"castGuards":              raw["castGuards"],
			"nameParams":              raw["nameParams"],
			"customForge":             raw["customForge"],
			"statusRequirement":       statusRequirement,
			"weaponRequirement":       weaponRequirement,
			"eligible":                eligibility.Eligible,
			"unmetStatusRequirements": unmetStatus,
			"missingWeaponClasses":    eligibility.MissingWeaponClasses,
		})
	}
	return rows
}

func (s *Server) applyBattleSkillCatalogAttackStatPreview(
	rows []fiber.Map,
	actor battleUnitState,
	ownedSkills []store.OwnedSkill,
	custom map[string]map[string]any,
	defaultBasicSkillID string,
) {
	if s == nil || len(rows) == 0 {
		return
	}
	previewActor, ok := battleAttackStatPolicyPreviewActor(actor)
	if !ok {
		return
	}
	type skillProgress struct {
		level int
		exp   int64
	}
	progressByID := make(map[string]skillProgress, len(ownedSkills))
	for _, owned := range ownedSkills {
		id := strings.ToLower(strings.TrimSpace(owned.SkillID))
		if id == "" {
			continue
		}
		progressByID[id] = skillProgress{level: maxInt(1, owned.Level), exp: maxInt64(0, owned.Exp)}
	}

	skillsByID := make(map[string]battleSkillState, len(rows))
	previewActor.Skills = make([]battleSkillState, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(fmt.Sprintf("%v", row["id"]))
		if id == "" || id == "<nil>" {
			continue
		}
		raw, exists := s.battleSkillRawByIDWithCustom(id, custom)
		if !exists {
			continue
		}
		progress := progressByID[strings.ToLower(id)]
		skill := s.battleSkillByRawWithProgress(id, maxInt(1, progress.level), progress.exp, raw)
		skillsByID[strings.ToLower(id)] = skill
		previewActor.Skills = append(previewActor.Skills, skill)
	}

	for _, row := range rows {
		id := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", row["id"])))
		skill, exists := skillsByID[id]
		if !exists {
			continue
		}
		scope := "skill"
		if strings.EqualFold(skill.ID, defaultBasicSkillID) {
			scope = "basic_attack"
		}
		effectiveSkill := previewActor.applyBattleSkillScalarMutations(scope, skill)
		effectiveSkill.Effects = s.applyBattleAffixEffectMutations(&previewActor, scope, effectiveSkill.ID, effectiveSkill.Level, effectiveSkill.Targeting, effectiveSkill.Effects)
		if len(effectiveSkill.Effects) > 0 {
			effectiveSkill.Targeting = effectiveSkill.Effects[0].Targeting
		}
		row["hpCost"] = roundTo4(effectiveSkill.HPCost)
		row["mpCost"] = roundTo4(effectiveSkill.MPCost)
		row["cooldown"] = effectiveSkill.CooldownTurns
		row["targeting"] = effectiveSkill.Targeting
		row["effects"] = effectiveSkill.Effects
	}
}

func (s *Server) battleSkillExists(skillID string) bool {
	skillID = strings.ToLower(strings.TrimSpace(skillID))
	if skillID == "" {
		return false
	}
	for _, raw := range s.deps.GameData.Skills {
		id, _ := stringFromAny(raw["id"])
		if strings.ToLower(strings.TrimSpace(id)) == skillID {
			return true
		}
	}
	return false
}

func toAttributeProgress(value float64, progress store.AttributeProgress) attributeProgress {
	return attributeProgress{
		Value:   value,
		Level:   int(progress.Level),
		Exp:     progress.Exp,
		NextExp: progress.NextExp,
	}
}

func (s *Server) handleEquipmentOperationInfo(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	instanceID, err := strconv.ParseInt(c.Params("instanceID"), 10, 64)
	if err != nil || instanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIPMENT_OPERATION_INVALID_INSTANCE", "invalid instance id", "")
	}
	instance, _, getErr := s.deps.Store.GetItemInstance(c.Context(), userID, instanceID)
	if getErr != nil {
		if errors.Is(getErr, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "EQUIPMENT_OPERATION_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "EQUIPMENT_OPERATION_INSTANCE_LOAD_FAILED", "load item instance failed", getErr.Error())
	}
	itemDef := s.findCatalogItem(instance.TemplateItemID)
	if itemDef == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "EQUIPMENT_OPERATION_ITEM_NOT_FOUND", "item not found", "")
	}
	template := s.findEquipmentTemplate(itemDef.ID)
	if !isEquipmentItem(itemDef, template) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIPMENT_ITEM_NOT_EQUIPMENT", "item is not equipment", "")
	}
	info, err := s.deps.Store.GetEquipmentOperationInfo(c.Context(), userID, instanceID)
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "EQUIPMENT_OPERATION_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "EQUIPMENT_OPERATION_NOT_FOUND", "equipment not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "EQUIPMENT_OPERATION_INFO_FAILED", "load operation info failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, info)
}

func (s *Server) handleItemDetail(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	itemID := strings.TrimSpace(c.Query("itemId"))
	instanceID := int64(0)
	if raw := strings.TrimSpace(c.Query("instanceId")); raw != "" {
		instanceID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || instanceID <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_DETAIL_INVALID_INSTANCE", "invalid instance id", "")
		}
	}
	if itemID == "" && instanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_DETAIL_MISSING_KEY", "itemId or instanceId is required", "")
	}
	var (
		instance     *store.ItemInstance
		equippedSlot string
	)
	if instanceID > 0 {
		it, slot, getErr := s.deps.Store.GetItemInstance(c.Context(), userID, instanceID)
		if getErr != nil {
			if errors.Is(getErr, pgx.ErrNoRows) {
				return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_DETAIL_INSTANCE_NOT_FOUND", "item instance not found", "")
			}
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_DETAIL_INSTANCE_FAILED", "load item instance failed", getErr.Error())
		}
		if itemID != "" && itemID != it.TemplateItemID {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_DETAIL_ITEM_INSTANCE_MISMATCH", "itemId and instanceId mismatch", "")
		}
		itemID = it.TemplateItemID
		equippedSlot = slot
		instance = &it
	}

	itemDef := s.findCatalogItem(itemID)
	if itemDef == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_DETAIL_NOT_FOUND", "item not found", "")
	}
	enhanceSuccessRateBonus := float64(0)
	stats, statsErr := s.deps.Store.GetPlayerStats(c.Context(), userID)
	if statsErr != nil && !errors.Is(statsErr, pgx.ErrNoRows) {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_DETAIL_PLAYER_STATS_FAILED", "load player stats failed", statsErr.Error())
	}
	if statsErr == nil {
		if rawBonus, ok := stats.Derived["enhancement_success_rate"]; ok && rawBonus > 0 {
			enhanceSuccessRateBonus = rawBonus
		}
	}
	resp := fiber.Map{
		"item":                                 itemDef,
		"recycleGoldCoefficient":               s.deps.Config.GlobalSettings.Economy.ItemRecycleGoldCoefficient,
		"recycleAffixMismatchExtraCoefficient": s.deps.Config.GlobalSettings.Economy.ItemRecycleAffixMismatchExtraCoefficient,
	}
	if effectID := strings.TrimSpace(itemDef.UseOperationEffectID); effectID != "" {
		baseSnapshot := map[string]any(nil)
		if instance != nil {
			baseSnapshot = instance.BaseSnapshot
		}
		definition, ok, definitionErr := s.deps.Store.OperationEffectDefinitionForItem(itemID, baseSnapshot)
		if definitionErr != nil {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_EFFECT_SNAPSHOT_INVALID", "item effect snapshot is invalid", definitionErr.Error())
		}
		if ok {
			resp["operationEffect"] = definition
		}
	}
	if instance != nil {
		resp["instance"] = fiber.Map{
			"instanceId":     instance.InstanceID,
			"templateItemId": instance.TemplateItemID,
			"customData":     instance.CustomData,
			"quantity":       instance.Quantity,
			"enhanceLevel":   instance.EnhanceLevel,
			"durability":     instance.Durability,
			"maxDurability":  instance.MaxDurability,
			"state":          instance.State,
			"equippedSlot":   equippedSlot,
			"baseSnapshot":   instance.BaseSnapshot,
			"affixes":        instance.Affixes,
		}
		if instance.FishData != nil {
			resp["instance"].(fiber.Map)["fishData"] = instance.FishData
		}
		if instance.ForgedSkillbook != nil {
			resp["instance"].(fiber.Map)["forgedSkillBook"] = instance.ForgedSkillbook
		}
		info, infoErr := s.deps.Store.GetEquipmentOperationInfo(c.Context(), userID, instance.InstanceID)
		if infoErr != nil && !errors.Is(infoErr, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_DETAIL_OPERATION_INFO_FAILED", "load item operation info failed", infoErr.Error())
		}
		if infoErr == nil {
			resp["operations"] = info
		}
		if runtime, ok := s.deps.Store.EnhanceTemplateRuntime(instance.TemplateItemID); ok && runtime.MaxLevel > 0 {
			cumulativeAttrs := s.deps.Store.EnhanceCumulativeAttrs(instance.TemplateItemID, instance.EnhanceLevel)
			nextPreview, hasNext := s.deps.Store.EnhanceNextPreview(instance.TemplateItemID, instance.EnhanceLevel)
			enhancePayload := fiber.Map{
				"maxLevel":        runtime.MaxLevel,
				"currentLevel":    instance.EnhanceLevel,
				"cumulativeAttrs": cumulativeAttrs,
			}
			if hasNext {
				baseSuccessRate := clampEnhanceSuccessRate(nextPreview.SuccessRate)
				successRateBonus := clampEnhanceSuccessRate(enhanceSuccessRateBonus)
				totalSuccessRate := clampEnhanceSuccessRate(baseSuccessRate + successRateBonus)
				enhancePayload["nextPreview"] = fiber.Map{
					"nextLevel":        nextPreview.NextLevel,
					"baseSuccessRate":  baseSuccessRate,
					"successRateBonus": successRateBonus,
					"successRate":      totalSuccessRate,
					"costs":            nextPreview.Costs,
					"deltaAttrs":       nextPreview.DeltaAttrs,
				}
			}
			resp["enhance"] = enhancePayload
		}
	}
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}

type itemRenameRequest struct {
	InstanceID int64  `json:"instanceId"`
	Name       string `json:"name"`
	LabelColor string `json:"labelColor"`
}

func (s *Server) handleRenameItem(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req itemRenameRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RENAME_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	name := strings.TrimSpace(req.Name)
	labelColor := strings.TrimSpace(strings.ToLower(req.LabelColor))
	if labelColor == "" {
		labelColor = s.deps.Store.DefaultItemRenameLabelColor()
	}
	maxRunes := s.deps.Store.ItemRenameMaxNameRunes()
	if req.InstanceID <= 0 || name == "" || usernameLength(name) > maxRunes || !store.IsValidItemCustomName(name) || !s.deps.Store.IsValidItemRenameLabelColor(labelColor) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RENAME_INVALID_NAME", "invalid item name", "")
	}
	if err := s.rejectModeratedAPI(c, moderationRuleItemCustomName, name, userID); err != nil {
		return err
	}
	item, err := s.deps.Store.RenameItemInstance(c.Context(), userID, req.InstanceID, name, labelColor)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_RENAME_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "ITEM_RENAME_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_RENAME_INSTANCE_INVALID_STATE", "instance state invalid", "")
		}
		if errors.Is(err, store.ErrItemRenameStackableItem) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RENAME_STACKABLE_ITEM", "stackable item cannot be renamed", "")
		}
		if errors.Is(err, store.ErrItemRenameSameName) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_RENAME_SAME_NAME", "same item name", "")
		}
		if errors.Is(err, store.ErrItemRenameInvalidName) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RENAME_INVALID_NAME", "invalid item name", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_RENAME_INSUFFICIENT_ITEM", "insufficient rename material", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_RENAME_FAILED", "rename item failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":          true,
		"instance":    item,
		"cost":        s.deps.Store.ItemRenameCost(),
		"labelColors": s.deps.Store.ItemRenameLabelColors(),
	})
}

func clampEnhanceSuccessRate(raw float64) float64 {
	if raw < 0 {
		raw = 0
	}
	if raw > 1 {
		raw = 1
	}
	return math.Round(raw*10000) / 10000
}

func (s *Server) handleUseSkillbook(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req useSkillbookRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_USE_SKILLBOOK_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_USE_SKILLBOOK_INVALID_REQUEST", "invalid request", "")
	}
	instance, _, err := s.deps.Store.GetItemInstance(c.Context(), userID, req.InstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_USE_SKILLBOOK_NOT_FOUND", "item instance not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_USE_SKILLBOOK_LOAD_FAILED", "load item instance failed", err.Error())
	}
	itemDef := s.findCatalogItem(instance.TemplateItemID)
	if itemDef == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_USE_SKILLBOOK_ITEM_NOT_FOUND", "item not found", "")
	}
	forged, forgedErr := s.deps.Store.GetForgedSkillbookByInstance(c.Context(), userID, req.InstanceID)
	if forgedErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_USE_SKILLBOOK_LOAD_FAILED", "load forged skillbook failed", forgedErr.Error())
	}
	if forged == nil && (!itemDef.HasType("skillbook") || strings.TrimSpace(itemDef.UseGrantSkillID) == "") {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_USE_SKILLBOOK_INVALID_TYPE", "item is not a skillbook", "")
	}
	targetType := strings.ToLower(strings.TrimSpace(req.TargetType))
	if targetType == "" {
		targetType = "player"
	}
	if targetType != "player" && targetType != "kitty" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_USE_SKILLBOOK_INVALID_REQUEST", "invalid request", "")
	}
	if targetType == "kitty" && req.TargetKittyID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_USE_SKILLBOOK_INVALID_REQUEST", "invalid request", "")
	}
	if forged != nil {
		result, useErr := s.deps.Store.UseForgedSkillbook(c.Context(), userID, req.InstanceID, targetType, req.TargetKittyID)
		if useErr != nil {
			if errors.Is(useErr, store.ErrSkillForgeRulesetDisabled) {
				return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_RULESET_DISABLED", "this skill forge ruleset is disabled", useErr.Error())
			}
			if errors.Is(useErr, store.ErrForgedSkillbookConflict) {
				return s.writeAPIError(c, fiber.StatusConflict, "FORGED_SKILLBOOK_TARGET_CONFLICT", "forged skillbook already resolved for another target", useErr.Error())
			}
			if errors.Is(useErr, store.ErrKittyNotFound) {
				return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
			}
			return s.writeAPIError(c, fiber.StatusBadRequest, "FORGED_SKILLBOOK_USE_FAILED", "use forged skillbook failed", useErr.Error())
		}
		s.invalidatePlayerProfileCaches(userID)
		s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: true})
		return s.writeAPISuccess(c, fiber.StatusOK, result)
	}
	skillExpGain := itemDef.UseSkillExpGain
	if skillExpGain <= 0 {
		skillExpGain = 100
	}
	count := req.Count
	if count < 1 {
		count = 1
	}
	if count > 9999 {
		count = 9999
	}
	var result store.SkillbookUseResult
	if targetType == "kitty" {
		result, err = s.deps.Store.UseKittySkillbook(c.Context(), userID, req.TargetKittyID, req.InstanceID, itemDef.UseGrantSkillID, skillExpGain, count)
	} else {
		result, err = s.deps.Store.UseSkillbook(c.Context(), userID, req.InstanceID, itemDef.UseGrantSkillID, skillExpGain, count)
	}
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "ITEM_USE_SKILLBOOK_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_USE_SKILLBOOK_INVALID_STATE", "instance state invalid", "")
		}
		if errors.Is(err, store.ErrInvalidSkillID) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_USE_SKILLBOOK_INVALID_SKILL", "invalid skill id", "")
		}
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_USE_SKILLBOOK_INSUFFICIENT", "insufficient item quantity", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_USE_SKILLBOOK_FAILED", "use skillbook failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleDestroyForgedSkillbook(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req struct {
		InstanceID int64 `json:"instanceId"`
	}
	if err := decodeStrictJSON(c.Body(), &req); err != nil || req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FORGED_SKILLBOOK_DESTROY_INVALID", "invalid request", "")
	}
	if err := s.deps.Store.DestroyForgedSkillbook(c.Context(), userID, req.InstanceID); err != nil {
		return s.writeAPIError(c, fiber.StatusConflict, "FORGED_SKILLBOOK_DESTROY_FAILED", "destroy forged skillbook failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

type recycleItemRequest struct {
	InstanceID int64 `json:"instanceId"`
	Quantity   int64 `json:"quantity"`
}

func (s *Server) handleRecycleItem(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req recycleItemRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 || req.Quantity <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_INVALID_REQUEST", "invalid request", "")
	}
	result, err := s.deps.Store.RecycleItemInstance(c.Context(), userID, req.InstanceID, req.Quantity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_RECYCLE_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "ITEM_RECYCLE_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) || errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_RECYCLE_STATE_OR_QUANTITY_CONFLICT", "instance state or quantity conflict", "")
		}
		if errors.Is(err, store.ErrBadQuantity) || errors.Is(err, store.ErrItemNotRecyclable) || errors.Is(err, store.ErrRecyclePriceInvalid) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_INVALID_OPERATION", "invalid recycle operation", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_RECYCLE_FAILED", "recycle item failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":               true,
		"recycledQuantity": result.RecycledQuantity,
		"unitPrice":        result.UnitPrice,
		"totalGold":        result.TotalGold,
		"instanceRemoved":  result.InstanceRemoved,
	})
}

type recycleBatchRequestItem struct {
	InstanceID int64 `json:"instanceId"`
	Quantity   int64 `json:"quantity"`
}

type recycleBatchRequest struct {
	Items []recycleBatchRequestItem `json:"items"`
}

func (s *Server) handleRecycleItemBatch(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req recycleBatchRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_BATCH_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if len(req.Items) == 0 || len(req.Items) > store.RecycleBatchItemLimit {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_BATCH_INVALID_REQUEST", "invalid request: need 1-"+strconv.Itoa(store.RecycleBatchItemLimit)+" items", "")
	}
	storeItems := make([]store.RecycleBatchItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		if it.InstanceID <= 0 || it.Quantity <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_BATCH_INVALID_REQUEST", "invalid item in request", "")
		}
		storeItems = append(storeItems, store.RecycleBatchItemInput{
			InstanceID: it.InstanceID,
			Quantity:   it.Quantity,
		})
	}
	result, err := s.deps.Store.RecycleItemInstanceBatch(c.Context(), userID, storeItems)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ITEM_RECYCLE_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "ITEM_RECYCLE_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) || errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "ITEM_RECYCLE_STATE_OR_QUANTITY_CONFLICT", "instance state or quantity conflict", "")
		}
		if errors.Is(err, store.ErrBadQuantity) || errors.Is(err, store.ErrItemNotRecyclable) || errors.Is(err, store.ErrRecyclePriceInvalid) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ITEM_RECYCLE_INVALID_OPERATION", "invalid recycle operation", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ITEM_RECYCLE_BATCH_FAILED", "batch recycle failed", err.Error())
	}
	resultItems := make([]fiber.Map, 0, len(result.Results))
	for _, r := range result.Results {
		resultItems = append(resultItems, fiber.Map{
			"instanceId":       r.InstanceID,
			"recycledQuantity": r.RecycledQuantity,
			"unitPrice":        r.UnitPrice,
			"totalGold":        r.TotalGold,
			"instanceRemoved":  r.InstanceRemoved,
		})
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":        true,
		"results":   resultItems,
		"totalGold": result.TotalGold,
	})
}

func (s *Server) handleCatVillageCommissions(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cfg := s.deps.Config.GlobalSettings.CatVillageCommission
	tasks, nextRefreshAt, repState, err := s.deps.Store.ListCatVillageCommissions(c.Context(), userID, time.Now(), s.deps.GameData.CatVillageCommissions, cfg)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CAT_VILLAGE_TASK_LIST_FAILED", "list cat village commissions failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"tasks":         tasks,
		"nextRefreshAt": nextRefreshAt,
		"maxActive":     cfg.MaxActiveTasks,
		"refreshBatch":  cfg.RefreshBatchSize,
		"refreshEveryH": cfg.RefreshIntervalHours,
		"reputation":    repState,
		"refreshCost":   s.deps.Config.SharedGlobalSettings.CatVillageCommission.RefreshCost,
	})
}

func (s *Server) handleCatVillageCommissionDelete(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	taskID, err := strconv.ParseInt(c.Params("taskID"), 10, 64)
	if err != nil || taskID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CAT_VILLAGE_TASK_INVALID_ID", "invalid task id", "")
	}
	if err = s.deps.Store.DeleteCatVillageCommission(c.Context(), userID, taskID); err != nil {
		if errors.Is(err, store.ErrCatVillageTaskNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CAT_VILLAGE_TASK_NOT_FOUND", "task not found", "")
		}
		if errors.Is(err, store.ErrCatVillageTaskNotDeletable) {
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_NOT_DELETABLE", "commission cannot be deleted", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CAT_VILLAGE_TASK_DELETE_FAILED", "delete commission failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleCatVillageCommissionRefresh(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	taskID, err := strconv.ParseInt(c.Params("taskID"), 10, 64)
	if err != nil || taskID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CAT_VILLAGE_TASK_INVALID_ID", "invalid task id", "")
	}
	refreshCost := s.deps.Config.SharedGlobalSettings.CatVillageCommission.RefreshCost
	newTask, err := s.deps.Store.RefreshCatVillageCommission(c.Context(), userID, taskID, s.deps.GameData.CatVillageCommissions, refreshCost)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrCatVillageTaskNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "CAT_VILLAGE_TASK_NOT_FOUND", "task not found", "")
		case errors.Is(err, store.ErrCatVillageTaskNotOpen):
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_NOT_OPEN", "task not open", "")
		case errors.Is(err, store.ErrCatVillageTaskAlreadyRefreshed):
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_ALREADY_REFRESHED", "task already refreshed", "")
		case errors.Is(err, store.ErrCatVillageTaskNotRefreshable):
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_NOT_REFRESHABLE", "rumor commission cannot be refreshed", "")
		case errors.Is(err, store.ErrInsufficientItem):
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_INSUFFICIENT_ITEMS", "insufficient items for refresh", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CAT_VILLAGE_TASK_REFRESH_FAILED", "refresh commission failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"task": newTask})
}

func (s *Server) handleCatVillageCommissionSubmit(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	taskID, err := strconv.ParseInt(c.Params("taskID"), 10, 64)
	if err != nil || taskID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CAT_VILLAGE_TASK_INVALID_ID", "invalid task id", "")
	}
	commissionByID := make(map[string]catalog.CatVillageCommission, len(s.deps.GameData.CatVillageCommissions))
	for _, t := range s.deps.GameData.CatVillageCommissions {
		commissionByID[t.ID] = t
	}
	if err = s.deps.Store.SubmitCatVillageCommission(c.Context(), userID, taskID, commissionByID); err != nil {
		if errors.Is(err, store.ErrCatVillageTaskNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CAT_VILLAGE_TASK_NOT_FOUND", "task not found", "")
		}
		if errors.Is(err, store.ErrCatVillageTaskNotOpen) {
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_ALREADY_COMPLETED", "task already completed", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "CAT_VILLAGE_TASK_INSUFFICIENT_ITEMS", "insufficient items", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CAT_VILLAGE_TASK_SUBMIT_FAILED", "submit cat village commission failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

type gameplayEquipRequest struct {
	InstanceID int64  `json:"instanceId"`
	Slot       string `json:"slot"`
	EquipType  string `json:"equipType"`
}

type gameplayUnequipRequest struct {
	Slot      string `json:"slot"`
	EquipType string `json:"equipType"`
}

func (s *Server) handleEquip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req gameplayEquipRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.Slot == "" || req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_INVALID_REQUEST", "invalid equip request", "")
	}
	req.EquipType = normalizedEquipType(req.EquipType)
	if req.EquipType == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_TYPE_REQUIRED", "equip type required", "")
	}

	instance, _, getErr := s.deps.Store.GetItemInstance(c.Context(), userID, req.InstanceID)
	if getErr != nil {
		if errors.Is(getErr, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "EQUIP_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "EQUIP_INSTANCE_LOAD_FAILED", "load equip instance failed", getErr.Error())
	}

	itemDef := s.findCatalogItem(instance.TemplateItemID)
	if itemDef == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "EQUIP_ITEM_NOT_FOUND", "equip item not found", "")
	}
	template := s.findEquipmentTemplate(itemDef.ID)
	if !isEquipmentItem(itemDef, template) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIPMENT_ITEM_NOT_EQUIPMENT", "item is not equipment", "")
	}
	if templateEquipType(template) != req.EquipType {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_TYPE_MISMATCH", "equip type mismatch", "")
	}
	if !isEquipTargetAllowed(template, "player") {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_TARGET_NOT_ALLOWED", "equipment target not allowed", "")
	}
	if err = s.deps.Store.EquipItemToSlot(c.Context(), userID, req.InstanceID, req.EquipType, req.Slot); err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "EQUIP_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		instanceState := strings.TrimSpace(instance.State)
		var stateErr *store.InvalidInstanceStateError
		if errors.As(err, &stateErr) {
			if strings.TrimSpace(stateErr.State) != "" {
				instanceState = strings.TrimSpace(stateErr.State)
			}
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			if instanceState != "" {
				return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_INSTANCE_STATE_INVALID", "instance state invalid for equip: "+instanceState, "", instanceState)
			}
			return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_INSTANCE_STATE_INVALID", "instance state invalid for equip", "")
		}
		if errors.Is(err, store.ErrEquipSlotNotAllowed) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_SLOT_NOT_ALLOWED", "slot not allowed for this item", "")
		}
		if errors.Is(err, store.ErrEquipSlotDisabledBySpecialization) {
			return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_SLOT_DISABLED_BY_SPECIALIZATION", "slot is disabled by specialization", "")
		}
		if errors.Is(err, store.ErrEquipTargetNotAllowed) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIP_TARGET_NOT_ALLOWED", "equipment target not allowed", "")
		}
		if errors.Is(err, store.ErrStatusRequireNotMet) {
			return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_STATUS_REQUIREMENT_NOT_MET", "status requirements not met", "")
		}
		if errors.Is(err, store.ErrEquipAccessoryConflict) {
			return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_ACCESSORY_CONFLICT", "accessory template or group already equipped", "")
		}
		if errors.Is(err, store.ErrFishingTackleInUse) {
			return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_FISHING_TACKLE_IN_USE", "fishing tackle is in use", "")
		}
		if errors.Is(err, store.ErrAffixBodyMaxEffectiveExceeded) {
			affixID := ""
			var limitErr *store.AffixLimitExceededError
			if errors.As(err, &limitErr) {
				affixID = strings.TrimSpace(limitErr.AffixID)
			}
			return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_AFFIX_BODY_MAX_EFFECTIVE_EXCEEDED", "affix exceeds body max effective count", "", affixID)
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "EQUIP_FAILED", "equip failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleUnequip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req gameplayUnequipRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "UNEQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.Slot == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "UNEQUIP_SLOT_REQUIRED", "slot required", "")
	}
	req.EquipType = normalizedEquipType(req.EquipType)
	if req.EquipType == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "UNEQUIP_TYPE_REQUIRED", "equip type required", "")
	}
	if err = s.deps.Store.UnequipItem(c.Context(), userID, req.EquipType, req.Slot); err != nil {
		if errors.Is(err, store.ErrEquipSlotNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "UNEQUIP_SLOT_EMPTY", "slot is empty", "")
		}
		if errors.Is(err, store.ErrFishingTackleInUse) {
			return s.writeAPIError(c, fiber.StatusConflict, "UNEQUIP_FISHING_TACKLE_IN_USE", "fishing tackle is in use", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "UNEQUIP_FAILED", "unequip failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleUnequipInstance(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req unequipInstanceRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "UNEQUIP_INSTANCE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "UNEQUIP_INSTANCE_INVALID_REQUEST", "instanceId required", "")
	}
	slot, err := s.deps.Store.UnequipInstance(c.Context(), userID, req.InstanceID)
	if err != nil {
		if errors.Is(err, store.ErrEquipSlotNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "UNEQUIP_INSTANCE_NOT_EQUIPPED", "item is not equipped", "")
		}
		if errors.Is(err, store.ErrFishingTackleInUse) {
			return s.writeAPIError(c, fiber.StatusConflict, "UNEQUIP_FISHING_TACKLE_IN_USE", "fishing tackle is in use", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "UNEQUIP_INSTANCE_FAILED", "unequip instance failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
		Equipment:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "slot": slot})
}

func (s *Server) findCatalogItem(itemID string) *catalog.Item {
	return s.deps.GameData.ItemByID(itemID)
}

func (s *Server) findEquipmentTemplate(itemID string) *catalog.EquipmentTemplate {
	return s.deps.GameData.EquipmentTemplateByID(itemID)
}

func equipAllowedSlots(template *catalog.EquipmentTemplate) []string {
	if template == nil {
		return nil
	}
	slot := strings.TrimSpace(template.Slot)
	if slot == "" {
		return nil
	}
	switch slot {
	case "weapon":
		return []string{"left_hand", "right_hand"}
	case "accessory":
		return []string{"accessory_1", "accessory_2", "accessory_3", "accessory_4"}
	default:
		return []string{slot}
	}
}

func normalizedEquipType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "work":
		return "work"
	case "battle":
		return "battle"
	default:
		return ""
	}
}

func templateEquipType(template *catalog.EquipmentTemplate) string {
	if template == nil {
		return ""
	}
	if normalized := normalizedEquipType(template.EquipType); normalized != "" {
		return normalized
	}
	return "battle"
}

func isEquipmentItem(item *catalog.Item, template *catalog.EquipmentTemplate) bool {
	return item != nil && template != nil
}

func equipOccupySlots(template *catalog.EquipmentTemplate, selectedSlot string) []string {
	if template == nil || selectedSlot == "" {
		return nil
	}
	if len(template.OccupySlots) > 0 {
		return uniqueStrings(template.OccupySlots)
	}
	return []string{selectedSlot}
}

func accessoryTemplateIDsByTemplate(template *catalog.EquipmentTemplate) []string {
	if template == nil {
		return nil
	}
	if strings.TrimSpace(template.Slot) != "accessory" {
		return nil
	}
	id := strings.TrimSpace(template.ID)
	if id == "" {
		return nil
	}
	return []string{id}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intersectSlots(slots []string, whitelist []string) []string {
	if len(slots) == 0 || len(whitelist) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(whitelist))
	for _, slot := range whitelist {
		normalized := strings.TrimSpace(slot)
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	out := make([]string, 0, len(slots))
	for _, slot := range slots {
		normalized := strings.TrimSpace(slot)
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; ok {
			out = append(out, normalized)
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

type marketListInstanceRequest struct {
	InstanceID int64 `json:"instanceId"`
	Price      int64 `json:"price"`
}

func (s *Server) handleMarketListInstance(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketListInstanceRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INSTANCE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 || req.Price <= 0 || req.Price > 1_000_000_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INSTANCE_INVALID_REQUEST", "invalid request", "")
	}
	id, err := s.deps.Market.ListInstance(c.Context(), userID, req.InstanceID, req.Price)
	if err != nil {
		if errors.Is(err, market.ErrInstanceListingDisabled) {
			return s.writeAPIError(c, fiber.StatusGone, "MARKET_INSTANCE_LIST_DISABLED", "market instance listing is disabled; use auction for instances", "")
		}
		if errors.Is(err, market.ErrInvalidItem) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_ITEM_ID", "invalid item id", "")
		}
		if errors.Is(err, market.ErrItemNotListable) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_NOT_LISTABLE", "item cannot be listed on market", "")
		}
		if errors.Is(err, market.ErrPriceBelowRealValue) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_PRICE_BELOW_REAL_VALUE", "price below real value", "")
		}
		if errors.Is(err, market.ErrPriceStepInvalid) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_PRICE_STEP_INVALID", "price does not match market step rule", "")
		}
		if errors.Is(err, market.ErrInstanceMustAuction) || errors.Is(err, store.ErrMarketItemMustAuction) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSTANCE_REQUIRE_AUCTION", "special equipment must be listed in auction", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "MARKET_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSTANCE_STATE_INVALID", "instance state invalid for listing", "")
		}
		if errors.Is(err, store.ErrMarketItemOrderSideConflict) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_ORDER_SIDE_CONFLICT", "cannot create listing when active buy order exists for this item", "")
		}
		if errors.Is(err, store.ErrMarketItemOrderPriceConflict) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_ORDER_PRICE_CONFLICT", "active listing has different price for this item", "")
		}
		if errors.Is(err, store.ErrMarketItemOrderAlreadyExists) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_ORDER_ALREADY_EXISTS", "active item order already exists", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_INSTANCE_LIST_FAILED", "list instance failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"listingId": id})
}

type useCollectibleRequest struct {
	InstanceID int64 `json:"instanceId"`
}

func (s *Server) handleUseCollectible(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req useCollectibleRequest
	if err = c.BodyParser(&req); err != nil || req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COLLECTIBLE_USE_INVALID_REQUEST", "invalid request", "")
	}
	instance, _, err := s.deps.Store.GetItemInstance(c.Context(), userID, req.InstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "COLLECTIBLE_NOT_FOUND", "item instance not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COLLECTIBLE_LOAD_FAILED", "load item instance failed", err.Error())
	}
	itemDef := s.findCatalogItem(instance.TemplateItemID)
	if itemDef == nil || !itemDef.HasType("collectible") {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COLLECTIBLE_INVALID_TYPE", "item is not a collectible", "")
	}
	collectibleID := strings.TrimSpace(itemDef.UseGrantCollectibleID)
	collectibleDef := s.deps.GameData.CollectibleByID(collectibleID)
	resolvedViaDirectLookup := collectibleDef != nil
	if !resolvedViaDirectLookup {
		collectibleDef = s.deps.GameData.CollectibleByItemID(instance.TemplateItemID)
	}
	if collectibleDef == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "COLLECTIBLE_DEF_NOT_FOUND", "collectible definition not found", "")
	}
	// When resolved via itemID fallback, enforce ItemID match. Direct lookup is authorized by config.
	if !resolvedViaDirectLookup && collectibleDef.ItemID != instance.TemplateItemID {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COLLECTIBLE_ITEM_MISMATCH", "collectible item mismatch", "")
	}
	newLevel, err := s.deps.Store.AddCollectible(c.Context(), userID, req.InstanceID, *collectibleDef)
	if err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "COLLECTIBLE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrCollectibleAlreadyOwned) {
			return s.writeAPIError(c, fiber.StatusConflict, "COLLECTIBLE_ALREADY_OWNED", "collectible already owned", "")
		}
		if errors.Is(err, store.ErrCollectibleMaxLevel) {
			return s.writeAPIError(c, fiber.StatusConflict, "COLLECTIBLE_MAX_LEVEL", "collectible max level reached", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COLLECTIBLE_ADD_FAILED", "add collectible failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"collectibleId": collectibleDef.ID,
		"newLevel":      newLevel,
		"wasUpgraded":   newLevel > 1,
	})
}

func (s *Server) handleGetCollectibles(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	collectibles, err := s.deps.Store.GetCollectibles(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COLLECTIBLES_LIST_FAILED", "list collectibles failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"collectibles": collectibles})
}
