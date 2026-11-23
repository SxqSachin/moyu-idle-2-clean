package httpapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/market"
	"mysecondwebgame/server/internal/rules"
	"mysecondwebgame/server/internal/store"
)

type marketSnapshotResponse struct {
	PublicListings  []store.MarketListing  `json:"publicListings"`
	PublicBuyOrders []store.MarketBuyOrder `json:"publicBuyOrders"`
	MyListings      []store.MarketListing  `json:"myListings"`
	MyBuyOrders     []store.MarketBuyOrder `json:"myBuyOrders"`
}

func (s *Server) handleMarketListings(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	scope := strings.TrimSpace(strings.ToLower(c.Query("scope")))
	if scope == "" {
		scope = "public"
	}
	if scope != "public" && scope != "mine" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_LISTINGS_INVALID_SCOPE", "invalid scope", "")
	}
	includeSold := strings.EqualFold(c.Query("includeSold"), "1") || strings.EqualFold(c.Query("includeSold"), "true")
	itemID := strings.TrimSpace(c.Query("itemId"))
	limit := 30
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_LISTINGS_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}

	var listings []store.MarketListing
	if scope == "mine" {
		listings, err = s.deps.Store.ListMyMarketListings(c.Context(), userID, includeSold, limit)
	} else {
		listings, err = s.deps.Store.ListMarketListings(c.Context(), itemID, includeSold, limit)
	}
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_LISTINGS_QUERY_FAILED", "query market listings failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": listings})
}

func (s *Server) handleMarketBuyOrders(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	scope := strings.TrimSpace(strings.ToLower(c.Query("scope")))
	if scope == "" {
		scope = "public"
	}
	if scope != "public" && scope != "mine" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDERS_INVALID_SCOPE", "invalid scope", "")
	}
	includeFilled := strings.EqualFold(c.Query("includeFilled"), "1") || strings.EqualFold(c.Query("includeFilled"), "true")
	itemID := strings.TrimSpace(c.Query("itemId"))
	limit := 30
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDERS_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}

	var rows []store.MarketBuyOrder
	if scope == "mine" {
		rows, err = s.deps.Store.ListMyMarketBuyOrders(c.Context(), userID, includeFilled, limit)
	} else {
		rows, err = s.deps.Store.ListMarketBuyOrders(c.Context(), itemID, includeFilled, limit)
	}
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_BUY_ORDERS_QUERY_FAILED", "query buy orders failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows})
}

func (s *Server) handleMarketSnapshot(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	itemID := strings.TrimSpace(c.Query("itemId"))

	myListings, err := s.deps.Store.ListMyMarketListings(c.Context(), userID, true, 40)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_SNAPSHOT_MY_LISTINGS_FAILED", "query my market listings failed", err.Error())
	}
	myBuyOrders, err := s.deps.Store.ListMyMarketBuyOrders(c.Context(), userID, true, 40)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_SNAPSHOT_MY_BUY_ORDERS_FAILED", "query my buy orders failed", err.Error())
	}

	publicListings := []store.MarketListing{}
	publicBuyOrders := []store.MarketBuyOrder{}
	if itemID != "" {
		publicListings, err = s.deps.Store.ListMarketListings(c.Context(), itemID, false, 80)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_SNAPSHOT_PUBLIC_LISTINGS_FAILED", "query market listings failed", err.Error())
		}
		publicBuyOrders, err = s.deps.Store.ListMarketBuyOrders(c.Context(), itemID, false, 80)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_SNAPSHOT_PUBLIC_BUY_ORDERS_FAILED", "query buy orders failed", err.Error())
		}
	}

	return s.writeAPISuccess(c, fiber.StatusOK, marketSnapshotResponse{
		PublicListings:  publicListings,
		PublicBuyOrders: publicBuyOrders,
		MyListings:      myListings,
		MyBuyOrders:     myBuyOrders,
	})
}

