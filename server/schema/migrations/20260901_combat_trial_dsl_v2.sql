SET lock_timeout = '5s';

-- End reasons are authored by the combat-trial format DSL. Keep the database
-- constraint structural so new formats do not require a schema migration.
ALTER TABLE combat_trial_runs
  DROP CONSTRAINT IF EXISTS combat_trial_runs_end_reason_check;
ALTER TABLE combat_trial_runs
  DROP CONSTRAINT IF EXISTS chk_combat_trial_run_end_reason;
ALTER TABLE combat_trial_runs
  ADD CONSTRAINT chk_combat_trial_run_end_reason
  CHECK (end_reason = '' OR end_reason ~ '^[a-z0-9_:-]{1,64}$');
