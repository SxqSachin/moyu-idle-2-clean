import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

import { parseDamageSpec, validateHitDamageScaling } from "./build.mjs"

const validScaling = Object.freeze({
  kind: "powered_exponential_decay",
  secondHitMultiplier: 0.998,
  curvePower: 2.1708
})

test("damageSpec preserves optional powered exponential hit scaling without adding legacy defaults", () => {
  const parsed = parseDamageSpec({ hitDamageScaling: validScaling })
  assert.deepEqual(parsed.hitDamageScaling, validScaling)
  assert.notEqual(parsed.hitDamageScaling, validScaling)

  const reparsed = parseDamageSpec(parsed)
  assert.deepEqual(reparsed.hitDamageScaling, validScaling)

  const legacy = parseDamageSpec({ damageType: "magical" })
  assert.equal(Object.hasOwn(legacy, "hitDamageScaling"), false)
})

test("glacial shard storm authors the canonical hit scaling parameters", () => {
  const source = JSON.parse(fs.readFileSync(new URL("../source/json/skills.json", import.meta.url), "utf8"))
  const skill = source.skills.find((row) => row.id === "glacial_shard_storm")
  const effect = skill?.effects.find((row) => row.id === "glacial_shard_storm_damage")
  assert.deepEqual(effect?.damageSpec?.hitDamageScaling, validScaling)
})

test("hit scaling accepts inclusive curve bounds on every shared damageSpec path", () => {
  for (const path of [
    "skills.effects.damageSpec",
    "equipment_templates.basicAttackFormula.damageSpec",
    "equipment_templates.basicAttackEffects.damageSpec"
  ]) {
    for (const curvePower of [1, 3]) {
      assert.doesNotThrow(() => validateHitDamageScaling({
        hitDamageScaling: { ...validScaling, curvePower }
      }, path, "contract_test"))
    }
  }
})

test("malformed hit scaling is rejected instead of being silently dropped", () => {
  const invalidCases = [
    [null, /hitDamageScaling invalid/],
    [[], /hitDamageScaling invalid/],
    [{ ...validScaling, kind: "POWERED_EXPONENTIAL_DECAY" }, /hitDamageScaling\.kind invalid/],
    [{ secondHitMultiplier: 0.998, curvePower: 2.1708 }, /hitDamageScaling\.kind invalid/],
    [{ ...validScaling, secondHitMultiplier: "0.998" }, /hitDamageScaling\.secondHitMultiplier invalid/],
    [{ ...validScaling, secondHitMultiplier: 0 }, /hitDamageScaling\.secondHitMultiplier invalid/],
    [{ ...validScaling, secondHitMultiplier: 1 }, /hitDamageScaling\.secondHitMultiplier invalid/],
    [{ ...validScaling, secondHitMultiplier: Number.POSITIVE_INFINITY }, /hitDamageScaling\.secondHitMultiplier invalid/],
    [{ kind: validScaling.kind, secondHitMultiplier: 0.998 }, /hitDamageScaling\.curvePower invalid/],
    [{ ...validScaling, curvePower: "2.1708" }, /hitDamageScaling\.curvePower invalid/],
    [{ ...validScaling, curvePower: 0.9999 }, /hitDamageScaling\.curvePower invalid/],
    [{ ...validScaling, curvePower: 3.0001 }, /hitDamageScaling\.curvePower invalid/],
    [{ ...validScaling, curvePower: Number.NaN }, /hitDamageScaling\.curvePower invalid/],
    [{ ...validScaling, minimumMultiplier: 0 }, /hitDamageScaling\.minimumMultiplier unsupported/]
  ]

  for (const [hitDamageScaling, expectedError] of invalidCases) {
    assert.throws(() => parseDamageSpec({ hitDamageScaling }), expectedError)
  }
})
