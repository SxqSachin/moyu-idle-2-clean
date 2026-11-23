SET lock_timeout = '5s';

ALTER TABLE arena_player_states
  ADD COLUMN IF NOT EXISTS challenge_charges INT NOT NULL DEFAULT 10,
  ADD COLUMN IF NOT EXISTS challenge_charge_refreshed_at TIMESTAMPTZ NOT NULL DEFAULT now();

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'arena_player_states'::regclass
      AND conname = 'arena_player_states_challenge_charges_nonnegative'
  ) THEN
    ALTER TABLE arena_player_states
      ADD CONSTRAINT arena_player_states_challenge_charges_nonnegative
      CHECK (challenge_charges >= 0);
  END IF;
END $$;
