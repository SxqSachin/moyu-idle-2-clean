import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { validateCatteryFurniture as validate } from './cattery-furniture.mjs'
const rows = JSON.parse(readFileSync(new URL('../source/json/cattery_furniture.json', import.meta.url))).cattery_furniture
const items = JSON.parse(readFileSync(new URL('../source/json/items.json', import.meta.url))).items
const mainTypes = ['collecting','mining','farming_animal','crafting','sewing','forging','alchemy','cooking','fishing'].map(id => ({ id }))
const validateCatteryFurniture = (rows, items) => validate(rows, items, [], mainTypes)
test('authored furniture DSL validates', () => validateCatteryFurniture(rows, items))
test('DSL rejects unknown fields, effects, free maintenance, duplicate definitions and invalid references', () => {
  for (const mutate of [r => r[0].script = 'evil()', r => r[0].rules[0].kind = 'typo', r => r[0].refillCosts.wood = 0, r => r.push(r[0]), r => r[0].refillCosts.missing = 1, r => r[0].rules[0].rateBps = 10001]) {
    const copy = structuredClone(rows.filter(d => d.service !== 'permanent')); mutate(copy); assert.throws(() => validateCatteryFurniture(copy, items))
  }
})
test('six levels preserve base maintenance and use approved effect rates', () => {
  for (const d of rows.filter(d => d.service !== 'permanent')) {
    assert.equal(d.levels.length, 5)
    const rates = [d.rules[0].rateBps, ...d.levels.map(x => x.rules[0].rateBps)]
    assert.deepEqual(rates, d.id === 'study_desk' ? [1500,1600,1700,1800,1900,2000] : [500,700,900,1100,1300,1500])
    const total = {...d.refillCosts}
    for (const tier of d.levels) for (const [id,n] of Object.entries(tier.refillCostAdd)) total[id] = (total[id] || 0) + n
    for (const [id,n] of Object.entries(d.refillCosts)) assert.ok(total[id] >= n)
    assert.ok(total.wood && total.copper && total.iron && total.moonwood && total.crystal)
    if (d.id === 'loot_cabinet') assert.ok(total.base2_cave_membrane && total.base2_ruin_core_shard && total.base4_golem_dark_core)
  }
})
test('progression rejects gaps, missing cost references and removing old materials', () => {
  for (const mutate of [r => r[0].levels[0].level = 3, r => r[0].levels[0].refillCostAdd.wood = -1, r => r[0].levels[0].upgradeCosts.unknown = 1, r => r[0].levels[0].rules[0].rateBps = 10001, r => r[0].levels[0].script = 'bad']) {
    const copy = structuredClone(rows.filter(d => d.service !== 'permanent')); mutate(copy); assert.throws(() => validateCatteryFurniture(copy, items))
  }
})
test('training has ten permanent tiers, retains low-tier costs, and excludes fishing', () => {
  const d = rows.find(d => d.id === 'kitty_training_bench')
  assert.equal(d.service, 'permanent')
  assert.deepEqual([d.rules[0].rateBps, ...d.levels.map(x => x.rules[0].rateBps)], [2000,4000,6000,8000,10000,10800,11600,12400,13200,14000])
  for (const costs of [d.installCosts, ...d.levels.map(x => x.upgradeCosts)]) {
    assert.ok(costs.wood_blank >= 160 && costs.copper >= 12 && costs.iron >= 12 && costs.flax_fiber >= 160)
    assert.ok(costs.base2_cave_membrane >= 24 && costs.base2_ruin_core_shard >= 12)
  }
  for (const mutate of [r => r.capacitySeconds = 1, r => r.refillCosts.wood = 1, r => r.levels[0].refillCostAdd.iron = 1, r => r.rules[0].selector.mainTypes.push('fishing'), r => r.rules[0].rateBps = 20001, r => r.levels[8].level = 11]) {
    const copy = structuredClone(d); mutate(copy); assert.throws(() => validateCatteryFurniture([copy], items))
  }
})
