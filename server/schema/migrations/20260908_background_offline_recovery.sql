-- Apply after 20260907_offline_recovery.sql, before starting the new workers.
ALTER TABLE player_offline_recoveries ADD COLUMN IF NOT EXISTS background BOOLEAN NOT NULL DEFAULT false;
CREATE UNIQUE INDEX IF NOT EXISTS idx_offline_recovery_background_user
  ON player_offline_recoveries(user_id) WHERE background;
