ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'system';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS direction TEXT NOT NULL DEFAULT 'gain';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS subject_type TEXT NOT NULL DEFAULT '';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS subject_id TEXT NOT NULL DEFAULT '';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS item_totals JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS instance_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS audit_key TEXT NOT NULL DEFAULT '';

UPDATE player_warehouse_change_logs
SET item_totals = jsonb_build_object(template_item_id, quantity)
WHERE item_totals = '{}'::jsonb
  AND template_item_id <> '';

UPDATE player_warehouse_change_logs
SET instance_ids = jsonb_build_array(item_instance_id)
WHERE instance_ids = '[]'::jsonb
  AND item_instance_id IS NOT NULL;

UPDATE player_warehouse_change_logs
SET category = CASE
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%action%' THEN 'action_queue'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%offline_period%' THEN 'action_queue'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%battle%' THEN 'battle'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%auction%' THEN 'auction'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%market%' THEN 'market_trade'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%shop%' THEN 'shop'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%admin%' THEN 'admin'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%daily%' THEN 'daily_task'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%affix%' THEN 'affix'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%recycle%' THEN 'recycle'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%kitty%' THEN 'kitty'
    ELSE category
  END,
  direction = CASE
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%refund%' THEN 'refund'
    WHEN lower(source_surface || ' ' || target_surface || ' ' || COALESCE(details->>'type', '')) LIKE '%fee%' THEN 'fee'
    WHEN target_surface IN ('market', 'auction') THEN 'lock'
    WHEN event_type = 'system_to_warehouse' OR target_surface = 'inventory' THEN 'gain'
    ELSE 'consume'
  END;

CREATE INDEX IF NOT EXISTS idx_warehouse_change_logs_user_category_created
  ON player_warehouse_change_logs(user_id, category, created_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_warehouse_change_logs_audit_key
  ON player_warehouse_change_logs(audit_key)
  WHERE audit_key <> '';
