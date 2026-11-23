ALTER TABLE player_operation_effect_activations
  ADD COLUMN IF NOT EXISTS quantity BIGINT NOT NULL DEFAULT 1
  CHECK (quantity BETWEEN 1 AND 10000);
