package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type InventoryAuditCategory string

const (
	InventoryAuditCategoryActionQueue     InventoryAuditCategory = "action_queue"
	InventoryAuditCategoryBattle          InventoryAuditCategory = "battle"
	InventoryAuditCategoryMarketTrade     InventoryAuditCategory = "market_trade"
	InventoryAuditCategoryAuction         InventoryAuditCategory = "auction"
	InventoryAuditCategoryShop            InventoryAuditCategory = "shop"
	InventoryAuditCategoryAdmin           InventoryAuditCategory = "admin"
	InventoryAuditCategoryCatVillage      InventoryAuditCategory = "cat_village_commission"
	InventoryAuditCategoryAlchemy         InventoryAuditCategory = "alchemy"
	InventoryAuditCategoryAffix           InventoryAuditCategory = "affix"
	InventoryAuditCategoryRecycle         InventoryAuditCategory = "recycle"
	InventoryAuditCategoryKitty           InventoryAuditCategory = "kitty"
	InventoryAuditCategoryBattleItemBag   InventoryAuditCategory = "battle_item_bag"
	InventoryAuditCategoryOperationEffect InventoryAuditCategory = "operation_effect"
	InventoryAuditCategorySystem          InventoryAuditCategory = "system"
)

type InventoryAuditDirection string

const (
	InventoryAuditDirectionGain     InventoryAuditDirection = "gain"
	InventoryAuditDirectionConsume  InventoryAuditDirection = "consume"
	InventoryAuditDirectionTransfer InventoryAuditDirection = "transfer"
	InventoryAuditDirectionRefund   InventoryAuditDirection = "refund"
	InventoryAuditDirectionFee      InventoryAuditDirection = "fee"
	InventoryAuditDirectionLock     InventoryAuditDirection = "lock"
	InventoryAuditDirectionUnlock   InventoryAuditDirection = "unlock"
)

type InventoryAuditEvent struct {
	UserID        int64
	Category      InventoryAuditCategory
	Direction     InventoryAuditDirection
	SubjectType   string
	SubjectID     string
	Items         map[string]int64
	InstanceIDs   []int64
	EventType     WarehouseChangeEvent
	InstanceID    *int64
	TemplateItem  string
	Quantity      int64
	SourceSurface string
	TargetSurface string
	Details       map[string]any
}

type inventoryAuditBuffer struct {
	events        map[string]*InventoryAuditEvent
	lockedUserIDs map[int64]struct{}
}

func (s *Store) registerInventoryAuditBuffer(tx pgx.Tx) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	if s.auditBuffers == nil {
		s.auditBuffers = make(map[pgx.Tx]*inventoryAuditBuffer)
	}
	s.auditBuffers[tx] = &inventoryAuditBuffer{
		events:        make(map[string]*InventoryAuditEvent),
		lockedUserIDs: make(map[int64]struct{}),
	}
}

func (s *Store) discardInventoryAuditBuffer(tx pgx.Tx) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	delete(s.auditBuffers, tx)
}

func (s *Store) flushInventoryAuditBuffer(ctx context.Context, tx pgx.Tx) {
	events := s.takeInventoryAuditEvents(tx)
	if len(events) == 0 {
		return
	}
	startedAt := time.Now()
	source := auditSourceFromContext(ctx)
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.writeInventoryAuditEvents(flushCtx, events); err != nil {
		log.Printf(
			"inventory audit flush slow source=%s events=%d categories=%s elapsed_ms=%d err=%v",
			source,
			len(events),
			inventoryAuditCategorySummary(events),
			time.Since(startedAt).Milliseconds(),
			err,
		)
		return
	}
	elapsed := time.Since(startedAt)
	if elapsed >= 500*time.Millisecond {
		log.Printf(
			"inventory audit flush slow source=%s events=%d categories=%s elapsed_ms=%d err=nil",
			source,
			len(events),
			inventoryAuditCategorySummary(events),
			elapsed.Milliseconds(),
		)
	}
}

