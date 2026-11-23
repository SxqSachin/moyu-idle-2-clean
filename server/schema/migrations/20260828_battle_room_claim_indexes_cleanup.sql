-- Apply only after 20260828_battle_room_claim_indexes.sql has completed and
-- the v2 claim-query release is running. Must stay outside a transaction block.
-- Removing lease-bearing indexes allows owner/lease-only claims to use HOT updates.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_index
    WHERE indexrelid = to_regclass('idx_battle_rooms_v1_status_due')
      AND indisready
      AND indisvalid
  ) THEN
    RAISE EXCEPTION 'replacement index idx_battle_rooms_v1_status_due is missing, not ready, or invalid';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_index
    WHERE indexrelid = to_regclass('idx_battle_rooms_v2_playing_phase_due')
      AND indisready
      AND indisvalid
  ) THEN
    RAISE EXCEPTION 'replacement index idx_battle_rooms_v2_playing_phase_due is missing, not ready, or invalid';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_index
    WHERE indexrelid = to_regclass('idx_battle_rooms_v2_state_due')
      AND indisready
      AND indisvalid
  ) THEN
    RAISE EXCEPTION 'replacement index idx_battle_rooms_v2_state_due is missing, not ready, or invalid';
  END IF;
END $$;

DROP INDEX CONCURRENTLY IF EXISTS idx_battle_rooms_tick_claim;
DROP INDEX CONCURRENTLY IF EXISTS idx_battle_rooms_version_phase_claim;
