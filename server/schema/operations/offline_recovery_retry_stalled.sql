-- One bounded retry, NOT forced completion. Resolve the underlying error first.
-- Preview mode by default: the final ROLLBACK leaves all player data unchanged.
-- After reviewing the scan/error logs and this batch, change ONLY the final
-- ROLLBACK to COMMIT to apply once. Do not run this script in an automatic loop.
BEGIN;
SET LOCAL statement_timeout = '10s';
SET LOCAL lock_timeout = '2s';
SET LOCAL idle_in_transaction_session_timeout = '2min';

SELECT current_database() AS database_name, inet_server_addr() AS server_address,
       now() AS observed_at;

-- Capture/lock only stalled tasks. Running/complete tasks and busy rows stay put.
CREATE TEMP TABLE offline_recovery_retry_batch ON COMMIT DROP AS
SELECT r.id, r.user_id, r.background, r.revision, r.attempts, r.error_code,
       r.summary->>'completedSteps' AS completed_steps,
       r.summary->>'stalledQueueId' AS stalled_queue_id
FROM player_offline_recoveries r
WHERE r.status = 'stalled'
ORDER BY r.id
LIMIT 100
FOR UPDATE OF r SKIP LOCKED;

SELECT id, user_id, background, revision, attempts, error_code,
       completed_steps, stalled_queue_id
FROM offline_recovery_retry_batch
ORDER BY id;

-- Keep the original window, summary, pending battles, queue and reward receipts.
-- Increment revision so late workers/replayed requests cannot undo this retry.
UPDATE player_offline_recoveries r
SET status = 'running', revision = r.revision + 1,
    attempts = 0, error_code = '', available_at = now()
FROM offline_recovery_retry_batch b
WHERE r.id = b.id AND r.user_id = b.user_id
  AND r.revision = b.revision AND r.status = 'stalled'
RETURNING r.id, r.user_id, r.background,
          b.revision AS previous_revision, r.revision AS new_revision,
          b.error_code AS previous_error, r.status, r.available_at;

ROLLBACK;
