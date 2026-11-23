SET lock_timeout = '5s';
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

CREATE INDEX IF NOT EXISTS idx_arena_defenses_active_level
  ON arena_defenses(active, battle_level, user_id)
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

ALTER TABLE chat_messages
  ADD COLUMN IF NOT EXISTS attachment_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

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
