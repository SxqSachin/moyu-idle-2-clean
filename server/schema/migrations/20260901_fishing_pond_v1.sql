-- Fishing pond V1. Apply manually through the normal migration workflow.
CREATE TABLE IF NOT EXISTS fishing_ponds (
  user_id            BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  feed_stock_jsonb   JSONB       NOT NULL DEFAULT '{}'::jsonb,
  event_seq          BIGINT      NOT NULL DEFAULT 0 CHECK (event_seq >= 0),
  last_settled_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
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

-- Cultured adults may enter only the weekly open division. The measured fish
-- data is snapshotted so later growth, sale, or death cannot rewrite a result.
ALTER TABLE fishing_tournament_entries
  ALTER COLUMN catch_id DROP NOT NULL,
  ADD COLUMN IF NOT EXISTS fish_instance_id BIGINT REFERENCES player_item_instances(instance_id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS source_kind TEXT NOT NULL DEFAULT 'catch';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_fishing_tournament_entries_source_kind'
  ) THEN
    ALTER TABLE fishing_tournament_entries
      ADD CONSTRAINT chk_fishing_tournament_entries_source_kind
      CHECK (source_kind IN ('catch', 'cultured'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_fishing_tournament_entries_fish_instance
  ON fishing_tournament_entries(fish_instance_id)
  WHERE fish_instance_id IS NOT NULL;

ALTER TABLE market_auctions
  ADD COLUMN IF NOT EXISTS instance_fish_data JSONB NOT NULL DEFAULT '{}'::jsonb;
