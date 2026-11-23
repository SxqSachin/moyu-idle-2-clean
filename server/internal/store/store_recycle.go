package store

import (
	"context"
	"math"
	"sort"

	"github.com/jackc/pgx/v5"
)

type RecycleItemResult struct {
	RecycledQuantity int64
	UnitPrice        int64
	TotalGold        int64
	InstanceRemoved  bool
}

func computeRecycleGold(economyBaseValue float64, coefficient float64, quantity int64) (int64, int64, error) {
	if quantity <= 0 {
		return 0, 0, ErrBadQuantity
	}
	if !numberFinite(economyBaseValue) || economyBaseValue <= 0 {
		return 0, 0, ErrRecyclePriceInvalid
	}
	if !numberFinite(coefficient) || coefficient <= 0 {
		return 0, 0, ErrRecyclePriceInvalid
	}
	unitPriceFloat := math.Floor(economyBaseValue * coefficient)
	if !numberFinite(unitPriceFloat) || unitPriceFloat <= 0 || unitPriceFloat > math.MaxInt64 {
		return 0, 0, ErrRecyclePriceInvalid
	}
	totalGoldFloat := math.Floor(unitPriceFloat * float64(quantity))
	if !numberFinite(totalGoldFloat) || totalGoldFloat <= 0 || totalGoldFloat > math.MaxInt64 {
		return 0, 0, ErrRecyclePriceInvalid
	}
	return int64(unitPriceFloat), int64(totalGoldFloat), nil
}

func numberFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func resolveRecycleGoldCoefficient(baseCoefficient float64, affixMismatchExtraCoefficient float64, matchKind recycleTemplateMatchKind) (float64, error) {
	if !numberFinite(baseCoefficient) || baseCoefficient <= 0 {
		return 0, ErrRecyclePriceInvalid
	}
	if !numberFinite(affixMismatchExtraCoefficient) || affixMismatchExtraCoefficient <= 0 {
		return 0, ErrRecyclePriceInvalid
	}
	switch matchKind {
	case recycleTemplateMatchFull:
		return baseCoefficient, nil
	case recycleTemplateMatchAffixMismatchOnly:
		coefficient := baseCoefficient + affixMismatchExtraCoefficient - 1
		if !numberFinite(coefficient) || coefficient <= 0 {
			return 0, ErrRecyclePriceInvalid
		}
		return coefficient, nil
	default:
		return 0, ErrItemNotRecyclable
	}
}

type RecycleBatchItemInput struct {
	InstanceID int64
	Quantity   int64
}

type RecycleBatchItemResult struct {
	InstanceID       int64
	RecycledQuantity int64
	UnitPrice        int64
	TotalGold        int64
	InstanceRemoved  bool
}

type RecycleBatchResult struct {
	Results   []RecycleBatchItemResult
	TotalGold int64
}

type preparedRecycleBatchItem struct {
	Input       RecycleBatchItemInput
	Row         LockedItemInstance
	MatchKind   recycleTemplateMatchKind
	Coefficient float64
	UnitPrice   int64
	TotalGold   int64
}

const RecycleBatchItemLimit = 1000

func normalizeRecycleBatchItems(items []RecycleBatchItemInput) ([]RecycleBatchItemInput, error) {
	if len(items) == 0 || len(items) > RecycleBatchItemLimit {
		return nil, ErrBadQuantity
	}
	seen := make(map[int64]struct{}, len(items))
	deduped := make([]RecycleBatchItemInput, 0, len(items))
	for _, item := range items {
		if item.InstanceID <= 0 || item.Quantity <= 0 {
			return nil, ErrBadQuantity
		}
		if _, ok := seen[item.InstanceID]; ok {
			continue
		}
		seen[item.InstanceID] = struct{}{}
		deduped = append(deduped, item)
	}
	if len(deduped) == 0 {
		return nil, ErrBadQuantity
	}
	// sort by InstanceID to acquire locks in consistent order and prevent deadlocks
	sort.Slice(deduped, func(i, j int) bool { return deduped[i].InstanceID < deduped[j].InstanceID })
	return deduped, nil
}

func (s *Store) RecycleItemInstanceBatch(ctx context.Context, userID int64, items []RecycleBatchItemInput) (RecycleBatchResult, error) {
	deduped, err := normalizeRecycleBatchItems(items)
	if err != nil {
		return RecycleBatchResult{}, ErrBadQuantity
	}

	var out RecycleBatchResult
	instanceIDs := recycleBatchInstanceIDs(deduped)

	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}

		locked, err := s.lockRecycleBatchInstancesTx(ctx, tx, instanceIDs)
		if err != nil {
			return err
		}
		prepared, preparedOut, err := s.prepareRecycleBatchItems(userID, deduped, locked)
		if err != nil {
			return err
		}
		out = preparedOut

		detachedPlayerUsers, detachedKittyIDs, err := s.detachRecycleBatchEquipSlotsTx(ctx, tx, userID, instanceIDs)
		if err != nil {
			return err
		}
		if err = s.applyRecycleBatchConsumptionTx(ctx, tx, userID, prepared); err != nil {
			return err
		}
		for _, affectedUserID := range detachedPlayerUsers {
			if err := s.onPlayerAttributesChangedTx(ctx, tx, affectedUserID); err != nil {
				return err
			}
		}
		if len(detachedKittyIDs) > 0 {
			seeds := s.attributeSeedsFromGameData()
			for _, kittyID := range detachedKittyIDs {
				if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, seeds); err != nil {
					return err
				}
			}
		}
		if err = s.logRecycleBatchConsumptionTx(ctx, tx, userID, prepared); err != nil {
			return err
		}
		if err = s.LedgerGrantTemplateTx(ctx, tx, userID, GoldCoinItemID, out.TotalGold, InventoryLedgerMeta{
			SourceSurface: "recycle",
			TargetSurface: "inventory",
			Details: map[string]any{
				"batchRecycle": true,
				"itemCount":    len(out.Results),
			},
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return RecycleBatchResult{}, err
	}
	return out, nil
}

