-- Bound the daily "more teams" sample around an indexed active-player pivot.

CREATE INDEX IF NOT EXISTS idx_arena_defenses_active_user
  ON arena_defenses(user_id)
  WHERE active = TRUE;
