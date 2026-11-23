ALTER TABLE battle_rooms ADD COLUMN IF NOT EXISTS difficulty_level INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_battle_rooms_difficulty_level ON battle_rooms(difficulty_level, dungeon_id, updated_at DESC);
