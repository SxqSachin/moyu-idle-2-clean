CREATE TABLE IF NOT EXISTS player_loadouts (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  loadout_id TEXT NOT NULL,
  type TEXT NOT NULL CHECK(type IN ('battle', 'work')),
  name TEXT NOT NULL DEFAULT '',
  payload_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, loadout_id)
);

CREATE INDEX IF NOT EXISTS idx_player_loadouts_user_type_updated
  ON player_loadouts(user_id, type, updated_at DESC, loadout_id);

CREATE TABLE IF NOT EXISTS player_work_loadout_bindings (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action_main_type TEXT NOT NULL,
  loadout_id TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, action_main_type)
);

CREATE INDEX IF NOT EXISTS idx_player_work_loadout_bindings_user_loadout
  ON player_work_loadout_bindings(user_id, loadout_id);
