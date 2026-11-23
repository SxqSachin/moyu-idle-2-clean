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
