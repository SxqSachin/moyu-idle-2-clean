ALTER TABLE player_kitties
  ADD COLUMN IF NOT EXISTS battle_level_cap BIGINT NOT NULL DEFAULT 100;

UPDATE player_kitties
SET battle_level_cap = 100
WHERE battle_level_cap IS NULL OR battle_level_cap <= 0;

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_battle_level_cap_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_battle_level_cap_check
  CHECK (battle_level_cap >= 1);
