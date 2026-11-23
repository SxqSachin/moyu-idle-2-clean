package market

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/mq"
	"mysecondwebgame/server/internal/store"
)

var ErrInvalidItem = errors.New("invalid item")
var ErrItemNotListable = errors.New("item is not market listable")
var ErrInstanceMustAuction = errors.New("instance must be auctioned")
var ErrInstanceListingDisabled = errors.New("market instance listing disabled")
var ErrPriceBelowRealValue = errors.New("price below real value")
var ErrPriceStepInvalid = errors.New("price step invalid")

type PriceStep struct {
	Min  int64
	Max  *int64
	Step int64
}

type ServiceOptions struct {
	MarketPriceSteps []PriceStep
}

type itemMeta struct {
	ItemTypes           []string
	MarketListable      bool
	FixedMarketListable bool
	EconomyBaseValue    int64
	Stackable           bool
}

type Service struct {
	store      *store.Store
	mq         mq.Publisher
	itemMeta   map[string]itemMeta
	priceSteps []PriceStep
}

func NewService(st *store.Store, publisher mq.Publisher, c catalog.Data, opts ServiceOptions) *Service {
	meta := make(map[string]itemMeta, len(c.Items))
	randomDefaultAffixItemIDs := randomDefaultAffixRewardItemIDs(c)
	for _, it := range c.Items {
		meta[it.ID] = itemMeta{
			ItemTypes:           append([]string(nil), it.Type...),
			MarketListable:      it.AllowsAuction(),
			FixedMarketListable: fixedMarketListable(it, randomDefaultAffixItemIDs),
			EconomyBaseValue:    normalizeMarketRealValue(it.EconomyBaseValue),
			Stackable:           it.Stackable,
		}
	}
	priceSteps := normalizePriceSteps(opts.MarketPriceSteps)
	return &Service{store: st, mq: publisher, itemMeta: meta, priceSteps: priceSteps}
}

func (s *Service) lookupItemMeta(itemID string) (itemMeta, bool) {
	meta, ok := s.itemMeta[strings.TrimSpace(itemID)]
	return meta, ok
}

func (s *Service) assertItemListable(itemID string) error {
	meta, ok := s.lookupItemMeta(itemID)
	if !ok {
		return ErrInvalidItem
	}
	if !meta.MarketListable {
		return ErrItemNotListable
	}
	return nil
}

func (s *Service) assertFixedMarketItemListable(itemID string) error {
	meta, ok := s.lookupItemMeta(itemID)
	if !ok {
		return ErrInvalidItem
	}
	if !meta.FixedMarketListable {
		return ErrItemNotListable
	}
	return nil
}

func (s *Service) IsItemListable(itemID string) bool {
	meta, ok := s.lookupItemMeta(itemID)
	return ok && meta.MarketListable
}

func (s *Service) IsFixedMarketItemListable(itemID string) bool {
	meta, ok := s.lookupItemMeta(itemID)
	return ok && meta.FixedMarketListable
}

func randomDefaultAffixRewardItemIDs(c catalog.Data) map[string]struct{} {
	out := map[string]struct{}{}
	for _, action := range c.Actions {
		for itemID, reward := range action.ItemReward {
			if strings.TrimSpace(reward.GrantPolicy) == store.InventoryGrantPolicyRollDefaultAffixes {
				out[strings.TrimSpace(itemID)] = struct{}{}
			}
		}
	}
	return out
}

func fixedMarketListable(item catalog.Item, randomDefaultAffixItemIDs map[string]struct{}) bool {
	if item.MarketPolicy != nil {
		return false
	}
	if !item.MarketListable {
		return false
	}
	for _, itemType := range item.Type {
		if strings.TrimSpace(itemType) == "currency" {
			return false
		}
	}
	_, randomDefaultAffix := randomDefaultAffixItemIDs[strings.TrimSpace(item.ID)]
	return !randomDefaultAffix
}

func normalizeMarketRealValue(raw float64) int64 {
	if math.IsNaN(raw) || math.IsInf(raw, 0) || raw <= 0 {
		return 1
	}
	value := int64(math.Floor(raw))
	if value <= 0 {
		return 1
	}
	return value
}

func normalizePriceSteps(raw []PriceStep) []PriceStep {
	if len(raw) == 0 {
		return []PriceStep{
			{Min: 0, Max: int64Ptr(100), Step: 1},
			{Min: 100, Max: int64Ptr(1000), Step: 10},
			{Min: 1000, Max: int64Ptr(10000), Step: 50},
			{Min: 10000, Max: int64Ptr(100000), Step: 100},
			{Min: 100000, Max: int64Ptr(1000000), Step: 1000},
			{Min: 1000000, Max: int64Ptr(10000000), Step: 10000},
			{Min: 10000000, Max: nil, Step: 100000},
		}
	}
	out := make([]PriceStep, 0, len(raw))
	for _, row := range raw {
		if row.Step <= 0 || row.Min < 0 {
			continue
		}
		if row.Max != nil && *row.Max <= row.Min {
			continue
		}
		copied := PriceStep{
			Min:  row.Min,
			Max:  row.Max,
			Step: row.Step,
		}
		out = append(out, copied)
	}
	if len(out) == 0 {
		return normalizePriceSteps(nil)
	}
	return out
}

