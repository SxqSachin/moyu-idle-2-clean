SET lock_timeout = '5s';

ALTER TABLE arena_challenges
  DROP COLUMN IF EXISTS rating_eligible;
