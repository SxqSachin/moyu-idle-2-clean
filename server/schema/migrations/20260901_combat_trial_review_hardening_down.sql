SET lock_timeout = '5s';

DROP INDEX IF EXISTS idx_combat_trial_personal_bests_leaderboard_asc;

CREATE TABLE IF NOT EXISTS combat_trial_lineups (
  user_id          BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision         BIGINT      NOT NULL DEFAULT 1 CHECK (revision > 0),
  selection_jsonb  JSONB       NOT NULL DEFAULT '{}'::jsonb,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
