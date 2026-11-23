SET lock_timeout = '5s';

ALTER TABLE combat_trial_personal_bests
  DROP CONSTRAINT IF EXISTS chk_combat_trial_personal_best_end_reason;

ALTER TABLE combat_trial_personal_bests
  DROP COLUMN IF EXISTS end_reason;
