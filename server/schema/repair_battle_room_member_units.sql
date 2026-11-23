BEGIN;

-- 1. 删除 owner 已不在 battle_room_members 中的孤儿单位。
DELETE FROM battle_room_member_units u
WHERE NOT EXISTS (
  SELECT 1
  FROM battle_room_members m
  WHERE m.room_id = u.room_id
    AND m.user_id = u.owner_user_id
);

-- 2. 为仍在房间中的成员补齐 player unit。
INSERT INTO battle_room_member_units (
  room_id,
  owner_user_id,
  unit_type,
  unit_id,
  kitty_id,
  position,
  is_ready,
  joined_at,
  updated_at
)
SELECT
  m.room_id,
  m.user_id,
  'player',
  0,
  NULL,
  m.position,
  m.is_ready,
  m.joined_at,
  now()
FROM battle_room_members m
WHERE NOT EXISTS (
  SELECT 1
  FROM battle_room_member_units u
  WHERE u.room_id = m.room_id
    AND u.owner_user_id = m.user_id
    AND u.unit_type = 'player'
    AND u.unit_id = 0
);

-- 3. 把 player unit 的站位 / ready 状态回填为 member 当前值。
UPDATE battle_room_member_units u
SET position = m.position,
    is_ready = m.is_ready,
    updated_at = now()
FROM battle_room_members m
WHERE u.room_id = m.room_id
  AND u.owner_user_id = m.user_id
  AND u.unit_type = 'player'
  AND u.unit_id = 0
  AND (
    u.position IS DISTINCT FROM m.position
    OR u.is_ready IS DISTINCT FROM m.is_ready
  );

COMMIT;
