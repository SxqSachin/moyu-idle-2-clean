DROP INDEX IF EXISTS idx_fishing_trips_spot_running;
ALTER TABLE fishing_trips DROP COLUMN IF EXISTS supplies_jsonb;
DROP TABLE IF EXISTS player_one_time_grants;

