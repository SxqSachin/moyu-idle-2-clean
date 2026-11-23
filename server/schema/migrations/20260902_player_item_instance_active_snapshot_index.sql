-- Apply outside an explicit transaction block. PostgreSQL does not allow
-- CREATE INDEX CONCURRENTLY inside BEGIN/COMMIT.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_player_item_instances_user_active_instance_desc
  ON player_item_instances(user_id, instance_id DESC)
  WHERE state NOT IN ('consumed', 'destroyed');
