package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListBattleItemBag(ctx context.Context, userID int64) ([]BattleItemBagEntry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT slot_no, item_id, quantity, updated_at
		FROM player_battle_item_loadout
		WHERE user_id = $1
		ORDER BY slot_no ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleItemBagEntry, 0, 8)
	for rows.Next() {
		var row BattleItemBagEntry
		if err = rows.Scan(&row.SlotNo, &row.ItemID, &row.Quantity, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) AddBattleItemBagQuantity(
	ctx context.Context,
	userID int64,
	slotNo int,
	itemID string,
	qty int64,
	maxKinds int64,
	maxTotalQuantity int64,
) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return ErrInvalidItemID
	}
	if qty <= 0 {
		return ErrBadQuantity
	}
	if maxKinds <= 0 {
		maxKinds = 4
	}
	if slotNo <= 0 || int64(slotNo) > maxKinds {
		return ErrBattleItemBagSlotInvalid
	}
	if maxTotalQuantity <= 0 {
		maxTotalQuantity = 1000
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.ensureBattleItemBagMutableTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			SELECT 1
			FROM player_battle_item_loadout
			WHERE user_id = $1
			FOR UPDATE
		`, userID); err != nil {
			return err
		}
		var (
			totalQuantity int64
			kindCount     int64
		)
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(quantity), 0), COUNT(*) FILTER (WHERE quantity > 0)
			FROM player_battle_item_loadout
			WHERE user_id = $1
		`, userID).Scan(&totalQuantity, &kindCount); err != nil {
			return err
		}
		var currentSlotItemID string
		slotErr := tx.QueryRow(ctx, `
			SELECT item_id
			FROM player_battle_item_loadout
			WHERE user_id = $1 AND slot_no = $2
			FOR UPDATE
		`, userID, slotNo).Scan(&currentSlotItemID)
		if slotErr != nil && !errors.Is(slotErr, pgx.ErrNoRows) {
			return slotErr
		}
		if slotErr == nil && strings.TrimSpace(currentSlotItemID) != itemID {
			return ErrBattleItemBagSlotOccupied
		}
		var currentQuantity int64
		itemErr := tx.QueryRow(ctx, `
			SELECT quantity
			FROM player_battle_item_loadout
			WHERE user_id = $1 AND item_id = $2
			FOR UPDATE
		`, userID, itemID).Scan(&currentQuantity)
		if itemErr != nil && !errors.Is(itemErr, pgx.ErrNoRows) {
			return itemErr
		}
		if errors.Is(itemErr, pgx.ErrNoRows) && kindCount >= maxKinds {
			return ErrBattleItemBagKindsLimitExceeded
		}
		if totalQuantity+qty > maxTotalQuantity {
			return ErrBattleItemBagTotalLimitExceeded
		}
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryBattleItemBag,
			Direction:     InventoryAuditDirectionConsume,
			SubjectType:   "battle_item_bag",
			SubjectID:     itemID,
			SourceSurface: "inventory",
			TargetSurface: "battle_item_bag",
			Details: map[string]any{
				"type": "battle_item_bag_add",
			},
		}); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO player_battle_item_loadout(user_id, slot_no, item_id, quantity, updated_at)
			VALUES($1, $2, $3, $4, now())
			ON CONFLICT (user_id, item_id)
			DO UPDATE SET slot_no = EXCLUDED.slot_no, quantity = player_battle_item_loadout.quantity + EXCLUDED.quantity, updated_at = now()
		`, userID, slotNo, itemID, qty)
		return err
	})
}

func (s *Store) TakeBackBattleItemBagSlot(ctx context.Context, userID int64, slotNo int, maxKinds int64) error {
	if maxKinds <= 0 {
		maxKinds = 4
	}
	if slotNo <= 0 || int64(slotNo) > maxKinds {
		return ErrBattleItemBagSlotInvalid
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.ensureBattleItemBagMutableTx(ctx, tx, userID); err != nil {
			return err
		}
		var (
			itemID   string
			quantity int64
		)
		if err := tx.QueryRow(ctx, `
			SELECT item_id, quantity
			FROM player_battle_item_loadout
			WHERE user_id = $1 AND slot_no = $2
			FOR UPDATE
		`, userID, slotNo).Scan(&itemID, &quantity); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleItemBagSlotNotFound
			}
			return err
		}
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			return ErrBattleItemBagItemNotFound
		}
		quantity = maxInt64(0, quantity)
		if quantity > 0 {
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, quantity, InventoryLedgerMeta{
				Category:      InventoryAuditCategoryBattleItemBag,
				Direction:     InventoryAuditDirectionRefund,
				SubjectType:   "battle_item_bag",
				SubjectID:     itemID,
				SourceSurface: "battle_item_bag",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type": "battle_item_bag_take_back",
				},
			}); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM player_battle_item_loadout
			WHERE user_id = $1 AND slot_no = $2
		`, userID, slotNo)
		return err
	})
}

func (s *Store) ensureBattleItemBagMutableTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	var roomStatus string
	err := tx.QueryRow(ctx, `
		SELECT br.status
		FROM battle_room_members m
		JOIN battle_rooms br ON br.id = m.room_id
		WHERE m.user_id = $1
		ORDER BY m.joined_at DESC
		LIMIT 1
	`, userID).Scan(&roomStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	status := strings.ToLower(strings.TrimSpace(roomStatus))
	if status == "entering" || status == "playing" || status == "recovering" {
		return ErrBattleItemBagLocked
	}
	return nil
}

func (s *Store) SetBattleItemBagQuantity(ctx context.Context, userID int64, itemID string, qty int64) error {
	return s.SetBattleItemBagQuantities(ctx, []BattleItemBagQuantityUpdate{{
		UserID:   userID,
		ItemID:   itemID,
		Quantity: qty,
	}})
}

func (s *Store) SetBattleItemBagQuantities(ctx context.Context, updates []BattleItemBagQuantityUpdate) error {
	normalized, err := normalizeBattleItemBagQuantityUpdates(updates)
	if err != nil || len(normalized) == 0 {
		return err
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.setBattleItemBagQuantitiesTx(ctx, tx, normalized)
	})
}

func normalizeBattleItemBagQuantityUpdates(updates []BattleItemBagQuantityUpdate) ([]BattleItemBagQuantityUpdate, error) {
	if len(updates) == 0 {
		return nil, nil
	}
	normalized := make([]BattleItemBagQuantityUpdate, len(updates))
	for idx, update := range updates {
		update.ItemID = strings.TrimSpace(update.ItemID)
		if update.UserID <= 0 || update.ItemID == "" {
			return nil, ErrInvalidItemID
		}
		if update.Quantity < 0 {
			return nil, ErrBadQuantity
		}
		normalized[idx] = update
	}
	return normalized, nil
}

func (s *Store) setBattleItemBagQuantitiesTx(ctx context.Context, tx pgx.Tx, updates []BattleItemBagQuantityUpdate) error {
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
					UPDATE player_battle_item_loadout
					SET quantity = $3, updated_at = now()
					WHERE user_id = $1 AND item_id = $2
				`, update.UserID, update.ItemID, update.Quantity); err != nil {
			return err
		}
	}
	return nil
}
