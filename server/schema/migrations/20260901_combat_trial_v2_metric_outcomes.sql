SET lock_timeout = '5s';

-- A personal best keeps the complete metric snapshot after its source run is
-- pruned. Preserve the source outcome as well so secondary metrics can still
-- honor their authored eligibleEndReasons contract.
ALTER TABLE combat_trial_personal_bests
  ADD COLUMN IF NOT EXISTS end_reason TEXT NOT NULL DEFAULT '';

UPDATE combat_trial_personal_bests pb
SET end_reason = r.end_reason
FROM combat_trial_runs r
WHERE pb.run_id = r.id
  AND pb.end_reason = '';

ALTER TABLE combat_trial_personal_bests
  DROP CONSTRAINT IF EXISTS chk_combat_trial_personal_best_end_reason;

ALTER TABLE combat_trial_personal_bests
  ADD CONSTRAINT chk_combat_trial_personal_best_end_reason
  CHECK (end_reason = '' OR end_reason ~ '^[a-z0-9_:-]{1,64}$');
