-- Mark existing open daily tasks as completed before migration
UPDATE player_daily_tasks SET status = 'completed', completed_at = now() WHERE status = 'open';

-- Rename tables
ALTER TABLE player_daily_tasks RENAME TO player_cat_village_tasks;
ALTER TABLE player_daily_task_state RENAME TO player_cat_village_task_state;

-- Drop FK and old template_id column
ALTER TABLE player_cat_village_tasks DROP COLUMN IF EXISTS template_id;

-- Add new columns
ALTER TABLE player_cat_village_tasks ADD COLUMN IF NOT EXISTS commission_id TEXT NOT NULL DEFAULT '';
ALTER TABLE player_cat_village_tasks ADD COLUMN IF NOT EXISTS reputation_grant BIGINT NOT NULL DEFAULT 0;

-- Rename index
ALTER INDEX IF EXISTS idx_player_daily_tasks_user_status RENAME TO idx_player_cat_village_tasks_user_status;

-- Drop old templates table (no FK references after column drop above)
DROP TABLE IF EXISTS daily_task_templates;

-- Cat village reputation table
CREATE TABLE IF NOT EXISTS player_cat_village_reputation (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  reputation BIGINT NOT NULL DEFAULT 0 CHECK(reputation >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
