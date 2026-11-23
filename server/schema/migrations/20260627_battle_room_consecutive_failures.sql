ALTER TABLE battle_rooms
  ADD COLUMN IF NOT EXISTS consecutive_failures INT NOT NULL DEFAULT 0;

ALTER TABLE battle_rooms
  DROP CONSTRAINT IF EXISTS battle_rooms_consecutive_failures_check;

ALTER TABLE battle_rooms
  ADD CONSTRAINT battle_rooms_consecutive_failures_check
  CHECK (consecutive_failures >= 0);
