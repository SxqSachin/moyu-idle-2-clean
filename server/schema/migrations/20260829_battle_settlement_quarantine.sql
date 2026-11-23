SET lock_timeout = '5s';

ALTER TABLE battle_settlement_jobs
  DROP CONSTRAINT IF EXISTS battle_settlement_jobs_status_check,
  ADD CONSTRAINT battle_settlement_jobs_status_check
  CHECK (status IN ('pending', 'running', 'done', 'failed', 'quarantined')) NOT VALID;

ALTER TABLE battle_settlement_jobs
  VALIDATE CONSTRAINT battle_settlement_jobs_status_check;

ALTER TABLE battle_skill_exp_jobs
  DROP CONSTRAINT IF EXISTS battle_skill_exp_jobs_status_check,
  ADD CONSTRAINT battle_skill_exp_jobs_status_check
  CHECK (status IN ('pending', 'running', 'done', 'failed', 'quarantined')) NOT VALID;

ALTER TABLE battle_skill_exp_jobs
  VALIDATE CONSTRAINT battle_skill_exp_jobs_status_check;

ALTER TABLE battle_attr_exp_jobs
  DROP CONSTRAINT IF EXISTS battle_attr_exp_jobs_status_check,
  ADD CONSTRAINT battle_attr_exp_jobs_status_check
  CHECK (status IN ('pending', 'running', 'done', 'failed', 'quarantined')) NOT VALID;

ALTER TABLE battle_attr_exp_jobs
  VALIDATE CONSTRAINT battle_attr_exp_jobs_status_check;
