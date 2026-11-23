import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const readTable = (name) => {
  const parsed = JSON.parse(fs.readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))
  return parsed[name]
}

const itemsByID = new Map(readTable("items").map((item) => [item.id, item]))
const templatesByID = new Map(readTable("equipment_templates").map((template) => [template.id, template]))
const actionsByID = new Map(readTable("actions").map((action) => [action.id, action]))

const TARGET_IDS = [
  "cat_sword",
  "dark_cat_sword",
  "hollow_heart",
  "jackpot_biased_token",
  "sugarcoat_cheat_pin",
  "consolation_prize_paper_shield",
  "full_house_applause_crown",
  "haunted_house_exit_ticket",
  "king_of_amusement_park",
  "parade_cape",
  "empress_cloak",
  "encore_echo_mask",
  "last_light_of_the_parade"
]

const SWORD_IDS = new Set(["cat_sword", "dark_cat_sword"])
const EXPECTED_RARITY = {
  cat_sword: "legendary",
  dark_cat_sword: "mythic",
  hollow_heart: "mythic",
  jackpot_biased_token: "legendary",
  sugarcoat_cheat_pin: "epic",
  consolation_prize_paper_shield: "epic",
  full_house_applause_crown: "legendary",
  haunted_house_exit_ticket: "legendary",
  king_of_amusement_park: "legendary",
  parade_cape: "legendary",
  empress_cloak: "legendary",
  encore_echo_mask: "mythic",
  last_light_of_the_parade: "legendary"
}
const MATERIAL_COUNT_BY_RARITY = {
  epic: 3,
  legendary: 4,
  mythic: 5
}

function cumulativeAttrs(template, requestedLevel) {
  const maxLevel = Math.max(0, Number(template.enhanceMaxLevel) || 0)
  const level = Math.min(Math.max(0, requestedLevel), maxLevel)
  const curve = template.enhanceAttrCurve || { base: {}, step: {}, overrides: {} }
  const out = { ...(template.baseAttrs || {}) }
  for (let nextLevel = 1; nextLevel <= level; nextLevel += 1) {
    const increment = curve.overrides?.[String(nextLevel)]
      ?? (nextLevel === 1 ? curve.base : curve.step)
      ?? {}
    for (const [attrID, value] of Object.entries(increment)) {
      out[attrID] = Number(out[attrID] || 0) + Number(value || 0)
    }
  }
  return out
}

function assertAttrMapClose(actual, expected, label) {
  assert.deepEqual(Object.keys(actual).sort(), Object.keys(expected).sort(), `${label} attribute keys`)
  for (const [attrID, expectedValue] of Object.entries(expected)) {
    assert.ok(
      Math.abs(Number(actual[attrID]) - expectedValue) <= 1e-9,
      `${label}.${attrID}: expected ${expectedValue}, received ${actual[attrID]}`
    )
  }
}

test("amusement equipment uses rarity-scaled linear material breadth without parade medals", () => {
  for (const id of TARGET_IDS) {
    const item = itemsByID.get(id)
    const template = templatesByID.get(id)
    assert.ok(item, `${id} item must exist`)
    assert.ok(template, `${id} equipment template must exist`)
    assert.equal(item.rarity, EXPECTED_RARITY[id], `${id} rarity`)

    const expectedCount = SWORD_IDS.has(id) ? 4 : MATERIAL_COUNT_BY_RARITY[item.rarity]
    assert.ok(expectedCount, `${id} rarity ${item.rarity} must have a balance rule`)

    const baseMaterials = Object.keys(template.enhanceCostCurve?.base || {}).sort()
    const stepMaterials = Object.keys(template.enhanceCostCurve?.step || {}).sort()
    assert.equal(baseMaterials.length, expectedCount, `${id} enhancement material breadth`)
    assert.deepEqual(stepMaterials, baseMaterials, `${id} base and step materials must stay aligned`)
    assert.equal(baseMaterials.includes("parade_king_medal"), false, `${id} must not linearly consume the boss medal`)
    assert.equal(
      Object.values(template.enhanceCostCurve?.overrides || {})
        .some((costs) => Object.hasOwn(costs, "parade_king_medal")),
      false,
      `${id} milestone costs must not consume the boss medal`
    )
    for (const materialID of baseMaterials) {
      assert.equal(template.enhanceCostCurve.base[materialID], 1, `${id}.${materialID} base cost`)
      assert.equal(template.enhanceCostCurve.step[materialID], 1, `${id}.${materialID} step cost`)
    }
  }
})

