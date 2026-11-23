package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	AdminInventoryGrantModeTemplate = "template"
	AdminInventoryGrantModeEnhanced = "enhanced"
)

type AdminInventoryGrantInput struct {
	UserID       int64
	ItemID       string
	Quantity     int64
	GrantMode    string
	EnhanceLevel int
}

func normalizeAdminInventoryGrantMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", AdminInventoryGrantModeTemplate:
		return AdminInventoryGrantModeTemplate
	case AdminInventoryGrantModeEnhanced:
		return AdminInventoryGrantModeEnhanced
	default:
		return ""
	}
}

func (s *Store) AdminGrantInventory(ctx context.Context, input AdminInventoryGrantInput) error {
	itemID := strings.TrimSpace(input.ItemID)
	if input.UserID <= 0 || itemID == "" {
		return ErrBadQuantity
	}
	if input.Quantity <= 0 {
		return ErrBadQuantity
	}
	if s.gameData.ItemByID(itemID) == nil {
		return ErrInvalidItemID
	}
	grantMode := normalizeAdminInventoryGrantMode(input.GrantMode)
	if grantMode == "" {
		return ErrAdminInventoryGrantModeInvalid
	}
	if grantMode == AdminInventoryGrantModeEnhanced && input.EnhanceLevel <= 0 {
		return ErrAdminInventoryGrantEnhanceLevelInvalid
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		details := map[string]any{
			"grantMode": grantMode,
		}
		switch grantMode {
		case AdminInventoryGrantModeTemplate:
			return s.LedgerGrantTemplateTx(ctx, tx, input.UserID, itemID, input.Quantity, InventoryLedgerMeta{
				SourceSurface: "admin_inventory_grant",
				TargetSurface: "inventory",
				Details:       details,
			})
		case AdminInventoryGrantModeEnhanced:
			details["enhanceLevel"] = input.EnhanceLevel
			if err := s.lockUserTx(ctx, tx, input.UserID); err != nil {
				return err
			}
			for i := int64(0); i < input.Quantity; i++ {
				instanceID, err := s.AddInventoryEnhancedInstanceTx(ctx, tx, input.UserID, itemID, input.EnhanceLevel)
				if err != nil {
					return err
				}
				instanceIDCopy := instanceID
				if err := s.LedgerLogOnlyTx(ctx, tx, input.UserID, itemID, 1, InventoryLedgerMeta{
					EventType:     WarehouseChangeEventSystemToWarehouse,
					InstanceID:    &instanceIDCopy,
					SourceSurface: "admin_inventory_grant",
					TargetSurface: "inventory",
					Details:       details,
				}); err != nil {
					return err
				}
			}
			return nil
		default:
			return ErrAdminInventoryGrantModeInvalid
		}
	})
}
