CREATE TABLE IF NOT EXISTS player_kitty_personalities (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kitty_id BIGINT NOT NULL REFERENCES player_kitties(id) ON DELETE CASCADE,
  personality_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kitty_id, personality_id)
);

CREATE INDEX IF NOT EXISTS idx_player_kitty_personalities_user_kitty
  ON player_kitty_personalities(user_id, kitty_id, created_at, personality_id);

INSERT INTO player_kitty_personalities(user_id, kitty_id, personality_id, created_at)
SELECT
  pk.user_id,
  pk.id AS kitty_id,
  lower(trim(source.personality_id)) AS personality_id,
  now()
FROM player_kitties pk
JOIN LATERAL (
  SELECT DISTINCT value AS personality_id
  FROM (
    SELECT jsonb_array_elements_text(
      CASE
        WHEN jsonb_typeof(pk.extra_jsonb -> 'personalityIds') = 'array' THEN pk.extra_jsonb -> 'personalityIds'
        ELSE '[]'::jsonb
      END
    ) AS value
    UNION ALL
    SELECT nullif(trim(pk.extra_jsonb ->> 'personalityId'), '') AS value
  ) merged
  WHERE value IS NOT NULL AND trim(value) <> ''
) source ON TRUE
ON CONFLICT (kitty_id, personality_id) DO NOTHING;
