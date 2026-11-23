package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Cosmetic category constants.
const (
	CosmeticCategoryTitlePrefix = "title_prefix"
	CosmeticCategoryTitleSuffix = "title_suffix"
	CosmeticCategoryAvatar      = "avatar"
	CosmeticCategoryAvatarFrame = "avatar_frame"
)

// Wardrobe owner types.
const (
	WardrobeOwnerPlayer = "player"
	WardrobeOwnerKitty  = "kitty"
)

// Equip slot constants.
const (
	EquipSlotTitlePrefix      = "title_prefix"
	EquipSlotTitleSuffix      = "title_suffix"
	EquipSlotKittyTitlePrefix = "kitty_title_prefix"
	EquipSlotKittyTitleSuffix = "kitty_title_suffix"
	EquipSlotKittyAvatar      = "kitty_avatar"
	EquipSlotKittyAvatarFrame = "kitty_avatar_frame"
)

// playerEquipSlots are slots owned by the player (owner_type='player', owner_id=0).
var playerEquipSlots = map[string]string{
	EquipSlotTitlePrefix: CosmeticCategoryTitlePrefix,
	EquipSlotTitleSuffix: CosmeticCategoryTitleSuffix,
}

// kittyEquipSlots are slots owned by a kitty (owner_type='kitty', owner_id=kittyID).
var kittyEquipSlots = map[string]string{
	EquipSlotKittyTitlePrefix: CosmeticCategoryTitlePrefix,
	EquipSlotKittyTitleSuffix: CosmeticCategoryTitleSuffix,
	EquipSlotKittyAvatar:      CosmeticCategoryAvatar,
	EquipSlotKittyAvatarFrame: CosmeticCategoryAvatarFrame,
}

// validCosmeticCategories is the whitelist of allowed cosmetic categories.
var validCosmeticCategories = map[string]bool{
	CosmeticCategoryTitlePrefix: true,
	CosmeticCategoryTitleSuffix: true,
	CosmeticCategoryAvatar:      true,
	CosmeticCategoryAvatarFrame: true,
}

// slotToCategory maps a slot_id to its cosmetic category.
func slotToCategory(slotID string) (string, bool) {
	if cat, ok := playerEquipSlots[slotID]; ok {
		return cat, true
	}
	if cat, ok := kittyEquipSlots[slotID]; ok {
		return cat, true
	}
	return "", false
}

func validEquipSlot(slotID string) bool {
	_, ok := playerEquipSlots[slotID]
	return ok
}

func validKittySlot(slotID string) bool {
	_, ok := kittyEquipSlots[slotID]
	return ok
}

func validCosmeticCategory(category string) bool {
	return validCosmeticCategories[category]
}

// PlayerWardrobeItem represents one owned cosmetic.
type PlayerWardrobeItem struct {
	Category   string    `json:"category"`
	ItemID     string    `json:"itemId"`
	Source     string    `json:"source"`
	ObtainedAt time.Time `json:"obtainedAt"`
}

// WardrobeEquipEntry represents one equipped slot.
type WardrobeEquipEntry struct {
	OwnerType string `json:"ownerType"`
	OwnerID   int64  `json:"ownerId"`
	SlotID    string `json:"slotId"`
	ItemID    string `json:"itemId"`
}

// WardrobeState is the full wardrobe for a player.
type WardrobeState struct {
	Owned    []PlayerWardrobeItem `json:"owned"`
	Equipped []WardrobeEquipEntry `json:"equipped"`
}

// CosmeticGrant is a single grant request.
type CosmeticGrant struct {
	Category string
	ItemID   string
	Source   string
}

// GetWardrobe returns all owned cosmetics and equipped slots for a user.
func (s *Store) GetWardrobe(ctx context.Context, userID int64) (WardrobeState, error) {
	var state WardrobeState
	err := s.db.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1`, userID).Scan(new(int))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return state, fmt.Errorf("wardrobe.get: user %d not found", userID)
		}
		return state, fmt.Errorf("wardrobe.get: verify user: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT category, item_id, source, obtained_at
		FROM player_wardrobe_items
		WHERE user_id = $1
		ORDER BY category, obtained_at
	`, userID)
	if err != nil {
		return state, fmt.Errorf("wardrobe.get: query owned: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item PlayerWardrobeItem
		if err := rows.Scan(&item.Category, &item.ItemID, &item.Source, &item.ObtainedAt); err != nil {
			return state, fmt.Errorf("wardrobe.get: scan owned: %w", err)
		}
		state.Owned = append(state.Owned, item)
	}
	if err := rows.Err(); err != nil {
		return state, fmt.Errorf("wardrobe.get: iter owned: %w", err)
	}

	equipRows, err := s.db.Query(ctx, `
		SELECT owner_type, owner_id, slot_id, item_id
		FROM wardrobe_equip_slots
		WHERE user_id = $1
		ORDER BY owner_type, owner_id, slot_id
	`, userID)
	if err != nil {
		return state, fmt.Errorf("wardrobe.get: query equipped: %w", err)
	}
	defer equipRows.Close()

	for equipRows.Next() {
		var entry WardrobeEquipEntry
		if err := equipRows.Scan(&entry.OwnerType, &entry.OwnerID, &entry.SlotID, &entry.ItemID); err != nil {
			return state, fmt.Errorf("wardrobe.get: scan equipped: %w", err)
		}
		state.Equipped = append(state.Equipped, entry)
	}
	if err := equipRows.Err(); err != nil {
		return state, fmt.Errorf("wardrobe.get: iter equipped: %w", err)
	}

	if state.Owned == nil {
		state.Owned = []PlayerWardrobeItem{}
	}
	if state.Equipped == nil {
		state.Equipped = []WardrobeEquipEntry{}
	}
	return state, nil
}

