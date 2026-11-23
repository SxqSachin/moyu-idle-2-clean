package store

import (
	"context"
	"encoding/json"
	"mysecondwebgame/server/internal/marketpolicy"
	"time"
)

type InstanceMarketGroup struct {
	GroupKey    string                      `json:"groupKey"`
	CatalogKey  string                      `json:"catalogKey"`
	ItemID      string                      `json:"itemId"`
	Quantity    int64                       `json:"quantity"`
	SellerCount int64                       `json:"sellerCount"`
	LowestPrice int64                       `json:"lowestPrice"`
	Fields      []marketpolicy.DisplayField `json:"fields"`
	Snapshot    map[string]any              `json:"snapshot"`
}
type InstanceMarketListing struct {
	ID              int64                       `json:"id"`
	IsOwn           bool                        `json:"isOwn"`
	CanBuy          bool                        `json:"canBuy"`
	ItemID          string                      `json:"itemId"`
	GroupKey        string                      `json:"groupKey"`
	Price           int64                       `json:"price"`
	Quantity        int64                       `json:"quantity"`
	InitialQuantity int64                       `json:"initialQuantity"`
	Status          string                      `json:"status"`
	Fields          []marketpolicy.DisplayField `json:"fields"`
	CreatedAt       time.Time                   `json:"createdAt"`
}
type InstanceMarketDetail struct {
	InstanceID int64          `json:"instanceId"`
	State      string         `json:"state"`
	Snapshot   map[string]any `json:"snapshot"`
}
type InstanceMarketOrder struct {
	ID        int64                       `json:"id"`
	ListingID int64                       `json:"listingId"`
	ItemID    string                      `json:"itemId"`
	Quantity  int64                       `json:"quantity"`
	Total     int64                       `json:"total"`
	Fields    []marketpolicy.DisplayField `json:"fields"`
	CreatedAt time.Time                   `json:"createdAt"`
}

func marketPage(offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > 100000 {
		offset = 100000
	}
	if limit < 1 {
		limit = 30
	}
	if limit > 50 {
		limit = 50
	}
	return offset, limit
}

func (s *Store) InstanceMarketCatalogFilter(itemID, values string) (string, error) {
	item := s.gameData.ItemByID(itemID)
	if item == nil || !item.UsesInstanceMarket() {
		return "", ErrInstanceMarketUnavailable
	}
	profile := s.gameData.MarketProfile(item.MarketPolicy.FixedPrice.ProfileID)
	if profile == nil {
		return "", ErrInstanceMarketUnavailable
	}
	key, err := profile.CatalogFilter(itemID, values)
	if err != nil {
		return "", ErrInstanceMarketRequest
	}
	return key, nil
}

