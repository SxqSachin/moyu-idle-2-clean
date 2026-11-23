-- 传闻委托完成记录
CREATE TABLE player_rumor_completion (
  user_id        BIGINT NOT NULL,
  commission_id  TEXT NOT NULL,
  completed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, commission_id)
);

-- 为 player_cat_village_tasks 添加传闻限制字段
ALTER TABLE player_cat_village_tasks
  ADD COLUMN IF NOT EXISTS not_refreshable BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS not_deletable   BOOLEAN NOT NULL DEFAULT FALSE;

-- 传闻委托保底计数（仅统计自动批量生成的 slot，不含手动刷新）
CREATE TABLE player_rumor_pity (
  user_id        BIGINT NOT NULL,
  commission_id  TEXT NOT NULL,
  counter        BIGINT NOT NULL DEFAULT 0,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, commission_id)
);

-- 收藏品持有记录（效果从 DSL collectibles.json 中读取，无需单独存储 mod 值）
CREATE TABLE player_collectibles (
  user_id        BIGINT NOT NULL,
  collectible_id TEXT NOT NULL,
  obtained_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, collectible_id)
);
