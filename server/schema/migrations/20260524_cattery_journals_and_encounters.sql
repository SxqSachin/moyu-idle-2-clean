CREATE TABLE IF NOT EXISTS player_kitty_encounters (
  id BIGSERIAL PRIMARY KEY,
  from_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  from_kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  to_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  to_kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  content_key TEXT NOT NULL,
  content_params_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (from_kitty_id <> to_kitty_id),
  CHECK (from_user_id <> to_user_id)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_from_kitty_created
  ON player_kitty_encounters(from_kitty_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_to_kitty_created
  ON player_kitty_encounters(to_kitty_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_from_user_created
  ON player_kitty_encounters(from_user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_to_user_created
  ON player_kitty_encounters(to_user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS player_kitty_journals (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  encounter_id BIGINT REFERENCES player_kitty_encounters(id) ON DELETE SET NULL,
  entry_type TEXT NOT NULL DEFAULT 'diary' CHECK (entry_type IN ('diary', 'encounter')),
  title_key TEXT NOT NULL DEFAULT '',
  content_key TEXT NOT NULL,
  content_params_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  action_id TEXT NOT NULL DEFAULT '',
  mood DOUBLE PRECISION,
  reward_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_journals_user_kitty_created
  ON player_kitty_journals(user_id, kitty_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_journals_encounter
  ON player_kitty_journals(encounter_id, id DESC);
