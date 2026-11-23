package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
)

type RiskTransferRecord struct {
	FromUserID  int64
	ToUserID    int64
	Channel     string
	ReferenceID int64
	ItemID      string
	Quantity    int64
	UnitPrice   int64
	TotalPrice  int64
	FeeAmount   int64
	NetAmount   int64
	PriceAnchor int64
	Extra       map[string]any
}

func (s *Store) recordRiskTransferTx(ctx context.Context, tx pgx.Tx, input RiskTransferRecord) error {
	if tx == nil || input.FromUserID <= 0 || input.ToUserID <= 0 || input.FromUserID == input.ToUserID {
		return nil
	}
	channel := strings.TrimSpace(strings.ToLower(input.Channel))
	if channel == "" {
		channel = "unknown"
	}
	itemID := strings.TrimSpace(input.ItemID)
	if input.Quantity < 0 {
		input.Quantity = 0
	}
	if input.UnitPrice < 0 {
		input.UnitPrice = 0
	}
	if input.TotalPrice < 0 {
		input.TotalPrice = 0
	}
	if input.FeeAmount < 0 {
		input.FeeAmount = 0
	}
	if input.NetAmount < 0 {
		input.NetAmount = 0
	}
	if input.PriceAnchor < 0 {
		input.PriceAnchor = 0
	}
	if input.Extra == nil {
		input.Extra = map[string]any{}
	}
	extraJSON, err := json.Marshal(input.Extra)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO risk_account_transfers(
			from_user_id, to_user_id, channel, reference_id, item_id, quantity,
			unit_price, total_price, fee_amount, net_amount, price_anchor, occurred_at, extra_jsonb
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, now(), $12::jsonb
		)
	`, input.FromUserID, input.ToUserID, channel, input.ReferenceID, itemID, input.Quantity,
		input.UnitPrice, input.TotalPrice, input.FeeAmount, input.NetAmount, input.PriceAnchor, string(extraJSON))
	return err
}
