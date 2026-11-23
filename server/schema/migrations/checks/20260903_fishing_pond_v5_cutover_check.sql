-- Read-only post-migration check. unsupported_ruleset_rows and
-- invalid_pond_zone_rows must both be zero. legacy_pending_rows may remain
-- non-zero until dormant players next open or mutate their pond.
DO $$
DECLARE
  unsupported_rule_count BIGINT;
  invalid_zone_count BIGINT;
BEGIN
  SELECT COUNT(*)
  INTO unsupported_rule_count
  FROM fishing_ponds
  WHERE COALESCE(ruleset_version, '') NOT IN ('fishing-pond-v4', 'fishing-pond-v5');

  SELECT COUNT(*)
  INTO invalid_zone_count
  FROM player_item_instances
  WHERE state = 'fishing_pond'
    AND template_item_id = 'fish_catch'
    AND COALESCE(fish_data->>'pondZone', '') NOT IN ('main', 'nursery');

  IF unsupported_rule_count > 0 OR invalid_zone_count > 0 THEN
    RAISE EXCEPTION 'fishing pond V5 cutover check failed: unsupported rulesets=%, invalid zones=%',
      unsupported_rule_count, invalid_zone_count;
  END IF;
END $$;

SELECT COUNT(*) AS unsupported_ruleset_rows
FROM fishing_ponds
WHERE COALESCE(ruleset_version, '') NOT IN ('fishing-pond-v4', 'fishing-pond-v5');

SELECT COUNT(*) AS invalid_pond_zone_rows
FROM player_item_instances
WHERE state = 'fishing_pond'
  AND template_item_id = 'fish_catch'
  AND COALESCE(fish_data->>'pondZone', '') NOT IN ('main', 'nursery');

SELECT COUNT(*) FILTER (WHERE ruleset_version = 'fishing-pond-v4') AS legacy_pending_rows,
       COUNT(*) FILTER (WHERE ruleset_version = 'fishing-pond-v5') AS current_rows
FROM fishing_ponds;

SELECT COUNT(*) AS nursery_fish_rows
FROM player_item_instances
WHERE state = 'fishing_pond'
  AND template_item_id = 'fish_catch'
  AND fish_data->>'pondZone' = 'nursery';