func (s *Server) handleMarketCreateBuyOrder(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketBuyOrderRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.ItemID = strings.TrimSpace(req.ItemID)
	if req.ItemID == "" || req.Price <= 0 || req.Quantity <= 0 || req.Quantity > 1_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_INVALID_REQUEST", "invalid request", "")
	}
	orderID, err := s.deps.Market.CreateBuyOrder(c.Context(), userID, req.ItemID, req.Price, req.Quantity)
	if err != nil {
		if errors.Is(err, market.ErrInvalidItem) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_INVALID_ITEM", "invalid item id", "")
		}
		if errors.Is(err, market.ErrItemNotListable) || errors.Is(err, store.ErrMarketItemNotListable) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_NOT_LISTABLE", "item cannot be listed on market", "")
		}
		if errors.Is(err, market.ErrPriceBelowRealValue) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_PRICE_BELOW_REAL_VALUE", "price below real value", "")
		}
		if errors.Is(err, market.ErrPriceStepInvalid) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_PRICE_STEP_INVALID", "price does not match market step rule", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) || errors.Is(err, store.ErrInsufficientGold) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_BUY_ORDER_INSUFFICIENT_GOLD", "insufficient gold", "")
		}
		if errors.Is(err, store.ErrMarketItemOrderSideConflict) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_ORDER_SIDE_CONFLICT", "cannot create buy order when active listing exists for this item", "")
		}
		if errors.Is(err, store.ErrMarketItemOrderPriceConflict) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_ORDER_PRICE_CONFLICT", "active buy order has different price for this item", "")
		}
		if errors.Is(err, store.ErrMarketItemOrderAlreadyExists) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_ORDER_ALREADY_EXISTS", "active item order already exists", "")
		}
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_INVALID_REQUEST", "invalid request", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_BUY_ORDER_CREATE_FAILED", "create buy order failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"orderId": orderID})
}

func (s *Server) handleMarketFillBuyOrder(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketFillBuyOrderRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_FILL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.OrderID <= 0 || req.Quantity <= 0 || req.Quantity > 1_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_FILL_INVALID_REQUEST", "invalid request", "")
	}
	result, err := s.deps.Store.FillBuyOrder(c.Context(), userID, req.OrderID, req.Quantity)
	if err != nil {
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_BUY_ORDER_FILL_INSUFFICIENT_ITEM", "insufficient item quantity", "")
		}
		if errors.Is(err, store.ErrBuyOrderNotEnough) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_BUY_ORDER_FILL_NOT_ENOUGH", "buy order quantity not enough", "")
		}
		if errors.Is(err, store.ErrMarketItemNotListable) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_NOT_LISTABLE", "item cannot be traded on market", "")
		}
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_FILL_INVALID_REQUEST", "invalid request", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_BUY_ORDER_FILL_FAILED", "fill buy order failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID, result.BuyerID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":           true,
		"feeRateBps":   result.FeeRateBps,
		"feeAmount":    result.FeeAmount,
		"sellerAmount": result.SellerAmount,
	})
}

func (s *Server) handleMarketCancelBuyOrder(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketCancelBuyOrderRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_CANCEL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.OrderID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_BUY_ORDER_CANCEL_INVALID_REQUEST", "invalid request", "")
	}
	if err = s.deps.Store.CancelBuyOrder(c.Context(), userID, req.OrderID); err != nil {
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "MARKET_BUY_ORDER_CANCEL_FORBIDDEN", "buy order not owned", "")
		}
		if errors.Is(err, store.ErrBuyOrderNotEnough) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_BUY_ORDER_CANCEL_INVALID_STATE", "buy order is not active", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_BUY_ORDER_CANCEL_FAILED", "cancel buy order failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

type auctionCreateRequest struct {
	InstanceID      int64 `json:"instanceId"`
	StartPrice      int64 `json:"startPrice"`
	BuyoutPrice     int64 `json:"buyoutPrice"`
	MinBidIncrement int64 `json:"minBidIncrement"`
	DurationSeconds int64 `json:"durationSeconds"`
}

type auctionBidRequest struct {
	AuctionID int64 `json:"auctionId"`
	BidAmount int64 `json:"bidAmount"`
}

type auctionCancelRequest struct {
	AuctionID int64 `json:"auctionId"`
}

type marketCancelListingRequest struct {
	ListingID int64 `json:"listingId"`
}

