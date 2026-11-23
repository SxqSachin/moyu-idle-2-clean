CREATE TABLE user_known_ips (
  user_id      BIGINT       NOT NULL,
  ip           TEXT         NOT NULL,
  use_count    INT          NOT NULL DEFAULT 1,
  last_seen_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, ip)
);
CREATE INDEX ON user_known_ips (user_id);
