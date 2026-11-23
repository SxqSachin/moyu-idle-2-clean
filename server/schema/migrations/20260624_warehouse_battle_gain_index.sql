CREATE INDEX IF NOT EXISTS idx_warehouse_change_logs_user_category_direction_created
  ON player_warehouse_change_logs(user_id, category, direction, created_at DESC, id DESC);
