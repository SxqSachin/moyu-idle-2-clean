UPDATE player_kitty_attributes
SET final_value = final_value - 1,
    source_mods_jsonb = jsonb_set(
      jsonb_set(
        COALESCE(source_mods_jsonb, '{}'::jsonb),
        '{base,baseDefault}',
        to_jsonb((-1)::double precision),
        true
      ),
      '{base,value}',
      to_jsonb(COALESCE((source_mods_jsonb #>> '{base,value}')::double precision, 0) - 1),
      true
    ),
    updated_at = now()
WHERE attr_key IN (
    'collecting_speed',
    'mining_speed',
    'farming_animal_speed',
    'crafting_speed',
    'sewing_speed',
    'forging_speed'
  )
  AND COALESCE((source_mods_jsonb #>> '{base,baseDefault}')::double precision, 0) > -1;
