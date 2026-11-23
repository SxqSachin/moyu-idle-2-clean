CREATE TABLE IF NOT EXISTS player_kitty_journal_reward_claims (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  reward_date TEXT NOT NULL,
  reward_slot TEXT NOT NULL,
  reward_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, kitty_id, reward_date, reward_slot, reward_key)
);

CREATE INDEX IF NOT EXISTS idx_player_kitty_journal_reward_claims_user_kitty
  ON player_kitty_journal_reward_claims(user_id, kitty_id, reward_date DESC, reward_slot);