test("amusement equipment keeps the approved +40 and +50 final attribute endpoints", () => {
  const expectedAt40 = {
    cat_sword: { physics_damage: 157.5 },
    dark_cat_sword: { physics_damage: 237 },
    hollow_heart: {},
    jackpot_biased_token: { perception: 52.85, luck: 2.47, mp: 101 },
    sugarcoat_cheat_pin: {
      perception: 46,
      speed: 16.9,
      physics_damage_amplification: -0.08,
      magical_damage_amplification: -0.08
    },
    consolation_prize_paper_shield: { hp: 1465, physics_defence: 78, magical_defence: 78 },
    full_house_applause_crown: { perception: 53.85, luck: 5.74 },
    haunted_house_exit_ticket: { speed: 26, perception: 41.15 },
    king_of_amusement_park: { strength: 43.95, intelligence: 43.95, perception: 43.95 },
    parade_cape: { speed: 28, luck: 9.525 },
    empress_cloak: { strength: 43.95, dexterity: 43.95, intelligence: 43.95 },
    encore_echo_mask: { speed: 32.3, perception: 56.8, luck: 6.74 },
    last_light_of_the_parade: { speed: 26, perception: 55.8 }
  }

  for (const [id, expected] of Object.entries(expectedAt40)) {
    const template = templatesByID.get(id)
    assertAttrMapClose(cumulativeAttrs(template, 40), expected, `${id}@40`)
    if (template.enhanceMaxLevel === 40) {
      assertAttrMapClose(cumulativeAttrs(template, 50), expected, `${id}@50 capped`)
    }
  }

  assertAttrMapClose(cumulativeAttrs(templatesByID.get("cat_sword"), 50), { physics_damage: 223.3 }, "cat_sword@50")
  assertAttrMapClose(cumulativeAttrs(templatesByID.get("dark_cat_sword"), 50), { physics_damage: 336.4 }, "dark_cat_sword@50")
})

test("cat swords retain deliberate raw-damage headroom against their endgame benchmarks", () => {
  const cat40 = cumulativeAttrs(templatesByID.get("cat_sword"), 40).physics_damage
  const cat50 = cumulativeAttrs(templatesByID.get("cat_sword"), 50).physics_damage
  const dark40 = cumulativeAttrs(templatesByID.get("dark_cat_sword"), 40).physics_damage
  const dark50 = cumulativeAttrs(templatesByID.get("dark_cat_sword"), 50).physics_damage
  const crystal40 = cumulativeAttrs(templatesByID.get("crystal_sword"), 40).physics_damage
  const crystal50 = cumulativeAttrs(templatesByID.get("crystal_sword"), 50).physics_damage
  const dragon40 = cumulativeAttrs(templatesByID.get("base2_dragon_scale_lance"), 40).physics_damage
  const dragon50 = cumulativeAttrs(templatesByID.get("base2_dragon_scale_lance"), 50).physics_damage

  assert.ok(cat40 >= dragon40 * 0.65 && cat40 <= crystal40 * 0.7, "cat sword +40 leaves room for its lucky proc")
  assert.ok(cat50 >= dragon50 * 0.65 && cat50 <= crystal50 * 0.7, "cat sword +50 leaves room for its lucky proc")
  assert.ok(dark40 >= dragon40 * 0.95 && dark40 <= crystal40 * 1.05, "dark cat sword +40 matches raw endgame weapons")
  assert.ok(dark50 >= dragon50 * 0.95 && dark50 <= crystal50 * 1.05, "dark cat sword +50 matches raw endgame weapons")
  assert.ok(dark50 > cat50 * 1.4 && dark50 < cat50 * 1.6, "dark crafting upgrade remains substantial but bounded")
})

test("amusement crafting keeps equipment prizes inside the special-item category", () => {
  for (const actionID of [
    "craft_consolation_prize_paper_shield",
    "craft_full_house_applause_crown",
    "craft_king_of_amusement_park",
    "craft_encore_echo_mask"
  ]) {
    assert.equal(actionsByID.get(actionID)?.subType, "craft_special", `${actionID} category`)
  }
  assert.equal(
    readTable("actions").some((action) => action.subType === "craft_armor" || action.sub_type === "craft_armor"),
    false,
    "craft_armor must not reappear as a standalone crafting tab"
  )
})

test("encore mask uses the rumor trophy as a bounded gate, not an enhancement tax", () => {
  const action = actionsByID.get("craft_encore_echo_mask")
  const template = templatesByID.get("encore_echo_mask")
  assert.equal(action?.costs?.faceless_understudy_mask, 1)
  assert.equal(action?.costs?.parade_king_medal, 12)
  assert.equal(action?.costs?.final_volume_key_sigil, 1)
  assert.equal(action?.costs?.holy_cat_royal_jewel, 2)
  assert.equal(action?.costs?.moon_essence, 1)
  assert.equal(action?.costs?.condensed_stardust, 12)
  assert.equal(action?.costs?.goldvine_fiber, 10)
  assert.equal(action?.costs?.moon_resin, 6)
  assert.equal(Object.hasOwn(template?.enhanceCostCurve?.base || {}, "faceless_understudy_mask"), false)
  assert.equal(Object.hasOwn(template?.enhanceCostCurve?.base || {}, "spell_seal_lock_plate"), true)
  assert.equal(Object.hasOwn(template?.enhanceCostCurve?.base || {}, "hushed_ink_crystal"), false)
  assert.equal(Object.keys(template?.enhanceCostCurve?.base || {}).length, 5)
})

test("cat sword and parade cape upgrades accept a developed input instance", () => {
  assert.equal(
    actionsByID.get("craft_dark_cat_sword")?.materialInstanceRequirements?.cat_sword,
    undefined,
    "dark cat sword must not require an untouched full-durability cat sword"
  )
  assert.equal(
    actionsByID.get("sewing_empress_cloak")?.materialInstanceRequirements?.parade_cape,
    undefined,
    "empress cloak must not require an untouched full-durability parade cape"
  )
  assert.equal(actionsByID.get("sewing_empress_cloak")?.rarity, "legendary", "empress cloak recipe rarity")
})
