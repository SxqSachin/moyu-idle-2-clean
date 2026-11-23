BEGIN;

ALTER TABLE player_battle_item_loadout
  ADD COLUMN IF NOT EXISTS slot_no INT;

WITH ranked AS (
  SELECT
    user_id,
    item_id,
    ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at ASC, item_id ASC) AS rn
  FROM player_battle_item_loadout
)
UPDATE player_battle_item_loadout t
SET slot_no = r.rn
FROM ranked r
WHERE t.user_id = r.user_id
  AND t.item_id = r.item_id
  AND t.slot_no IS NULL;

ALTER TABLE player_battle_item_loadout
  ALTER COLUMN slot_no SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'player_battle_item_loadout_slot_no_check'
  ) THEN
    ALTER TABLE player_battle_item_loadout
      ADD CONSTRAINT player_battle_item_loadout_slot_no_check CHECK (slot_no > 0);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'player_battle_item_loadout_user_slot_no_key'
  ) THEN
    ALTER TABLE player_battle_item_loadout
      ADD CONSTRAINT player_battle_item_loadout_user_slot_no_key UNIQUE (user_id, slot_no);
  END IF;
END $$;

DROP INDEX IF EXISTS idx_player_battle_item_loadout_user_updated;
CREATE INDEX IF NOT EXISTS idx_player_battle_item_loadout_user_updated
  ON player_battle_item_loadout(user_id, slot_no, updated_at, item_id);

COMMIT;
