SET lock_timeout = '5s';

CREATE INDEX IF NOT EXISTS idx_arena_player_states_leaderboard
  ON arena_player_states(
    rating DESC,
    ((attack_wins + defense_wins)) DESC,
    ((attack_losses + defense_losses)) ASC,
    user_id ASC
  );
