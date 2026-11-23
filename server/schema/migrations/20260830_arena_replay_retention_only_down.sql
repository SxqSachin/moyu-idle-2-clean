SET lock_timeout = '5s';

UPDATE arena_replay_shares
SET expires_at = GREATEST(created_at + INTERVAL '1 second', now() + INTERVAL '7 days')
WHERE expires_at IS NULL;

ALTER TABLE IF EXISTS arena_replay_shares
  ALTER COLUMN expires_at SET NOT NULL;

ALTER TABLE IF EXISTS arena_replay_shares
  ADD CONSTRAINT arena_replay_shares_expiry CHECK (expires_at > created_at);

CREATE INDEX IF NOT EXISTS idx_arena_replay_shares_expiry
  ON arena_replay_shares(expires_at);
