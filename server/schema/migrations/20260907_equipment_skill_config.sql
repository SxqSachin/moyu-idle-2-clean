BEGIN;
ALTER TABLE player_skills ADD COLUMN IF NOT EXISTS equipment_config_jsonb JSONB;
DO $$ BEGIN
IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='player_skills'::regclass AND conname='player_skills_equipment_config_shape') THEN
ALTER TABLE player_skills ADD CONSTRAINT player_skills_equipment_config_shape CHECK (
  equipment_config_jsonb IS NULL OR (
    skill_id IN ('equipment_domination', 'equipment_martial_combo')
    AND jsonb_typeof(equipment_config_jsonb) = 'object'
    AND octet_length(equipment_config_jsonb::text) <= 16384
  )
) NOT VALID;
END IF;
END $$;
COMMIT;
ALTER TABLE player_skills VALIDATE CONSTRAINT player_skills_equipment_config_shape;
