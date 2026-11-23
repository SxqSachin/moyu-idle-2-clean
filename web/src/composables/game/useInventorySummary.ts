import { computed } from "vue"
import { storeToRefs } from "pinia"
import { useInventoryStore, type InventoryInstance } from "@stores/inventory"
import { getEquipmentTemplateDefinition, itemHasRandomDefaultAffixGrant } from "@services/item-definition"
import { hasItemCustomization } from "@services/item-customization"

export type RecycleTemplateMatchKind = "full_match" | "affix_mismatch_only" | "not_recyclable"

export function useInventorySummary() {
  const inventoryStore = useInventoryStore()
  const { instances } = storeToRefs(inventoryStore)

  const inventoryInstances = computed<InventoryInstance[]>(() =>
    instances.value.filter((item) => item.state === "inventory" && item.quantity > 0)
  )

  const quantityByItemId = computed(() => {
    const map = new Map<string, number>()
    for (const item of inventoryInstances.value) {
      map.set(item.templateItemId, (map.get(item.templateItemId) || 0) + item.quantity)
    }
    return map
  })

  const commodityQuantityByItemId = computed(() => {
    const map = new Map<string, number>()
    for (const item of inventoryInstances.value) {
      if (!isTemplateCommodityInstance(item)) continue
      map.set(item.templateItemId, (map.get(item.templateItemId) || 0) + item.quantity)
    }
    return map
  })

  const inventoryInstanceIds = computed(() => {
    const set = new Set<number>()
    for (const item of inventoryInstances.value) {
      set.add(item.instanceId)
    }
    return set
  })

  function getInventoryQuantity(itemId: string): number {
    if (!itemId) return 0
    return quantityByItemId.value.get(itemId) || 0
  }

  function getTemplateCommodityQuantity(itemId: string): number {
    if (!itemId) return 0
    return commodityQuantityByItemId.value.get(itemId) || 0
  }

  function hasInventoryInstance(instanceId: number): boolean {
    if (instanceId <= 0) return false
    return inventoryInstanceIds.value.has(instanceId)
  }

  return {
    inventoryInstances,
    getInventoryQuantity,
    getTemplateCommodityQuantity,
    hasInventoryInstance
  }
}

function isEmptyRecord(value: unknown): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return true
  return Object.keys(value).length === 0
}

function numberMapMatches(actual: Record<string, unknown> | undefined, expected: Record<string, number> | undefined): boolean {
  if (!actual || isEmptyRecord(actual)) return true
  const expectedEntries = Object.entries(expected || {}).filter(([key]) => key.trim() !== "")
  const actualEntries = Object.entries(actual).filter(([key]) => key.trim() !== "")
  if (actualEntries.length !== expectedEntries.length) return false
  const actualByKey = new Map(actualEntries.map(([key, value]) => [key.trim(), Number(value)]))
  for (const [key, expectedValue] of expectedEntries) {
    const actualValue = actualByKey.get(key.trim())
    if (!Number.isFinite(actualValue) || Math.abs((actualValue || 0) - Number(expectedValue || 0)) > 0.000000001) {
      return false
    }
  }
  return true
}

function fixedAffixesMatchTemplate(item: InventoryInstance): boolean {
  const template = getEquipmentTemplateDefinition(item.templateItemId)
  const expected = template?.defaultAffixSlots || []
  const actual = item.affixes || []
  const normalize = (slots: typeof actual) =>
    (slots || [])
      .filter((slot) => Number.isFinite(Number(slot.slotIndex)) && String(slot.affixId || "").trim() !== "" && String(slot.rarityId || "").trim() !== "")
      .map((slot) => ({
        slotIndex: Math.floor(Number(slot.slotIndex)),
        affixId: String(slot.affixId || "").trim(),
        rarityId: String(slot.rarityId || "").trim(),
        rollValue: Number(slot.rollValue || 0)
      }))
      .sort((a, b) => a.slotIndex - b.slotIndex || a.affixId.localeCompare(b.affixId) || a.rarityId.localeCompare(b.rarityId))
  const actualRows = normalize(actual)
  const expectedRows = normalize(expected)
  if (actualRows.length !== expectedRows.length) return false
  for (let idx = 0; idx < actualRows.length; idx += 1) {
    const row = actualRows[idx]
    const want = expectedRows[idx]
    if (row.slotIndex !== want.slotIndex || row.affixId !== want.affixId || row.rarityId !== want.rarityId) return false
    if (Math.abs(row.rollValue - want.rollValue) > 0.000000001) return false
  }
  return true
}

