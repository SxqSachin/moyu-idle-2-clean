package store

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
)

func checkedMulInt64(a, b int64) (int64, error) {
	if a <= 0 || b <= 0 {
		return 0, ErrBadQuantity
	}
	if a > math.MaxInt64/b {
		return 0, ErrInventoryQuantityOverflow
	}
	return a * b, nil
}

func (s *Store) ListShopOffers(ctx context.Context) ([]ShopOffer, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, item_id, shop_buy_price, shop_sell_price, active
		FROM shop_offers
		WHERE active = TRUE
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ShopOffer, 0, 16)
	for rows.Next() {
		var offer ShopOffer
		if err := rows.Scan(&offer.ID, &offer.ItemID, &offer.ShopBuyPrice, &offer.ShopSellPrice, &offer.Active); err != nil {
			return nil, err
		}
		out = append(out, offer)
	}
	return out, rows.Err()
}

func (s *Store) BuyFromShop(ctx context.Context, userID, offerID, quantity int64) error {
	if quantity <= 0 {
		return ErrBadQuantity
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var itemID string
		var unitPrice int64
		err := tx.QueryRow(ctx, `
			SELECT item_id, shop_sell_price
			FROM shop_offers
			WHERE id = $1 AND active = TRUE
			FOR UPDATE
		`, offerID).Scan(&itemID, &unitPrice)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrShopOfferNotFound
		}
		if err != nil {
			return err
		}
		total, err := checkedMulInt64(unitPrice, quantity)
		if err != nil {
			return err
		}

		gold, err := s.ConsumableQuantityByTemplateTx(ctx, tx, userID, GoldCoinItemID)
		if err != nil {
			return err
		}
		if gold < total {
			return ErrInsufficientItem
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, userID, GoldCoinItemID, total, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "shop_buy",
			Details: map[string]any{
				"offerId":  offerID,
				"itemId":   itemID,
				"quantity": quantity,
			},
		}); err != nil {
			return err
		}
		return s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "shop_buy",
			TargetSurface: "inventory",
			Details: map[string]any{
				"offerId": offerID,
			},
		})
	})
}

func (s *Store) SellToShop(ctx context.Context, userID int64, itemID string, quantity int64) error {
	if quantity <= 0 {
		return ErrBadQuantity
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var unitPrice int64
		err := tx.QueryRow(ctx, `
			SELECT shop_buy_price
			FROM shop_offers
			WHERE item_id = $1 AND active = TRUE
			FOR UPDATE
		`, itemID).Scan(&unitPrice)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrShopOfferNotFound
		}
		if err != nil {
			return err
		}

		owned, err := s.ConsumableQuantityByTemplateTx(ctx, tx, userID, itemID)
		if err != nil {
			return err
		}
		if owned < quantity {
			return ErrInsufficientItem
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "shop_sell",
			Details: map[string]any{
				"itemId": itemID,
			},
		}); err != nil {
			return err
		}
		total, err := checkedMulInt64(unitPrice, quantity)
		if err != nil {
			return err
		}
		return s.LedgerGrantTemplateTx(ctx, tx, userID, GoldCoinItemID, total, InventoryLedgerMeta{
			SourceSurface: "shop_sell",
			TargetSurface: "inventory",
			Details: map[string]any{
				"itemId":   itemID,
				"quantity": quantity,
			},
		})
	})
}

func (s *Store) MarketFeeSummary(ctx context.Context) (MarketFeeSummary, error) {
	var summary MarketFeeSummary
	if err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COALESCE(SUM(fee_amount), 0)::bigint,
			COALESCE(SUM(seller_amount), 0)::bigint
		FROM market_orders
	`).Scan(&summary.OrdersTotal, &summary.FeeAmountTotal, &summary.SellerAmountTotal); err != nil {
		return summary, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT
			fee_rate_bps,
			COUNT(*)::bigint AS order_count,
			COALESCE(SUM(fee_amount), 0)::bigint AS fee_amount
		FROM market_orders
		GROUP BY fee_rate_bps
		ORDER BY fee_rate_bps DESC
	`)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	tierStats := make([]MarketFeeTierStat, 0, 8)
	for rows.Next() {
		var stat MarketFeeTierStat
		if err := rows.Scan(&stat.FeeRateBps, &stat.OrderCount, &stat.FeeAmount); err != nil {
			return summary, err
		}
		tierStats = append(tierStats, stat)
	}
	if err := rows.Err(); err != nil {
		return summary, err
	}
	summary.Tiers = tierStats
	return summary, nil
}
