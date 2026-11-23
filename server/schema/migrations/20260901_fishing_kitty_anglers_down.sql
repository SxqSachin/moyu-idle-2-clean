ALTER TABLE fishing_trips
  DROP CONSTRAINT IF EXISTS fishing_trips_actor_id_check,
  DROP CONSTRAINT IF EXISTS fishing_trips_actor_type_check,
  DROP COLUMN IF EXISTS actor_id,
  DROP COLUMN IF EXISTS actor_type;

DELETE FROM player_kitty_equip_slots
WHERE equip_type = 'work';

UPDATE player_item_instances AS item
SET state = 'inventory', updated_at = now()
WHERE item.state = 'equipped'
  AND NOT EXISTS (SELECT 1 FROM player_equip_slots AS slot WHERE slot.instance_id = item.instance_id)
  AND NOT EXISTS (SELECT 1 FROM player_kitty_equip_slots AS slot WHERE slot.instance_id = item.instance_id);

ALTER TABLE player_kitty_equip_slots
  DROP CONSTRAINT IF EXISTS player_kitty_equip_slots_equip_type_check;

ALTER TABLE player_kitty_equip_slots
  ADD CONSTRAINT player_kitty_equip_slots_equip_type_check
  CHECK(equip_type IN ('battle'));

DROP INDEX IF EXISTS uq_player_kitty_equip_slots_instance;
CREATE UNIQUE INDEX uq_player_kitty_equip_slots_instance
  ON player_kitty_equip_slots(instance_id)
  WHERE equip_type = 'battle';
