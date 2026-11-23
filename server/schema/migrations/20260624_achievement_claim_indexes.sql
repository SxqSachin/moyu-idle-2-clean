CREATE INDEX IF NOT EXISTS idx_achievement_events_pending_claim
  ON achievement_events(status, available_at, created_at);

CREATE INDEX IF NOT EXISTS idx_achievement_events_processing_claim
  ON achievement_events(status, locked_until, created_at);
