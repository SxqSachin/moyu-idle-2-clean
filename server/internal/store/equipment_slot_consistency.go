package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

type validPlayerEquippedItemRow struct {
	InstanceID     int64
	TemplateItemID string
	EquipType      string
	AffixRaw       []byte
	EnhanceLevel   int
}

func equipmentAllowedSlots(template *catalog.EquipmentTemplate) []string {
	if template == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(template.OccupySlots)+1)
	for _, raw := range template.OccupySlots {
		slot := strings.TrimSpace(raw)
		if slot == "" {
			continue
		}
		if _, ok := seen[slot]; ok {
			continue
		}
		seen[slot] = struct{}{}
		out = append(out, slot)
	}
	if len(out) == 0 {
		slot := strings.TrimSpace(template.Slot)
		if slot != "" {
			out = append(out, slot)
		}
	}
	if len(out) == 1 && out[0] == "accessory" {
		out = []string{"accessory_1", "accessory_2", "accessory_3", "accessory_4"}
	}
	sort.Strings(out)
	return out
}

func equipmentTemplateAllowsEquipSlot(template *catalog.EquipmentTemplate, equipType string, slot string) bool {
	return equipmentTemplateAllowsEquipSlotWithProfile(template, equipType, slot, BasePlayerBattleSlotProfile())
}

func equipmentTemplateAllowsEquipSlotWithProfile(template *catalog.EquipmentTemplate, equipType string, slot string, battleProfile EquipmentSlotProfile) bool {
	if template == nil {
		return false
	}
	normalizedType := normalizeEquipType(equipType)
	if normalizeEquipType(template.EquipType) != normalizedType {
		return false
	}
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return false
	}
	if normalizedType == "battle" {
		definition, ok := battleProfile.Definition(slot)
		if !ok || !definition.Enabled {
			return false
		}
		if len(template.OccupySlots) > 0 {
			for _, occupied := range template.OccupySlots {
				if strings.TrimSpace(occupied) == slot {
					return true
				}
			}
			return false
		}
		templateSlot := strings.TrimSpace(template.Slot)
		for _, accepted := range definition.Accepts {
			if accepted == templateSlot {
				return true
			}
		}
		return false
	}
	for _, allowed := range equipmentAllowedSlots(template) {
		if strings.TrimSpace(allowed) == slot {
			return true
		}
	}
	return false
}

func (s *Store) cleanupPlayerEquipSlotsTx(ctx context.Context, tx pgx.Tx, userID int64) (bool, error) {
	return s.cleanupPlayerEquipSlotsWithOptionsTx(ctx, tx, userID, true)
}

func (s *Store) cleanupPlayerEquipSlotsForStatsTx(ctx context.Context, tx pgx.Tx, userID int64) (bool, error) {
	return s.cleanupPlayerEquipSlotsWithOptionsTx(ctx, tx, userID, false)
}

