-- Preserve player-visible records across the fishing-v1 -> fishing-v2 rollout,
-- and keep the already-open 2026-08-31 Emberfin tournament as this week's
-- authoritative tournament. This migration is intentionally idempotent.
--
-- Historical trips, catches, and fish instances keep their original ruleset
-- provenance. Only the read-model records and the current tournament identity
-- are promoted to fishing-v2.
DO $migration$
DECLARE
  legacy_tournament_id      TEXT;
  legacy_target_species_id  TEXT;
  legacy_status             TEXT;
  legacy_ends_at            TIMESTAMPTZ;
  current_tournament_id     TEXT;
  current_target_species_id TEXT;
  current_status            TEXT;
  current_ends_at           TIMESTAMPTZ;
BEGIN
  -- Block concurrent record/tournament writers for the duration of this one
  -- statement so the record merge and tournament promotion stay atomic.
  LOCK TABLE fishing_records IN ACCESS EXCLUSIVE MODE;
  LOCK TABLE fishing_tournaments IN ACCESS EXCLUSIVE MODE;
  LOCK TABLE fishing_tournament_entries IN ACCESS EXCLUSIVE MODE;
  LOCK TABLE fishing_tournament_rewards IN ACCESS EXCLUSIVE MODE;

  -- Keep the better value independently for length and weight. Existing v2
  -- records win ties; v1 timestamps are retained when both v1 records win.
  INSERT INTO fishing_records AS current_record (
    user_id,
    species_id,
    ruleset_version,
    best_length_cm,
    best_length_catch_id,
    best_weight_kg,
    best_weight_catch_id,
    updated_at
  )
  SELECT
    legacy.user_id,
    legacy.species_id,
    'fishing-v2',
    legacy.best_length_cm,
    legacy.best_length_catch_id,
    legacy.best_weight_kg,
    legacy.best_weight_catch_id,
    legacy.updated_at
  FROM fishing_records legacy
  WHERE legacy.ruleset_version = 'fishing-v1'
  ON CONFLICT (user_id, species_id, ruleset_version) DO UPDATE
  SET best_length_cm = GREATEST(current_record.best_length_cm, EXCLUDED.best_length_cm),
      best_length_catch_id = CASE
        WHEN EXCLUDED.best_length_cm > current_record.best_length_cm
          THEN EXCLUDED.best_length_catch_id
        WHEN EXCLUDED.best_length_cm = current_record.best_length_cm
             AND current_record.best_length_catch_id IS NULL
          THEN EXCLUDED.best_length_catch_id
        ELSE current_record.best_length_catch_id
      END,
      best_weight_kg = GREATEST(current_record.best_weight_kg, EXCLUDED.best_weight_kg),
      best_weight_catch_id = CASE
        WHEN EXCLUDED.best_weight_kg > current_record.best_weight_kg
          THEN EXCLUDED.best_weight_catch_id
        WHEN EXCLUDED.best_weight_kg = current_record.best_weight_kg
             AND current_record.best_weight_catch_id IS NULL
          THEN EXCLUDED.best_weight_catch_id
        ELSE current_record.best_weight_catch_id
      END,
      updated_at = CASE
        WHEN EXCLUDED.best_length_cm > current_record.best_length_cm
             AND EXCLUDED.best_weight_kg > current_record.best_weight_kg
          THEN EXCLUDED.updated_at
        WHEN EXCLUDED.best_length_cm <= current_record.best_length_cm
             AND EXCLUDED.best_weight_kg <= current_record.best_weight_kg
          THEN current_record.updated_at
        ELSE GREATEST(current_record.updated_at, EXCLUDED.updated_at)
      END;

  SELECT id, target_species_id, status, ends_at
  INTO legacy_tournament_id, legacy_target_species_id, legacy_status, legacy_ends_at
  FROM fishing_tournaments
  WHERE week_key = '2026-08-31'
    AND ruleset_version = 'fishing-v1';

  SELECT id, target_species_id, status, ends_at
  INTO current_tournament_id, current_target_species_id, current_status, current_ends_at
  FROM fishing_tournaments
  WHERE week_key = '2026-08-31'
    AND ruleset_version = 'fishing-v2';

  -- No tournament rows is the expected state for fresh databases.
  IF legacy_tournament_id IS NULL THEN
    IF current_tournament_id IS NULL
       OR current_ends_at <= now()
       OR current_target_species_id = 'emberfin'
    THEN
      RETURN;
    END IF;
    IF current_status <> 'open'
       OR EXISTS (
         SELECT 1
         FROM fishing_tournament_rewards
         WHERE tournament_id = current_tournament_id
       )
    THEN
      RAISE EXCEPTION
        'the fishing-v2 tournament is no longer safe to retarget';
    END IF;

    -- There is no previous leaderboard to recover. Keep the current row but
    -- discard entries scored for the unintended species before restoring the
    -- requested target.
    DELETE FROM fishing_tournament_entries
    WHERE tournament_id = current_tournament_id;

    UPDATE fishing_tournaments
    SET target_species_id = 'emberfin'
    WHERE id = current_tournament_id;
    RETURN;
  END IF;

  -- The repair is scoped to the still-active season only. Applying this file
  -- later must not rewrite finalized historical tournaments.
  IF legacy_ends_at <= now() THEN
    RETURN;
  END IF;
  IF legacy_target_species_id <> 'emberfin' OR legacy_status <> 'open' THEN
    RAISE EXCEPTION
      'expected the active fishing-v1 tournament to be open Emberfin, got target=% status=%',
      legacy_target_species_id,
      legacy_status;
  END IF;

  IF current_tournament_id IS NOT NULL THEN
    IF current_status <> 'open'
       OR current_ends_at <= now()
       OR EXISTS (
         SELECT 1
         FROM fishing_tournament_rewards
         WHERE tournament_id = current_tournament_id
       )
    THEN
      RAISE EXCEPTION
        'the fishing-v2 duplicate tournament is no longer safe to supersede';
    END IF;

    -- Preserve the accidental v2 entries for audit, but make the duplicate
    -- ineligible for automatic finalization and rewards.
    UPDATE fishing_tournaments
    SET ruleset_version = 'fishing-v2-superseded-20260903',
        status = 'finalized',
        finalized_at = now()
    WHERE id = current_tournament_id;
  END IF;

  -- Keep the original tournament id so all Emberfin entries and their standard
  -- qualification seeds remain unchanged. The current server now finds it via
  -- its fishing-v2 lookup; the next week is generated normally.
  UPDATE fishing_tournaments
  SET ruleset_version = 'fishing-v2'
  WHERE id = legacy_tournament_id;
END
$migration$;
