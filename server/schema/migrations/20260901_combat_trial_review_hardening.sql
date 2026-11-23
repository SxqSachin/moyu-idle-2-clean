SET lock_timeout = '5s';

-- Trial composition is submitted with each run and frozen into that run's
-- snapshot. Remove the obsolete preference table so starting a trial has no
-- hidden persistence side effect.
DROP TABLE IF EXISTS combat_trial_lineups;

-- Lower-is-better projects need the same stable leaderboard access path as
-- higher-is-better projects.
CREATE INDEX IF NOT EXISTS idx_combat_trial_personal_bests_leaderboard_asc
  ON combat_trial_personal_bests(format_definition_digest, metric_id, value ASC, achieved_at ASC, user_id ASC);
