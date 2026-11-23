package store

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var itemRenameAllowedStates = map[string]struct{}{
	"inventory":      {},
	"equipped":       {},
	"auction":        {},
	fishingPondState: {},
}

func (s *Store) ItemRenameMaxNameRunes() int {
	if s.config.ItemRenameMaxNameRunes <= 0 {
		return 16
	}
	return s.config.ItemRenameMaxNameRunes
}

func (s *Store) ItemRenameCost() map[string]int64 {
	if len(s.config.ItemRenameCost) == 0 {
		return map[string]int64{"harmonic_embryo": 1}
	}
	out := make(map[string]int64, len(s.config.ItemRenameCost))
	for itemID, count := range s.config.ItemRenameCost {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || count <= 0 {
			continue
		}
		out[itemID] += count
	}
	if len(out) == 0 {
		out["harmonic_embryo"] = 1
	}
	return out
}

func (s *Store) ItemRenameLabelColors() []string {
	return normalizeItemRenameLabelColors(s.config.ItemRenameLabelColors)
}

func (s *Store) DefaultItemRenameLabelColor() string {
	colors := s.ItemRenameLabelColors()
	if len(colors) == 0 {
		return "gold"
	}
	return colors[0]
}

func (s *Store) IsValidItemRenameLabelColor(labelColor string) bool {
	labelColor = strings.TrimSpace(strings.ToLower(labelColor))
	if labelColor == "" {
		return false
	}
	for _, allowed := range s.ItemRenameLabelColors() {
		if labelColor == allowed {
			return true
		}
	}
	return false
}

func (s *Store) RenameItemInstance(ctx context.Context, userID, instanceID int64, customName string, labelColor string) (ItemInstance, error) {
	customName = strings.TrimSpace(customName)
	labelColor = strings.TrimSpace(strings.ToLower(labelColor))
	if labelColor == "" {
		labelColor = s.DefaultItemRenameLabelColor()
	}
	if customName == "" || utf8.RuneCountInString(customName) > s.ItemRenameMaxNameRunes() || !IsValidItemCustomName(customName) {
		return ItemInstance{}, ErrItemRenameInvalidName
	}
	if !s.IsValidItemRenameLabelColor(labelColor) {
		return ItemInstance{}, ErrItemRenameInvalidName
	}
	cost := s.ItemRenameCost()
	if len(cost) == 0 {
		return ItemInstance{}, ErrItemRenameCostInvalid
	}
	nextCustomData := ItemCustomization{Name: customName, LabelColor: labelColor}
	nextCustomDataRaw, err := encodeItemCustomization(nextCustomData)
	if err != nil {
		return ItemInstance{}, err
	}

	var renamed ItemInstance
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		row, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if row.UserID != userID {
			return ErrInstanceNotOwned
		}
		if !itemInstanceSupportsRename(row.TemplateItem) {
			return ErrItemRenameInvalidName
		}
		if _, ok := itemRenameAllowedStates[strings.TrimSpace(row.State)]; !ok {
			return ErrInvalidInstanceState
		}
		if s.itemStackable(row.TemplateItem) {
			return ErrItemRenameStackableItem
		}
		if normalizeItemCustomization(row.CustomData) == nextCustomData {
			return ErrItemRenameSameName
		}
		for itemID, count := range cost {
			if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, count, InventoryLedgerMeta{
				SourceSurface: "inventory",
				TargetSurface: "item_rename",
				Details: map[string]any{
					"type":       "item_rename_cost",
					"instanceId": instanceID,
				},
			}); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE player_item_instances
			SET custom_data = $1::jsonb, updated_at = now()
			WHERE instance_id = $2 AND user_id = $3
		`, string(nextCustomDataRaw), instanceID, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE market_auctions
			SET instance_custom_data = $1::jsonb, updated_at = now()
			WHERE instance_id = $2 AND seller_id = $3 AND status = 'active'
		`, string(nextCustomDataRaw), instanceID, userID); err != nil {
			return err
		}
		renamed, _, err = s.getItemInstanceTx(ctx, tx, userID, instanceID)
		return err
	})
	if err != nil {
		return ItemInstance{}, err
	}
	return renamed, nil
}

func itemInstanceSupportsRename(templateItem string) bool {
	switch strings.TrimSpace(templateItem) {
	case "forged_skillbook":
		return false
	default:
		return true
	}
}

