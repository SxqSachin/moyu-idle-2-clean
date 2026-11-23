package store

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetKittyBattlePointState(ctx context.Context, userID, kittyID int64) (KittyBattlePointState, error) {
	if userID <= 0 || kittyID <= 0 {
		return KittyBattlePointState{}, ErrBattleRoomConfigInvalid
	}
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return KittyBattlePointState{}, err
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO player_kitty_battle_point_state(kitty_id, total_points, spent_points, updated_at)
			VALUES($1, 0, 0, now())
			ON CONFLICT (kitty_id) DO NOTHING
		`, kittyID)
		return err
	}); err != nil {
		return KittyBattlePointState{}, err
	}
	var state KittyBattlePointState
	if err := s.db.QueryRow(ctx, `
		SELECT kitty_id, total_points, spent_points
		FROM player_kitty_battle_point_state
		WHERE kitty_id = $1
	`, kittyID).Scan(&state.KittyID, &state.TotalPoints, &state.SpentPoints); err != nil {
		return KittyBattlePointState{}, err
	}
	if state.SpentPoints > state.TotalPoints {
		state.SpentPoints = state.TotalPoints
	}
	return state, nil
}

func (s *Store) AllocateKittyBattlePointsBatch(ctx context.Context, userID, kittyID int64, allocations map[string]int64) (KittyBattlePointState, error) {
	if userID <= 0 || kittyID <= 0 || len(allocations) == 0 {
		return KittyBattlePointState{}, ErrBattleRoomConfigInvalid
	}
	normalized := make(map[string]int64, len(allocations))
	var total int64
	for key, value := range allocations {
		normKey := strings.TrimSpace(key)
		if normKey == "" {
			return KittyBattlePointState{}, ErrBattleRoomConfigInvalid
		}
		if value <= 0 {
			return KittyBattlePointState{}, ErrBattleRoomConfigInvalid
		}
		if !s.battlePointAttrAllowed(normKey) {
			return KittyBattlePointState{}, ErrBattlePointAttrNotAllowed
		}
		normalized[normKey] += value
		total += value
	}
	if total <= 0 {
		return KittyBattlePointState{}, ErrBattleRoomConfigInvalid
	}

	seeds := s.attributeSeedsFromGameData()
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, true); err != nil {
			return err
		}
		if err := s.ensureKittyAttributesTx(ctx, tx, kittyID, seeds); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_kitty_battle_point_state(kitty_id, total_points, spent_points, updated_at)
			VALUES($1, 0, 0, now())
			ON CONFLICT (kitty_id) DO NOTHING
		`, kittyID); err != nil {
			return err
		}

		var totalPoints, spentPoints int64
		if err := tx.QueryRow(ctx, `
			SELECT total_points, spent_points
			FROM player_kitty_battle_point_state
			WHERE kitty_id = $1
			FOR UPDATE
		`, kittyID).Scan(&totalPoints, &spentPoints); err != nil {
			return err
		}
		available := totalPoints - spentPoints
		if available < total {
			return ErrBattlePointInsufficient
		}

		for attrKey, value := range normalized {
			tag, err := tx.Exec(ctx, `
				UPDATE player_kitty_attributes
				SET skill_point_level = skill_point_level + $3,
				    updated_at = now()
				WHERE kitty_id = $1
				  AND attr_key = $2
			`, kittyID, attrKey, value)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrBattlePointAttrNotAllowed
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_kitty_battle_point_state
			SET spent_points = spent_points + $2,
			    updated_at = now()
			WHERE kitty_id = $1
		`, kittyID, total); err != nil {
			return err
		}
		return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds)
	})
	if err != nil {
		return KittyBattlePointState{}, err
	}
	return s.GetKittyBattlePointState(ctx, userID, kittyID)
}

func (s *Store) ResetKittyBattlePoints(ctx context.Context, userID, kittyID int64) (KittyBattlePointState, error) {
	if userID <= 0 || kittyID <= 0 {
		return KittyBattlePointState{}, ErrBattleRoomConfigInvalid
	}
	seeds := s.attributeSeedsFromGameData()
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, true); err != nil {
			return err
		}
		if err := s.ensureKittyAttributesTx(ctx, tx, kittyID, seeds); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_kitty_battle_point_state(kitty_id, total_points, spent_points, updated_at)
			VALUES($1, 0, 0, now())
			ON CONFLICT (kitty_id) DO NOTHING
		`, kittyID); err != nil {
			return err
		}

		var allocated int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(skill_point_level), 0)
			FROM player_kitty_attributes
			WHERE kitty_id = $1
		`, kittyID).Scan(&allocated); err != nil {
			return err
		}
		if allocated <= 0 {
			_, err := tx.Exec(ctx, `
				UPDATE player_kitty_battle_point_state
				SET spent_points = 0,
				    updated_at = now()
				WHERE kitty_id = $1
			`, kittyID)
			return err
		}

		itemID := strings.TrimSpace(s.config.Battle.ResetItemID)
		if itemID == "" || s.config.Battle.ResetItemCount <= 0 {
			return ErrBattleRoomConfigInvalid
		}
		if err := s.ConsumeByTemplateTx(ctx, tx, userID, itemID, s.config.Battle.ResetItemCount); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE player_kitty_attributes
			SET skill_point_level = 0,
			    updated_at = now()
			WHERE kitty_id = $1
			  AND skill_point_level > 0
		`, kittyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_kitty_battle_point_state
			SET spent_points = 0,
			    updated_at = now()
			WHERE kitty_id = $1
		`, kittyID); err != nil {
			return err
		}
		if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds); err != nil {
			return err
		}
		return s.autoUnequipKittyUnmetEquipmentStatusReqTx(ctx, tx, userID, kittyID)
	})
	if err != nil {
		return KittyBattlePointState{}, err
	}
	return s.GetKittyBattlePointState(ctx, userID, kittyID)
}

func (s *Store) applyKittyAttrExpLockedTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	kittyID int64,
	attrExp map[string]float64,
	seeds []PlayerAttributeSeed,
) error {
	_, err := s.applyKittyAttrExpLockedResultTx(ctx, tx, userID, kittyID, attrExp, seeds)
	return err
}

func (s *Store) applyKittyAttrExpLockedResultTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	kittyID int64,
	attrExp map[string]float64,
	seeds []PlayerAttributeSeed,
) (attributeExpApplyResult, error) {
	result := attributeExpApplyResult{}
	if kittyID <= 0 || len(attrExp) == 0 {
		return result, nil
	}
	var storedBattleLevelCap int64
	var kittyLevel int
	if err := tx.QueryRow(ctx, `
		SELECT battle_level_cap, level
		FROM player_kitties
		WHERE user_id = $1
		  AND id = $2
		FOR UPDATE
	`, userID, kittyID).Scan(&storedBattleLevelCap, &kittyLevel); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, ErrKittyNotFound
		}
		return result, err
	}
	battleLevelCap := s.effectiveBattleLevelCap(storedBattleLevelCap, kittyLevel)

	attrKeys := make([]string, 0, len(attrExp))
	for attrKey := range attrExp {
		if strings.TrimSpace(attrKey) != "" {
			attrKeys = append(attrKeys, attrKey)
		}
	}
	sort.Strings(attrKeys)
	attributesEnsured := false
	for _, attrKey := range attrKeys {
		gain := attrExp[attrKey]
		key := strings.TrimSpace(attrKey)
		if key == "" || math.IsNaN(gain) || math.IsInf(gain, 0) || gain == 0 {
			continue
		}

		var beforeExp float64
		err := tx.QueryRow(ctx, `
			SELECT attr_exp
			FROM player_kitty_attributes
			WHERE kitty_id = $1
			  AND attr_key = $2
			FOR UPDATE
		`, kittyID, key).Scan(&beforeExp)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) && !attributesEnsured {
				if ensureErr := s.ensureKittyAttributesTx(ctx, tx, kittyID, seeds); ensureErr != nil {
					return result, ensureErr
				}
				attributesEnsured = true
				err = tx.QueryRow(ctx, `
					SELECT attr_exp
					FROM player_kitty_attributes
					WHERE kitty_id = $1
					  AND attr_key = $2
					FOR UPDATE
				`, kittyID, key).Scan(&beforeExp)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return result, err
			}
		}

		beforeProgress, err := s.resolveAttributeProgress(key, beforeExp)
		if err != nil {
			return result, err
		}
		afterExp, err := s.attributeExpProfile.clampTotalExp(key, roundFloat(beforeExp+gain, 4))
		if err != nil {
			return result, err
		}
		if key == "battle" {
			capTotalExp, capErr := s.attributeExpProfile.totalExpBeforeLevel(key, battleLevelCap)
			if capErr != nil {
				return result, capErr
			}
			if afterExp > capTotalExp {
				afterExp = capTotalExp
			}
		}
		afterExp = roundFloat(afterExp, 4)
		afterProgress, err := s.resolveAttributeProgress(key, afterExp)
		if err != nil {
			return result, err
		}
		levelDelta := afterProgress.Level - beforeProgress.Level
		if levelDelta == 0 && math.Abs(afterExp-beforeExp) < 1e-9 {
			continue
		}

		if _, err := tx.Exec(ctx, `
			UPDATE player_kitty_attributes
			SET attr_exp = $3,
			    base_level = GREATEST(1, base_level + $4),
			    final_value = final_value + $4,
			    lock_version = lock_version + 1,
			    updated_at = now()
			WHERE kitty_id = $1
			  AND attr_key = $2
		`, kittyID, key, afterExp, levelDelta); err != nil {
			return result, err
		}
		result.Changed = true
		result.LevelChanged = result.LevelChanged || levelDelta != 0

		if key == "battle" && levelDelta > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO player_kitty_battle_point_state(kitty_id, total_points, spent_points, updated_at)
				VALUES($1, $2, 0, now())
				ON CONFLICT (kitty_id) DO UPDATE
				SET total_points = player_kitty_battle_point_state.total_points + EXCLUDED.total_points,
				    updated_at = now()
			`, kittyID, levelDelta); err != nil {
				return result, err
			}
		}
	}
	if !result.LevelChanged {
		return result, nil
	}
	return result, s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds)
}

