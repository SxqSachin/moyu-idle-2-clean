-- Data normalization before enforcing the auction listing constraint.
BEGIN;

-- 1) Normalize unexpected listing_kind values from legacy rows.
UPDATE market_auctions
SET listing_kind = CASE
    WHEN instance_id IS NULL THEN 'stack'
    ELSE 'instance'
  END,
  updated_at = now()
WHERE listing_kind NOT IN ('instance', 'stack');

-- 2) Rows carrying instance_id are treated as instance auctions.
UPDATE market_auctions
SET listing_kind = 'instance',
    quantity = 1,
    updated_at = now()
WHERE listing_kind = 'stack'
  AND instance_id IS NOT NULL;

-- 3) Active instance rows without instance_id are invalid; cancel them.
UPDATE market_auctions
SET status = 'cancelled',
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status = 'active'
  AND instance_id IS NULL;

-- 4) Terminal instance auctions must detach instance ownership linkage.
UPDATE market_auctions
SET instance_id = NULL,
    quantity = 1,
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status <> 'active';

-- 5) Active instance auctions must keep single-item semantics.
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

COMMIT;
