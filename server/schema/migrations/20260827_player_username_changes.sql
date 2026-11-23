BEGIN;

CREATE TABLE IF NOT EXISTS player_username_changes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  old_username TEXT NOT NULL CHECK(length(btrim(old_username)) > 0),
  new_username TEXT NOT NULL CHECK(length(btrim(new_username)) > 0),
  changed_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  change_source TEXT NOT NULL DEFAULT 'self_service' CHECK(length(btrim(change_source)) > 0),
  cost_item_id TEXT NOT NULL CHECK(length(btrim(cost_item_id)) > 0),
  cost_item_count BIGINT NOT NULL CHECK(cost_item_count > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  CHECK(old_username <> new_username)
);

CREATE INDEX IF NOT EXISTS idx_player_username_changes_user_created
  ON player_username_changes(user_id, created_at DESC, id DESC);

COMMIT;
