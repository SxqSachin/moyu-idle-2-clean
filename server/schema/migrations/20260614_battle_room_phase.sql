-- Battle room phase + schema_version columns
-- Supports state machine v2: phase tracks sub-state within status='playing',
-- schema_version partitions rooms between v1 (legacy) and v2 dispatch loops.

-- 1) Add phase column (backend-only, not exposed to API/frontend)
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT '';

-- 2) Add schema_version column (1=legacy, 2=v2 state machine)
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS schema_version INT NOT NULL DEFAULT 1;

-- 3) Backfill phase for existing playing rooms based on current run status.
--    Non-playing rooms keep phase='' (default). Idempotent.
UPDATE battle_rooms r
SET phase = CASE
  WHEN r.current_run_id IS NULL THEN 'awaiting_run'
  WHEN cr.status = 'compiled' THEN 'compiling'
  WHEN cr.status = 'playing' THEN 'playback'
  WHEN cr.status = 'settling' THEN 'settling'
  ELSE ''
END
FROM battle_runs cr
WHERE cr.id = r.current_run_id AND r.status = 'playing';

-- 4) Composite index for v2 claim queries (version + status + phase + scheduling).
CREATE INDEX IF NOT EXISTS idx_battle_rooms_version_phase_claim
  ON battle_rooms(schema_version, status, phase, next_tick_at, lease_until, id);
