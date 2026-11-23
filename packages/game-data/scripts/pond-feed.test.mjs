import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'

// Exercise the production validator without rebuilding atlases for each case.
const builder = readFileSync(new URL('./build.mjs', import.meta.url), 'utf8')
const begin = builder.indexOf('function validatePondFeed(')
const end = builder.indexOf('\nfunction validateItems(', begin)
assert.ok(begin >= 0 && end > begin)
const validate = vm.runInNewContext(`(${builder.slice(begin, end)})`)
const good = () => ({ id: 'test', pondFeed: {
  nutrition: 10, feedClass: 'herbal', rulesetVersion: 'pond-feed-v2', role: 'functional',
  maxPortionsPerFishHour: 0.5,
  effects: [{ kind: 'stat_delta', stat: 'health', mode: 'flat', valuePerPortion: 0.01 }]
} })

const care = () => ({ id: 'care', pondFeed: { nutrition: 0, feedClass: 'arcane', rulesetVersion: 'pond-feed-v2', role: 'functional', delivery: 'targeted', effects: [
  { kind: 'stat_delta', stat: 'longevityDays', mode: 'flat', valuePerPortion: 1095 },
  { kind: 'stat_delta', stat: 'naturalProtectionDays', mode: 'flat', valuePerPortion: 1095 },
] } })
test('targeted life effects compose without feeder limits', () => assert.doesNotThrow(() => validate(care())))
for (const [name, mutate] of [
  ['shared longevity feed', item => { item.pondFeed.delivery = 'feeder'; item.pondFeed.nutrition = 10 }],
  ['unknown delivery', item => item.pondFeed.delivery = 'automatic'],
  ['targeted hourly cap', item => item.pondFeed.maxPortionsPerFishHour = 0.5],
  ['targeted nutrition ignored', item => item.pondFeed.nutrition = 10],
  ['targeted priority ignored', item => item.pondFeed.consumePriority = 1],
  ['negative life gain', item => item.pondFeed.effects[0].valuePerPortion = -1],
  ['unbounded life gain', item => item.pondFeed.effects[0].valuePerPortion = 1e20],
  ['life effect ceiling', item => item.pondFeed.effects[0].ceiling = 2000],
]) test(`rejects ${name}`, () => { const item = care(); mutate(item); assert.throws(() => validate(item)) })

test('sacred feed requires at least two jewels per portion without extra feed nesting', () => {
  const { actions } = JSON.parse(readFileSync(new URL('../source/json/actions.json', import.meta.url), 'utf8'))
  const { items } = JSON.parse(readFileSync(new URL('../source/json/items.json', import.meta.url), 'utf8'))
  const recipe = actions.find(a => a.mainOutput === 'pond_feed_sacred_longevity')
  const output = recipe.itemReward.pond_feed_sacred_longevity
  assert.equal(output, 1)
  assert.ok(recipe.costs.holy_cat_royal_jewel >= 2 * output)
  assert.ok(Object.values(recipe.costs).every(n => n >= output))
  assert.equal(Object.keys(recipe.costs).filter(id => items.find(item => item.id === id)?.pondFeed).length, 1)
  assert.ok(items.find(item => item.id === recipe.mainOutput).alchemy.convertExcluded)
  assert.equal(recipe.buildingRequirement.fish_pond, undefined)
})

test('flat composable feed accepts bounded positive and negative effects', () => {
  const item = good()
  item.pondFeed.effects.push({ kind: 'stat_delta', stat: 'satiety', mode: 'flat', valuePerPortion: -2, floor: 10 })
  assert.doesNotThrow(() => validate(item))
})
for (const [name, mutate] of [
  ['duplicate stat', item => item.pondFeed.effects.push({ ...item.pondFeed.effects[0] })],
  ['unbounded dose', item => item.pondFeed.maxPortionsPerFishHour = 100],
  ['zero dose', item => item.pondFeed.maxPortionsPerFishHour = 0],
  ['effects on staple', item => item.pondFeed.role = 'staple'],
  ['unknown role', item => item.pondFeed.role = 'magic'],
  ['unknown ruleset', item => item.pondFeed.rulesetVersion = 'pond-feed-v999'],
  ['cost without floor', item => item.pondFeed.effects.push({kind:'stat_delta', stat:'satiety', mode:'flat', valuePerPortion:-1})],
  ['all-negative compound', item => { item.pondFeed.effects[0].valuePerPortion = -0.01; item.pondFeed.effects[0].floor = 0.1 }],
  ['empty functional', item => item.pondFeed.effects = []],
]) test(`rejects ${name}`, () => {
  const item = good()
  mutate(item)
  assert.throws(() => validate(item))
})

const actionsBegin = builder.indexOf('function validateActions(')
const actionsEnd = builder.indexOf('\nfunction renderGo(', actionsBegin)
assert.ok(actionsBegin >= 0 && actionsEnd > actionsBegin)
const validateActions = vm.runInNewContext(`(${builder.slice(actionsBegin, actionsEnd)})`)
const action = (id, overrides) => ({ id, mainType: 'craft', subType: 'feed', menu: 'craft', rarity: 'common', economyIndustryTier: 1, economyDifficultyMultiplier: 1, ...overrides })
const recipeFixture = () => ({
  actions: [action('produce', { itemReward: { crop: 1 } }), action('make', { mainOutput: 'feed', itemReward: { feed: 2 }, costs: { crop: 3, loot: 2, basic: 2 } })],
  items: [{ id: 'feed', economyBaseValue: 10, pondFeed: good().pondFeed }, { id: 'basic', economyBaseValue: 10, pondFeed: { nutrition: 10 } }, { id: 'crop', economyBaseValue: 5 }, { id: 'loot', economyBaseValue: 5, type: ['loot'] }]
})
const validateRecipes = fixture => validateActions(fixture.actions, [{ id: 'craft' }], [{ id: 'feed' }], null, [], [], [], fixture.items)
test('accepts shallow recipe using produce, loot and one finished feed', () => assert.doesNotThrow(() => validateRecipes(recipeFixture())))
for (const [name, mutate] of [
  ['side-output feed', f => { f.actions[1].mainOutput = 'crop'; f.actions[1].itemReward.crop = 1 }],
  ['two finished feeds', f => { f.items.push({ id: 'second', economyBaseValue: 10, pondFeed: { nutrition: 10 } }); f.actions[1].costs.second = 2 }],
  ['output greater than ingredient', f => f.actions[1].costs.loot = 1],
  ['no loot', f => delete f.actions[1].costs.loot],
  ['no produce', f => delete f.actions[1].costs.crop],
  ['loot counted twice as produce', f => { delete f.actions[1].costs.crop; f.actions[0].itemReward = { loot: 1 } }],
  ['pond building lock', f => f.actions[1].buildingRequirement = { fish_pond: 1 }],
  ['insufficient input value', f => { f.items[1].economyBaseValue = 1; f.items[2].economyBaseValue = 1; f.items[3].economyBaseValue = 1 }],
]) test(`rejects recipe with ${name}`, () => {
  const fixture = recipeFixture()
  mutate(fixture)
  assert.throws(() => validateRecipes(fixture))
})
