CREATE TABLE IF NOT EXISTS battle_slow_run_archives (
  id BIGSERIAL PRIMARY KEY,
  room_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL,
  dungeon_id TEXT NOT NULL DEFAULT '',
  worker_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  wave INT NOT NULL DEFAULT 1 CHECK (wave > 0),
  outcome TEXT NOT NULL DEFAULT '',
  queue_since_start_ms BIGINT NOT NULL DEFAULT 0,
  compile_elapsed_ms BIGINT NOT NULL DEFAULT 0,
  total_ms BIGINT NOT NULL DEFAULT 0,
  total_steps INT NOT NULL DEFAULT 0,
  total_turns BIGINT NOT NULL DEFAULT 0,
  timeline_events INT NOT NULL DEFAULT 0,
  final_seq BIGINT NOT NULL DEFAULT 0,
  player_units INT NOT NULL DEFAULT 0,
  guard_timed_out BOOLEAN NOT NULL DEFAULT FALSE,
  guard_step_limited BOOLEAN NOT NULL DEFAULT FALSE,
  trigger_flags_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  turn_profile_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  compile_input_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  checkpoint_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  timeline_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  final_result_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  run_started_at TIMESTAMPTZ,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_battle_slow_run_archives_room_run
  ON battle_slow_run_archives(room_id, run_id);

CREATE INDEX IF NOT EXISTS idx_battle_slow_run_archives_room_captured
  ON battle_slow_run_archives(room_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_battle_slow_run_archives_run
  ON battle_slow_run_archives(run_id);

CREATE INDEX IF NOT EXISTS idx_battle_slow_run_archives_captured
  ON battle_slow_run_archives(captured_at DESC);
