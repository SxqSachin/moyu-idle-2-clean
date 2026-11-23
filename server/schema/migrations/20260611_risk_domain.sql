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
