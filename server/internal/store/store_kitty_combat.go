package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type KittyCombatProfile struct {
	Kitty      PlayerKitty            `json:"kitty"`
	Attributes []PlayerAttributeState `json:"attributes"`
	Stats      PlayerStats            `json:"stats"`
	Skills     []PlayerSkill          `json:"skills"`
	Equipment  []EquippedItem         `json:"equipment"`
}

const kittyActionEfficiencyBaseDefault = -1

func (s *Store) kittyAttributeBaseDefault(attrKey string, fallback float64) float64 {
	normalizedKey := normalizeLower(attrKey)
	if normalizedKey == "" {
		return fallback
	}
	if normalizedKey == "fishing_efficiency" {
		return fallback
	}
	for _, row := range s.gameData.Attributes {
		if normalizeLower(row.ID) != normalizedKey {
			continue
		}
		relatedMainType := normalizeLower(row.RelatedActionMainType)
		if relatedMainType == "" {
			return fallback
		}
		relatedEffect := normalizeLower(row.RelatedActionEffect)
		if relatedEffect == "" || relatedEffect == "actionefficiency" {
			return kittyActionEfficiencyBaseDefault
		}
		return fallback
	}
	return fallback
}

func (s *Store) EnsureKittyAttributes(ctx context.Context, userID, kittyID int64, seeds []PlayerAttributeSeed) error {
	if userID <= 0 || kittyID <= 0 || len(seeds) == 0 {
		return nil
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, false); err != nil {
			return err
		}
		return s.ensureKittyAttributesTx(ctx, tx, kittyID, seeds)
	})
}

