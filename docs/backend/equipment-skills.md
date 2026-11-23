# Equipment Skills

## Player Contract

| Accessory | Granted skill | +0..9 | +10..19 | +20..25 |
| --- | --- | --- | --- | --- |
| 统御印记 / Domination Seal | 统御 / Domination | 1 target skill | 2 | 3 |
| 百战徽记 / Hundred-Battle Badge | 连武 / Martial Chain | 2 components | 3 | 4 |

- Both are player-only accessories, with maximum enhancement +25. Each has a dedicated 128x128 transparent item icon.
- Martial Chain's badge costs assassination_core x1, wolf_king_fang x1, base4_phantom_claw x12, base4_shadow_iron x12 and sun_resin x12. Domination Seal costs congealed_vow_ember x1, base4_abyssal_core x8, base4_ancestor_shadow_jade x8, base4_shadow_ink x4, base4_shadow_iron x12 and moon_resin x8. These are existing obtainable materials, not collectible trophies. Crafting and enhancement requirements otherwise remain unchanged.
- Domination is passive and does not occupy a carried-skill slot. It selects public official active skills, including enemy-only skills, but excludes basic attacks, private forged skills, other equipment skills and explicitly uninterruptible skills.
- When an enemy attempts a selected skill, the first affordable interceptor in frozen unit order pays floor(15% maximum HP) and floor(15% maximum MP), each at least 1. HP must remain positive. Allies are never intercepted. The caster still pays the original cost and cooldown; the interrupted skill has no effects or successful-cast reward.
- Martial Chain is one action with a fixed 6-turn cooldown. Select 2-4 distinct, currently carried physical skills. Every direct damage effect must be physical and at least one must exist; attached status effects keep their original behavior. Owned, accepted forged skills are supported, with existing same-source restrictions.
- Components retain their levels, effects, targeting, costs and actual cooldown entries. The wrapper does not add costs, skill experience, before/after-cast hooks or a second turn lifecycle. Scalar skill mutations apply to components, not the wrapper.
- All components must initially be ready and affordable; otherwise selection falls through to normal skills. Components then execute sequentially and recheck the actor. Death, action suppression or insufficient resources stops the remainder. Intercepting one component does not stop the next.
- Default: components cast only through Martial Chain; an explicit setting permits ordinary standalone selection. Components still consume ordinary carried-skill capacity. Their standalone rule groups are not conditions for the combination; the wrapper has its own groups and priority.
- A missing priority anchor falls back to first priority in the snapshot and settings dialog, never after basic attack. Draft loadout settings follow the ordinary skill editor: they do not apply the live character's weapon/status eligibility; the server validates the applied loadout and snapshot.
- Local component hooks use the existing hook budgets. The wrapper cannot be mirrored or recursively composed. Combination components do not each fan out to other-unit mirror observers, avoiding an additional full-unit observer pass per component. Summoned copies do not inherit these equipment grants.
- Invalid saved selections are retained but do not execute. Removing a component, losing equipment or reducing capacity cannot silently truncate the configured order. Existing battles retain their frozen snapshot; changes affect subsequent battle snapshots.

## Persistence And Runtime

- `player_skills.equipment_config_jsonb` is nullable. Only the two fixed equipment skill IDs may carry it. No extra table or per-player custom skill definition is created.
- `ListPlayerSkills` selects the new column in its existing SELECT. Normal rows do not parse extra JSON. `GetCombatConfig` and battle snapshot construction reuse that data and the already-loaded equipment/custom definitions.
- POST `/api/battle/skills/equipment-config` validates shape, ownership, equipped capacity and component eligibility within a user-locked transaction. Fixed limits are 3/4 selections, 6 groups of 6 conditions, 16 cooldown references per condition and a 16 KiB config. Revisions use compare-and-swap; stale saves receive HTTP 409.
- Saves and loadout changes necessarily perform writes and validation reads outside combat. No added configuration query is issued while executing a battle turn. Custom loadout components are resolved in one bulk ownership-filtered query; ordinary loadouts do not issue that query.
- Loadout JSON includes `equipmentSkills`. An empty object explicitly clears the preferences; legacy absent/null data leaves existing preferences unchanged. Config revision is advanced on loadout application so a stale settings dialog cannot overwrite the applied preset.
- Snapshot compilation freezes a maximum-four-element combo plan with direct skill indexes. The runtime does not reparse config, resolve templates, or search the skill catalog each turn.
- Domination indexing is attached to the existing unit-index construction, with no separate full-scene pass. Cast interception looks up only the attempted skill and its possible interceptors. Rebuilding after summons or snapshot recovery also rebuilds this derived index.
- Interception compares current perspective allegiance directly. Panic/camouflage targeting remaps neither intercept allies nor trigger target-selection scans. Each successful interception atomically pays HP and MP and emits two signed resource-loss rows plus the interruption trigger. Compact timeline serialization and frontend normalization preserve both costs and interrupted skill/actor IDs.
- Runtime and carry snapshots preserve the combo plan, component mode and domination IDs. Ordinary snapshots omit these fields. Timeline steps reuse existing cast/resource/trigger formats, with component IDs and an explicit interruption log entry.