func (s *Store) takeInventoryAuditEvents(tx pgx.Tx) []InventoryAuditEvent {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	buf := s.auditBuffers[tx]
	delete(s.auditBuffers, tx)
	if buf == nil || len(buf.events) == 0 {
		return nil
	}
	keys := make([]string, 0, len(buf.events))
	for key := range buf.events {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]InventoryAuditEvent, 0, len(keys))
	for _, key := range keys {
		out = append(out, *buf.events[key])
	}
	return out
}

func (s *Store) recordInventoryAuditTx(tx pgx.Tx, event InventoryAuditEvent) {
	event = normalizeInventoryAuditEvent(event)
	if event.UserID <= 0 || len(event.Items) == 0 {
		return
	}
	key := inventoryAuditMergeKey(event)
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	buf := s.auditBuffers[tx]
	if buf == nil {
		return
	}
	existing := buf.events[key]
	if existing == nil {
		copied := event
		copied.Items = copyInt64Map(event.Items)
		copied.InstanceIDs = append([]int64(nil), event.InstanceIDs...)
		buf.events[key] = &copied
		return
	}
	for itemID, qty := range event.Items {
		if qty > 0 {
			existing.Items[itemID] += qty
		}
	}
	existing.InstanceIDs = mergeInstanceIDs(existing.InstanceIDs, event.InstanceIDs)
	existing.Quantity += event.Quantity
	if existing.Quantity <= 0 {
		existing.Quantity = primaryQuantity(existing.Items)
	}
	existing.Details = mergeAuditDetails(existing.Details, event.Details)
}

func (s *Store) writeInventoryAuditEvents(ctx context.Context, events []InventoryAuditEvent) error {
	// Keep individual rows, ordering and conflict handling, but amortize network
	// round trips after a settlement commit. Bound each pipeline and its memory.
	const batchSize = 128
	for offset := 0; offset < len(events); offset += batchSize {
		batch := &pgx.Batch{}
		for _, event := range events[offset:min(offset+batchSize, len(events))] {
			if event.UserID <= 0 || len(event.Items) == 0 {
				continue
			}
			args, err := inventoryAuditInsertArgs(event)
			if err != nil {
				return err
			}
			batch.Queue(inventoryAuditInsertSQL, args...)
		}
		if batch.Len() == 0 {
			continue
		}
		if err := s.db.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) insertInventoryAuditEvent(ctx context.Context, event InventoryAuditEvent) error {
	args, err := inventoryAuditInsertArgs(event)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, inventoryAuditInsertSQL, args...)
	return err
}

func inventoryAuditInsertArgs(event InventoryAuditEvent) ([]any, error) {
	itemsRaw, err := json.Marshal(event.Items)
	if err != nil {
		return nil, err
	}
	instanceRaw, err := json.Marshal(event.InstanceIDs)
	if err != nil {
		return nil, err
	}
	details := event.Details
	if details == nil {
		details = map[string]any{}
	}
	detailsRaw, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	auditKey := inventoryAuditIDKey(event)
	return []any{event.UserID, string(event.EventType), event.InstanceID, event.TemplateItem, event.Quantity,
		event.SourceSurface, event.TargetSurface, string(detailsRaw),
		string(event.Category), string(event.Direction), event.SubjectType, event.SubjectID, string(itemsRaw), string(instanceRaw), auditKey}, nil
}

const inventoryAuditInsertSQL = `
		INSERT INTO player_warehouse_change_logs(
			user_id, event_type, item_instance_id, template_item_id, quantity,
			source_surface, target_surface, details,
			category, direction, subject_type, subject_id, item_totals, instance_ids, audit_key,
			created_at
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13::jsonb, $14::jsonb, $15, now())
		ON CONFLICT (audit_key) WHERE audit_key <> '' DO UPDATE
		SET quantity = EXCLUDED.quantity,
		    details = EXCLUDED.details,
		    item_totals = EXCLUDED.item_totals,
		    instance_ids = EXCLUDED.instance_ids
	`

