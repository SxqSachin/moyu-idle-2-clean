-- Baseline schema init for local/dev
-- Source of truth: server/schema/init.sql

SELECT 'CREATE DATABASE game OWNER game'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'game')\gexec
\connect game

-- === Identity / account ===
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL DEFAULT '',
  email_verified BOOLEAN NOT NULL DEFAULT false,
  email_verified_at TIMESTAMPTZ,
  password_hash TEXT NOT NULL,
  ban_reason TEXT NOT NULL DEFAULT '',
  ban_until TIMESTAMPTZ NULL,
  ban_is_permanent BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS ban_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS ban_until TIMESTAMPTZ NULL;
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS ban_is_permanent BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_non_empty
  ON users ((lower(email)))
  WHERE btrim(email) <> '';

CREATE TABLE IF NOT EXISTS player_username_changes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  old_username TEXT NOT NULL CHECK(length(btrim(old_username)) > 0),
  new_username TEXT NOT NULL CHECK(length(btrim(new_username)) > 0),
  changed_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  change_source TEXT NOT NULL DEFAULT 'self_service' CHECK(length(btrim(change_source)) > 0),
  cost_item_id TEXT NOT NULL CHECK(length(btrim(cost_item_id)) > 0),
  cost_item_count BIGINT NOT NULL CHECK(cost_item_count > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  CHECK(old_username <> new_username)
);
CREATE INDEX IF NOT EXISTS idx_player_username_changes_user_created
  ON player_username_changes(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user
  ON password_reset_tokens(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS user_settings (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  language TEXT NOT NULL DEFAULT 'zh-CN' CHECK (language IN ('zh-CN', 'zh-TW', 'en-US', 'ja', 'ko', 'ru')),
  theme TEXT NOT NULL DEFAULT 'light' CHECK (theme IN ('light', 'dark')),
  extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_policy_acceptances (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  policy_key TEXT NOT NULL,
  policy_version TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  accepted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, policy_key, policy_version)
);
CREATE INDEX IF NOT EXISTS idx_user_policy_acceptances_user_policy
  ON user_policy_acceptances(user_id, policy_key, accepted_at DESC);

CREATE TABLE IF NOT EXISTS player_presence (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'offline' CHECK(status IN ('online', 'offline')),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  session_version BIGINT NOT NULL DEFAULT 1 CHECK(session_version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_presence_online ON player_presence(user_id) WHERE status = 'online';

-- === Risk / anti-abuse ===
CREATE TABLE IF NOT EXISTS risk_account_signals (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ip TEXT NOT NULL DEFAULT '',
  ip_prefix TEXT NOT NULL DEFAULT '',
  asn TEXT NOT NULL DEFAULT '',
  country_code TEXT NOT NULL DEFAULT '',
  fingerprint_id TEXT NOT NULL DEFAULT '',
  fingerprint_version TEXT NOT NULL DEFAULT '',
  device_family_hash TEXT NOT NULL DEFAULT '',
  browser_family_hash TEXT NOT NULL DEFAULT '',
  runtime_family_hash TEXT NOT NULL DEFAULT '',
  cookie_id TEXT NOT NULL DEFAULT '',
  local_device_id TEXT NOT NULL DEFAULT '',
  browser_install_id TEXT NOT NULL DEFAULT '',
  session_chain_id TEXT NOT NULL DEFAULT '',
  ua_hash TEXT NOT NULL DEFAULT '',
  accept_language TEXT NOT NULL DEFAULT '',
  timezone TEXT NOT NULL DEFAULT '',
  screen_hash TEXT NOT NULL DEFAULT '',
  canvas_hash TEXT NOT NULL DEFAULT '',
  webgl_hash TEXT NOT NULL DEFAULT '',
  audio_hash TEXT NOT NULL DEFAULT '',
  cpu_hint TEXT NOT NULL DEFAULT '',
  memory_hint TEXT NOT NULL DEFAULT '',
  platform_hint TEXT NOT NULL DEFAULT '',
  captcha_score DOUBLE PRECISION NOT NULL DEFAULT 0,
  extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_risk_account_signals_user_time
  ON risk_account_signals(user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_account_signals_local_device
  ON risk_account_signals(local_device_id, occurred_at DESC)
  WHERE btrim(local_device_id) <> '';
CREATE INDEX IF NOT EXISTS idx_risk_account_signals_fingerprint
  ON risk_account_signals(fingerprint_id, occurred_at DESC)
  WHERE btrim(fingerprint_id) <> '';
CREATE INDEX IF NOT EXISTS idx_risk_account_signals_device_family
  ON risk_account_signals(device_family_hash, occurred_at DESC)
  WHERE btrim(device_family_hash) <> '';
CREATE INDEX IF NOT EXISTS idx_risk_account_signals_ip_prefix
  ON risk_account_signals(ip_prefix, occurred_at DESC)
  WHERE btrim(ip_prefix) <> '';

CREATE TABLE IF NOT EXISTS risk_password_evidence (
  password_hash_digest TEXT NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (password_hash_digest, user_id)
);
CREATE INDEX IF NOT EXISTS idx_risk_password_evidence_user
  ON risk_password_evidence(user_id, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS risk_account_transfers (
  id BIGSERIAL PRIMARY KEY,
  from_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  to_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel TEXT NOT NULL,
  reference_id BIGINT NOT NULL DEFAULT 0,
  item_id TEXT NOT NULL DEFAULT '',
  quantity BIGINT NOT NULL DEFAULT 0,
  unit_price BIGINT NOT NULL DEFAULT 0,
  total_price BIGINT NOT NULL DEFAULT 0,
  fee_amount BIGINT NOT NULL DEFAULT 0,
  net_amount BIGINT NOT NULL DEFAULT 0,
  price_anchor BIGINT NOT NULL DEFAULT 0,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_from_time
  ON risk_account_transfers(from_user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_to_time
  ON risk_account_transfers(to_user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_pair_time
  ON risk_account_transfers(from_user_id, to_user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_account_transfers_auction_time
  ON risk_account_transfers(occurred_at DESC)
  WHERE channel LIKE 'auction_%';

CREATE TABLE IF NOT EXISTS risk_account_evidence (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  peer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  evidence_type TEXT NOT NULL,
  strength TEXT NOT NULL DEFAULT 'weak',
  score INT NOT NULL DEFAULT 0,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  evidence_payload_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (user_id, peer_user_id, evidence_type)
);
CREATE INDEX IF NOT EXISTS idx_risk_account_evidence_user
  ON risk_account_evidence(user_id, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS risk_account_links (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  peer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  total_score INT NOT NULL DEFAULT 0,
  risk_level TEXT NOT NULL DEFAULT 'observe',
  evidence_summary_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, peer_user_id)
);
CREATE INDEX IF NOT EXISTS idx_risk_account_links_user_score
  ON risk_account_links(user_id, total_score DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS risk_clusters (
  cluster_id BIGSERIAL PRIMARY KEY,
  risk_score INT NOT NULL DEFAULT 0,
  risk_level TEXT NOT NULL DEFAULT 'observe',
  status TEXT NOT NULL DEFAULT 'active',
  member_count INT NOT NULL DEFAULT 0,
  primary_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS risk_cluster_members (
  cluster_id BIGINT NOT NULL REFERENCES risk_clusters(cluster_id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  member_score INT NOT NULL DEFAULT 0,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  reason_summary TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (cluster_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_risk_cluster_members_user
  ON risk_cluster_members(user_id, cluster_id);

CREATE TABLE IF NOT EXISTS risk_capability_blocks (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  capability TEXT NOT NULL,
  block_level TEXT NOT NULL DEFAULT 'restricted',
  reason_code TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  cluster_id BIGINT REFERENCES risk_clusters(cluster_id) ON DELETE SET NULL,
  expires_at TIMESTAMPTZ NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, capability)
);
CREATE INDEX IF NOT EXISTS idx_risk_capability_blocks_user
  ON risk_capability_blocks(user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS risk_review_cases (
  id BIGSERIAL PRIMARY KEY,
  cluster_id BIGINT REFERENCES risk_clusters(cluster_id) ON DELETE CASCADE,
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'open',
  decision TEXT NOT NULL DEFAULT '',
  operator_name TEXT NOT NULL DEFAULT '',
  operator_note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_risk_review_cases_cluster
  ON risk_review_cases(cluster_id, status, updated_at DESC);

-- === User IP tracking ===
CREATE TABLE IF NOT EXISTS user_known_ips (
  user_id      BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  ip           TEXT         NOT NULL,
  use_count    INT          NOT NULL DEFAULT 1,
  last_seen_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_user_known_ips_user ON user_known_ips (user_id);

CREATE TABLE IF NOT EXISTS user_trusted_devices (
  id           BIGSERIAL    PRIMARY KEY,
  user_id      BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_hash  TEXT         NOT NULL,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  expires_at   TIMESTAMPTZ  NOT NULL,
  UNIQUE (user_id, device_hash)
);
CREATE INDEX IF NOT EXISTS idx_user_trusted_devices_user ON user_trusted_devices(user_id);

-- === Inventory / equipment ===
CREATE TABLE IF NOT EXISTS player_item_instances (
  instance_id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  template_item_id TEXT NOT NULL,
  custom_data JSONB NOT NULL DEFAULT '{}'::jsonb,
  enhance_level INT NOT NULL DEFAULT 0,
  roll_seed BIGINT NOT NULL DEFAULT 0,
  base_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  affixes JSONB NOT NULL DEFAULT '[]'::jsonb,
  milestone_affix_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
  durability INT NOT NULL DEFAULT 100,
  max_durability INT NOT NULL DEFAULT 100,
  quantity BIGINT NOT NULL DEFAULT 1 CHECK(quantity > 0),
  state TEXT NOT NULL DEFAULT 'inventory',
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_item_instances_user_state ON player_item_instances(user_id, state);
CREATE INDEX IF NOT EXISTS idx_player_item_instances_user_template_state_enhance
  ON player_item_instances(user_id, template_item_id, state, enhance_level);
CREATE INDEX IF NOT EXISTS idx_player_item_instances_user_instance_desc
  ON player_item_instances(user_id, instance_id DESC);
CREATE INDEX IF NOT EXISTS idx_player_item_instances_user_active_instance_desc
  ON player_item_instances(user_id, instance_id DESC)
  WHERE state NOT IN ('consumed', 'destroyed');
ALTER TABLE player_item_instances
  ALTER COLUMN affixes SET DEFAULT '[]'::jsonb;
ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS custom_data JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE player_item_instances
  DROP COLUMN IF EXISTS custom_name;
ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS milestone_affix_claims JSONB NOT NULL DEFAULT '{}'::jsonb;
UPDATE player_item_instances
SET affixes = '[]'::jsonb
WHERE affixes = '{}'::jsonb;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_player_item_instances_qty_affixes_empty'
  ) THEN
    ALTER TABLE player_item_instances
      ADD CONSTRAINT chk_player_item_instances_qty_affixes_empty
      CHECK (
        quantity <= 1
        OR COALESCE(affixes, '[]'::jsonb) IN ('[]'::jsonb, '{}'::jsonb)
      ) NOT VALID;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS alchemy_discoveries (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK(kind IN ('group', 'trait', 'combo')),
  id TEXT NOT NULL,
  discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, kind, id)
);
CREATE INDEX IF NOT EXISTS idx_alchemy_discoveries_user
  ON alchemy_discoveries(user_id, discovered_at DESC);

CREATE TABLE IF NOT EXISTS player_equip_slots (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  equip_type TEXT NOT NULL CHECK(equip_type IN ('battle', 'work')),
  slot TEXT NOT NULL,
  instance_id BIGINT NOT NULL REFERENCES player_item_instances(instance_id) ON DELETE CASCADE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, equip_type, slot)
);
CREATE INDEX IF NOT EXISTS idx_player_equip_slots_user_instance
  ON player_equip_slots(user_id, instance_id);
CREATE INDEX IF NOT EXISTS idx_player_equip_slots_user_type_instance
  ON player_equip_slots(user_id, equip_type, instance_id);

CREATE TABLE IF NOT EXISTS player_loadouts (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  loadout_id TEXT NOT NULL,
  type TEXT NOT NULL CHECK(type IN ('battle', 'work')),
  name TEXT NOT NULL DEFAULT '',
  payload_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, loadout_id)
);
CREATE INDEX IF NOT EXISTS idx_player_loadouts_user_type_updated
  ON player_loadouts(user_id, type, updated_at DESC, loadout_id);

CREATE TABLE IF NOT EXISTS player_work_loadout_bindings (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action_main_type TEXT NOT NULL,
  loadout_id TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, action_main_type)
);
CREATE INDEX IF NOT EXISTS idx_player_work_loadout_bindings_user_loadout
  ON player_work_loadout_bindings(user_id, loadout_id);

CREATE TABLE IF NOT EXISTS player_derived_stats_cache (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  stats_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 动态属性主表（按属性 key 存储）
CREATE TABLE IF NOT EXISTS player_attributes (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  attr_key TEXT NOT NULL,
  base_level BIGINT NOT NULL DEFAULT 1 CHECK(base_level > 0),
  skill_point_level BIGINT NOT NULL DEFAULT 0 CHECK(skill_point_level >= 0),
  attr_exp DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK(attr_exp >= 0),
  source_mods_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  final_value DOUBLE PRECISION NOT NULL DEFAULT 0,
  lock_version BIGINT NOT NULL DEFAULT 1 CHECK(lock_version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, attr_key)
);
CREATE INDEX IF NOT EXISTS idx_player_attributes_user ON player_attributes(user_id);

CREATE TABLE IF NOT EXISTS player_battle_point_state (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  total_points BIGINT NOT NULL DEFAULT 0 CHECK(total_points >= 0),
  spent_points BIGINT NOT NULL DEFAULT 0 CHECK(spent_points >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_specialization_states (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tree_id TEXT NOT NULL,
  ruleset_version TEXT NOT NULL CHECK(length(btrim(ruleset_version)) > 0),
  revision BIGINT NOT NULL DEFAULT 0 CHECK(revision >= 0),
  formula_points INT NOT NULL DEFAULT 0 CHECK(formula_points >= 0),
  bonus_points INT NOT NULL DEFAULT 0 CHECK(bonus_points >= 0),
  spent_points INT NOT NULL DEFAULT 0 CHECK(spent_points >= 0),
  last_ruleset_reset_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, tree_id)
);

ALTER TABLE player_specialization_states
  ADD COLUMN IF NOT EXISTS last_ruleset_reset_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS player_specialization_allocations (
  user_id BIGINT NOT NULL,
  tree_id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  rank INT NOT NULL DEFAULT 1 CHECK(rank > 0),
  paid_cost INT NOT NULL CHECK(paid_cost >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, tree_id, node_id),
  FOREIGN KEY (user_id, tree_id)
    REFERENCES player_specialization_states(user_id, tree_id)
    ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS player_specialization_ledger (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tree_id TEXT NOT NULL,
  revision BIGINT NOT NULL CHECK(revision >= 0),
  operation TEXT NOT NULL CHECK(operation IN ('allocate', 'reset')),
  node_id TEXT NOT NULL DEFAULT '',
  paid_cost INT NOT NULL DEFAULT 0 CHECK(paid_cost >= 0),
  reset_item_id TEXT NOT NULL DEFAULT '',
  reset_item_count BIGINT NOT NULL DEFAULT 0 CHECK(reset_item_count >= 0),
  impact_digest TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  result_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, tree_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_player_specialization_ledger_user_tree
  ON player_specialization_ledger(user_id, tree_id, revision DESC);


-- === Market / economy ===
CREATE TABLE IF NOT EXISTS market_listings (
  id BIGSERIAL PRIMARY KEY,
  seller_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id TEXT NOT NULL,
  listing_kind TEXT NOT NULL DEFAULT 'stack',
  instance_id BIGINT REFERENCES player_item_instances(instance_id) ON DELETE RESTRICT,
  price BIGINT NOT NULL CHECK(price > 0),
  quantity BIGINT NOT NULL CHECK(quantity >= 0),
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_market_listings_status ON market_listings(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_market_active_instance_listing
  ON market_listings(instance_id)
  WHERE instance_id IS NOT NULL AND status = 'active';
CREATE INDEX IF NOT EXISTS idx_market_listings_item_active
  ON market_listings(item_id, price ASC, id ASC) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_market_listings_seller
  ON market_listings(seller_id, id DESC);

CREATE TABLE IF NOT EXISTS market_orders (
  id BIGSERIAL PRIMARY KEY,
  listing_id BIGINT NOT NULL REFERENCES market_listings(id) ON DELETE RESTRICT,
  buyer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  total_price BIGINT NOT NULL CHECK(total_price >= 0),
  fee_rate_bps INT NOT NULL DEFAULT 500 CHECK(fee_rate_bps >= 0),
  fee_amount BIGINT NOT NULL DEFAULT 0 CHECK(fee_amount >= 0),
  seller_amount BIGINT NOT NULL DEFAULT 0 CHECK(seller_amount >= 0),
  status TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_market_orders_buyer_idempotency
  ON market_orders(buyer_id, idempotency_key);

CREATE TABLE IF NOT EXISTS market_ledger (
  id BIGSERIAL PRIMARY KEY,
  order_id BIGINT NOT NULL REFERENCES market_orders(id) ON DELETE CASCADE,
  from_user BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  to_user BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount BIGINT NOT NULL CHECK(amount >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS market_fee_tiers (
  id BIGSERIAL PRIMARY KEY,
  scope TEXT NOT NULL DEFAULT 'default',
  min_total BIGINT NOT NULL DEFAULT 0 CHECK(min_total >= 0),
  max_total BIGINT CHECK(max_total IS NULL OR max_total >= min_total),
  fee_bps INT NOT NULL CHECK(fee_bps >= 0 AND fee_bps <= 5000),
  priority INT NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_market_fee_tiers_active ON market_fee_tiers(active, scope, priority);

INSERT INTO market_fee_tiers(scope, min_total, max_total, fee_bps, priority)
VALUES
  ('default', 0, 1000, 500, 100),
  ('default', 1001, 10000, 450, 100),
  ('default', 10001, NULL, 400, 100)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS shop_offers (
  id BIGSERIAL PRIMARY KEY,
  item_id TEXT NOT NULL UNIQUE,
  shop_buy_price BIGINT NOT NULL CHECK(shop_buy_price >= 0),
  shop_sell_price BIGINT NOT NULL CHECK(shop_sell_price >= 0),
  active BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- INSERT INTO shop_offers(item_id, shop_buy_price, shop_sell_price, active)
-- VALUES
--   ('wood_sword', 50, 120, TRUE)
-- ON CONFLICT (item_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS market_buy_orders (
  id BIGSERIAL PRIMARY KEY,
  buyer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id TEXT NOT NULL,
  price BIGINT NOT NULL CHECK(price > 0),
  quantity BIGINT NOT NULL CHECK(quantity > 0),
  remaining_quantity BIGINT NOT NULL CHECK(remaining_quantity >= 0),
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_market_buy_orders_status ON market_buy_orders(status);
CREATE INDEX IF NOT EXISTS idx_market_buy_orders_item ON market_buy_orders(item_id);

CREATE TABLE IF NOT EXISTS market_auctions (
  id BIGSERIAL PRIMARY KEY,
  seller_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  listing_kind TEXT NOT NULL DEFAULT 'instance',
  item_id TEXT NOT NULL,
  instance_id BIGINT REFERENCES player_item_instances(instance_id) ON DELETE RESTRICT,
  instance_custom_data JSONB NOT NULL DEFAULT '{}'::jsonb,
  instance_fish_data JSONB NOT NULL DEFAULT '{}'::jsonb,
  quantity BIGINT NOT NULL DEFAULT 1 CHECK(quantity > 0),
  instance_enhance_level BIGINT,
  instance_base_snapshot JSONB,
  instance_affixes JSONB,
  instance_durability BIGINT,
  instance_max_durability BIGINT,
  start_price BIGINT NOT NULL CHECK(start_price > 0),
  current_price BIGINT NOT NULL DEFAULT 0 CHECK(current_price >= 0),
  min_bid_increment BIGINT NOT NULL DEFAULT 1 CHECK(min_bid_increment > 0),
  current_bidder_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  buyout_price BIGINT CHECK(buyout_price IS NULL OR buyout_price > 0),
  fee_rate_bps BIGINT NOT NULL DEFAULT 0 CHECK(fee_rate_bps >= 0),
  fee_amount BIGINT NOT NULL DEFAULT 0 CHECK(fee_amount >= 0),
  seller_amount BIGINT NOT NULL DEFAULT 0 CHECK(seller_amount >= 0),
  anti_snipe_extend_count INT NOT NULL DEFAULT 0 CHECK(anti_snipe_extend_count >= 0),
  status TEXT NOT NULL DEFAULT 'active',
  ends_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_market_auction_listing_kind CHECK (
    (
      listing_kind = 'instance'
      AND quantity = 1
      AND (
        (status = 'active' AND instance_id IS NOT NULL) OR
        (status <> 'active' AND instance_id IS NULL)
      )
    ) OR
    (listing_kind = 'stack' AND instance_id IS NULL)
  )
);
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS listing_kind TEXT NOT NULL DEFAULT 'instance';
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS quantity BIGINT NOT NULL DEFAULT 1;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_custom_data JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_fish_data JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE market_auctions
  DROP COLUMN IF EXISTS instance_custom_name;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS fee_rate_bps BIGINT NOT NULL DEFAULT 0;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS fee_amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS seller_amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS anti_snipe_extend_count INT NOT NULL DEFAULT 0;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS min_bid_increment BIGINT NOT NULL DEFAULT 1;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_enhance_level BIGINT;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_base_snapshot JSONB;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_affixes JSONB;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_durability BIGINT;
ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_max_durability BIGINT;
ALTER TABLE market_auctions
  ALTER COLUMN instance_id DROP NOT NULL;
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
CREATE INDEX IF NOT EXISTS idx_market_auctions_status ON market_auctions(status);
CREATE INDEX IF NOT EXISTS idx_market_auctions_item ON market_auctions(item_id);
CREATE INDEX IF NOT EXISTS idx_market_auctions_ends_at ON market_auctions(ends_at);
CREATE INDEX IF NOT EXISTS idx_market_auctions_seller_status ON market_auctions(seller_id, status);
DROP INDEX IF EXISTS uq_market_active_instance_auction;
CREATE UNIQUE INDEX IF NOT EXISTS uq_market_active_instance_auction
  ON market_auctions(instance_id)
  WHERE status = 'active' AND instance_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS market_auction_bids (
  id BIGSERIAL PRIMARY KEY,
  auction_id BIGINT NOT NULL REFERENCES market_auctions(id) ON DELETE CASCADE,
  bidder_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  bid_amount BIGINT NOT NULL CHECK(bid_amount > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_market_auction_bids_auction ON market_auction_bids(auction_id, id DESC);

-- === Realtime messaging ===
CREATE TABLE IF NOT EXISTS chat_messages (
  id BIGSERIAL PRIMARY KEY,
  channel TEXT NOT NULL DEFAULT 'world',
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  username TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'world';
ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS username TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_chat_messages_channel_id_desc ON chat_messages(channel, id DESC);
ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS title_prefix_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS title_suffix_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS display_meta JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS attachment_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE chat_messages DROP COLUMN IF EXISTS title;

-- === Activity / tasks ===

CREATE TABLE IF NOT EXISTS daily_task_templates (
  id BIGSERIAL PRIMARY KEY,
  required_item_id TEXT NOT NULL,
  required_quantity BIGINT NOT NULL CHECK(required_quantity > 0),
  reward_items JSONB NOT NULL DEFAULT '{}'::jsonb,
  weight INT NOT NULL DEFAULT 1 CHECK(weight > 0),
  active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS player_daily_task_state (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  next_refresh_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_daily_tasks (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  template_id BIGINT NOT NULL REFERENCES daily_task_templates(id) ON DELETE RESTRICT,
  required_item_id TEXT NOT NULL,
  required_quantity BIGINT NOT NULL CHECK(required_quantity > 0),
  reward_items JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_player_daily_tasks_user_status ON player_daily_tasks(user_id, status, id DESC);

ALTER TABLE daily_task_templates
  ADD COLUMN IF NOT EXISTS reward_items JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE daily_task_templates DROP COLUMN IF EXISTS reward_gold;
ALTER TABLE daily_task_templates DROP COLUMN IF EXISTS reward_materials;

ALTER TABLE player_daily_tasks
  ADD COLUMN IF NOT EXISTS reward_items JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE player_daily_tasks DROP COLUMN IF EXISTS reward_gold;
ALTER TABLE player_daily_tasks DROP COLUMN IF EXISTS reward_materials;

CREATE TABLE IF NOT EXISTS player_rule_sets (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  rule_kind TEXT NOT NULL,
  slot_key TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  groups_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, rule_kind, slot_key)
);

CREATE TABLE IF NOT EXISTS player_skills (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  skill_id TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  cast_order INT NOT NULL DEFAULT 0 CHECK(cast_order >= 0),
  rule_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  rule_groups_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  level INT NOT NULL DEFAULT 1 CHECK(level >= 1),
  exp BIGINT NOT NULL DEFAULT 0 CHECK(exp >= 0),
  equipment_config_jsonb JSONB CONSTRAINT player_skills_equipment_config_shape CHECK (
    equipment_config_jsonb IS NULL OR (
      skill_id IN ('equipment_domination', 'equipment_martial_combo')
      AND jsonb_typeof(equipment_config_jsonb) = 'object'
      AND octet_length(equipment_config_jsonb::text) <= 16384
    )
  ),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, skill_id)
);
CREATE INDEX IF NOT EXISTS idx_player_skills_user_order
  ON player_skills(user_id, cast_order, skill_id);

CREATE TABLE IF NOT EXISTS player_custom_skills (
  id TEXT PRIMARY KEY CHECK(id LIKE 'custom:%'),
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_kind TEXT NOT NULL CHECK(source_kind IN ('blank', 'official')),
  source_skill_id TEXT NOT NULL DEFAULT '',
  source_definition_hash TEXT NOT NULL DEFAULT '',
  current_revision_id TEXT,
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, id)
);
CREATE INDEX IF NOT EXISTS idx_player_custom_skills_user_active
  ON player_custom_skills(user_id, archived_at, created_at);
CREATE INDEX IF NOT EXISTS idx_player_custom_skills_user_source
  ON player_custom_skills(user_id, source_skill_id)
  WHERE archived_at IS NULL AND source_skill_id <> '';

CREATE TABLE IF NOT EXISTS player_custom_skill_drafts (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  custom_skill_id TEXT REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  source_kind TEXT NOT NULL CHECK(source_kind IN ('blank', 'official')),
  source_skill_id TEXT NOT NULL DEFAULT '',
  source_skillbook_item_id TEXT NOT NULL DEFAULT '',
  source_skillbook_count INT NOT NULL DEFAULT 0 CHECK(source_skillbook_count >= 0),
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'completed')),
  source_definition_hash TEXT NOT NULL DEFAULT '',
  graph_jsonb JSONB NOT NULL,
  graph_version INT NOT NULL DEFAULT 1 CHECK(graph_version = 1),
  revision_no BIGINT NOT NULL DEFAULT 1 CHECK(revision_no > 0),
  pending_revision_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_drafts_user_updated
  ON player_custom_skill_drafts(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_drafts_user_active_updated
  ON player_custom_skill_drafts(user_id, updated_at DESC)
  WHERE status = 'active';

CREATE TABLE IF NOT EXISTS player_custom_skill_revisions (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  custom_skill_id TEXT NOT NULL REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  draft_id TEXT REFERENCES player_custom_skill_drafts(id) ON DELETE SET NULL,
  revision_no BIGINT NOT NULL CHECK(revision_no > 0),
  source_skill_id TEXT NOT NULL DEFAULT '',
  source_definition_hash TEXT NOT NULL DEFAULT '',
  graph_jsonb JSONB NOT NULL,
  compiled_skill_jsonb JSONB NOT NULL,
  recipe_jsonb JSONB NOT NULL,
  requested_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  consumed_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  refunded_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_skillbook_item_id TEXT NOT NULL DEFAULT '',
  source_skillbook_count INT NOT NULL DEFAULT 0,
  budget_jsonb JSONB NOT NULL,
  rolls_jsonb JSONB NOT NULL,
  failed BOOLEAN NOT NULL DEFAULT FALSE,
  result_status TEXT NOT NULL CHECK(result_status IN ('pending', 'accepted', 'discarded', 'superseded', 'failed')),
  compiler_version INT NOT NULL CHECK(compiler_version > 0),
  ruleset_version INT NOT NULL DEFAULT 1 CHECK(ruleset_version > 0),
  idempotency_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  UNIQUE(user_id, idempotency_key),
  UNIQUE(custom_skill_id, revision_no)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_one_pending
  ON player_custom_skill_revisions(draft_id)
  WHERE result_status = 'pending';
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_skill_created
  ON player_custom_skill_revisions(custom_skill_id, revision_no DESC);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_ruleset
  ON player_custom_skill_revisions(ruleset_version, result_status, user_id);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_user_pending_created
  ON player_custom_skill_revisions(user_id, created_at)
  WHERE result_status = 'pending';
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_user_failed_created
  ON player_custom_skill_revisions(user_id, created_at DESC, id DESC)
  WHERE result_status = 'failed';

CREATE TABLE IF NOT EXISTS player_skill_forge_idempotency_receipts (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK(length(idempotency_key) BETWEEN 1 AND 160),
  response_jsonb JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_player_skill_forge_receipts_user_created
  ON player_skill_forge_idempotency_receipts(user_id, created_at);

CREATE TABLE IF NOT EXISTS player_forged_skillbooks (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  instance_id BIGINT UNIQUE REFERENCES player_item_instances(instance_id) ON DELETE SET NULL,
  custom_skill_id TEXT NOT NULL UNIQUE REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  revision_id TEXT NOT NULL UNIQUE REFERENCES player_custom_skill_revisions(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'inventory' CHECK(status IN ('inventory', 'learned_player', 'learned_kitty', 'destroyed', 'archived')),
  target_kitty_id BIGINT,
  learned_at TIMESTAMPTZ,
  destroyed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_forged_skillbooks_user_active
  ON player_forged_skillbooks(user_id, status, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'player_custom_skills_current_revision_fk') THEN
    ALTER TABLE player_custom_skills
      ADD CONSTRAINT player_custom_skills_current_revision_fk
      FOREIGN KEY(current_revision_id) REFERENCES player_custom_skill_revisions(id) ON DELETE SET NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'player_custom_skill_drafts_pending_revision_fk') THEN
    ALTER TABLE player_custom_skill_drafts
      ADD CONSTRAINT player_custom_skill_drafts_pending_revision_fk
      FOREIGN KEY(pending_revision_id) REFERENCES player_custom_skill_revisions(id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS player_battle_item_loadout (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slot_no INT NOT NULL CHECK(slot_no > 0),
  item_id TEXT NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 0 CHECK(quantity >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, item_id),
  UNIQUE(user_id, slot_no)
);
CREATE INDEX IF NOT EXISTS idx_player_battle_item_loadout_user_updated
  ON player_battle_item_loadout(user_id, slot_no, updated_at, item_id);

-- === Battle domain ===
CREATE TABLE IF NOT EXISTS battle_rooms (
  id BIGSERIAL PRIMARY KEY,
  owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  dungeon_id TEXT NOT NULL,
  room_name TEXT NOT NULL DEFAULT '',
  auto_reenter_on_defeat BOOLEAN NOT NULL DEFAULT TRUE,
  is_public BOOLEAN NOT NULL DEFAULT FALSE,
  disable_entry_round BOOLEAN NOT NULL DEFAULT FALSE,
  status TEXT NOT NULL DEFAULT 'idle' CHECK(status IN ('idle', 'preparing', 'entering', 'playing', 'recovering')),
  completed_runs INT NOT NULL DEFAULT 0 CHECK(completed_runs >= 0),
  consecutive_failures INT NOT NULL DEFAULT 0 CHECK(consecutive_failures >= 0),
  difficulty_level INT NOT NULL DEFAULT 0,
  current_run_id BIGINT,
  seed BIGINT NOT NULL DEFAULT 0,
  owner_worker_id TEXT NOT NULL DEFAULT '',
  lease_until TIMESTAMPTZ,
  next_tick_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  entering_started_at TIMESTAMPTZ,
  recovering_started_at TIMESTAMPTZ,
  recovery_ends_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_battle_rooms_public ON battle_rooms(is_public, dungeon_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_battle_rooms_owner ON battle_rooms(owner_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_battle_rooms_difficulty_level ON battle_rooms(difficulty_level, dungeon_id, updated_at DESC);
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS room_name TEXT NOT NULL DEFAULT '';
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS auto_reenter_on_defeat BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS recovering_started_at TIMESTAMPTZ;
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS recovery_ends_at TIMESTAMPTZ;
ALTER TABLE battle_rooms
  DROP COLUMN IF EXISTS battle_limit;
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS disable_entry_round BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE battle_rooms
  DROP CONSTRAINT IF EXISTS battle_rooms_status_check;
ALTER TABLE battle_rooms
  ADD CONSTRAINT battle_rooms_status_check
  CHECK(status IN ('idle', 'preparing', 'entering', 'playing', 'recovering'));
UPDATE battle_rooms
SET room_name = CONCAT('room-', id)
WHERE trim(room_name) = '';
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT '';
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS schema_version INT NOT NULL DEFAULT 1;
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS consecutive_failures INT NOT NULL DEFAULT 0;
-- Keep lease columns out of claim indexes so owner/lease-only claims remain
-- eligible for HOT updates after the superseded claim indexes are removed.
CREATE INDEX IF NOT EXISTS idx_battle_rooms_v1_status_due
  ON battle_rooms(status, next_tick_at, updated_at, id)
  WHERE schema_version = 1;
CREATE INDEX IF NOT EXISTS idx_battle_rooms_v2_playing_phase_due
  ON battle_rooms(phase, next_tick_at, updated_at, id)
  WHERE schema_version = 2 AND status = 'playing';
CREATE INDEX IF NOT EXISTS idx_battle_rooms_v2_state_due
  ON battle_rooms(next_tick_at, updated_at, id)
  WHERE schema_version = 2
    AND phase = ''
    AND status IN ('entering', 'recovering');

CREATE TABLE IF NOT EXISTS battle_room_members (
  room_id BIGINT NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  position TEXT NOT NULL DEFAULT 'vanguard' CHECK(position IN ('vanguard', 'midguard', 'rearguard')),
  is_ready BOOLEAN NOT NULL DEFAULT FALSE,
  ready_at TIMESTAMPTZ,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(room_id, user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_battle_room_members_user ON battle_room_members(user_id);
CREATE INDEX IF NOT EXISTS idx_battle_room_members_room ON battle_room_members(room_id, joined_at);


CREATE TABLE IF NOT EXISTS battle_runs (
  id BIGSERIAL PRIMARY KEY,
  room_id BIGINT NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  wave INT NOT NULL DEFAULT 1 CHECK(wave > 0),
  status TEXT NOT NULL DEFAULT 'compiled' CHECK(status IN ('compiled', 'playing', 'settling', 'ended', 'failed', 'interrupted')),
  compile_input_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  playback_state_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  final_result_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  result_summary_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_battle_runs_room ON battle_runs(room_id, id DESC);
CREATE TABLE IF NOT EXISTS battle_run_payloads (
  run_id BIGINT PRIMARY KEY REFERENCES battle_runs(id) ON DELETE CASCADE,
  compile_input_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  compile_input_raw BYTEA NOT NULL DEFAULT ''::bytea,
  timeline_raw BYTEA NOT NULL DEFAULT ''::bytea,
  final_result_raw BYTEA NOT NULL DEFAULT ''::bytea,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS battle_run_carry_states (
  run_id BIGINT PRIMARY KEY REFERENCES battle_runs(id) ON DELETE CASCADE,
  carry_state_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS battle_slow_run_archives (
  id BIGSERIAL PRIMARY KEY,
  room_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL,
  dungeon_id TEXT NOT NULL DEFAULT '',
  worker_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  wave INT NOT NULL DEFAULT 1 CHECK (wave > 0),
  outcome TEXT NOT NULL DEFAULT '',
  queue_since_start_ms BIGINT NOT NULL DEFAULT 0,
  compile_elapsed_ms BIGINT NOT NULL DEFAULT 0,
  total_ms BIGINT NOT NULL DEFAULT 0,
  total_steps INT NOT NULL DEFAULT 0,
  total_turns BIGINT NOT NULL DEFAULT 0,
  timeline_events INT NOT NULL DEFAULT 0,
  final_seq BIGINT NOT NULL DEFAULT 0,
  player_units INT NOT NULL DEFAULT 0,
  guard_timed_out BOOLEAN NOT NULL DEFAULT FALSE,
  guard_step_limited BOOLEAN NOT NULL DEFAULT FALSE,
  trigger_flags_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  turn_profile_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  compile_input_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  checkpoint_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  run_started_at TIMESTAMPTZ,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_battle_slow_run_archives_room_run
  ON battle_slow_run_archives(room_id, run_id);
CREATE INDEX IF NOT EXISTS idx_battle_slow_run_archives_room_captured
  ON battle_slow_run_archives(room_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_battle_slow_run_archives_run
  ON battle_slow_run_archives(run_id);
CREATE INDEX IF NOT EXISTS idx_battle_slow_run_archives_captured
  ON battle_slow_run_archives(captured_at DESC);
CREATE TABLE IF NOT EXISTS battle_settlement_jobs (
  id BIGSERIAL PRIMARY KEY,
  room_id BIGINT NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  run_id BIGINT NOT NULL REFERENCES battle_runs(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'done', 'failed', 'quarantined')),
  attempts INT NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  last_error TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by TEXT NOT NULL DEFAULT '',
  locked_until TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(run_id)
);
CREATE INDEX IF NOT EXISTS idx_battle_settlement_jobs_claim
  ON battle_settlement_jobs(status, available_at, locked_until, id);
CREATE TABLE IF NOT EXISTS battle_skill_exp_jobs (
  id BIGSERIAL PRIMARY KEY,
  run_id BIGINT NOT NULL REFERENCES battle_runs(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'done', 'failed', 'quarantined')),
  payload_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  attempts INT NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  last_error TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by TEXT NOT NULL DEFAULT '',
  locked_until TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(run_id)
);
CREATE INDEX IF NOT EXISTS idx_battle_skill_exp_jobs_claim
  ON battle_skill_exp_jobs(status, available_at, locked_until, id);
CREATE TABLE IF NOT EXISTS battle_attr_exp_jobs (
  id BIGSERIAL PRIMARY KEY,
  run_id BIGINT NOT NULL REFERENCES battle_runs(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'done', 'failed', 'quarantined')),
  payload_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  attempts INT NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  last_error TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by TEXT NOT NULL DEFAULT '',
  locked_until TIMESTAMPTZ,
  applied_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(run_id)
);
CREATE INDEX IF NOT EXISTS idx_battle_attr_exp_jobs_claim
  ON battle_attr_exp_jobs(status, available_at, locked_until, id);
CREATE INDEX IF NOT EXISTS idx_battle_attr_exp_jobs_ready
  ON battle_attr_exp_jobs(available_at, id)
  WHERE applied_at IS NULL AND status IN ('pending', 'failed');
CREATE INDEX IF NOT EXISTS idx_battle_attr_exp_jobs_expired_lease
  ON battle_attr_exp_jobs(locked_until, id)
  WHERE applied_at IS NULL AND status = 'running';
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'fk_battle_rooms_current_run'
  ) THEN
    ALTER TABLE battle_rooms
      ADD CONSTRAINT fk_battle_rooms_current_run
      FOREIGN KEY (current_run_id) REFERENCES battle_runs(id) ON DELETE SET NULL;
  END IF;
END $$;

-- Guard battle-domain TRUNCATE to avoid catastrophic cleanup under cyclic FKs.
CREATE OR REPLACE FUNCTION battle_guard_truncate() RETURNS trigger AS $$
DECLARE
  allow_truncate TEXT;
BEGIN
  allow_truncate := COALESCE(current_setting('app.allow_battle_truncate', true), '');
  IF lower(allow_truncate) IN ('on', 'true', '1') THEN
    RETURN NULL;
  END IF;
  RAISE EXCEPTION 'TRUNCATE is blocked for %. Use DELETE-based cleanup or set app.allow_battle_truncate=on in this session explicitly.', TG_TABLE_NAME
    USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_guard_battle_rooms_truncate ON battle_rooms;
CREATE TRIGGER trg_guard_battle_rooms_truncate
  BEFORE TRUNCATE ON battle_rooms
  FOR EACH STATEMENT
  EXECUTE FUNCTION battle_guard_truncate();

DROP TRIGGER IF EXISTS trg_guard_battle_runs_truncate ON battle_runs;
CREATE TRIGGER trg_guard_battle_runs_truncate
  BEFORE TRUNCATE ON battle_runs
  FOR EACH STATEMENT
  EXECUTE FUNCTION battle_guard_truncate();

DROP TRIGGER IF EXISTS trg_guard_battle_room_members_truncate ON battle_room_members;
CREATE TRIGGER trg_guard_battle_room_members_truncate
  BEFORE TRUNCATE ON battle_room_members
  FOR EACH STATEMENT
  EXECUTE FUNCTION battle_guard_truncate();

-- === Equipment growth ===
-- Equipment growth / affix configs now live in shared game-data (packages/game-data).
-- Runtime state only keeps item instances and equip-slot bindings.

-- === Action queue / dynamic attribute progression ===

CREATE TABLE IF NOT EXISTS player_buildings (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  building_id TEXT NOT NULL,
  level BIGINT NOT NULL DEFAULT 0 CHECK(level >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, building_id)
);
CREATE INDEX IF NOT EXISTS idx_player_buildings_user ON player_buildings(user_id);

CREATE TABLE IF NOT EXISTS player_kitties (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  level INT NOT NULL DEFAULT 1 CHECK (level >= 1),
  battle_level_cap BIGINT NOT NULL DEFAULT 100 CHECK (battle_level_cap >= 1),
  exp BIGINT NOT NULL DEFAULT 0 CHECK (exp >= 0),
  total_exp BIGINT NOT NULL DEFAULT 0 CHECK (total_exp >= 0),
  bond_exp BIGINT NOT NULL DEFAULT 0 CHECK (bond_exp >= 0),
  mood DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (mood >= -1 AND mood <= 1),
  status TEXT NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'in_life_queue', 'in_combat')),
  last_commission_at TIMESTAMPTZ,
  extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_kitties_user_created ON player_kitties(user_id, created_at, id);

CREATE TABLE IF NOT EXISTS player_kitty_personalities (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  personality_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kitty_id, personality_id)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_personalities_user_kitty
  ON player_kitty_personalities(user_id, kitty_id, created_at, personality_id);

CREATE TABLE IF NOT EXISTS player_kitty_battle_point_state (
  kitty_id BIGINT PRIMARY KEY REFERENCES player_kitties(id) ON DELETE CASCADE,
  total_points BIGINT NOT NULL DEFAULT 0 CHECK(total_points >= 0),
  spent_points BIGINT NOT NULL DEFAULT 0 CHECK(spent_points >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_kitty_attributes (
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  attr_key TEXT NOT NULL,
  base_level BIGINT NOT NULL DEFAULT 1 CHECK(base_level > 0),
  skill_point_level BIGINT NOT NULL DEFAULT 0 CHECK(skill_point_level >= 0),
  attr_exp DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK(attr_exp >= 0),
  source_mods_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  final_value DOUBLE PRECISION NOT NULL DEFAULT 0,
  lock_version BIGINT NOT NULL DEFAULT 1 CHECK(lock_version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kitty_id, attr_key)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_attributes_kitty ON player_kitty_attributes(kitty_id);

CREATE TABLE IF NOT EXISTS player_kitty_stats_cache (
  kitty_id BIGINT PRIMARY KEY REFERENCES player_kitties(id) ON DELETE CASCADE,
  stats_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_kitty_skills (
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  skill_id TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  cast_order INT NOT NULL DEFAULT 0,
  rule_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  rule_groups_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  level INT NOT NULL DEFAULT 1 CHECK(level >= 1),
  exp BIGINT NOT NULL DEFAULT 0 CHECK(exp >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(kitty_id, skill_id)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_skills_kitty_order
  ON player_kitty_skills(kitty_id, cast_order, skill_id);

CREATE TABLE IF NOT EXISTS player_kitty_equip_slots (
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  equip_type TEXT NOT NULL CHECK(equip_type IN ('battle', 'work')),
  slot TEXT NOT NULL,
  instance_id BIGINT NOT NULL REFERENCES player_item_instances(instance_id) ON DELETE CASCADE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kitty_id, equip_type, slot)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_equip_slots_kitty_instance
  ON player_kitty_equip_slots(kitty_id, instance_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_player_kitty_equip_slots_instance
  ON player_kitty_equip_slots(instance_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_player_equip_slots_battle_instance
  ON player_equip_slots(instance_id)
  WHERE equip_type = 'battle';

CREATE TABLE IF NOT EXISTS battle_room_member_units (
  room_id BIGINT NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  unit_type TEXT NOT NULL CHECK(unit_type IN ('player', 'kitty')),
  unit_id BIGINT NOT NULL DEFAULT 0 CHECK(unit_id >= 0),
  kitty_id BIGINT REFERENCES player_kitties(id) ON DELETE CASCADE,
  position TEXT NOT NULL DEFAULT 'vanguard' CHECK(position IN ('vanguard', 'midguard', 'rearguard')),
  is_ready BOOLEAN NOT NULL DEFAULT FALSE,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(room_id, owner_user_id, unit_type, unit_id),
  CHECK (
    (unit_type = 'player' AND unit_id = 0 AND kitty_id IS NULL)
    OR
    (unit_type = 'kitty' AND unit_id > 0 AND kitty_id = unit_id)
  )
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_battle_room_member_units_kitty_active
  ON battle_room_member_units(kitty_id)
  WHERE unit_type = 'kitty' AND kitty_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_battle_room_member_units_room
  ON battle_room_member_units(room_id, joined_at, owner_user_id);
CREATE INDEX IF NOT EXISTS idx_battle_room_member_units_owner
  ON battle_room_member_units(owner_user_id, room_id, unit_type);

CREATE TABLE IF NOT EXISTS player_kitty_commissions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  commission_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'claimed', 'cancelled')),
  costs_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  reward_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ready_at TIMESTAMPTZ NOT NULL,
  claimed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_commissions_user_kitty_started
  ON player_kitty_commissions(user_id, kitty_id, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_commissions_user_kitty_status
  ON player_kitty_commissions(user_id, kitty_id, status, ready_at, id);

CREATE TABLE IF NOT EXISTS player_kitty_encounters (
  id BIGSERIAL PRIMARY KEY,
  from_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  from_kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  to_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  to_kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  content_key TEXT NOT NULL,
  content_params_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (from_kitty_id <> to_kitty_id),
  CHECK (from_user_id <> to_user_id)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_from_kitty_created
  ON player_kitty_encounters(from_kitty_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_to_kitty_created
  ON player_kitty_encounters(to_kitty_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_from_user_created
  ON player_kitty_encounters(from_user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_encounters_to_user_created
  ON player_kitty_encounters(to_user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS player_kitty_journals (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  encounter_id BIGINT REFERENCES player_kitty_encounters(id) ON DELETE SET NULL,
  entry_type TEXT NOT NULL DEFAULT 'diary' CHECK (entry_type IN ('diary', 'encounter')),
  title_key TEXT NOT NULL DEFAULT '',
  content_key TEXT NOT NULL,
  content_params_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  action_id TEXT NOT NULL DEFAULT '',
  mood DOUBLE PRECISION,
  reward_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  extra_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_journals_user_kitty_created
  ON player_kitty_journals(user_id, kitty_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_kitty_journals_encounter
  ON player_kitty_journals(encounter_id, id DESC);

CREATE TABLE IF NOT EXISTS player_kitty_journal_reward_claims (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  reward_date TEXT NOT NULL,
  reward_slot TEXT NOT NULL,
  reward_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, kitty_id, reward_date, reward_slot, reward_key)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_journal_reward_claims_user_kitty
  ON player_kitty_journal_reward_claims(user_id, kitty_id, reward_date DESC, reward_slot);

CREATE TABLE IF NOT EXISTS player_action_queue (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_type TEXT NOT NULL DEFAULT 'player' CHECK(actor_type IN ('player', 'kitty')),
  actor_id BIGINT NOT NULL DEFAULT 0 CHECK(actor_id >= 0),
  action_id TEXT NOT NULL,
  action_type TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL DEFAULT 'queue' CHECK(mode IN ('run', 'queue')),
  total_count INT NOT NULL CHECK(total_count > 0),
  done_count INT NOT NULL DEFAULT 0 CHECK(done_count >= 0),
  queue_order BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'blocked', 'done', 'canceled', 'failed')),
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  run_context_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  next_process_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  claim_version BIGINT NOT NULL DEFAULT 0,
  current_run_started_at TIMESTAMPTZ,
  current_run_ends_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_action_queue_user_actor_status_order
  ON player_action_queue(user_id, actor_type, actor_id, status, queue_order, id);
CREATE INDEX IF NOT EXISTS idx_player_action_queue_active_lane_order
  ON player_action_queue(user_id, status, actor_type, actor_id, queue_order, id)
  WHERE status IN ('pending', 'running');
CREATE INDEX IF NOT EXISTS idx_player_action_queue_running_due_user
  ON player_action_queue(user_id, current_run_ends_at ASC NULLS FIRST, queue_order, id)
  WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_player_action_queue_pending_user_order
  ON player_action_queue(user_id, queue_order, id, actor_type, actor_id)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_player_action_queue_player_next_process
  ON player_action_queue(user_id, next_process_at, id);
CREATE INDEX IF NOT EXISTS idx_player_action_queue_due ON player_action_queue(next_process_at, status, id);

-- === Account-scoped operation effects ===
CREATE TABLE IF NOT EXISTS player_operation_effect_state (
  user_id    BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision   BIGINT      NOT NULL DEFAULT 0 CHECK(revision >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_operation_effect_grants (
  id                        BIGSERIAL   PRIMARY KEY,
  user_id                   BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  definition_id             TEXT        NOT NULL,
  ruleset_version           TEXT        NOT NULL,
  domain                    TEXT        NOT NULL CHECK(domain ~ '^[a-z][a-z0-9_]{0,63}$'),
  effect_kind               TEXT        NOT NULL,
  stacking_key              TEXT        NOT NULL,
  priority                  INT         NOT NULL DEFAULT 0,
  target_kind               TEXT        NOT NULL CHECK(target_kind IN ('player', 'kitty', 'all_kitties', 'player_and_kitties')),
  target_actor_id           BIGINT,
  lifetime_kind             TEXT        NOT NULL CHECK(lifetime_kind IN ('duration', 'charges')),
  valid_from                TIMESTAMPTZ NOT NULL,
  valid_until               TIMESTAMPTZ,
  initial_charges           BIGINT      NOT NULL DEFAULT 0 CHECK(initial_charges >= 0),
  remaining_charges         BIGINT      NOT NULL DEFAULT 0 CHECK(remaining_charges >= 0),
  definition_snapshot_jsonb JSONB       NOT NULL CHECK(jsonb_typeof(definition_snapshot_jsonb) = 'object'),
  source_item_id            TEXT        NOT NULL DEFAULT '',
  created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (
    (target_kind = 'kitty' AND target_actor_id IS NOT NULL AND target_actor_id > 0)
    OR (target_kind <> 'kitty' AND target_actor_id IS NULL)
  ),
  CHECK (
    (lifetime_kind = 'duration' AND valid_until IS NOT NULL AND valid_until > valid_from AND initial_charges = 0 AND remaining_charges = 0)
    OR
    (lifetime_kind = 'charges' AND valid_until IS NULL AND initial_charges > 0 AND remaining_charges <= initial_charges)
  )
);
CREATE INDEX IF NOT EXISTS idx_player_operation_effect_grants_user_lifetime
  ON player_operation_effect_grants(user_id, lifetime_kind, valid_until, id);
CREATE INDEX IF NOT EXISTS idx_player_operation_effect_grants_user_stack
  ON player_operation_effect_grants(user_id, definition_id, ruleset_version, stacking_key, target_kind, target_actor_id, id);

CREATE TABLE IF NOT EXISTS player_operation_effect_activations (
  id                 BIGSERIAL   PRIMARY KEY,
  user_id            BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  request_id         TEXT        NOT NULL CHECK(length(request_id) BETWEEN 1 AND 160),
  grant_id           BIGINT      NOT NULL REFERENCES player_operation_effect_grants(id) ON DELETE CASCADE,
  source_instance_id BIGINT      NOT NULL,
  source_item_id     TEXT        NOT NULL,
  quantity           BIGINT      NOT NULL DEFAULT 1 CHECK(quantity BETWEEN 1 AND 10000),
  definition_id      TEXT        NOT NULL,
  target_kind        TEXT        NOT NULL CHECK(target_kind IN ('player', 'kitty', 'all_kitties', 'player_and_kitties')),
  target_actor_id    BIGINT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, request_id),
  CHECK (
    (target_kind = 'kitty' AND target_actor_id IS NOT NULL AND target_actor_id > 0)
    OR (target_kind <> 'kitty' AND target_actor_id IS NULL)
  )
);
CREATE INDEX IF NOT EXISTS idx_player_operation_effect_activations_user_created
  ON player_operation_effect_activations(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS player_operation_effect_charge_receipts (
  grant_id      BIGINT      NOT NULL REFERENCES player_operation_effect_grants(id) ON DELETE CASCADE,
  user_id       BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  operation_key TEXT        NOT NULL CHECK(length(operation_key) > 0),
  started_at    TIMESTAMPTZ NOT NULL,
  consumed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(grant_id, operation_key)
);
CREATE INDEX IF NOT EXISTS idx_player_operation_effect_receipts_user_consumed
  ON player_operation_effect_charge_receipts(user_id, consumed_at DESC, grant_id);

CREATE TABLE IF NOT EXISTS player_warehouse_change_logs (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN ('system_to_warehouse', 'warehouse_operation')),
  item_instance_id BIGINT,
  template_item_id TEXT NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 1 CHECK(quantity > 0),
  source_surface TEXT NOT NULL DEFAULT '',
  target_surface TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT 'system',
  direction TEXT NOT NULL DEFAULT 'gain',
  subject_type TEXT NOT NULL DEFAULT '',
  subject_id TEXT NOT NULL DEFAULT '',
  item_totals JSONB NOT NULL DEFAULT '{}'::jsonb,
  instance_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  audit_key TEXT NOT NULL DEFAULT '',
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_warehouse_change_logs_user_created
  ON player_warehouse_change_logs(user_id, created_at DESC, id DESC);
ALTER TABLE player_warehouse_change_logs
  ALTER COLUMN item_instance_id DROP NOT NULL;
ALTER TABLE player_warehouse_change_logs
  DROP CONSTRAINT IF EXISTS player_warehouse_change_logs_item_instance_id_fkey;
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'system';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS direction TEXT NOT NULL DEFAULT 'gain';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS subject_type TEXT NOT NULL DEFAULT '';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS subject_id TEXT NOT NULL DEFAULT '';
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS item_totals JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS instance_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE player_warehouse_change_logs
  ADD COLUMN IF NOT EXISTS audit_key TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_warehouse_change_logs_user_category_created
  ON player_warehouse_change_logs(user_id, category, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_warehouse_change_logs_user_category_direction_created
  ON player_warehouse_change_logs(user_id, category, direction, created_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_warehouse_change_logs_audit_key
  ON player_warehouse_change_logs(audit_key)
  WHERE audit_key <> '';

-- warehouse-only destructive cleanup
DROP TABLE IF EXISTS player_backpack_items;
DROP TABLE IF EXISTS player_backpacks;
DROP TABLE IF EXISTS player_loot_cache_entries;
DROP TABLE IF EXISTS player_attr_modifiers;
DROP TABLE IF EXISTS player_routine_settlements;
DROP TABLE IF EXISTS player_routine_jobs;
DROP TABLE IF EXISTS player_activity_state;
ALTER TABLE player_warehouse_change_logs
  DROP COLUMN IF EXISTS warehouse_id;
DROP TABLE IF EXISTS player_warehouse_items;
DROP TABLE IF EXISTS player_warehouses;

-- === Cosmetic wardrobe ===
CREATE TABLE IF NOT EXISTS player_wardrobe_items (
  user_id      BIGINT      NOT NULL,
  category     TEXT        NOT NULL,
  item_id      TEXT        NOT NULL,
  source       TEXT        NOT NULL DEFAULT '',
  obtained_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, category, item_id)
);
CREATE INDEX IF NOT EXISTS idx_wardrobe_items_user ON player_wardrobe_items (user_id);

CREATE TABLE IF NOT EXISTS wardrobe_equip_slots (
  user_id      BIGINT      NOT NULL,
  owner_type   TEXT        NOT NULL,
  owner_id     BIGINT      NOT NULL DEFAULT 0,
  slot_id      TEXT        NOT NULL,
  item_id      TEXT        NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, owner_type, owner_id, slot_id),
  UNIQUE (user_id, item_id)
);

-- === Feline sparring arena ===

CREATE TABLE IF NOT EXISTS arena_player_states (
  user_id         BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  rating          BIGINT      NOT NULL DEFAULT 1000 CHECK (rating >= 0),
  tokens          BIGINT      NOT NULL DEFAULT 0 CHECK (tokens >= 0),
  attack_wins     BIGINT      NOT NULL DEFAULT 0 CHECK (attack_wins >= 0),
  attack_losses   BIGINT      NOT NULL DEFAULT 0 CHECK (attack_losses >= 0),
  defense_wins    BIGINT      NOT NULL DEFAULT 0 CHECK (defense_wins >= 0),
  defense_losses  BIGINT      NOT NULL DEFAULT 0 CHECK (defense_losses >= 0),
  challenge_charges             INT         NOT NULL DEFAULT 10,
  challenge_charge_refreshed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT arena_player_states_challenge_charges_nonnegative
    CHECK (challenge_charges >= 0)
);

CREATE INDEX IF NOT EXISTS idx_arena_player_states_leaderboard
  ON arena_player_states(
    rating DESC,
    ((attack_wins + defense_wins)) DESC,
    ((attack_losses + defense_losses)) ASC,
    user_id ASC
  );

CREATE OR REPLACE FUNCTION public.arena_kitty_ids_are_valid(candidate BIGINT[])
RETURNS BOOLEAN
LANGUAGE SQL
IMMUTABLE
PARALLEL SAFE
AS $$
  SELECT COALESCE(
    array_position($1, NULL) IS NULL
    AND 0::bigint < ALL($1)
    AND cardinality($1)::bigint = (
      SELECT count(DISTINCT kitty_id)
      FROM unnest($1) AS entries(kitty_id)
    ),
    FALSE
  );
$$;

CREATE TABLE IF NOT EXISTS arena_defenses (
  user_id               BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision              BIGINT      NOT NULL DEFAULT 1 CHECK (revision > 0),
  active                BOOLEAN     NOT NULL DEFAULT TRUE,
  battle_level          BIGINT      NOT NULL DEFAULT 1 CHECK (battle_level > 0),
  kitty_ids             BIGINT[]    NOT NULL,
  selection_jsonb       JSONB       NOT NULL DEFAULT '{}'::jsonb,
  lineup_jsonb          JSONB       NOT NULL DEFAULT '[]'::jsonb,
  lineup_digest         TEXT        NOT NULL DEFAULT '',
  bounty_tokens         BIGINT      NOT NULL DEFAULT 0 CHECK (bounty_tokens >= 0),
  successful_defenses   BIGINT      NOT NULL DEFAULT 0 CHECK (successful_defenses >= 0),
  challenge_count       BIGINT      NOT NULL DEFAULT 0 CHECK (challenge_count >= 0),
  published_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT arena_defenses_valid_kitty_ids
    CHECK (public.arena_kitty_ids_are_valid(kitty_ids))
);

-- CREATE TABLE IF NOT EXISTS does not replace constraints on an existing deployment.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'arena_defenses'::regclass
      AND conname = 'arena_defenses_two_distinct_kitties'
  ) THEN
    ALTER TABLE arena_defenses
      DROP CONSTRAINT arena_defenses_two_distinct_kitties;
  END IF;
  IF EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'arena_defenses'::regclass
      AND conname = 'arena_defenses_up_to_two_distinct_kitties'
  ) THEN
    ALTER TABLE arena_defenses
      DROP CONSTRAINT arena_defenses_up_to_two_distinct_kitties;
  END IF;
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'arena_defenses'::regclass
      AND conname = 'arena_defenses_valid_kitty_ids'
  ) THEN
    ALTER TABLE arena_defenses
      ADD CONSTRAINT arena_defenses_valid_kitty_ids
      CHECK (public.arena_kitty_ids_are_valid(kitty_ids));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_arena_defenses_active_level
  ON arena_defenses(active, battle_level, user_id)
  WHERE active = TRUE;

CREATE INDEX IF NOT EXISTS idx_arena_defenses_active_user
  ON arena_defenses(user_id)
  WHERE active = TRUE;

CREATE TABLE IF NOT EXISTS arena_challenges (
  id                       BIGSERIAL   PRIMARY KEY,
  battle_kind              TEXT        NOT NULL DEFAULT 'arena' CHECK (battle_kind = 'arena'),
  attacker_id              BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  defender_id              BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  defense_revision         BIGINT      NOT NULL CHECK (defense_revision > 0),
  seed                     BIGINT      NOT NULL,
  status                   TEXT        NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'running', 'attacker_win', 'defender_win', 'failed')),
  outcome                  TEXT        NOT NULL DEFAULT ''
    CHECK (outcome IN ('', 'attacker_win', 'defender_win')),
  worker_id                TEXT        NOT NULL DEFAULT '',
  lease_until              TIMESTAMPTZ,
  attempts                 INT         NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  error_code               TEXT        NOT NULL DEFAULT '',
  attacker_lineup_jsonb    JSONB       NOT NULL DEFAULT '[]'::jsonb,
  defender_lineup_jsonb    JSONB       NOT NULL DEFAULT '[]'::jsonb,
  format_snapshot_jsonb    JSONB       NOT NULL DEFAULT '{}'::jsonb,
  rating_eligible          BOOLEAN     NOT NULL DEFAULT FALSE,
  attacker_reward_tokens   BIGINT      NOT NULL DEFAULT 0 CHECK (attacker_reward_tokens >= 0),
  defender_reward_tokens   BIGINT      NOT NULL DEFAULT 0 CHECK (defender_reward_tokens >= 0),
  bounty_reward_tokens     BIGINT      NOT NULL DEFAULT 0 CHECK (bounty_reward_tokens >= 0),
  attacker_rating_delta    BIGINT      NOT NULL DEFAULT 0,
  defender_rating_delta    BIGINT      NOT NULL DEFAULT 0,
  created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at               TIMESTAMPTZ,
  completed_at             TIMESTAMPTZ,
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT arena_challenges_distinct_players CHECK (attacker_id <> defender_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_arena_challenges_attacker_active
  ON arena_challenges(attacker_id)
  WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_arena_challenges_claim
  ON arena_challenges(status, lease_until, created_at, id)
  WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_arena_challenges_attacker_history
  ON arena_challenges(attacker_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_arena_challenges_defender_history
  ON arena_challenges(defender_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_arena_challenges_pair_cooldown
  ON arena_challenges(attacker_id, defender_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_arena_defenses_active_format
  ON arena_defenses((selection_jsonb->>'formatId'), user_id)
  WHERE active = TRUE;

CREATE TABLE IF NOT EXISTS arena_challenge_payloads (
  challenge_id         BIGINT PRIMARY KEY REFERENCES arena_challenges(id) ON DELETE CASCADE,
  initial_state_jsonb  JSONB  NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb       JSONB  NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb   JSONB  NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS arena_replay_shares (
  token          TEXT        PRIMARY KEY,
  challenge_id   BIGINT      NOT NULL REFERENCES arena_challenges(id) ON DELETE CASCADE,
  created_by     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at     TIMESTAMPTZ,
  revoked_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_arena_replay_shares_challenge
  ON arena_replay_shares(challenge_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_arena_replay_shares_challenge_creator
  ON arena_replay_shares(challenge_id, created_by);

-- === Cat Village Combat Trial ===

CREATE TABLE IF NOT EXISTS combat_trial_runs (
  id                     BIGSERIAL   PRIMARY KEY,
  battle_kind            TEXT        NOT NULL DEFAULT 'combat_trial' CHECK (battle_kind = 'combat_trial'),
  user_id                BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  format_id              TEXT        NOT NULL,
  format_definition_digest TEXT      NOT NULL,
  seed                   BIGINT      NOT NULL CHECK (seed > 0),
  status                 TEXT        NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'running', 'completed', 'failed')),
  worker_id              TEXT        NOT NULL DEFAULT '',
  lease_until            TIMESTAMPTZ,
  attempts               INT         NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  error_code             TEXT        NOT NULL DEFAULT '',
  lineup_jsonb           JSONB       NOT NULL DEFAULT '[]'::jsonb,
  lineup_digest          TEXT        NOT NULL,
  format_snapshot_jsonb  JSONB       NOT NULL DEFAULT '{}'::jsonb,
  metrics_jsonb          JSONB       NOT NULL DEFAULT '{}'::jsonb,
  action_count           INT         NOT NULL DEFAULT 0 CHECK (action_count >= 0),
  action_budget          INT         NOT NULL CHECK (action_budget > 0),
  end_reason             TEXT        NOT NULL DEFAULT '',
  kitty_count            INT         NOT NULL DEFAULT 0 CHECK (kitty_count BETWEEN 0 AND 2),
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at             TIMESTAMPTZ,
  completed_at           TIMESTAMPTZ,
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_combat_trial_run_action_budget CHECK (action_count <= action_budget),
  CONSTRAINT chk_combat_trial_run_end_reason CHECK (end_reason = '' OR end_reason ~ '^[a-z0-9_:-]{1,64}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_combat_trial_runs_user_active
  ON combat_trial_runs(user_id)
  WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_combat_trial_runs_claim
  ON combat_trial_runs(status, lease_until, created_at, id)
  WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_combat_trial_runs_user_history
  ON combat_trial_runs(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_combat_trial_runs_current_format
  ON combat_trial_runs(format_definition_digest, completed_at DESC, id DESC)
  WHERE status = 'completed';

CREATE TABLE IF NOT EXISTS combat_trial_run_payloads (
  run_id               BIGINT PRIMARY KEY REFERENCES combat_trial_runs(id) ON DELETE CASCADE,
  initial_state_jsonb  JSONB  NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb       JSONB  NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb   JSONB  NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS combat_trial_personal_bests (
  user_id                 BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  format_id               TEXT             NOT NULL,
  format_definition_digest TEXT            NOT NULL,
  metric_id               TEXT             NOT NULL,
  value                   DOUBLE PRECISION NOT NULL CHECK (value >= 0),
  metrics_jsonb           JSONB            NOT NULL DEFAULT '{}'::jsonb,
  end_reason              TEXT             NOT NULL DEFAULT '',
  kitty_count             INT              NOT NULL DEFAULT 0 CHECK (kitty_count BETWEEN 0 AND 2),
  achieved_at             TIMESTAMPTZ      NOT NULL,
  run_id                  BIGINT            REFERENCES combat_trial_runs(id) ON DELETE SET NULL,
  CONSTRAINT chk_combat_trial_personal_best_end_reason CHECK (end_reason = '' OR end_reason ~ '^[a-z0-9_:-]{1,64}$'),
  PRIMARY KEY (user_id, format_definition_digest, metric_id)
);

CREATE INDEX IF NOT EXISTS idx_combat_trial_personal_bests_leaderboard
  ON combat_trial_personal_bests(format_definition_digest, metric_id, value DESC, achieved_at ASC, user_id ASC);
CREATE INDEX IF NOT EXISTS idx_combat_trial_personal_bests_leaderboard_asc
  ON combat_trial_personal_bests(format_definition_digest, metric_id, value ASC, achieved_at ASC, user_id ASC);

CREATE TABLE IF NOT EXISTS combat_trial_replay_shares (
  token       TEXT        PRIMARY KEY,
  run_id      BIGINT      NOT NULL REFERENCES combat_trial_runs(id) ON DELETE CASCADE,
  created_by  BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_combat_trial_replay_shares_run
  ON combat_trial_replay_shares(run_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_combat_trial_replay_shares_run_creator
  ON combat_trial_replay_shares(run_id, created_by);


CREATE TABLE IF NOT EXISTS arena_shop_orders (
  id                BIGSERIAL   PRIMARY KEY,
  user_id           BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  offer_id          TEXT        NOT NULL,
  item_id           TEXT        NOT NULL,
  quantity          BIGINT      NOT NULL CHECK (quantity > 0),
  token_cost        BIGINT      NOT NULL CHECK (token_cost > 0),
  idempotency_key   TEXT        NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_arena_shop_orders_user_idempotency UNIQUE (user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_arena_shop_orders_user_created
  ON arena_shop_orders(user_id, created_at DESC, id DESC);


-- === Achievements ===
CREATE UNLOGGED TABLE IF NOT EXISTS achievement_events (
  event_id      TEXT        PRIMARY KEY,
  user_id       BIGINT      NOT NULL,
  kind          TEXT        NOT NULL,
  counter_key   TEXT        NOT NULL,
  delta         BIGINT      NOT NULL DEFAULT 1,
  payload       JSONB       NOT NULL DEFAULT '{}'::jsonb,
  status        TEXT        NOT NULL DEFAULT 'pending',
  attempts      INT         NOT NULL DEFAULT 0,
  available_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by     TEXT        NOT NULL DEFAULT '',
  locked_until  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at  TIMESTAMPTZ,
  last_error    TEXT        NOT NULL DEFAULT ''
);
-- Keep the hot queue on purpose-built pending/reclaim indexes. The former
-- broad claim and user/status indexes duplicated this work and bloated badly.
CREATE INDEX IF NOT EXISTS idx_achievement_events_pending_claim
  ON achievement_events(status, available_at, created_at);
CREATE INDEX IF NOT EXISTS idx_achievement_events_processing_claim
  ON achievement_events(status, locked_until, created_at);
CREATE INDEX IF NOT EXISTS idx_achievement_events_pending_due
  ON achievement_events(available_at, created_at, event_id)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_achievement_events_processing_due
  ON achievement_events(locked_until, created_at, event_id)
  WHERE status = 'processing' AND locked_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_achievement_events_processed
  ON achievement_events(status, processed_at);

CREATE TABLE IF NOT EXISTS player_achievement_states (
  user_id          BIGINT      NOT NULL,
  achievement_id   TEXT        NOT NULL,
  unlocked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at     TIMESTAMPTZ,
  claimed_at       TIMESTAMPTZ,
  progress_value   BIGINT      NOT NULL DEFAULT 0,
  definition_hash  TEXT        NOT NULL DEFAULT '',
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, achievement_id)
);
CREATE INDEX IF NOT EXISTS idx_player_achievement_states_user_completed
  ON player_achievement_states(user_id, completed_at);
CREATE INDEX IF NOT EXISTS idx_player_achievement_states_user_claimed
  ON player_achievement_states(user_id, claimed_at);

-- === Rumor / collectibles ===

CREATE TABLE IF NOT EXISTS player_rumor_completion (
  user_id        BIGINT NOT NULL,
  commission_id  TEXT NOT NULL,
  completed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, commission_id)
);

CREATE TABLE IF NOT EXISTS player_rumor_pity (
  user_id        BIGINT NOT NULL,
  commission_id  TEXT NOT NULL,
  counter        BIGINT NOT NULL DEFAULT 0,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, commission_id)
);

CREATE TABLE IF NOT EXISTS player_collectibles (
  user_id        BIGINT NOT NULL,
  collectible_id TEXT NOT NULL,
  level          INT NOT NULL DEFAULT 1,
  obtained_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, collectible_id)
);

CREATE TABLE IF NOT EXISTS user_known_ips (
  user_id      BIGINT       NOT NULL,
  ip           TEXT         NOT NULL,
  use_count    INT          NOT NULL DEFAULT 1,
  last_seen_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_user_known_ips_user_id ON user_known_ips (user_id);

-- === Cosmetic wardrobe ===

CREATE TABLE IF NOT EXISTS player_wardrobe_items (
  user_id      BIGINT      NOT NULL,
  category     TEXT        NOT NULL,
  item_id      TEXT        NOT NULL,
  source       TEXT        NOT NULL DEFAULT '',
  obtained_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, category, item_id)
);

CREATE TABLE IF NOT EXISTS wardrobe_equip_slots (
  user_id      BIGINT      NOT NULL,
  owner_type   TEXT        NOT NULL,
  owner_id     BIGINT      NOT NULL DEFAULT 0,
  slot_id      TEXT        NOT NULL,
  item_id      TEXT        NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, owner_type, owner_id, slot_id),
  UNIQUE (user_id, item_id)
);
-- === Real idle fishing (current schema) ===
CREATE TABLE IF NOT EXISTS player_one_time_grants (
  user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  grant_key    TEXT        NOT NULL CHECK(length(btrim(grant_key)) BETWEEN 1 AND 120),
  grant_jsonb  JSONB       NOT NULL DEFAULT '{}'::jsonb,
  granted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, grant_key)
);

ALTER TABLE player_item_instances
  ADD COLUMN IF NOT EXISTS fish_data JSONB NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_player_item_instances_fish_quantity'
  ) THEN
    ALTER TABLE player_item_instances
      ADD CONSTRAINT chk_player_item_instances_fish_quantity
      CHECK (fish_data = '{}'::jsonb OR quantity = 1) NOT VALID;
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_player_item_instances_fish_shape'
  ) THEN
    ALTER TABLE player_item_instances
      ADD CONSTRAINT chk_player_item_instances_fish_shape
      CHECK (
        fish_data = '{}'::jsonb OR (
          template_item_id = 'fish_catch'
          AND quantity = 1
          AND enhance_level = 0
          AND affixes = '[]'::jsonb
        )
      ) NOT VALID;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_player_item_instances_user_fish
  ON player_item_instances(user_id, instance_id DESC)
  WHERE fish_data <> '{}'::jsonb AND state = 'inventory';

CREATE TABLE IF NOT EXISTS fishing_loadouts (
  id                  BIGSERIAL   PRIMARY KEY,
  user_id             BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  loadout_id          TEXT        NOT NULL,
  name                TEXT        NOT NULL DEFAULT '',
  method              TEXT        NOT NULL CHECK (method IN ('float', 'bottom', 'lure')),
  strategy            TEXT        NOT NULL DEFAULT 'balanced' CHECK (strategy IN ('conservative', 'balanced', 'aggressive')),
  safety_load_percent INT         NOT NULL DEFAULT 82 CHECK (safety_load_percent BETWEEN 50 AND 100),
  components_jsonb    JSONB       NOT NULL DEFAULT '{}'::jsonb,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, loadout_id)
);
CREATE INDEX IF NOT EXISTS idx_fishing_loadouts_user_updated
  ON fishing_loadouts(user_id, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS fishing_spot_pressure (
  user_id       BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  spot_id       TEXT             NOT NULL,
  pressure      DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (pressure BETWEEN 0 AND 1),
  last_fished_at TIMESTAMPTZ,
  updated_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, spot_id)
);

CREATE TABLE IF NOT EXISTS fishing_tackle_wear (
  user_id    BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id    TEXT             NOT NULL CHECK(length(btrim(item_id)) BETWEEN 1 AND 120),
  wear       DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK(wear >= 0 AND wear < 1),
  updated_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, item_id)
);

CREATE TABLE IF NOT EXISTS fishing_trips (
  id                    TEXT        PRIMARY KEY,
  user_id               BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  queue_id              BIGINT      REFERENCES player_action_queue(id) ON DELETE SET NULL,
  actor_type            TEXT        NOT NULL DEFAULT 'player' CHECK(actor_type IN ('player', 'kitty')),
  actor_id              BIGINT      NOT NULL DEFAULT 0 CHECK((actor_type = 'player' AND actor_id = 0) OR (actor_type = 'kitty' AND actor_id > 0)),
  water_id              TEXT        NOT NULL,
  spot_id               TEXT        NOT NULL,
  loadout_id            TEXT        NOT NULL,
  loadout_snapshot_jsonb JSONB       NOT NULL DEFAULT '{}'::jsonb,
  strategy              TEXT        NOT NULL CHECK (strategy IN ('conservative', 'balanced', 'aggressive')),
  safety_load_percent   INT         NOT NULL CHECK (safety_load_percent BETWEEN 50 AND 100),
  keep_rule_jsonb       JSONB       NOT NULL DEFAULT '{}'::jsonb,
  status                TEXT        NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'stopped')),
  phase                 TEXT        NOT NULL DEFAULT 'waiting' CHECK (phase IN ('waiting', 'bite', 'fighting', 'landed', 'resting', 'finished')),
  phase_started_at      TIMESTAMPTZ NOT NULL,
  phase_ends_at         TIMESTAMPTZ NOT NULL,
  planned_end_at        TIMESTAMPTZ,
  current_fish_jsonb    JSONB       NOT NULL DEFAULT '{}'::jsonb,
  summary_jsonb         JSONB       NOT NULL DEFAULT '{}'::jsonb,
  supplies_jsonb        JSONB       NOT NULL DEFAULT '{}'::jsonb,
  stop_reason           TEXT        NOT NULL DEFAULT '',
  ruleset_version       TEXT        NOT NULL DEFAULT 'fishing-v1',
  seed                  BIGINT      NOT NULL,
  event_seq             BIGINT      NOT NULL DEFAULT 0 CHECK (event_seq >= 0),
  started_at            TIMESTAMPTZ NOT NULL,
  ended_at              TIMESTAMPTZ,
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_fishing_trips_user_running
  ON fishing_trips(user_id)
  WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_fishing_trips_user_started
  ON fishing_trips(user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_fishing_trips_queue
  ON fishing_trips(queue_id)
  WHERE queue_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fishing_trips_spot_running
  ON fishing_trips(spot_id)
  WHERE status = 'running';

CREATE TABLE IF NOT EXISTS fishing_trip_events (
  trip_id     TEXT   NOT NULL REFERENCES fishing_trips(id) ON DELETE CASCADE,
  sequence    BIGINT NOT NULL CHECK (sequence > 0),
  event_jsonb JSONB  NOT NULL CHECK (jsonb_typeof(event_jsonb) = 'object'),
  PRIMARY KEY(trip_id, sequence)
);

CREATE TABLE IF NOT EXISTS fishing_catches (
  id                    BIGSERIAL   PRIMARY KEY,
  trip_id               TEXT        NOT NULL REFERENCES fishing_trips(id) ON DELETE CASCADE,
  user_id               BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  species_id            TEXT        NOT NULL,
  fish_data_jsonb       JSONB       NOT NULL,
  kept                  BOOLEAN     NOT NULL,
  inventory_instance_id BIGINT      REFERENCES player_item_instances(instance_id) ON DELETE SET NULL,
  caught_at             TIMESTAMPTZ NOT NULL,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_fishing_catches_user_caught
  ON fishing_catches(user_id, caught_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_fishing_catches_user_id
  ON fishing_catches(user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_fishing_catches_inventory_instance
  ON fishing_catches(inventory_instance_id)
  WHERE inventory_instance_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fishing_catches_species_length
  ON fishing_catches(species_id, ((fish_data_jsonb->>'lengthCm')::double precision) DESC, caught_at ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_fishing_catches_species_weight
  ON fishing_catches(species_id, ((fish_data_jsonb->>'weightKg')::double precision) DESC, caught_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS fishing_species_knowledge (
  user_id          BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  species_id       TEXT        NOT NULL,
  encounters       BIGINT      NOT NULL DEFAULT 0 CHECK (encounters >= 0),
  catches          BIGINT      NOT NULL DEFAULT 0 CHECK (catches >= 0),
  releases         BIGINT      NOT NULL DEFAULT 0 CHECK (releases >= 0),
  first_caught_at  TIMESTAMPTZ,
  last_caught_at   TIMESTAMPTZ,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, species_id)
);

CREATE TABLE IF NOT EXISTS fishing_records (
  user_id              BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  species_id           TEXT             NOT NULL,
  ruleset_version      TEXT             NOT NULL,
  best_length_cm       DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (best_length_cm >= 0),
  best_length_catch_id BIGINT           REFERENCES fishing_catches(id) ON DELETE SET NULL,
  best_weight_kg       DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (best_weight_kg >= 0),
  best_weight_catch_id BIGINT           REFERENCES fishing_catches(id) ON DELETE SET NULL,
  updated_at           TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, species_id, ruleset_version)
);
CREATE INDEX IF NOT EXISTS idx_fishing_records_length_board
  ON fishing_records(ruleset_version, species_id, best_length_cm DESC, updated_at ASC, user_id ASC);
CREATE INDEX IF NOT EXISTS idx_fishing_records_weight_board
  ON fishing_records(ruleset_version, species_id, best_weight_kg DESC, updated_at ASC, user_id ASC);
CREATE INDEX IF NOT EXISTS idx_fishing_records_best_length_catch
  ON fishing_records(best_length_catch_id)
  WHERE best_length_catch_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fishing_records_best_weight_catch
  ON fishing_records(best_weight_catch_id)
  WHERE best_weight_catch_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS fishing_tournaments (
  id               TEXT        PRIMARY KEY,
  week_key         TEXT        NOT NULL,
  target_species_id TEXT       NOT NULL,
  ruleset_version  TEXT        NOT NULL DEFAULT 'fishing-v1',
  starts_at        TIMESTAMPTZ NOT NULL,
  ends_at          TIMESTAMPTZ NOT NULL,
  status           TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'finalized')),
  finalized_at     TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_fishing_tournaments_week_ruleset
  ON fishing_tournaments(week_key, ruleset_version);

CREATE TABLE IF NOT EXISTS fishing_tournament_entries (
  tournament_id TEXT             NOT NULL REFERENCES fishing_tournaments(id) ON DELETE CASCADE,
  user_id       BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  division      TEXT             NOT NULL CHECK (division IN ('open', 'standard')),
  catch_id      BIGINT           REFERENCES fishing_catches(id) ON DELETE CASCADE,
  fish_instance_id BIGINT        REFERENCES player_item_instances(instance_id) ON DELETE SET NULL,
  source_kind   TEXT             NOT NULL DEFAULT 'catch' CHECK (source_kind IN ('catch', 'cultured')),
  length_cm     DOUBLE PRECISION NOT NULL CHECK (length_cm > 0),
  weight_kg     DOUBLE PRECISION NOT NULL CHECK (weight_kg > 0),
  submitted_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(tournament_id, user_id, division)
);
CREATE INDEX IF NOT EXISTS idx_fishing_tournament_entries_board
  ON fishing_tournament_entries(tournament_id, division, weight_kg DESC, length_cm DESC, submitted_at ASC, user_id ASC);
CREATE INDEX IF NOT EXISTS idx_fishing_tournament_entries_catch
  ON fishing_tournament_entries(catch_id);
CREATE INDEX IF NOT EXISTS idx_fishing_tournament_entries_fish_instance
  ON fishing_tournament_entries(fish_instance_id)
  WHERE fish_instance_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS fishing_tournament_rewards (
  tournament_id   TEXT        NOT NULL REFERENCES fishing_tournaments(id) ON DELETE CASCADE,
  user_id         BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  division        TEXT        NOT NULL CHECK (division IN ('open', 'standard')),
  rank            INT         NOT NULL CHECK (rank > 0),
  token_amount    BIGINT      NOT NULL DEFAULT 0 CHECK (token_amount >= 0),
  item_id         TEXT        NOT NULL DEFAULT '',
  cosmetic_key    TEXT        NOT NULL DEFAULT '',
  claimed_at      TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(tournament_id, user_id, division)
);
CREATE INDEX IF NOT EXISTS idx_fishing_tournament_rewards_user_created
  ON fishing_tournament_rewards(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS fishing_player_wallets (
  user_id    BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  tokens     BIGINT      NOT NULL DEFAULT 0 CHECK (tokens >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fishing_token_redemptions (
  user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reward_key  TEXT        NOT NULL,
  token_cost  BIGINT      NOT NULL CHECK (token_cost >= 0),
  redeemed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, reward_key)
);

CREATE TABLE IF NOT EXISTS fishing_npc_supply (
  market_date DATE             NOT NULL,
  species_id  TEXT             NOT NULL,
  sold_weight_kg DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (sold_weight_kg >= 0),
  sold_count  BIGINT           NOT NULL DEFAULT 0 CHECK (sold_count >= 0),
  updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(market_date, species_id)
);

CREATE TABLE IF NOT EXISTS fishing_npc_sales (
  id              BIGSERIAL   PRIMARY KEY,
  user_id         BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idempotency_key TEXT        NOT NULL,
  instance_ids    BIGINT[]    NOT NULL,
  total_gold      BIGINT      NOT NULL CHECK (total_gold >= 0),
  quote_jsonb     JSONB       NOT NULL DEFAULT '{}'::jsonb,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_fishing_npc_sales_user_created
  ON fishing_npc_sales(user_id, created_at DESC, id DESC);

-- === Player fish ponds ===
CREATE TABLE IF NOT EXISTS fishing_ponds (
  user_id            BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  feed_stock_jsonb   JSONB       NOT NULL DEFAULT '{}'::jsonb,
  ruleset_version    TEXT        NOT NULL DEFAULT 'fishing-pond-v5',
  event_seq          BIGINT      NOT NULL DEFAULT 0 CHECK (event_seq >= 0),
  last_settled_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_fishing_ponds_ruleset_version
    CHECK (ruleset_version IN ('fishing-pond-v4', 'fishing-pond-v5'))
);

CREATE TABLE IF NOT EXISTS fishing_pond_pairs (
  user_id             BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slot_index          INT         NOT NULL CHECK (slot_index >= 0),
  female_instance_id  BIGINT      NOT NULL REFERENCES player_item_instances(instance_id) ON DELETE CASCADE,
  male_instance_id    BIGINT      NOT NULL REFERENCES player_item_instances(instance_id) ON DELETE CASCADE,
  mash_ready          BOOLEAN     NOT NULL DEFAULT FALSE,
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, slot_index),
  UNIQUE(user_id, female_instance_id),
  UNIQUE(user_id, male_instance_id),
  CHECK(female_instance_id <> male_instance_id)
);

CREATE TABLE IF NOT EXISTS fishing_pond_events (
  id                   BIGSERIAL   PRIMARY KEY,
  user_id              BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_kind           TEXT        NOT NULL,
  fish_instance_id     BIGINT,
  related_instance_id  BIGINT,
  details_jsonb        JSONB       NOT NULL DEFAULT '{}'::jsonb,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_fishing_pond_events_user_created
  ON fishing_pond_events(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_player_item_instances_user_pond_fish
  ON player_item_instances(user_id, instance_id)
  WHERE state = 'fishing_pond' AND fish_data <> '{}'::jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_player_item_instances_fishing_pond_zone'
  ) THEN
    ALTER TABLE player_item_instances
      ADD CONSTRAINT chk_player_item_instances_fishing_pond_zone
      CHECK (
        state <> 'fishing_pond'
        OR template_item_id <> 'fish_catch'
        OR COALESCE(fish_data->>'pondZone', '') IN ('main', 'nursery')
      ) NOT VALID;
  END IF;
END $$;

ALTER TABLE player_item_instances
  VALIDATE CONSTRAINT chk_player_item_instances_fishing_pond_zone;

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

CREATE TABLE IF NOT EXISTS player_cattery_furniture_state (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
  exp_remainders JSONB NOT NULL DEFAULT '{}'::jsonb,
  contribution_totals JSONB NOT NULL DEFAULT '{}'::jsonb,
  contribution_seen JSONB NOT NULL DEFAULT '{}'::jsonb,
  contribution_report JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE TABLE IF NOT EXISTS player_cattery_furniture (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  furniture_id TEXT NOT NULL,
  definition JSONB NOT NULL,
  remaining_ms BIGINT NOT NULL CHECK (remaining_ms >= 0),
  enabled_until TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL,
  effective_at TIMESTAMPTZ,
  previous_state JSONB,
  PRIMARY KEY (user_id, furniture_id)
);
CREATE TABLE IF NOT EXISTS player_cattery_furniture_periods (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  furniture_id TEXT NOT NULL,
  valid_from TIMESTAMPTZ NOT NULL,
  valid_until TIMESTAMPTZ,
  source JSONB NOT NULL,
  CHECK (valid_until >= valid_from)
);
CREATE INDEX IF NOT EXISTS cattery_furniture_period_window_idx
  ON player_cattery_furniture_periods(user_id, valid_until, valid_from);
CREATE TABLE IF NOT EXISTS player_cattery_furniture_commands (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  request_id TEXT NOT NULL,
  request JSONB NOT NULL,
  revision BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, request_id)
);
CREATE INDEX IF NOT EXISTS cattery_furniture_command_cleanup_idx
  ON player_cattery_furniture_commands(user_id, created_at);

CREATE OR REPLACE FUNCTION cattery_merge_contributions(previous jsonb, delta jsonb)
RETURNS jsonb LANGUAGE plpgsql IMMUTABLE STRICT AS $$
DECLARE
  result jsonb := previous;
  source_id text;
  addition jsonb;
  current_row jsonb;
  field_name text;
  counter_name text;
  metric_key text;
  metric_value jsonb;
  metrics jsonb;
  amount numeric;
BEGIN
  FOR source_id, addition IN SELECT * FROM jsonb_each(delta) LOOP
    current_row := COALESCE(result->source_id, '{}'::jsonb);
    FOREACH field_name IN ARRAY ARRAY['experience','items'] LOOP
      metrics := COALESCE(current_row->field_name, '{}'::jsonb);
      FOR metric_key, metric_value IN SELECT * FROM jsonb_each(COALESCE(addition->field_name,'{}'::jsonb)) LOOP
        amount := COALESCE((metrics->>metric_key)::numeric,0) + (metric_value #>> '{}')::numeric;
        IF field_name='experience' THEN amount := round(amount,4); END IF;
        IF amount<0 OR amount>1000000000000 THEN RAISE EXCEPTION 'furniture contribution overflow'; END IF;
        metrics := jsonb_set(metrics,ARRAY[metric_key],to_jsonb(amount));
      END LOOP;
      current_row := jsonb_set(current_row,ARRAY[field_name],metrics);
    END LOOP;
    FOREACH counter_name IN ARRAY ARRAY['opportunities','triggered','empty'] LOOP
      amount := COALESCE((current_row->>counter_name)::numeric,0) + COALESCE((addition->>counter_name)::numeric,0);
      IF amount<0 OR amount>1000000000000 THEN RAISE EXCEPTION 'furniture contribution overflow'; END IF;
      current_row := jsonb_set(current_row,ARRAY[counter_name],to_jsonb(amount));
    END LOOP;
    result := jsonb_set(result,ARRAY[source_id],current_row);
  END LOOP;
  RETURN result;
END;
$$;

-- Apply before deploying the server and action-worker together.
CREATE TABLE IF NOT EXISTS player_offline_recoveries (
  id BIGSERIAL PRIMARY KEY,
  background BOOLEAN NOT NULL DEFAULT false,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  started_at TIMESTAMPTZ NOT NULL,
  target_at TIMESTAMPTZ NOT NULL,
  resumed_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running','complete','stalled')),
  revision BIGINT NOT NULL DEFAULT 0,
  summary JSONB NOT NULL DEFAULT '{}',
  pending_run_ids BIGINT[] NOT NULL DEFAULT '{}',
  attempts INT NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  acknowledged_at TIMESTAMPTZ,
  schedule_pending BOOLEAN NOT NULL DEFAULT false,
  CHECK (target_at >= started_at AND resumed_at >= target_at)
);
CREATE INDEX IF NOT EXISTS idx_offline_recovery_user ON player_offline_recoveries(user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_offline_recovery_ready ON player_offline_recoveries(available_at, id) WHERE status='running';
CREATE INDEX IF NOT EXISTS idx_offline_recovery_unfinished ON player_offline_recoveries(user_id, id) WHERE status IN ('running','stalled');
CREATE INDEX IF NOT EXISTS idx_offline_recovery_unread ON player_offline_recoveries(user_id, id) WHERE acknowledged_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_offline_recovery_schedule ON player_offline_recoveries(id) WHERE schedule_pending;
CREATE UNIQUE INDEX IF NOT EXISTS idx_offline_recovery_background_user ON player_offline_recoveries(user_id) WHERE background;

-- No run/room FK: room cleanup must never remove reward evidence.
CREATE TABLE IF NOT EXISTS battle_reward_receipts (
  run_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_rewards JSONB NOT NULL,
  settled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id,user_id)
);
CREATE INDEX IF NOT EXISTS idx_battle_reward_receipt_user_time ON battle_reward_receipts(user_id,settled_at);
