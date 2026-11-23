SET lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS arena_replay_shares (
  token          TEXT        PRIMARY KEY,
  challenge_id   BIGINT      NOT NULL REFERENCES arena_challenges(id) ON DELETE CASCADE,
  created_by     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at     TIMESTAMPTZ,
  revoked_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_arena_replay_shares_challenge
  ON arena_replay_shares(challenge_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_arena_replay_shares_challenge_creator
  ON arena_replay_shares(challenge_id, created_by);

ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS attachment_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;