// GrantCosmetic grants a single cosmetic to a player (idempotent).
// Returns alreadyOwned=true if the player already owns the item.
func (s *Store) GrantCosmetic(ctx context.Context, userID int64, grant CosmeticGrant) (alreadyOwned bool, err error) {
	if !validCosmeticCategory(grant.Category) {
		return false, ErrCosmeticCategoryInvalid
	}
	if grant.ItemID == "" {
		return false, fmt.Errorf("wardrobe.grant: empty item_id")
	}

	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.grant: lock user: %w", err)
		}
		var granted bool
		granted, err = s.grantCosmeticTx(ctx, tx, userID, grant)
		if err != nil {
			return err
		}
		alreadyOwned = !granted
		return nil
	})
	return alreadyOwned, err
}

// grantCosmeticTx performs the actual insert within an existing tx.
// Returns granted=true if a new row was inserted, false if already existed.
func (s *Store) grantCosmeticTx(ctx context.Context, tx pgx.Tx, userID int64, grant CosmeticGrant) (bool, error) {
	var insertedID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO player_wardrobe_items (user_id, category, item_id, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, category, item_id) DO NOTHING
		RETURNING user_id
	`, userID, grant.Category, grant.ItemID, grant.Source).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("wardrobe.grant: insert: %w", err)
	}
	return true, nil
}

// GrantCosmeticTx grants a cosmetic within an external transaction.
// For use by other services that need cross-feature atomicity.
// Caller MUST call s.lockUserTx(ctx, tx, userID) before invoking this method
// to ensure concurrency safety — this method does NOT acquire the lock itself.
func (s *Store) GrantCosmeticTx(ctx context.Context, tx pgx.Tx, userID int64, grant CosmeticGrant) (alreadyOwned bool, err error) {
	if !validCosmeticCategory(grant.Category) {
		return false, ErrCosmeticCategoryInvalid
	}
	if grant.ItemID == "" {
		return false, fmt.Errorf("wardrobe.grant: empty item_id")
	}
	granted, err := s.grantCosmeticTx(ctx, tx, userID, grant)
	return !granted, err
}

// GrantCosmetics grants multiple cosmetics in one transaction.
// Returns the list of newly granted items (excludes already-owned).
func (s *Store) GrantCosmetics(ctx context.Context, userID int64, grants []CosmeticGrant) ([]PlayerWardrobeItem, error) {
	for i, g := range grants {
		if !validCosmeticCategory(g.Category) {
			return nil, fmt.Errorf("wardrobe.grantBatch: index %d: %w", i, ErrCosmeticCategoryInvalid)
		}
		if g.ItemID == "" {
			return nil, fmt.Errorf("wardrobe.grantBatch: index %d: empty item_id", i)
		}
	}

	var granted []PlayerWardrobeItem
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.grantBatch: lock user: %w", err)
		}
		for _, g := range grants {
			isNew, err := s.grantCosmeticTx(ctx, tx, userID, g)
			if err != nil {
				return fmt.Errorf("wardrobe.grantBatch: %w", err)
			}
			if isNew {
				granted = append(granted, PlayerWardrobeItem{
					Category:   g.Category,
					ItemID:     g.ItemID,
					Source:     g.Source,
					ObtainedAt: time.Now(),
				})
			}
		}
		return nil
	})
	if granted == nil {
		granted = []PlayerWardrobeItem{}
	}
	return granted, err
}

// EquipCosmetic equips a cosmetic to a player slot.
// Validates ownership, catalog existence. ON CONFLICT moves item from other slot/owner.
func (s *Store) EquipCosmetic(ctx context.Context, userID int64, slotID, itemID string) error {
	if !validEquipSlot(slotID) {
		return ErrWardrobeSlotInvalid
	}
	category, _ := slotToCategory(slotID)
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.equip: lock user: %w", err)
		}
		return s.equipCosmeticTx(ctx, tx, userID, WardrobeOwnerPlayer, 0, slotID, category, itemID)
	})
}

// UnequipCosmetic clears a player equip slot. No-op if slot is empty.
func (s *Store) UnequipCosmetic(ctx context.Context, userID int64, slotID string) error {
	if !validEquipSlot(slotID) {
		return ErrWardrobeSlotInvalid
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.unequip: lock user: %w", err)
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM wardrobe_equip_slots
			WHERE user_id = $1 AND owner_type = $2 AND owner_id = 0 AND slot_id = $3
		`, userID, WardrobeOwnerPlayer, slotID)
		if err != nil {
			return fmt.Errorf("wardrobe.unequip: delete: %w", err)
		}
		return nil
	})
}

