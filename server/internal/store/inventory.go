package store

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const GoldCoinItemID = "gold_coin"

const InventoryGrantPolicyRollDefaultAffixes = "rollDefaultAffixes"

type InventoryGrantOptions struct {
	Policy                    string
	DefaultAffixRarityWeights map[string]float64
	RNG                       *rand.Rand
}

func normalizeStackableFallback(itemID string) bool {
	// Stack behavior is config-driven in shared game-data.
	// Fallback safe set for catalog-missing items.
	// equipment/special items are non-stackable; consumable/currency-like ids are stackable.
	switch itemID {
	case "wood_sword", "iron_sword", "iron_armor", "cloth_armor", "training_warhammer", "guardian_ring":
		return false
	case "craftsmans_hammer", "hammer_of_divine_work", "engrave_stone":
		return false
	default:
		return true
	}
}

func (s *Store) itemStackable(itemID string) bool {
	if item := s.gameData.ItemByID(itemID); item != nil {
		return item.Stackable
	}
	return normalizeStackableFallback(itemID)
}

func (s *Store) defaultMaxDurability(itemID string) int64 {
	template := s.gameData.EquipmentTemplateByID(itemID)
	if template != nil && template.MaxDurability > 0 {
		return template.MaxDurability
	}
	return 100
}

func inventoryGrantLayout(stackable bool, qty int64) (rows int64, qtyPerRow int64, err error) {
	if qty <= 0 {
		return 0, 0, ErrBadQuantity
	}
	if stackable {
		return 1, qty, nil
	}
	return qty, 1, nil
}

func shouldMergeInventoryStack(stackable bool) bool {
	return stackable
}

// InventoryQuantityTx returns the total quantity of a stackable item in a user's inventory.
func (s *Store) InventoryQuantityTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string) (int64, error) {
	return s.inventoryQuantityTx(ctx, tx, userID, itemID)
}

func (s *Store) inventoryQuantityTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string) (int64, error) {
	var quantity int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM player_item_instances
		WHERE user_id = $1 AND template_item_id = $2 AND state = 'inventory'
	`, userID, itemID).Scan(&quantity)
	return quantity, err
}

// AddInventoryItemTx is the central inventory add entry used by gameplay modules.
func (s *Store) AddInventoryItemTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64) error {
	return s.AddInventoryItemWithOptionsTx(ctx, tx, userID, itemID, qty, InventoryGrantOptions{})
}

func (s *Store) AddInventoryItemWithOptionsTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, options InventoryGrantOptions) error {
	options = normalizeInventoryGrantOptions(options)
	stackable := s.itemStackable(itemID)
	perInstanceAffixes := inventoryGrantRollsDefaultAffixes(options)
	var defaultAffixes []EquipmentAffixSlot
	if !perInstanceAffixes {
		defaultAffixes = s.templateDefaultAffixSlots(itemID)
	}
	if len(defaultAffixes) > 0 {
		// Affix-bearing instances are always persisted as independent rows.
		stackable = false
	}
	if perInstanceAffixes {
		stackable = false
	}
	rowsToInsert, qtyPerRow, err := inventoryGrantLayout(stackable, qty)
	if err != nil {
		return err
	}
	maxDurability := s.defaultMaxDurability(itemID)
	if shouldMergeInventoryStack(stackable) {
		rows, err := s.lockUnarchivedTemplateCommodityRowsByTemplateIDsTx(ctx, tx, userID, []string{itemID})
		if err != nil {
			return err
		}
		type stackRow struct {
			id  int64
			qty int64
		}
		stackRows := make([]stackRow, 0, 8)
		for _, row := range rows {
			stackRows = append(stackRows, stackRow{id: row.InstanceID, qty: row.Quantity})
		}
		if len(stackRows) > 0 {
			mergedQty := qty
			for _, sr := range stackRows {
				if sr.qty > math.MaxInt64-mergedQty {
					return ErrInventoryQuantityOverflow
				}
				mergedQty += sr.qty
			}
			keepID := stackRows[0].id
			if _, err = tx.Exec(ctx, `
					UPDATE player_item_instances
					SET quantity = $1,
					    durability = $2,
					    max_durability = $2,
					    base_snapshot = '{}'::jsonb,
					    affixes = '[]'::jsonb,
					    updated_at = now()
					WHERE instance_id = $3
				`, mergedQty, maxDurability, keepID); err != nil {
				return err
			}
			for _, sr := range stackRows[1:] {
				if _, err = tx.Exec(ctx, `DELETE FROM player_item_instances WHERE instance_id = $1`, sr.id); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if perInstanceAffixes {
		for i := int64(0); i < rowsToInsert; i++ {
			affixes, _, err := s.resolveGrantDefaultAffixSlots(itemID, options)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `
				INSERT INTO player_item_instances(
					user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
					durability, max_durability, quantity, state, created_at, updated_at
				)
				VALUES($1, $2, 0, 0, '{}'::jsonb, $4::jsonb, $3, $3, $5, 'inventory', now(), now())
			`, userID, itemID, maxDurability, string(encodeAffixSlots(affixes)), qtyPerRow); err != nil {
				return err
			}
		}
		return nil
	}
	affixRaw := string(encodeAffixSlots(defaultAffixes))
	_, err = tx.Exec(ctx, `
		INSERT INTO player_item_instances(
			user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
			durability, max_durability, quantity, state, created_at, updated_at
		)
		SELECT $1, $2, 0, 0, '{}'::jsonb, $4::jsonb, $3, $3, $5, 'inventory', now(), now()
		FROM generate_series(1, $6)
	`, userID, itemID, maxDurability, affixRaw, qtyPerRow, rowsToInsert)
	return err
}

func (s *Store) AddInventoryItemInstanceTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, baseSnapshot map[string]any, affixes []EquipmentAffixSlot) (int64, error) {
	if qty != 1 {
		return 0, ErrBadQuantity
	}
	if baseSnapshot == nil {
		baseSnapshot = map[string]any{}
	}
	if len(affixes) == 0 {
		affixes = s.templateDefaultAffixSlots(itemID)
	}
	baseRaw, err := json.Marshal(baseSnapshot)
	if err != nil {
		return 0, err
	}
	maxDurability := s.defaultMaxDurability(itemID)
	var instanceID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO player_item_instances(
			user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
			durability, max_durability, quantity, state, created_at, updated_at
		)
		VALUES($1, $2, 0, 0, $3::jsonb, $4::jsonb, $5, $5, $6, 'inventory', now(), now())
		RETURNING instance_id
	`, userID, itemID, string(baseRaw), string(encodeAffixSlots(affixes)), maxDurability, qty).Scan(&instanceID)
	return instanceID, err
}

