import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

function readTable(name) {
  return JSON.parse(readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))[name]
}

const expectedMaterialCosts = [
  { base2_moss_spore: 240, base2_cave_membrane: 80 },
  { base2_moss_spore: 480, base2_cave_membrane: 160, base2_ruin_core_shard: 80 },
  { white_coin_shard: 400, snowseal_wax: 400, base2_ruin_core_shard: 200, base2_dragon_scale_chip: 40 },
  { base4_shadow_silk: 400, base4_phantom_claw: 160, base4_cursed_wing_membrane: 160, base4_golem_dark_core: 60, base4_abyssal_core: 20 },
  { base4_abyssal_core: 100, star_rail_copper_plate: 400, refraction_lens_shard: 240, meteor_core_fragment: 160, condensed_stardust: 80, timekeeper_core: 20 },
]

const expectedConstructionCosts = [
  { wood: 1200, copper: 300, soft_resin: 500, stone: 1500 },
  { sturdy_wood: 1600, iron: 1250, silverleaf_resin: 650 },
  { silkwood: 2200, silver: 1600, clear_resin: 900 },
  { goldvine_wood: 3000, gold: 2200, sun_resin: 1300 },
  { moonwood: 4200, crystal: 3000, moon_resin: 1800 },
]

const expectedDurations = [600, 1800, 3600, 7200, 14400]

test("fishing pond construction plan keeps the intended time curve and loot sink", () => {
  const buildings = readTable("buildings")
  const items = new Map(readTable("items").map((row) => [row.id, row]))
  const enemies = readTable("enemies")
  const pond = buildings.find((row) => row.id === "fish_pond")

  assert.ok(pond)
  assert.equal(pond.levels.length, 5)
  assert.equal(pond.baseDurationS, expectedDurations[0])
  assert.equal(pond.levels[0].costs.copper, 300)

  for (const [index, level] of pond.levels.entries()) {
    const materialCosts = {}
    const constructionCosts = {}

    for (const [itemId, quantity] of Object.entries(level.costs)) {
      const item = items.get(itemId)
      assert.ok(item, `unknown fishing pond cost item: ${itemId}`)
      if (item.type?.includes("loot")) {
        materialCosts[itemId] = quantity
        assert.equal(quantity % 20, 0, `level ${level.level} loot cost is not scaled by 20: ${itemId}`)
        assert.ok(enemies.some((enemy) => enemy.drops?.[itemId]), `loot cost is not battle-acquired: ${itemId}`)
      } else {
        constructionCosts[itemId] = quantity
      }
    }

    assert.equal(level.durationS, expectedDurations[index])
    assert.deepEqual(materialCosts, expectedMaterialCosts[index])
    assert.deepEqual(constructionCosts, expectedConstructionCosts[index])
    assert.equal(Object.keys(materialCosts).length, index + 2)
  }
})
