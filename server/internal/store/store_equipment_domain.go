package store

import (
	"context"
	"errors"
	"strings"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

func (s *Store) EquipSelectedSlotTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64, equipType, selectedSlot string) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	equipType = normalizeEquipType(equipType)
	selectedSlot = strings.TrimSpace(selectedSlot)
	if equipType == "" || selectedSlot == "" {
		return ErrEquipSlotNotAllowed
	}
	if _, err := s.cleanupPlayerEquipSlotsTx(ctx, tx, userID); err != nil {
		return err
	}
	row, err := s.LockInstanceTx(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	if row.UserID != userID {
		return ErrInstanceNotOwned
	}
	if row.State != "inventory" && row.State != "equipped" {
		return &InvalidInstanceStateError{State: row.State}
	}
	template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(row.TemplateItem))
	if template == nil || normalizeEquipType(template.EquipType) != equipType {
		return ErrEquipSlotNotAllowed
	}
	if !equipmentTemplateAllowsPlayer(template) {
		return ErrEquipTargetNotAllowed
	}
	battleProfile, err := s.resolvePlayerBattleSlotProfileTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if equipType == "battle" {
		definition, known := battleProfile.Definition(selectedSlot)
		if known && !definition.Enabled {
			return ErrEquipSlotDisabledBySpecialization
		}
	}
	if !equipmentTemplateSelectableInSlot(template, equipType, selectedSlot, battleProfile) {
		return ErrEquipSlotNotAllowed
	}
	if selectedSlot == "right_hand_2" && len(template.OccupySlots) > 0 {
		return ErrEquipSlotNotAllowed
	}
	occupySlots := []string{selectedSlot}
	if len(template.OccupySlots) > 0 {
		occupySlots = uniqueTrimmedStrings(template.OccupySlots)
	}
	if len(occupySlots) == 0 {
		return ErrEquipSlotNotAllowed
	}
	for _, occupiedSlot := range occupySlots {
		if !equipmentTemplateAllowsEquipSlotWithProfile(template, equipType, occupiedSlot, battleProfile) {
			if equipType == "battle" {
				if definition, known := battleProfile.Definition(occupiedSlot); known && !definition.Enabled {
					return ErrEquipSlotDisabledBySpecialization
				}
			}
			return ErrEquipSlotNotAllowed
		}
	}
	if err := s.ensureEquipmentStatusRequirementsTx(ctx, tx, userID, template); err != nil {
		return err
	}
	return s.EquipTx(ctx, tx, userID, instanceID, equipType, occupySlots, s.accessoryConflictTemplateIDs(template))
}

func equipmentTemplateAllowsPlayer(template *catalog.EquipmentTemplate) bool {
	if template == nil || len(template.EquipTargets) == 0 {
		return template != nil
	}
	foundPlayer := false
	foundKitty := false
	for _, raw := range template.EquipTargets {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "player":
			foundPlayer = true
		case "kitty":
			foundKitty = true
		}
	}
	return foundPlayer || (foundPlayer && foundKitty)
}

func equipmentTemplateAllowsKitty(template *catalog.EquipmentTemplate) bool {
	if template == nil || len(template.EquipTargets) == 0 {
		return template != nil
	}
	for _, raw := range template.EquipTargets {
		if strings.EqualFold(strings.TrimSpace(raw), "kitty") {
			return true
		}
	}
	return false
}

func equipmentTemplateSelectableInSlot(template *catalog.EquipmentTemplate, equipType, selectedSlot string, battleProfile EquipmentSlotProfile) bool {
	if template == nil {
		return false
	}
	if normalizeEquipType(equipType) != "battle" || len(template.OccupySlots) == 0 {
		return equipmentTemplateAllowsEquipSlotWithProfile(template, equipType, selectedSlot, battleProfile)
	}
	templateSlot := strings.TrimSpace(template.Slot)
	if templateSlot == "weapon" {
		return (selectedSlot == "left_hand" || selectedSlot == "right_hand") && battleProfile.Enabled(selectedSlot)
	}
	return templateSlot == selectedSlot && battleProfile.Enabled(selectedSlot)
}

func (s *Store) ensureEquipmentStatusRequirementsTx(ctx context.Context, tx pgx.Tx, userID int64, template *catalog.EquipmentTemplate) error {
	if template == nil || len(template.StatusRequirement) == 0 {
		return nil
	}
	for rawKey, required := range template.StatusRequirement {
		key := strings.TrimSpace(rawKey)
		if key == "" || required <= 0 {
			continue
		}
		var value float64
		if err := tx.QueryRow(ctx, `
			SELECT final_value
			FROM player_attributes
			WHERE user_id = $1 AND attr_key = $2
			FOR UPDATE
		`, userID, key).Scan(&value); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrStatusRequireNotMet
			}
			return err
		}
		if value < float64(required) {
			return ErrStatusRequireNotMet
		}
	}
	return nil
}

