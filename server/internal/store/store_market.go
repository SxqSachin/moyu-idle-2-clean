package store

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type activeMarketListing struct {
	ID       int64
	Price    int64
	Kind     string
	Quantity int64
}

type activeMarketBuyOrder struct {
	ID                int64
	Price             int64
	Quantity          int64
	RemainingQuantity int64
}

func (s *Store) loadActiveMarketListingForItemTx(ctx context.Context, tx pgx.Tx, sellerID int64, itemID string) (*activeMarketListing, error) {
	var row activeMarketListing
	err := tx.QueryRow(ctx, `
		SELECT id, price, listing_kind, quantity
		FROM market_listings
		WHERE seller_id = $1
		  AND item_id = $2
		  AND status = 'active'
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE
	`, sellerID, itemID).Scan(&row.ID, &row.Price, &row.Kind, &row.Quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) loadActiveMarketBuyOrderForItemTx(ctx context.Context, tx pgx.Tx, buyerID int64, itemID string) (*activeMarketBuyOrder, error) {
	var row activeMarketBuyOrder
	err := tx.QueryRow(ctx, `
		SELECT id, price, quantity, remaining_quantity
		FROM market_buy_orders
		WHERE buyer_id = $1
		  AND item_id = $2
		  AND status = 'active'
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE
	`, buyerID, itemID).Scan(&row.ID, &row.Price, &row.Quantity, &row.RemainingQuantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) CreateListing(ctx context.Context, sellerID int64, itemID string, price int64, quantity int64) (int64, error) {
	itemID = strings.TrimSpace(itemID)
	if item := s.gameData.ItemByID(itemID); item != nil && item.MarketPolicy != nil {
		return 0, ErrMarketItemNotListable
	}
	if quantity <= 0 {
		return 0, ErrBadQuantity
	}
	if !s.TemplateItemMarketListable(itemID) {
		return 0, ErrMarketItemNotListable
	}
	var id int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		existingBuyOrder, err := s.loadActiveMarketBuyOrderForItemTx(ctx, tx, sellerID, itemID)
		if err != nil {
			return err
		}
		if existingBuyOrder != nil {
			return ErrMarketItemOrderSideConflict
		}
		existingListing, err := s.loadActiveMarketListingForItemTx(ctx, tx, sellerID, itemID)
		if err != nil {
			return err
		}
		if existingListing != nil {
			if existingListing.Price != price {
				return ErrMarketItemOrderPriceConflict
			}
			if existingListing.Kind != "stack" {
				return ErrMarketItemOrderAlreadyExists
			}
		}
		owned, err := s.ConsumableQuantityByTemplateTx(ctx, tx, sellerID, itemID)
		if err != nil {
			return err
		}
		if owned < quantity {
			return ErrInsufficientItem
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, sellerID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "market_listing",
			Details: map[string]any{
				"type": "market_list",
			},
		}); err != nil {
			return err
		}
		if existingListing != nil {
			if existingListing.Quantity > math.MaxInt64-quantity {
				return ErrInventoryQuantityOverflow
			}
			mergedQuantity := existingListing.Quantity + quantity
			if _, err := tx.Exec(ctx, `
				UPDATE market_listings
				SET quantity = $2
				WHERE id = $1
			`, existingListing.ID, mergedQuantity); err != nil {
				return err
			}
			id = existingListing.ID
			return nil
		}
		return tx.QueryRow(ctx, `
			INSERT INTO market_listings(seller_id, item_id, price, quantity, status, created_at)
			VALUES($1, $2, $3, $4, 'active', now())
			RETURNING id
		`, sellerID, itemID, price, quantity).Scan(&id)
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

func hasInstanceSpecialState(enhanceLevel int, affixRaw []byte) bool {
	return enhanceLevel > 0 || !affixesJSONEmptyRaw(affixRaw)
}

func (s *Store) InstanceTemplateItemID(ctx context.Context, instanceID int64) (string, error) {
	var itemID string
	if err := s.db.QueryRow(ctx, `
		SELECT template_item_id
		FROM player_item_instances
		WHERE instance_id = $1
	`, instanceID).Scan(&itemID); err != nil {
		return "", err
	}
	return itemID, nil
}

func (s *Store) CreateInstanceListing(ctx context.Context, sellerID, instanceID, price int64, allowSpecial bool) (int64, error) {
	return 0, ErrMarketItemMustAuction
}

func (s *Store) CancelListing(ctx context.Context, sellerID, listingID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var itemID string
		var listingKind string
		var instanceID *int64
		var quantity int64
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT seller_id, item_id, listing_kind, instance_id, quantity, status
			FROM market_listings
			WHERE id = $1
			FOR UPDATE
		`, listingID).Scan(&ownerID, &itemID, &listingKind, &instanceID, &quantity, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrMarketListingNotFound
			}
			return err
		}
		if ownerID != sellerID {
			return ErrMarketListingForbidden
		}
		if status != "active" || quantity <= 0 {
			return ErrMarketListingNotActive
		}

		if listingKind == "instance" {
			if instanceID == nil {
				return ErrInvalidInstanceState
			}
			if err := s.LedgerSetStateTx(ctx, tx, *instanceID, sellerID, "market", "inventory", itemID, 1, InventoryLedgerMeta{
				EventType:     WarehouseChangeEventSystemToWarehouse,
				SourceSurface: "market_listing",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_listing_cancel",
					"listingId": listingID,
				},
			}); err != nil {
				return err
			}
		} else {
			if err := s.LedgerGrantTemplateTx(ctx, tx, sellerID, itemID, quantity, InventoryLedgerMeta{
				SourceSurface: "market_listing",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_listing_cancel",
					"listingId": listingID,
				},
			}); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE market_listings
			SET status = 'cancelled'
			WHERE id = $1
		`, listingID)
		return err
	})
}