func (s *Store) ensureKittyAttributesTx(ctx context.Context, tx pgx.Tx, kittyID int64, seeds []PlayerAttributeSeed) error {
	batch := &pgx.Batch{}
	validSeeds := 0
	for _, seed := range seeds {
		attrKey := strings.TrimSpace(seed.AttrKey)
		if attrKey == "" {
			continue
		}
		baseDefault := s.kittyAttributeBaseDefault(attrKey, seed.BaseDefault)
		sourceMods := map[string]any{
			"version": "v2",
			"base": map[string]any{
				"value":           roundStatValue(baseDefault),
				"baseDefault":     roundStatValue(baseDefault),
				"baseLevelBonus":  0,
				"skillPointBonus": 0,
				"levelBonus":      0,
				"excluded":        false,
			},
			"equipment":   []map[string]any{},
			"personality": []map[string]any{},
			"summary":     map[string]any{"add": 0, "mul": 1},
		}
		payload, err := json.Marshal(sourceMods)
		if err != nil {
			return err
		}
		batch.Queue(`
			INSERT INTO player_kitty_attributes(
				kitty_id, attr_key, base_level, skill_point_level, attr_exp, source_mods_jsonb, final_value, lock_version, updated_at
			)
			VALUES($1, $2, 1, 0, 0, $3::jsonb, $4, 1, now())
			ON CONFLICT (kitty_id, attr_key) DO NOTHING
		`, kittyID, attrKey, string(payload), roundStatValue(baseDefault))
		validSeeds++
	}
	if validSeeds == 0 {
		return nil
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < validSeeds; i++ {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListKittyAttributes(ctx context.Context, userID, kittyID int64, seeds []PlayerAttributeSeed) ([]PlayerAttributeState, error) {
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level, attr_exp, source_mods_jsonb, final_value, lock_version
		FROM player_kitty_attributes
		WHERE kitty_id = $1
	`, kittyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seedMap := make(map[string]PlayerAttributeSeed, len(seeds))
	for _, seed := range seeds {
		key := strings.TrimSpace(seed.AttrKey)
		if key != "" {
			seedMap[key] = seed
		}
	}
	out := make([]PlayerAttributeState, 0, len(seeds))
	for rows.Next() {
		var item PlayerAttributeState
		var raw []byte
		if err := rows.Scan(&item.AttrKey, &item.BaseLevel, &item.SkillPointLevel, &item.AttrExp, &raw, &item.FinalValue, &item.LockVersion); err != nil {
			return nil, err
		}
		item.SourceMods = map[string]any{}
		_ = json.Unmarshal(raw, &item.SourceMods)
		if seed, ok := seedMap[item.AttrKey]; ok {
			item.BaseDefault = s.kittyAttributeBaseDefault(item.AttrKey, seed.BaseDefault)
			item.GrowthType = seed.GrowthType
			item.Unit = seed.Unit
			item.Category = seed.Category
			item.Order = seed.Order
			item.NameKey = seed.NameKey
			item.DescKey = seed.DescKey
			item.IconID = seed.IconID
			item.EmojiIcon = seed.EmojiIcon
			item.AllowBattlePointLevelUp = seed.AllowBattlePointLevelUp
		}
		if item.GrowthType == "levelable" {
			progress, progressErr := s.resolveAttributeProgress(item.AttrKey, item.AttrExp)
			if progressErr != nil {
				return nil, progressErr
			}
			item.ProgressLevel = progress.Level
			item.ProgressExp = int64(math.Round(progress.Exp))
			item.ProgressNextExp = int64(math.Round(progress.NextExp))
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) RecomputeKittyDerivedStats(ctx context.Context, userID, kittyID int64, seeds []PlayerAttributeSeed) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, false); err != nil {
			return err
		}
		return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds)
	})
}

func (s *Store) recomputeKittyDerivedStatsTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, seeds []PlayerAttributeSeed) error {
	if err := s.ensureKittyAttributesTx(ctx, tx, kittyID, seeds); err != nil {
		return err
	}
	baseDefaultMap := make(map[string]float64, len(seeds))
	for _, seed := range seeds {
		key := normalizeLower(seed.AttrKey)
		if key != "" {
			baseDefaultMap[key] = s.kittyAttributeBaseDefault(key, seed.BaseDefault)
		}
	}
	attrTotals := map[string]float64{}
	baseValues := map[string]float64{}
	baseLevelBonusMap := map[string]float64{}
	skillPointBonusMap := map[string]float64{}
	levelBonusMap := map[string]float64{}
	levelMap := map[string]int64{}
	attrDBKeyByNorm := map[string]string{}
	rows, err := tx.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level
		FROM player_kitty_attributes
		WHERE kitty_id = $1
		FOR UPDATE
	`, kittyID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rawAttrKey string
		var baseLevel, skillPointLevel int64
		if err := rows.Scan(&rawAttrKey, &baseLevel, &skillPointLevel); err != nil {
			rows.Close()
			return err
		}
		key := normalizeLower(rawAttrKey)
		if key == "" {
			continue
		}
		dbKey := strings.TrimSpace(rawAttrKey)
		if dbKey == "" {
			dbKey = key
		}
		baseLevelBonus := roundStatValue(math.Max(0, float64(baseLevel-1)))
		skillPointBonus := roundStatValue(math.Max(0, float64(skillPointLevel)))
		baseValue := roundStatValue(baseDefaultMap[key] + baseLevelBonus + skillPointBonus)
		attrTotals[key] = baseValue
		baseValues[key] = baseValue
		baseLevelBonusMap[key] = baseLevelBonus
		skillPointBonusMap[key] = skillPointBonus
		levelBonusMap[key] = baseLevelBonus
		levelMap[key] = baseLevel
		attrDBKeyByNorm[key] = dbKey
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	equipmentRows, err := tx.Query(ctx, `
		SELECT DISTINCT ON (i.instance_id) i.instance_id, i.template_item_id, i.affixes, i.enhance_level
		FROM player_item_instances i
		JOIN player_kitty_equip_slots s ON s.instance_id = i.instance_id
		WHERE s.kitty_id = $1
		ORDER BY i.instance_id ASC
	`, kittyID)
	if err != nil {
		return err
	}
	equipmentByAttr := map[string][]equipmentAttrContribution{}
	bodyAffixSeen := map[string]int{}
	for equipmentRows.Next() {
		var instanceID int64
		var templateItemID string
		var affixRaw []byte
		var enhanceLevel int
		if err := equipmentRows.Scan(&instanceID, &templateItemID, &affixRaw, &enhanceLevel); err != nil {
			equipmentRows.Close()
			return err
		}
		baseAttrs := map[string]float64{}
		if tmpl := s.gameData.EquipmentTemplateByID(strings.TrimSpace(templateItemID)); tmpl != nil {
			for key, value := range tmpl.BaseAttrs {
				key = normalizeLower(key)
				if key != "" {
					baseAttrs[key] = roundStatValue(value)
				}
			}
		}
		enhanceByAttr := map[string]float64{}
		if s.enhanceCache != nil && enhanceLevel > 0 {
			for key, value := range s.enhanceCache.CumulativeAttrs(strings.TrimSpace(templateItemID), enhanceLevel) {
				key = normalizeLower(key)
				if key != "" {
					enhanceByAttr[key] = roundStatValue(value)
				}
			}
		}
		affixByAttr := map[string][]equipmentAffixContribution{}
		for _, slot := range FilterEffectiveAffixSlotsByLimit(decodeAffixSlots(affixRaw), s.gameData, bodyAffixSeen) {
			for key, value := range slot.AttrMods {
				key = normalizeLower(key)
				if key != "" {
					affixByAttr[key] = append(affixByAttr[key], equipmentAffixContribution{
						AffixID: strings.TrimSpace(slot.AffixID),
						Value:   roundStatValue(value),
					})
				}
			}
		}
		seenAttr := map[string]struct{}{}
		for key := range baseAttrs {
			seenAttr[key] = struct{}{}
		}
		for key := range enhanceByAttr {
			seenAttr[key] = struct{}{}
		}
		for key := range affixByAttr {
			seenAttr[key] = struct{}{}
		}
		for attrKey := range seenAttr {
			affixItems := append([]equipmentAffixContribution{}, affixByAttr[attrKey]...)
			sort.Slice(affixItems, func(i, j int) bool {
				if affixItems[i].AffixID != affixItems[j].AffixID {
					return affixItems[i].AffixID < affixItems[j].AffixID
				}
				return affixItems[i].Value > affixItems[j].Value
			})
			entry := equipmentAttrContribution{
				InstanceID:     instanceID,
				TemplateItemID: strings.TrimSpace(templateItemID),
				Base:           roundStatValue(baseAttrs[attrKey]),
				Enhance:        roundStatValue(enhanceByAttr[attrKey]),
				Affixes:        affixItems,
			}
			total := entry.Total()
			if total == 0 {
				continue
			}
			equipmentByAttr[attrKey] = append(equipmentByAttr[attrKey], entry)
			attrTotals[attrKey] = roundStatValue(attrTotals[attrKey] + total)
		}
	}
	if err := equipmentRows.Err(); err != nil {
		equipmentRows.Close()
		return err
	}
	equipmentRows.Close()

	for attrKey := range equipmentByAttr {
		sort.Slice(equipmentByAttr[attrKey], func(i, j int) bool {
			if equipmentByAttr[attrKey][i].TemplateItemID != equipmentByAttr[attrKey][j].TemplateItemID {
				return equipmentByAttr[attrKey][i].TemplateItemID < equipmentByAttr[attrKey][j].TemplateItemID
			}
			return equipmentByAttr[attrKey][i].InstanceID < equipmentByAttr[attrKey][j].InstanceID
		})
	}

	personalityByAttr := map[string][]personalityAttrContribution{}
	profiles, err := s.getOwnedKittyPersonalityProfilesTx(ctx, tx, userID, kittyID)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		for _, effect := range profile.Effects {
			if effect.Kind != "attr_modifier" {
				continue
			}
			for rawAttrKey, rawValue := range effect.Attrs {
				attrKey := normalizeLower(rawAttrKey)
				value := roundStatValue(rawValue)
				if attrKey == "" || value == 0 {
					continue
				}
				personalityByAttr[attrKey] = append(personalityByAttr[attrKey], personalityAttrContribution{
					PersonalityID: strings.TrimSpace(profile.ID),
					NameKey:       strings.TrimSpace(profile.NameKey),
					Value:         value,
				})
				attrTotals[attrKey] = roundStatValue(attrTotals[attrKey] + value)
			}
		}
	}
	for attrKey := range personalityByAttr {
		sort.Slice(personalityByAttr[attrKey], func(i, j int) bool {
			return personalityByAttr[attrKey][i].PersonalityID < personalityByAttr[attrKey][j].PersonalityID
		})
	}

	collectiblesByAttr, collectibleKittyMods, err := s.loadCollectibleKittyModsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	for rawAttrKey, modValue := range collectibleKittyMods {
		attrKey := normalizeLower(rawAttrKey)
		if attrKey == "" || modValue == 0 {
			continue
		}
		attrTotals[attrKey] = roundStatValue(attrTotals[attrKey] + modValue)
	}

	formulaByAttr := map[string]formulaContribution{}
	for _, rawFormula := range s.config.DerivedStatsFormula.Formulas {
		targetAttr := normalizeLower(rawFormula.TargetAttr)
		if targetAttr == "" {
			continue
		}
		terms := make([]formulaTerm, 0, len(rawFormula.Terms))
		for _, term := range rawFormula.Terms {
			key := normalizeLower(term.AttrKey)
			if key == "" {
				continue
			}
			terms = append(terms, formulaTerm{
				AttrKey:     key,
				Attribute:   roundStatValue(attrTotals[key]),
				Coefficient: roundStatValue(term.Coefficient),
			})
		}
		if len(terms) > 0 {
			applyFormulaContribution(attrTotals, formulaByAttr, targetAttr, buildDerivedStatFormulaExpression(rawFormula.Terms), terms)
		}
	}
	derived := make(map[string]float64, len(attrTotals))
	for key, value := range attrTotals {
		derived[key] = roundStatValue(value)
	}
	updateBatch := &pgx.Batch{}
	updateCount := 0
	for key := range levelMap {
		finalValue := roundStatValue(attrTotals[key])
		sourcePayload := buildAttributeSourcePayload(attributeSourceBuildInput{
			AttrKey:            key,
			BaseDefault:        roundStatValue(baseDefaultMap[key]),
			BaseLevelBonus:     baseLevelBonusMap[key],
			SkillPointBonus:    skillPointBonusMap[key],
			LevelBonus:         levelBonusMap[key],
			BaseValue:          baseValues[key],
			FinalValue:         finalValue,
			CollectiblesByAttr: collectiblesByAttr,
			EquipmentByAttr:    equipmentByAttr,
			PersonalityByAttr:  personalityByAttr,
			FormulaByAttr:      formulaByAttr,
			BaseValuesByAttr:   baseValues,
		})
		sourceRaw, err := json.Marshal(sourcePayload)
		if err != nil {
			return err
		}
		attrDBKey := attrDBKeyByNorm[key]
		if attrDBKey == "" {
			attrDBKey = key
		}
		updateBatch.Queue(`
			UPDATE player_kitty_attributes
			SET final_value = $3,
			    source_mods_jsonb = $4::jsonb,
			    lock_version = lock_version + 1,
			    updated_at = now()
			WHERE kitty_id = $1 AND attr_key = $2
		`, kittyID, attrDBKey, finalValue, string(sourceRaw))
		updateCount++
	}
	if updateCount > 0 {
		ubr := tx.SendBatch(ctx, updateBatch)
		for i := 0; i < updateCount; i++ {
			if _, err := ubr.Exec(); err != nil {
				ubr.Close()
				return err
			}
		}
		ubr.Close()
	}
	payload, err := json.Marshal(derived)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_kitty_stats_cache(kitty_id, stats_jsonb, version, updated_at)
		VALUES($1, $2::jsonb, 1, now())
		ON CONFLICT (kitty_id)
		DO UPDATE SET stats_jsonb = EXCLUDED.stats_jsonb, version = player_kitty_stats_cache.version + 1, updated_at = now()
	`, kittyID, string(payload)); err != nil {
		return err
	}
	return nil
}

func (s *Store) GetKittyStats(ctx context.Context, userID, kittyID int64) (PlayerStats, error) {
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return PlayerStats{}, err
	}
	var out PlayerStats
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT stats_jsonb, version, updated_at
		FROM player_kitty_stats_cache
		WHERE kitty_id = $1
	`, kittyID).Scan(&raw, &out.CacheVer, &out.UpdatedAt)
	if err != nil {
		return out, err
	}
	_ = json.Unmarshal(raw, &out.Derived)
	return out, nil
}

func (s *Store) ListKittySkills(ctx context.Context, userID, kittyID int64) ([]PlayerSkill, error) {
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT skill_id, enabled, cast_order, rule_enabled, rule_groups_jsonb, level, exp, updated_at
		FROM player_kitty_skills
		WHERE kitty_id = $1
		ORDER BY cast_order ASC, skill_id ASC
	`, kittyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlayerSkill, 0, 8)
	for rows.Next() {
		var row PlayerSkill
		var raw []byte
		if err := rows.Scan(&row.SkillID, &row.Enabled, &row.CastOrder, &row.RuleEnabled, &raw, &row.Level, &row.Exp, &row.UpdatedAt); err != nil {
			return nil, err
		}
		var groups any
		_ = json.Unmarshal(raw, &groups)
		row.RuleGroups = groups
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) UpsertKittyCombatConfig(ctx context.Context, userID, kittyID int64, cfg CombatConfig) error {
	skillSlots := sanitizeSkillIDs(cfg.SkillSlots)
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, false); err != nil {
			return err
		}
		mutexIDs := append([]string(nil), skillSlots...)
		rows, err := tx.Query(ctx, `SELECT skill_id FROM player_kitty_skills WHERE kitty_id=$1 AND rule_enabled`, kittyID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var skillID string
			if err := rows.Scan(&skillID); err != nil {
				rows.Close()
				return err
			}
			mutexIDs = append(mutexIDs, skillID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if err := s.validateSkillForgeSourceMutexTx(ctx, tx, userID, sanitizeSkillIDs(mutexIDs)); err != nil {
			return err
		}
		for _, skillID := range skillSlots {
			if _, err := tx.Exec(ctx, `
				INSERT INTO player_kitty_skills(kitty_id, skill_id, enabled, cast_order, rule_enabled, rule_groups_jsonb, level, exp, updated_at)
				VALUES($1, $2, FALSE, 0, FALSE, '[]'::jsonb, 1, 0, now())
				ON CONFLICT (kitty_id, skill_id) DO UPDATE SET updated_at = now()
			`, kittyID, skillID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE player_kitty_skills SET enabled = FALSE, cast_order = 0, updated_at = now() WHERE kitty_id = $1`, kittyID); err != nil {
			return err
		}
		for idx, skillID := range skillSlots {
			if _, err := tx.Exec(ctx, `
				UPDATE player_kitty_skills
				SET enabled = TRUE, cast_order = $3, updated_at = now()
				WHERE kitty_id = $1 AND skill_id = $2
			`, kittyID, skillID, idx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) UpsertKittySkillRule(ctx context.Context, userID, kittyID int64, skillID string, enabled bool, groups any) error {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return ErrInvalidSkillID
	}
	raw, err := json.Marshal(groups)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, false); err != nil {
			return err
		}
		if enabled {
			rows, err := tx.Query(ctx, `SELECT skill_id FROM player_kitty_skills WHERE kitty_id=$1 AND (enabled OR rule_enabled) AND skill_id<>$2`, kittyID, skillID)
			if err != nil {
				return err
			}
			ids := []string{skillID}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, id)
			}
			rows.Close()
			if err := s.validateSkillForgeSourceMutexTx(ctx, tx, userID, ids); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO player_kitty_skills(kitty_id, skill_id, enabled, cast_order, rule_enabled, rule_groups_jsonb, level, exp, updated_at)
			VALUES($1, $2, FALSE, 0, $3, $4::jsonb, 1, 0, now())
			ON CONFLICT (kitty_id, skill_id)
			DO UPDATE SET
				rule_enabled = EXCLUDED.rule_enabled,
				rule_groups_jsonb = EXCLUDED.rule_groups_jsonb,
				updated_at = now()
		`, kittyID, skillID, enabled, string(raw))
		return err
	})
}

func (s *Store) ListKittySkillRuleSets(ctx context.Context, userID, kittyID int64) ([]RuleSet, error) {
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT skill_id, rule_enabled, rule_groups_jsonb
		FROM player_kitty_skills
		WHERE kitty_id = $1
		  AND jsonb_typeof(rule_groups_jsonb) = 'array'
		  AND jsonb_array_length(rule_groups_jsonb) > 0
		ORDER BY skill_id
	`, kittyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RuleSet, 0, 8)
	for rows.Next() {
		var (
			skillID string
			enabled bool
			raw     []byte
		)
		if err := rows.Scan(&skillID, &enabled, &raw); err != nil {
			return nil, err
		}
		var groups any
		_ = json.Unmarshal(raw, &groups)
		out = append(out, RuleSet{
			RuleKind: "skill",
			SlotKey:  strings.TrimSpace(skillID),
			Enabled:  enabled,
			Groups:   groups,
		})
	}
	return out, rows.Err()
}

