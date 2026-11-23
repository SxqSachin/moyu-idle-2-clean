package store

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetBattlePointState(ctx context.Context, userID int64) (PlayerBattlePointState, error) {
	if userID <= 0 {
		return PlayerBattlePointState{}, errors.New("invalid user id")
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO player_battle_point_state(user_id, total_points, spent_points, updated_at)
		VALUES($1, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return PlayerBattlePointState{}, err
	}
	var out PlayerBattlePointState
	err := s.db.QueryRow(ctx, `
		SELECT user_id, total_points, spent_points
		FROM player_battle_point_state
		WHERE user_id = $1
	`, userID).Scan(&out.UserID, &out.TotalPoints, &out.SpentPoints)
	if err != nil {
		return PlayerBattlePointState{}, err
	}
	if out.SpentPoints > out.TotalPoints {
		out.SpentPoints = out.TotalPoints
	}
	return out, nil
}

func (s *Store) battlePointAttrAllowed(attrKey string) bool {
	key := strings.TrimSpace(attrKey)
	if key == "" {
		return false
	}
	for _, row := range s.gameData.Attributes {
		if strings.TrimSpace(row.ID) != key {
			continue
		}
		return row.AllowBattlePointLevelUp
	}
	return false
}

func (s *Store) AllocateBattlePoints(ctx context.Context, userID int64, attrKey string, points int64) (PlayerBattlePointState, error) {
	return s.AllocateBattlePointsBatch(ctx, userID, map[string]int64{
		attrKey: points,
	})
}

func (s *Store) AllocateBattlePointsBatch(ctx context.Context, userID int64, allocations map[string]int64) (PlayerBattlePointState, error) {
	if userID <= 0 {
		return PlayerBattlePointState{}, ErrBattleRoomConfigInvalid
	}
	if len(allocations) == 0 {
		return PlayerBattlePointState{}, ErrBattleRoomConfigInvalid
	}
	normalizedAllocations := make(map[string]int64, len(allocations))
	var totalPoints int64
	for rawKey, rawPoints := range allocations {
		attrKey := strings.TrimSpace(rawKey)
		points := rawPoints
		if attrKey == "" || points <= 0 {
			return PlayerBattlePointState{}, ErrBattleRoomConfigInvalid
		}
		if !s.battlePointAttrAllowed(attrKey) {
			return PlayerBattlePointState{}, ErrBattlePointAttrNotAllowed
		}
		if points > math.MaxInt64-normalizedAllocations[attrKey] {
			return PlayerBattlePointState{}, ErrBattleRoomConfigInvalid
		}
		normalizedAllocations[attrKey] += points
		if points > math.MaxInt64-totalPoints {
			return PlayerBattlePointState{}, ErrBattleRoomConfigInvalid
		}
		totalPoints += points
	}
	if totalPoints <= 0 {
		return PlayerBattlePointState{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_battle_point_state(user_id, total_points, spent_points, updated_at)
			VALUES($1, 0, 0, now())
			ON CONFLICT (user_id) DO NOTHING
		`, userID); err != nil {
			return err
		}
		var total int64
		var spent int64
		if err := tx.QueryRow(ctx, `
			SELECT total_points, spent_points
			FROM player_battle_point_state
			WHERE user_id = $1
			FOR UPDATE
		`, userID).Scan(&total, &spent); err != nil {
			return err
		}
		available := total - spent
		if available < totalPoints {
			return ErrBattlePointInsufficient
		}
		for attrKey, points := range normalizedAllocations {
			tag, err := tx.Exec(ctx, `
				UPDATE player_attributes
				SET skill_point_level = skill_point_level + $3,
				    updated_at = now()
				WHERE user_id = $1 AND attr_key = $2
			`, userID, attrKey, points)
			if err != nil {
				return err
			}
			if tag.RowsAffected() <= 0 {
				return ErrBattlePointAttrNotAllowed
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_battle_point_state
			SET spent_points = spent_points + $2,
			    updated_at = now()
			WHERE user_id = $1
		`, userID, totalPoints); err != nil {
			return err
		}
		return s.onPlayerAttributesChangedTx(ctx, tx, userID)
	}); err != nil {
		return PlayerBattlePointState{}, err
	}
	return s.GetBattlePointState(ctx, userID)
}

