BEGIN;

ALTER TABLE player_custom_skill_revisions
  ADD COLUMN IF NOT EXISTS ruleset_version INT NOT NULL DEFAULT 1;

ALTER TABLE player_custom_skill_revisions
  DROP CONSTRAINT IF EXISTS player_custom_skill_revisions_ruleset_version_check;
ALTER TABLE player_custom_skill_revisions
  ADD CONSTRAINT player_custom_skill_revisions_ruleset_version_check
  CHECK(ruleset_version > 0);

CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_ruleset
  ON player_custom_skill_revisions(ruleset_version, result_status, user_id);

COMMIT;
