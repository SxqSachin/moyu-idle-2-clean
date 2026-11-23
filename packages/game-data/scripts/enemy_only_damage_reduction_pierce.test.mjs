import assert from "node:assert/strict"
import test from "node:test"

import {
  parseDamageSpec,
  parseSkillTargeting,
  rejectEnemyOnlyDamageReductionPierce,
  rejectEnemyOnlyTargetPriority,
  shouldGenerateAutoSkillbook,
  validateEnemyOnlyDamageReductionPierceSkills,
  validateEnemyOnlyTargetPrioritySkills
} from "./build.mjs"

const damageEffect = (damageSpec) => ({ kind: "damage", damageSpec })
const skill = (overrides = {}) => ({
  id: "enemy_pressure_test",
  enemyOnly: true,
  effects: [damageEffect({
    enemyOnlyDamageReductionPierce: 0.35,
    enemyOnlyDamageReductionPierceTargetStatuses: ["visible_mark"]
  })],
  ...overrides
})
const buffs = [{ id: "visible_mark" }]

test("enemy-only damage-reduction pierce is normalized without hiding invalid bounds", () => {
  const parsed = parseDamageSpec({
    enemyOnlyDamageReductionPierce: 0.35,
    enemyOnlyDamageReductionPierceTargetStatuses: [" visible_mark "]
  })
  assert.equal(parsed.enemyOnlyDamageReductionPierce, 0.35)
  assert.deepEqual(parsed.enemyOnlyDamageReductionPierceTargetStatuses, ["visible_mark"])

  const outOfRange = parseDamageSpec({ enemyOnlyDamageReductionPierce: 1.01 })
  assert.equal(outOfRange.enemyOnlyDamageReductionPierce, 1.01)
  assert.throws(
    () => validateEnemyOnlyDamageReductionPierceSkills([skill({ effects: [damageEffect(outOfRange)] })], buffs),
    /enemyOnlyDamageReductionPierce invalid/
  )
})

test("pierce requires a monster-only skill and validates every conditional status", () => {
  assert.doesNotThrow(() => validateEnemyOnlyDamageReductionPierceSkills([skill()], buffs))
  assert.throws(
    () => validateEnemyOnlyDamageReductionPierceSkills([skill({ enemyOnly: false })], buffs),
    /skills\.enemyOnly required/
  )
  assert.throws(
    () => validateEnemyOnlyDamageReductionPierceSkills([skill({
      effects: [damageEffect({
        enemyOnlyDamageReductionPierce: 0.35,
        enemyOnlyDamageReductionPierceTargetStatuses: ["missing_mark"]
      })]
    })], buffs),
    /buff not found/
  )
})

test("player-owned damage specs reject monster-only fields even when configured to zero", () => {
  assert.throws(
    () => rejectEnemyOnlyDamageReductionPierce(
      { enemyOnlyDamageReductionPierce: 0 },
      "equipment_templates.basicAttackEffects.damageSpec: player_weapon"
    ),
    /enemy-skill-only/
  )
  assert.doesNotThrow(() => rejectEnemyOnlyDamageReductionPierce({ damageType: "physical" }, "ordinary damage"))
})

test("monster-only skills never receive auto-generated skillbooks", () => {
  assert.equal(shouldGenerateAutoSkillbook({ id: "enemy_pressure_test", enemyOnly: true }), false)
  assert.equal(shouldGenerateAutoSkillbook({ id: "player_skill", enemyOnly: false }), true)
  assert.equal(shouldGenerateAutoSkillbook({ id: "basic_attack" }), false)
})

test("target-priority bypass is preserved and restricted to monster-only skills", () => {
  const targeting = parseSkillTargeting({
    targetPositions: ["vanguard"],
    ignoreTargetPriority: true
  })
  assert.equal(targeting.ignoreTargetPriority, true)
  assert.doesNotThrow(() => validateEnemyOnlyTargetPrioritySkills([skill({
    effects: [{ kind: "damage", targeting }]
  })]))
  assert.throws(
    () => validateEnemyOnlyTargetPrioritySkills([skill({
      enemyOnly: false,
      effects: [{ kind: "damage", targeting }]
    })]),
    /skills\.enemyOnly required/
  )
  assert.throws(
    () => validateEnemyOnlyTargetPrioritySkills([skill({
      effects: [{ kind: "damage", targeting: { ...targeting, ignoreTargetPriority: "yes" } }]
    })]),
    /ignoreTargetPriority invalid/
  )
  assert.throws(
    () => rejectEnemyOnlyTargetPriority(targeting, "equipment_templates.basicAttackEffects.targeting: player_weapon"),
    /enemy-skill-only/
  )
})