func (s *Server) handleAuctionListings(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	scope := strings.TrimSpace(strings.ToLower(c.Query("scope")))
	if scope == "" {
		scope = "public"
	}
	if scope != "public" && scope != "mine" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_LISTINGS_INVALID_SCOPE", "invalid scope", "")
	}
	includeEnded := strings.EqualFold(c.Query("includeEnded"), "1") || strings.EqualFold(c.Query("includeEnded"), "true")
	itemID := strings.TrimSpace(c.Query("itemId"))
	limit := 30
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_LISTINGS_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}

	var rows []store.AuctionListing
	if scope == "mine" {
		rows, err = s.deps.Store.ListMyAuctions(c.Context(), userID, includeEnded, limit)
	} else {
		rows, err = s.deps.Store.ListAuctions(c.Context(), itemID, includeEnded, limit)
	}
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUCTION_LISTINGS_QUERY_FAILED", "query auction listings failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows})
}

func (s *Server) handleAuctionDetail(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	auctionID, err := strconv.ParseInt(strings.TrimSpace(c.Query("auctionId")), 10, 64)
	if err != nil || auctionID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_DETAIL_INVALID_AUCTION_ID", "invalid auction id", "")
	}
	row, err := s.deps.Store.GetAuctionByID(c.Context(), auctionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "AUCTION_NOT_FOUND", "auction not found", "")
		}
		if errors.Is(err, store.ErrAuctionNotActive) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_NOT_ACTIVE", "auction is not active", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUCTION_DETAIL_QUERY_FAILED", "query auction detail failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"item": row})
}

func (s *Server) handleAuctionBids(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	auctionID, err := strconv.ParseInt(strings.TrimSpace(c.Query("auctionId")), 10, 64)
	if err != nil || auctionID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_BIDS_INVALID_AUCTION_ID", "invalid auction id", "")
	}
	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_BIDS_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	rows, err := s.deps.Store.ListAuctionBids(c.Context(), auctionID, limit)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUCTION_BIDS_QUERY_FAILED", "query auction bids failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows})
}

func (s *Server) handleAuctionListInstance(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req auctionCreateRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.StartPrice <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_INVALID_REQUEST", "invalid request", "")
	}
	if req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_STACK_ITEM_NOT_ALLOWED", "stack auction item not allowed", "")
	}
	if req.BuyoutPrice > 0 && req.BuyoutPrice < req.StartPrice {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_INVALID_REQUEST", "invalid request", "")
	}
	auctionID, err := s.deps.Market.CreateAuction(
		c.Context(),
		userID,
		req.InstanceID,
		req.StartPrice,
		req.BuyoutPrice,
		req.MinBidIncrement,
		req.DurationSeconds,
	)
	if err != nil {
		if errors.Is(err, market.ErrInvalidItem) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_INVALID_ITEM_ID", "invalid item id", "")
		}
		if errors.Is(err, market.ErrItemNotListable) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_ITEM_NOT_LISTABLE", "item cannot be listed on auction", "")
		}
		if errors.Is(err, store.ErrInstanceNotOwned) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AUCTION_INSTANCE_NOT_OWNED", "instance not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_INSTANCE_STATE_INVALID", "instance state invalid for auction", "")
		}
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_INVALID_REQUEST", "invalid request", "")
		}
		if errors.Is(err, store.ErrAuctionSellerHasActiveListing) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_SELLER_HAS_ACTIVE_AUCTION", "seller already has an active auction", "")
		}
		if errors.Is(err, store.ErrAuctionStartPriceTooLow) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_START_PRICE_TOO_LOW", "start price too low", "")
		}
		if errors.Is(err, store.ErrAuctionBuyoutTooLow) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_BUYOUT_PRICE_TOO_LOW", "buyout price too low", "")
		}
		if errors.Is(err, store.ErrAuctionMinBidIncrementInvalid) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_MIN_BID_INCREMENT_INVALID", "min bid increment invalid", "")
		}
		if errors.Is(err, store.ErrAuctionStackItemNotAllowed) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_STACK_ITEM_NOT_ALLOWED", "stack auction item not allowed", "")
		}
		if errors.Is(err, store.ErrAuctionReputationTooLow) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AUCTION_REPUTATION_TOO_LOW", "cat village reputation level too low", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUCTION_CREATE_FAILED", "create auction failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"auctionId": auctionID})
}

