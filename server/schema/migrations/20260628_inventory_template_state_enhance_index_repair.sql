CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_player_item_instances_user_template_state_enhance
  ON player_item_instances(user_id, template_item_id, state, enhance_level);
