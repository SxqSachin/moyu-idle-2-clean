package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type LockedItemInstance struct {
	InstanceID          int64
	UserID              int64
	TemplateItem        string
	CustomData          ItemCustomization
	Quantity            int64
	EnhanceLevel        int
	RollSeed            int64
	BaseSnapshot        []byte
	AffixesRaw          []byte
	MilestoneAffixesRaw []byte
	Durability          int
	MaxDurability       int
	State               string
	ArchivedAt          *time.Time
}

func (s *Store) LockInstanceTx(ctx context.Context, tx pgx.Tx, instanceID int64) (LockedItemInstance, error) {
	var out LockedItemInstance
	var customDataRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT instance_id, user_id, template_item_id, COALESCE(custom_data, '{}'::jsonb), quantity, enhance_level, roll_seed,
		       base_snapshot, affixes, milestone_affix_claims, durability, max_durability, state, archived_at
		FROM player_item_instances
		WHERE instance_id = $1
		FOR UPDATE
	`, instanceID).Scan(
		&out.InstanceID,
		&out.UserID,
		&out.TemplateItem,
		&customDataRaw,
		&out.Quantity,
		&out.EnhanceLevel,
		&out.RollSeed,
		&out.BaseSnapshot,
		&out.AffixesRaw,
		&out.MilestoneAffixesRaw,
		&out.Durability,
		&out.MaxDurability,
		&out.State,
		&out.ArchivedAt,
	)
	if err != nil {
		return LockedItemInstance{}, err
	}
	out.CustomData = decodeItemCustomization(customDataRaw)
	return out, nil
}

func (s *Store) TransferOwnershipAndStateTx(
	ctx context.Context,
	tx pgx.Tx,
	instanceID int64,
	fromUserID int64,
	toUserID int64,
	fromState string,
	toState string,
) error {
	tag, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET user_id = $1,
		    state = $2,
		    archived_at = CASE WHEN $1 <> $4 THEN NULL ELSE archived_at END,
		    updated_at = now()
		WHERE instance_id = $3 AND user_id = $4 AND state = $5
	`, toUserID, toState, instanceID, fromUserID, fromState)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrInvalidInstanceState
	}
	return nil
}

func (s *Store) SetStateTx(ctx context.Context, tx pgx.Tx, instanceID int64, ownerID int64, fromState string, toState string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET state = $1, updated_at = now()
		WHERE instance_id = $2 AND user_id = $3 AND state = $4
	`, toState, instanceID, ownerID, fromState)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrInvalidInstanceState
	}
	return nil
}

func (s *Store) SetAffixesTx(ctx context.Context, tx pgx.Tx, instanceID int64, affixes []EquipmentAffixSlot) error {
	return s.SetAffixesRawTx(ctx, tx, instanceID, encodeAffixSlots(affixes))
}

func (s *Store) SetAffixesRawTx(ctx context.Context, tx pgx.Tx, instanceID int64, affixRaw []byte) error {
	_, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET affixes = $1::jsonb, updated_at = now()
		WHERE instance_id = $2
	`, string(affixRaw), instanceID)
	return err
}

func (s *Store) SetMilestoneAffixClaimsRawTx(ctx context.Context, tx pgx.Tx, instanceID int64, raw []byte) error {
	_, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET milestone_affix_claims = $1::jsonb, updated_at = now()
		WHERE instance_id = $2
	`, string(raw), instanceID)
	return err
}

func (s *Store) DecrementOrDeleteTx(ctx context.Context, tx pgx.Tx, instanceID int64, ownerID int64, delta int64) error {
	if delta <= 0 {
		return nil
	}
	var quantity int64
	if err := tx.QueryRow(ctx, `
		SELECT quantity
		FROM player_item_instances
		WHERE instance_id = $1 AND user_id = $2
		FOR UPDATE
	`, instanceID, ownerID).Scan(&quantity); err != nil {
		return err
	}
	if quantity < delta {
		return ErrInsufficientItem
	}
	if quantity == delta {
		_, err := tx.Exec(ctx, `
			DELETE FROM player_item_instances
			WHERE instance_id = $1 AND user_id = $2
		`, instanceID, ownerID)
		if err != nil {
			return err
		}
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET quantity = quantity - $1, updated_at = now()
		WHERE instance_id = $2 AND user_id = $3
	`, delta, instanceID, ownerID)
	return err
}

