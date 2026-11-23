WITH disabled_items(item_id, max_durability) AS (
  VALUES
    ('gold_coin', 100),
    ('enhance_drop', 100),
    ('reset_scroll', 100),
    ('base2_mistwarden_charm', 170),
    ('base2_dragon_scale_bracer', 190),
    ('base4_boss_stalker_dagger', 220),
    ('base4_night_fan_blades', 205),
    ('base4_shadowcleaver_dadao', 245),
    ('base4_umbral_focus', 180),
    ('base4_enhancer_shadow_lens', 170)
),
listing_refunds AS (
  SELECT l.seller_id, l.item_id, d.max_durability, SUM(l.quantity) AS quantity
  FROM market_listings l
  JOIN disabled_items d ON d.item_id = l.item_id
  WHERE l.status = 'active'
    AND l.listing_kind = 'stack'
    AND l.quantity > 0
  GROUP BY l.seller_id, l.item_id, d.max_durability
)
INSERT INTO player_item_instances(
  user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
  durability, max_durability, quantity, state, created_at, updated_at
)
SELECT seller_id, item_id, 0, 0, '{}'::jsonb, '[]'::jsonb,
       max_durability, max_durability, quantity, 'inventory', now(), now()
FROM listing_refunds;

WITH disabled_items(item_id) AS (
  VALUES
    ('gold_coin'),
    ('enhance_drop'),
    ('reset_scroll'),
    ('base2_mistwarden_charm'),
    ('base2_dragon_scale_bracer'),
    ('base4_boss_stalker_dagger'),
    ('base4_night_fan_blades'),
    ('base4_shadowcleaver_dadao'),
    ('base4_umbral_focus'),
    ('base4_enhancer_shadow_lens')
)
UPDATE market_listings l
SET status = 'cancelled'
FROM disabled_items d
WHERE l.item_id = d.item_id
  AND l.status = 'active';

WITH disabled_items(item_id) AS (
  VALUES
    ('gold_coin'),
    ('enhance_drop'),
    ('reset_scroll'),
    ('base2_mistwarden_charm'),
    ('base2_dragon_scale_bracer'),
    ('base4_boss_stalker_dagger'),
    ('base4_night_fan_blades'),
    ('base4_shadowcleaver_dadao'),
    ('base4_umbral_focus'),
    ('base4_enhancer_shadow_lens')
),
buy_order_refunds AS (
  SELECT o.buyer_id, SUM(o.price * o.remaining_quantity) AS quantity
  FROM market_buy_orders o
  JOIN disabled_items d ON d.item_id = o.item_id
  WHERE o.status = 'active'
    AND o.remaining_quantity > 0
  GROUP BY o.buyer_id
)
INSERT INTO player_item_instances(
  user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
  durability, max_durability, quantity, state, created_at, updated_at
)
SELECT buyer_id, 'gold_coin', 0, 0, '{}'::jsonb, '[]'::jsonb,
       100, 100, quantity, 'inventory', now(), now()
FROM buy_order_refunds
WHERE quantity > 0;

WITH disabled_items(item_id) AS (
  VALUES
    ('gold_coin'),
    ('enhance_drop'),
    ('reset_scroll'),
    ('base2_mistwarden_charm'),
    ('base2_dragon_scale_bracer'),
    ('base4_boss_stalker_dagger'),
    ('base4_night_fan_blades'),
    ('base4_shadowcleaver_dadao'),
    ('base4_umbral_focus'),
    ('base4_enhancer_shadow_lens')
)
UPDATE market_buy_orders o
SET remaining_quantity = 0,
    status = 'cancelled',
    updated_at = now()
FROM disabled_items d
WHERE o.item_id = d.item_id
  AND o.status = 'active';