func (s *Store) ListKittyEquippedItems(ctx context.Context, userID, kittyID int64) ([]EquippedItem, error) {
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT i.instance_id, i.template_item_id, COALESCE(i.custom_data, '{}'::jsonb), s.equip_type, i.enhance_level, i.affixes, i.durability, i.max_durability, i.state, s.slot, i.updated_at
		FROM player_kitty_equip_slots s
		JOIN player_item_instances i ON i.instance_id = s.instance_id
		WHERE s.kitty_id = $1
		ORDER BY i.instance_id DESC, s.equip_type ASC
	`, kittyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EquippedItem, 0, 8)
	for rows.Next() {
		var item EquippedItem
		var affixRaw []byte
		var customDataRaw []byte
		if err := rows.Scan(&item.InstanceID, &item.TemplateItemID, &customDataRaw, &item.EquipType, &item.EnhanceLevel, &affixRaw, &item.Durability, &item.MaxDurability, &item.State, &item.EquippedSlot, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.CustomData = decodeItemCustomization(customDataRaw)
		item.Affixes = decodeAffixSlots(affixRaw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) EquipKittyItem(ctx context.Context, userID, kittyID, instanceID int64, equipType string, occupySlots []string) error {
	if kittyID <= 0 || instanceID <= 0 || len(occupySlots) == 0 {
		return ErrEquipSlotNotAllowed
	}
	if strings.TrimSpace(equipType) == "" {
		equipType = "battle"
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.equipKittyItemTx(ctx, tx, userID, kittyID, instanceID, equipType, occupySlots)
	})
}

func (s *Store) equipKittyItemTx(ctx context.Context, tx pgx.Tx, userID, kittyID, instanceID int64, equipType string, occupySlots []string) error {
	if kittyID <= 0 || instanceID <= 0 || len(occupySlots) == 0 {
		return ErrEquipSlotNotAllowed
	}
	equipType = normalizeEquipType(equipType)
	if equipType == "" {
		return ErrEquipSlotNotAllowed
	}
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, false); err != nil {
		return err
	}
	var itemUserID int64
	var state string
	var targetAffixesRaw []byte
	if err := tx.QueryRow(ctx, `SELECT user_id, state, affixes FROM player_item_instances WHERE instance_id = $1 FOR UPDATE`, instanceID).Scan(&itemUserID, &state, &targetAffixesRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInstanceNotOwned
		}
		return err
	}
	if itemUserID != userID {
		return ErrInstanceNotOwned
	}
	if state != "inventory" && state != "equipped" {
		return &InvalidInstanceStateError{State: state}
	}
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, instanceID); err != nil {
		return err
	}

	targetAffixes := decodeAffixSlots(targetAffixesRaw)
	if err := s.ensureKittyEquipAffixBodyLimitsTx(ctx, tx, kittyID, instanceID, targetAffixes); err != nil {
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
		if err := sourceKittyRows.Scan(&sourceKittyID); err != nil {
			sourceKittyRows.Close()
			return err
		}
		sourceKittyIDs = append(sourceKittyIDs, sourceKittyID)
	}
	if err := sourceKittyRows.Err(); err != nil {
		sourceKittyRows.Close()
		return err
	}
	sourceKittyRows.Close()

	playerRows, err := tx.Query(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND instance_id = $2
		RETURNING user_id
	`, userID, instanceID)
	if err != nil {
		return err
	}
	playerEquipRemoved := false
	for playerRows.Next() {
		var removedUserID int64
		if err := playerRows.Scan(&removedUserID); err != nil {
			playerRows.Close()
			return err
		}
		if removedUserID == userID {
			playerEquipRemoved = true
		}
	}
	if err := playerRows.Err(); err != nil {
		playerRows.Close()
		return err
	}
	playerRows.Close()
	if _, err := tx.Exec(ctx, `DELETE FROM player_kitty_equip_slots WHERE instance_id = $1`, instanceID); err != nil {
		return err
	}
	displacedRows, err := tx.Query(ctx, `
		SELECT DISTINCT instance_id
		FROM player_kitty_equip_slots
		WHERE kitty_id = $1 AND equip_type = $2 AND slot = ANY($3::text[])
	`, kittyID, equipType, occupySlots)
	if err != nil {
		return err
	}
	displacedInstanceIDs := make([]int64, 0, len(occupySlots))
	for displacedRows.Next() {
		var displacedInstanceID int64
		if err := displacedRows.Scan(&displacedInstanceID); err != nil {
			displacedRows.Close()
			return err
		}
		if displacedInstanceID > 0 {
			displacedInstanceIDs = append(displacedInstanceIDs, displacedInstanceID)
		}
	}
	if err := displacedRows.Err(); err != nil {
		displacedRows.Close()
		return err
	}
	displacedRows.Close()
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, displacedInstanceIDs...); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_kitty_equip_slots
		WHERE kitty_id = $1 AND equip_type = $2 AND slot = ANY($3::text[])
	`, kittyID, equipType, occupySlots); err != nil {
		return err
	}
	for _, slot := range occupySlots {
		slot = strings.TrimSpace(slot)
		if slot == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_kitty_equip_slots(kitty_id, equip_type, slot, instance_id, updated_at)
			VALUES($1, $2, $3, $4, now())
		`, kittyID, equipType, slot, instanceID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE player_item_instances SET state = 'equipped', updated_at = now() WHERE instance_id = $1`, instanceID); err != nil {
		return err
	}
	for _, displacedInstanceID := range displacedInstanceIDs {
		if displacedInstanceID == instanceID {
			continue
		}
		if err := s.setInstanceInventoryWhenFullyUnequippedTx(ctx, tx, userID, displacedInstanceID); err != nil {
			return err
		}
	}
	if playerEquipRemoved {
		if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
			return err
		}
	}
	affectedKittyIDs := map[int64]struct{}{kittyID: {}}
	for _, sourceKittyID := range sourceKittyIDs {
		if sourceKittyID > 0 {
			affectedKittyIDs[sourceKittyID] = struct{}{}
		}
	}
	seeds := s.attributeSeedsFromGameData()
	for affectedKittyID := range affectedKittyIDs {
		if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, affectedKittyID, seeds); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureKittyEquipAffixBodyLimitsTx(
	ctx context.Context,
	tx pgx.Tx,
	kittyID int64,
	targetInstanceID int64,
	targetAffixes []EquipmentAffixSlot,
) error {
	bodySeen := map[string]int{}
	rows, err := tx.Query(ctx, `
		SELECT i.affixes
		FROM player_item_instances i
		JOIN player_kitty_equip_slots s ON s.instance_id = i.instance_id
		WHERE s.kitty_id = $1
		  AND i.state = 'equipped'
		  AND i.instance_id <> $2
		ORDER BY i.instance_id ASC
		FOR UPDATE
	`, kittyID, targetInstanceID)
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

func (s *Store) UnequipKittyInstance(ctx context.Context, userID, kittyID, instanceID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, false); err != nil {
			return err
		}
		if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, instanceID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM player_kitty_equip_slots WHERE kitty_id = $1 AND instance_id = $2`, kittyID, instanceID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrEquipSlotNotFound
		}
		if err := s.setInstanceInventoryWhenFullyUnequippedTx(ctx, tx, userID, instanceID); err != nil {
			return err
		}
		return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData())
	})
}

