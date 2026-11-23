package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrDestructiveRecycleChanged = errors.New("destructive recycle quote changed")

// Prices stay within the exact integer range of the browser confirmation UI.
const destructiveRecycleMaxGold int64 = 1<<53 - 1
const destructiveRecycleFeePercent = 10

type DestructiveRecycleInput struct {
	InstanceID int64  `json:"instanceId"`
	QuoteToken string `json:"quoteToken,omitempty"`
}

type DestructiveRecycleQuoteItem struct {
	InstanceID     int64                `json:"instanceId"`
	TemplateItemID string               `json:"templateItemId"`
	EnhanceLevel   int                  `json:"enhanceLevel"`
	Affixes        []EquipmentAffixSlot `json:"affixes"`
	BodyGold       int64                `json:"bodyGold"`
	DisposalFee    int64                `json:"disposalFee"`
	TotalGold      int64                `json:"totalGold"`
	QuoteToken     string               `json:"quoteToken"`
}

type DestructiveRecycleQuote struct {
	Items       []DestructiveRecycleQuoteItem `json:"items"`
	FeePercent  int                           `json:"feePercent"`
	BodyGold    int64                         `json:"bodyGold"`
	DisposalFee int64                         `json:"disposalFee"`
	TotalGold   int64                         `json:"totalGold"`
}

func normalizeDestructiveRecycleItems(items []DestructiveRecycleInput, confirm bool) ([]RecycleBatchItemInput, map[int64]string, error) {
	if len(items) == 0 || len(items) > RecycleBatchItemLimit {
		return nil, nil, ErrBadQuantity
	}
	inputs := make([]RecycleBatchItemInput, 0, len(items))
	tokens := make(map[int64]string, len(items))
	for _, item := range items {
		if _, duplicate := tokens[item.InstanceID]; duplicate {
			return nil, nil, ErrBadQuantity
		}
		if confirm {
			if len(item.QuoteToken) != sha256.Size*2 {
				return nil, nil, ErrDestructiveRecycleChanged
			}
			raw, err := hex.DecodeString(item.QuoteToken)
			if err != nil || len(raw) != sha256.Size {
				return nil, nil, ErrDestructiveRecycleChanged
			}
		}
		tokens[item.InstanceID] = item.QuoteToken
		inputs = append(inputs, RecycleBatchItemInput{InstanceID: item.InstanceID, Quantity: 1})
	}
	inputs, err := normalizeRecycleBatchItems(inputs)
	return inputs, tokens, err
}

func destructiveRecycleNetGold(bodyGold int64) (fee, net int64, err error) {
	if bodyGold <= 0 || bodyGold > destructiveRecycleMaxGold {
		return 0, 0, ErrRecyclePriceInvalid
	}
	// Round the fee up per piece, so splitting batches cannot reduce it.
	fee = bodyGold/100*destructiveRecycleFeePercent + (bodyGold%100*destructiveRecycleFeePercent+99)/100
	net = bodyGold - fee
	if net <= 0 {
		return 0, 0, ErrRecyclePriceInvalid
	}
	return fee, net, nil
}

