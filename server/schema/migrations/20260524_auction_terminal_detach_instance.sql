-- Data normalization before adding strict lifecycle constraint.
-- 1) Normalize unexpected listing_kind values from legacy rows.
UPDATE market_auctions
SET listing_kind = CASE
    WHEN instance_id IS NULL THEN 'stack'
    ELSE 'instance'
  END,
  updated_at = now()
WHERE listing_kind NOT IN ('instance', 'stack');

-- 2) Resolve contradictory shape: stack rows should not carry instance_id;
--    rows carrying instance_id are treated as instance auctions.
UPDATE market_auctions
SET listing_kind = 'instance',
    quantity = 1,
    updated_at = now()
WHERE listing_kind = 'stack'
  AND instance_id IS NOT NULL;

-- 3) Resolve contradictory shape: instance rows without instance_id.
--    Active rows are invalid in runtime, so cancel them to prevent ghost listings.
UPDATE market_auctions
SET status = 'cancelled',
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status = 'active'
  AND instance_id IS NULL;

-- 4) Ensure terminal instance auctions detach instance ownership linkage.
UPDATE market_auctions
SET instance_id = NULL,
    quantity = 1,
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status <> 'active';

-- 5) Ensure active instance auctions have single-item semantics.
UPDATE market_auctions
SET quantity = 1,
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status = 'active'
  AND quantity <> 1;

ALTER TABLE market_auctions
  DROP CONSTRAINT IF EXISTS chk_market_auction_listing_kind;

ALTER TABLE market_auctions
  ADD CONSTRAINT chk_market_auction_listing_kind CHECK (
    (
      listing_kind = 'instance'
      AND quantity = 1
      AND (
        (status = 'active' AND instance_id IS NOT NULL) OR
        (status <> 'active' AND instance_id IS NULL)
      )
    ) OR
    (listing_kind = 'stack' AND instance_id IS NULL)
  );
