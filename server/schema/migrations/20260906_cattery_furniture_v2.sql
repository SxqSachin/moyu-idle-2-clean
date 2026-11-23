-- Run after 20260906_cattery_furniture.sql, with old workers stopped.
BEGIN;
ALTER TABLE player_cattery_furniture_state
  ADD COLUMN IF NOT EXISTS contribution_totals JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS contribution_seen JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS contribution_report JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Correct V1's 20% cabinet only for future starts. Paid time and old costs
-- remain intact; the legacy version cannot refill at the old cheaper price.
-- Active run/checkpoint snapshots are deliberately not rewritten.
CREATE TEMP TABLE furniture_v2_balance_windows ON COMMIT DROP AS
SELECT id,user_id,furniture_id,GREATEST(valid_from,transaction_timestamp()) AS start_at,
       valid_until AS end_at,
       jsonb_set(jsonb_set(source,'{version}','"loot-cabinet-v1-balance-v2"'),'{rules}',
         (SELECT jsonb_agg(CASE WHEN r->>'kind'='kitty_extra_loot'
           THEN jsonb_set(r,'{rateBps}','500') ELSE r END ORDER BY ord)
          FROM jsonb_array_elements(source->'rules') WITH ORDINALITY AS rules(r,ord))) AS new_source
FROM player_cattery_furniture_periods
WHERE furniture_id='loot_cabinet' AND source->>'version'='loot-cabinet-v1'
  AND valid_until>transaction_timestamp();
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM player_cattery_furniture_periods p
    WHERE p.user_id IN (SELECT user_id FROM furniture_v2_balance_windows)
    GROUP BY p.user_id HAVING COUNT(*) +
      (SELECT COUNT(*) FROM furniture_v2_balance_windows w WHERE w.user_id=p.user_id)>4096)
  THEN RAISE EXCEPTION 'Furniture history limit: recover outstanding work before V2 migration';
  END IF;
END $$;
UPDATE player_cattery_furniture_periods p SET valid_until=w.start_at
FROM furniture_v2_balance_windows w WHERE p.id=w.id;
INSERT INTO player_cattery_furniture_periods(user_id,furniture_id,valid_from,valid_until,source)
SELECT user_id,furniture_id,start_at,end_at,new_source FROM furniture_v2_balance_windows;
WITH updated AS (
  UPDATE player_cattery_furniture SET definition=
    jsonb_set(jsonb_set(definition,'{version}','"loot-cabinet-v1-balance-v2"'),'{rules}',
      (SELECT jsonb_agg(CASE WHEN r->>'kind'='kitty_extra_loot'
        THEN jsonb_set(r,'{rateBps}','500') ELSE r END ORDER BY ord)
       FROM jsonb_array_elements(definition->'rules') WITH ORDINALITY AS rules(r,ord))),
    updated_at=transaction_timestamp()
  WHERE furniture_id='loot_cabinet' AND definition->>'version'='loot-cabinet-v1'
  RETURNING user_id
)
UPDATE player_cattery_furniture_state SET revision=revision+1
WHERE user_id IN (SELECT user_id FROM updated);
COMMIT;
