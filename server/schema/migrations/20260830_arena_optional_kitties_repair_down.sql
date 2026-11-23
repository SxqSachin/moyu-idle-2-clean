SET lock_timeout = '5s';

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM arena_defenses
    WHERE cardinality(kitty_ids) > 2
  ) THEN
    RAISE EXCEPTION 'cannot restore the two-kitty limit while larger formations exist';
  END IF;
END $$;

ALTER TABLE arena_defenses
  DROP CONSTRAINT IF EXISTS arena_defenses_valid_kitty_ids;
ALTER TABLE arena_defenses
  ADD CONSTRAINT arena_defenses_up_to_two_distinct_kitties
  CHECK (
    cardinality(kitty_ids) BETWEEN 0 AND 2
    AND public.arena_kitty_ids_are_valid(kitty_ids)
  );
