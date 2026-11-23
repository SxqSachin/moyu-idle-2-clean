import test from "node:test"
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { validateEquipmentSkills } from "./equipment-skills.mjs"
import { shouldGenerateAutoSkillbook } from "./build.mjs"

const read = name => JSON.parse(readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))[name]
const templates = read("equipment_templates")
const skills = read("skills")
test("equipment grants use the bounded accessory contract", () => {
  assert.doesNotThrow(() => validateEquipmentSkills(templates, skills))
  const grants = templates.filter(row => row.grantedSkill)
  assert.equal(grants.length, 2)
  for (const skill of skills.filter(row => row.equipmentOnly)) assert.equal(shouldGenerateAutoSkillbook(skill), false)
  for (const change of [row => row.grantedSkill.baseCapacity = 9, row => row.grantedSkill.capacityMilestones = [5, 10], row => row.slot = "weapon", row => row.equipTargets = ["kitty"]]) {
    const rows = structuredClone(grants)
    change(rows[0])
    assert.throws(() => validateEquipmentSkills(rows, skills))
  }
})

test("equipment skill recipes require their rare cores and themed materials", () => {
  const items = new Map(read("items").map(row => [row.id, row]))
  const actions = new Map(read("actions").map(row => [row.id, row]))
  const expected = {
    domination_seal: {congealed_vow_ember: 1, base4_abyssal_core: 8, base4_ancestor_shadow_jade: 8, base4_shadow_ink: 4, base4_shadow_iron: 12, moon_resin: 8},
    hundred_battle_badge: {assassination_core: 1, wolf_king_fang: 1, base4_phantom_claw: 12, base4_shadow_iron: 12, sun_resin: 12}
  }
  for (const [id, costs] of Object.entries(expected)) {
    const action = actions.get(`craft_${id}`)
    assert.deepEqual(action.costs, costs)
    assert.deepEqual(action.itemReward, {[id]: 1})
    assert.equal(action.iconId, `item.${id}`)
    assert.equal(items.get(id).iconId, `item.${id}`)
    for (const material of Object.keys(costs)) {
      assert.ok(items.has(material), `missing ${material}`)
      assert.equal(items.get(material).disabled, false)
      assert.equal(items.get(material).type.includes("collectible"), false)
    }
    const png = readFileSync(new URL(`../../../web/public/assets/icons/item/${id}.png`, import.meta.url))
    assert.equal(png.subarray(1, 4).toString(), "PNG")
    assert.equal(png.readUInt32BE(16), 128)
    assert.equal(png.readUInt32BE(20), 128)
    assert.equal(png[25], 6, "icons must be RGBA")
  }
})
