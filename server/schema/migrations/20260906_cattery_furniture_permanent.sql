BEGIN;
-- NULL is an unbounded service interval, not an artificial expiry date.
ALTER TABLE player_cattery_furniture_periods ALTER COLUMN valid_until DROP NOT NULL;
COMMIT;
