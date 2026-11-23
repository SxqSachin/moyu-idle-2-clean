BEGIN;

CREATE TABLE IF NOT EXISTS player_skill_forge_idempotency_receipts (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK(length(idempotency_key) BETWEEN 1 AND 160),
  response_jsonb JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_player_skill_forge_receipts_user_created
  ON player_skill_forge_idempotency_receipts(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_user_failed_created
  ON player_custom_skill_revisions(user_id, created_at DESC, id DESC)
  WHERE result_status = 'failed';

COMMIT;