func (s *Store) accessoryConflictTemplateIDs(template *catalog.EquipmentTemplate) []string {
	if template == nil || strings.TrimSpace(template.Slot) != "accessory" {
		return nil
	}
	templateID := strings.TrimSpace(template.ID)
	group := strings.TrimSpace(template.AccessoryGroup)
	out := make([]string, 0, 4)
	for idx := range s.gameData.EquipmentTemplates {
		candidate := &s.gameData.EquipmentTemplates[idx]
		candidateID := strings.TrimSpace(candidate.ID)
		if candidateID == "" || strings.TrimSpace(candidate.Slot) != "accessory" {
			continue
		}
		if candidateID == templateID || (group != "" && strings.TrimSpace(candidate.AccessoryGroup) == group) {
			out = append(out, candidateID)
		}
	}
	return uniqueTrimmedStrings(out)
}

func uniqueTrimmedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *Store) EquipTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64, equipType string, occupySlots []string, accessoryTemplateIDs []string) error {
	if len(occupySlots) == 0 {
		return ErrEquipSlotNotAllowed
	}
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := s.cleanupPlayerEquipSlotsTx(ctx, tx, userID); err != nil {
		return err
	}
	equipType = normalizeEquipType(equipType)
	if equipType == "" {
		return ErrEquipSlotNotAllowed
	}

	row, err := s.LockInstanceTx(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	if row.UserID != userID {
		return ErrInstanceNotOwned
	}
	if row.State != "inventory" && row.State != "equipped" {
		return &InvalidInstanceStateError{State: row.State}
	}
	if err = s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, instanceID); err != nil {
		return err
	}
	if err = s.ensureEquipAffixBodyLimitsTx(ctx, tx, userID, instanceID, decodeAffixSlots(row.AffixesRaw)); err != nil {
		return err
	}
	sourceKittyRows, err := tx.Query(ctx, `
		SELECT DISTINCT kitty_id
		FROM player_kitty_equip_slots
		WHERE instance_id = $1
	`, instanceID)
	if err != nil {
		return err
	}
	sourceKittyIDs := make([]int64, 0, 1)
	for sourceKittyRows.Next() {
		var sourceKittyID int64
		if err = sourceKittyRows.Scan(&sourceKittyID); err != nil {
			sourceKittyRows.Close()
			return err
		}
		sourceKittyIDs = append(sourceKittyIDs, sourceKittyID)
	}
	if err = sourceKittyRows.Err(); err != nil {
		sourceKittyRows.Close()
		return err
	}
	sourceKittyRows.Close()

	if len(accessoryTemplateIDs) > 0 {
		var conflictInstanceID int64
		err = tx.QueryRow(ctx, `
			SELECT s.instance_id
			FROM player_equip_slots s
			JOIN player_item_instances i ON i.instance_id = s.instance_id
			WHERE s.user_id = $1
			  AND s.equip_type = $2
			  AND i.template_item_id = ANY($3::text[])
			  AND s.instance_id <> $4
			LIMIT 1
		`, userID, equipType, accessoryTemplateIDs, instanceID).Scan(&conflictInstanceID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			return ErrEquipAccessoryConflict
		}
	}

	conflictRows, err := tx.Query(ctx, `
		SELECT slot, instance_id
		FROM player_equip_slots
		WHERE user_id = $1
		  AND equip_type = $2
		  AND slot = ANY($3::text[])
		FOR UPDATE
	`, userID, equipType, occupySlots)
	if err != nil {
		return err
	}
	defer conflictRows.Close()

	conflictInstanceIDs := make(map[int64]struct{})
	for conflictRows.Next() {
		var occupiedSlot string
		var occupiedInstanceID int64
		if err = conflictRows.Scan(&occupiedSlot, &occupiedInstanceID); err != nil {
			return err
		}
		if occupiedInstanceID != instanceID {
			conflictInstanceIDs[occupiedInstanceID] = struct{}{}
		}
	}
	if err = conflictRows.Err(); err != nil {
		return err
	}
	conflictedIDs := make([]int64, 0, len(conflictInstanceIDs))
	for conflictedInstanceID := range conflictInstanceIDs {
		conflictedIDs = append(conflictedIDs, conflictedInstanceID)
	}
	if err = s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, conflictedIDs...); err != nil {
		return err
	}

	for conflictedInstanceID := range conflictInstanceIDs {
		if _, err = tx.Exec(ctx, `
			DELETE FROM player_equip_slots
			WHERE user_id = $1 AND equip_type = $2 AND instance_id = $3
		`, userID, equipType, conflictedInstanceID); err != nil {
			return err
		}
		if err = s.SetStateTx(ctx, tx, conflictedInstanceID, userID, "equipped", "inventory"); err != nil {
			return err
		}
	}

	if _, err = tx.Exec(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND equip_type = $2 AND instance_id = $3
	`, userID, equipType, instanceID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		DELETE FROM player_kitty_equip_slots
		WHERE instance_id = $1
	`, instanceID); err != nil {
		return err
	}

	for _, occupySlot := range occupySlots {
		if _, err = tx.Exec(ctx, `
			INSERT INTO player_equip_slots(user_id, equip_type, slot, instance_id, updated_at)
			VALUES($1, $2, $3, $4, now())
			ON CONFLICT (user_id, equip_type, slot)
			DO UPDATE SET instance_id = EXCLUDED.instance_id, updated_at = now()
		`, userID, equipType, occupySlot, instanceID); err != nil {
			return err
		}
	}

	if row.State == "inventory" {
		if err = s.SetStateTx(ctx, tx, instanceID, userID, "inventory", "equipped"); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx, `
			UPDATE player_item_instances
			SET updated_at = now()
			WHERE instance_id = $1 AND user_id = $2
		`, instanceID, userID); err != nil {
			return err
		}
	}
	if err = s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
		return err
	}
	if len(sourceKittyIDs) > 0 {
		seeds := s.attributeSeedsFromGameData()
		for _, sourceKittyID := range sourceKittyIDs {
			if err = s.recomputeKittyDerivedStatsTx(ctx, tx, userID, sourceKittyID, seeds); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) UnequipBySlotTx(ctx context.Context, tx pgx.Tx, userID int64, equipType, slot string) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := s.cleanupPlayerEquipSlotsTx(ctx, tx, userID); err != nil {
		return err
	}
	equipType = normalizeEquipType(equipType)
	if equipType == "" {
		return ErrEquipSlotNotAllowed
	}
	var instanceID int64
	if err := tx.QueryRow(ctx, `
		SELECT instance_id FROM player_equip_slots
		WHERE user_id = $1 AND equip_type = $2 AND slot = $3
		FOR UPDATE
	`, userID, equipType, slot).Scan(&instanceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEquipSlotNotFound
		}
		return err
	}
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, instanceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND equip_type = $2 AND instance_id = $3
	`, userID, equipType, instanceID); err != nil {
		return err
	}
	if err := s.SetStateTx(ctx, tx, instanceID, userID, "equipped", "inventory"); err != nil {
		return err
	}
	return s.onPlayerAttributesChangedTx(ctx, tx, userID)
}

func (s *Store) UnequipByInstanceTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64) (string, error) {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return "", err
	}
	if _, err := s.cleanupPlayerEquipSlotsTx(ctx, tx, userID); err != nil {
		return "", err
	}
	var slot string
	if err := tx.QueryRow(ctx, `
		SELECT slot
		FROM player_equip_slots
		WHERE user_id = $1 AND instance_id = $2
		FOR UPDATE
	`, userID, instanceID).Scan(&slot); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrEquipSlotNotFound
		}
		return "", err
	}
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, instanceID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND instance_id = $2
	`, userID, instanceID); err != nil {
		return "", err
	}
	if err := s.SetStateTx(ctx, tx, instanceID, userID, "equipped", "inventory"); err != nil {
		return "", err
	}
	if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
		return "", err
	}
	return slot, nil
}

