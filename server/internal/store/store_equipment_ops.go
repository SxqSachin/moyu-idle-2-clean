package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) RepairItem(ctx context.Context, userID, instanceID int64) error {
	res, err := s.db.Exec(ctx, `
		UPDATE player_item_instances
		SET durability = max_durability, updated_at = now()
		WHERE instance_id = $1 AND user_id = $2
	`, instanceID, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrInstanceNotOwned
	}
	return nil
}

func (s *Store) GetEquipmentOperationInfo(ctx context.Context, userID, instanceID int64) (EquipmentOperationInfo, error) {
	var out EquipmentOperationInfo
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var affixRaw []byte
		if err := tx.QueryRow(ctx, `
			SELECT instance_id, user_id, template_item_id, enhance_level, affixes
			FROM player_item_instances
			WHERE instance_id = $1
			FOR SHARE
		`, instanceID).Scan(&out.InstanceID, &ownerID, &out.TemplateItemID, &out.EnhanceLevel, &affixRaw); err != nil {
			return err
		}
		if ownerID != userID {
			return ErrInstanceNotOwned
		}

		template := s.gameData.EquipmentTemplateByID(out.TemplateItemID)
		if template != nil {
			out.AffixSlotCount = template.AffixSlotCount
		}
		out.AffixSlots = decodeAffixSlots(affixRaw)
		out.DetachRules = s.listActiveDetachRules()
		out.EnchantRules = s.listActiveEnchantRules(out.TemplateItemID)
		out.CarrierRules = s.listActiveCarrierRules()
		return nil
	})
	if err != nil {
		return EquipmentOperationInfo{}, err
	}
	return out, nil
}

func (s *Store) listActiveDetachRules() []EquipmentDetachRule {
	out := make([]EquipmentDetachRule, 0, len(s.gameData.AffixDetachRules))
	for _, rule := range s.gameData.AffixDetachRules {
		if !rule.Active {
			continue
		}
		out = append(out, EquipmentDetachRule{
			ToolItemID:                  rule.ToolItemID,
			DestroyChanceBase:           rule.DestroyChanceBase,
			LuckImpactFactor:            rule.LuckImpactFactor,
			AffixDropBeforeDestroyCheck: rule.AffixDropBeforeDestroyCheck,
		})
	}
	return out
}

func (s *Store) listActiveEnchantRules(templateID string) []EquipmentEnchantRule {
	out := make([]EquipmentEnchantRule, 0, len(s.gameData.AffixEnchantRules))
	for _, rule := range s.gameData.AffixEnchantRules {
		if !rule.Active {
			continue
		}
		effectiveAllowedAffixIDs := s.resolveEnchantAllowedAffixIDs(&rule, templateID)
		allowedAffixIDs := make([]string, 0, len(effectiveAllowedAffixIDs))
		for _, affixID := range effectiveAllowedAffixIDs {
			normalized := strings.TrimSpace(affixID)
			if normalized == "" {
				continue
			}
			allowedAffixIDs = append(allowedAffixIDs, normalized)
		}
		if len(rule.AllowedAffixScopes) > 0 && len(allowedAffixIDs) == 0 {
			continue
		}
		weights := rule.RarityWeights
		if templateID != "" {
			_, compatible, err := s.enchantAffixCandidates(&rule, templateID)
			if err != nil {
				weights = map[string]float64{}
			} else {
				weights = compatible
			}
		}
		out = append(out, EquipmentEnchantRule{
			ToolItemID:      rule.ToolItemID,
			ConsumeCount:    rule.ConsumeCount,
			Costs:           rule.Costs,
			FailChance:      rule.FailChance,
			RarityWeights:   weights,
			AllowedAffixIDs: allowedAffixIDs,
		})
	}
	return out
}

func (s *Store) listActiveCarrierRules() []EquipmentCarrierRule {
	out := make([]EquipmentCarrierRule, 0, len(s.gameData.AffixCarrierRules))
	for _, rule := range s.gameData.AffixCarrierRules {
		if !rule.Active {
			continue
		}
		allowedRarityIDs := make([]string, 0, len(rule.AllowedRarityIDs))
		for _, rarityID := range rule.AllowedRarityIDs {
			normalized := strings.TrimSpace(rarityID)
			if normalized == "" {
				continue
			}
			allowedRarityIDs = append(allowedRarityIDs, normalized)
		}
		out = append(out, EquipmentCarrierRule{
			CarrierItemID:    rule.CarrierItemID,
			AllowedRarityIDs: allowedRarityIDs,
		})
	}
	return out
}
