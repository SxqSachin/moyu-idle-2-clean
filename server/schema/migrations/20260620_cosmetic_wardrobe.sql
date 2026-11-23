-- Cosmetic wardrobe system: ownership pool + equip slots (player + kitty)

-- Owned cosmetics (polymorphic: all categories share one table)
CREATE TABLE player_wardrobe_items (
  user_id      BIGINT      NOT NULL,
  category     TEXT        NOT NULL,
  item_id      TEXT        NOT NULL,
  source       TEXT        NOT NULL DEFAULT '',
  obtained_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, category, item_id)
);

-- Equip slots: supports both player and kitty as equip owner
-- UNIQUE(user_id, item_id) enforces mutual exclusion: same cosmetic can only be
-- equipped by one owner (player or kitty) at a time per user.
CREATE TABLE wardrobe_equip_slots (
  user_id      BIGINT      NOT NULL,
  owner_type   TEXT        NOT NULL,
  owner_id     BIGINT      NOT NULL DEFAULT 0,
  slot_id      TEXT        NOT NULL,
  item_id      TEXT        NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, owner_type, owner_id, slot_id),
  UNIQUE (user_id, item_id)
);

-- Chat messages: add cosmetic display columns.
-- title_prefix_id / title_suffix_id: dedicated columns for the two-part title.
-- display_meta: JSONB for future extensibility (avatar, avatar_frame, signature, etc.)
-- Example display_meta: {"avatarId": "...", "avatarFrameId": "..."}
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS title_prefix_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS title_suffix_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS display_meta JSONB NOT NULL DEFAULT '{}'::jsonb;
-- If the old single-column migration was already applied, drop it.
ALTER TABLE chat_messages DROP COLUMN IF EXISTS title;
