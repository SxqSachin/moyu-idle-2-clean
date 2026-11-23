SET lock_timeout = '5s';

UPDATE combat_trial_runs
SET end_reason = 'action_budget_reached'
WHERE end_reason NOT IN ('', 'action_budget_reached', 'team_defeated');

ALTER TABLE combat_trial_runs
  DROP CONSTRAINT IF EXISTS chk_combat_trial_run_end_reason;
ALTER TABLE combat_trial_runs
  ADD CONSTRAINT combat_trial_runs_end_reason_check
  CHECK (end_reason IN ('', 'action_budget_reached', 'team_defeated'));
