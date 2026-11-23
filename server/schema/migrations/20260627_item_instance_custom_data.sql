ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS custom_data JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE player_item_instances
  DROP COLUMN IF EXISTS custom_name;

ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_custom_data JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE market_auctions
  DROP COLUMN IF EXISTS instance_custom_name;