func (s *Store) setInstanceInventoryWhenFullyUnequippedTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64) error {
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, userID, instanceID); err != nil {
		return err
	}
	var stillEquipped bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM player_equip_slots WHERE instance_id = $1
			UNION ALL
			SELECT 1 FROM player_kitty_equip_slots WHERE instance_id = $1
		)
	`, instanceID).Scan(&stillEquipped); err != nil {
		return err
	}
	if stillEquipped {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET state = 'inventory', updated_at = now()
		WHERE instance_id = $1 AND user_id = $2
	`, instanceID, userID)
	return err
}

func (s *Store) attributeSeedsFromGameData() []PlayerAttributeSeed {
	seeds := make([]PlayerAttributeSeed, 0, len(s.gameData.Attributes))
	for _, row := range s.gameData.Attributes {
		seeds = append(seeds, PlayerAttributeSeed{
			AttrKey:                 row.ID,
			BaseDefault:             row.BaseDefault,
			GrowthType:              row.GrowthType,
			Unit:                    row.Unit,
			Category:                row.Category,
			Order:                   row.Order,
			NameKey:                 row.NameKey,
			DescKey:                 row.DescKey,
			IconID:                  row.IconID,
			EmojiIcon:               row.EmojiIcon,
			AllowBattlePointLevelUp: row.AllowBattlePointLevelUp,
		})
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].Order < seeds[j].Order })
	return seeds
}

