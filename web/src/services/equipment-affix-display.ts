import type { EquipmentAffixSlot } from "@/types/equipment"

const EMPTY_AFFIX_SLOT: Omit<EquipmentAffixSlot, "slotIndex"> = {
  affixId: "",
  rarityId: "",
  effectType: ""
}

export function buildAffixSlotSummary(affixSlotCount: number, affixSlots: EquipmentAffixSlot[]): EquipmentAffixSlot[] {
  if (!Number.isFinite(affixSlotCount) || affixSlotCount <= 0) return []
  return Array.from({ length: affixSlotCount }, (_, slotIndex) => {
    const found = affixSlots.find((slot) => slot.slotIndex === slotIndex)
    if (found) return found
    return { slotIndex, ...EMPTY_AFFIX_SLOT }
  })
}

export function aggregateEquipmentAttrChanges(
  baseAttrs: Record<string, number>,
  affixSlots: EquipmentAffixSlot[]
): Record<string, number> {
  const out: Record<string, number> = {}
  for (const [attrKey, value] of Object.entries(baseAttrs || {})) {
    if (!Number.isFinite(value)) continue
    out[attrKey] = (out[attrKey] || 0) + Number(value)
  }
  for (const slot of affixSlots || []) {
    for (const [attrKey, value] of Object.entries(slot.attrMods || {})) {
      if (!Number.isFinite(value)) continue
      out[attrKey] = (out[attrKey] || 0) + Number(value)
    }
  }
  return out
}

export function formatSignedNumber(value: number): string {
  if (!Number.isFinite(value)) return "0"
  const rounded = Math.round(value * 1000) / 1000
  return `${rounded >= 0 ? "+" : ""}${rounded}`
}

function toPositiveInteger(raw: unknown): number | null {
  const numeric = Number(raw)
  if (!Number.isFinite(numeric)) return null
  const rounded = Math.trunc(numeric)
  return rounded > 0 ? rounded : null
}

export function resolveAffixMaxEffectiveLimits(definition: {
  maxEffectivePerEquipment?: unknown
  maxEffectivePerBody?: unknown
} | null | undefined): {
  maxEffectivePerEquipment: number | null
  maxEffectivePerBody: number | null
} {
  return {
    maxEffectivePerEquipment: toPositiveInteger(definition?.maxEffectivePerEquipment),
    maxEffectivePerBody: toPositiveInteger(definition?.maxEffectivePerBody)
  }
}
