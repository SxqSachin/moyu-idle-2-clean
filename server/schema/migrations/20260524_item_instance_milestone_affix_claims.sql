ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS milestone_affix_claims JSONB NOT NULL DEFAULT '{}'::jsonb;

