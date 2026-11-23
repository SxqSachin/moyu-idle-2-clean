import assert from "node:assert/strict"
import test from "node:test"

import { validateEquipmentExtraEnemyRefreshEffect } from "./build.mjs"

const effect = {
  kind: "extra_enemy_refresh",
  enemyPool: "current_wave_boss",
  countFormula: {
    baseAdd: 1,
    terms: [{ attrKey: "equipment_enhance_level", coefficient: 0.05, perSkillLevel: 0 }]
  },
  maxCount: 3,
  partyStackMode: "highest",
  settlementMode: "normal"
}
const hook = { trigger: "on_enemy_refresh", conditions: [], effects: [effect] }
const template = { id: "cursed_silk_sachet", enhanceMaxLevel: 40 }

test("equipment extra boss refresh accepts the direct-enemy enhancement contract", () => {
  assert.doesNotThrow(() => validateEquipmentExtraEnemyRefreshEffect(effect, hook, template))
})

test("equipment extra boss refresh rejects summon-like or misleading contracts", () => {
  assert.throws(
    () => validateEquipmentExtraEnemyRefreshEffect({ ...effect, settlementMode: "summon" }, hook, template),
    /settlementMode invalid/
  )
  assert.throws(
    () => validateEquipmentExtraEnemyRefreshEffect({ ...effect, enemyPool: "current_world" }, hook, template),
    /enemyPool invalid/
  )
  assert.throws(
    () => validateEquipmentExtraEnemyRefreshEffect({ ...effect, maxCount: 2 }, hook, template),
    /truncates authored enhancement curve/
  )
  assert.throws(
    () => validateEquipmentExtraEnemyRefreshEffect(effect, { ...hook, trigger: "on_wave_start" }, template),
    /trigger invalid/
  )
})
