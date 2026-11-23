package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func auctionMinBuyoutPrice(startPrice int64) int64 {
	if startPrice <= 0 {
		return 1
	}
	// Strictly greater than startPrice * 1.5
	return (startPrice*3)/2 + 1
}

func normalizeAuctionRealValue(raw float64) int64 {
	if raw <= 0 {
		return 1
	}
	value := int64(raw)
	if value <= 0 {
		return 1
	}
	return value
}

func (s *Store) auctionFeeRateBps() int64 {
	rate := s.config.AuctionFeeRateBps
	if rate < 0 {
		return 0
	}
	if rate > 10_000 {
		return 10_000
	}
	return rate
}

func auctionMinBidIncrementBounds(startPrice int64) (int64, int64) {
	if startPrice <= 0 {
		return 1, 1
	}
	minValue := (startPrice + 9) / 10
	if minValue <= 0 {
		minValue = 1
	}
	maxValue := startPrice
	if maxValue < minValue {
		maxValue = minValue
	}
	return minValue, maxValue
}

func (s *Store) auctionMinStartPrice(itemID string, quantity int64, hasAffix bool) int64 {
	if quantity <= 0 {
		quantity = 1
	}
	unit := int64(1)
	if item := s.gameData.ItemByID(itemID); item != nil {
		unit = normalizeAuctionRealValue(item.EconomyBaseValue)
	}
	base, err := checkedMulInt64(unit, quantity)
	if err != nil || base <= 0 {
		base = 1
	}
	if hasAffix {
		withAffix, mulErr := checkedMulInt64(base, 5)
		if mulErr == nil && withAffix > 0 {
			return withAffix
		}
	}
	return base
}

func (s *Store) auctionItemListable(itemID string) bool {
	item := s.gameData.ItemByID(itemID)
	return item.AllowsAuction()
}

func (s *Store) auctionCancelFeeAmount(currentPrice int64) int64 {
	if currentPrice <= 0 {
		return 0
	}
	fee := (currentPrice * s.auctionFeeRateBps()) / 10_000
	if fee <= 0 {
		return 1
	}
	return fee
}

