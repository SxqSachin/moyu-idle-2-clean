CREATE TABLE IF NOT EXISTS player_kitty_battle_point_state (
  kitty_id BIGINT PRIMARY KEY REFERENCES player_kitties(id) ON DELETE CASCADE,
  total_points BIGINT NOT NULL DEFAULT 0 CHECK(total_points >= 0),
  spent_points BIGINT NOT NULL DEFAULT 0 CHECK(spent_points >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
