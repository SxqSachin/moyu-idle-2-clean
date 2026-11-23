-- Fishing tackle wear: preserve fractional consumable wear across fishing trips.
CREATE TABLE IF NOT EXISTS fishing_tackle_wear (
  user_id    BIGINT           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id    TEXT             NOT NULL CHECK(length(btrim(item_id)) BETWEEN 1 AND 120),
  wear       DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK(wear >= 0 AND wear < 1),
  updated_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, item_id)
);
