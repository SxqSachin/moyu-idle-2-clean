SET lock_timeout = '5s';

DROP TABLE IF EXISTS fishing_npc_sales;
DROP TABLE IF EXISTS fishing_npc_supply;
DROP TABLE IF EXISTS fishing_token_redemptions;
DROP TABLE IF EXISTS fishing_player_wallets;
DROP TABLE IF EXISTS fishing_tournament_rewards;
DROP TABLE IF EXISTS fishing_tournament_entries;
DROP TABLE IF EXISTS fishing_tournaments;
DROP TABLE IF EXISTS fishing_records;
DROP TABLE IF EXISTS fishing_species_knowledge;
DROP TABLE IF EXISTS fishing_catches;
DROP TABLE IF EXISTS fishing_trips;
DROP TABLE IF EXISTS fishing_spot_pressure;
DROP TABLE IF EXISTS fishing_loadouts;

DROP INDEX IF EXISTS idx_player_item_instances_user_fish;
ALTER TABLE player_item_instances
  DROP CONSTRAINT IF EXISTS chk_player_item_instances_fish_shape;
ALTER TABLE player_item_instances
  DROP CONSTRAINT IF EXISTS chk_player_item_instances_fish_quantity;
ALTER TABLE player_item_instances
  DROP COLUMN IF EXISTS fish_data;
