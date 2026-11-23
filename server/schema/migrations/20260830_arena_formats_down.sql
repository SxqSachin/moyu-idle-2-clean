DROP INDEX IF EXISTS idx_arena_defenses_active_format;

ALTER TABLE arena_challenges
  DROP COLUMN IF EXISTS format_snapshot_jsonb;
