SET lock_timeout = '5s';

DROP TABLE IF EXISTS arena_shop_orders;
DROP TABLE IF EXISTS arena_replay_shares;
DROP TABLE IF EXISTS arena_challenge_payloads;
DROP TABLE IF EXISTS arena_challenges;
DROP TABLE IF EXISTS arena_defenses;
DROP FUNCTION IF EXISTS public.arena_kitty_ids_are_valid(BIGINT[]);
DROP TABLE IF EXISTS arena_player_states;
ALTER TABLE chat_messages DROP COLUMN IF EXISTS attachment_jsonb;
