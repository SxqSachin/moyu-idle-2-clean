-- Allow abandoning currently active first-stage rumor commissions.
-- Follow-up rumor stages remain protected by their template notDeletable flag.
UPDATE player_cat_village_tasks
SET not_deletable = FALSE
WHERE status = 'open'
  AND not_deletable = TRUE
  AND commission_id IN (
    'cvc_rumor_wolf_king_prequel',
    'cvc_rumor_super_berry_bird',
    'cvc_rumor_shadow_assassin_cat_prequel',
    'cvc_rumor_glacial_phantom',
    'cvc_shadow_paw_footprints',
    'cvc_rumor_harmonic_lunar_loom_thread',
    'cvc_rumor_harmonic_shadow_echo_print',
    'cvc_rumor_harmonic_sacred_accord_seal'
  );
