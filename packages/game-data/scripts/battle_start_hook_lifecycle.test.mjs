import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const parsed = JSON.parse(fs.readFileSync(
  new URL("../source/json/equipment_templates.json", import.meta.url),
  "utf8"
))
const equipment = new Map(parsed.equipment_templates.map((row) => [row.id, row]))
const authoredHookTables = [
  "equipment_templates",
  "alchemy_combo_recipes",
  "affix_definitions",
  "kitty_personalities",
  "specialization_trees",
  "skills",
  "buff_effects",
]

const recurringSummonHooks = new Map([
  ["rock_golem_silkwood_staff", "rock_golem_staff_on_battle_start"],
  ["berry_birds_beloved_staff", "berry_birds_beloved_staff_battle_start"],
  ["king_of_amusement_park", "king_of_amusement_park_opening_parade"],
])

test("battle-start summon equipment remains eligible on every wave", () => {
  for (const [itemID, hookID] of recurringSummonHooks) {
    const hook = equipment.get(itemID)?.battleHooks?.find((row) => row.id === hookID)
    assert.equal(hook?.trigger, "on_battle_start", `${itemID} trigger`)
    assert.equal(hook?.maxTriggersPerBattle, undefined, `${itemID} whole-battle cap`)
    assert.ok(hook?.effects?.some((effect) => effect.kind === "summon"), `${itemID} summon effect`)
  }
})

test("only the empress internal permanent setup is battle-capped at battle start", () => {
  const cappedBattleStartHooks = []
  const visit = (node, tableName) => {
    if (Array.isArray(node)) {
      node.forEach((value) => visit(value, tableName))
      return
    }
    if (!node || typeof node !== "object") return
    const trigger = String(node.trigger ?? "").toLowerCase().replaceAll("_", "")
    const conditions = Array.isArray(node.conditions) ? node.conditions : []
    const hasConditionBattleCap = conditions.some((condition) => {
      if (!condition || typeof condition !== "object") return false
      const kind = String(condition.kind ?? condition.type ?? "").toLowerCase().replaceAll("_", "")
      return kind === "triggerlimit" && Number(condition.perBattle ?? condition.per_battle ?? 0) > 0
    })
    const hasTopLevelBattleCap = Number(node.maxTriggersPerBattle ?? node.max_triggers_per_battle ?? 0) > 0
    const hasNestedBattleCap = Number(node.limits?.perBattle ?? node.limits?.per_battle ?? 0) > 0
    if (trigger === "onbattlestart" && (hasTopLevelBattleCap || hasNestedBattleCap || hasConditionBattleCap)) {
      cappedBattleStartHooks.push(`${tableName}:${node.id ?? "anonymous"}`)
    }
    Object.values(node).forEach((value) => visit(value, tableName))
  }

  for (const tableName of authoredHookTables) {
    const table = JSON.parse(fs.readFileSync(
      new URL(`../source/json/${tableName}.json`, import.meta.url),
      "utf8"
    ))
    visit(table, tableName)
  }

  assert.deepEqual(cappedBattleStartHooks, ["equipment_templates:empress_cloak_enthrone"])
})