// EquipKittyCosmetic equips a cosmetic to a kitty slot.
// Validates kitty ownership, cosmetic ownership, and catalog existence.
func (s *Store) EquipKittyCosmetic(ctx context.Context, userID int64, kittyID int64, slotID, itemID string) error {
	if !validKittySlot(slotID) {
		return ErrWardrobeSlotInvalid
	}
	category, _ := slotToCategory(slotID)
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.equipKitty: lock user: %w", err)
		}
		owned, err := s.isOwnedKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return fmt.Errorf("wardrobe.equipKitty: check kitty: %w", err)
		}
		if !owned {
			return ErrKittyNotFound
		}
		return s.equipCosmeticTx(ctx, tx, userID, WardrobeOwnerKitty, kittyID, slotID, category, itemID)
	})
}

// UnequipKittyCosmetic clears a kitty equip slot. No-op if slot is empty.
func (s *Store) UnequipKittyCosmetic(ctx context.Context, userID int64, kittyID int64, slotID string) error {
	if !validKittySlot(slotID) {
		return ErrWardrobeSlotInvalid
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.unequipKitty: lock user: %w", err)
		}
		owned, err := s.isOwnedKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return fmt.Errorf("wardrobe.unequipKitty: check kitty: %w", err)
		}
		if !owned {
			return ErrKittyNotFound
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM wardrobe_equip_slots
			WHERE user_id = $1 AND owner_type = $2 AND owner_id = $3 AND slot_id = $4
		`, userID, WardrobeOwnerKitty, kittyID, slotID)
		if err != nil {
			return fmt.Errorf("wardrobe.unequipKitty: delete: %w", err)
		}
		return nil
	})
}

// equipCosmeticTx is the shared equip logic within an existing tx.
func (s *Store) equipCosmeticTx(ctx context.Context, tx pgx.Tx, userID int64, ownerType string, ownerID int64, slotID, category, itemID string) error {
	// Validate ownership of the cosmetic.
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM player_wardrobe_items
			WHERE user_id = $1 AND category = $2 AND item_id = $3
		)
	`, userID, category, itemID).Scan(&exists); err != nil {
		return fmt.Errorf("wardrobe.equip: check ownership: %w", err)
	}
	if !exists {
		return ErrCosmeticNotOwned
	}
	// Validate item exists in catalog and is not disabled.
	if !s.cosmeticExistsInCatalog(category, itemID) {
		return ErrCosmeticItemNotFound
	}
	// The table has two relevant constraints:
	//   PK:     (user_id, owner_type, owner_id, slot_id) — one item per slot
	//   UNIQUE: (user_id, item_id)                       — one slot per item
	// When equipping a DIFFERENT item to an OCCUPIED slot, the PK is violated
	// (not the UNIQUE), so ON CONFLICT alone cannot handle both. Solve by
	// clearing both conflicts first, then inserting.
	if _, err := tx.Exec(ctx, `
		DELETE FROM wardrobe_equip_slots
		WHERE user_id = $1 AND item_id = $2
	`, userID, itemID); err != nil {
		return fmt.Errorf("wardrobe.equip: clear item conflict: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM wardrobe_equip_slots
		WHERE user_id = $1 AND owner_type = $2 AND owner_id = $3 AND slot_id = $4
	`, userID, ownerType, ownerID, slotID); err != nil {
		return fmt.Errorf("wardrobe.equip: clear slot conflict: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO wardrobe_equip_slots (user_id, owner_type, owner_id, slot_id, item_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
	`, userID, ownerType, ownerID, slotID, itemID); err != nil {
		return fmt.Errorf("wardrobe.equip: insert: %w", err)
	}
	return nil
}

// cosmeticExistsInCatalog checks if a cosmetic item exists and is not disabled.
// Categories without catalog backing (avatar, avatar_frame) return true — they are
// validated by ownership alone until catalog support is added.
func (s *Store) cosmeticExistsInCatalog(category, itemID string) bool {
	switch category {
	case CosmeticCategoryTitlePrefix:
		entry := s.gameData.TitlePrefixByID(itemID)
		return entry != nil && !entry.Disabled
	case CosmeticCategoryTitleSuffix:
		entry := s.gameData.TitleSuffixByID(itemID)
		return entry != nil && !entry.Disabled
	default:
		// No catalog backing yet — trust ownership check.
		return true
	}
}

