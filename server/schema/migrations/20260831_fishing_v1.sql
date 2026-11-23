-- === Real idle fishing V1 ===
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
          AND custom_data = '{}'::jsonb
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

CREATE TABLE IF NOT EXISTS fishing_trips (
  id                    TEXT        PRIMARY KEY,
  user_id               BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  queue_id              BIGINT      REFERENCES player_action_queue(id) ON DELETE SET NULL,
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
ALTER TABLE fishing_tournaments
  DROP CONSTRAINT IF EXISTS fishing_tournaments_week_key_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_fishing_tournaments_week_ruleset
  ON fishing_tournaments(week_key, ruleset_version);

CREATE TABLE IF NOT EXISTS fishing_tournament_entries (
  tournament_id TEXT             NOT NULL REFERENCES fishing_tournaments(id) ON DELETE CASCADE,
  user_id       BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  division      TEXT             NOT NULL CHECK (division IN ('open', 'standard')),
  catch_id      BIGINT           NOT NULL REFERENCES fishing_catches(id) ON DELETE CASCADE,
  length_cm     DOUBLE PRECISION NOT NULL CHECK (length_cm > 0),
  weight_kg     DOUBLE PRECISION NOT NULL CHECK (weight_kg > 0),
  submitted_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(tournament_id, user_id, division)
);
CREATE INDEX IF NOT EXISTS idx_fishing_tournament_entries_board
  ON fishing_tournament_entries(tournament_id, division, weight_kg DESC, length_cm DESC, submitted_at ASC, user_id ASC);
CREATE INDEX IF NOT EXISTS idx_fishing_tournament_entries_catch
  ON fishing_tournament_entries(catch_id);

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