func (s *Store) finalizeExpiredAuctionsTx(ctx context.Context, tx pgx.Tx, now time.Time, limit int) error {
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, listing_kind, item_id, instance_id, quantity, current_bidder_id, current_price,
		       COALESCE(instance_enhance_level, 0), COALESCE(instance_affixes, '[]'::jsonb)
		FROM market_auctions
		WHERE status = 'active'
		  AND ends_at <= $1
		ORDER BY id ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	type expiredAuction struct {
		id                   int64
		sellerID             int64
		listingKind          string
		itemID               string
		instanceID           *int64
		quantity             int64
		currentBidderID      *int64
		currentPrice         int64
		instanceEnhanceLevel int
		instanceAffixesRaw   []byte
	}
	expired := make([]expiredAuction, 0, limit)
	for rows.Next() {
		var row expiredAuction
		if err := rows.Scan(
			&row.id,
			&row.sellerID,
			&row.listingKind,
			&row.itemID,
			&row.instanceID,
			&row.quantity,
			&row.currentBidderID,
			&row.currentPrice,
			&row.instanceEnhanceLevel,
			&row.instanceAffixesRaw,
		); err != nil {
			return err
		}
		if row.quantity <= 0 {
			row.quantity = 1
		}
		expired = append(expired, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	feeRateBps := s.auctionFeeRateBps()

	for _, row := range expired {
		if row.currentBidderID != nil && row.currentPrice > 0 {
			if row.listingKind == "instance" {
				if row.instanceID == nil {
					return ErrInvalidInstanceState
				}
				if err := s.LedgerTransferOwnershipAndStateTx(ctx, tx, *row.instanceID, row.sellerID, *row.currentBidderID, "auction", "inventory", row.itemID, 1, InventoryLedgerMeta{
					SourceSurface: "auction",
					TargetSurface: "inventory",
					Details: map[string]any{
						"auctionId": row.id,
						"type":      "auction_settle_item",
					},
				}); err != nil {
					return err
				}
			} else {
				if err := s.LedgerGrantTemplateTx(ctx, tx, *row.currentBidderID, row.itemID, row.quantity, InventoryLedgerMeta{
					SourceSurface: "auction",
					TargetSurface: "inventory",
					Details: map[string]any{
						"auctionId": row.id,
						"type":      "auction_settle_item_stack",
					},
				}); err != nil {
					return err
				}
			}
			feeAmount := (row.currentPrice * feeRateBps) / 10_000
			sellerAmount := row.currentPrice - feeAmount
			if sellerAmount < 0 {
				sellerAmount = 0
			}
			if sellerAmount > 0 {
				if err := s.LedgerGrantTemplateTx(ctx, tx, row.sellerID, GoldCoinItemID, sellerAmount, InventoryLedgerMeta{
					SourceSurface: "auction_bid_hold",
					TargetSurface: "inventory",
					Details: map[string]any{
						"auctionId": row.id,
						"type":      "auction_settle_gold",
					},
				}); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `
				UPDATE market_auctions
				SET status = 'sold',
				    instance_id = NULL,
				    fee_rate_bps = $2,
				    fee_amount = $3,
				    seller_amount = $4,
				    updated_at = now()
				WHERE id = $1
			`, row.id, feeRateBps, feeAmount, sellerAmount); err != nil {
				return err
			}
			unitPrice := row.currentPrice
			if row.quantity > 1 {
				unitPrice = row.currentPrice / row.quantity
			}
			settleExtra := map[string]any{"listingKind": row.listingKind}
			if row.listingKind == "instance" {
				settleExtra["enhanceLevel"] = row.instanceEnhanceLevel
				settleExtra["affixCount"] = countEquippedAffixes(row.instanceAffixesRaw)
			}
			if err := s.recordRiskTransferTx(ctx, tx, RiskTransferRecord{
				FromUserID:  *row.currentBidderID,
				ToUserID:    row.sellerID,
				Channel:     "auction_settle",
				ReferenceID: row.id,
				ItemID:      row.itemID,
				Quantity:    row.quantity,
				UnitPrice:   unitPrice,
				TotalPrice:  row.currentPrice,
				FeeAmount:   feeAmount,
				NetAmount:   sellerAmount,
				PriceAnchor: s.ItemEconomyBaseValue(row.itemID),
				Extra:       settleExtra,
			}); err != nil {
				return err
			}
			continue
		}

		if row.listingKind == "instance" {
			if row.instanceID == nil {
				return ErrInvalidInstanceState
			}
			if err := s.LedgerSetStateTx(ctx, tx, *row.instanceID, row.sellerID, "auction", "inventory", row.itemID, 1, InventoryLedgerMeta{
				EventType:     WarehouseChangeEventSystemToWarehouse,
				SourceSurface: "auction",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": row.id,
					"type":      "auction_expired_return",
				},
			}); err != nil {
				return err
			}
		} else {
			if err := s.LedgerGrantTemplateTx(ctx, tx, row.sellerID, row.itemID, row.quantity, InventoryLedgerMeta{
				SourceSurface: "auction",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": row.id,
					"type":      "auction_expired_return_stack",
				},
			}); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE market_auctions
			SET status = 'expired',
			    instance_id = NULL,
			    updated_at = now()
			WHERE id = $1
		`, row.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateAuction(
	ctx context.Context,
	sellerID int64,
	instanceID int64,
	startPrice int64,
	buyoutPrice int64,
	minBidIncrement int64,
	durationSeconds int64,
) (int64, error) {
	if instanceID <= 0 {
		return 0, ErrAuctionStackItemNotAllowed
	}
	if startPrice <= 0 {
		return 0, ErrBadQuantity
	}
	minAllowedIncrement, maxAllowedIncrement := auctionMinBidIncrementBounds(startPrice)
	if minBidIncrement < minAllowedIncrement || minBidIncrement > maxAllowedIncrement {
		return 0, ErrAuctionMinBidIncrementInvalid
	}
	if buyoutPrice > 0 && buyoutPrice < auctionMinBuyoutPrice(startPrice) {
		return 0, ErrAuctionBuyoutTooLow
	}
	durationSeconds = clampAuctionDurationSeconds(durationSeconds)

	var buyout any
	if buyoutPrice > 0 {
		buyout = buyoutPrice
	}
	var auctionID int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		now := time.Now()
		if err := s.finalizeExpiredAuctionsTx(ctx, tx, now, 200); err != nil {
			return err
		}
		if err := s.lockUserTx(ctx, tx, sellerID); err != nil {
			return err
		}
		// 猫村声望等级检查
		totalRep, err := s.GetCatVillageReputation(ctx, sellerID)
		if err != nil {
			return fmt.Errorf("get cat village reputation: %w", err)
		}
		if int64(ComputeRepLevel(totalRep, s.config.CatVillageRepLevels).Level) < s.config.AuctionMinRepLevel {
			return ErrAuctionReputationTooLow
		}
		var activeCount int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM market_auctions
			WHERE seller_id = $1
			  AND status = 'active'
		`, sellerID).Scan(&activeCount); err != nil {
			return err
		}
		if activeCount > 0 {
			return ErrAuctionSellerHasActiveListing
		}

		row, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if row.UserID != sellerID {
			return ErrInstanceNotOwned
		}
		if row.State != "inventory" {
			return ErrInvalidInstanceState
		}
		if !s.auctionItemListable(row.TemplateItem) {
			return ErrAuctionStackItemNotAllowed
		}
		minStart := s.auctionMinStartPrice(row.TemplateItem, 1, !affixesJSONEmptyRaw(row.AffixesRaw))
		if startPrice <= minStart {
			return ErrAuctionStartPriceTooLow
		}
		if err = s.LedgerSetStateTx(ctx, tx, instanceID, sellerID, "inventory", "auction", row.TemplateItem, 1, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "auction",
			Details: map[string]any{
				"type": "auction_create_instance",
			},
		}); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
				INSERT INTO market_auctions(
					seller_id, listing_kind, item_id, instance_id, quantity,
					instance_custom_data, instance_fish_data, instance_enhance_level, instance_base_snapshot, instance_affixes, instance_durability, instance_max_durability,
					start_price, current_price, min_bid_increment, buyout_price, status, ends_at,
					fee_rate_bps, fee_amount, seller_amount, anti_snipe_extend_count, created_at, updated_at
				)
				VALUES($1, 'instance', $2, $3, 1, $4::jsonb, $5::jsonb, $6, $7::jsonb, $8::jsonb, $9, $10, $11, 0, $12, $13, 'active', now() + ($14::bigint * interval '1 second'), 0, 0, 0, 0, now(), now())
				RETURNING id
			`,
			sellerID,
			row.TemplateItem,
			instanceID,
			mustEncodeItemCustomizationString(row.CustomData),
			"{}",
			row.EnhanceLevel,
			snapshotJSONOr(row.BaseSnapshot, "{}"),
			snapshotJSONOr(row.AffixesRaw, "[]"),
			row.Durability,
			row.MaxDurability,
			startPrice,
			minBidIncrement,
			buyout,
			durationSeconds,
		).Scan(&auctionID)
	})
	if err != nil {
		return 0, err
	}
	return auctionID, nil
}

func (s *Store) listAuctionRowsTx(ctx context.Context, tx pgx.Tx, whereSQL string, whereArgs ...any) ([]AuctionListing, error) {
	query := `
		SELECT a.id, a.seller_id, u.username, a.listing_kind, a.item_id, a.instance_id,
		       COALESCE(a.instance_custom_data, '{}'::jsonb), COALESCE(a.instance_fish_data, '{}'::jsonb), a.quantity,
		       a.instance_enhance_level, a.instance_base_snapshot, a.instance_affixes, a.instance_durability, a.instance_max_durability,
		       a.start_price, a.current_price, a.min_bid_increment, a.current_bidder_id, ub.username, a.buyout_price,
		       a.fee_rate_bps, a.fee_amount, a.seller_amount, a.anti_snipe_extend_count,
		       a.status, a.ends_at, a.created_at, a.updated_at
		FROM market_auctions a
		JOIN users u ON u.id = a.seller_id
		LEFT JOIN users ub ON ub.id = a.current_bidder_id
	` + whereSQL
	rows, err := tx.Query(ctx, query, whereArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuctionListing, 0, 32)
	for rows.Next() {
		var row AuctionListing
		var customDataRaw []byte
		var fishDataRaw []byte
		var baseSnapshotRaw []byte
		var affixesRaw []byte
		var currentBidderUsername sql.NullString
		if err := rows.Scan(
			&row.ID,
			&row.SellerID,
			&row.SellerUsername,
			&row.ListingKind,
			&row.ItemID,
			&row.InstanceID,
			&customDataRaw,
			&fishDataRaw,
			&row.Quantity,
			&row.InstanceEnhanceLevel,
			&baseSnapshotRaw,
			&affixesRaw,
			&row.InstanceDurability,
			&row.InstanceMaxDurability,
			&row.StartPrice,
			&row.CurrentPrice,
			&row.MinBidIncrement,
			&row.CurrentBidderID,
			&currentBidderUsername,
			&row.BuyoutPrice,
			&row.FeeRateBps,
			&row.FeeAmount,
			&row.SellerAmount,
			&row.AntiSnipeExtendCount,
			&row.Status,
			&row.EndsAt,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(baseSnapshotRaw) > 0 {
			_ = json.Unmarshal(baseSnapshotRaw, &row.InstanceBaseSnapshot)
		}
		row.InstanceCustomData = decodeItemCustomization(customDataRaw)
		if len(fishDataRaw) > 0 && string(fishDataRaw) != "{}" {
			var fish FishInstanceData
			if json.Unmarshal(fishDataRaw, &fish) == nil && strings.TrimSpace(fish.SpeciesID) != "" {
				row.FishData = &fish
			}
		}
		if len(affixesRaw) > 0 {
			row.InstanceAffixes = decodeAffixSlots(affixesRaw)
		}
		if currentBidderUsername.Valid {
			row.CurrentBidderUsername = currentBidderUsername.String
		}
		if row.Quantity <= 0 {
			row.Quantity = 1
		}
		if row.MinBidIncrement <= 0 {
			row.MinBidIncrement = 1
		}
		if row.Status == "active" {
			row.MinNextBid = row.StartPrice
			if row.CurrentPrice > 0 {
				row.MinNextBid = row.CurrentPrice + row.MinBidIncrement
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func snapshotJSONOr(raw []byte, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	return string(raw)
}

func (s *Store) ListAuctions(ctx context.Context, itemID string, includeEnded bool, limit int) ([]AuctionListing, error) {
	limit = clampMarketListLimit(limit)
	itemID = strings.TrimSpace(itemID)
	out := make([]AuctionListing, 0, limit)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.finalizeExpiredAuctionsTx(ctx, tx, time.Now(), 200); err != nil {
			return err
		}
		rows, err := s.listAuctionRowsTx(ctx, tx, `
			WHERE ($1 = '' OR a.item_id = $1)
			  AND ($2 = TRUE OR a.status = 'active')
			ORDER BY
			  CASE WHEN a.status = 'active' THEN 0 ELSE 1 END,
			  a.id DESC
			LIMIT $3
		`, itemID, includeEnded, limit)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListMyAuctions(ctx context.Context, sellerID int64, includeEnded bool, limit int) ([]AuctionListing, error) {
	limit = clampMarketListLimit(limit)
	out := make([]AuctionListing, 0, limit)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.finalizeExpiredAuctionsTx(ctx, tx, time.Now(), 200); err != nil {
			return err
		}
		rows, err := s.listAuctionRowsTx(ctx, tx, `
			WHERE a.seller_id = $1
			  AND ($2 = TRUE OR a.status = 'active')
			ORDER BY a.id DESC
			LIMIT $3
		`, sellerID, includeEnded, limit)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetAuctionByID(ctx context.Context, auctionID int64) (AuctionListing, error) {
	out := AuctionListing{}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.finalizeExpiredAuctionsTx(ctx, tx, time.Now(), 200); err != nil {
			return err
		}
		rows, err := s.listAuctionRowsTx(ctx, tx, `
			WHERE a.id = $1
			LIMIT 1
		`, auctionID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return pgx.ErrNoRows
		}
		out = rows[0]
		return nil
	})
	return out, err
}

func (s *Store) ListAuctionBids(ctx context.Context, auctionID int64, limit int) ([]AuctionBid, error) {
	limit = clampMarketListLimit(limit)
	rows, err := s.db.Query(ctx, `
		SELECT b.id, b.auction_id, b.bidder_id, u.username, b.bid_amount, b.created_at
		FROM market_auction_bids b
		JOIN users u ON u.id = b.bidder_id
		WHERE b.auction_id = $1
		ORDER BY b.id DESC
		LIMIT $2
	`, auctionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuctionBid, 0, limit)
	for rows.Next() {
		var row AuctionBid
		if err := rows.Scan(&row.ID, &row.AuctionID, &row.BidderID, &row.BidderUsername, &row.BidAmount, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) BidAuction(ctx context.Context, bidderID, auctionID, bidAmount int64) (AuctionBidResult, error) {
	if bidAmount <= 0 {
		return AuctionBidResult{}, ErrBadQuantity
	}
	now := time.Now()
	result := AuctionBidResult{AuctionID: auctionID, BidderID: bidderID}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.finalizeExpiredAuctionsTx(ctx, tx, now, 200); err != nil {
			return err
		}
		var sellerID int64
		var listingKind string
		var startPrice int64
		var currentPrice int64
		var minBidIncrement int64
		var currentBidderID *int64
		var buyoutPrice *int64
		var status string
		var endsAt time.Time
		var itemID string
		var instanceID *int64
		var quantity int64
		var antiSnipeExtendCount int
		var instanceEnhanceLevel int
		var instanceAffixesRaw []byte
		if err := tx.QueryRow(ctx, `
			SELECT seller_id, listing_kind, start_price, current_price, min_bid_increment, current_bidder_id, buyout_price,
			       status, ends_at, item_id, instance_id, quantity, anti_snipe_extend_count,
			       COALESCE(instance_enhance_level, 0), COALESCE(instance_affixes, '[]'::jsonb)
			FROM market_auctions
			WHERE id = $1
			FOR UPDATE
		`, auctionID).Scan(
			&sellerID,
			&listingKind,
			&startPrice,
			&currentPrice,
			&minBidIncrement,
			&currentBidderID,
			&buyoutPrice,
			&status,
			&endsAt,
			&itemID,
			&instanceID,
			&quantity,
			&antiSnipeExtendCount,
			&instanceEnhanceLevel,
			&instanceAffixesRaw,
		); err != nil {
			return err
		}
		// 猫村声望等级检查
		totalRep, err := s.GetCatVillageReputation(ctx, bidderID)
		if err != nil {
			return fmt.Errorf("get cat village reputation: %w", err)
		}
		if int64(ComputeRepLevel(totalRep, s.config.CatVillageRepLevels).Level) < s.config.AuctionMinRepLevel {
			return ErrAuctionReputationTooLow
		}

		result.SellerID = sellerID
		result.PreviousBidderID = currentBidderID
		if sellerID == bidderID {
			return ErrAuctionForbidden
		}
		if status != "active" || !endsAt.After(now) {
			return ErrAuctionNotActive
		}
		minBid := startPrice
		if currentPrice > 0 {
			if minBidIncrement <= 0 {
				minBidIncrement = 1
			}
			minBid = currentPrice + minBidIncrement
		}
		if bidAmount < minBid {
			return ErrAuctionBidTooLow
		}

		bidderGold, err := s.ConsumableQuantityByTemplateTx(ctx, tx, bidderID, GoldCoinItemID)
		if err != nil {
			return err
		}
		if bidderGold < bidAmount {
			return ErrInsufficientItem
		}
		if err = s.LedgerConsumeTemplateTx(ctx, tx, bidderID, GoldCoinItemID, bidAmount, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "auction_bid_hold",
			Details: map[string]any{
				"auctionId": auctionID,
			},
		}); err != nil {
			return err
		}
		if currentBidderID != nil && currentPrice > 0 {
			if err = s.LedgerGrantTemplateTx(ctx, tx, *currentBidderID, GoldCoinItemID, currentPrice, InventoryLedgerMeta{
				SourceSurface: "auction_bid_hold",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": auctionID,
					"type":      "auction_outbid_refund",
				},
			}); err != nil {
				return err
			}
		}

		nextEndsAt := endsAt
		nextAntiSnipeCount := antiSnipeExtendCount
		triggerSeconds := s.config.AuctionAntiSnipeTriggerSeconds
		extendSeconds := s.config.AuctionAntiSnipeExtendSeconds
		if triggerSeconds > 0 && extendSeconds > 0 {
			remain := endsAt.Sub(now)
			if remain > 0 && remain <= time.Duration(triggerSeconds)*time.Second {
				nextEndsAt = endsAt.Add(time.Duration(extendSeconds) * time.Second)
				nextAntiSnipeCount++
			}
		}

		if _, err = tx.Exec(ctx, `
			UPDATE market_auctions
			SET current_price = $1,
			    current_bidder_id = $2,
			    ends_at = $3,
			    anti_snipe_extend_count = $4,
			    updated_at = now()
			WHERE id = $5
		`, bidAmount, bidderID, nextEndsAt, nextAntiSnipeCount, auctionID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO market_auction_bids(auction_id, bidder_id, bid_amount, created_at)
			VALUES($1, $2, $3, now())
		`, auctionID, bidderID, bidAmount); err != nil {
			return err
		}

		shouldFinalize := buyoutPrice != nil && *buyoutPrice > 0 && bidAmount >= *buyoutPrice
		if shouldFinalize {
			if listingKind == "instance" {
				if instanceID == nil {
					return ErrInvalidInstanceState
				}
				if err = s.LedgerTransferOwnershipAndStateTx(ctx, tx, *instanceID, sellerID, bidderID, "auction", "inventory", itemID, 1, InventoryLedgerMeta{
					SourceSurface: "auction",
					TargetSurface: "inventory",
					Details: map[string]any{
						"auctionId": auctionID,
						"type":      "auction_buyout_item",
					},
				}); err != nil {
					return err
				}
			} else {
				if quantity <= 0 {
					quantity = 1
				}
				if err = s.LedgerGrantTemplateTx(ctx, tx, bidderID, itemID, quantity, InventoryLedgerMeta{
					SourceSurface: "auction",
					TargetSurface: "inventory",
					Details: map[string]any{
						"auctionId": auctionID,
						"type":      "auction_buyout_item_stack",
					},
				}); err != nil {
					return err
				}
			}
			feeRateBps := s.auctionFeeRateBps()
			feeAmount := (bidAmount * feeRateBps) / 10_000
			sellerAmount := bidAmount - feeAmount
			if sellerAmount < 0 {
				sellerAmount = 0
			}
			if sellerAmount > 0 {
				if err = s.LedgerGrantTemplateTx(ctx, tx, sellerID, GoldCoinItemID, sellerAmount, InventoryLedgerMeta{
					SourceSurface: "auction_bid_hold",
					TargetSurface: "inventory",
					Details: map[string]any{
						"auctionId": auctionID,
						"type":      "auction_buyout_gold",
					},
				}); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `
				UPDATE market_auctions
				SET status = 'sold',
				    instance_id = NULL,
				    fee_rate_bps = $2,
				    fee_amount = $3,
				    seller_amount = $4,
				    updated_at = now()
				WHERE id = $1
			`, auctionID, feeRateBps, feeAmount, sellerAmount); err != nil {
				return err
			}
			unitPrice := bidAmount
			if quantity > 1 {
				unitPrice = bidAmount / quantity
			}
			buyoutExtra := map[string]any{"listingKind": listingKind}
			if listingKind == "instance" {
				buyoutExtra["enhanceLevel"] = instanceEnhanceLevel
				buyoutExtra["affixCount"] = countEquippedAffixes(instanceAffixesRaw)
			}
			if err = s.recordRiskTransferTx(ctx, tx, RiskTransferRecord{
				FromUserID:  bidderID,
				ToUserID:    sellerID,
				Channel:     "auction_buyout",
				ReferenceID: auctionID,
				ItemID:      itemID,
				Quantity:    quantity,
				UnitPrice:   unitPrice,
				TotalPrice:  bidAmount,
				FeeAmount:   feeAmount,
				NetAmount:   sellerAmount,
				PriceAnchor: s.ItemEconomyBaseValue(itemID),
				Extra:       buyoutExtra,
			}); err != nil {
				return err
			}
		}
		result.BuyoutFinalized = shouldFinalize
		return nil
	})
	if err != nil {
		return AuctionBidResult{}, err
	}
	return result, nil
}