// enrichPlayerKittyRow fills in derived fields after scanning a player_kitties row.
func (s *Store) enrichPlayerKittyRow(row *PlayerKitty, extraRaw []byte) {
	row.Name = strings.TrimSpace(row.Name)
	row.Status = normalizeKittyStatus(row.Status)
	row.BattleLevelCap = s.effectiveBattleLevelCap(row.BattleLevelCap, row.Level)
	row.Extra = decodeJSONMap(extraRaw)
	row.NextExp = s.KittyNextExp(row.Level)
}

// queryPersonalityIDs scans personality IDs from a tx/db Querier with fallback for legacy rows.
func (s *Store) queryPersonalityIDs(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, primarySQL string, fallbackSQL string, primaryArgs []any, fallbackArgs []any,
) ([]string, error) {
	rows, err := q.Query(ctx, primarySQL, primaryArgs...)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, normalizeLower(id))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(ids) > 0 {
		return normalizeUniqueIDs(ids), nil
	}
	if fallbackSQL == "" {
		return nil, nil
	}
	rows2, err := q.Query(ctx, fallbackSQL, fallbackArgs...)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	ids = ids[:0]
	for rows2.Next() {
		var id string
		if err := rows2.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, normalizeLower(id))
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}
	return normalizeUniqueIDs(ids), nil
}

