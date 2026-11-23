-- Cat village commission latency diagnostics.
-- Run with:
--   docker compose -f infra/docker-compose.yml exec -T pg psql -U game -d game < server/schema/cat_village_pg_diagnostics.sql
--
-- The EXPLAIN ANALYZE statements are SELECT-only, but the inventory query uses
-- FOR UPDATE to match runtime behavior and may briefly wait on row locks.

\echo '== index presence =='
SELECT
  to_regclass('public.idx_player_item_instances_user_template_state_enhance') AS inventory_template_state_enhance,
  to_regclass('public.idx_player_cat_village_tasks_user_status') AS cat_village_user_status,
  to_regclass('public.uq_warehouse_change_logs_audit_key') AS warehouse_audit_key;

\echo '== table stats =='
SELECT
  relname,
  seq_scan,
  seq_tup_read,
  idx_scan,
  n_live_tup,
  n_dead_tup
FROM pg_stat_user_tables
WHERE relname IN (
  'player_cat_village_tasks',
  'player_cat_village_task_state',
  'player_cat_village_reputation',
  'player_rumor_pity',
  'player_rumor_completion',
  'player_item_instances',
  'player_warehouse_change_logs',
  'achievement_events'
)
ORDER BY relname;

\echo '== heaviest cat village users =='
SELECT
  user_id,
  count(*) AS task_rows,
  count(*) FILTER (WHERE status = 'open') AS open_task_rows
FROM player_cat_village_tasks
GROUP BY user_id
ORDER BY task_rows DESC
LIMIT 10;

\echo '== cat village open task plan =='
EXPLAIN (ANALYZE, BUFFERS)
WITH diag_user AS (
  SELECT user_id
  FROM player_cat_village_tasks
  GROUP BY user_id
  ORDER BY count(*) DESC
  LIMIT 1
)
SELECT
  id,
  commission_id,
  costs_jsonb,
  reward_items,
  reputation_grant,
  refreshed,
  status,
  created_at,
  completed_at,
  not_refreshable,
  not_deletable
FROM player_cat_village_tasks
WHERE user_id = (SELECT user_id FROM diag_user)
  AND status = 'open'
ORDER BY id DESC;

\echo '== inventory template lock plan =='
EXPLAIN (ANALYZE, BUFFERS)
WITH diag_user AS (
  SELECT user_id
  FROM player_item_instances
  GROUP BY user_id
  ORDER BY count(*) DESC
  LIMIT 1
),
diag_item AS (
  SELECT template_item_id
  FROM player_item_instances
  WHERE user_id = (SELECT user_id FROM diag_user)
  GROUP BY template_item_id
  ORDER BY count(*) DESC
  LIMIT 1
)
SELECT
  instance_id,
  user_id,
  template_item_id,
  COALESCE(custom_data, '{}'::jsonb),
  quantity,
  enhance_level,
  roll_seed,
  base_snapshot,
  affixes,
  durability,
  max_durability,
  state
FROM player_item_instances
WHERE user_id = (SELECT user_id FROM diag_user)
  AND template_item_id = ANY(ARRAY[(SELECT template_item_id FROM diag_item)])
  AND state = 'inventory'
  AND COALESCE(custom_data, '{}'::jsonb) = '{}'::jsonb
  AND enhance_level = 0
ORDER BY instance_id
FOR UPDATE;

\echo '== current blocking =='
SELECT
  blocked.pid AS blocked_pid,
  blocking.pid AS blocking_pid,
  left(blocked.query, 160) AS blocked_query,
  left(blocking.query, 160) AS blocking_query,
  now() - blocked.query_start AS wait_duration
FROM pg_stat_activity blocked
JOIN pg_stat_activity blocking
  ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
ORDER BY wait_duration DESC;