func (s *Store) CancelAuction(ctx context.Context, sellerID, auctionID int64) error {
	now := time.Now()
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.finalizeExpiredAuctionsTx(ctx, tx, now, 200); err != nil {
			return err
		}

		var ownerID int64
		var itemID string
		var listingKind string
		var quantity int64
		var currentPrice int64
		var status string
		var currentBidderID *int64
		var instanceID *int64
		if err := tx.QueryRow(ctx, `
			SELECT seller_id, item_id, listing_kind, quantity, current_price, status, current_bidder_id, instance_id
			FROM market_auctions
			WHERE id = $1
			FOR UPDATE
		`, auctionID).Scan(&ownerID, &itemID, &listingKind, &quantity, &currentPrice, &status, &currentBidderID, &instanceID); err != nil {
			return err
		}
		if ownerID != sellerID {
			return ErrAuctionForbidden
		}
		if status != "active" {
			return ErrAuctionNotActive
		}
		cancelFee := s.auctionCancelFeeAmount(currentPrice)
		if currentBidderID != nil && cancelFee > 0 {
			gold, err := s.ConsumableQuantityByTemplateTx(ctx, tx, sellerID, GoldCoinItemID)
			if err != nil {
				return err
			}
			if gold < cancelFee {
				return ErrAuctionCancelFeeInsufficientGold
			}
		}
		if currentBidderID != nil && currentPrice > 0 {
			if err := s.LedgerGrantTemplateTx(ctx, tx, *currentBidderID, GoldCoinItemID, currentPrice, InventoryLedgerMeta{
				SourceSurface: "auction_bid_hold",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": auctionID,
					"type":      "auction_cancel_refund",
				},
			}); err != nil {
				return err
			}
		}
		if cancelFee > 0 {
			if err := s.LedgerConsumeTemplateTx(ctx, tx, sellerID, GoldCoinItemID, cancelFee, InventoryLedgerMeta{
				SourceSurface: "inventory",
				TargetSurface: "auction_cancel_fee",
				Details: map[string]any{
					"auctionId": auctionID,
					"type":      "auction_cancel_fee",
				},
			}); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE market_auctions
			SET status = 'cancelled',
			    instance_id = NULL,
			    updated_at = now()
			WHERE id = $1
		`, auctionID); err != nil {
			return err
		}

		if listingKind == "instance" {
			if instanceID == nil {
				return ErrInvalidInstanceState
			}
			return s.LedgerSetStateTx(ctx, tx, *instanceID, sellerID, "auction", "inventory", itemID, 1, InventoryLedgerMeta{
				EventType:     WarehouseChangeEventSystemToWarehouse,
				SourceSurface: "auction",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": auctionID,
					"type":      "auction_cancel_return",
				},
			})
		}
		if quantity <= 0 {
			quantity = 1
		}
		return s.LedgerGrantTemplateTx(ctx, tx, sellerID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "auction",
			TargetSurface: "inventory",
			Details: map[string]any{
				"auctionId": auctionID,
				"type":      "auction_cancel_return_stack",
			},
		})
	})
}