func (s *Store) cleanupPlayerEquipSlotsWithOptionsTx(ctx context.Context, tx pgx.Tx, userID int64, repairInstanceState bool) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	battleProfile, err := s.resolvePlayerBattleSlotProfileTx(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	rows, err := tx.Query(ctx, `
		SELECT s.user_id, s.equip_type, s.slot, s.instance_id,
		       i.user_id, i.template_item_id, i.state
		FROM player_equip_slots s
		LEFT JOIN player_item_instances i ON i.instance_id = s.instance_id
		WHERE s.user_id = $1
		ORDER BY s.equip_type ASC, s.slot ASC, s.instance_id ASC
		FOR UPDATE OF s
	`, userID)
	if err != nil {
		return false, err
	}
	type deleteKey struct {
		userID     int64
		equipType  string
		slot       string
		instanceID int64
	}
	toDelete := make([]deleteKey, 0, 4)
	equippedAffectedInstances := map[int64]struct{}{}
	for rows.Next() {
		var (
			rowUserID  int64
			equipType  string
			slot       string
			instanceID int64
			itemUserID sql.NullInt64
			templateID sql.NullString
			state      sql.NullString
		)
		if err := rows.Scan(&rowUserID, &equipType, &slot, &instanceID, &itemUserID, &templateID, &state); err != nil {
			rows.Close()
			return false, err
		}
		valid := false
		itemWasEquipped := state.Valid && strings.TrimSpace(state.String) == "equipped"
		if itemUserID.Valid && templateID.Valid &&
			itemUserID.Int64 == rowUserID &&
			itemWasEquipped {
			template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(templateID.String))
			valid = equipmentTemplateAllowsEquipSlotWithProfile(template, equipType, slot, battleProfile)
		}
		if !valid {
			toDelete = append(toDelete, deleteKey{
				userID:     rowUserID,
				equipType:  strings.TrimSpace(equipType),
				slot:       strings.TrimSpace(slot),
				instanceID: instanceID,
			})
			if itemWasEquipped {
				equippedAffectedInstances[instanceID] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	for _, row := range toDelete {
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_equip_slots
			WHERE user_id = $1
			  AND equip_type = $2
			  AND slot = $3
			  AND instance_id = $4
		`, row.userID, row.equipType, row.slot, row.instanceID); err != nil {
			return false, err
		}
	}
	if repairInstanceState {
		for instanceID := range equippedAffectedInstances {
			if err := s.setInstanceInventoryWhenFullyUnequippedTx(ctx, tx, userID, instanceID); err != nil {
				return false, err
			}
		}
	}
	return len(toDelete) > 0, nil
}

func validPlayerEquippedItemsQuery() string {
	return `
		SELECT s.equip_type, s.slot, i.instance_id, i.template_item_id, i.affixes, i.enhance_level
		FROM player_equip_slots s
		JOIN player_item_instances i ON i.instance_id = s.instance_id AND i.user_id = s.user_id
		WHERE s.user_id = $1
		  AND i.state = 'equipped'
		ORDER BY i.instance_id ASC, s.equip_type ASC, s.slot ASC
	`
}

func (s *Store) listValidPlayerEquippedItemsTx(ctx context.Context, tx pgx.Tx, userID int64) ([]validPlayerEquippedItemRow, error) {
	battleProfile, err := s.resolvePlayerBattleSlotProfileTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, validPlayerEquippedItemsQuery(), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type groupedEquippedItem struct {
		row   validPlayerEquippedItemRow
		slots []string
	}
	grouped := make(map[int64]*groupedEquippedItem, 16)
	instanceOrder := make([]int64, 0, 16)
	for rows.Next() {
		var row validPlayerEquippedItemRow
		var slot string
		if err := rows.Scan(&row.EquipType, &slot, &row.InstanceID, &row.TemplateItemID, &row.AffixRaw, &row.EnhanceLevel); err != nil {
			return nil, err
		}
		entry := grouped[row.InstanceID]
		if entry == nil {
			row.TemplateItemID = strings.TrimSpace(row.TemplateItemID)
			row.EquipType = normalizeEquipType(row.EquipType)
			entry = &groupedEquippedItem{row: row}
			grouped[row.InstanceID] = entry
			instanceOrder = append(instanceOrder, row.InstanceID)
		}
		entry.slots = append(entry.slots, strings.TrimSpace(slot))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]validPlayerEquippedItemRow, 0, len(instanceOrder))
	for _, instanceID := range instanceOrder {
		entry := grouped[instanceID]
		if entry == nil {
			continue
		}
		template := s.gameData.EquipmentTemplateByID(entry.row.TemplateItemID)
		valid := template != nil && len(entry.slots) > 0
		for _, slot := range entry.slots {
			if !equipmentTemplateAllowsEquipSlotWithProfile(template, entry.row.EquipType, slot, battleProfile) || (entry.row.EquipType == "battle" && slot == "right_hand_2" && len(template.OccupySlots) > 0) {
				valid = false
				break
			}
		}
		if valid {
			out = append(out, entry.row)
		}
	}
	return out, nil
}

func (s *Store) RepairPlayerEquipSlotConsistency(ctx context.Context, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	var repaired int64
	var lastUserID int64
	for {
		tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return repaired, err
		}
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT user_id
			FROM player_equip_slots
			WHERE user_id > $1
			ORDER BY user_id ASC
			LIMIT $2
		`, lastUserID, batchSize)
		if err != nil {
			_ = tx.Rollback(ctx)
			return repaired, err
		}
		userIDs := make([]int64, 0, batchSize)
		for rows.Next() {
			var userID int64
			if err := rows.Scan(&userID); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return repaired, err
			}
			userIDs = append(userIDs, userID)
			lastUserID = userID
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return repaired, err
		}
		rows.Close()
		if len(userIDs) == 0 {
			_ = tx.Rollback(ctx)
			break
		}
		for _, userID := range userIDs {
			if err := s.lockUserTx(ctx, tx, userID); err != nil {
				_ = tx.Rollback(ctx)
				return repaired, err
			}
			changed, err := s.cleanupPlayerEquipSlotsTx(ctx, tx, userID)
			if err != nil {
				_ = tx.Rollback(ctx)
				return repaired, err
			}
			if !changed {
				continue
			}
			if err := s.recomputeDerivedStatsTx(ctx, tx, userID); err != nil {
				_ = tx.Rollback(ctx)
				return repaired, fmt.Errorf("recompute repaired equipment slots for user %d: %w", userID, err)
			}
			repaired++
		}
		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return repaired, err
		}
		if len(userIDs) < batchSize {
			break
		}
	}
	return repaired, nil
}
