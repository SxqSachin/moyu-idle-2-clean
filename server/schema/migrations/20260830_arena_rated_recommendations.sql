SET lock_timeout = '5s';

ALTER TABLE arena_challenges
  ADD COLUMN IF NOT EXISTS rating_eligible BOOLEAN NOT NULL DEFAULT FALSE;
