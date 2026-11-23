-- Achievement system: reliable event queue and player state.
DROP TABLE IF EXISTS player_achievement_event_keys;
DROP TABLE IF EXISTS player_achievement_counters;
DROP TABLE IF EXISTS player_achievement_unlocks;

CREATE TABLE IF NOT EXISTS achievement_events (
  event_id      TEXT        PRIMARY KEY,
  user_id       BIGINT      NOT NULL,
  kind          TEXT        NOT NULL,
  counter_key   TEXT        NOT NULL,
  delta         BIGINT      NOT NULL DEFAULT 1,
  payload       JSONB       NOT NULL DEFAULT '{}'::jsonb,
  status        TEXT        NOT NULL DEFAULT 'pending',
  attempts      INT         NOT NULL DEFAULT 0,
  available_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by     TEXT        NOT NULL DEFAULT '',
  locked_until  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at  TIMESTAMPTZ,
  last_error    TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_achievement_events_claim
  ON achievement_events(status, available_at, locked_until);
CREATE INDEX IF NOT EXISTS idx_achievement_events_user_status
  ON achievement_events(user_id, status);
CREATE INDEX IF NOT EXISTS idx_achievement_events_processed
  ON achievement_events(status, processed_at);

CREATE TABLE IF NOT EXISTS player_achievement_states (
  user_id          BIGINT      NOT NULL,
  achievement_id   TEXT        NOT NULL,
  unlocked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at     TIMESTAMPTZ,
  claimed_at       TIMESTAMPTZ,
  progress_value   BIGINT      NOT NULL DEFAULT 0,
  definition_hash  TEXT        NOT NULL DEFAULT '',
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, achievement_id)
);
CREATE INDEX IF NOT EXISTS idx_player_achievement_states_user_completed
  ON player_achievement_states(user_id, completed_at);
CREATE INDEX IF NOT EXISTS idx_player_achievement_states_user_claimed
  ON player_achievement_states(user_id, claimed_at);
