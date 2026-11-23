ALTER TABLE player_kitties
  ADD COLUMN IF NOT EXISTS level INT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS exp BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS total_exp BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bond_exp BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS mood DOUBLE PRECISION NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_commission_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_level_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_level_check CHECK (level >= 1);

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_exp_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_exp_check CHECK (exp >= 0);

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_total_exp_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_total_exp_check CHECK (total_exp >= 0);

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_bond_exp_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_bond_exp_check CHECK (bond_exp >= 0);

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_mood_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_mood_check CHECK (mood >= -1 AND mood <= 1);

CREATE TABLE IF NOT EXISTS player_kitty_commissions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  commission_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'claimed', 'cancelled')),
  costs_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  reward_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ready_at TIMESTAMPTZ NOT NULL,
  claimed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_player_kitty_commissions_user_kitty_started
  ON player_kitty_commissions(user_id, kitty_id, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_player_kitty_commissions_user_kitty_status
  ON player_kitty_commissions(user_id, kitty_id, status, ready_at, id);
