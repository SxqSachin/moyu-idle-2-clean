SET lock_timeout = '5s';

-- === Cat Village Combat Trial ===

CREATE TABLE IF NOT EXISTS combat_trial_lineups (
  user_id          BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision         BIGINT      NOT NULL DEFAULT 1 CHECK (revision > 0),
  selection_jsonb  JSONB       NOT NULL DEFAULT '{}'::jsonb,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS combat_trial_runs (
  id                     BIGSERIAL   PRIMARY KEY,
  battle_kind            TEXT        NOT NULL DEFAULT 'combat_trial' CHECK (battle_kind = 'combat_trial'),
  user_id                BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  format_id              TEXT        NOT NULL,
  format_definition_digest TEXT      NOT NULL,
  seed                   BIGINT      NOT NULL CHECK (seed > 0),
  status                 TEXT        NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'running', 'completed', 'failed')),
  worker_id              TEXT        NOT NULL DEFAULT '',
  lease_until            TIMESTAMPTZ,
  attempts               INT         NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  error_code             TEXT        NOT NULL DEFAULT '',
  lineup_jsonb           JSONB       NOT NULL DEFAULT '[]'::jsonb,
  lineup_digest          TEXT        NOT NULL,
  format_snapshot_jsonb  JSONB       NOT NULL DEFAULT '{}'::jsonb,
  metrics_jsonb          JSONB       NOT NULL DEFAULT '{}'::jsonb,
  action_count           INT         NOT NULL DEFAULT 0 CHECK (action_count >= 0),
  action_budget          INT         NOT NULL CHECK (action_budget > 0),
  end_reason             TEXT        NOT NULL DEFAULT '',
  kitty_count            INT         NOT NULL DEFAULT 0 CHECK (kitty_count BETWEEN 0 AND 2),
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at             TIMESTAMPTZ,
  completed_at           TIMESTAMPTZ,
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_combat_trial_run_action_budget CHECK (action_count <= action_budget),
  CONSTRAINT chk_combat_trial_run_end_reason CHECK (end_reason = '' OR end_reason ~ '^[a-z0-9_:-]{1,64}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_combat_trial_runs_user_active
  ON combat_trial_runs(user_id)
  WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_combat_trial_runs_claim
  ON combat_trial_runs(status, lease_until, created_at, id)
  WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_combat_trial_runs_user_history
  ON combat_trial_runs(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_combat_trial_runs_current_format
  ON combat_trial_runs(format_definition_digest, completed_at DESC, id DESC)
  WHERE status = 'completed';

CREATE TABLE IF NOT EXISTS combat_trial_run_payloads (
  run_id               BIGINT PRIMARY KEY REFERENCES combat_trial_runs(id) ON DELETE CASCADE,
  initial_state_jsonb  JSONB  NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb       JSONB  NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb   JSONB  NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS combat_trial_personal_bests (
  user_id                 BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  format_id               TEXT             NOT NULL,
  format_definition_digest TEXT            NOT NULL,
  metric_id               TEXT             NOT NULL,
  value                   DOUBLE PRECISION NOT NULL CHECK (value >= 0),
  metrics_jsonb           JSONB            NOT NULL DEFAULT '{}'::jsonb,
  kitty_count             INT              NOT NULL DEFAULT 0 CHECK (kitty_count BETWEEN 0 AND 2),
  achieved_at             TIMESTAMPTZ      NOT NULL,
  run_id                  BIGINT            REFERENCES combat_trial_runs(id) ON DELETE SET NULL,
  PRIMARY KEY (user_id, format_definition_digest, metric_id)
);

CREATE INDEX IF NOT EXISTS idx_combat_trial_personal_bests_leaderboard
  ON combat_trial_personal_bests(format_definition_digest, metric_id, value DESC, achieved_at ASC, user_id ASC);

CREATE TABLE IF NOT EXISTS combat_trial_replay_shares (
  token       TEXT        PRIMARY KEY,
  run_id      BIGINT      NOT NULL REFERENCES combat_trial_runs(id) ON DELETE CASCADE,
  created_by  BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_combat_trial_replay_shares_run
  ON combat_trial_replay_shares(run_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_combat_trial_replay_shares_run_creator
  ON combat_trial_replay_shares(run_id, created_by);