func (s *Server) handleAuctionBid(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req auctionBidRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_BID_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.AuctionID <= 0 || req.BidAmount <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_BID_INVALID_REQUEST", "invalid request", "")
	}
	bidResult, err := s.deps.Store.BidAuction(c.Context(), userID, req.AuctionID, req.BidAmount)
	if err != nil {
		if errors.Is(err, store.ErrInsufficientItem) || errors.Is(err, store.ErrInsufficientGold) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_BID_INSUFFICIENT_GOLD", "insufficient gold", "")
		}
		if errors.Is(err, store.ErrAuctionBidTooLow) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_BID_TOO_LOW", "bid amount too low", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_NOT_ACTIVE", "auction is not active", "")
		}
		if errors.Is(err, store.ErrAuctionNotActive) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_NOT_ACTIVE", "auction is not active", "")
		}
		if errors.Is(err, store.ErrAuctionForbidden) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AUCTION_BID_FORBIDDEN", "forbidden bid operation", "")
		}
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_BID_INVALID_REQUEST", "invalid request", "")
		}
		if errors.Is(err, store.ErrAuctionReputationTooLow) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AUCTION_REPUTATION_TOO_LOW", "cat village reputation level too low", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUCTION_BID_FAILED", "bid auction failed", err.Error())
	}
	participantIDs := []int64{userID, bidResult.SellerID}
	if bidResult.PreviousBidderID != nil {
		participantIDs = append(participantIDs, *bidResult.PreviousBidderID)
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     participantIDs,
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleAuctionCancel(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req auctionCancelRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_CANCEL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.AuctionID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUCTION_CANCEL_INVALID_REQUEST", "invalid request", "")
	}
	if err := s.deps.Store.CancelAuction(c.Context(), userID, req.AuctionID); err != nil {
		if errors.Is(err, store.ErrAuctionForbidden) {
			return s.writeAPIError(c, fiber.StatusForbidden, "AUCTION_CANCEL_FORBIDDEN", "auction not owned", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_NOT_ACTIVE", "auction is not active", "")
		}
		if errors.Is(err, store.ErrAuctionNotActive) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_NOT_ACTIVE", "auction is not active", "")
		}
		if errors.Is(err, store.ErrAuctionHasBid) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_CANCEL_HAS_BID", "cannot cancel auction with existing bids", "")
		}
		if errors.Is(err, store.ErrAuctionCancelFeeInsufficientGold) {
			return s.writeAPIError(c, fiber.StatusConflict, "AUCTION_CANCEL_FEE_INSUFFICIENT_GOLD", "insufficient gold for cancel fee", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUCTION_CANCEL_FAILED", "cancel auction failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleMarketList(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketListRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.Price <= 0 || req.Price > 1_000_000_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_PRICE", "invalid price", "")
	}
	if req.Quantity <= 0 || req.Quantity > 1_000_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_QUANTITY", "invalid quantity", "")
	}
	id, err := s.deps.Market.List(c.Context(), userID, req.ItemID, req.Price, req.Quantity)
	if err != nil {
		if errors.Is(err, market.ErrInvalidItem) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_ITEM_ID", "invalid item id", "")
		}
		if errors.Is(err, market.ErrItemNotListable) || errors.Is(err, store.ErrMarketItemNotListable) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_ITEM_NOT_LISTABLE", "item cannot be listed on market", "")
		}
		if errors.Is(err, market.ErrPriceBelowRealValue) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_PRICE_BELOW_REAL_VALUE", "price below real value", "")
		}
		if errors.Is(err, market.ErrPriceStepInvalid) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_PRICE_STEP_INVALID", "price does not match market step rule", "")
		}
		if errors.Is(err, store.ErrInsufficientItem) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_INSUFFICIENT_ITEM_QUANTITY", "insufficient item quantity", "")
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
		if errors.Is(err, store.ErrBadQuantity) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_INVALID_QUANTITY", "invalid quantity", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_LIST_FAILED", "list failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"listingId": id})
}

