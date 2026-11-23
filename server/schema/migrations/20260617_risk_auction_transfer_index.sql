-- Partial index to support admin auction-transfers query efficiently.
CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_auction_time
  ON risk_account_transfers(occurred_at DESC)
  WHERE channel LIKE 'auction_%';
