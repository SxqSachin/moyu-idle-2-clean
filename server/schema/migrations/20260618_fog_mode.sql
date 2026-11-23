-- Fog mode for battle rooms
-- Adds is_fog_mode toggle to battle_rooms and a member snapshot table
-- used for cross-room player phantom encounters.

-- 1) Fog mode toggle on battle_rooms
ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS is_fog_mode BOOLEAN NOT NULL DEFAULT FALSE;

-- 2) Fog room member snapshots (compiled battle units stored at dungeon entry)
CREATE TABLE IF NOT EXISTS fog_room_member_snapshots (
  room_id    BIGINT       NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  user_id    BIGINT       NOT NULL REFERENCES users(id),
  dungeon_id TEXT         NOT NULL,  -- denormalised for efficient same-dungeon queries
  snapshot   JSONB        NOT NULL,  -- []battleUnitState compiled at entry time
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (room_id, user_id)
);

-- 3) Index for querying active fog rooms in the same dungeon
CREATE INDEX IF NOT EXISTS idx_fog_snapshots_dungeon
  ON fog_room_member_snapshots(dungeon_id);

-- 4) Partial index on battle_rooms to speed up active-fog-room look-ups
CREATE INDEX IF NOT EXISTS idx_battle_rooms_fog_playing
  ON battle_rooms(dungeon_id, status)
  WHERE is_fog_mode = TRUE;