func (s *Store) BuyListing(ctx context.Context, buyerID, listingID, quantity int64, idempotencyKey string) (BuyListingResult, error) {
	if quantity <= 0 {
		return BuyListingResult{}, ErrBadQuantity
	}
	out := BuyListingResult{}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if idempotencyKey != "" {
			var existingOrderID int64
			err := tx.QueryRow(ctx, `
				SELECT id
				FROM market_orders
				WHERE buyer_id = $1 AND idempotency_key = $2
				LIMIT 1
			`, buyerID, idempotencyKey).Scan(&existingOrderID)
			if err == nil {
				out = BuyListingResult{OrderID: existingOrderID, Created: false}
				return nil
			}
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}

		var sellerID, price, available int64
		var itemID string
		var listingKind string
		var instanceID *int64
		if err := tx.QueryRow(ctx, `
			SELECT seller_id, item_id, price, quantity, listing_kind, instance_id
			FROM market_listings
			WHERE id = $1 AND status = 'active'
			FOR UPDATE
		`, listingID).Scan(&sellerID, &itemID, &price, &available, &listingKind, &instanceID); err != nil {
			return err
		}
		if !s.TemplateItemMarketListable(itemID) {
			return ErrMarketItemNotListable
		}
		if available < quantity {
			return ErrListingNotEnough
		}
		if listingKind != "stack" {
			return ErrMarketItemMustAuction
		}
		total, err := checkedMulInt64(price, quantity)
		if err != nil {
			return err
		}
		feeRateBps, err := s.queryMarketFeeTierBpsTx(ctx, tx, itemID, total)
		if err != nil {
			return err
		}
		if feeRateBps < 0 {
			feeRateBps = 0
		}
		feeAmount := (total * feeRateBps) / 10_000
		sellerAmount := total - feeAmount
		if sellerAmount < 0 {
			sellerAmount = 0
		}

		buyerGold, err := s.ConsumableQuantityByTemplateTx(ctx, tx, buyerID, GoldCoinItemID)
		if err != nil {
			return err
		}
		if buyerGold < total {
			return ErrInsufficientItem
		}

		if err = s.LedgerConsumeTemplateTx(ctx, tx, buyerID, GoldCoinItemID, total, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "market_trade",
			Details: map[string]any{
				"type":      "market_buy_cost",
				"listingId": listingID,
			},
		}); err != nil {
			return err
		}
		if sellerAmount > 0 {
			if err = s.LedgerGrantTemplateTx(ctx, tx, sellerID, GoldCoinItemID, sellerAmount, InventoryLedgerMeta{
				SourceSurface: "market_trade",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_buy_income",
					"listingId": listingID,
				},
			}); err != nil {
				return err
			}
		}
		if listingKind == "instance" {
			if instanceID == nil {
				return ErrInvalidInstanceState
			}
			if err = s.LedgerTransferOwnershipAndStateTx(ctx, tx, *instanceID, sellerID, buyerID, "market", "inventory", itemID, 1, InventoryLedgerMeta{
				SourceSurface: "market_trade",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_buy_instance_gain",
					"listingId": listingID,
				},
			}); err != nil {
				return err
			}
		} else {
			if err = s.LedgerGrantTemplateTx(ctx, tx, buyerID, itemID, quantity, InventoryLedgerMeta{
				SourceSurface: "market_trade",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_buy_gain",
					"listingId": listingID,
				},
			}); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `
			UPDATE market_listings
			SET quantity = quantity - $1,
			    status = CASE WHEN quantity - $1 = 0 THEN 'sold' ELSE status END
			WHERE id = $2
		`, quantity, listingID); err != nil {
			return err
		}

		var orderID int64
		if err = tx.QueryRow(ctx, `
			INSERT INTO market_orders(listing_id, buyer_id, total_price, fee_rate_bps, fee_amount, seller_amount, status, idempotency_key, created_at)
			VALUES($1, $2, $3, $4, $5, $6, 'done', $7, now())
			RETURNING id
		`, listingID, buyerID, total, feeRateBps, feeAmount, sellerAmount, idempotencyKey).Scan(&orderID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrDuplicateIdempotency
			}
			return err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO market_ledger(order_id, from_user, to_user, amount, created_at)
			VALUES($1, $2, $3, $4, now())
		`, orderID, buyerID, sellerID, sellerAmount); err != nil {
			return err
		}
		if err = s.recordRiskTransferTx(ctx, tx, RiskTransferRecord{
			FromUserID:  buyerID,
			ToUserID:    sellerID,
			Channel:     "market_listing",
			ReferenceID: orderID,
			ItemID:      itemID,
			Quantity:    quantity,
			UnitPrice:   price,
			TotalPrice:  total,
			FeeAmount:   feeAmount,
			NetAmount:   sellerAmount,
			PriceAnchor: s.ItemEconomyBaseValue(itemID),
			Extra: map[string]any{
				"listingId":   listingID,
				"listingKind": listingKind,
			},
		}); err != nil {
			return err
		}
		out = BuyListingResult{
			OrderID:      orderID,
			Created:      true,
			FeeRateBps:   feeRateBps,
			FeeAmount:    feeAmount,
			SellerAmount: sellerAmount,
			SellerID:     sellerID,
			BuyerID:      buyerID,
		}
		return nil
	})
	if err != nil {
		return BuyListingResult{}, err
	}
	return out, nil
}

func (s *Store) queryMarketFeeTierBpsTx(ctx context.Context, tx pgx.Tx, _ string, total int64) (int64, error) {
	var bps int64
	err := tx.QueryRow(ctx, `
		SELECT fee_bps
		FROM market_fee_tiers
		WHERE active = TRUE
		  AND scope = 'default'
		  AND min_total <= $1
		  AND (max_total IS NULL OR $1 <= max_total)
		ORDER BY priority DESC, min_total DESC
		LIMIT 1
	`, total).Scan(&bps)
	if errors.Is(err, pgx.ErrNoRows) {
		return 500, nil
	}
	return bps, err
}

func clampMarketListLimit(limit int) int {
	if limit <= 0 {
		return 30
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (s *Store) ListMarketListings(ctx context.Context, itemID string, includeSold bool, limit int) ([]MarketListing, error) {
	limit = clampMarketListLimit(limit)
	itemID = strings.TrimSpace(itemID)
	allowedItemIDs := s.marketTemplateItemIDs()
	if len(allowedItemIDs) == 0 {
		return []MarketListing{}, nil
	}
	var rows pgx.Rows
	var err error
	if !includeSold {
		// Active-only path: ORDER BY price/id lets the planner use
		// idx_market_listings_item_active(item_id, price, id) WHERE status='active'.
		rows, err = s.db.Query(ctx, `
			SELECT id, seller_id, item_id, listing_kind, instance_id, price, quantity, status, created_at
			FROM market_listings
			WHERE ($1 = '' OR item_id = $1)
			  AND status = 'active'
			  AND item_id = ANY($3)
			ORDER BY price ASC, id ASC
			LIMIT $2
		`, itemID, limit, allowedItemIDs)
	} else {
		// Include-sold path: mixed statuses, keep CASE ordering so active rows surface first.
		rows, err = s.db.Query(ctx, `
			SELECT id, seller_id, item_id, listing_kind, instance_id, price, quantity, status, created_at
			FROM market_listings
			WHERE ($1 = '' OR item_id = $1)
			  AND item_id = ANY($3)
			ORDER BY
			  CASE WHEN status = 'active' THEN 0 ELSE 1 END,
			  CASE WHEN status = 'active' THEN price END ASC,
			  CASE WHEN status = 'active' THEN id END ASC,
			  id DESC
			LIMIT $2
		`, itemID, limit, allowedItemIDs)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]MarketListing, 0, limit)
	for rows.Next() {
		var listing MarketListing
		if err := rows.Scan(
			&listing.ID,
			&listing.SellerID,
			&listing.ItemID,
			&listing.ListingKind,
			&listing.InstanceID,
			&listing.Price,
			&listing.Quantity,
			&listing.Status,
			&listing.CreatedAt,
		); err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}
	return listings, rows.Err()
}

func (s *Store) ListMyMarketListings(ctx context.Context, userID int64, includeSold bool, limit int) ([]MarketListing, error) {
	limit = clampMarketListLimit(limit)
	allowedItemIDs := s.marketTemplateItemIDs()
	if len(allowedItemIDs) == 0 {
		return []MarketListing{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, seller_id, item_id, listing_kind, instance_id, price, quantity, status, created_at
		FROM market_listings
		WHERE seller_id = $1
		  AND ($2 = TRUE OR status = 'active')
		  AND item_id = ANY($4)
		ORDER BY id DESC
		LIMIT $3
	`, userID, includeSold, limit, allowedItemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]MarketListing, 0, limit)
	for rows.Next() {
		var listing MarketListing
		if err := rows.Scan(
			&listing.ID,
			&listing.SellerID,
			&listing.ItemID,
			&listing.ListingKind,
			&listing.InstanceID,
			&listing.Price,
			&listing.Quantity,
			&listing.Status,
			&listing.CreatedAt,
		); err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}
	return listings, rows.Err()
}