func normalizeInventoryAuditEvent(event InventoryAuditEvent) InventoryAuditEvent {
	if event.Items == nil {
		event.Items = map[string]int64{}
	}
	if strings.TrimSpace(event.TemplateItem) != "" && event.Quantity > 0 {
		event.Items[strings.TrimSpace(event.TemplateItem)] += event.Quantity
	}
	for itemID, qty := range event.Items {
		if strings.TrimSpace(itemID) == "" || qty <= 0 {
			delete(event.Items, itemID)
		}
	}
	if event.Category == "" {
		event.Category = inferInventoryAuditCategory(event.SourceSurface, event.TargetSurface, event.Details)
	}
	if event.Direction == "" {
		event.Direction = inferInventoryAuditDirection(event.EventType, event.SourceSurface, event.TargetSurface, event.Details)
	}
	if event.EventType == "" {
		if event.Direction == InventoryAuditDirectionGain || event.Direction == InventoryAuditDirectionRefund || event.Direction == InventoryAuditDirectionUnlock {
			event.EventType = WarehouseChangeEventSystemToWarehouse
		} else {
			event.EventType = WarehouseChangeEventWarehouseOp
		}
	}
	if event.TemplateItem == "" {
		event.TemplateItem = primaryItemID(event.Items)
	}
	if event.Quantity <= 0 {
		event.Quantity = primaryQuantity(event.Items)
	}
	if event.SourceSurface == "" {
		event.SourceSurface = string(event.Category)
	}
	if event.TargetSurface == "" {
		event.TargetSurface = "inventory"
	}
	if event.SubjectType == "" || event.SubjectID == "" {
		subjectType, subjectID := inferInventoryAuditSubject(event.Category, event.Details)
		if event.SubjectType == "" {
			event.SubjectType = subjectType
		}
		if event.SubjectID == "" {
			event.SubjectID = subjectID
		}
	}
	if event.InstanceID != nil {
		event.InstanceIDs = mergeInstanceIDs(event.InstanceIDs, []int64{*event.InstanceID})
	}
	sort.Slice(event.InstanceIDs, func(i, j int) bool { return event.InstanceIDs[i] < event.InstanceIDs[j] })
	return event
}

func inferInventoryAuditCategory(source, target string, details map[string]any) InventoryAuditCategory {
	text := strings.ToLower(strings.TrimSpace(source + " " + target + " " + detailString(details, "type")))
	switch {
	case strings.Contains(text, "alchemy"):
		return InventoryAuditCategoryAlchemy
	case strings.Contains(text, "operation_effect"):
		return InventoryAuditCategoryOperationEffect
	case strings.Contains(text, "action") || strings.Contains(text, "offline_period"):
		return InventoryAuditCategoryActionQueue
	case strings.Contains(text, "battle"):
		return InventoryAuditCategoryBattle
	case strings.Contains(text, "auction"):
		return InventoryAuditCategoryAuction
	case strings.Contains(text, "market"):
		return InventoryAuditCategoryMarketTrade
	case strings.Contains(text, "shop"):
		return InventoryAuditCategoryShop
	case strings.Contains(text, "admin"):
		return InventoryAuditCategoryAdmin
	case strings.Contains(text, "cat_village"):
		return InventoryAuditCategoryCatVillage
	case strings.Contains(text, "affix") || strings.Contains(text, "enchant"):
		return InventoryAuditCategoryAffix
	case strings.Contains(text, "recycle"):
		return InventoryAuditCategoryRecycle
	case strings.Contains(text, "kitty") || strings.Contains(text, "commission"):
		return InventoryAuditCategoryKitty
	default:
		return InventoryAuditCategorySystem
	}
}

func inferInventoryAuditDirection(eventType WarehouseChangeEvent, source, target string, details map[string]any) InventoryAuditDirection {
	text := strings.ToLower(strings.TrimSpace(source + " " + target + " " + detailString(details, "type")))
	switch {
	case strings.Contains(text, "refund") || strings.Contains(text, "return"):
		return InventoryAuditDirectionRefund
	case strings.Contains(text, "fee"):
		return InventoryAuditDirectionFee
	case strings.Contains(text, "lock") || strings.Contains(target, "market") || strings.Contains(target, "auction"):
		return InventoryAuditDirectionLock
	case strings.Contains(text, "unlock"):
		return InventoryAuditDirectionUnlock
	case eventType == WarehouseChangeEventSystemToWarehouse || strings.TrimSpace(target) == "inventory":
		return InventoryAuditDirectionGain
	default:
		return InventoryAuditDirectionConsume
	}
}

