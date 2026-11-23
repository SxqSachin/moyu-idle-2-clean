-- Fishing V1.1: reusable one-time grants, trip supply snapshots, and spot population reads.
CREATE TABLE IF NOT EXISTS player_one_time_grants (
  user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  grant_key    TEXT        NOT NULL CHECK(length(btrim(grant_key)) BETWEEN 1 AND 120),
  grant_jsonb  JSONB       NOT NULL DEFAULT '{}'::jsonb,
  granted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, grant_key)
);

ALTER TABLE fishing_trips
  ADD COLUMN IF NOT EXISTS supplies_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_fishing_trips_spot_running
  ON fishing_trips(spot_id)
  WHERE status = 'running';

