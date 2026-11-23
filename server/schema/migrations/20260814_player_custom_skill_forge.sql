BEGIN;

CREATE TABLE IF NOT EXISTS player_custom_skills (
  id TEXT PRIMARY KEY CHECK(id LIKE 'custom:%'),
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_kind TEXT NOT NULL CHECK(source_kind IN ('blank', 'official')),
  source_skill_id TEXT NOT NULL DEFAULT '',
  source_definition_hash TEXT NOT NULL DEFAULT '',
  current_revision_id TEXT,
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, id)
);
CREATE INDEX IF NOT EXISTS idx_player_custom_skills_user_active
  ON player_custom_skills(user_id, archived_at, created_at);
CREATE INDEX IF NOT EXISTS idx_player_custom_skills_user_source
  ON player_custom_skills(user_id, source_skill_id)
  WHERE archived_at IS NULL AND source_skill_id <> '';

CREATE TABLE IF NOT EXISTS player_custom_skill_drafts (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  custom_skill_id TEXT REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  source_kind TEXT NOT NULL CHECK(source_kind IN ('blank', 'official')),
  source_skill_id TEXT NOT NULL DEFAULT '',
  source_skillbook_item_id TEXT NOT NULL DEFAULT '',
  source_skillbook_count INT NOT NULL DEFAULT 0 CHECK(source_skillbook_count >= 0),
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'completed')),
  source_definition_hash TEXT NOT NULL DEFAULT '',
  graph_jsonb JSONB NOT NULL,
  graph_version INT NOT NULL DEFAULT 1 CHECK(graph_version = 1),
  revision_no BIGINT NOT NULL DEFAULT 1 CHECK(revision_no > 0),
  pending_revision_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_drafts_user_updated
  ON player_custom_skill_drafts(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_drafts_user_active_updated
  ON player_custom_skill_drafts(user_id, updated_at DESC)
  WHERE status = 'active';

CREATE TABLE IF NOT EXISTS player_custom_skill_revisions (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  custom_skill_id TEXT NOT NULL REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  draft_id TEXT REFERENCES player_custom_skill_drafts(id) ON DELETE SET NULL,
  revision_no BIGINT NOT NULL CHECK(revision_no > 0),
  source_skill_id TEXT NOT NULL DEFAULT '',
  source_definition_hash TEXT NOT NULL DEFAULT '',
  graph_jsonb JSONB NOT NULL,
  compiled_skill_jsonb JSONB NOT NULL,
  recipe_jsonb JSONB NOT NULL,
  requested_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  consumed_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  refunded_recipe_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_skillbook_item_id TEXT NOT NULL DEFAULT '',
  source_skillbook_count INT NOT NULL DEFAULT 0,
  budget_jsonb JSONB NOT NULL,
  rolls_jsonb JSONB NOT NULL,
  failed BOOLEAN NOT NULL DEFAULT FALSE,
  result_status TEXT NOT NULL CHECK(result_status IN ('pending', 'accepted', 'discarded', 'superseded', 'failed')),
  compiler_version INT NOT NULL CHECK(compiler_version > 0),
  ruleset_version INT NOT NULL DEFAULT 1 CHECK(ruleset_version > 0),
  idempotency_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  UNIQUE(user_id, idempotency_key),
  UNIQUE(custom_skill_id, revision_no)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_one_pending
  ON player_custom_skill_revisions(draft_id)
  WHERE result_status = 'pending';
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_skill_created
  ON player_custom_skill_revisions(custom_skill_id, revision_no DESC);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_ruleset
  ON player_custom_skill_revisions(ruleset_version, result_status, user_id);
CREATE INDEX IF NOT EXISTS idx_player_custom_skill_revisions_user_pending_created
  ON player_custom_skill_revisions(user_id, created_at)
  WHERE result_status = 'pending';

CREATE TABLE IF NOT EXISTS player_forged_skillbooks (
  id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  instance_id BIGINT UNIQUE REFERENCES player_item_instances(instance_id) ON DELETE SET NULL,
  custom_skill_id TEXT NOT NULL UNIQUE REFERENCES player_custom_skills(id) ON DELETE CASCADE,
  revision_id TEXT NOT NULL UNIQUE REFERENCES player_custom_skill_revisions(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'inventory' CHECK(status IN ('inventory', 'learned_player', 'learned_kitty', 'destroyed', 'archived')),
  target_kitty_id BIGINT,
  learned_at TIMESTAMPTZ,
  destroyed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_forged_skillbooks_user_active
  ON player_forged_skillbooks(user_id, status, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'player_custom_skills_current_revision_fk') THEN
    ALTER TABLE player_custom_skills
      ADD CONSTRAINT player_custom_skills_current_revision_fk
      FOREIGN KEY(current_revision_id) REFERENCES player_custom_skill_revisions(id) ON DELETE SET NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'player_custom_skill_drafts_pending_revision_fk') THEN
    ALTER TABLE player_custom_skill_drafts
      ADD CONSTRAINT player_custom_skill_drafts_pending_revision_fk
      FOREIGN KEY(pending_revision_id) REFERENCES player_custom_skill_revisions(id) ON DELETE SET NULL;
  END IF;
END $$;

COMMIT;
