-- Fishing trips can be owned by the player or one of their kitties.
ALTER TABLE player_kitty_equip_slots
  DROP CONSTRAINT IF EXISTS player_kitty_equip_slots_equip_type_check;

ALTER TABLE player_kitty_equip_slots
  ADD CONSTRAINT player_kitty_equip_slots_equip_type_check
  CHECK(equip_type IN ('battle', 'work'));

DROP INDEX IF EXISTS uq_player_kitty_equip_slots_instance;
CREATE UNIQUE INDEX uq_player_kitty_equip_slots_instance
  ON player_kitty_equip_slots(instance_id);

ALTER TABLE fishing_trips
  ADD COLUMN IF NOT EXISTS actor_type TEXT NOT NULL DEFAULT 'player',
  ADD COLUMN IF NOT EXISTS actor_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE fishing_trips
  DROP CONSTRAINT IF EXISTS fishing_trips_actor_type_check,
  DROP CONSTRAINT IF EXISTS fishing_trips_actor_id_check;

ALTER TABLE fishing_trips
  ADD CONSTRAINT fishing_trips_actor_type_check
    CHECK(actor_type IN ('player', 'kitty')),
  ADD CONSTRAINT fishing_trips_actor_id_check
    CHECK((actor_type = 'player' AND actor_id = 0) OR (actor_type = 'kitty' AND actor_id > 0));