func (s *Store) CreateBuyOrder(ctx context.Context, buyerID int64, itemID string, price int64, quantity int64) (int64, error) {
	if item := s.gameData.ItemByID(strings.TrimSpace(itemID)); item != nil && item.MarketPolicy != nil {
		return 0, ErrMarketItemNotListable
	}
	itemID = strings.TrimSpace(itemID)
	if price <= 0 || quantity <= 0 {
		return 0, ErrBadQuantity
	}
	if !s.TemplateItemMarketListable(itemID) {
		return 0, ErrMarketItemNotListable
	}
	total, err := checkedMulInt64(price, quantity)
	if err != nil {
		return 0, err
	}
	var orderID int64
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		existingListing, err := s.loadActiveMarketListingForItemTx(ctx, tx, buyerID, itemID)
		if err != nil {
			return err
		}
		if existingListing != nil {
			return ErrMarketItemOrderSideConflict
		}
		existingBuyOrder, err := s.loadActiveMarketBuyOrderForItemTx(ctx, tx, buyerID, itemID)
		if err != nil {
			return err
		}
		if existingBuyOrder != nil && existingBuyOrder.Price != price {
			return ErrMarketItemOrderPriceConflict
		}
		buyerGold, err := s.ConsumableQuantityByTemplateTx(ctx, tx, buyerID, GoldCoinItemID)
		if err != nil {
			return err
		}
		if buyerGold < total {
			return ErrInsufficientItem
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, buyerID, GoldCoinItemID, total, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "market_buy_order",
			Details: map[string]any{
				"type": "market_buy_order_create",
			},
		}); err != nil {
			return err
		}
		if existingBuyOrder != nil {
			if existingBuyOrder.Quantity > math.MaxInt64-quantity || existingBuyOrder.RemainingQuantity > math.MaxInt64-quantity {
				return ErrInventoryQuantityOverflow
			}
			mergedQuantity := existingBuyOrder.Quantity + quantity
			mergedRemainingQuantity := existingBuyOrder.RemainingQuantity + quantity
			if _, err := tx.Exec(ctx, `
				UPDATE market_buy_orders
				SET quantity = $2,
				    remaining_quantity = $3,
				    updated_at = now()
				WHERE id = $1
			`, existingBuyOrder.ID, mergedQuantity, mergedRemainingQuantity); err != nil {
				return err
			}
			orderID = existingBuyOrder.ID
			return nil
		}
		return tx.QueryRow(ctx, `
			INSERT INTO market_buy_orders(
				buyer_id, item_id, price, quantity, remaining_quantity, status, created_at, updated_at
			)
			VALUES($1, $2, $3, $4, $4, 'active', now(), now())
			RETURNING id
		`, buyerID, itemID, price, quantity).Scan(&orderID)
	})
	if err != nil {
		return 0, err
	}
	return orderID, nil
}