func (s *Store) detachInstanceEquipSlotsTx(ctx context.Context, tx pgx.Tx, ownerID int64, instanceID int64) ([]int64, []int64, error) {
	if err := s.ensureEquipmentInstancesNotInActiveFishingTripTx(ctx, tx, ownerID, instanceID); err != nil {
		return nil, nil, err
	}
	playerRows, err := tx.Query(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND instance_id = $2
		RETURNING user_id
	`, ownerID, instanceID)
	if err != nil {
		return nil, nil, err
	}
	playerSeen := map[int64]struct{}{}
	playerUsers := make([]int64, 0, 1)
	for playerRows.Next() {
		var userID int64
		if err := playerRows.Scan(&userID); err != nil {
			playerRows.Close()
			return nil, nil, err
		}
		if _, ok := playerSeen[userID]; ok {
			continue
		}
		playerSeen[userID] = struct{}{}
		playerUsers = append(playerUsers, userID)
	}
	if err := playerRows.Err(); err != nil {
		playerRows.Close()
		return nil, nil, err
	}
	playerRows.Close()

	kittyRows, err := tx.Query(ctx, `
		DELETE FROM player_kitty_equip_slots k
		USING player_kitties p
		WHERE k.kitty_id = p.id
		  AND p.user_id = $1
		  AND k.instance_id = $2
		RETURNING p.id
	`, ownerID, instanceID)
	if err != nil {
		return nil, nil, err
	}
	kittySeen := map[int64]struct{}{}
	kittyIDs := make([]int64, 0, 1)
	for kittyRows.Next() {
		var kittyID int64
		if err := kittyRows.Scan(&kittyID); err != nil {
			kittyRows.Close()
			return nil, nil, err
		}
		if _, ok := kittySeen[kittyID]; ok {
			continue
		}
		kittySeen[kittyID] = struct{}{}
		kittyIDs = append(kittyIDs, kittyID)
	}
	if err := kittyRows.Err(); err != nil {
		kittyRows.Close()
		return nil, nil, err
	}
	kittyRows.Close()
	return playerUsers, kittyIDs, nil
}

// SplitOneFromStackTx splits one instance from a stack row. If the source row is already quantity=1,
// it returns the original instance id.
func (s *Store) SplitOneFromStackTx(ctx context.Context, tx pgx.Tx, instanceID int64, ownerID int64) (int64, error) {
	row, err := s.LockInstanceTx(ctx, tx, instanceID)
	if err != nil {
		return 0, err
	}
	if row.UserID != ownerID {
		return 0, ErrInstanceNotOwned
	}
	if row.Quantity <= 1 {
		return row.InstanceID, nil
	}
	var newID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO player_item_instances(
			user_id, template_item_id, custom_data, enhance_level, roll_seed, base_snapshot, affixes,
			milestone_affix_claims, durability, max_durability, quantity, state, created_at, updated_at
		)
		VALUES($1, $2, $3::jsonb, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9, $10, 1, $11, now(), now())
		RETURNING instance_id
	`, row.UserID, row.TemplateItem, mustEncodeItemCustomizationString(row.CustomData), row.EnhanceLevel, row.RollSeed, string(row.BaseSnapshot), string(row.AffixesRaw), string(row.MilestoneAffixesRaw), row.Durability, row.MaxDurability, row.State).Scan(&newID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET quantity = quantity - 1, updated_at = now()
		WHERE instance_id = $1 AND user_id = $2 AND quantity > 1
	`, instanceID, ownerID); err != nil {
		return 0, err
	}
	return newID, nil
}
