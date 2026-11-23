BEGIN;
ALTER TABLE IF EXISTS player_custom_skill_drafts
  DROP CONSTRAINT IF EXISTS player_custom_skill_drafts_pending_revision_fk;
ALTER TABLE IF EXISTS player_custom_skills
  DROP CONSTRAINT IF EXISTS player_custom_skills_current_revision_fk;
DROP TABLE IF EXISTS player_forged_skillbooks;
DROP TABLE IF EXISTS player_custom_skill_revisions;
DROP TABLE IF EXISTS player_custom_skill_drafts;
DROP TABLE IF EXISTS player_custom_skills;
COMMIT;
