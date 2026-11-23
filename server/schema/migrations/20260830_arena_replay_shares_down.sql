SET lock_timeout = '5s';

ALTER TABLE chat_messages DROP COLUMN IF EXISTS attachment_jsonb;
DROP TABLE IF EXISTS arena_replay_shares;
