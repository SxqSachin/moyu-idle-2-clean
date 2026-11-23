CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_player_action_queue_running_due_user
  ON player_action_queue(user_id, current_run_ends_at ASC NULLS FIRST, queue_order, id)
  WHERE status = 'running';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_player_action_queue_pending_user_order
  ON player_action_queue(user_id, queue_order, id, actor_type, actor_id)
  WHERE status = 'pending';
