CREATE TABLE IF NOT EXISTS battle_attr_exp_jobs (
  id BIGSERIAL PRIMARY KEY,
  run_id BIGINT NOT NULL REFERENCES battle_runs(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'done', 'failed')),
  payload_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  attempts INT NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  last_error TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by TEXT NOT NULL DEFAULT '',
  locked_until TIMESTAMPTZ,
  applied_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(run_id)
);

CREATE INDEX IF NOT EXISTS idx_battle_attr_exp_jobs_claim
  ON battle_attr_exp_jobs(status, available_at, locked_until, id);
CREATE INDEX IF NOT EXISTS idx_battle_attr_exp_jobs_ready
  ON battle_attr_exp_jobs(available_at, id)
  WHERE applied_at IS NULL AND status IN ('pending', 'failed');
CREATE INDEX IF NOT EXISTS idx_battle_attr_exp_jobs_expired_lease
  ON battle_attr_exp_jobs(locked_until, id)
  WHERE applied_at IS NULL AND status = 'running';
