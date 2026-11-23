ALTER TABLE battle_runs
  ADD COLUMN IF NOT EXISTS result_summary_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS battle_run_payloads (
  run_id BIGINT PRIMARY KEY REFERENCES battle_runs(id) ON DELETE CASCADE,
  compile_input_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS battle_run_carry_states (
  run_id BIGINT PRIMARY KEY REFERENCES battle_runs(id) ON DELETE CASCADE,
  carry_state_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
