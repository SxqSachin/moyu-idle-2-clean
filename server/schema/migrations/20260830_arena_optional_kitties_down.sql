SET lock_timeout = '5s';

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM arena_defenses
    WHERE cardinality(kitty_ids) <> 2
  ) THEN
    RAISE EXCEPTION 'cannot restore two-kitty arena constraint while shorter formations exist';
  END IF;
END $$;

ALTER TABLE arena_defenses
  DROP CONSTRAINT IF EXISTS arena_defenses_up_to_two_distinct_kitties;
ALTER TABLE arena_defenses
  DROP CONSTRAINT IF EXISTS arena_defenses_valid_kitty_ids;
ALTER TABLE arena_defenses
  ADD CONSTRAINT arena_defenses_two_distinct_kitties
  CHECK (
    cardinality(kitty_ids) = 2
    AND array_position(kitty_ids, NULL) IS NULL
    AND kitty_ids[1] > 0
    AND kitty_ids[2] > 0
    AND kitty_ids[1] <> kitty_ids[2]
  );