function affixIdentitiesMatchTemplate(item: InventoryInstance): boolean {
  const template = getEquipmentTemplateDefinition(item.templateItemId)
  const expected = template?.defaultAffixSlots || []
  const actual = item.affixes || []
  const normalize = (slots: typeof actual) =>
    (slots || [])
      .filter((slot) => Number.isFinite(Number(slot.slotIndex)) && String(slot.affixId || "").trim() !== "")
      .map((slot) => ({
        slotIndex: Math.floor(Number(slot.slotIndex)),
        affixId: String(slot.affixId || "").trim()
      }))
      .sort((a, b) => a.slotIndex - b.slotIndex || a.affixId.localeCompare(b.affixId))
  const actualRows = normalize(actual)
  const expectedRows = normalize(expected)
  if (actualRows.length !== expectedRows.length) return false
  for (let idx = 0; idx < actualRows.length; idx += 1) {
    const row = actualRows[idx]
    const want = expectedRows[idx]
    if (row.slotIndex !== want.slotIndex || row.affixId !== want.affixId) return false
  }
  return true
}

function hasAnyAffixId(slots: Array<{ slotIndex?: number; affixId?: string | null }> | undefined): boolean {
  return (slots || []).some((slot) =>
    Number.isFinite(Number(slot?.slotIndex)) &&
    String(slot?.affixId || "").trim() !== ""
  )
}

function instanceMatchesTemplateBase(item: InventoryInstance, rejectRandomDefaultAffixGrant: boolean): boolean {
  if (!item || item.state !== "inventory" || item.quantity <= 0) return false
  if (hasItemCustomization(item.customData)) return false
  if (Number(item.enhanceLevel || 0) !== 0) return false
  if (rejectRandomDefaultAffixGrant && itemHasRandomDefaultAffixGrant(item.templateItemId)) return false
  const maxDurability = Number(item.maxDurability || 0)
  const durability = Number(item.durability || 0)
  if (durability <= 0 || maxDurability <= 0 || durability !== maxDurability) return false
  const template = getEquipmentTemplateDefinition(item.templateItemId)
  const expectedMaxDurability = template?.maxDurability || 100
  if (maxDurability !== expectedMaxDurability) return false
  if (!numberMapMatches(item.baseSnapshot as Record<string, unknown> | undefined, template?.baseAttrs || {})) return false
  return true
}

export function isTemplateCommodityInstance(item: InventoryInstance): boolean {
  return instanceMatchesTemplateBase(item, true) && fixedAffixesMatchTemplate(item)
}

export function isRecycleAffixMismatchOnlyInstance(item: InventoryInstance): boolean {
  return getRecycleTemplateMatchKind(item) === "affix_mismatch_only"
}

export function getRecycleTemplateMatchKind(item: InventoryInstance): RecycleTemplateMatchKind {
  if (!instanceMatchesTemplateBase(item, false)) return "not_recyclable"
  const template = getEquipmentTemplateDefinition(item.templateItemId)
  const templateHasAffix = hasAnyAffixId(template?.defaultAffixSlots)
  const instanceHasAffix = hasAnyAffixId(item.affixes)
  if (itemHasRandomDefaultAffixGrant(item.templateItemId)) {
    if (templateHasAffix && affixIdentitiesMatchTemplate(item)) return "full_match"
    if (templateHasAffix && !instanceHasAffix) return "affix_mismatch_only"
    return "not_recyclable"
  }
  if (fixedAffixesMatchTemplate(item)) return "full_match"
  if (templateHasAffix && !instanceHasAffix) return "affix_mismatch_only"
  return "not_recyclable"
}
