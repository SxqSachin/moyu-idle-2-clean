BEGIN;

ALTER TABLE player_custom_skill_drafts
  ADD COLUMN IF NOT EXISTS source_skillbook_item_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS source_skillbook_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE player_custom_skill_drafts DROP CONSTRAINT IF EXISTS player_custom_skill_drafts_source_kind_check;
ALTER TABLE player_custom_skill_drafts ADD CONSTRAINT player_custom_skill_drafts_source_kind_check CHECK(source_kind IN ('blank', 'official'));
ALTER TABLE player_custom_skill_drafts DROP CONSTRAINT IF EXISTS player_custom_skill_drafts_source_skillbook_count_check;
ALTER TABLE player_custom_skill_drafts ADD CONSTRAINT player_custom_skill_drafts_source_skillbook_count_check CHECK(source_skillbook_count >= 0);

ALTER TABLE player_custom_skill_revisions
  ADD COLUMN IF NOT EXISTS requested_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS consumed_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS refunded_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS source_skillbook_item_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS source_skillbook_count INT NOT NULL DEFAULT 0;
ALTER TABLE player_custom_skill_revisions DROP CONSTRAINT IF EXISTS player_custom_skill_revisions_result_status_check;
ALTER TABLE player_custom_skill_revisions ADD CONSTRAINT player_custom_skill_revisions_result_status_check CHECK(result_status IN ('pending', 'accepted', 'discarded', 'superseded', 'failed'));

CREATE TABLE IF NOT EXISTS player_forged_skillbooks (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  instance_id BIGINT UNIQUE REFERENCES player_item_instances(instance_id) ON DELETE SET NULL,
  custom_skill_id TEXT NOT NULL UNIQUE REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  revision_id TEXT NOT NULL UNIQUE REFERENCES player_custom_skill_revisions(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'inventory' CHECK(status IN ('inventory', 'learned_player', 'learned_kitty', 'destroyed', 'archived')),
  target_kitty_id BIGINT,
  learned_at TIMESTAMPTZ,
  destroyed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_forged_skillbooks_user_active
  ON player_forged_skillbooks(user_id, status, created_at DESC);

COMMIT;