const personalityPrimarySQL = `
	SELECT personality_id
	FROM player_kitty_personalities
	WHERE user_id = $1 AND kitty_id = $2
	ORDER BY created_at ASC, personality_id ASC`

const personalityFallbackSQL = `
	SELECT personality_id
	FROM player_kitty_personalities
	WHERE kitty_id = $1
	  AND EXISTS (SELECT 1 FROM player_kitties WHERE id = $1 AND user_id = $2)
	ORDER BY created_at ASC, personality_id ASC`

// fetchPlayerKittyTx fetches a kitty within an existing transaction. No FOR UPDATE (read-only).
func (s *Store) fetchPlayerKittyTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64) (PlayerKitty, error) {
	var row PlayerKitty
	var extraRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT id, name, status, level, battle_level_cap, exp, total_exp, bond_exp, mood, extra_jsonb, created_at
		FROM player_kitties
		WHERE user_id = $1 AND id = $2
	`, userID, kittyID).Scan(&row.ID, &row.Name, &row.Status, &row.Level, &row.BattleLevelCap, &row.Exp, &row.TotalExp, &row.BondExp, &row.Mood, &extraRaw, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerKitty{}, ErrKittyNotFound
	}
	if err != nil {
		return PlayerKitty{}, err
	}
	s.enrichPlayerKittyRow(&row, extraRaw)
	return row, nil
}

// fetchKittyPersonalityIDsTx fetches personality IDs within an existing transaction.
func (s *Store) fetchKittyPersonalityIDsTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64) ([]string, error) {
	return s.queryPersonalityIDs(ctx, tx, personalityPrimarySQL, personalityFallbackSQL,
		[]any{userID, kittyID}, []any{kittyID, userID})
}

func (s *Store) GetKittyCombatProfile(ctx context.Context, userID, kittyID int64, seeds []PlayerAttributeSeed) (KittyCombatProfile, error) {
	var profile KittyCombatProfile
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// 1. Fetch kitty once (FOR UPDATE for consistent snapshot)
		kitty, err := s.fetchPlayerKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return err
		}
		personalityIDs, err := s.fetchKittyPersonalityIDsTx(ctx, tx, userID, kittyID)
		if err != nil {
			return err
		}
		kitty.Extra = setKittyPersonalityIDsExtra(kitty.Extra, personalityIDs)

		// 2. Recompute derived stats (internally ensures attributes + FOR UPDATE + batch UPDATE)
		if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds); err != nil {
			return err
		}

		// 3. Fetch attributes (enriched with seed metadata)
		attrs, err := s.listKittyAttributesTx(ctx, tx, kittyID, seeds)
		if err != nil {
			return err
		}

		// 4. Fetch stats cache
		var stats PlayerStats
		var statsRaw []byte
		statsErr := tx.QueryRow(ctx, `
			SELECT stats_jsonb, version, updated_at
			FROM player_kitty_stats_cache WHERE kitty_id = $1
		`, kittyID).Scan(&statsRaw, &stats.CacheVer, &stats.UpdatedAt)
		if statsErr == nil {
			_ = json.Unmarshal(statsRaw, &stats.Derived)
		}

		// 5. Fetch skills
		skillRows, err := tx.Query(ctx, `
			SELECT skill_id, enabled, cast_order, rule_enabled, rule_groups_jsonb, level, exp, updated_at
			FROM player_kitty_skills WHERE kitty_id = $1
			ORDER BY cast_order ASC, skill_id ASC
		`, kittyID)
		if err != nil {
			return err
		}
		skills := make([]PlayerSkill, 0, 8)
		for skillRows.Next() {
			var row PlayerSkill
			var raw []byte
			if err := skillRows.Scan(&row.SkillID, &row.Enabled, &row.CastOrder, &row.RuleEnabled, &raw, &row.Level, &row.Exp, &row.UpdatedAt); err != nil {
				skillRows.Close()
				return err
			}
			var groups any
			_ = json.Unmarshal(raw, &groups)
			row.RuleGroups = groups
			skills = append(skills, row)
		}
		if err := skillRows.Err(); err != nil {
			skillRows.Close()
			return err
		}
		skillRows.Close()

		// 6. Fetch equipped items
		equipRows, err := tx.Query(ctx, `
			SELECT i.instance_id, i.template_item_id, s.equip_type, i.enhance_level, i.affixes,
			       i.durability, i.max_durability, i.state, s.slot, i.updated_at
			FROM player_kitty_equip_slots s
			JOIN player_item_instances i ON i.instance_id = s.instance_id
			WHERE s.kitty_id = $1
			ORDER BY i.instance_id DESC, s.equip_type ASC
		`, kittyID)
		if err != nil {
			return err
		}
		equipment := make([]EquippedItem, 0, 8)
		for equipRows.Next() {
			var item EquippedItem
			var affixRaw []byte
			if err := equipRows.Scan(&item.InstanceID, &item.TemplateItemID, &item.EquipType, &item.EnhanceLevel, &affixRaw, &item.Durability, &item.MaxDurability, &item.State, &item.EquippedSlot, &item.UpdatedAt); err != nil {
				equipRows.Close()
				return err
			}
			item.Affixes = decodeAffixSlots(affixRaw)
			equipment = append(equipment, item)
		}
		if err := equipRows.Err(); err != nil {
			equipRows.Close()
			return err
		}
		equipRows.Close()

		profile = KittyCombatProfile{Kitty: kitty, Attributes: attrs, Stats: stats, Skills: skills, Equipment: equipment}
		return nil
	})
	return profile, err
}

// listKittyAttributesTx fetches attributes within a transaction, enriched with seed metadata.
func (s *Store) listKittyAttributesTx(ctx context.Context, tx pgx.Tx, kittyID int64, seeds []PlayerAttributeSeed) ([]PlayerAttributeState, error) {
	rows, err := tx.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level, attr_exp, source_mods_jsonb, final_value, lock_version
		FROM player_kitty_attributes WHERE kitty_id = $1
	`, kittyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seedMap := make(map[string]PlayerAttributeSeed, len(seeds))
	for _, seed := range seeds {
		key := strings.TrimSpace(seed.AttrKey)
		if key != "" {
			seedMap[key] = seed
		}
	}
	out := make([]PlayerAttributeState, 0, len(seeds))
	for rows.Next() {
		var item PlayerAttributeState
		var raw []byte
		if err := rows.Scan(&item.AttrKey, &item.BaseLevel, &item.SkillPointLevel, &item.AttrExp, &raw, &item.FinalValue, &item.LockVersion); err != nil {
			return nil, err
		}
		item.SourceMods = map[string]any{}
		_ = json.Unmarshal(raw, &item.SourceMods)
		if seed, ok := seedMap[item.AttrKey]; ok {
			item.BaseDefault = s.kittyAttributeBaseDefault(item.AttrKey, seed.BaseDefault)
			item.GrowthType = seed.GrowthType
			item.Unit = seed.Unit
			item.Category = seed.Category
			item.Order = seed.Order
			item.NameKey = seed.NameKey
			item.DescKey = seed.DescKey
			item.IconID = seed.IconID
			item.EmojiIcon = seed.EmojiIcon
			item.AllowBattlePointLevelUp = seed.AllowBattlePointLevelUp
		}
		if item.GrowthType == "levelable" {
			progress, progressErr := s.resolveAttributeProgress(item.AttrKey, item.AttrExp)
			if progressErr != nil {
				return nil, progressErr
			}
			item.ProgressLevel = progress.Level
			item.ProgressExp = int64(math.Round(progress.Exp))
			item.ProgressNextExp = int64(math.Round(progress.NextExp))
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func sortedBattleRoomUnits(units []BattleRoomUnit) {
	sort.Slice(units, func(i, j int) bool {
		if units[i].JoinedAt.Equal(units[j].JoinedAt) {
			if units[i].OwnerUserID == units[j].OwnerUserID {
				if units[i].UnitType == units[j].UnitType {
					return units[i].UnitID < units[j].UnitID
				}
				return units[i].UnitType == "player"
			}
			return units[i].OwnerUserID < units[j].OwnerUserID
		}
		return units[i].JoinedAt.Before(units[j].JoinedAt)
	})
}