func (s *Service) priceStepFor(price int64) PriceStep {
	for _, row := range s.priceSteps {
		if price < row.Min {
			continue
		}
		if row.Max != nil && price >= *row.Max {
			continue
		}
		return row
	}
	return PriceStep{Min: 0, Max: nil, Step: 1}
}

func (s *Service) isPriceOnGrid(price int64) bool {
	stepCfg := s.priceStepFor(price)
	if stepCfg.Step <= 1 {
		return true
	}
	return (price-stepCfg.Min)%stepCfg.Step == 0
}

func (s *Service) assertMarketPrice(itemID string, price int64) error {
	meta, ok := s.lookupItemMeta(itemID)
	if !ok {
		return ErrInvalidItem
	}
	minPrice := meta.EconomyBaseValue
	if minPrice <= 0 {
		minPrice = 1
	}
	if price < minPrice {
		return ErrPriceBelowRealValue
	}
	if price == minPrice {
		return nil
	}
	if !s.isPriceOnGrid(price) {
		return ErrPriceStepInvalid
	}
	return nil
}

func (s *Service) CreateBuyOrder(ctx context.Context, buyerID int64, itemID string, price int64, quantity int64) (int64, error) {
	itemID = strings.TrimSpace(itemID)
	if err := s.assertFixedMarketItemListable(itemID); err != nil {
		return 0, err
	}
	if err := s.assertMarketPrice(itemID, price); err != nil {
		return 0, err
	}
	return s.store.CreateBuyOrder(ctx, buyerID, itemID, price, quantity)
}

func (s *Service) List(ctx context.Context, sellerID int64, itemID string, price int64, quantity int64) (int64, error) {
	itemID = strings.TrimSpace(itemID)
	if err := s.assertFixedMarketItemListable(itemID); err != nil {
		return 0, err
	}
	if err := s.assertMarketPrice(itemID, price); err != nil {
		return 0, err
	}
	listingID, err := s.store.CreateListing(ctx, sellerID, itemID, price, quantity)
	if err != nil {
		return 0, err
	}
	_ = s.mq.PublishJSON(ctx, "market.listed", map[string]any{
		"listingId": listingID,
		"sellerId":  sellerID,
		"itemId":    itemID,
		"price":     price,
		"quantity":  quantity,
		"ts":        time.Now().UnixMilli(),
	})
	return listingID, nil
}

func (s *Service) ListInstance(ctx context.Context, sellerID, instanceID, price int64) (int64, error) {
	return 0, ErrInstanceListingDisabled
}

func (s *Service) ListInstanceVariants(ctx context.Context, sellerID int64, itemID string, price int64, items []store.InstanceMarketSelection, key string) (int64, error) {
	return s.store.CreateInstanceMarketListing(ctx, sellerID, itemID, price, items, key, s.assertMarketPrice)
}

func (s *Service) CreateAuction(
	ctx context.Context,
	sellerID int64,
	instanceID int64,
	startPrice int64,
	buyoutPrice int64,
	minBidIncrement int64,
	durationSeconds int64,
) (int64, error) {
	if instanceID > 0 {
		templateItemID, err := s.store.InstanceTemplateItemID(ctx, instanceID)
		if err != nil {
			return 0, err
		}
		if err := s.assertItemListable(templateItemID); err != nil {
			return 0, err
		}
		return s.store.CreateAuction(ctx, sellerID, instanceID, startPrice, buyoutPrice, minBidIncrement, durationSeconds)
	}
	return 0, store.ErrAuctionStackItemNotAllowed
}

func (s *Service) Buy(ctx context.Context, buyerID, listingID, quantity int64) error {
	_, err := s.store.BuyListing(ctx, buyerID, listingID, quantity, "")
	return err
}

func (s *Service) BuyWithIdempotency(ctx context.Context, buyerID, listingID, quantity int64, idempotencyKey string) (store.BuyListingResult, error) {
	result, err := s.store.BuyListing(ctx, buyerID, listingID, quantity, idempotencyKey)
	if err != nil {
		return result, err
	}
	if result.Created {
		_ = s.mq.PublishJSON(ctx, "market.bought", map[string]any{
			"orderId":        result.OrderID,
			"listingId":      listingID,
			"buyerId":        buyerID,
			"quantity":       quantity,
			"idempotencyKey": idempotencyKey,
			"feeRateBps":     result.FeeRateBps,
			"feeAmount":      result.FeeAmount,
			"sellerAmount":   result.SellerAmount,
			"ts":             time.Now().UnixMilli(),
		})
	}
	return result, nil
}

func (s *Service) CancelListing(ctx context.Context, sellerID, listingID int64) error {
	return s.store.CancelListing(ctx, sellerID, listingID)
}

func int64Ptr(value int64) *int64 {
	return &value
}
