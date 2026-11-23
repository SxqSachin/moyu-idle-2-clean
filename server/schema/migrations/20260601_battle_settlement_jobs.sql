ALTER TABLE battle_runs
  DROP CONSTRAINT IF EXISTS battle_runs_status_check;

ALTER TABLE battle_runs
  ADD CONSTRAINT battle_runs_status_check
  CHECK(status IN ('compiled', 'playing', 'settling', 'ended', 'failed', 'interrupted'));

CREATE TABLE IF NOT EXISTS battle_settlement_jobs (
  id BIGSERIAL PRIMARY KEY,
  room_id BIGINT NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  run_id BIGINT NOT NULL REFERENCES battle_runs(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'done', 'failed')),
  attempts INT NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  last_error TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by TEXT NOT NULL DEFAULT '',
  locked_until TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(run_id)
);

CREATE INDEX IF NOT EXISTS idx_battle_settlement_jobs_claim
  ON battle_settlement_jobs(status, available_at, locked_until, id);