func (s *Store) destructiveRecycleBody(row LockedItemInstance) (LockedItemInstance, []EquipmentAffixSlot, error) {
	template := s.gameData.EquipmentTemplateByID(row.TemplateItem)
	item := s.gameData.ItemByID(row.TemplateItem)
	if template == nil || strings.TrimSpace(template.Slot) == "" || item == nil || item.Stackable || row.Quantity != 1 {
		return row, nil, ErrItemNotRecyclable
	}
	// Never redirect an already recyclable item to the more expensive path.
	if s.lockedInstanceRecycleTemplateMatchKind(row) != recycleTemplateMatchNone {
		return row, nil, ErrItemNotRecyclable
	}
	slots, ok := decodeAffixSlotsForTemplateConsistency(row.AffixesRaw)
	if !ok || row.EnhanceLevel < 0 || (len(slots) == 0 && row.EnhanceLevel == 0) {
		return row, nil, ErrItemNotRecyclable
	}
	seen := make(map[int]bool, len(slots))
	for _, slot := range slots {
		if !isValidAffixSlotIndex(slot.SlotIndex, template.AffixSlotCount) || strings.TrimSpace(slot.AffixID) == "" || strings.TrimSpace(slot.RarityID) == "" || seen[slot.SlotIndex] {
			return row, nil, ErrItemNotRecyclable
		}
		seen[slot.SlotIndex] = true
	}
	// Base snapshots may contain enhancement and effective affix modifiers. Accept the original
	// template snapshot or the existing snapshot-builder result, never arbitrary
	// altered base attributes. Only the in-memory pricing copy loses upgrades.
	expected := make(map[string]float64, len(template.BaseAttrs))
	for key, value := range template.BaseAttrs {
		expected[key] = value
	}
	if row.EnhanceLevel > 0 {
		runtime, exists := s.enhanceCache.Template(row.TemplateItem)
		if !exists || row.EnhanceLevel > runtime.MaxLevel {
			return row, nil, ErrItemNotRecyclable
		}
		for key, value := range s.enhanceCache.CumulativeAttrs(row.TemplateItem, row.EnhanceLevel) {
			if key = strings.TrimSpace(key); key != "" {
				expected[key] += value
			}
		}
	}
	for _, slot := range FilterEffectiveAffixSlotsByLimit(slots, s.gameData, nil) {
		for key, value := range slot.AttrMods {
			if key = strings.TrimSpace(key); key != "" {
				expected[key] += value
			}
		}
	}
	if !baseSnapshotMatchesTemplateDefault(template.BaseAttrs, row.BaseSnapshot) && !baseSnapshotMatchesTemplateDefault(expected, row.BaseSnapshot) {
		return row, nil, ErrItemNotRecyclable
	}
	body := row
	body.EnhanceLevel = 0
	body.AffixesRaw = []byte(`[]`)
	var err error
	body.BaseSnapshot, err = json.Marshal(template.BaseAttrs)
	if err != nil || !s.lockedInstanceMatchesTemplateRecycleBase(body) {
		return row, nil, ErrItemNotRecyclable
	}
	return body, slots, nil
}

func (s *Store) prepareDestructiveRecycle(userID int64, inputs []RecycleBatchItemInput, tokens map[int64]string, locked map[int64]LockedItemInstance, confirm bool) ([]preparedRecycleBatchItem, DestructiveRecycleQuote, error) {
	out := DestructiveRecycleQuote{Items: make([]DestructiveRecycleQuoteItem, 0, len(inputs)), FeePercent: destructiveRecycleFeePercent}
	prepared := make([]preparedRecycleBatchItem, 0, len(inputs))
	for _, input := range inputs {
		row, ok := locked[input.InstanceID]
		if !ok {
			return nil, out, pgx.ErrNoRows
		}
		if row.UserID != userID {
			return nil, out, ErrInstanceNotOwned
		}
		if row.State != "inventory" {
			return nil, out, ErrInvalidInstanceState
		}
		body, affixes, err := s.destructiveRecycleBody(row)
		if err != nil {
			return nil, out, err
		}
		bodyPrepared, _, err := s.prepareRecycleBatchItems(userID, []RecycleBatchItemInput{input}, map[int64]LockedItemInstance{input.InstanceID: body})
		if err != nil {
			return nil, out, err
		}
		p := bodyPrepared[0]
		fee, net, err := destructiveRecycleNetGold(p.TotalGold)
		if err != nil {
			return nil, out, err
		}
		q := DestructiveRecycleQuoteItem{InstanceID: row.InstanceID, TemplateItemID: row.TemplateItem, EnhanceLevel: row.EnhanceLevel, Affixes: affixes, BodyGold: p.TotalGold, DisposalFee: fee, TotalGold: net}
		// This is a state fingerprint, not a price supplied by the client. All
		// authorization, eligibility and prices are recomputed under row locks.
		payload, err := json.Marshal(struct {
			Version string
			Row     LockedItemInstance
			Quote   DestructiveRecycleQuoteItem
		}{"destructive-recycle-v2", row, q})
		if err != nil {
			return nil, out, err
		}
		digest := sha256.Sum256(payload)
		q.QuoteToken = hex.EncodeToString(digest[:])
		if confirm && tokens[row.InstanceID] != q.QuoteToken {
			return nil, out, ErrDestructiveRecycleChanged
		}
		if q.BodyGold > destructiveRecycleMaxGold-out.BodyGold {
			return nil, out, ErrRecyclePriceInvalid
		}
		out.BodyGold += q.BodyGold
		out.DisposalFee += fee
		out.TotalGold += net
		out.Items = append(out.Items, q)
		p.Row = row
		p.UnitPrice, p.TotalGold = net, net
		prepared = append(prepared, p)
	}
	return prepared, out, nil
}

