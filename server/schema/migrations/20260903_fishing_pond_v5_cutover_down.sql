-- Refuse a lossy rollback. Move every nursery fish into the main pond (or
-- remove it from the pond) with the V5 server before applying this migration.
BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM player_item_instances
    WHERE state = 'fishing_pond'
      AND template_item_id = 'fish_catch'
      AND fish_data->>'pondZone' = 'nursery'
  ) THEN
    RAISE EXCEPTION 'cannot roll back fishing pond V5 while nursery fish exist';
  END IF;
END $$;

ALTER TABLE player_item_instances
  DROP CONSTRAINT IF EXISTS chk_player_item_instances_fishing_pond_zone;

UPDATE player_item_instances
SET fish_data = fish_data - 'pondZone',
    updated_at = now()
WHERE state = 'fishing_pond'
  AND template_item_id = 'fish_catch';

ALTER TABLE fishing_ponds
  DROP CONSTRAINT IF EXISTS chk_fishing_ponds_ruleset_version,
  DROP COLUMN IF EXISTS ruleset_version;

COMMIT;
