ALTER TABLE market_auctions DROP COLUMN IF EXISTS instance_fish_data;

DROP INDEX IF EXISTS idx_fishing_tournament_entries_fish_instance;
DELETE FROM fishing_tournament_entries WHERE source_kind = 'cultured';
ALTER TABLE fishing_tournament_entries
  DROP CONSTRAINT IF EXISTS chk_fishing_tournament_entries_source_kind,
  DROP COLUMN IF EXISTS source_kind,
  DROP COLUMN IF EXISTS fish_instance_id,
  ALTER COLUMN catch_id SET NOT NULL;

DROP INDEX IF EXISTS idx_player_item_instances_user_pond_fish;
DROP INDEX IF EXISTS idx_fishing_pond_events_user_created;
DROP TABLE IF EXISTS fishing_pond_events;
DROP TABLE IF EXISTS fishing_pond_pairs;
DROP TABLE IF EXISTS fishing_ponds;
