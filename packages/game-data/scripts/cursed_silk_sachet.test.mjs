import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const readTable = (name) => {
  const parsed = JSON.parse(fs.readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))
  return parsed[name]
}

const byID = (name, id) => readTable(name).find((row) => row.id === id)
const ITEM_ID = "cursed_silk_sachet"
const BUFF_ID = `${ITEM_ID}_curse`

function cumulativeAttr(template, attrID, requestedLevel) {
  const maxLevel = Math.max(0, Number(template?.enhanceMaxLevel) || 0)
  const level = Math.min(Math.max(0, requestedLevel), maxLevel)
  const curve = template?.enhanceAttrCurve || { base: {}, step: {}, overrides: {} }
  let value = Number(template?.baseAttrs?.[attrID] || 0)
  for (let nextLevel = 1; nextLevel <= level; nextLevel += 1) {
    const increment = curve.overrides?.[String(nextLevel)]
      ?? (nextLevel === 1 ? curve.base : curve.step)
      ?? {}
    value += Number(increment[attrID] || 0)
  }
  return value
}

test("cursed silk sachet keeps its risk and reward contract in authored data", () => {
  const item = byID("items", ITEM_ID)
  const template = byID("equipment_templates", ITEM_ID)
  const buff = byID("buff", BUFF_ID)
  const effects = readTable("buff_effects").filter((row) => row.buffId === BUFF_ID)

  assert.equal(item?.isEquipment, true)
  assert.equal(item?.rarity, "legendary")
  assert.equal(item?.iconId, `item.${ITEM_ID}`)
  assert.equal(item?.baseAttrs?.battle_drop_rate_bonus, 0.08)

  assert.equal(template?.slot, "accessory")
  assert.deepEqual(template?.equipTargets, ["player"])
  assert.equal(template?.baseAttrs?.battle_drop_rate_bonus, 0.08)
  assert.equal(template?.enhanceMaxLevel, 40)
  assert.deepEqual(template?.enhanceAttrCurve, {
    base: { battle_drop_rate_bonus: 0.001 },
    step: { battle_drop_rate_bonus: 0.001 },
    overrides: {
      10: { battle_drop_rate_bonus: 0.011 },
      20: { battle_drop_rate_bonus: 0.011 },
      30: { battle_drop_rate_bonus: 0.011 },
      40: { battle_drop_rate_bonus: 0.011 }
    }
  })
  assert.equal(cumulativeAttr(template, "battle_drop_rate_bonus", 0), 0.08)
  assert.ok(Math.abs(cumulativeAttr(template, "battle_drop_rate_bonus", 10) - 0.10) < 1e-9)
  assert.ok(Math.abs(cumulativeAttr(template, "battle_drop_rate_bonus", 20) - 0.12) < 1e-9)
  assert.ok(Math.abs(cumulativeAttr(template, "battle_drop_rate_bonus", 40) - 0.16) < 1e-9)
  assert.equal(template?.battleHooks?.[0]?.trigger, "on_wave_start")
  assert.equal(template?.battleHooks?.[0]?.maxTriggersPerBattle, undefined)
  assert.deepEqual(template?.battleHooks?.[0]?.effects, [{
    kind: "add_buff",
    buffId: BUFF_ID,
    targetMode: "self",
    dispatchHooks: false
  }])
  assert.deepEqual(template?.battleHooks?.[1], {
    id: "cursed_silk_sachet_extra_boss_refresh",
    trigger: "on_enemy_refresh",
    priority: 100,
    conditions: [],
    effects: [{
      kind: "extra_enemy_refresh",
      enemyPool: "current_wave_boss",
      countFormula: {
        baseAdd: 1,
        terms: [{ attrKey: "equipment_enhance_level", coefficient: 0.05, perSkillLevel: 0 }]
      },
      maxCount: 3,
      partyStackMode: "highest",
      settlementMode: "normal"
    }]
  })

  assert.equal(buff?.durationTurns, -1)
  assert.equal(buff?.permanent, true)
  assert.equal(buff?.dispellable, true)
  assert.deepEqual(
    effects.map((row) => [row.trigger, row.effects]).sort((a, b) => a[0].localeCompare(b[0])),
    [
      ["on_before_take_damage", [{ kind: "set_damage_multiplier", value: 1.2 }]],
      ["while_active", [{ kind: "add_stat_percent", attr: "speed", value: -0.2 }]]
    ]
  )
})

test("cursed silk sachet uses boss materials in crafting and enhancement milestones", () => {
  const action = byID("actions", "sewing_cursed_silk_sachet")
  const template = byID("equipment_templates", ITEM_ID)
  const enemies = readTable("enemies")

  assert.equal(action?.mainType, "sewing")
  assert.equal(action?.rarity, "legendary")
  assert.equal(action?.statusRequirement?.sewing, 60)
  assert.deepEqual(action?.costs, {
    silver_vein_silk: 10,
    ghost_essence: 2,
    base2_cave_membrane: 6,
    base4_cursed_wing_membrane: 1,
    base4_abyssal_core: 1,
    base4_ancestor_shadow_jade: 1
  })
  assert.deepEqual(action?.itemReward, { [ITEM_ID]: 1 })
  assert.deepEqual(template?.enhanceCostCurve?.overrides?.[20], {
    base4_abyssal_core: 1,
    base4_ancestor_shadow_jade: 1,
    ghost_essence: 2
  })
  assert.deepEqual(template?.enhanceCostCurve?.overrides?.[40], {
    final_volume_key_sigil: 1,
    ghost_essence: 4,
    shadow_paw_master_sigil: 1
  })

  const bossDropIDs = new Set(enemies
    .filter((enemy) => enemy.type === "boss")
    .flatMap((enemy) => Object.keys(enemy.drops || {})))
  for (const materialID of [
    "base4_abyssal_core",
    "base4_ancestor_shadow_jade",
    "shadow_paw_master_sigil",
    "final_volume_key_sigil"
  ]) {
    assert.equal(bossDropIDs.has(materialID), true, `${materialID} should remain a Boss material`)
  }
})