func (s *Store) autoUnequipKittyUnmetEquipmentStatusReqTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64) error {
	type equipRow struct {
		Slot           string
		InstanceID     int64
		TemplateItemID string
	}
	rows, err := tx.Query(ctx, `
		SELECT slot, instance_id, template_item_id
		FROM player_kitty_equip_slots
		JOIN player_item_instances USING(instance_id)
		WHERE kitty_id = $1
	`, kittyID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var equipped []equipRow
	for rows.Next() {
		var row equipRow
		if err := rows.Scan(&row.Slot, &row.InstanceID, &row.TemplateItemID); err != nil {
			return err
		}
		equipped = append(equipped, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(equipped) == 0 {
		return nil
	}

	attrRows, err := tx.Query(ctx, `
		SELECT attr_key, final_value
		FROM player_kitty_attributes
		WHERE kitty_id = $1
	`, kittyID)
	if err != nil {
		return err
	}
	defer attrRows.Close()
	attrMap := make(map[string]float64)
	for attrRows.Next() {
		var key string
		var value float64
		if err := attrRows.Scan(&key, &value); err != nil {
			return err
		}
		attrMap[key] = value
	}
	if err := attrRows.Err(); err != nil {
		return err
	}

	var toUnequip []int64
	for _, row := range equipped {
		template := s.gameData.EquipmentTemplateByID(row.TemplateItemID)
		if template == nil || len(template.StatusRequirement) == 0 {
			continue
		}
		unmet := false
		for key, required := range template.StatusRequirement {
			if attrMap[strings.TrimSpace(key)] < float64(required) {
				unmet = true
				break
			}
		}
		if unmet {
			toUnequip = append(toUnequip, row.InstanceID)
		}
	}
	if len(toUnequip) == 0 {
		return nil
	}

	changed := false
	for _, instanceID := range toUnequip {
		tag, err := tx.Exec(ctx, `
			DELETE FROM player_kitty_equip_slots
			WHERE kitty_id = $1
			  AND instance_id = $2
		`, kittyID, instanceID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		changed = true
		if err := s.setInstanceInventoryWhenFullyUnequippedTx(ctx, tx, userID, instanceID); err != nil {
			return err
		}
	}
	if !changed {
		return nil
	}
	return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData())
}
