import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import { validateAlchemyTables } from './build.mjs'

const builder = readFileSync(new URL('./build.mjs', import.meta.url), 'utf8')
const begin = builder.indexOf('function validateFishFoodRecipes(')
const end = builder.indexOf('\nfunction validateFishingTables(', begin)
assert.ok(begin >= 0 && end > begin)
const validate = vm.runInNewContext(`(${builder.slice(begin, end)})`)
const read = name => JSON.parse(readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), 'utf8'))[name]

test('transmute rules no longer restrict food bonuses by production source', () => {
  const data = JSON.parse(readFileSync(new URL('../generated/game-data.json', import.meta.url), 'utf8'))
  const run = () => validateAlchemyTables(data.alchemy_groups, data.item_traits, data.alchemy_combo_recipes, data.alchemy_transmute_rules, data.items, data.attributes, data.actions, [])
  assert.doesNotThrow(run)
  data.actions.push({ id: 'unsafe_gold_source', itemReward: { wolf_king_fang: { quantity: 1, chance: 1 } } })
  assert.doesNotThrow(run)
  assert.ok(read('alchemy_transmute_rules').every(rule => !('foodBonusItemIds' in rule)))
})

test('new food modes reject unsafe rates, zero-second operations and repeatable enchant servings', () => {
  for (const [kind, params] of [['alchemy.transmute_duration', { durationSeconds: 0 }], ['alchemy.transmute_gold_bonus', { bonusRate: .1001 }], ['enchantment.high_rarity_weight', { weightBonusRate: .251 }]]) {
    const f = effectFixture()
    f[0].find(row => row.kind === kind).params = params
    assert.throws(() => validateEffects(...f))
  }
  const f = fixture()
  f[0].find(row => row.id === 'moonglass_dual_fish_grill').mutationLifetimeBonuses = { prismatic: { charges: 1 } }
  assert.throws(() => validate(...f))
})
const fixture = () => ['fish_food_recipes', 'items', 'actions', 'operation_effects', 'fishing_species'].map(read)

test('work and conversion servings halve base, growth, mutations and stacking together', () => {
  const [recipes, items, , effects] = fixture()
  const meals = recipes.filter(row => /^(field_honey|patient_dual|clear_mind|gentle_current)_/.test(row.id))
  assert.equal(meals.length, 8)
  for (const recipe of meals) {
    const effect = effects.find(row => row.id === items.find(item => item.id === recipe.outputItemId).useOperationEffectId)
    const grill = recipe.kind === 'grilled_fish'
    const key = grill ? 'charges' : 'durationSeconds'
    const scale = recipe.lifetimeScaling[key]
    assert.equal(effect.lifetime[key], grill ? 220 : 900)
    assert.deepEqual([scale.min, scale.maxBeforeMutation, scale.max], grill ? [220, 325, 500] : [900, 1300, 2000])
    assert.equal(recipe.mutationLifetimeBonuses[grill ? 'prismatic' : 'rainbow'][key], grill ? 50 : 200)
    assert.equal(effect.stacking[grill ? 'maxCharges' : 'maxDurationSeconds'], grill ? effect.lifetime.charges * 100 : 86400)
    assert.ok(recipe.replacedEffectIds.length)
  }
})

const effectBegin = builder.indexOf('function validateOperationEffects(')
assert.ok(effectBegin >= 0 && begin > effectBegin)
const validateEffects = vm.runInNewContext(`(${builder.slice(effectBegin, begin)})`)
const effectFixture = () => {
  const tables = ['operation_effects', 'items', 'actions', 'action_main_types', 'action_sub_types'].map(read)
  // Base action types originate in workbooks; JSON contains only overrides.
  tables[3] = JSON.parse(readFileSync(new URL('../generated/game-data.json', import.meta.url), 'utf8')).action_main_types
  return [...tables, new Set(tables[0].map(row => row.iconId).filter(Boolean))]
}

test('primary work foods have narrow scopes, bounded scaling, cheap fish and paired serving modes', () => {
  const [recipes,items,actions,effects] = fixture()
  const meals = recipes.filter(recipe => effects.find(effect => effect.id === items.find(item => item.id === recipe.outputItemId).useOperationEffectId).stacking.key === 'food.primary_work')
  assert.equal(meals.length, 6)
  for (const recipe of meals) {
    const effect = effects.find(effect => effect.id === items.find(item => item.id === recipe.outputItemId).useOperationEffectId)
    assert.deepEqual(effect.selector.mainTypes, ['collecting','mining','farming_animal'])
    assert.equal(effect.stacking.exclusive,true)
    assert.ok(recipe.fishIngredients.every(fish => ['dace','bluegill'].includes(fish.speciesId)))
    assert.equal(recipe.fishIngredients.length,recipe.id.startsWith('patient') ? 2 : 1)
    assert.equal(actions.find(action => action.id === recipe.actionId).durationS,20)
  }
  for (const [id,param,max] of [['field_honey_grill','chance',.051],['patient_dual_grill','chance',.031],['patient_dual_soup','durationReductionRate',.081],['clear_mind_soup','experienceBonusRate',.151]]) {
    const f=fixture(); f[0].find(row=>row.id===id).parameterScaling[param].max=max
    assert.throws(()=>validate(...f),/handler bounds/)
  }
  const f=effectFixture(); f[0].find(row=>row.kind==='life_action.harvest_bonus').selector.mainTypes=['cooking']
  assert.throws(()=>validateEffects(...f))
})