func normalizeItemRenameLabelColors(values []string) []string {
	defaults := []string{"gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia"}
	if len(values) == 0 {
		return defaults
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(strings.ToLower(value))
		if normalized == "" || !isSupportedItemRenameLabelColor(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return defaults
	}
	return out
}

func isSupportedItemRenameLabelColor(value string) bool {
	switch value {
	case "gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia":
		return true
	default:
		return false
	}
}

func normalizeItemCustomization(data ItemCustomization) ItemCustomization {
	return ItemCustomization{
		Name:       strings.TrimSpace(data.Name),
		LabelColor: strings.TrimSpace(strings.ToLower(data.LabelColor)),
	}
}

func encodeItemCustomization(data ItemCustomization) ([]byte, error) {
	normalized := normalizeItemCustomization(data)
	if normalized.Name == "" && normalized.LabelColor == "" {
		return []byte("{}"), nil
	}
	return json.Marshal(normalized)
}

func mustEncodeItemCustomizationString(data ItemCustomization) string {
	raw, err := encodeItemCustomization(data)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func decodeItemCustomization(raw []byte) ItemCustomization {
	if len(raw) == 0 {
		return ItemCustomization{}
	}
	var data ItemCustomization
	if err := json.Unmarshal(raw, &data); err != nil {
		return ItemCustomization{}
	}
	return normalizeItemCustomization(data)
}

func itemCustomizationEmpty(data ItemCustomization) bool {
	data = normalizeItemCustomization(data)
	return data.Name == "" && data.LabelColor == ""
}

func IsValidItemCustomName(name string) bool {
	if name == "" {
		return false
	}
	seenVisible := false
	previousEmoji := false
	previousJoiner := false
	for _, r := range name {
		switch {
		case isItemCustomNameTextRune(r):
			if previousJoiner {
				return false
			}
			seenVisible = true
			previousEmoji = false
			previousJoiner = false
		case isEmojiModifierRune(r):
			if !previousEmoji {
				return false
			}
			previousJoiner = false
		case isEmojiVariationRune(r):
			if !previousEmoji {
				return false
			}
			previousJoiner = false
		case r == '\u200d':
			if !previousEmoji || previousJoiner {
				return false
			}
			previousEmoji = false
			previousJoiner = true
		case isEmojiBaseRune(r):
			seenVisible = true
			previousEmoji = true
			previousJoiner = false
		default:
			return false
		}
	}
	return seenVisible && !previousJoiner
}

func isItemCustomNameTextRune(r rune) bool {
	return r == '_' ||
		(r >= '0' && r <= '9') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

func isEmojiBaseRune(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF)
}

func isEmojiModifierRune(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

func isEmojiVariationRune(r rune) bool {
	return r == '\ufe0f'
}

func (s *Store) getItemInstanceTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64) (ItemInstance, string, error) {
	var it ItemInstance
	var baseRaw []byte
	var affixRaw []byte
	var customDataRaw []byte
	var equippedSlot string
	err := tx.QueryRow(ctx, `
		SELECT i.instance_id, i.user_id, i.template_item_id, COALESCE(i.custom_data, '{}'::jsonb), i.quantity, i.enhance_level, i.base_snapshot, i.affixes, i.durability, i.max_durability, i.state, i.created_at, i.updated_at,
			       COALESCE((
			           SELECT string_agg(slot, ',' ORDER BY slot)
			           FROM player_equip_slots s
			           WHERE s.user_id = i.user_id AND s.instance_id = i.instance_id
			       ), '')
		FROM player_item_instances i
		WHERE i.user_id = $1 AND i.instance_id = $2
	`, userID, instanceID).Scan(
		&it.InstanceID,
		&it.UserID,
		&it.TemplateItemID,
		&customDataRaw,
		&it.Quantity,
		&it.EnhanceLevel,
		&baseRaw,
		&affixRaw,
		&it.Durability,
		&it.MaxDurability,
		&it.State,
		&it.CreatedAt,
		&it.UpdatedAt,
		&equippedSlot,
	)
	if err != nil {
		return ItemInstance{}, "", err
	}
	_ = json.Unmarshal(baseRaw, &it.BaseSnapshot)
	it.CustomData = decodeItemCustomization(customDataRaw)
	it.Affixes = decodeAffixSlots(affixRaw)
	return it, equippedSlot, nil
}
