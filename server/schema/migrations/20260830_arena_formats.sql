SET lock_timeout = '5s';

ALTER TABLE arena_challenges
  ADD COLUMN IF NOT EXISTS format_snapshot_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_arena_defenses_active_format
  ON arena_defenses((selection_jsonb->>'formatId'), user_id)
  WHERE active = TRUE;
