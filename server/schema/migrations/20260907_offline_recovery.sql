-- Apply before deploying the server, action-worker, battle-worker and web together.
CREATE TABLE IF NOT EXISTS player_offline_recoveries (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  started_at TIMESTAMPTZ NOT NULL,
  target_at TIMESTAMPTZ NOT NULL,
  resumed_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running','complete','stalled')),
  revision BIGINT NOT NULL DEFAULT 0,
  summary JSONB NOT NULL DEFAULT '{}',
  pending_run_ids BIGINT[] NOT NULL DEFAULT '{}',
  attempts INT NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  acknowledged_at TIMESTAMPTZ,
  schedule_pending BOOLEAN NOT NULL DEFAULT false,
  CHECK (target_at >= started_at AND resumed_at >= target_at)
);
CREATE INDEX IF NOT EXISTS idx_offline_recovery_user ON player_offline_recoveries(user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_offline_recovery_ready ON player_offline_recoveries(available_at, id) WHERE status='running';
CREATE INDEX IF NOT EXISTS idx_offline_recovery_unfinished ON player_offline_recoveries(user_id, id) WHERE status IN ('running','stalled');
CREATE INDEX IF NOT EXISTS idx_offline_recovery_unread ON player_offline_recoveries(user_id, id) WHERE acknowledged_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_offline_recovery_schedule ON player_offline_recoveries(id) WHERE schedule_pending;

-- No run/room FK: room cleanup must never remove reward evidence.
CREATE TABLE IF NOT EXISTS battle_reward_receipts (
  run_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_rewards JSONB NOT NULL,
  settled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id,user_id)
);
CREATE INDEX IF NOT EXISTS idx_battle_reward_receipt_user_time ON battle_reward_receipts(user_id,settled_at);
