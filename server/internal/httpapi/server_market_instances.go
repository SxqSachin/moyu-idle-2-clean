package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/market"
	"mysecondwebgame/server/internal/store"
)

func decodeInstanceMarketBody(c *fiber.Ctx, out any) error {
	if len(c.Body()) == 0 || len(c.Body()) > 32768 {
		return store.ErrInstanceMarketRequest
	}
	d := json.NewDecoder(bytes.NewReader(c.Body()))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return store.ErrInstanceMarketRequest
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return store.ErrInstanceMarketRequest
	}
	return nil
}

func (s *Server) instanceMarketError(c *fiber.Ctx, err error) error {
	code, status := "INSTANCE_MARKET_FAILED", fiber.StatusInternalServerError
	switch {
	case errors.Is(err, store.ErrInstanceMarketChanged):
		code, status = "INSTANCE_MARKET_CHANGED", fiber.StatusConflict
	case errors.Is(err, store.ErrInstanceMarketUnavailable), errors.Is(err, store.ErrInvalidInstanceState):
		code, status = "INSTANCE_MARKET_UNAVAILABLE", fiber.StatusConflict
	case errors.Is(err, store.ErrInsufficientItem):
		code, status = "MARKET_INSUFFICIENT_ITEM_QUANTITY", fiber.StatusConflict
	case errors.Is(err, store.ErrListingNotEnough), errors.Is(err, store.ErrMarketListingNotActive), errors.Is(err, pgx.ErrNoRows):
		code, status = "INSTANCE_MARKET_SOLD", fiber.StatusConflict
	case errors.Is(err, store.ErrDuplicateIdempotency):
		code, status = "MARKET_DUPLICATE_IDEMPOTENCY_KEY", fiber.StatusConflict
	case errors.Is(err, store.ErrInstanceNotOwned), errors.Is(err, store.ErrMarketListingForbidden):
		code, status = "INSTANCE_MARKET_FORBIDDEN", fiber.StatusForbidden
	case errors.Is(err, store.ErrBadQuantity), errors.Is(err, store.ErrInstanceMarketRequest), errors.Is(err, market.ErrInvalidItem):
		code, status = "INSTANCE_MARKET_INVALID", fiber.StatusBadRequest
	case errors.Is(err, market.ErrPriceBelowRealValue):
		code, status = "MARKET_PRICE_BELOW_REAL_VALUE", fiber.StatusBadRequest
	case errors.Is(err, market.ErrPriceStepInvalid):
		code, status = "MARKET_PRICE_STEP_INVALID", fiber.StatusBadRequest
	}
	return s.writeAPIError(c, status, code, "instance market request failed", "")
}
func (s *Server) handleInstanceMarketQuote(c *fiber.Ctx) error {
	user, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var req struct {
		Items []store.InstanceMarketSelection `json:"items"`
	}
	if err = decodeInstanceMarketBody(c, &req); err != nil || len(req.Items) == 0 || len(req.Items) > store.InstanceMarketBatchLimit {
		return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
	}
	items, err := s.deps.Store.QuoteInstanceMarket(c.Context(), user, req.Items)
	if err != nil {
		return s.instanceMarketError(c, err)
	}
	return s.writeAPISuccess(c, 200, fiber.Map{"items": items})
}
func (s *Server) handleInstanceMarketList(c *fiber.Ctx) error {
	user, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var req struct {
		ItemID string                          `json:"itemId"`
		Price  int64                           `json:"price"`
		Items  []store.InstanceMarketSelection `json:"items"`
	}
	if err = decodeInstanceMarketBody(c, &req); err != nil || len(req.Items) == 0 || len(req.Items) > store.InstanceMarketBatchLimit || len(req.ItemID) == 0 || len(req.ItemID) > 256 || req.Price <= 0 || req.Price > 1_000_000_000_000 {
		return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
	}
	id, err := s.deps.Market.ListInstanceVariants(c.Context(), user, req.ItemID, req.Price, req.Items, c.Get("X-Idempotency-Key"))
	if err != nil {
		return s.instanceMarketError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{user}, PlayerState: true, Inventory: true})
	return s.writeAPISuccess(c, 200, fiber.Map{"listingId": id})
}
func (s *Server) handleInstanceMarketBuy(c *fiber.Ctx) error {
	user, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var req store.InstanceMarketPurchase
	if err = decodeInstanceMarketBody(c, &req); err != nil {
		return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
	}
	result, err := s.deps.Store.BuyInstanceMarketListing(c.Context(), user, req, c.Get("X-Idempotency-Key"))
	if err != nil {
		return s.instanceMarketError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{user, result.SellerID}, PlayerState: true, Inventory: true})
	return s.writeAPISuccess(c, 200, fiber.Map{"orderId": result.OrderID, "created": result.Created})
}
func (s *Server) handleInstanceMarketCancel(c *fiber.Ctx) error {
	user, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var req struct {
		ListingID int64 `json:"listingId"`
	}
	if err = decodeInstanceMarketBody(c, &req); err != nil || req.ListingID <= 0 {
		return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
	}
	if err = s.deps.Store.CancelInstanceMarketListing(c.Context(), user, req.ListingID); err != nil {
		return s.instanceMarketError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{user}, PlayerState: true, Inventory: true})
	return s.writeAPISuccess(c, 200, fiber.Map{"ok": true})
}
func (s *Server) handleInstanceMarketRead(c *fiber.Ctx) error {
	user, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 || offset > 100000 {
		return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
	}
	const limit = 30
	switch c.Params("view") {
	case "groups":
		catalogKey := c.Query("catalogKey")
		if values := c.Query("catalogValues"); c.Context().QueryArgs().Has("catalogValues") {
			if catalogKey != "" {
				return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
			}
			catalogKey, err = s.deps.Store.InstanceMarketCatalogFilter(c.Query("itemId"), values)
			if err != nil {
				return s.instanceMarketError(c, err)
			}
		}
		rows, e := s.deps.Store.InstanceMarketGroups(c.Context(), c.Query("itemId"), catalogKey, offset, limit)
		if e != nil {
			return s.instanceMarketError(c, e)
		}
		more := len(rows) > limit
		if more {
			rows = rows[:limit]
		}
		return s.writeAPISuccess(c, 200, fiber.Map{"items": rows, "hasMore": more})
	case "listings", "mine":
		mine := int64(0)
		if c.Params("view") == "mine" {
			mine = user
		}
		rows, e := s.deps.Store.InstanceMarketListings(c.Context(), c.Query("itemId"), c.Query("groupKey"), mine, user, offset, limit)
		if e != nil {
			return s.instanceMarketError(c, e)
		}
		more := len(rows) > limit
		if more {
			rows = rows[:limit]
		}
		return s.writeAPISuccess(c, 200, fiber.Map{"items": rows, "hasMore": more})
	case "orders":
		rows, e := s.deps.Store.InstanceMarketOrders(c.Context(), user, offset, limit)
		if e != nil {
			return s.instanceMarketError(c, e)
		}
		more := len(rows) > limit
		if more {
			rows = rows[:limit]
		}
		return s.writeAPISuccess(c, 200, fiber.Map{"items": rows, "hasMore": more})
	case "details":
		id, e := strconv.ParseInt(c.Query("listingId"), 10, 64)
		if e != nil || id <= 0 {
			return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
		}
		rows, e := s.deps.Store.InstanceMarketListingDetails(c.Context(), user, id)
		if e != nil {
			return s.instanceMarketError(c, e)
		}
		return s.writeAPISuccess(c, 200, fiber.Map{"items": rows})
	default:
		return s.instanceMarketError(c, store.ErrInstanceMarketRequest)
	}
}
