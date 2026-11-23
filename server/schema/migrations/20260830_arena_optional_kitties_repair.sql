SET lock_timeout = '5s';

-- Existing deployments may still carry the original exactly-two-kitties check:
-- replaying init.sql cannot replace a constraint created by an older release.
CREATE OR REPLACE FUNCTION public.arena_kitty_ids_are_valid(candidate BIGINT[])
RETURNS BOOLEAN
LANGUAGE SQL
IMMUTABLE
PARALLEL SAFE
AS $$
  SELECT COALESCE(
    array_position($1, NULL) IS NULL
    AND 0::bigint < ALL($1)
    AND cardinality($1)::bigint = (
      SELECT count(DISTINCT kitty_id)
      FROM unnest($1) AS entries(kitty_id)
    ),
    FALSE
  );
$$;

ALTER TABLE arena_defenses
  DROP CONSTRAINT IF EXISTS arena_defenses_two_distinct_kitties,
  DROP CONSTRAINT IF EXISTS arena_defenses_up_to_two_distinct_kitties,
  DROP CONSTRAINT IF EXISTS arena_defenses_valid_kitty_ids,
  ADD CONSTRAINT arena_defenses_valid_kitty_ids
    CHECK (public.arena_kitty_ids_are_valid(kitty_ids)) NOT VALID;

ALTER TABLE arena_defenses
  VALIDATE CONSTRAINT arena_defenses_valid_kitty_ids;