func (s *Store) ResetBattlePoints(ctx context.Context, userID int64) (PlayerBattlePointState, error) {
	if userID <= 0 {
		return PlayerBattlePointState{}, errors.New("invalid user id")
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_battle_point_state(user_id, total_points, spent_points, updated_at)
			VALUES($1, 0, 0, now())
			ON CONFLICT (user_id) DO NOTHING
		`, userID); err != nil {
			return err
		}
		var allocated int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(skill_point_level), 0)
			FROM player_attributes
			WHERE user_id = $1
		`, userID).Scan(&allocated); err != nil {
			return err
		}
		if allocated <= 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE player_battle_point_state
				SET spent_points = 0,
				    updated_at = now()
				WHERE user_id = $1
			`, userID); err != nil {
				return err
			}
			return nil
		}
		itemID := strings.TrimSpace(s.config.Battle.ResetItemID)
		if itemID == "" {
			return ErrBattleRoomConfigInvalid
		}
		resetItemCount := s.config.Battle.ResetItemCount
		if resetItemCount <= 0 {
			return ErrBattleRoomConfigInvalid
		}
		if err := s.ConsumeByTemplateTx(ctx, tx, userID, itemID, resetItemCount); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_attributes
			SET skill_point_level = 0,
			    updated_at = now()
			WHERE user_id = $1 AND skill_point_level > 0
		`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_battle_point_state
			SET spent_points = 0,
			    updated_at = now()
			WHERE user_id = $1
		`, userID); err != nil {
			return err
		}
		if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.autoUnequipUnmetEquipmentStatusReqTx(ctx, tx, userID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return PlayerBattlePointState{}, err
	}
	return s.GetBattlePointState(ctx, userID)
}

func (s *Store) autoUnequipUnmetEquipmentStatusReqTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := s.reconcileUnmetEquipmentStatusReqTx(ctx, tx, userID)
	return err
}

type autoUnequippedEquipmentReceipt struct {
	InstanceID int64
	TemplateID string
	FromSlots  []string
}

func (s *Store) reconcileUnmetEquipmentStatusReqTx(ctx context.Context, tx pgx.Tx, userID int64) ([]autoUnequippedEquipmentReceipt, error) {
	receipts := make([]autoUnequippedEquipmentReceipt, 0, 4)
	for pass := 0; pass < 32; pass++ {
		removed, err := s.autoUnequipUnmetEquipmentStatusReqOnceTx(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		if len(removed) == 0 {
			return receipts, nil
		}
		receipts = append(receipts, removed...)
	}
	return nil, errors.New("equipment status requirement reconciliation did not converge")
}

func (s *Store) autoUnequipUnmetEquipmentStatusReqOnceTx(ctx context.Context, tx pgx.Tx, userID int64) ([]autoUnequippedEquipmentReceipt, error) {
	rows, err := tx.Query(ctx, `
		SELECT i.instance_id, i.template_item_id, s.slot
		FROM player_equip_slots s
		JOIN player_item_instances i ON i.instance_id = s.instance_id
		WHERE s.user_id = $1
		  AND i.user_id = $1
		  AND i.state = 'equipped'
		ORDER BY i.instance_id ASC, s.slot ASC
		FOR UPDATE
	`, userID)
	if err != nil {
		return nil, err
	}

	type equippedInstance struct {
		instanceID int64
		templateID string
		slots      []string
	}
	equippedByID := make(map[int64]*equippedInstance, 8)
	instanceOrder := make([]int64, 0, 8)
	for rows.Next() {
		var instanceID int64
		var templateID, slot string
		if err := rows.Scan(&instanceID, &templateID, &slot); err != nil {
			rows.Close()
			return nil, err
		}
		entry := equippedByID[instanceID]
		if entry == nil {
			entry = &equippedInstance{instanceID: instanceID, templateID: strings.TrimSpace(templateID)}
			equippedByID[instanceID] = entry
			instanceOrder = append(instanceOrder, instanceID)
		}
		entry.slots = append(entry.slots, strings.TrimSpace(slot))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(instanceOrder) == 0 {
		return nil, nil
	}

	attrRows, err := tx.Query(ctx, `
		SELECT attr_key, final_value
		FROM player_attributes
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	attrs := make(map[string]float64, 32)
	for attrRows.Next() {
		var key string
		var value float64
		if err := attrRows.Scan(&key, &value); err != nil {
			attrRows.Close()
			return nil, err
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		attrs[key] = value
	}
	if err := attrRows.Err(); err != nil {
		attrRows.Close()
		return nil, err
	}
	attrRows.Close()

	needsUnequip := make([]autoUnequippedEquipmentReceipt, 0, len(instanceOrder))
	for _, instanceID := range instanceOrder {
		item := equippedByID[instanceID]
		template := s.gameData.EquipmentTemplateByID(item.templateID)
		if template == nil || len(template.StatusRequirement) == 0 {
			continue
		}
		unmet := false
		for key, required := range template.StatusRequirement {
			if attrs[strings.TrimSpace(key)] < float64(required) {
				unmet = true
				break
			}
		}
		if unmet {
			needsUnequip = append(needsUnequip, autoUnequippedEquipmentReceipt{
				InstanceID: item.instanceID,
				TemplateID: item.templateID,
				FromSlots:  append([]string(nil), item.slots...),
			})
		}
	}
	if len(needsUnequip) == 0 {
		return nil, nil
	}

	for _, item := range needsUnequip {
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_equip_slots
			WHERE user_id = $1 AND instance_id = $2
		`, userID, item.InstanceID); err != nil {
			return nil, err
		}
		if err := s.SetStateTx(ctx, tx, item.InstanceID, userID, "equipped", "inventory"); err != nil {
			return nil, err
		}
	}
	if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	return needsUnequip, nil
}
