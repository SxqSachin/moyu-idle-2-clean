-- Fishing pond V5 cutover. Apply manually while all older application nodes
-- are drained; V4 nodes do not preserve pondZone when rewriting fish_data.
BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE fishing_ponds
  ADD COLUMN IF NOT EXISTS ruleset_version TEXT;

-- Every row that predates this migration owns an unsettled V4 interval. The
-- V5 server settles that interval with V4 rules, then promotes the row to V5.
UPDATE fishing_ponds
SET ruleset_version = 'fishing-pond-v4'
WHERE ruleset_version IS NULL OR btrim(ruleset_version) = '';

ALTER TABLE fishing_ponds
  ALTER COLUMN ruleset_version SET DEFAULT 'fishing-pond-v5',
  ALTER COLUMN ruleset_version SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_fishing_ponds_ruleset_version'
  ) THEN
    ALTER TABLE fishing_ponds
      ADD CONSTRAINT chk_fishing_ponds_ruleset_version
      CHECK (ruleset_version IN ('fishing-pond-v4', 'fishing-pond-v5')) NOT VALID;
  END IF;
END $$;

ALTER TABLE fishing_ponds
  VALIDATE CONSTRAINT chk_fishing_ponds_ruleset_version;

-- Legacy pond fish had no zone. Preserve every V4 owner's fish in the main
-- pond; for V5 owners, repair only missing/invalid zones. Nursery rows owned
-- by an already-promoted V5 pond remain untouched on a safe re-run.
UPDATE player_item_instances AS instance
SET fish_data = jsonb_set(instance.fish_data, '{pondZone}', to_jsonb('main'::text), true),
    updated_at = now()
WHERE instance.state = 'fishing_pond'
  AND instance.template_item_id = 'fish_catch'
  AND (
    COALESCE(instance.fish_data->>'pondZone', '') NOT IN ('main', 'nursery')
    OR EXISTS (
      SELECT 1
      FROM fishing_ponds AS pond
      WHERE pond.user_id = instance.user_id
        AND pond.ruleset_version = 'fishing-pond-v4'
        AND instance.fish_data->>'pondZone' IS DISTINCT FROM 'main'
    )
  );

-- This is also a mixed-version fence: an old node that drops pondZone while
-- rewriting a pond fish fails its transaction instead of silently merging the
-- nursery back into the main pond.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_player_item_instances_fishing_pond_zone'
  ) THEN
    ALTER TABLE player_item_instances
      ADD CONSTRAINT chk_player_item_instances_fishing_pond_zone
      CHECK (
        state <> 'fishing_pond'
        OR template_item_id <> 'fish_catch'
        OR COALESCE(fish_data->>'pondZone', '') IN ('main', 'nursery')
      ) NOT VALID;
  END IF;
END $$;

ALTER TABLE player_item_instances
  VALIDATE CONSTRAINT chk_player_item_instances_fishing_pond_zone;

COMMIT;