func normalizeEquipType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "work", "battle":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return ""
	}
}

func (s *Store) ensureEquipAffixBodyLimitsTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	targetInstanceID int64,
	targetAffixes []EquipmentAffixSlot,
) error {
	bodySeen := map[string]int{}
	rows, err := tx.Query(ctx, `
		SELECT i.affixes
		FROM player_item_instances i
		WHERE i.user_id = $1
		  AND i.state = 'equipped'
		  AND i.instance_id <> $2
		  AND NOT EXISTS (
		    SELECT 1 FROM player_kitty_equip_slots k
		    WHERE k.instance_id = i.instance_id
		  )
		ORDER BY i.instance_id ASC
		FOR UPDATE
	`, userID, targetInstanceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var affixesRaw []byte
		if err = rows.Scan(&affixesRaw); err != nil {
			return err
		}
		_ = FilterEffectiveAffixSlotsByLimit(decodeAffixSlots(affixesRaw), s.gameData, bodySeen)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	targetEffective := FilterEffectiveAffixSlotsByLimit(targetAffixes, s.gameData, nil)
	for _, slot := range targetEffective {
		if componentID, limit, exceeded := WouldExceedCustomAlchemyBodyLimitWithLookup(bodySeen, slot, s.gameData); exceeded {
			return NewAffixBodyMaxEffectiveExceededError(componentID, limit)
		}
		for _, component := range alchemyLimitComponents(slot, s.gameData) {
			bodySeen[component.key]++
		}
		affixID := strings.TrimSpace(slot.AffixID)
		if affixID == "" {
			continue
		}
		def := s.gameData.AffixDefinitionByID(affixID)
		bodyLimit := AffixMaxEffectivePerBody(def)
		if bodyLimit <= 0 {
			continue
		}
		nextCount := bodySeen[affixID] + 1
		if nextCount > bodyLimit {
			return NewAffixBodyMaxEffectiveExceededError(affixID, bodyLimit)
		}
		bodySeen[affixID] = nextCount
	}
	return nil
}
