-- Account-scoped, start-time-frozen operation effects. Applying this migration
-- remains a deliberate deployment step; application startup does not run it.
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
