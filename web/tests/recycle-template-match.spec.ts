import { describe, expect, it, vi } from "vitest"
import type { EquipmentTemplateDefinition, InventoryInstance } from "../src/types/equipment"

const templateByID = new Map<string, Partial<EquipmentTemplateDefinition>>()
const randomDefaultAffixItems = new Set<string>()

vi.mock("@services/item-definition", () => ({
  getEquipmentTemplateDefinition: (itemId: string) => templateByID.get(itemId) || null,
  itemHasRandomDefaultAffixGrant: (itemId: string) => randomDefaultAffixItems.has(itemId)
}))

import { getRecycleTemplateMatchKind, isTemplateCommodityInstance } from "../src/composables/game/useInventorySummary"

function buildInstance(overrides?: Partial<InventoryInstance>): InventoryInstance {
  return {
    instanceId: 1,
    templateItemId: "coarse_linen_gatherer_trousers",
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 200,
    maxDurability: 200,
    baseSnapshot: { collecting_speed: 0.05 },
    affixes: [],
    ...overrides
  }
}

describe("recycle template match kind", () => {
  it("marks unchanged template instance as full_match", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: []
    })
    randomDefaultAffixItems.clear()
    expect(getRecycleTemplateMatchKind(buildInstance())).toBe("full_match")
  })

  it("marks affix-changed instance as affix_mismatch_only", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: [{ slotIndex: 0, affixId: "affix.collecting_speed", rarityId: "common", rollValue: 1 }]
    })
    randomDefaultAffixItems.clear()
    expect(getRecycleTemplateMatchKind(buildInstance({ affixes: [] }))).toBe("affix_mismatch_only")
  })

  it("marks template-without-affix but instance-with-affix as not_recyclable", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: []
    })
    randomDefaultAffixItems.clear()
    expect(getRecycleTemplateMatchKind(buildInstance({
      affixes: [{ slotIndex: 0, affixId: "affix.collecting_speed", rarityId: "common", rollValue: 1 }]
    }))).toBe("not_recyclable")
  })

  it("marks template-and-instance-affix-id-mismatch as not_recyclable", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: [{ slotIndex: 0, affixId: "affix.template", rarityId: "common", rollValue: 1 }]
    })
    randomDefaultAffixItems.clear()
    expect(getRecycleTemplateMatchKind(buildInstance({
      affixes: [{ slotIndex: 0, affixId: "affix.instance", rarityId: "common", rollValue: 1 }]
    }))).toBe("not_recyclable")
  })

  it("marks random default-affix instance with matching affix identity as full_match", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: [{ slotIndex: 0, affixId: "affix.template", rarityId: "rare", rollValue: 1 }]
    })
    randomDefaultAffixItems.clear()
    randomDefaultAffixItems.add("coarse_linen_gatherer_trousers")
    const item = buildInstance({
      affixes: [{ slotIndex: 0, affixId: "affix.template", rarityId: "legendary", rollValue: 0.42 }]
    })
    expect(getRecycleTemplateMatchKind(item)).toBe("full_match")
    expect(isTemplateCommodityInstance(item)).toBe(false)
  })

  it("marks random default-affix instance with mismatched affix identity as not_recyclable", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: [{ slotIndex: 0, affixId: "affix.template", rarityId: "rare", rollValue: 1 }]
    })
    randomDefaultAffixItems.clear()
    randomDefaultAffixItems.add("coarse_linen_gatherer_trousers")
    expect(getRecycleTemplateMatchKind(buildInstance({
      affixes: [{ slotIndex: 0, affixId: "affix.other", rarityId: "rare", rollValue: 1 }]
    }))).toBe("not_recyclable")
  })

  it("marks random default-affix item without template affix as not_recyclable", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: []
    })
    randomDefaultAffixItems.clear()
    randomDefaultAffixItems.add("coarse_linen_gatherer_trousers")
    expect(getRecycleTemplateMatchKind(buildInstance({ affixes: [] }))).toBe("not_recyclable")
  })

  it("marks non-template-consistent instance as not_recyclable", () => {
    templateByID.set("coarse_linen_gatherer_trousers", {
      maxDurability: 200,
      baseAttrs: { collecting_speed: 0.05 },
      defaultAffixSlots: []
    })
    randomDefaultAffixItems.clear()
    expect(getRecycleTemplateMatchKind(buildInstance({ durability: 199 }))).toBe("not_recyclable")
  })
})
