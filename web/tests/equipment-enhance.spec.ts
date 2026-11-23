import { describe, expect, it } from "vitest"
import { buildEnhanceCumulativeAttrs, buildEnhancePreview } from "@/services/equipment-enhance"
import type { EquipmentTemplateDefinition } from "@/types/equipment"

const template: EquipmentTemplateDefinition = {
  id: "wood_sword",
  equipType: "battle",
  slot: "weapon",
  occupySlots: [],
  statusRequirement: {},
  accessoryGroup: "",
  affixSlotCount: 0,
  affixAllowedIds: [],
  baseAttrs: { atk: 10 },
  maxDurability: 100,
  decayRate: 0,
  enhanceMaxLevel: 5,
  enhanceDurationS: 3,
  enhanceExp: 1,
  enhanceProtectItemId: ["protect_scroll"],
  enhanceCostCurve: {
    base: { stone: 2 },
    step: { stone: 1 },
    overrides: { "3": { stone: 9 } }
  },
  enhanceAttrCurve: {
    base: { atk: 1 },
    step: { atk: 1 },
    overrides: { "2": { atk: 7 } }
  },
  basicAttackTargetDirection: "vanguard",
  targetSeekStrategy: "front_to_rear",
  basicAttackFormula: {
    damageType: "physical",
    formula: [{ attrKey: "strength", coefficient: 1 }]
  }
}

describe("equipment enhance helpers", () => {
  it("builds preview with formula-based success and cost override", () => {
    const p1 = buildEnhancePreview(template, 0, "common")
    expect(p1).not.toBeNull()
    expect(p1?.nextLevel).toBe(1)
    expect(p1?.successRate).toBe(1)
    expect(p1?.costs.stone).toBe(2)

    const p2 = buildEnhancePreview(template, 2, "common")
    expect(p2?.nextLevel).toBe(3)
    expect(p2?.costs.stone).toBe(9) // override
    expect(p2?.deltaAttrs.atk).toBe(1)

    const p3 = buildEnhancePreview(template, 3, "common")
    expect(p3?.successRate).toBe(0.5875)
  })

  it("builds cumulative attrs with level1 base + later step semantics", () => {
    const attrs = buildEnhanceCumulativeAttrs(template, 3)
    // lvl1:+1, lvl2:+7(override), lvl3:+1(step)
    expect(attrs.atk).toBe(9)
  })
})
