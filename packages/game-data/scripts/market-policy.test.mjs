import { test } from "node:test"
import assert from "node:assert/strict"
import fs from "node:fs"
import { normalizeMarketPolicy, validateMarketPolicies } from "./market-policy.mjs"
const profiles = JSON.parse(fs.readFileSync(new URL("../source/json/market_profiles.json", import.meta.url))).market_profiles
test('catalog source validates table rows and remains opt-in', () => {
  const tables = { fishing_species: [{ id: 'one', names: { 'en-US': 'One' } }, { id: 'two', names: { 'en-US': 'Two' } }] }
  validateMarketPolicies([], profiles, tables)
  assert.throws(() => validateMarketPolicies([], profiles, {}))
  assert.throws(() => validateMarketPolicies([], profiles, { fishing_species: [...tables.fishing_species, tables.fishing_species[0]] }))
  assert.throws(() => validateMarketPolicies([], profiles, { fishing_species: [{ id: 1, names: {} }] }))
  const changed = structuredClone(profiles)
  changed.find(p => p.catalogSource).catalogSource.valuePaths = ['/__proto__/id']
  assert.throws(() => validateMarketPolicies([], changed, tables))
})
test("legacy data stays unchanged and opt-in is explicit", () => {
  assert.equal(normalizeMarketPolicy(undefined), undefined)
  validateMarketPolicies([{ id: "wood", type: ["material"] }], profiles)
  validateMarketPolicies([{ id: "not_a_fish_or_food", type: ["special"], stackable: false, marketPolicy: { fixedPrice: { mode: "instance_variants", profileId: "effect_consumable_v1" }, buyOrders: false, auction: false } }], profiles)
})
test("reject unknown fields, missing profiles and unsafe selectors", () => {
  assert.throws(() => normalizeMarketPolicy({ fixedPrice: { mode: "instance_variants", profileId: "x" }, buyOrders: true, auction: false }))
  const changed = structuredClone(profiles); changed[0].groupKeys[0].path = "/params/*"
  assert.throws(() => validateMarketPolicies([], changed))
  assert.throws(() => validateMarketPolicies([{ id: "x", type: [], marketPolicy: { fixedPrice: { mode: "instance_variants", profileId: "missing" }, buyOrders: false, auction: false } }], profiles))
  assert.throws(() => normalizeMarketPolicy({ fixedPrice: { mode: "instance_variants", profileId: "x", script: "anything" }, buyOrders: false, auction: false }))
})
test("source bindings opt in without changing legacy flags", () => {
  const items = JSON.parse(fs.readFileSync(new URL("../source/json/items.json", import.meta.url))).items
  validateMarketPolicies(items, profiles)
  const target = items.filter(i => i.marketPolicy)
  const recipes = JSON.parse(fs.readFileSync(new URL("../source/json/fish_food_recipes.json", import.meta.url))).fish_food_recipes
  assert.equal(target.length, new Set(recipes.map(r => r.outputItemId)).size + 1)
  assert.equal(items.find(i => i.id === "fish_catch").marketListable, false)
  assert.ok(target.every(i => !i.marketPolicy.buyOrders && !i.marketPolicy.auction))
})
