BEGIN;

CREATE INDEX IF NOT EXISTS idx_player_custom_skill_drafts_user_active_updated
  ON player_custom_skill_drafts(user_id, updated_at DESC)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_user_pending_created
  ON player_custom_skill_revisions(user_id, created_at)
  WHERE result_status = 'pending';

COMMIT;
