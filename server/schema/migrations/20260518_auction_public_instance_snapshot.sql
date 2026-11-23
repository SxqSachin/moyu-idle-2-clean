ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_enhance_level BIGINT,
  ADD COLUMN IF NOT EXISTS instance_base_snapshot JSONB,
  ADD COLUMN IF NOT EXISTS instance_affixes JSONB,
  ADD COLUMN IF NOT EXISTS instance_durability BIGINT,
  ADD COLUMN IF NOT EXISTS instance_max_durability BIGINT;
