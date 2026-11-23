ALTER TABLE player_kitties
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'idle';

ALTER TABLE player_kitties
  DROP CONSTRAINT IF EXISTS player_kitties_status_check;
ALTER TABLE player_kitties
  ADD CONSTRAINT player_kitties_status_check
  CHECK (status IN ('idle', 'in_life_queue', 'in_combat'));

CREATE TABLE IF NOT EXISTS player_kitty_attributes (
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  attr_key TEXT NOT NULL,
  base_level BIGINT NOT NULL DEFAULT 1 CHECK(base_level > 0),
  skill_point_level BIGINT NOT NULL DEFAULT 0 CHECK(skill_point_level >= 0),
  attr_exp DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK(attr_exp >= 0),
  source_mods_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  final_value DOUBLE PRECISION NOT NULL DEFAULT 0,
  lock_version BIGINT NOT NULL DEFAULT 1 CHECK(lock_version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kitty_id, attr_key)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_attributes_kitty ON player_kitty_attributes(kitty_id);

CREATE TABLE IF NOT EXISTS player_kitty_stats_cache (
  kitty_id BIGINT PRIMARY KEY REFERENCES player_kitties(id) ON DELETE CASCADE,
  stats_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_kitty_skills (
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  skill_id TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  cast_order INT NOT NULL DEFAULT 0,
  rule_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  rule_groups_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  level INT NOT NULL DEFAULT 1 CHECK(level >= 1),
  exp BIGINT NOT NULL DEFAULT 0 CHECK(exp >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(kitty_id, skill_id)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_skills_kitty_order
  ON player_kitty_skills(kitty_id, cast_order, skill_id);

CREATE TABLE IF NOT EXISTS player_kitty_equip_slots (
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  equip_type TEXT NOT NULL CHECK(equip_type IN ('battle')),
  slot TEXT NOT NULL,
  instance_id BIGINT NOT NULL REFERENCES player_item_instances(instance_id) ON DELETE CASCADE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kitty_id, equip_type, slot)
);
CREATE INDEX IF NOT EXISTS idx_player_kitty_equip_slots_kitty_instance
  ON player_kitty_equip_slots(kitty_id, instance_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_player_kitty_equip_slots_instance
  ON player_kitty_equip_slots(instance_id)
  WHERE equip_type = 'battle';

CREATE UNIQUE INDEX IF NOT EXISTS uq_player_equip_slots_battle_instance
  ON player_equip_slots(instance_id)
  WHERE equip_type = 'battle';

CREATE TABLE IF NOT EXISTS battle_room_member_units (
  room_id BIGINT NOT NULL REFERENCES battle_rooms(id) ON DELETE CASCADE,
  owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  unit_type TEXT NOT NULL CHECK(unit_type IN ('player', 'kitty')),
  unit_id BIGINT NOT NULL DEFAULT 0 CHECK(unit_id >= 0),
  kitty_id BIGINT REFERENCES player_kitties(id) ON DELETE CASCADE,
  position TEXT NOT NULL DEFAULT 'vanguard' CHECK(position IN ('vanguard', 'midguard', 'rearguard')),
  is_ready BOOLEAN NOT NULL DEFAULT FALSE,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(room_id, owner_user_id, unit_type, unit_id),
  CHECK (
    (unit_type = 'player' AND unit_id = 0 AND kitty_id IS NULL)
    OR
    (unit_type = 'kitty' AND unit_id > 0 AND kitty_id = unit_id)
  )
);

ALTER TABLE battle_room_member_units
  ADD COLUMN IF NOT EXISTS unit_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE battle_room_member_units
  DROP CONSTRAINT IF EXISTS battle_room_member_units_pkey;

ALTER TABLE battle_room_member_units
  ALTER COLUMN kitty_id DROP NOT NULL;

UPDATE battle_room_member_units
SET unit_id = CASE WHEN unit_type = 'kitty' THEN COALESCE(kitty_id, 0) ELSE 0 END
WHERE unit_id IS NULL OR unit_id = 0;

ALTER TABLE battle_room_member_units
  DROP CONSTRAINT IF EXISTS battle_room_member_units_unit_shape_check;
ALTER TABLE battle_room_member_units
  ADD CONSTRAINT battle_room_member_units_unit_shape_check
  CHECK (
    (unit_type = 'player' AND unit_id = 0 AND kitty_id IS NULL)
    OR
    (unit_type = 'kitty' AND unit_id > 0 AND kitty_id = unit_id)
  );

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'battle_room_member_units_pkey'
  ) THEN
    ALTER TABLE battle_room_member_units
      ADD CONSTRAINT battle_room_member_units_pkey
      PRIMARY KEY(room_id, owner_user_id, unit_type, unit_id);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_battle_room_member_units_kitty_active
  ON battle_room_member_units(kitty_id)
  WHERE unit_type = 'kitty' AND kitty_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_battle_room_member_units_room
  ON battle_room_member_units(room_id, joined_at, owner_user_id);

CREATE INDEX IF NOT EXISTS idx_battle_room_member_units_owner
  ON battle_room_member_units(owner_user_id, room_id, unit_type);

INSERT INTO battle_room_member_units(room_id, owner_user_id, unit_type, unit_id, kitty_id, position, is_ready, joined_at, updated_at)
SELECT m.room_id, m.user_id, 'player', 0, NULL, m.position, m.is_ready, m.joined_at, m.updated_at
FROM battle_room_members m
WHERE NOT EXISTS (
  SELECT 1
  FROM battle_room_member_units u
  WHERE u.room_id = m.room_id
    AND u.owner_user_id = m.user_id
    AND u.unit_type = 'player'
    AND u.unit_id = 0
);
