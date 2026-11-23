import assert from "node:assert/strict"
import test from "node:test"

import { probabilityLifecycleProcChanceFields } from "./build.mjs"

const supportedAliases = [
  "procChance",
  "proc_chance",
  "procChanceBase",
  "proc_chance_base",
  "procChanceFormula",
  "proc_chance_formula",
  "procChanceMin",
  "proc_chance_min",
  "procChanceMax",
  "proc_chance_max"
]

test("probability lifecycle validation recognizes every runtime proc chance alias", () => {
  for (const field of supportedAliases) {
    assert.deepEqual(
      probabilityLifecycleProcChanceFields({ kind: "damage", [field]: null }),
      [field],
      `${field} must be rejected even when its configured value is null`
    )
  }
})

test("probability lifecycle validation ignores unrelated effect fields", () => {
  assert.deepEqual(probabilityLifecycleProcChanceFields({
    kind: "add_probability",
    value: 0.08,
    applyChance: 0.5,
    probabilityGroup: "lucky_blessing"
  }), [])
})
