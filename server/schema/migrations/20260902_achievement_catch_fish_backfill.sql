-- Reconcile authored catch-fish achievements against the existing authoritative
-- per-species catch totals. The event is idempotent per user and intentionally
-- does not copy counts into the achievement queue payload.
INSERT INTO achievement_events(
  event_id,
  user_id,
  kind,
  counter_key,
  delta,
  payload,
  status,
  available_at
)
SELECT
  'fishing_achievement_backfill_v1:' || knowledge.user_id,
  knowledge.user_id,
  'state_reconcile',
  'state_reconcile:catch_fish',
  1,
  jsonb_build_object(
    'Kind', 'state_reconcile',
    'Count', 1,
    'StateKind', 'catch_fish',
    'SourceKey', 'fishing_achievement_backfill_v1'
  ),
  'pending',
  now()
FROM (
  SELECT DISTINCT user_id
  FROM fishing_species_knowledge
  WHERE catches > 0
) AS knowledge
ON CONFLICT (event_id) DO NOTHING;
