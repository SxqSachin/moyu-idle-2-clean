ALTER TABLE market_listings
  DROP CONSTRAINT IF EXISTS market_listings_quantity_check;

ALTER TABLE market_listings
  ADD CONSTRAINT market_listings_quantity_check CHECK (quantity >= 0);
