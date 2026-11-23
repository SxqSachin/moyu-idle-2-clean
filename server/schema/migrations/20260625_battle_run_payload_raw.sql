ALTER TABLE battle_run_payloads
  ADD COLUMN IF NOT EXISTS compile_input_raw BYTEA,
  ADD COLUMN IF NOT EXISTS timeline_raw BYTEA,
  ADD COLUMN IF NOT EXISTS final_result_raw BYTEA;