func (s *Store) ensureDestructiveRecycleUnusedTx(ctx context.Context, tx pgx.Tx, userID int64, ids []int64) error {
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, ids...); err != nil {
		return err
	}
	var used bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM player_equip_slots WHERE instance_id = ANY($1)
		UNION ALL SELECT 1 FROM player_kitty_equip_slots WHERE instance_id = ANY($1)
	)`, ids).Scan(&used); err != nil {
		return err
	}
	if used {
		return ErrInvalidInstanceState
	}
	return nil
}

func (s *Store) QuoteDestructiveRecycle(ctx context.Context, userID int64, items []DestructiveRecycleInput) (DestructiveRecycleQuote, error) {
	return s.destructiveRecycle(ctx, userID, items, false)
}

func (s *Store) DestructiveRecycle(ctx context.Context, userID int64, items []DestructiveRecycleInput) (DestructiveRecycleQuote, error) {
	return s.destructiveRecycle(ctx, userID, items, true)
}

func (s *Store) destructiveRecycle(ctx context.Context, userID int64, items []DestructiveRecycleInput, confirm bool) (DestructiveRecycleQuote, error) {
	inputs, tokens, err := normalizeDestructiveRecycleItems(items, confirm)
	if err != nil {
		return DestructiveRecycleQuote{}, err
	}
	var out DestructiveRecycleQuote
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		ids := recycleBatchInstanceIDs(inputs)
		locked, err := s.lockRecycleBatchInstancesTx(ctx, tx, ids)
		if err != nil {
			return err
		}
		prepared, quote, err := s.prepareDestructiveRecycle(userID, inputs, tokens, locked, confirm)
		if err != nil {
			return err
		}
		if err := s.ensureDestructiveRecycleUnusedTx(ctx, tx, userID, ids); err != nil {
			return err
		}
		out = quote
		if !confirm {
			return nil
		}
		if err := s.applyRecycleBatchConsumptionTx(ctx, tx, userID, prepared); err != nil {
			return err
		}
		itemTotals := make(map[string]int64)
		operationTokens := make([]string, 0, len(quote.Items))
		for _, q := range quote.Items {
			itemTotals[q.TemplateItemID]++
			operationTokens = append(operationTokens, q.QuoteToken)
		}
		operationDigest := sha256.Sum256([]byte(strings.Join(operationTokens, ":")))
		operationKey := "destructive-recycle:" + hex.EncodeToString(operationDigest[:])
		// Aggregate once: the ordinary audit merge retains only the first value
		// for a repeated detail key and would otherwise lose later affix records.
		s.recordInventoryAuditTx(tx, InventoryAuditEvent{
			UserID: userID, EventType: WarehouseChangeEventWarehouseOp,
			SourceSurface: "inventory", TargetSurface: "recycle", Items: itemTotals, InstanceIDs: ids,
			Details: map[string]any{"destructiveRecycle": true, "operationKey": operationKey, "gold": quote.TotalGold, "bodyGold": quote.BodyGold, "disposalFee": quote.DisposalFee, "destroyedEquipment": quote.Items},
		})
		return s.LedgerGrantTemplateTx(ctx, tx, userID, GoldCoinItemID, quote.TotalGold, InventoryLedgerMeta{
			SourceSurface: "recycle", TargetSurface: "inventory",
			Details: map[string]any{"destructiveRecycle": true, "operationKey": operationKey, "itemCount": len(quote.Items), "disposalFee": quote.DisposalFee},
		})
	})
	if err != nil {
		return DestructiveRecycleQuote{}, err
	}
	return out, nil
}
