import { beforeEach, describe, expect, it, vi } from "vitest"
import type { EquipmentTemplateDefinition, InventoryInstance } from "../src/types/equipment"

const templates = new Map<string, Partial<EquipmentTemplateDefinition>>()
const definitions = new Map<string, Record<string, unknown>>()
vi.mock("@services/item-definition", () => ({
  getEquipmentTemplateDefinition: (id: string) => templates.get(id),
  getItemDefinition: (id: string) => definitions.get(id),
  itemHasRandomDefaultAffixGrant: () => false
}))
import { destructiveRecycleSelectionKey, isDestructiveRecycleCandidate } from "../src/services/destructive-recycle"
import { getRecycleTemplateMatchKind, isTemplateCommodityInstance } from "../src/composables/game/useInventorySummary"

function instance(extra: Partial<InventoryInstance> = {}): InventoryInstance {
  return { instanceId: 11, templateItemId: "shield", quantity: 1, state: "inventory", enhanceLevel: 0, durability: 100, maxDurability: 100,
    baseSnapshot: { armor: 8 }, affixes: [{slotIndex: 0, affixId: "added", rarityId: "epic", attrMods: {armor: 5}}], ...extra }
}
beforeEach(() => {
  templates.clear(); definitions.clear()
  templates.set("shield", {slot:"offhand", affixSlotCount:3, enhanceMaxLevel:20, maxDurability:100, baseAttrs:{armor:3},defaultAffixSlots:[{slotIndex:0,affixId:"native",rarityId:"common"}]})
  definitions.set("shield", {stackable:false,economyBaseValue:100})
})

describe("destructive recycle entry filter", () => {
  it("adds a separate entry without changing market or ordinary recycling", () => {
    const item = instance()
    const original = JSON.stringify(item)
    expect(isDestructiveRecycleCandidate(item)).toBe(true)
    expect(getRecycleTemplateMatchKind(item)).toBe("not_recyclable")
    expect(isTemplateCommodityInstance(item)).toBe(false)
    expect(JSON.stringify(item)).toBe(original)
  })
  it.each([
    {enhanceLevel:-1}, {enhanceLevel:21}, {enhanceLevel:1.5}, {durability:99}, {durability:101,maxDurability:101}, {customData:{name:"Keep"}},
    {quantity:2}, {quantity:0}, {state:"equipped"}, {state:"auction"}, {affixes:[]},
    {affixes:[{slotIndex:9,affixId:"added",rarityId:"epic"}]},
    {affixes:[{slotIndex:0,affixId:"added",rarityId:"epic"},{slotIndex:0,affixId:"added",rarityId:"rare"}]},
    {baseSnapshot:{armor:3},affixes:[{slotIndex:0,affixId:"native",rarityId:"common"}]}
  ])("does not broaden unrelated restrictions: %j", (extra) => {
    expect(isDestructiveRecycleCandidate(instance(extra as Partial<InventoryInstance>))).toBe(false)
  })
  it("supports archived equipment but excludes carriers and stackable items", () => {
    expect(isDestructiveRecycleCandidate(instance({archivedAt:"2026-09-05T00:00:00Z"}))).toBe(true)
    templates.set("shield", {...templates.get("shield"),slot:""})
    expect(isDestructiveRecycleCandidate(instance())).toBe(false)
    templates.get("shield")!.slot = "body"
    definitions.get("shield")!.stackable = true
    expect(isDestructiveRecycleCandidate(instance())).toBe(false)
  })
  it("allows enhanced equipment with or without affixes, without changing ordinary recycling", () => {
    for (const affixes of [[], instance().affixes]) {
      const item = instance({enhanceLevel:10,affixes})
      expect(isDestructiveRecycleCandidate(item)).toBe(true)
      expect(getRecycleTemplateMatchKind(item)).toBe("not_recyclable")
      expect(isTemplateCommodityInstance(item)).toBe(false)
      expect(destructiveRecycleSelectionKey([item])).not.toBe(destructiveRecycleSelectionKey([{...item,enhanceLevel:11}]))
    }
  })
  it("invalidates consent for changed affixes, ownership surfaces and selection, not ordering", () => {
    const a = instance(), b = instance({instanceId:12})
    expect(destructiveRecycleSelectionKey([a,b])).toBe(destructiveRecycleSelectionKey([b,a]))
    expect(destructiveRecycleSelectionKey([a])).not.toBe(destructiveRecycleSelectionKey([a,b]))
    expect(destructiveRecycleSelectionKey([a])).not.toBe(destructiveRecycleSelectionKey([instance({affixes:[{slotIndex:0,affixId:"changed",rarityId:"mythic"}]})]))
    expect(destructiveRecycleSelectionKey([a])).not.toBe(destructiveRecycleSelectionKey([instance({state:"equipped"})]))
  })
})
