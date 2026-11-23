import { describe, expect, it } from "vitest"
import type { ActionMaterialInstanceRequirement } from "../src/types/generated/game-data"
import {
  actionMaterialAffixesMatchTemplate,
  actionMaterialCanProvideEnhancementInheritance,
  actionMaterialInstanceSatisfiesRequirement,
  buildActionMaterialBindingArgs
} from "../src/services/action-material-requirement"

const requirement: ActionMaterialInstanceRequirement = {
  enhanceLevel: 0,
  requireTemplateAffixes: true,
  durability: "full"
}

const templateAffixes = [
  { slotIndex: 0, affixId: "cat_edge" },
  { slotIndex: 1, affixId: "cat_luck" }
]

const eligibleInstance = {
  state: "inventory",
  quantity: 1,
  enhanceLevel: 0,
  durability: 100,
  maxDurability: 100,
  affixes: [...templateAffixes].reverse(),
  customData: {}
}

describe("action material instance requirements", () => {
  it("matches an unenhanced, full-durability original regardless of affix order", () => {
    expect(actionMaterialAffixesMatchTemplate(eligibleInstance.affixes, templateAffixes)).toBe(true)
    expect(actionMaterialInstanceSatisfiesRequirement(eligibleInstance, requirement, templateAffixes)).toBe(true)
  })

  it("rejects enhanced, worn, customized, and non-template instances", () => {
    expect(actionMaterialInstanceSatisfiesRequirement({ ...eligibleInstance, enhanceLevel: 1 }, requirement, templateAffixes)).toBe(false)
    expect(actionMaterialInstanceSatisfiesRequirement({ ...eligibleInstance, durability: 99 }, requirement, templateAffixes)).toBe(false)
    expect(actionMaterialInstanceSatisfiesRequirement({ ...eligibleInstance, customData: { name: "改装猫剑" } }, requirement, templateAffixes)).toBe(false)
    expect(actionMaterialInstanceSatisfiesRequirement({ ...eligibleInstance, customData: { labelColor: "gold" } }, requirement, templateAffixes)).toBe(false)
    expect(actionMaterialInstanceSatisfiesRequirement({
      ...eligibleInstance,
      affixes: [{ slotIndex: 0, affixId: "cat_edge" }]
    }, requirement, templateAffixes)).toBe(false)
  })

  it("keeps restricted materials out of enhancement inheritance while allowing a binding without a source", () => {
    const requirements = { cat_sword: requirement }
    expect(actionMaterialCanProvideEnhancementInheritance(requirements, "cat_sword")).toBe(false)
    expect(actionMaterialCanProvideEnhancementInheritance(requirements, "ordinary_blade")).toBe(true)
    expect(buildActionMaterialBindingArgs({ cat_sword: [41] })).toEqual({
      materialInstanceBindings: { cat_sword: [41] }
    })
    expect(buildActionMaterialBindingArgs({ ordinary_blade: [42] }, 42)).toEqual({
      materialInstanceBindings: { ordinary_blade: [42] },
      inheritFromInstanceId: 42
    })
  })
})