func recycleBatchInstanceIDs(items []RecycleBatchItemInput) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.InstanceID)
	}
	return ids
}

func (s *Store) lockRecycleBatchInstancesTx(ctx context.Context, tx pgx.Tx, instanceIDs []int64) (map[int64]LockedItemInstance, error) {
	rows, err := tx.Query(ctx, `
		SELECT instance_id, user_id, template_item_id, COALESCE(custom_data, '{}'::jsonb), quantity, enhance_level, roll_seed,
		       base_snapshot, affixes, milestone_affix_claims, durability, max_durability, state, archived_at
		FROM player_item_instances
		WHERE instance_id = ANY($1)
		ORDER BY instance_id
		FOR UPDATE
	`, instanceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locked := make(map[int64]LockedItemInstance, len(instanceIDs))
	for rows.Next() {
		var row LockedItemInstance
		var customDataRaw []byte
		if err := rows.Scan(
			&row.InstanceID,
			&row.UserID,
			&row.TemplateItem,
			&customDataRaw,
			&row.Quantity,
			&row.EnhanceLevel,
			&row.RollSeed,
			&row.BaseSnapshot,
			&row.AffixesRaw,
			&row.MilestoneAffixesRaw,
			&row.Durability,
			&row.MaxDurability,
			&row.State,
			&row.ArchivedAt,
		); err != nil {
			return nil, err
		}
		row.CustomData = decodeItemCustomization(customDataRaw)
		locked[row.InstanceID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return locked, nil
}

func (s *Store) prepareRecycleBatchItems(userID int64, items []RecycleBatchItemInput, locked map[int64]LockedItemInstance) ([]preparedRecycleBatchItem, RecycleBatchResult, error) {
	prepared := make([]preparedRecycleBatchItem, 0, len(items))
	out := RecycleBatchResult{Results: make([]RecycleBatchItemResult, 0, len(items))}
	for _, inp := range items {
		row, ok := locked[inp.InstanceID]
		if !ok {
			return nil, RecycleBatchResult{}, pgx.ErrNoRows
		}
		if row.UserID != userID {
			return nil, RecycleBatchResult{}, ErrInstanceNotOwned
		}
		if row.State != "inventory" {
			return nil, RecycleBatchResult{}, ErrInvalidInstanceState
		}
		if inp.Quantity > row.Quantity {
			return nil, RecycleBatchResult{}, ErrInsufficientItem
		}
		matchKind := s.lockedInstanceRecycleTemplateMatchKind(row)
		if matchKind == recycleTemplateMatchNone {
			return nil, RecycleBatchResult{}, ErrItemNotRecyclable
		}
		item := s.gameData.ItemByID(row.TemplateItem)
		if item == nil || item.EconomyBaseValue <= 0 {
			return nil, RecycleBatchResult{}, ErrItemNotRecyclable
		}
		coefficient, err := resolveRecycleGoldCoefficient(
			s.config.ItemRecycleGoldCoefficient,
			s.config.ItemRecycleAffixMismatchExtraCoefficient,
			matchKind,
		)
		if err != nil {
			return nil, RecycleBatchResult{}, err
		}
		unitPrice, totalGold, err := computeRecycleGold(item.EconomyBaseValue, coefficient, inp.Quantity)
		if err != nil {
			return nil, RecycleBatchResult{}, err
		}
		if out.TotalGold+totalGold < out.TotalGold {
			return nil, RecycleBatchResult{}, ErrRecyclePriceInvalid
		}
		prepared = append(prepared, preparedRecycleBatchItem{
			Input:       inp,
			Row:         row,
			MatchKind:   matchKind,
			Coefficient: coefficient,
			UnitPrice:   unitPrice,
			TotalGold:   totalGold,
		})
		out.Results = append(out.Results, RecycleBatchItemResult{
			InstanceID:       inp.InstanceID,
			RecycledQuantity: inp.Quantity,
			UnitPrice:        unitPrice,
			TotalGold:        totalGold,
			InstanceRemoved:  inp.Quantity == row.Quantity,
		})
		out.TotalGold += totalGold
	}
	return prepared, out, nil
}

func (s *Store) detachRecycleBatchEquipSlotsTx(ctx context.Context, tx pgx.Tx, ownerID int64, instanceIDs []int64) ([]int64, []int64, error) {
	playerRows, err := tx.Query(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND instance_id = ANY($2)
		RETURNING user_id
	`, ownerID, instanceIDs)
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
		  AND k.instance_id = ANY($2)
		RETURNING p.id
	`, ownerID, instanceIDs)
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

func (s *Store) applyRecycleBatchConsumptionTx(ctx context.Context, tx pgx.Tx, userID int64, prepared []preparedRecycleBatchItem) error {
	deleteIDs := make([]int64, 0, len(prepared))
	updateIDs := make([]int64, 0, len(prepared))
	updateDeltas := make([]int64, 0, len(prepared))

	for _, item := range prepared {
		if item.Input.Quantity == item.Row.Quantity {
			deleteIDs = append(deleteIDs, item.Row.InstanceID)
			continue
		}
		updateIDs = append(updateIDs, item.Row.InstanceID)
		updateDeltas = append(updateDeltas, item.Input.Quantity)
	}

	if len(deleteIDs) > 0 {
		tag, err := tx.Exec(ctx, `
			DELETE FROM player_item_instances
			WHERE user_id = $1 AND instance_id = ANY($2)
		`, userID, deleteIDs)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != int64(len(deleteIDs)) {
			return ErrInvalidInstanceState
		}
	}
	if len(updateIDs) > 0 {
		tag, err := tx.Exec(ctx, `
			UPDATE player_item_instances AS inst
			SET quantity = inst.quantity - input.delta,
			    updated_at = now()
			FROM unnest($1::bigint[], $2::bigint[]) AS input(instance_id, delta)
			WHERE inst.instance_id = input.instance_id
			  AND inst.user_id = $3
			  AND inst.quantity >= input.delta
		`, updateIDs, updateDeltas, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != int64(len(updateIDs)) {
			return ErrInsufficientItem
		}
	}
	return nil
}

func (s *Store) logRecycleBatchConsumptionTx(ctx context.Context, tx pgx.Tx, userID int64, prepared []preparedRecycleBatchItem) error {
	for _, item := range prepared {
		instanceID := item.Row.InstanceID
		if err := s.ledgerLogTx(ctx, tx, userID, item.Row.TemplateItem, item.Input.Quantity, InventoryLedgerMeta{
			EventType:     WarehouseChangeEventWarehouseOp,
			InstanceID:    &instanceID,
			SourceSurface: "inventory",
			TargetSurface: "recycle",
			Details: map[string]any{
				"unitPrice":    item.UnitPrice,
				"gold":         item.TotalGold,
				"coefficient":  item.Coefficient,
				"recycleMatch": int(item.MatchKind),
				"batchRecycle": true,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecycleItemInstance(ctx context.Context, userID int64, instanceID int64, quantity int64) (RecycleItemResult, error) {
	if instanceID <= 0 || quantity <= 0 {
		return RecycleItemResult{}, ErrBadQuantity
	}
	var out RecycleItemResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		row, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if row.UserID != userID {
			return ErrInstanceNotOwned
		}
		if row.State != "inventory" {
			return ErrInvalidInstanceState
		}
		if quantity > row.Quantity {
			return ErrInsufficientItem
		}
		matchKind := s.lockedInstanceRecycleTemplateMatchKind(row)
		if matchKind == recycleTemplateMatchNone {
			return ErrItemNotRecyclable
		}
		item := s.gameData.ItemByID(row.TemplateItem)
		if item == nil || item.EconomyBaseValue <= 0 {
			return ErrItemNotRecyclable
		}
		coefficient, err := resolveRecycleGoldCoefficient(
			s.config.ItemRecycleGoldCoefficient,
			s.config.ItemRecycleAffixMismatchExtraCoefficient,
			matchKind,
		)
		if err != nil {
			return err
		}
		unitPrice, totalGold, err := computeRecycleGold(item.EconomyBaseValue, coefficient, quantity)
		if err != nil {
			return err
		}
		if err = s.LedgerConsumeInstanceTx(ctx, tx, userID, instanceID, row.TemplateItem, quantity, InventoryLedgerMeta{
			SourceSurface: "inventory",
			TargetSurface: "recycle",
			Details: map[string]any{
				"unitPrice":    unitPrice,
				"gold":         totalGold,
				"coefficient":  coefficient,
				"recycleMatch": int(matchKind),
			},
		}); err != nil {
			return err
		}
		if err = s.LedgerGrantTemplateTx(ctx, tx, userID, GoldCoinItemID, totalGold, InventoryLedgerMeta{
			SourceSurface: "recycle",
			TargetSurface: "inventory",
			Details: map[string]any{
				"instanceId": instanceID,
				"itemId":     row.TemplateItem,
			},
		}); err != nil {
			return err
		}
		out = RecycleItemResult{
			RecycledQuantity: quantity,
			UnitPrice:        unitPrice,
			TotalGold:        totalGold,
			InstanceRemoved:  quantity == row.Quantity,
		}
		return nil
	})
	if err != nil {
		return RecycleItemResult{}, err
	}
	return out, nil
}
