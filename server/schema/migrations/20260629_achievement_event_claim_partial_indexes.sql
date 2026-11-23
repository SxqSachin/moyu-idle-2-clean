-- Must be applied outside an explicit transaction block.
-- PostgreSQL does not allow CREATE INDEX CONCURRENTLY inside BEGIN/COMMIT.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_achievement_events_pending_due
  ON achievement_events(available_at, created_at, event_id)
  WHERE status = 'pending';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_achievement_events_processing_due
  ON achievement_events(locked_until, created_at, event_id)
  WHERE status = 'processing' AND locked_until IS NOT NULL;
