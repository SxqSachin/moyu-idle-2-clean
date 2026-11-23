package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) refreshInstanceBaseSnapshotTx(ctx context.Context, tx pgx.Tx, instanceID int64, ownerID int64) error {
	row, err := s.LockInstanceTx(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	if row.UserID != ownerID {
		return ErrInstanceNotOwned
	}
	return s.refreshLockedInstanceBaseSnapshotTx(ctx, tx, row)
}

func (s *Store) refreshLockedInstanceBaseSnapshotTx(ctx context.Context, tx pgx.Tx, row LockedItemInstance) error {
	templateID := strings.TrimSpace(row.TemplateItem)
	template := s.gameData.EquipmentTemplateByID(templateID)
	if template == nil {
		_, err := tx.Exec(ctx, `
			UPDATE player_item_instances
			SET base_snapshot = '{}'::jsonb, updated_at = now()
			WHERE instance_id = $1
		`, row.InstanceID)
		return err
	}
	base := make(map[string]float64, len(template.BaseAttrs))
	for key, value := range template.BaseAttrs {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		base[normalizedKey] = value
	}
	if s.enhanceCache != nil {
		enhanceAttrs := s.enhanceCache.CumulativeAttrs(templateID, row.EnhanceLevel)
		for key, value := range enhanceAttrs {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			base[normalizedKey] += value
		}
	}
	for _, slot := range FilterEffectiveAffixSlotsByLimit(decodeAffixSlots(row.AffixesRaw), s.gameData, nil) {
		for attrKey, value := range slot.AttrMods {
			normalizedKey := strings.TrimSpace(attrKey)
			if normalizedKey == "" {
				continue
			}
			base[normalizedKey] += value
		}
	}
	payload, err := json.Marshal(base)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE player_item_instances
		SET base_snapshot = $2::jsonb, updated_at = now()
		WHERE instance_id = $1
	`, row.InstanceID, string(payload))
	return err
}
