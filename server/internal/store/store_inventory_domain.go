package store

import (
	"context"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

// GrantByTemplateTx is the unified template-item grant entry in a transaction.
func (s *Store) GrantByTemplateTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64) error {
	return s.AddInventoryItemTx(ctx, tx, userID, itemID, qty)
}

func (s *Store) GrantByTemplateWithOptionsTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, options InventoryGrantOptions) error {
	return s.AddInventoryItemWithOptionsTx(ctx, tx, userID, itemID, qty, options)
}

// ConsumeByTemplateTx is the unified template-item consume entry in a transaction.
func (s *Store) ConsumeByTemplateTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64) error {
	return s.RemoveInventoryItemTx(ctx, tx, userID, itemID, qty)
}

func normalizeTemplateItemIDs(itemIDs []string) []string {
	normalized := make([]string, 0, len(itemIDs))
	seen := map[string]struct{}{}
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		normalized = append(normalized, itemID)
	}
	return normalized
}

func (s *Store) lockTemplateCommodityRowsByTemplateIDsTx(ctx context.Context, tx pgx.Tx, userID int64, itemIDs []string) ([]LockedItemInstance, error) {
	return s.lockTemplateCommodityRowsByTemplateIDsQueryTx(ctx, tx, userID, itemIDs, false)
}

func (s *Store) lockUnarchivedTemplateCommodityRowsByTemplateIDsTx(ctx context.Context, tx pgx.Tx, userID int64, itemIDs []string) ([]LockedItemInstance, error) {
	return s.lockTemplateCommodityRowsByTemplateIDsQueryTx(ctx, tx, userID, itemIDs, true)
}

func (s *Store) lockTemplateCommodityRowsByTemplateIDsQueryTx(ctx context.Context, tx pgx.Tx, userID int64, itemIDs []string, unarchivedOnly bool) ([]LockedItemInstance, error) {
	normalized := normalizeTemplateItemIDs(itemIDs)
	if len(normalized) == 0 {
		return []LockedItemInstance{}, nil
	}
	query := `
		SELECT instance_id, user_id, template_item_id, COALESCE(custom_data, '{}'::jsonb), quantity, enhance_level, roll_seed,
		       base_snapshot, affixes, durability, max_durability, state, archived_at
		FROM player_item_instances
		WHERE user_id = $1
		  AND template_item_id = ANY($2)
		  AND state = 'inventory'
		  AND COALESCE(custom_data, '{}'::jsonb) = '{}'::jsonb
		  AND enhance_level = 0
	`
	if unarchivedOnly {
		query += `
		  AND archived_at IS NULL
		ORDER BY instance_id
		FOR UPDATE
	`
	} else {
		query += `
		ORDER BY (archived_at IS NOT NULL), instance_id
		FOR UPDATE
	`
	}
	rows, err := tx.Query(ctx, query, userID, normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LockedItemInstance, 0, 16)
	for rows.Next() {
		var row LockedItemInstance
		var customDataRaw []byte
		if err = rows.Scan(
			&row.InstanceID,
			&row.UserID,
			&row.TemplateItem,
			&customDataRaw,
			&row.Quantity,
			&row.EnhanceLevel,
			&row.RollSeed,
			&row.BaseSnapshot,
			&row.AffixesRaw,
			&row.Durability,
			&row.MaxDurability,
			&row.State,
			&row.ArchivedAt,
		); err != nil {
			return nil, err
		}
		row.CustomData = decodeItemCustomization(customDataRaw)
		if s.lockedInstanceIsTemplateCommodity(row) {
			out = append(out, row)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ConsumableQuantityByTemplateTx returns quantity that can be consumed by template-level flows.
// Only template commodity rows are included; custom instance rows are intentionally excluded.
func (s *Store) ConsumableQuantityByTemplateTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string) (int64, error) {
	rows, err := s.lockTemplateCommodityRowsByTemplateIDsTx(ctx, tx, userID, []string{itemID})
	if err != nil {
		return 0, err
	}
	var owned int64
	for _, row := range rows {
		if row.Quantity > math.MaxInt64-owned {
			return 0, ErrInventoryQuantityOverflow
		}
		owned += row.Quantity
	}
	return owned, nil
}