test('current fish cuisine effects restrict timed audiences and preserve shared charges', () => {
  const f = effectFixture()
  const effects = new Map(f[0].map(row => [row.id, row]))
  for (const recipe of read('fish_food_recipes')) {
    const item = f[1].find(row => row.id === recipe.outputItemId)
    const effect = effects.get(item.useOperationEffectId)
    assert.deepEqual(effect.allowedTargets, effect.domain === 'enchantment' ? ['player'] : effect.lifetime.kind === 'duration' ? ['player', 'kitty'] : ['player', 'kitty', 'player_and_kitties'])
  }
  assert.doesNotThrow(() => validateEffects(...f))
})

test('shared domain handlers retain target and action validation', () => {
  for (const domain of ['alchemy', 'enhancement', 'fishing_cast']) {
    const f = effectFixture()
    const effect = f[0].find(row => row.domain === domain)
    effect.allowedTargets = ['kitty']
    assert.doesNotThrow(() => validateEffects(...f))
    effect.allowedTargets = ['unknown_actor']
    assert.throws(() => validateEffects(...f), /allowedTargets is invalid/)
    effect.allowedTargets = ['player_and_kitties']
    effect.selector = { actionIds: [domain === 'enhancement' ? 'fishing.trip' : 'equipment_enhance'] }
    assert.throws(() => validateEffects(...f), /selector/)
  }
})

