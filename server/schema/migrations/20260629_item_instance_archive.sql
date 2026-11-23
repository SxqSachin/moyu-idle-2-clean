ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_player_item_instances_unarchived_template_state_enhance;
