UPDATE player_item_instances i
SET state = 'inventory',
    updated_at = now()
FROM market_listings l
WHERE l.listing_kind = 'instance'
  AND l.status = 'active'
  AND l.instance_id = i.instance_id
  AND i.user_id = l.seller_id
  AND i.state = 'market';

UPDATE market_listings
SET status = 'cancelled'
WHERE listing_kind = 'instance'
  AND status = 'active';
