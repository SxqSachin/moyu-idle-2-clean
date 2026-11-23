CREATE TABLE IF NOT EXISTS market_instance_listings (
  id BIGSERIAL PRIMARY KEY,
  seller_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  item_id TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  revision TEXT NOT NULL,
  catalog_key TEXT NOT NULL,
  group_key TEXT NOT NULL,
  descriptor JSONB NOT NULL,
  fields JSONB NOT NULL,
  price BIGINT NOT NULL CHECK (price > 0 AND price <= 1000000000000),
  quantity INT NOT NULL CHECK (quantity BETWEEN 0 AND 100),
  initial_quantity INT NOT NULL CHECK (initial_quantity BETWEEN 1 AND 100),
  status TEXT NOT NULL CHECK (status IN ('active', 'sold', 'cancelled')),
  request_key TEXT NOT NULL CHECK (length(request_key) BETWEEN 1 AND 128),
  request_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (quantity <= initial_quantity),
  CHECK ((status = 'active') = (quantity > 0)),
  UNIQUE(seller_id, request_key)
);
CREATE INDEX IF NOT EXISTS idx_market_instance_group
  ON market_instance_listings(item_id, revision, catalog_key, group_key, price, id) WHERE status='active';
CREATE INDEX IF NOT EXISTS idx_market_instance_seller
  ON market_instance_listings(seller_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_market_instance_offers
  ON market_instance_listings(item_id, group_key, price, id) WHERE status='active';

CREATE TABLE IF NOT EXISTS market_instance_orders (
  id BIGSERIAL PRIMARY KEY,
  listing_id BIGINT NOT NULL REFERENCES market_instance_listings(id) ON DELETE RESTRICT,
  buyer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  seller_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  item_id TEXT NOT NULL,
  quantity INT NOT NULL CHECK (quantity BETWEEN 1 AND 100),
  unit_price BIGINT NOT NULL CHECK (unit_price > 0),
  total_price BIGINT NOT NULL CHECK (total_price > 0),
  fee_rate_bps INT NOT NULL CHECK (fee_rate_bps BETWEEN 0 AND 5000),
  fee_amount BIGINT NOT NULL CHECK (fee_amount >= 0),
  seller_amount BIGINT NOT NULL CHECK (seller_amount >= 0),
  fields JSONB NOT NULL,
  group_key TEXT NOT NULL,
  request_key TEXT NOT NULL CHECK (length(request_key) BETWEEN 1 AND 128),
  request_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (buyer_id <> seller_id),
  CHECK (total_price = unit_price * quantity),
  CHECK (total_price = fee_amount + seller_amount),
  UNIQUE(buyer_id, request_key)
);
CREATE INDEX IF NOT EXISTS idx_market_instance_orders_buyer ON market_instance_orders(buyer_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_market_instance_orders_seller ON market_instance_orders(seller_id, id DESC);

CREATE TABLE IF NOT EXISTS market_instance_listing_items (
  id BIGSERIAL PRIMARY KEY,
  listing_id BIGINT NOT NULL REFERENCES market_instance_listings(id) ON DELETE RESTRICT,
  original_instance_id BIGINT NOT NULL,
  escrow_instance_id BIGINT REFERENCES player_item_instances(instance_id) ON DELETE RESTRICT,
  snapshot JSONB NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('listed', 'sold', 'returned')),
  order_id BIGINT REFERENCES market_instance_orders(id) ON DELETE RESTRICT,
  UNIQUE(listing_id, original_instance_id),
  CHECK ((state = 'listed') = (escrow_instance_id IS NOT NULL)),
  CHECK ((state = 'sold') = (order_id IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_market_instance_escrow ON market_instance_listing_items(escrow_instance_id) WHERE escrow_instance_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_market_instance_items_listing ON market_instance_listing_items(listing_id, state, id);
CREATE INDEX IF NOT EXISTS idx_market_instance_items_order ON market_instance_listing_items(order_id) WHERE order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS market_instance_ledger (
  order_id BIGINT PRIMARY KEY REFERENCES market_instance_orders(id) ON DELETE RESTRICT,
  from_user BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  to_user BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  amount BIGINT NOT NULL CHECK(amount >= 0),
  fee_amount BIGINT NOT NULL CHECK(fee_amount >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION check_market_instance_listing_balance() RETURNS TRIGGER AS $$
DECLARE listing BIGINT; expected INT; actual BIGINT;
BEGIN
  IF TG_TABLE_NAME = 'market_instance_listings' THEN listing := NEW.id;
  ELSE listing := COALESCE(NEW.listing_id, OLD.listing_id); END IF;
  SELECT quantity INTO expected FROM market_instance_listings WHERE id=listing;
  SELECT count(*) INTO actual FROM market_instance_listing_items WHERE listing_id=listing AND state='listed';
  IF expected IS DISTINCT FROM actual THEN RAISE EXCEPTION 'market instance escrow imbalance' USING ERRCODE='23514'; END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS market_instance_listing_balance ON market_instance_listings;
CREATE CONSTRAINT TRIGGER market_instance_listing_balance AFTER INSERT OR UPDATE ON market_instance_listings
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_market_instance_listing_balance();
DROP TRIGGER IF EXISTS market_instance_items_balance ON market_instance_listing_items;
CREATE CONSTRAINT TRIGGER market_instance_items_balance AFTER INSERT OR UPDATE OR DELETE ON market_instance_listing_items
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_market_instance_listing_balance();
