BEGIN;

ALTER TABLE player_custom_skill_drafts
  DROP CONSTRAINT IF EXISTS player_custom_skill_drafts_source_skillbook_count_check;
ALTER TABLE player_custom_skill_drafts
  ADD CONSTRAINT player_custom_skill_drafts_source_skillbook_count_check
  CHECK(source_skillbook_count >= 0);

COMMIT;
