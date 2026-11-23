package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/marketpolicy"
)

func instanceMarketMeta(from, to, kind string, listingID int64) InventoryLedgerMeta {
	return InventoryLedgerMeta{SourceSurface: from, TargetSurface: to, Details: map[string]any{"type": kind, "listingId": listingID, "listingKind": "instance_variants"}}
}
func (s *Store) lockInstanceMarketUsersTx(ctx context.Context, tx pgx.Tx, a, b int64) error {
	if a > b {
		a, b = b, a
	}
	if err := s.lockUserTx(ctx, tx, a); err != nil {
		return err
	}
	if b != a {
		return s.lockUserTx(ctx, tx, b)
	}
	return nil
}

func (s *Store) moveMarketInstancesTx(ctx context.Context, tx pgx.Tx, ids []int64, fromUser, toUser int64, fromState, toState, itemID string, listingID int64) error {
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, fromUser, ids...); err != nil {
		return err
	}
	var used bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_equip_slots WHERE instance_id=ANY($1) UNION ALL SELECT 1 FROM player_kitty_equip_slots WHERE instance_id=ANY($1))`, ids).Scan(&used); err != nil {
		return err
	}
	if used {
		return ErrInvalidInstanceState
	}
	// An active competition claim cannot be duplicated by transferring its
	// underlying instance to another entrant. Historical records stay untouched.
	if fromUser == toUser && fromState == "inventory" {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fishing_tournament_entries e JOIN fishing_tournaments t ON t.id=e.tournament_id WHERE e.fish_instance_id=ANY($1) AND t.status='open' AND t.ends_at>now())`, ids).Scan(&used); err != nil {
			return err
		}
		if used {
			return ErrInvalidInstanceState
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE player_item_instances SET user_id=$1,state=$2,archived_at=CASE WHEN user_id<>$1 THEN NULL ELSE archived_at END,updated_at=now()
 WHERE instance_id=ANY($3) AND user_id=$4 AND state=$5 AND template_item_id=$6 AND quantity=1`, toUser, toState, ids, fromUser, fromState, itemID)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(ids) {
		return ErrInvalidInstanceState
	}
	for _, id := range ids {
		meta := instanceMarketMeta(fromState, toState, "instance_market_transfer", listingID)
		meta.InstanceID = &id
		if err = s.LedgerLogOnlyTx(ctx, tx, toUser, itemID, 1, meta); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateInstanceMarketListing(ctx context.Context, userID int64, itemID string, price int64, items []InstanceMarketSelection, key string, validatePrice func(string, int64) error) (int64, error) {
	selected, ids, err := normalizedMarketSelections(items, true)
	if err != nil {
		return 0, err
	}
	if !validMarketRequestKey(key) || price <= 0 || price > 1_000_000_000_000 {
		return 0, ErrInstanceMarketRequest
	}
	hash, err := marketpolicy.Digest([]any{itemID, price, selected})
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var previous string
		err := tx.QueryRow(ctx, `SELECT id,request_hash FROM market_instance_listings WHERE seller_id=$1 AND request_key=$2`, userID, key).Scan(&id, &previous)
		if err == nil {
			if previous != hash {
				return ErrDuplicateIdempotency
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// A committed receipt remains replayable after pricing/configuration edits.
		// The service owns the shared price grid; validation runs only for new writes.
		if validatePrice != nil {
			if err = validatePrice(itemID, price); err != nil {
				return err
			}
		}
		if minimum := s.ItemEconomyBaseValue(itemID); price < minimum {
			return ErrInstanceMarketRequest
		}
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM market_instance_listings WHERE seller_id=$1 AND status='active'`, userID).Scan(&count); err != nil {
			return err
		}
		if count >= 200 {
			return ErrInstanceMarketUnavailable
		}
		rows, err := s.lockMarketInstancesTx(ctx, tx, userID, ids, "inventory")
		if err != nil {
			return err
		}
		quotes := make([]InstanceMarketQuote, len(rows))
		for i, r := range rows {
			q, e := s.projectMarketInstance(r)
			if e != nil {
				return e
			}
			if q.ItemID != itemID || q.QuoteToken != selected[i].QuoteToken || (i > 0 && q.GroupKey != quotes[0].GroupKey) {
				return ErrInstanceMarketChanged
			}
			quotes[i] = q
		}
		q := quotes[0]
		var collision bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM market_instance_listings WHERE group_key=$1 AND descriptor<>$2::jsonb)`, q.GroupKey, string(q.Descriptor)).Scan(&collision); err != nil {
			return err
		}
		if collision {
			return ErrInstanceMarketChanged
		}
		fields, err := json.Marshal(q.Fields)
		if err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `INSERT INTO market_instance_listings(seller_id,item_id,profile_id,revision,catalog_key,group_key,descriptor,fields,price,quantity,initial_quantity,status,request_key,request_hash)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,'active',$11,$12) RETURNING id`, userID, itemID, q.ProfileID, q.Revision, q.CatalogKey, q.GroupKey, string(q.Descriptor), string(fields), price, len(ids), key, hash).Scan(&id); err != nil {
			return err
		}
		batch := &pgx.Batch{}
		for _, q := range quotes {
			snapshot, e := json.Marshal(q.Snapshot)
			if e != nil {
				return e
			}
			batch.Queue(`INSERT INTO market_instance_listing_items(listing_id,original_instance_id,escrow_instance_id,snapshot,state) VALUES($1,$2,$2,$3,'listed')`, id, q.InstanceID, string(snapshot))
		}
		if err = tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
		return s.moveMarketInstancesTx(ctx, tx, ids, userID, userID, "inventory", "market", itemID, id)
	})
	return id, err
}

type InstanceMarketPurchase struct {
	ListingID int64  `json:"listingId"`
	Quantity  int64  `json:"quantity"`
	Price     int64  `json:"price"`
	GroupKey  string `json:"groupKey"`
}

func (s *Store) BuyInstanceMarketListing(ctx context.Context, buyer int64, input InstanceMarketPurchase, key string) (BuyListingResult, error) {
	var out BuyListingResult
	if input.ListingID <= 0 || input.Quantity < 1 || input.Quantity > InstanceMarketBatchLimit || input.Price <= 0 || input.Price > 1_000_000_000_000 || len(input.GroupKey) != 64 || !validMarketRequestKey(key) {
		return out, ErrInstanceMarketRequest
	}
	hash, err := marketpolicy.Digest(input)
	if err != nil {
		return out, err
	}
	// Seller identity is immutable. Read it before locking either account so all
	// new-market paths acquire users, then listings, then instances in that order.
	var seller int64
	if err = s.db.QueryRow(ctx, `SELECT seller_id FROM market_instance_listings WHERE id=$1`, input.ListingID).Scan(&seller); err != nil {
		return out, err
	}
	if seller == buyer {
		return out, ErrInstanceMarketRequest
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockInstanceMarketUsersTx(ctx, tx, buyer, seller); err != nil {
			return err
		}
		var previous string
		err := tx.QueryRow(ctx, `SELECT id,request_hash,fee_rate_bps,fee_amount,seller_amount,seller_id FROM market_instance_orders WHERE buyer_id=$1 AND request_key=$2`, buyer, key).Scan(&out.OrderID, &previous, &out.FeeRateBps, &out.FeeAmount, &out.SellerAmount, &out.SellerID)
		if err == nil {
			if previous != hash {
				return ErrDuplicateIdempotency
			}
			out.BuyerID = buyer
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var itemID, group, revision, status string
		var price, available int64
		var fields []byte
		if err = tx.QueryRow(ctx, `SELECT item_id,group_key,revision,price,quantity,status,fields FROM market_instance_listings WHERE id=$1 FOR UPDATE`, input.ListingID).Scan(&itemID, &group, &revision, &price, &available, &status, &fields); err != nil {
			return err
		}
		if status != "active" || available < input.Quantity {
			return ErrListingNotEnough
		}
		if price != input.Price || group != input.GroupKey {
			return ErrInstanceMarketChanged
		}
		// A stale or crafted request must not skip a cheaper/earlier offer.
		// Own offers cannot be bought and therefore do not block the queue.
		var earlier bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM market_instance_listings
 WHERE item_id=$1 AND group_key=$2 AND status='active' AND seller_id<>$3
 AND (price,id)<($4,$5))`, itemID, group, buyer, price, input.ListingID).Scan(&earlier); err != nil {
			return err
		}
		if earlier {
			return ErrInstanceMarketChanged
		}
		rows, err := tx.Query(ctx, `SELECT escrow_instance_id FROM market_instance_listing_items WHERE listing_id=$1 AND state='listed' ORDER BY original_instance_id LIMIT $2 FOR UPDATE`, input.ListingID, input.Quantity)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, input.Quantity)
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if int64(len(ids)) != input.Quantity {
			return ErrInvalidInstanceState
		}
		instances, err := s.lockMarketInstancesTx(ctx, tx, seller, ids, "market")
		if err != nil {
			return err
		}
		for _, r := range instances {
			q, e := s.projectMarketInstance(r)
			if e != nil {
				return errors.Join(ErrInstanceMarketChanged, e)
			}
			if q.GroupKey != group || q.Revision != revision {
				return ErrInstanceMarketChanged
			}
		}
		total, err := checkedMulInt64(price, input.Quantity)
		if err != nil {
			return err
		}
		bps, err := s.queryMarketFeeTierBpsTx(ctx, tx, itemID, total)
		if err != nil {
			return err
		}
		if bps < 0 || bps > 5000 {
			return ErrInstanceMarketRequest
		}
		fee := total/10000*bps + (total%10000*bps)/10000
		income := total - fee
		if err = s.LedgerConsumeTemplateTx(ctx, tx, buyer, GoldCoinItemID, total, instanceMarketMeta("inventory", "market_trade", "instance_market_buy_cost", input.ListingID)); err != nil {
			return err
		}
		if income > 0 {
			if err = s.LedgerGrantTemplateTx(ctx, tx, seller, GoldCoinItemID, income, instanceMarketMeta("market_trade", "inventory", "instance_market_sell_income", input.ListingID)); err != nil {
				return err
			}
		}
		if err = tx.QueryRow(ctx, `INSERT INTO market_instance_orders(listing_id,buyer_id,seller_id,item_id,quantity,unit_price,total_price,fee_rate_bps,fee_amount,seller_amount,fields,group_key,request_key,request_hash)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`, input.ListingID, buyer, seller, itemID, input.Quantity, price, total, bps, fee, income, string(fields), group, key, hash).Scan(&out.OrderID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE market_instance_listing_items SET state='sold',order_id=$1,escrow_instance_id=NULL WHERE listing_id=$2 AND escrow_instance_id=ANY($3) AND state='listed'`, out.OrderID, input.ListingID, ids)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != input.Quantity {
			return ErrInvalidInstanceState
		}
		if err = s.moveMarketInstancesTx(ctx, tx, ids, seller, buyer, "market", "inventory", itemID, input.ListingID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE market_instance_listings SET quantity=quantity-$2,status=CASE WHEN quantity=$2 THEN 'sold' ELSE 'active' END WHERE id=$1`, input.ListingID, input.Quantity); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO market_instance_ledger(order_id,from_user,to_user,amount,fee_amount) VALUES($1,$2,$3,$4,$5)`, out.OrderID, buyer, seller, income, fee); err != nil {
			return err
		}
		if err = s.recordRiskTransferTx(ctx, tx, RiskTransferRecord{FromUserID: buyer, ToUserID: seller, Channel: "market_instance", ReferenceID: out.OrderID, ItemID: itemID, Quantity: input.Quantity, UnitPrice: price, TotalPrice: total, FeeAmount: fee, NetAmount: income, PriceAnchor: s.ItemEconomyBaseValue(itemID), Extra: map[string]any{"listingId": input.ListingID, "groupKey": group}}); err != nil {
			return err
		}
		out.Created = true
		out.SellerID = seller
		out.BuyerID = buyer
		out.FeeRateBps = bps
		out.FeeAmount = fee
		out.SellerAmount = income
		return nil
	})
	return out, err
}

func (s *Store) CancelInstanceMarketListing(ctx context.Context, userID, listingID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var owner int64
		var itemID, status string
		if err := tx.QueryRow(ctx, `SELECT seller_id,item_id,status FROM market_instance_listings WHERE id=$1 FOR UPDATE`, listingID).Scan(&owner, &itemID, &status); err != nil {
			return err
		}
		if owner != userID {
			return ErrMarketListingForbidden
		}
		if status == "cancelled" {
			return nil
		}
		if status != "active" {
			return ErrMarketListingNotActive
		}
		rows, err := tx.Query(ctx, `SELECT escrow_instance_id FROM market_instance_listing_items WHERE listing_id=$1 AND state='listed' ORDER BY original_instance_id FOR UPDATE`, listingID)
		if err != nil {
			return err
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		if _, err = s.lockMarketInstancesTx(ctx, tx, userID, ids, "market"); err != nil {
			return err
		}
		if err = s.moveMarketInstancesTx(ctx, tx, ids, userID, userID, "market", "inventory", itemID, listingID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE market_instance_listing_items SET state='returned',escrow_instance_id=NULL WHERE listing_id=$1 AND state='listed'`, listingID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE market_instance_listings SET status='cancelled',quantity=0 WHERE id=$1`, listingID)
		return err
	})
}
