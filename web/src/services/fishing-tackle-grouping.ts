import { getGameDataTable } from "@services/game-data-runtime"
import type { FishingTackleDefinition } from "@services/fishing"

export type FishingTackleGroupSection<T> = {
  id: string
  label: string
  items: T[]
}

type FishingTackleGroupingDefinition = Pick<FishingTackleDefinition, "itemId" | "slot" | "methods">

type Translate = (key: string) => string

const FISHING_TACKLE_SLOT_ORDER = [
  "rod",
  "reel",
  "main_line",
  "leader",
  "hook",
  "float",
  "sinker",
  "lure",
  "bait"
] as const

const FISHING_TACKLE_SLOT_LOCALE_KEYS: Record<string, string> = {
  rod: "rod",
  reel: "reel",
  main_line: "mainLine",
  leader: "leader",
  hook: "hook",
  float: "float",
  sinker: "sinker",
  lure: "lure",
  bait: "bait"
}

function localizeFallback(t: Translate, key: string, fallback: string): string {
  const localized = t(key)
  return localized === key ? fallback : localized
}

function localizeFishingTackleSlot(t: Translate, slotId: string): string {
  if (slotId === "float_or_sinker") {
    const floatLabel = localizeFallback(t, "game.fishing.float", "float")
    const sinkerLabel = localizeFallback(t, "game.fishing.sinker", "sinker")
    return `${floatLabel} / ${sinkerLabel}`
  }
  const localeKey = FISHING_TACKLE_SLOT_LOCALE_KEYS[slotId]
  if (!localeKey) return slotId
  return localizeFallback(t, `game.fishing.${localeKey}`, slotId)
}

export function resolveFishingTackleSecondaryCategory(
  definition: Pick<FishingTackleDefinition, "slot" | "methods">
): string {
  const slot = String(definition?.slot || "").trim()
  if (slot !== "float_or_sinker") return slot
  const methods = Array.isArray(definition.methods)
    ? definition.methods.map((method) => String(method || "").trim()).filter(Boolean)
    : []
  if (methods.includes("float") && !methods.includes("bottom")) return "float"
  if (methods.includes("bottom") && !methods.includes("float")) return "sinker"
  return slot
}

export function getLocalizedFishingTackleSecondaryCategoryLabel(
  t: Translate,
  definition: Pick<FishingTackleDefinition, "slot" | "methods">
): string {
  return localizeFishingTackleSlot(t, resolveFishingTackleSecondaryCategory(definition))
}

export function buildFishingTackleSecondarySections<T>(params: {
  items: T[]
  getItemId: (item: T) => string
  t: Translate
}): FishingTackleGroupSection<T>[] {
  const { items, getItemId, t } = params
  if (!items.length) return []

  const slotByItemId = new Map<string, string>()
  for (const definition of getGameDataTable<FishingTackleGroupingDefinition>("fishing_tackle")) {
    const itemId = String(definition?.itemId || "").trim()
    const section = resolveFishingTackleSecondaryCategory(definition)
    if (itemId && section) slotByItemId.set(itemId, section)
  }

  const itemsBySlot = new Map<string, T[]>()
  const otherItems: T[] = []
  for (const item of items) {
    const slot = slotByItemId.get(String(getItemId(item) || "").trim())
    if (!slot) {
      otherItems.push(item)
      continue
    }
    if (!itemsBySlot.has(slot)) itemsBySlot.set(slot, [])
    itemsBySlot.get(slot)?.push(item)
  }

  const knownSlots = new Set<string>(FISHING_TACKLE_SLOT_ORDER)
  const orderedSlots = [
    ...FISHING_TACKLE_SLOT_ORDER.filter((slot) => itemsBySlot.has(slot)),
    ...[...itemsBySlot.keys()].filter((slot) => !knownSlots.has(slot)).sort()
  ]
  const sections = orderedSlots.map((slot) => ({
    id: slot,
    label: localizeFishingTackleSlot(t, slot),
    items: itemsBySlot.get(slot) || []
  }))
  if (otherItems.length > 0) {
    sections.push({
      id: "other",
      label: localizeFallback(t, "game.itemSectionOther", "other"),
      items: otherItems
    })
  }
  return sections
}