## Deployment

1. Apply `server/schema/migrations/20260907_equipment_skill_config.sql` before starting updated API/worker processes. It is idempotent. The shape constraint is added NOT VALID, committed, then validated separately to avoid holding the initial exclusive table lock throughout the validation scan.
2. Publish generated game data and deploy API, battle workers and frontend together. Do not mix old workers with snapshots containing the new combo plan: old binaries do not understand it.
3. Do not rewrite ongoing or historical battle snapshots when preferences or equipment change. Use the normal new-battle snapshot boundary.

## Verification

- Go tests cover one-turn lifecycle, shared cooldowns, component ordering, interrupted casts, resource affordability, invalid indexes, zero Store dependency in execution, capacity boundaries, unequipping, disabling, standalone selection and snapshot/carry recovery.
- Loopback PostgreSQL temporary-table tests cover repeated migration, revision conflicts, concurrent competing saves, ownership, capacity, unavailable components, config/ownership separation and the single-query skill read. Private loadout definitions are bulk-read and ownership-filtered.
- Frontend tests cover draft isolation, mobile reorder bounds, invalidation without truncation and player-readable interruption logging. Data tests reject invalid accessory grants and prevent automatic skillbook generation for equipment-only skills.
- Browser QA uses `/tests/fixtures/equipment-skills-harness.html`: real components and game definitions, with isolated fake API responses. Desktop 1440px and mobile 360px layouts, selection, ordering, save/reopen and conflict preservation were checked. This is not an authenticated production battle proof.
- Apple M4 Pro microbenchmark, three samples: ordinary selection 87-93 ns/op, two-component readiness 218-223 ns/op, domination non-match approximately 10 ns/op. These paths measured 0 B/op and 0 allocations/op. They are not production capacity measurements or proof that additional component damage/effect execution is free.
- Full HTTP package tests passed. The store package has an existing failure in `TestArenaAcceptsProportionalCapacityStampAfterSerialization`; it also reproduces in an isolated HEAD checkout without this change.
- Frontend typecheck and Vite production build passed. The full frontend suite passed 122 files and failed 6 historical update-log assertions; the same 6 assertions fail in an isolated HEAD checkout. The new equipment-skill update-log assertion passes separately. Asset compression was not part of this verification.

## Review Notes

The feature, integration and operational review passes caught and corrected wrapper double-cost risk, reactive loadout cloning, stale-save dialog identity, empty-loadout persistence, automatic skillbook leakage, summoned-copy inheritance and private loadout component resolution. Production migration, mixed-worker rollout and real player battle validation remain deployment checks.

Follow-up regression coverage includes missing priority anchors, draft/live eligibility separation, repeated paid interruptions, insufficient HP/MP, minimum nonzero costs, allegiance versus targeting remaps, compact replay decoding and recipe/icon contracts. Local follow-up microbenchmarks with 16 and 256 camouflaged enemies show approximately 61-63 ns and zero allocations for an unaffordable matching interceptor; successful paid interceptions take approximately 0.72 us with a fixed 27 allocations for execution/log records, independent of scene size. This removes the previous scene-size-dependent scans and allocations, not the necessary cost of executing/logging the skill. No additional database reads were introduced.