func inferInventoryAuditSubject(category InventoryAuditCategory, details map[string]any) (string, string) {
	switch category {
	case InventoryAuditCategoryActionQueue:
		return "queue", detailID(details, "queueId")
	case InventoryAuditCategoryBattle:
		return "battle_room", detailID(details, "roomId")
	case InventoryAuditCategoryMarketTrade:
		if id := detailID(details, "orderId"); id != "" {
			return "market_order", id
		}
		return "market_listing", detailID(details, "listingId")
	case InventoryAuditCategoryAuction:
		return "auction", detailID(details, "auctionId")
	case InventoryAuditCategoryShop:
		return "shop_order", detailID(details, "orderId")
	case InventoryAuditCategoryAlchemy:
		return "alchemy", detailID(details, "queueId")
	default:
		return string(category), ""
	}
}

func inventoryAuditMergeKey(event InventoryAuditEvent) string {
	return strings.Join([]string{
		strconv.FormatInt(event.UserID, 10),
		string(event.Category),
		string(event.Direction),
		event.SubjectType,
		event.SubjectID,
		detailString(event.Details, "phase"),
		detailString(event.Details, "runId"),
		detailString(event.Details, "grantId"),
		detailString(event.Details, "operationKey"),
		detailString(event.Details, "operationRangeKey"),
	}, "|")
}

func inventoryAuditIDKey(event InventoryAuditEvent) string {
	event = normalizeInventoryAuditEvent(event)
	hash := sha1.Sum([]byte(strings.Join([]string{
		inventoryAuditMergeKey(event),
		canonicalItems(event.Items),
		canonicalInstanceIDs(event.InstanceIDs),
	}, "|")))
	return hex.EncodeToString(hash[:])
}

func canonicalItems(items map[string]int64) string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, items[key]))
	}
	return strings.Join(parts, ",")
}

func canonicalInstanceIDs(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	copied := append([]int64(nil), ids...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	parts := make([]string, 0, len(copied))
	for _, id := range copied {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

func detailString(details map[string]any, key string) string {
	if details == nil {
		return ""
	}
	value, ok := details[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func detailID(details map[string]any, key string) string {
	return detailString(details, key)
}

func primaryItemID(items map[string]int64) string {
	keys := make([]string, 0, len(items))
	for key, qty := range items {
		if qty > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func primaryQuantity(items map[string]int64) int64 {
	itemID := primaryItemID(items)
	if itemID == "" {
		return 1
	}
	return items[itemID]
}

func copyInt64Map(in map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeInstanceIDs(a, b []int64) []int64 {
	if len(b) == 0 {
		return a
	}
	seen := make(map[int64]struct{}, len(a)+len(b))
	out := make([]int64, 0, len(a)+len(b))
	for _, id := range a {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range b {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Store) GetBattleItemGainsSince(ctx context.Context, userID int64, since, until time.Time) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT entry.key, SUM(entry.value::bigint) AS total_qty
		FROM player_warehouse_change_logs
		CROSS JOIN LATERAL jsonb_each_text(item_totals) AS entry(key, value)
		WHERE user_id = $1
		  AND category = 'battle'
		  AND direction = 'gain'
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY entry.key
	`, userID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var itemID string
		var qty int64
		if err := rows.Scan(&itemID, &qty); err != nil {
			return nil, err
		}
		itemID = strings.TrimSpace(itemID)
		if itemID != "" && qty > 0 {
			result[itemID] = qty
		}
	}
	return result, rows.Err()
}

func mergeAuditDetails(a, b map[string]any) map[string]any {
	if len(a) == 0 {
		return b
	}
	for key, value := range b {
		if _, exists := a[key]; !exists {
			a[key] = value
		}
	}
	return a
}
