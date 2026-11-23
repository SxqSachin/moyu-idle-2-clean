package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type InventoryLedgerMeta struct {
	EventType     WarehouseChangeEvent
	Category      InventoryAuditCategory
	Direction     InventoryAuditDirection
	SubjectType   string
	SubjectID     string
	InstanceID    *int64
	SourceSurface string
	TargetSurface string
	Details       map[string]any
}

func (s *Store) ledgerLogTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, meta InventoryLedgerMeta) error {
	if qty <= 0 {
		return ErrBadQuantity
	}
	if userID <= 0 {
		return ErrBadQuantity
	}
	eventType := meta.EventType
	if eventType == "" {
		if meta.TargetSurface == "inventory" {
			eventType = WarehouseChangeEventSystemToWarehouse
		} else {
			eventType = WarehouseChangeEventWarehouseOp
		}
	}
	s.recordInventoryAuditTx(tx, InventoryAuditEvent{
		UserID:        userID,
		Category:      meta.Category,
		Direction:     meta.Direction,
		SubjectType:   meta.SubjectType,
		SubjectID:     meta.SubjectID,
		EventType:     eventType,
		InstanceID:    meta.InstanceID,
		TemplateItem:  itemID,
		Quantity:      qty,
		SourceSurface: meta.SourceSurface,
		TargetSurface: meta.TargetSurface,
		Details:       meta.Details,
	})
	if eventType == WarehouseChangeEventSystemToWarehouse && meta.TargetSurface == "inventory" {
		enhanceLevel := 0
		if meta.Details != nil {
			enhanceLevel = int(int64FromAchievementAny(meta.Details["enhanceLevel"], 0))
		}
		if err := s.ApplyAchievementEventsTx(ctx, tx, userID, []AchievementEvent{{
			Kind:         AchievementEventObtainItem,
			ItemID:       itemID,
			SubjectID:    itemID,
			Count:        qty,
			EnhanceLevel: enhanceLevel,
		}, {
			Kind:      AchievementEventStateReconcile,
			StateKind: "own_item",
			ItemID:    itemID,
			SubjectID: itemID,
		}}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LedgerConsumeTemplateTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, meta InventoryLedgerMeta) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.ConsumeByTemplateTx(ctx, tx, userID, itemID, qty); err != nil {
		return err
	}
	if meta.EventType == "" {
		meta.EventType = WarehouseChangeEventWarehouseOp
	}
	return s.ledgerLogTx(ctx, tx, userID, itemID, qty, meta)
}

func (s *Store) LedgerGrantTemplateTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, meta InventoryLedgerMeta) error {
	return s.LedgerGrantTemplateWithOptionsTx(ctx, tx, userID, itemID, qty, meta, InventoryGrantOptions{})
}

func (s *Store) LedgerGrantTemplateWithOptionsTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, meta InventoryLedgerMeta, options InventoryGrantOptions) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.GrantByTemplateWithOptionsTx(ctx, tx, userID, itemID, qty, options); err != nil {
		return err
	}
	if meta.EventType == "" {
		meta.EventType = WarehouseChangeEventSystemToWarehouse
	}
	return s.ledgerLogTx(ctx, tx, userID, itemID, qty, meta)
}

func (s *Store) LedgerConsumeInstanceTx(ctx context.Context, tx pgx.Tx, userID int64, instanceID int64, itemID string, qty int64, meta InventoryLedgerMeta) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	detachedPlayerUsers, detachedKittyIDs, err := s.detachInstanceEquipSlotsTx(ctx, tx, userID, instanceID)
	if err != nil {
		return err
	}
	if err := s.DecrementOrDeleteTx(ctx, tx, instanceID, userID, qty); err != nil {
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
	meta.InstanceID = &instanceID
	if meta.EventType == "" {
		meta.EventType = WarehouseChangeEventWarehouseOp
	}
	return s.ledgerLogTx(ctx, tx, userID, itemID, qty, meta)
}

func (s *Store) LedgerSetStateTx(ctx context.Context, tx pgx.Tx, instanceID, userID int64, fromState string, toState string, itemID string, qty int64, meta InventoryLedgerMeta) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	var detachedPlayerUsers []int64
	var detachedKittyIDs []int64
	if toState != "equipped" {
		var err error
		detachedPlayerUsers, detachedKittyIDs, err = s.detachInstanceEquipSlotsTx(ctx, tx, userID, instanceID)
		if err != nil {
			return err
		}
	}
	if err := s.SetStateTx(ctx, tx, instanceID, userID, fromState, toState); err != nil {
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
	meta.InstanceID = &instanceID
	return s.ledgerLogTx(ctx, tx, userID, itemID, qty, meta)
}

func (s *Store) LedgerTransferOwnershipAndStateTx(ctx context.Context, tx pgx.Tx, instanceID, fromUserID, toUserID int64, fromState string, toState string, itemID string, qty int64, meta InventoryLedgerMeta) error {
	firstUserID := fromUserID
	secondUserID := toUserID
	if secondUserID > 0 && firstUserID > secondUserID {
		firstUserID, secondUserID = secondUserID, firstUserID
	}
	if err := s.lockUserTx(ctx, tx, firstUserID); err != nil {
		return err
	}
	if secondUserID > 0 && secondUserID != firstUserID {
		if err := s.lockUserTx(ctx, tx, secondUserID); err != nil {
			return err
		}
	}
	detachedPlayerUsers, detachedKittyIDs, err := s.detachInstanceEquipSlotsTx(ctx, tx, fromUserID, instanceID)
	if err != nil {
		return err
	}
	if err := s.TransferOwnershipAndStateTx(ctx, tx, instanceID, fromUserID, toUserID, fromState, toState); err != nil {
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
			if err := s.recomputeKittyDerivedStatsTx(ctx, tx, fromUserID, kittyID, seeds); err != nil {
				return err
			}
		}
	}
	meta.InstanceID = &instanceID
	return s.ledgerLogTx(ctx, tx, toUserID, itemID, qty, meta)
}

func (s *Store) LedgerLogOnlyTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string, qty int64, meta InventoryLedgerMeta) error {
	return s.ledgerLogTx(ctx, tx, userID, itemID, qty, meta)
}
