-- Apply outside an explicit transaction block. PostgreSQL does not allow
-- DROP INDEX CONCURRENTLY inside BEGIN/COMMIT.
DROP INDEX CONCURRENTLY IF EXISTS idx_player_item_instances_user_active_instance_desc;
