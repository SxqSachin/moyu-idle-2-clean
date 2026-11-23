-- Apply outside an explicit transaction block. PostgreSQL does not allow
-- DROP INDEX CONCURRENTLY inside BEGIN/COMMIT.
--
-- Production usage and the current claim SQL confirm that the status-specific
-- indexes below replace the broad claim index. The application has no
-- user/status lookup on this short-lived event queue.
DO $$
DECLARE
  replacement_index TEXT;
BEGIN
  FOREACH replacement_index IN ARRAY ARRAY[
    'idx_achievement_events_pending_claim',
    'idx_achievement_events_processing_claim',
    'idx_achievement_events_pending_due',
    'idx_achievement_events_processing_due'
  ]
  LOOP
    IF NOT EXISTS (
      SELECT 1
      FROM pg_index
      WHERE indexrelid = to_regclass(replacement_index)
        AND indisready
        AND indisvalid
    ) THEN
      RAISE EXCEPTION 'replacement index % is missing, not ready, or invalid', replacement_index;
    END IF;
  END LOOP;
END $$;

DROP INDEX CONCURRENTLY IF EXISTS idx_achievement_events_claim;
DROP INDEX CONCURRENTLY IF EXISTS idx_achievement_events_user_status;
