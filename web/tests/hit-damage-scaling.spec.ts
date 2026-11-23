import { describe, expect, it } from "vitest"
import enGame from "../src/locales/game/en-US"
import zhGame from "../src/locales/game/zh-CN"
import { buildEffectDetailSections } from "../src/services/effect-detail-renderer"
import {
  DEFAULT_POWERED_EXPONENTIAL_DECAY,
  approximateHitDamageEquivalentCap,
  hitDamageEquivalentSum,
  hitDamageMultiplier,
  normalizeHitDamageScaling
} from "../src/services/hit-damage-scaling"
import { scoreSkillDefinition } from "../src/services/combat-scoring"
import { getSkillDefinition, installRuntimeSkillDefinitions } from "../src/services/skill-definition"

type LocaleTree = Record<string, unknown>

function readLocale(tree: LocaleTree, key: string): unknown {
  let cursor: unknown = tree
  for (const part of key.split(".")) {
    if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) return undefined
    cursor = (cursor as LocaleTree)[part]
  }
  return cursor
}

function createT(tree: LocaleTree) {
  return (key: string, params: Record<string, unknown> = {}): string => {
    const value = readLocale(tree, key)
    if (typeof value !== "string") return key
    return value.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
  }
}

describe("powered exponential hit damage scaling", () => {
  const scaling = DEFAULT_POWERED_EXPONENTIAL_DECAY

  it("matches canonical multiplier, equivalent-hit, and cap vectors", () => {
    expect(hitDamageMultiplier(scaling, 1)).toBe(1)
    expect(hitDamageMultiplier(scaling, 2)).toBe(0.998)
    expect(hitDamageMultiplier(scaling, 12)).toBeCloseTo(0.6942975551370444, 12)
    expect(hitDamageMultiplier(scaling, 36)).toBeCloseTo(0.011095759646342131, 12)
    expect(hitDamageEquivalentSum(scaling, 36)).toBeCloseTo(15.969712553678615, 12)
    const cap = approximateHitDamageEquivalentCap(scaling)
    expect(cap?.converged).toBe(true)
    expect(cap?.equivalentHits).toBeCloseTo(16.00049329726089, 9)
  })

  it("rejects unknown or unsafe configurations", () => {
    expect(normalizeHitDamageScaling(scaling)).toEqual(scaling)
    expect(normalizeHitDamageScaling({ ...scaling, kind: "linear" })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, kind: " powered_exponential_decay " })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, secondHitMultiplier: "0.998" })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, curvePower: "2.1708" })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, floor: 0.01 })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, secondHitMultiplier: 1 })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, curvePower: 0.99 })).toBeNull()
    expect(normalizeHitDamageScaling({ ...scaling, curvePower: 3.01 })).toBeNull()
  })

  it("uses the same 64-hit technical ceiling as runtime summaries", () => {
    expect(hitDamageEquivalentSum(scaling, 100)).toBe(hitDamageEquivalentSum(scaling, 64))
  })

  it("preserves the typed DSL and explains current falloff in Chinese and English", () => {
    installRuntimeSkillDefinitions([{
      id: "falloff_projection_test",
      effects: [{
        kind: "damage",
        damageType: "magical",
        targeting: { targetScope: "enemy", targetPositions: ["vanguard"] },
        amountFormula: { baseAdd: 10, terms: [] },
        damageSpec: {
          hitCount: { base: 2, formula: [{ attrKey: "intelligence", coefficient: 0.005 }] },
          hitDamageScaling: scaling
        }
      }]
    }])
    const skill = getSkillDefinition("falloff_projection_test")
    expect(skill?.effects[0]?.damageSpec?.hitDamageScaling).toEqual(scaling)

    const input = { effects: skill?.effects || [], skillLevel: 1, unitAttributes: { intelligence: 6800 } }
    const zhRow = buildEffectDetailSections({ ...input, t: createT(zhGame) })[0]?.rows.find((row) => row.label === "多段伤害衰减")
    const enRow = buildEffectDetailSections({ ...input, t: createT(enGame) })[0]?.rows.find((row) => row.label === "Multi-hit Damage Falloff")
    expect(zhRow?.value).toContain("第2击为99.8%")
    expect(zhRow?.value).toContain("理论累计上限约为16.0005次")
    expect(zhRow?.preview).toContain("当前36击")
    expect(zhRow?.preview).toContain("15.9697次完整伤害")
    expect(enRow?.value).toContain("hit 2 is 99.8%")
    expect(enRow?.preview).toContain("Current 36 hits")
  })

  it("scores scaled multi-hit damage by equivalent hits", () => {
    const result = scoreSkillDefinition({
      id: "falloff_score_test",
      level: 1,
      effects: [{
        kind: "damage",
        targeting: { targetCount: { base: 1, formula: [] } },
        amountFormula: { baseAdd: 0, terms: [] },
        damageSpec: {
          amountFormula: { baseAdd: 10, terms: [] },
          hitCount: { base: 36, formula: [] },
          hitDamageScaling: scaling
        }
      }]
    })
    const damage = result?.sections?.[0]?.items?.find((item) => item.label === "DamageSpec数值")
    expect(damage?.score).toBe(159.7)
    expect(damage?.meta).toContain("equivalentHits 15.9697")
  })
})