func (s *Store) ListMarketBuyOrders(ctx context.Context, itemID string, includeFilled bool, limit int) ([]MarketBuyOrder, error) {
	limit = clampMarketListLimit(limit)
	itemID = strings.TrimSpace(itemID)
	allowedItemIDs := s.marketTemplateItemIDs()
	if len(allowedItemIDs) == 0 {
		return []MarketBuyOrder{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, buyer_id, item_id, price, quantity, remaining_quantity, status, created_at, updated_at
		FROM market_buy_orders
		WHERE ($1 = '' OR item_id = $1)
		  AND ($2 = TRUE OR status = 'active')
		  AND item_id = ANY($4)
		ORDER BY
		  CASE WHEN status = 'active' THEN 0 ELSE 1 END,
		  CASE WHEN status = 'active' THEN price END DESC,
		  CASE WHEN status = 'active' THEN id END ASC,
		  id DESC
		LIMIT $3
	`, itemID, includeFilled, limit, allowedItemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MarketBuyOrder, 0, limit)
	for rows.Next() {
		var row MarketBuyOrder
		if err := rows.Scan(
			&row.ID,
			&row.BuyerID,
			&row.ItemID,
			&row.Price,
			&row.Quantity,
			&row.RemainingQuantity,
			&row.Status,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListMyMarketBuyOrders(ctx context.Context, buyerID int64, includeFilled bool, limit int) ([]MarketBuyOrder, error) {
	limit = clampMarketListLimit(limit)
	allowedItemIDs := s.marketTemplateItemIDs()
	if len(allowedItemIDs) == 0 {
		return []MarketBuyOrder{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, buyer_id, item_id, price, quantity, remaining_quantity, status, created_at, updated_at
		FROM market_buy_orders
		WHERE buyer_id = $1
		  AND ($2 = TRUE OR status = 'active')
		  AND item_id = ANY($4)
		ORDER BY id DESC
		LIMIT $3
	`, buyerID, includeFilled, limit, allowedItemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MarketBuyOrder, 0, limit)
	for rows.Next() {
		var row MarketBuyOrder
		if err := rows.Scan(
			&row.ID,
			&row.BuyerID,
			&row.ItemID,
			&row.Price,
			&row.Quantity,
			&row.RemainingQuantity,
			&row.Status,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) FillBuyOrder(ctx context.Context, sellerID int64, orderID int64, quantity int64) (BuyListingResult, error) {
	if quantity <= 0 {
		return BuyListingResult{}, ErrBadQuantity
	}
	out := BuyListingResult{}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var buyerID int64
		var itemID string
		var price int64
		var remaining int64
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT buyer_id, item_id, price, remaining_quantity, status
			FROM market_buy_orders
			WHERE id = $1
			FOR UPDATE
		`, orderID).Scan(&buyerID, &itemID, &price, &remaining, &status); err != nil {
			return err
		}
		if !s.TemplateItemMarketListable(itemID) {
			return ErrMarketItemNotListable
		}
		if status != "active" || remaining < quantity {
			return ErrBuyOrderNotEnough
		}

		sellerOwned, err := s.ConsumableQuantityByTemplateTx(ctx, tx, sellerID, itemID)
		if err != nil {
			return err
		}
		if sellerOwned < quantity {
			return ErrInsufficientItem
		}

		total, err := checkedMulInt64(price, quantity)
		if err != nil {
			return err
		}
		feeRateBps, err := s.queryMarketFeeTierBpsTx(ctx, tx, itemID, total)
		if err != nil {
			return err
		}
		if feeRateBps < 0 {
			feeRateBps = 0
		}
		feeAmount := (total * feeRateBps) / 10_000
		sellerAmount := total - feeAmount
		if sellerAmount < 0 {
			sellerAmount = 0
		}

		if err = s.LedgerConsumeTemplateTx(ctx, tx, sellerID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "market_buy_order_fill",
			Details: map[string]any{
				"type":    "market_buy_order_fill_cost",
				"orderId": orderID,
			},
		}); err != nil {
			return err
		}
		if err = s.LedgerGrantTemplateTx(ctx, tx, buyerID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "market_buy_order_fill",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":    "market_buy_order_fill_gain",
				"orderId": orderID,
			},
		}); err != nil {
			return err
		}
		if sellerAmount > 0 {
			if err = s.LedgerGrantTemplateTx(ctx, tx, sellerID, GoldCoinItemID, sellerAmount, InventoryLedgerMeta{
				SourceSurface: "market_buy_order_fill",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":    "market_buy_order_fill_income",
					"orderId": orderID,
				},
			}); err != nil {
				return err
			}
		}

		if _, err = tx.Exec(ctx, `
			UPDATE market_buy_orders
			SET remaining_quantity = remaining_quantity - $1,
				status = CASE WHEN remaining_quantity - $1 <= 0 THEN 'filled' ELSE status END,
				updated_at = now()
			WHERE id = $2
		`, quantity, orderID); err != nil {
			return err
		}
		out = BuyListingResult{
			Created:      true,
			FeeRateBps:   feeRateBps,
			FeeAmount:    feeAmount,
			SellerAmount: sellerAmount,
			SellerID:     sellerID,
			BuyerID:      buyerID,
		}
		if err = s.recordRiskTransferTx(ctx, tx, RiskTransferRecord{
			FromUserID:  buyerID,
			ToUserID:    sellerID,
			Channel:     "market_buy_order",
			ReferenceID: orderID,
			ItemID:      itemID,
			Quantity:    quantity,
			UnitPrice:   price,
			TotalPrice:  total,
			FeeAmount:   feeAmount,
			NetAmount:   sellerAmount,
			PriceAnchor: s.ItemEconomyBaseValue(itemID),
			Extra: map[string]any{
				"orderId": orderID,
			},
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return BuyListingResult{}, err
	}
	return out, nil
}

func (s *Store) CancelBuyOrder(ctx context.Context, buyerID int64, orderID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var price int64
		var remaining int64
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT buyer_id, price, remaining_quantity, status
			FROM market_buy_orders
			WHERE id = $1
			FOR UPDATE
		`, orderID).Scan(&ownerID, &price, &remaining, &status); err != nil {
			return err
		}
		if ownerID != buyerID {
			return ErrInstanceNotOwned
		}
		if status != "active" || remaining <= 0 {
			return ErrBuyOrderNotEnough
		}

		refund, err := checkedMulInt64(price, remaining)
		if err != nil {
			return err
		}
		if refund > 0 {
			if err := s.LedgerGrantTemplateTx(ctx, tx, buyerID, GoldCoinItemID, refund, InventoryLedgerMeta{
				SourceSurface: "market_buy_order",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":    "market_buy_order_cancel_refund",
					"orderId": orderID,
				},
			}); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE market_buy_orders
			SET remaining_quantity = 0, status = 'cancelled', updated_at = now()
			WHERE id = $1
		`, orderID)
		return err
	})
}

func clampAuctionDurationSeconds(durationSeconds int64) int64 {
	if durationSeconds <= 0 {
		return 24 * 60 * 60
	}
	if durationSeconds < 4*60*60 {
		return 4 * 60 * 60
	}
	if durationSeconds > 72*60*60 {
		return 72 * 60 * 60
	}
	return durationSeconds
}
