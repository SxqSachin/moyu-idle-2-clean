UPDATE player_kitties
SET extra_jsonb = (COALESCE(extra_jsonb, '{}'::jsonb) - 'personalityId' - 'personalityIds')
WHERE COALESCE(extra_jsonb, '{}'::jsonb) ?| ARRAY['personalityId', 'personalityIds'];
