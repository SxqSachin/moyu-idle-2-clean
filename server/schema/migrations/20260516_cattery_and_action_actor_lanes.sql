CREATE TABLE IF NOT EXISTS player_kitties (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_kitties_user_created
  ON player_kitties(user_id, created_at, id);

ALTER TABLE player_action_queue
  ADD COLUMN IF NOT EXISTS actor_type TEXT NOT NULL DEFAULT 'player';

ALTER TABLE player_action_queue
  ADD COLUMN IF NOT EXISTS actor_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE player_action_queue
  DROP CONSTRAINT IF EXISTS player_action_queue_actor_type_check;

ALTER TABLE player_action_queue
  ADD CONSTRAINT player_action_queue_actor_type_check
  CHECK (actor_type IN ('player', 'kitty'));

ALTER TABLE player_action_queue
  DROP CONSTRAINT IF EXISTS player_action_queue_actor_id_check;

ALTER TABLE player_action_queue
  ADD CONSTRAINT player_action_queue_actor_id_check
  CHECK (actor_id >= 0);

UPDATE player_action_queue
SET actor_type = 'player'
WHERE actor_type IS NULL OR actor_type = '';

UPDATE player_action_queue
SET actor_id = 0
WHERE actor_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_player_action_queue_user_actor_status_order
  ON player_action_queue(user_id, actor_type, actor_id, status, queue_order, id);
