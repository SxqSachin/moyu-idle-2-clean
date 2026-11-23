BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE player_item_instances
  DROP CONSTRAINT IF EXISTS chk_player_item_instances_fish_shape;

ALTER TABLE player_item_instances
  ADD CONSTRAINT chk_player_item_instances_fish_shape
  CHECK (
    fish_data = '{}'::jsonb OR (
      template_item_id = 'fish_catch'
      AND quantity = 1
      AND enhance_level = 0
      AND affixes = '[]'::jsonb
      AND custom_data = '{}'::jsonb
    )
  ) NOT VALID;

COMMIT;
