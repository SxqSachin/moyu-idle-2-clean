import { getEquipmentTemplateDefinition, getItemDefinition } from "@services/item-definition"
import { hasItemCustomization } from "@services/item-customization"
import { getRecycleTemplateMatchKind } from "@composables/game/useInventorySummary"
import type { EquipmentAffixSlot, InventoryInstance } from "@/types/equipment"

export type DestructiveRecycleQuote = {
  items: Array<{
    instanceId: number
    templateItemId: string
    enhanceLevel: number
    affixes: EquipmentAffixSlot[]
    bodyGold: number
    disposalFee: number
    totalGold: number
    quoteToken: string
  }>
  feePercent: number
  bodyGold: number
  disposalFee: number
  totalGold: number
}

// This is an entry-point filter only. Eligibility and all prices come from the
// server quote; the browser must not strip affixes or calculate a payout.
export function isDestructiveRecycleCandidate(item: InventoryInstance): boolean {
  const template = getEquipmentTemplateDefinition(item.templateItemId)
  const definition = getItemDefinition(item.templateItemId)
  if (!template?.slot || !definition || definition.stackable || !(Number(definition.economyBaseValue) > 0)) return false
  const enhanceLevel = Number(item.enhanceLevel ?? 0)
  if (!Number.isInteger(enhanceLevel) || enhanceLevel < 0 || enhanceLevel > (template.enhanceMaxLevel || 0)) return false
  if (item.state !== "inventory" || item.quantity !== 1 || hasItemCustomization(item.customData)) return false
  if (!(item.durability > 0) || item.durability !== item.maxDurability || item.maxDurability !== (template.maxDurability || 100)) return false
  if (getRecycleTemplateMatchKind(item) !== "not_recyclable") return false
  if (!item.affixes?.length && enhanceLevel === 0) return false
  const seen = new Set<number>()
  for (const slot of item.affixes || []) {
    if (!Number.isInteger(slot.slotIndex) || slot.slotIndex < 0 || slot.slotIndex >= template.affixSlotCount || !slot.affixId?.trim() || !slot.rarityId?.trim() || seen.has(slot.slotIndex)) return false
    seen.add(slot.slotIndex)
  }
  return getRecycleTemplateMatchKind({ ...item, enhanceLevel: 0, affixes: [], baseSnapshot: template.baseAttrs }) !== "not_recyclable"
}

export function destructiveRecycleSelectionKey(items: InventoryInstance[]): string {
  return JSON.stringify(items.map((item) => ({
    instanceId: item.instanceId, templateItemId: item.templateItemId, quantity: item.quantity,
    state: item.state, archivedAt: item.archivedAt, enhanceLevel: item.enhanceLevel,
    durability: item.durability, maxDurability: item.maxDurability,
    customData: item.customData, baseSnapshot: item.baseSnapshot, affixes: item.affixes
  })).sort((a, b) => a.instanceId - b.instanceId))
}
