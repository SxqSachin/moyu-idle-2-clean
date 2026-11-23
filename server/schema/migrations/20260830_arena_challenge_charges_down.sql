SET lock_timeout = '5s';

ALTER TABLE arena_player_states
  DROP CONSTRAINT IF EXISTS arena_player_states_challenge_charges_nonnegative,
  DROP COLUMN IF EXISTS challenge_charge_refreshed_at,
  DROP COLUMN IF EXISTS challenge_charges;
