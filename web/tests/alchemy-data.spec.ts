import { describe, expect, it } from "vitest"
import { alchemyStabilityQuantityMultiplier, computeTraitMaxQuantity, selectAlchemyTransmuteRule, type AlchemyTrait, type AlchemyTransmuteRule } from "@services/alchemy-data"

const trait = {
  unitBasis: 20,
  maxBasisMultiplier: 10
} as AlchemyTrait

describe("alchemy stability material capacity", () => {
  it.each([
    [0, 200],
    [1, 400],
    [5, 1200],
    [10, 1600],
    [15, 1960],
    [20, 2400],
    [25, 2780],
    [30, 3000],
    [40, 3200],
    [60, 3600],
    [100, 4400],
    [200, 5400]
  ])("maps stability %s to an aligned material limit of %s", (stability, expected) => {
    expect(computeTraitMaxQuantity(trait, stability)).toBe(expected)
  })

  it.each([
    [10, 8],
    [15, 9.84375],
    [20, 12],
    [25, 13.90625],
    [30, 15],
    [40, 16],
    [60, 18],
    [100, 22],
    [120, 23.6666667],
    [150, 25.3333333],
    [200, 27],
    [300, 28.6666667],
    [500, 30],
    [1000, 31]
  ])("maps stability %s to pursuit multiplier %s", (stability, expected) => {
    expect(alchemyStabilityQuantityMultiplier(stability)).toBeCloseTo(expected, 6)
  })

  it("stays monotonic and bounded by the 32x soft cap", () => {
    const values = [0, 1, 5, 10, 15, 20, 25, 30, 40, 60, 100, 120, 150, 200, 300, 500, 1000, 1_000_000]
      .map(alchemyStabilityQuantityMultiplier)
    for (let index = 1; index < values.length; index += 1) {
      expect(values[index]).toBeGreaterThanOrEqual(values[index - 1])
    }
    expect(values.every((value) => Number.isFinite(value) && value <= 32)).toBe(true)
    expect(alchemyStabilityQuantityMultiplier(Number.MAX_VALUE)).toBeLessThanOrEqual(32)
    expect(computeTraitMaxQuantity(trait, Number.MAX_VALUE)).toBeLessThanOrEqual(6400)
  })
})

function transmuteRule(id: string, overrides: Partial<AlchemyTransmuteRule> = {}): AlchemyTransmuteRule {
  return {
    id,
    itemIds: [],
    priority: 0,
    default: false,
    returnRatio: { min: 0.75, max: 0.95 },
    failChanceBase: 0.18,
    minFailChance: 0.1,
    durationSec: 8,
    rarityDurationAddSecByRarityIndex: [],
    alchemyExp: 1.25,
    luckyChanceBase: 0.04,
    luckyTiers: [],
    minAlchemyLevel: 1,
    active: true,
    ...overrides
  }
}

describe("alchemy transmute rule selection", () => {
  it("selects the highest-priority explicit item rule and otherwise falls back to default", () => {
    const rules = [
      transmuteRule("transmute.default", { default: true }),
      transmuteRule("transmute.advanced", { itemIds: ["boss_drop"], priority: 100, failChanceBase: 0.22 }),
      transmuteRule("transmute.boss", { itemIds: ["boss_drop"], priority: 200, failChanceBase: 0.26 })
    ]
    expect(selectAlchemyTransmuteRule(rules, "boss_drop")?.id).toBe("transmute.boss")
    expect(selectAlchemyTransmuteRule(rules, "ordinary_drop")?.id).toBe("transmute.default")
  })

  it("keeps the first active empty-scope rule compatible with legacy catalogs", () => {
    const rules = [
      transmuteRule("disabled", { active: false }),
      transmuteRule("legacy.first"),
      transmuteRule("legacy.second")
    ]
    expect(selectAlchemyTransmuteRule(rules, "anything")?.id).toBe("legacy.first")
  })
})
