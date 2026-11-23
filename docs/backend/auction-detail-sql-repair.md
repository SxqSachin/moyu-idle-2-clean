# 拍卖详情报错 SQL 修复

线上报错点来自 `market_auctions` 记录与 `chk_market_auction_listing_kind` 约束不一致。下面这段 SQL 可以直接在数据库里执行，用于归一化历史脏数据并重建约束。

```sql
BEGIN;

-- 可选：先看线上当前约束定义
-- SELECT conname, pg_get_constraintdef(oid)
-- FROM pg_constraint
-- WHERE conrelid = 'market_auctions'::regclass
--   AND conname = 'chk_market_auction_listing_kind';

-- 0) 先放开旧约束（避免旧语义阻断修复）
ALTER TABLE market_auctions
  DROP CONSTRAINT IF EXISTS chk_market_auction_listing_kind;

-- 1) 先做大小写/空白归一化
UPDATE market_auctions
SET listing_kind = lower(trim(listing_kind)),
    updated_at = now()
WHERE listing_kind <> lower(trim(listing_kind));

-- 2) 归一化非法 listing_kind
UPDATE market_auctions
SET listing_kind = CASE
    WHEN instance_id IS NULL THEN 'stack'
    ELSE 'instance'
  END,
  updated_at = now()
WHERE listing_kind NOT IN ('instance', 'stack');

-- 3) 任何带 instance_id 的 stack 记录，按 instance 处理
UPDATE market_auctions
SET listing_kind = 'instance',
    quantity = 1,
    updated_at = now()
WHERE listing_kind = 'stack'
  AND instance_id IS NOT NULL;

-- 4) instance 但缺少 instance_id 的 active 记录无法继续保留
UPDATE market_auctions
SET status = 'cancelled',
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status = 'active'
  AND instance_id IS NULL;

-- 5) instance 的终态必须脱离 instance_id，且数量固定为 1
UPDATE market_auctions
SET instance_id = NULL,
    quantity = 1,
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status <> 'active';

-- 6) active 的 instance 也必须保持单件语义
UPDATE market_auctions
SET quantity = 1,
    updated_at = now()
WHERE listing_kind = 'instance'
  AND status = 'active'
  AND quantity <> 1;

-- 7) 加约束前做一次硬校验，发现残留脏数据就直接失败回滚
DO $$
DECLARE
  bad_count BIGINT;
BEGIN
  SELECT COUNT(1) INTO bad_count
  FROM market_auctions
  WHERE
    listing_kind NOT IN ('instance', 'stack')
    OR (listing_kind = 'stack' AND instance_id IS NOT NULL)
    OR (
      listing_kind = 'instance'
      AND (
        quantity <> 1
        OR (status = 'active' AND instance_id IS NULL)
        OR (status <> 'active' AND instance_id IS NOT NULL)
      )
    );

  IF bad_count > 0 THEN
    RAISE EXCEPTION 'market_auctions still has % invalid rows after normalization', bad_count;
  END IF;
END $$;

-- 8) 重新加回“新语义”检查约束
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
```

验证：

```sql
SELECT id, listing_kind, status, quantity, instance_id
FROM market_auctions
WHERE
  listing_kind NOT IN ('instance', 'stack')
  OR (listing_kind = 'stack' AND instance_id IS NOT NULL)
  OR (
    listing_kind = 'instance'
    AND (
      quantity <> 1
      OR (status = 'active' AND instance_id IS NULL)
      OR (status <> 'active' AND instance_id IS NOT NULL)
    )
  );
```

预检（只读，不改数据）：

```sql
SELECT
  id,
  seller_id,
  listing_kind,
  status,
  quantity,
  instance_id,
  CASE
    WHEN listing_kind NOT IN ('instance', 'stack') THEN 'invalid listing_kind'
    WHEN listing_kind = 'stack' AND instance_id IS NOT NULL THEN 'stack has instance_id'
    WHEN listing_kind = 'instance' AND quantity <> 1 THEN 'instance quantity must be 1'
    WHEN listing_kind = 'instance' AND status = 'active' AND instance_id IS NULL THEN 'active instance missing instance_id'
    WHEN listing_kind = 'instance' AND status <> 'active' AND instance_id IS NOT NULL THEN 'terminal instance should detach instance_id'
    ELSE 'ok'
  END AS issue
FROM market_auctions
WHERE
  listing_kind NOT IN ('instance', 'stack')
  OR (listing_kind = 'stack' AND instance_id IS NOT NULL)
  OR (
    listing_kind = 'instance'
    AND (
      quantity <> 1
      OR (status = 'active' AND instance_id IS NULL)
      OR (status <> 'active' AND instance_id IS NOT NULL)
    )
  )
ORDER BY id ASC;
```
