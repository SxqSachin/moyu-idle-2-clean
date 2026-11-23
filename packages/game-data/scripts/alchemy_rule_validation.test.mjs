import assert from "node:assert/strict"
import test from "node:test"

import { validateAlchemyTables } from "./build.mjs"

const material = {
  id: "final_volume_key_sigil",
  type: ["loot", "material"],
  category: "loot",
  rarity: "legendary",
  isEquipment: false,
  disabled: false
}

const baseSkillbook = {
  id: "skillbook_base",
  type: ["skillbook"],
  category: "skillbook",
  rarity: "legendary",
  isEquipment: false,
  disabled: false
}

const mythicSkillbook = {
  ...baseSkillbook,
  id: "skillbook_mythic",
  rarity: "mythic"
}

function rule(id, overrides = {}) {
  return {
    id,
    itemIds: [],
    priority: 0,
    default: false,
    returnRatio: { minRatio: 0.75, maxRatio: 0.95 },
    failChanceBase: 0.18,
    minFailChance: 0.1,
    durationSec: 8,
    rarityDurationAddSecByRarityIndex: [],
    alchemyExp: 1.25,
    luckyTiers: [],
    disabled: false,
    ...overrides
  }
}

function validate(rules, items = [material]) {
  validateAlchemyTables([], [], [], rules, items, [], [], [])
}

function conversionGroup(overrides = {}) {
  return {
    id: "group.skillbook_test",
    entries: [{ itemId: baseSkillbook.id, weight: 1 }],
    upgradeEntries: [],
    goldCost: 0,
    durationSec: 0,
    alchemyExp: 0,
    failChanceBase: 0.15,
    minFailChance: 0.15,
    upgradeChanceBase: 0,
    disabled: false,
    ...overrides
  }
}

test("alchemy transmute selector accepts key_ inside a typed material id", () => {
  assert.doesNotThrow(() => validate([
    rule("transmute.sigil", { itemIds: [material.id], priority: 100 }),
    rule("transmute.default", { default: true })
  ]))
})

test("alchemy transmute selector rejects dangling item references", () => {
  assert.throws(() => validate([
    rule("transmute.missing", { itemIds: ["missing_item"], priority: 100 }),
    rule("transmute.default", { default: true })
  ]), /item not found/)
})

test("alchemy transmute selector rejects duplicate active item matches", () => {
  assert.throws(() => validate([
    rule("transmute.first", { itemIds: [material.id], priority: 100 }),
    rule("transmute.second", { itemIds: [material.id], priority: 200 }),
    rule("transmute.default", { default: true })
  ]), /active item match is ambiguous/)
})

test("alchemy conversion rejects extra ultra-rare reward branches", () => {
  assert.throws(() => validateAlchemyTables([
    conversionGroup({
      ultraRareChanceBase: 0.0005,
      ultraRareEntries: [{ itemId: mythicSkillbook.id, weight: 1 }]
    })
  ], [], [], [], [baseSkillbook, mythicSkillbook], [], [], []), /does not support ultra-rare reward branches/)
})

test("alchemy conversion rejects mythic upgrade outputs", () => {
  assert.throws(() => validateAlchemyTables([
    conversionGroup({
      upgradeChanceBase: 0.05,
      upgradeEntries: [{ itemId: mythicSkillbook.id, weight: 1 }]
    })
  ], [], [], [], [baseSkillbook, mythicSkillbook], [], [], []), /cannot produce mythic items/)
})
