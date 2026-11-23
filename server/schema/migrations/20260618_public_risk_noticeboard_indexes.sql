CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_time
  ON risk_account_transfers(occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_item_time
  ON risk_account_transfers(item_id, occurred_at DESC)
  WHERE btrim(item_id) <> '';

CREATE INDEX IF NOT EXISTS idx_risk_account_evidence_type_time
  ON risk_account_evidence(evidence_type, last_seen_at DESC, score DESC);

CREATE INDEX IF NOT EXISTS idx_risk_account_links_pair_score_time
  ON risk_account_links(user_id, peer_user_id, total_score DESC, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_users_ban_active
  ON users(created_at DESC)
  WHERE btrim(ban_reason) <> '';
