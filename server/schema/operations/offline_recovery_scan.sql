-- Read-only diagnostics. Run on the affected environment, not a local replica.
BEGIN READ ONLY;
SET LOCAL statement_timeout = '10s';
SET LOCAL lock_timeout = '2s';

SELECT current_database() AS database_name, inet_server_addr() AS server_address,
       now() AS observed_at;

SELECT status, background, count(*) AS task_count,
       min(created_at) AS oldest_created_at, min(available_at) AS earliest_available_at
FROM player_offline_recoveries
WHERE status IN ('running', 'stalled')
GROUP BY status, background
ORDER BY status, background;

-- A running task is a diagnostic candidate, not permission to reset its state.
-- Sample twice to distinguish an advancing task from a persistent failure.
SELECT id, user_id, status, background, revision, attempts, error_code,
       available_at, started_at, target_at,
       summary->>'completedSteps' AS completed_steps,
       summary->>'settledUntilMS' AS settled_until_ms,
       summary->>'stalledQueueId' AS stalled_queue_id,
       pending_run_ids
FROM player_offline_recoveries
WHERE status = 'stalled'
   OR (status = 'running' AND (attempts > 0 OR available_at < now() - interval '5 minutes'))
ORDER BY id
LIMIT 100;

-- These tasks already finished action settlement. Repair the battle worker,
-- not the action receipt. Match the ACK gate, including earlier attribution.
SELECT j.id AS recovery_id, j.user_id, j.revision,
       b.id AS pending_battle_run_id, b.status AS battle_status, b.started_at
FROM player_offline_recoveries j
CROSS JOIN LATERAL unnest(j.pending_run_ids) AS pending(run_id)
JOIN battle_runs b ON b.id = pending.run_id
WHERE j.status = 'complete' AND NOT j.background AND j.acknowledged_at IS NULL
  AND b.status IN ('compiled', 'playing', 'settling')
  AND NOT EXISTS (
    SELECT 1 FROM battle_reward_receipts r
    WHERE r.user_id = j.user_id AND r.run_id = b.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM player_offline_recoveries prior
    WHERE prior.user_id = j.user_id AND prior.started_at < j.started_at
      AND b.id = ANY(prior.pending_run_ids)
  )
ORDER BY j.id, b.id
LIMIT 100;

COMMIT;