func (s *Store) InstanceMarketGroups(ctx context.Context, itemID, catalogKey string, offset, limit int) ([]InstanceMarketGroup, error) {
	offset, limit = marketPage(offset, limit)
	item := s.gameData.ItemByID(itemID)
	if item == nil || !item.UsesInstanceMarket() {
		return nil, ErrInstanceMarketUnavailable
	}
	profile := s.gameData.MarketProfile(item.MarketPolicy.FixedPrice.ProfileID)
	if profile == nil {
		return nil, ErrInstanceMarketUnavailable
	}
	revision, err := marketpolicy.Digest([]any{profile.ID, profile.SchemaVersion, profile.GroupingVersion, profile.CatalogKeys, profile.GroupKeys, profile.UniqueByInstance})
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT g.group_key,g.catalog_key,g.quantity,g.sellers,g.price,l.fields,s.snapshot
 FROM (SELECT group_key,catalog_key,SUM(quantity) AS quantity,COUNT(DISTINCT seller_id) AS sellers,MIN(price) AS price,MIN(id) AS sample
 FROM market_instance_listings WHERE item_id=$1 AND revision=$2 AND status='active' AND ($3='' OR catalog_key=$3)
 GROUP BY group_key,catalog_key ORDER BY MIN(price),group_key OFFSET $4 LIMIT $5) g
 JOIN market_instance_listings l ON l.id=g.sample
 JOIN LATERAL (SELECT snapshot FROM market_instance_listing_items WHERE listing_id=l.id AND state='listed' ORDER BY id LIMIT 1) s ON true
 ORDER BY g.price,g.group_key`, itemID, revision, catalogKey, offset, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InstanceMarketGroup{}
	for rows.Next() {
		var r InstanceMarketGroup
		var fields, snapshot []byte
		r.ItemID = itemID
		if err = rows.Scan(&r.GroupKey, &r.CatalogKey, &r.Quantity, &r.SellerCount, &r.LowestPrice, &fields, &snapshot); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(fields, &r.Fields); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(snapshot, &r.Snapshot); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) InstanceMarketListings(ctx context.Context, itemID, group string, mine, viewer int64, offset, limit int) ([]InstanceMarketListing, error) {
	offset, limit = marketPage(offset, limit)
	if mine <= 0 && (itemID == "" || len(group) != 64) {
		return nil, ErrInstanceMarketRequest
	}
	rows, err := s.db.Query(ctx, `WITH best AS (
 SELECT id FROM market_instance_listings WHERE $3=0 AND item_id=$1 AND group_key=$2 AND status='active' AND seller_id<>$6 ORDER BY price,id LIMIT 1)
 SELECT l.id,l.seller_id=$6,COALESCE(l.id=(SELECT id FROM best),false),l.item_id,l.group_key,l.price,l.quantity,l.initial_quantity,l.status,l.fields,l.created_at
 FROM market_instance_listings l
 WHERE ($1='' OR l.item_id=$1) AND (($3>0 AND l.seller_id=$3) OR ($3=0 AND l.status='active' AND l.group_key=$2))
 ORDER BY CASE WHEN $3>0 THEN -l.id ELSE l.price END,l.id OFFSET $4 LIMIT $5`, itemID, group, mine, offset, limit+1, viewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InstanceMarketListing{}
	for rows.Next() {
		var r InstanceMarketListing
		var raw []byte
		if err = rows.Scan(&r.ID, &r.IsOwn, &r.CanBuy, &r.ItemID, &r.GroupKey, &r.Price, &r.Quantity, &r.InitialQuantity, &r.Status, &raw, &r.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.Fields); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) InstanceMarketListingDetails(ctx context.Context, userID, listingID int64) ([]InstanceMarketDetail, error) {
	rows, err := s.db.Query(ctx, `SELECT i.original_instance_id,i.state,i.snapshot FROM market_instance_listing_items i
 JOIN market_instance_listings l ON l.id=i.listing_id
 LEFT JOIN market_instance_orders o ON o.id=i.order_id
 WHERE l.id=$1 AND ((l.status='active' AND i.state='listed') OR l.seller_id=$2 OR o.buyer_id=$2)
 ORDER BY i.original_instance_id LIMIT 100`, listingID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InstanceMarketDetail{}
	for rows.Next() {
		var r InstanceMarketDetail
		var raw []byte
		if err = rows.Scan(&r.InstanceID, &r.State, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.Snapshot); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) InstanceMarketOrders(ctx context.Context, userID int64, offset, limit int) ([]InstanceMarketOrder, error) {
	offset, limit = marketPage(offset, limit)
	rows, err := s.db.Query(ctx, `SELECT id,listing_id,item_id,quantity,total_price,fields,created_at FROM market_instance_orders WHERE buyer_id=$1 ORDER BY id DESC OFFSET $2 LIMIT $3`, userID, offset, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InstanceMarketOrder{}
	for rows.Next() {
		var r InstanceMarketOrder
		var raw []byte
		if err = rows.Scan(&r.ID, &r.ListingID, &r.ItemID, &r.Quantity, &r.Total, &raw, &r.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.Fields); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