func (s *Store) AddInventoryEnhancedInstanceTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, enhanceLevel int) (int64, error) {
	return s.AddInventoryEnhancedInstanceWithOptionsTx(ctx, tx, userID, itemID, enhanceLevel, InventoryGrantOptions{})
}

func (s *Store) AddInventoryEnhancedInstanceWithOptionsTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, enhanceLevel int, options InventoryGrantOptions) (int64, error) {
	options = normalizeInventoryGrantOptions(options)
	if enhanceLevel <= 0 {
		return 0, ErrBadQuantity
	}
	maxDurability := s.defaultMaxDurability(itemID)
	defaultAffixes, _, err := s.resolveGrantDefaultAffixSlots(itemID, options)
	if err != nil {
		return 0, err
	}
	var instanceID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO player_item_instances(
			user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
			durability, max_durability, quantity, state, created_at, updated_at
		)
		VALUES($1, $2, $3, 0, '{}'::jsonb, $4::jsonb, $5, $5, 1, 'inventory', now(), now())
		RETURNING instance_id
	`, userID, itemID, enhanceLevel, string(encodeAffixSlots(defaultAffixes)), maxDurability).Scan(&instanceID); err != nil {
		return 0, err
	}
	locked, err := s.LockInstanceTx(ctx, tx, instanceID)
	if err != nil {
		return 0, err
	}
	if err := s.refreshLockedInstanceBaseSnapshotTx(ctx, tx, locked); err != nil {
		return 0, err
	}
	return instanceID, nil
}

func (s *Store) resolveGrantDefaultAffixSlots(itemID string, options InventoryGrantOptions) ([]EquipmentAffixSlot, bool, error) {
	if !inventoryGrantRollsDefaultAffixes(options) {
		return s.templateDefaultAffixSlots(itemID), false, nil
	}
	affixes, err := s.rollTemplateDefaultAffixSlots(itemID, options.DefaultAffixRarityWeights, options.RNG)
	if err != nil {
		return nil, true, err
	}
	return affixes, true, nil
}

func inventoryGrantRollsDefaultAffixes(options InventoryGrantOptions) bool {
	return strings.TrimSpace(options.Policy) == InventoryGrantPolicyRollDefaultAffixes
}

func normalizeInventoryGrantOptions(options InventoryGrantOptions) InventoryGrantOptions {
	if inventoryGrantRollsDefaultAffixes(options) && options.RNG == nil {
		options.RNG = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return options
}

func (s *Store) rollTemplateDefaultAffixSlots(itemID string, rarityWeights map[string]float64, rng *rand.Rand) ([]EquipmentAffixSlot, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(itemID))
	if template == nil || len(template.DefaultAffixSlots) == 0 || template.AffixSlotCount <= 0 {
		return nil, ErrAffixInvalidCarrier
	}
	weights := normalizedPositiveWeights(rarityWeights)
	if len(weights) == 0 {
		return nil, ErrAffixEnchantNoRarityCandidate
	}
	out := make([]EquipmentAffixSlot, 0, len(template.DefaultAffixSlots))
	for _, entry := range template.DefaultAffixSlots {
		affixID := strings.TrimSpace(entry.AffixID)
		if entry.SlotIndex < 0 || int64(entry.SlotIndex) >= template.AffixSlotCount || affixID == "" {
			return nil, ErrAffixInvalidCarrier
		}
		def := s.gameData.AffixDefinitionByID(affixID)
		if def == nil || !def.Active {
			return nil, ErrAffixInvalidCarrier
		}
		compatibleWeights := make(map[string]float64, len(weights))
		for rarityID, weight := range weights {
			if affixSupportsRarity(*def, rarityID) {
				compatibleWeights[rarityID] = weight
			}
		}
		rarityID := rollByWeight(compatibleWeights, rng)
		if rarityID == "" {
			return nil, ErrAffixEnchantNoRarityCandidate
		}
		out = append(out, buildEquipmentAffixSlot(*def, rarityID, rng.Float64(), entry.SlotIndex))
	}
	if len(out) == 0 {
		return nil, ErrAffixInvalidCarrier
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].SlotIndex < out[j].SlotIndex
	})
	return out, nil
}

func normalizedPositiveWeights(raw map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(raw))
	for rarityID, weight := range raw {
		normalized := strings.TrimSpace(rarityID)
		if normalized == "" || weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			continue
		}
		out[normalized] = weight
	}
	return out
}

func (s *Store) templateDefaultAffixSlots(itemID string) []EquipmentAffixSlot {
	template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(itemID))
	if template == nil || len(template.DefaultAffixSlots) == 0 || template.AffixSlotCount <= 0 {
		return nil
	}
	out := make([]EquipmentAffixSlot, 0, len(template.DefaultAffixSlots))
	for _, entry := range template.DefaultAffixSlots {
		affixID := strings.TrimSpace(entry.AffixID)
		rarityID := strings.TrimSpace(entry.RarityID)
		if entry.SlotIndex < 0 || affixID == "" || rarityID == "" {
			continue
		}
		if int64(entry.SlotIndex) >= template.AffixSlotCount {
			continue
		}
		def := s.gameData.AffixDefinitionByID(affixID)
		if def == nil || !def.Active || !affixSupportsRarity(*def, rarityID) {
			continue
		}
		out = append(out, buildEquipmentAffixSlot(*def, rarityID, entry.RollValue, entry.SlotIndex))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SlotIndex != out[j].SlotIndex {
			return out[i].SlotIndex < out[j].SlotIndex
		}
		return out[i].AffixID < out[j].AffixID
	})
	return out
}

// RemoveInventoryItemTx is the central inventory remove entry used by gameplay modules.
func (s *Store) RemoveInventoryItemTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64) error {
	if qty <= 0 {
		return ErrBadQuantity
	}
	stackRows, err := s.lockTemplateCommodityRowsByTemplateIDsTx(ctx, tx, userID, []string{itemID})
	if err != nil {
		return err
	}
	var owned int64
	for _, row := range stackRows {
		if row.Quantity > math.MaxInt64-owned {
			return ErrInventoryQuantityOverflow
		}
		owned += row.Quantity
	}
	if owned < qty {
		return ErrInsufficientItem
	}
	remain := qty
	for _, row := range stackRows {
		if remain <= 0 {
			break
		}
		if row.Quantity > remain {
			if _, err = tx.Exec(ctx, `
				UPDATE player_item_instances
				SET quantity = quantity - $1, updated_at = now()
				WHERE instance_id = $2
			`, remain, row.InstanceID); err != nil {
				return err
			}
			remain = 0
			break
		}
		if _, err = tx.Exec(ctx, `DELETE FROM player_item_instances WHERE instance_id = $1`, row.InstanceID); err != nil {
			return err
		}
		remain -= row.Quantity
	}
	if remain != 0 {
		return ErrInsufficientItem
	}
	return nil
}
