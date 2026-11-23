import assert from "node:assert/strict"
import test from "node:test"

import { validateBuffConditionTrigger, validateBuffEventFilter } from "./build.mjs"

test("buff turn filters accept authored ordered-turn semantics", () => {
  assert.doesNotThrow(() => validateBuffEventFilter({
    turnKinds: ["ordered"],
    includePreparation: false,
    includeSuppressedTurns: true
  }, "curtain.eventFilter", "on_turn_end"))
})

test("buff event filters fail closed for mismatched triggers and unknown keys", () => {
  assert.throws(
    () => validateBuffEventFilter({ turnKinds: ["ordered"] }, "filter", "on_after_take_damage"),
    /turn filters require on_turn_start or on_turn_end/
  )
  assert.throws(
    () => validateBuffEventFilter({ includeSuppressedTurns: true }, "filter", "on_turn_start"),
    /includeSuppressedTurns requires on_turn_end/
  )
  assert.throws(
    () => validateBuffEventFilter({ probabilityKinds: ["equipment_proc"] }, "filter", "on_turn_end"),
    /probability filters require a probability lifecycle trigger/
  )
  assert.throws(
    () => validateBuffEventFilter({ turnKinds: ["ordered"], typo: true }, "filter", "on_turn_end"),
    /filter\.typo is an unknown key/
  )
})

test("buff event filters validate values", () => {
  assert.throws(
    () => validateBuffEventFilter({ turnKinds: ["normal"] }, "filter", "on_turn_end"),
    /turnKinds invalid/
  )
  assert.throws(
    () => validateBuffEventFilter({ includePreparation: "false" }, "filter", "on_turn_end"),
    /includePreparation invalid/
  )
  assert.throws(
    () => validateBuffEventFilter({}, "filter", "on_turn_end"),
    /must not be empty/
  )
})

test("lethal incoming-damage conditions require the damage interception trigger", () => {
  assert.doesNotThrow(() => validateBuffConditionTrigger(
    { type: "incoming_damage_would_be_lethal" },
    "on_before_take_damage",
    "tail light"
  ))
  assert.throws(
    () => validateBuffConditionTrigger(
      { type: "incoming_damage_would_be_lethal" },
      "on_turn_end",
      "tail light"
    ),
    /requires on_before_take_damage/
  )
  assert.doesNotThrow(() => validateBuffConditionTrigger(
    { type: "self_hp_percent_lte", value: 0.5 },
    "on_turn_end",
    "ordinary condition"
  ))
})
