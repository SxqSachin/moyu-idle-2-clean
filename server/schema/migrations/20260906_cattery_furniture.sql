BEGIN;
CREATE TABLE IF NOT EXISTS player_cattery_furniture_state (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
  exp_remainders JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE TABLE IF NOT EXISTS player_cattery_furniture (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  furniture_id TEXT NOT NULL,
  definition JSONB NOT NULL,
  remaining_ms BIGINT NOT NULL CHECK (remaining_ms >= 0),
  enabled_until TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (user_id, furniture_id)
);
CREATE TABLE IF NOT EXISTS player_cattery_furniture_periods (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  furniture_id TEXT NOT NULL,
  valid_from TIMESTAMPTZ NOT NULL,
  valid_until TIMESTAMPTZ NOT NULL,
  source JSONB NOT NULL,
  CHECK (valid_until >= valid_from)
);
CREATE INDEX IF NOT EXISTS cattery_furniture_period_window_idx
  ON player_cattery_furniture_periods(user_id, valid_until, valid_from);
CREATE TABLE IF NOT EXISTS player_cattery_furniture_commands (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  request_id TEXT NOT NULL,
  request JSONB NOT NULL,
  revision BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, request_id)
);
CREATE INDEX IF NOT EXISTS cattery_furniture_command_cleanup_idx
  ON player_cattery_furniture_commands(user_id, created_at);
COMMIT;