func (s *Server) handleMarketCancelListing(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req marketCancelListingRequest
	if err = c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_LISTING_CANCEL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.ListingID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "MARKET_LISTING_CANCEL_INVALID_REQUEST", "invalid request", "")
	}
	if err = s.deps.Market.CancelListing(c.Context(), userID, req.ListingID); err != nil {
		if errors.Is(err, store.ErrMarketListingNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "MARKET_LISTING_CANCEL_NOT_FOUND", "listing not found", "")
		}
		if errors.Is(err, store.ErrMarketListingForbidden) {
			return s.writeAPIError(c, fiber.StatusForbidden, "MARKET_LISTING_CANCEL_FORBIDDEN", "listing not owned", "")
		}
		if errors.Is(err, store.ErrMarketListingNotActive) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_LISTING_CANCEL_INVALID_STATE", "listing is not active", "")
		}
		if errors.Is(err, store.ErrInvalidInstanceState) {
			return s.writeAPIError(c, fiber.StatusConflict, "MARKET_LISTING_CANCEL_INVALID_INSTANCE_STATE", "listing instance state invalid", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "MARKET_LISTING_CANCEL_FAILED", "cancel listing failed", err.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
		Inventory:   true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

type marketBuyRequest struct {
	ListingID int64 `json:"listingId"`
	Quantity  int64 `json:"quantity"`
}

type shopTradeRequest struct {
	OfferID  int64  `json:"offerId"`
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

type ruleEvaluateRequest struct {
	Groups  []rules.ConditionGroup `json:"groups"`
	Context rules.EvalContext      `json:"context"`
}

type ruleConfigUpsertRequest struct {
	RuleKind string                 `json:"ruleKind"`
	SlotKey  string                 `json:"slotKey"`
	Enabled  bool                   `json:"enabled"`
	Groups   []rules.ConditionGroup `json:"groups"`
}

type combatConfigUpdateRequest struct {
	SkillSlots []string `json:"skillSlots"`
}

type ownedSkillMutationRequest struct {
	SkillID string `json:"skillId"`
}

type battleItemBagMutationRequest struct {
	ItemID   string `json:"itemId"`
	SlotNo   int    `json:"slotNo"`
	Quantity int64  `json:"quantity"`
}

type battleItemBagTakeBackRequest struct {
	SlotNo int `json:"slotNo"`
}

type repairRequest struct {
	InstanceID int64 `json:"instanceId"`
}

type detachAffixRequest struct {
	InstanceID        int64  `json:"instanceId"`
	SlotIndex         int    `json:"slotIndex"`
	ToolItemID        string `json:"toolItemId"`
	CarrierInstanceID int64  `json:"carrierInstanceId"`
}

type attachAffixRequest struct {
	EquipmentInstanceID int64 `json:"equipmentInstanceId"`
	AffixInstanceID     int64 `json:"affixInstanceId"`
	SlotIndex           int   `json:"slotIndex"`
}

type enchantAffixRequest struct {
	EquipmentInstanceID int64  `json:"equipmentInstanceId"`
	SlotIndex           int    `json:"slotIndex"`
	ToolItemID          string `json:"toolItemId"`
}

type smeltAffixRequest struct {
	CarrierInstanceIDs []int64 `json:"carrierInstanceIds"`
}

type equipRequest struct {
	InstanceID int64  `json:"instanceId"`
	Slot       string `json:"slot"`
	EquipType  string `json:"equipType"`
}

type unequipRequest struct {
	Slot      string `json:"slot"`
	EquipType string `json:"equipType"`
}

type unequipInstanceRequest struct {
	InstanceID int64 `json:"instanceId"`
}

type useSkillbookRequest struct {
	InstanceID    int64  `json:"instanceId"`
	TargetType    string `json:"targetType"`
	TargetKittyID int64  `json:"targetKittyId"`
	Count         int64  `json:"count"`
}

type attributeProgress struct {
	Value   float64 `json:"value"`
	Level   int     `json:"level"`
	Exp     float64 `json:"exp"`
	NextExp float64 `json:"nextExp"`
}