test('all cuisine recipes validate with per-recipe mutation bonuses', () => assert.doesNotThrow(() => validate(...fixture())))
test('timed food cannot reintroduce either shared audience', () => {
  for (const target of ['all_kitties', 'player_and_kitties']) {
    const f = fixture()
    const recipe = f[0].find(row => row.kind === 'fish_soup')
    const effect = f[3].find(row => row.id === f[1].find(item => item.id === recipe.outputItemId).useOperationEffectId)
    effect.allowedTargets.push(target)
    assert.throws(() => validate(...f), /timed food requires an individual target/)
  }
})
test('creamfin uses accessible fish and a bounded standalone infusion success effect', () => {
  const [recipes,items,actions,effects,species]=fixture()
  for(const form of ['grill','soup']) {
    const id='creamfin_dual_fish_'+form
    const recipe=recipes.find(row=>row.id===id)
    assert.deepEqual(recipe.fishIngredients,[{speciesId:'whitefish',quantity:1},{speciesId:'burbot',quantity:1}])
    for(const fish of recipe.fishIngredients) assert.ok(['common','uncommon'].includes(species.find(row=>row.id===fish.speciesId).rarity))
    const effect=effects.find(row=>row.id===items.find(row=>row.id===id).useOperationEffectId)
    assert.equal(effect.kind,'alchemy.infusion_success_bonus')
    assert.deepEqual(effect.params,{bonusRate:.05})
    assert.equal(recipe.parameterScaling.bonusRate.max,.10)
    assert.equal(actions.find(row=>row.id===recipe.actionId).costs.milk,3)
  }
  for(const params of [{bonusRate:0},{bonusRate:.101},{bonusRate:.03,extraMaterialQuantity:1}]) {
    const f=effectFixture()
    f[0].find(row=>row.kind==='alchemy.infusion_success_bonus').params=params
    assert.throws(()=>validateEffects(...f),/infusion success bonus/)
  }
})
test('capacity tiers validate exact scopes and independent quality ceilings', () => {
  for(const [tier,rate] of [[3,.07],[1.5,.07],[1,.2001],[2,.1501]]) {
    const f=effectFixture()
    Object.assign(f[0].find(row=>row.id==='food_mossfire_dual_fish_grill_v7').params,{materialTier:tier,quantityLimitBonusRate:rate})
    assert.throws(()=>validateEffects(...f))
  }
  const [recipes,items,,effects]=fixture()
  for(const [series,tier,before,max] of [['mossfire',1,.12,.20],['lotusflame',2,.10,.15],['frostmoon',0,.09,.12]]) {
    for(const form of ['grill','soup']) {
      const id=series+'_dual_fish_'+form
      const effect=effects.find(row=>row.id===items.find(row=>row.id===id).useOperationEffectId)
      const scale=recipes.find(row=>row.id===id).parameterScaling.quantityLimitBonusRate
      assert.equal(effect.params.materialTier||0,tier)
      assert.equal(effect.params.quantityLimitBonusRate,.07)
      assert.equal(scale.maxBeforeMutation,before)
      assert.equal(scale.max,max)
      assert.equal(scale.curve,'size_growth')
    }
  }
})
test('capacity penalties are bounded, inverse-scaled and do not rewrite old food', () => {
  const [recipes,items,,effects]=fixture()
  for(const recipe of recipes.filter(row=>row.id.startsWith('frostmoon')||row.id.startsWith('mossfire'))) {
    const effect=effects.find(row=>row.id===items.find(row=>row.id===recipe.outputItemId).useOperationEffectId)
    assert.ok(effect.params.successRatePenalty>0 && effect.params.successRatePenalty<=.06)
    assert.equal(recipe.parameterScaling.successRatePenalty.inverse,true)
    assert.ok(recipe.parameterScaling.successRatePenalty.min>0)
  }
  assert.equal(effects.find(row=>row.id==='food_frostmoon_dual_fish_grill_v2').params.successRatePenalty,undefined)
  for(const value of [-.01,.201,'0.06']) {
    const f=effectFixture()
    f[0].find(row=>row.id==='food_frostmoon_dual_fish_grill_v3').params.successRatePenalty=value
    assert.throws(()=>validateEffects(...f))
  }
})
test('precious capacity rejects floor penalties and invalid quantities', () => {
  for (const params of [{extraMaterialQuantity:0},{extraMaterialQuantity:25.5},{extraMaterialQuantity:1000001},{extraMaterialQuantity:25,positiveFloorReductionRate:.1}]) {
    const f = effectFixture()
    f[0].find(row => row.kind === 'alchemy.infusion_precious_capacity_bonus').params = params
    assert.throws(() => validateEffects(...f))
  }
})
test('breakthrough rates are bounded and the costly pair keeps old definitions frozen', () => {
  for(const rate of [-.1,.1701,1,"0.1"]) {
    const f=effectFixture()
    f[0].find(row=>row.id==='food_frostmoon_dual_fish_grill_v2').params.quantityLimitBonusRate=rate
    assert.throws(()=>validateEffects(...f))
  }
  const [recipes,items,actions,effects]=fixture()
  for(const recipe of recipes.filter(row=>row.id.startsWith('frostmoon')||row.id.startsWith('mossfire'))) {
    const effect=effects.find(row=>row.id===items.find(row=>row.id===recipe.outputItemId).useOperationEffectId)
    assert.equal(effect.params.quantityLimitBonusRate,.07)
    assert.equal(recipe.parameterScaling.quantityLimitBonusRate.max,recipe.id.startsWith('mossfire') ? .20 : .12)
  }
  assert.equal(effects.find(row=>row.id==='food_frostmoon_dual_fish_grill_v1').params.quantityLimitBonusRate,undefined)
  for(const action of actions.filter(row=>row.id.startsWith('cook_frostmoon'))) {
    assert.equal(action.costs.moon_dew,24)
    assert.equal(action.costs.bottled_starlight,12)
  }
})
test('every current food has its own matching transparent 128px icon', () => {
  const [recipes,items,actions,effects] = fixture()
  const seen = new Set()
  for (const recipe of recipes) {
    const item = items.find(row => row.id === recipe.outputItemId)
    const action = actions.find(row => row.id === recipe.actionId)
    const effect = effects.find(row => row.id === item.useOperationEffectId)
    assert.equal(item.iconId, `item.${item.id}`)
    assert.equal(action.iconId,item.iconId)
    assert.equal(effect.iconId,item.iconId)
    assert.ok(!seen.has(item.iconId))
    seen.add(item.iconId)
    const png = readFileSync(new URL(`../../../web/public/assets/icons/item/${item.id}.png`,import.meta.url))
    assert.equal(png.readUInt32BE(16),128)
    assert.equal(png.readUInt32BE(20),128)
    assert.equal(png[25],6)
  }
  assert.equal(seen.size,30)
})
for (const change of [{length:0.8,weight:0.8},{maxBeforeMutation:10},{curve:'unknown'},{inverse:true},{traits:0.5}]) test(`rejects unsafe growth ${JSON.stringify(change)}`, () => {
  const f=fixture()
  Object.assign(f[0].find(row=>row.id==='mossfire_dual_fish_grill').parameterScaling.extraMaterialQuantity,change)
  assert.throws(()=>validate(...f))
})
test('a recipe may independently assign extra uses to multiple mutations', () => {
  const f = fixture()
  const recipe = f[0].find(row => row.id === 'reedbank_grill')
  recipe.mutationLifetimeBonuses = { prismatic: { charges: 8 }, shimmering: { charges: 3 } }
  assert.doesNotThrow(() => validate(...f))
})
for (const [name, bonus] of [
  ['fractional uses', { prismatic: { charges: 1.5 } }],
  ['negative uses', { prismatic: { charges: -1 } }],
  ['wrong lifetime', { rainbow: { durationSeconds: 900 } }],
  ['mixed lifetime', { prismatic: { charges: 8, durationSeconds: 900 } }],
  ['unknown mutation', { invented: { charges: 8 } }],
  ['stack cap overflow', { prismatic: { charges: 1200 } }],
  ['empty bonus', { prismatic: {} }]
]) test(`rejects ${name}`, () => {
  const f = fixture()
  f[0].find(row => row.id === 'reedbank_grill').mutationLifetimeBonuses = bonus
  assert.throws(() => validate(...f))
})
