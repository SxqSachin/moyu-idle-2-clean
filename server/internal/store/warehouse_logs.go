package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

type WarehouseChangeLogWrite struct {
	UserID        int64
	EventType     WarehouseChangeEvent
	Category      InventoryAuditCategory
	Direction     InventoryAuditDirection
	SubjectType   string
	SubjectID     string
	InstanceID    *int64
	TemplateItem  string
	Quantity      int64
	Items         map[string]int64
	InstanceIDs   []int64
	SourceSurface string
	TargetSurface string
	Details       map[string]any
}

func (s *Store) insertWarehouseChangeLogTx(ctx context.Context, tx pgx.Tx, item WarehouseChangeLogWrite) error {
	if item.Quantity <= 0 {
		item.Quantity = 1
	}
	details := item.Details
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	event := normalizeInventoryAuditEvent(InventoryAuditEvent{
		UserID:        item.UserID,
		Category:      item.Category,
		Direction:     item.Direction,
		SubjectType:   item.SubjectType,
		SubjectID:     item.SubjectID,
		EventType:     item.EventType,
		InstanceID:    item.InstanceID,
		TemplateItem:  item.TemplateItem,
		Quantity:      item.Quantity,
		Items:         item.Items,
		InstanceIDs:   item.InstanceIDs,
		SourceSurface: item.SourceSurface,
		TargetSurface: item.TargetSurface,
		Details:       item.Details,
	})
	itemsRaw, err := json.Marshal(event.Items)
	if err != nil {
		return err
	}
	instanceRaw, err := json.Marshal(event.InstanceIDs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO player_warehouse_change_logs(
			user_id, event_type, item_instance_id, template_item_id, quantity,
			source_surface, target_surface, details,
			category, direction, subject_type, subject_id, item_totals, instance_ids,
			created_at
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13::jsonb, $14::jsonb, now())
	`, event.UserID, string(event.EventType), event.InstanceID, event.TemplateItem, event.Quantity, event.SourceSurface, event.TargetSurface, string(raw),
		string(event.Category), string(event.Direction), event.SubjectType, event.SubjectID, string(itemsRaw), string(instanceRaw))
	return err
}

func (s *Store) ListWarehouseChangeLogs(ctx context.Context, userID int64, filter WarehouseChangeLogFilter) ([]WarehouseChangeLog, time.Time, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	since := filter.Since
	if since.IsZero() {
		since = time.Now().Add(-s.config.WarehouseLogDefaultLookback)
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, event_type, item_instance_id, template_item_id, quantity,
		       source_surface, target_surface, details,
		       category, direction, subject_type, subject_id, item_totals, instance_ids,
		       created_at
		FROM player_warehouse_change_logs
		WHERE user_id = $1
		  AND created_at >= $2
		  AND ($3::text = '' OR event_type = $3)
		  AND ($4::text = '' OR category = $4)
		ORDER BY created_at DESC, id DESC
		LIMIT $5
	`, userID, since, string(filter.EventType), string(filter.Category), limit)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	out := make([]WarehouseChangeLog, 0, limit)
	for rows.Next() {
		var item WarehouseChangeLog
		var raw []byte
		var itemsRaw []byte
		var instanceRaw []byte
		var eventType string
		var category string
		var direction string
		var instanceID *int64
		item.WarehouseID = 1
		if err := rows.Scan(&item.ID, &eventType, &instanceID, &item.TemplateItem, &item.Quantity,
			&item.SourceSurface, &item.TargetSurface, &raw,
			&category, &direction, &item.SubjectType, &item.SubjectID, &itemsRaw, &instanceRaw,
			&item.CreatedAt); err != nil {
			return nil, time.Time{}, err
		}
		if instanceID != nil {
			item.InstanceID = *instanceID
		}
		item.EventType = WarehouseChangeEvent(eventType)
		item.Category = InventoryAuditCategory(category)
		item.Direction = InventoryAuditDirection(direction)
		_ = json.Unmarshal(raw, &item.Details)
		_ = json.Unmarshal(itemsRaw, &item.Items)
		_ = json.Unmarshal(instanceRaw, &item.InstanceIDs)
		if len(item.Items) == 0 && item.TemplateItem != "" {
			item.Items = map[string]int64{item.TemplateItem: item.Quantity}
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, time.Time{}, err
	}
	return out, since, nil
}
