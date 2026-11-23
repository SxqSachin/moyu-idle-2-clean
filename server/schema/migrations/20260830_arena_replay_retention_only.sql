SET lock_timeout = '5s';

ALTER TABLE IF EXISTS arena_replay_shares
  DROP CONSTRAINT IF EXISTS arena_replay_shares_expiry;

ALTER TABLE IF EXISTS arena_replay_shares
  ALTER COLUMN expires_at DROP NOT NULL;

UPDATE arena_replay_shares
SET expires_at = NULL
WHERE expires_at IS NOT NULL;

DROP INDEX IF EXISTS idx_arena_replay_shares_expiry;
