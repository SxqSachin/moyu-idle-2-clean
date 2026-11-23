-- Must be applied outside an explicit transaction block.
-- PostgreSQL does not allow CREATE INDEX CONCURRENTLY inside BEGIN/COMMIT.
-- Lease fields stay out of these indexes because every successful claim updates them.
-- After the v2 claim-query release is active, apply the matching cleanup migration
-- to remove idx_battle_rooms_tick_claim and idx_battle_rooms_version_phase_claim.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_battle_rooms_v1_status_due
  ON battle_rooms(status, next_tick_at, updated_at, id)
  WHERE schema_version = 1;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_battle_rooms_v2_playing_phase_due
  ON battle_rooms(phase, next_tick_at, updated_at, id)
  WHERE schema_version = 2 AND status = 'playing';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_battle_rooms_v2_state_due
  ON battle_rooms(next_tick_at, updated_at, id)
  WHERE schema_version = 2
    AND phase = ''
    AND status IN ('entering', 'recovering');