// RemoveKittyWardrobeSlots removes all equip slots for a kitty. Idempotent.
// Verifies kitty ownership before deleting.
func (s *Store) RemoveKittyWardrobeSlots(ctx context.Context, userID int64, kittyID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.removeKitty: lock user: %w", err)
		}
		owned, err := s.isOwnedKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return fmt.Errorf("wardrobe.removeKitty: check ownership: %w", err)
		}
		if !owned {
			return ErrKittyNotFound
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM wardrobe_equip_slots
			WHERE user_id = $1 AND owner_type = $2 AND owner_id = $3
		`, userID, WardrobeOwnerKitty, kittyID)
		if err != nil {
			return fmt.Errorf("wardrobe.removeKitty: delete: %w", err)
		}
		return nil
	})
}

// RevokeCosmetic removes a cosmetic from a player's ownership.
// Also clears any equip slot using that item (for the owning user).
// Idempotent — no error if the item was not owned.
func (s *Store) RevokeCosmetic(ctx context.Context, userID int64, category, itemID string) error {
	if !validCosmeticCategory(category) {
		return ErrCosmeticCategoryInvalid
	}
	if itemID == "" {
		return fmt.Errorf("wardrobe.revoke: empty item_id")
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("wardrobe.revoke: lock user: %w", err)
		}
		// Remove ownership.
		_, err := tx.Exec(ctx, `
			DELETE FROM player_wardrobe_items
			WHERE user_id = $1 AND category = $2 AND item_id = $3
		`, userID, category, itemID)
		if err != nil {
			return fmt.Errorf("wardrobe.revoke: delete owned: %w", err)
		}
		// Clear any equip slot using this item (for this user).
		_, err = tx.Exec(ctx, `
			DELETE FROM wardrobe_equip_slots
			WHERE user_id = $1 AND item_id = $2
		`, userID, itemID)
		if err != nil {
			return fmt.Errorf("wardrobe.revoke: delete equip: %w", err)
		}
		return nil
	})
}

// GetPlayerTitle returns the equipped title prefix and suffix item IDs for a player.
// Returns empty strings if no title is equipped. Read-only, no lock needed.
func (s *Store) GetPlayerTitle(ctx context.Context, userID int64) (prefixItemID, suffixItemID string, err error) {
	rows, err := s.db.Query(ctx, `
		SELECT slot_id, item_id
		FROM wardrobe_equip_slots
		WHERE user_id = $1 AND owner_type = $2 AND owner_id = 0
		  AND slot_id IN ($3, $4)
	`, userID, WardrobeOwnerPlayer, EquipSlotTitlePrefix, EquipSlotTitleSuffix)
	if err != nil {
		return "", "", fmt.Errorf("wardrobe.getPlayerTitle: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slotID, itemID string
		if err := rows.Scan(&slotID, &itemID); err != nil {
			return "", "", fmt.Errorf("wardrobe.getPlayerTitle: scan: %w", err)
		}
		switch slotID {
		case EquipSlotTitlePrefix:
			prefixItemID = itemID
		case EquipSlotTitleSuffix:
			suffixItemID = itemID
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", fmt.Errorf("wardrobe.getPlayerTitle: iter: %w", err)
	}
	return prefixItemID, suffixItemID, nil
}

// GetKittyTitle returns the equipped title prefix and suffix item IDs for a kitty.
func (s *Store) GetKittyTitle(ctx context.Context, userID int64, kittyID int64) (prefixItemID, suffixItemID string, err error) {
	rows, err := s.db.Query(ctx, `
		SELECT slot_id, item_id
		FROM wardrobe_equip_slots
		WHERE user_id = $1 AND owner_type = $2 AND owner_id = $3
		  AND slot_id IN ($4, $5)
	`, userID, WardrobeOwnerKitty, kittyID, EquipSlotKittyTitlePrefix, EquipSlotKittyTitleSuffix)
	if err != nil {
		return "", "", fmt.Errorf("wardrobe.getKittyTitle: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slotID, itemID string
		if err := rows.Scan(&slotID, &itemID); err != nil {
			return "", "", fmt.Errorf("wardrobe.getKittyTitle: scan: %w", err)
		}
		switch slotID {
		case EquipSlotKittyTitlePrefix:
			prefixItemID = itemID
		case EquipSlotKittyTitleSuffix:
			suffixItemID = itemID
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", fmt.Errorf("wardrobe.getKittyTitle: iter: %w", err)
	}
	return prefixItemID, suffixItemID, nil
}
