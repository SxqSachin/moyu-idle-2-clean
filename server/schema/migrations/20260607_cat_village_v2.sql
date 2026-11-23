-- Migrate costs from single-item to JSONB map
ALTER TABLE player_cat_village_tasks ADD COLUMN IF NOT EXISTS costs_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Migrate any remaining open tasks (snapshot existing single-item cost)
UPDATE player_cat_village_tasks
SET costs_jsonb = jsonb_build_object(required_item_id, required_quantity)
WHERE status = 'open' AND required_item_id != '' AND required_quantity > 0;

-- Drop old single-item columns
ALTER TABLE player_cat_village_tasks DROP COLUMN IF EXISTS required_item_id;
ALTER TABLE player_cat_village_tasks DROP COLUMN IF EXISTS required_quantity;
