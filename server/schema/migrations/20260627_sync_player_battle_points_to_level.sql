-- One-off repair after battle exp curve changes.
--
-- Rule:
--   level-earned battle points   = current battle base_level - 1
--   spent battle points          = SUM(player_attributes.skill_point_level)
--   target total battle points   = max(level-earned points, spent points)
--   available battle points      = target total - spent
--
-- This keeps already allocated historical points intact, but removes unspent
-- surplus points when old total_points is higher than the new level-earned
-- amount. Players below the new level-earned amount receive the missing
-- unallocated points.

BEGIN;

LOCK TABLE player_attributes IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE player_battle_point_state IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE _player_battle_point_repair ON COMMIT DROP AS
WITH battle_levels AS (
  SELECT
    pa.user_id,
    pa.base_level AS battle_level,
    GREATEST(pa.base_level - 1, 0)::BIGINT AS expected_total_points
  FROM player_attributes pa
  WHERE pa.attr_key = 'battle'
),
allocated_points AS (
  SELECT
    pa.user_id,
    COALESCE(SUM(pa.skill_point_level), 0)::BIGINT AS allocated_points
  FROM player_attributes pa
  GROUP BY pa.user_id
)
SELECT
  bl.user_id,
  bl.battle_level,
  bl.expected_total_points,
  COALESCE(ap.allocated_points, 0)::BIGINT AS allocated_points,
  GREATEST(bl.expected_total_points, COALESCE(ap.allocated_points, 0))::BIGINT AS target_total_points,
  COALESCE(bp.total_points, 0)::BIGINT AS old_total_points,
  COALESCE(bp.spent_points, 0)::BIGINT AS old_spent_points
FROM battle_levels bl
LEFT JOIN allocated_points ap ON ap.user_id = bl.user_id
LEFT JOIN player_battle_point_state bp ON bp.user_id = bl.user_id;

WITH upserted AS (
  INSERT INTO player_battle_point_state(user_id, total_points, spent_points, updated_at)
  SELECT
    user_id,
    target_total_points,
    allocated_points,
    now()
  FROM _player_battle_point_repair
  ON CONFLICT (user_id) DO UPDATE
  SET total_points = EXCLUDED.total_points,
      spent_points = EXCLUDED.spent_points,
      updated_at = now()
  WHERE player_battle_point_state.total_points IS DISTINCT FROM EXCLUDED.total_points
     OR player_battle_point_state.spent_points IS DISTINCT FROM EXCLUDED.spent_points
  RETURNING user_id
)
SELECT
  COUNT(*) AS repaired_players
FROM upserted;

SELECT
  COUNT(*) FILTER (WHERE target_total_points <> old_total_points OR allocated_points <> old_spent_points) AS changed_players,
  COUNT(*) FILTER (WHERE allocated_points > expected_total_points) AS grandfathered_players,
  COALESCE(SUM(GREATEST(target_total_points - old_total_points, 0)), 0)::BIGINT AS total_points_added,
  COALESCE(SUM(GREATEST(old_total_points - target_total_points, 0)), 0)::BIGINT AS unspent_surplus_removed
FROM _player_battle_point_repair;

COMMIT;
