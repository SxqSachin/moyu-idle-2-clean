import type { EquipmentAffixSlot } from "@/types/equipment"

export function hasAffixCapacity(affixSlotCount: number): boolean {
  return Number.isFinite(affixSlotCount) && affixSlotCount > 0
}

export function getOccupiedAffixSlotIndexSet(affixSlots: EquipmentAffixSlot[], affixSlotCount: number): Set<number> {
  const occupied = new Set<number>()
  if (!hasAffixCapacity(affixSlotCount)) return occupied
  for (const slot of affixSlots || []) {
    if (!Number.isInteger(slot.slotIndex)) continue
    if (slot.slotIndex < 0 || slot.slotIndex >= affixSlotCount) continue
    if (!String(slot.affixId || "").trim()) continue
    occupied.add(slot.slotIndex)
  }
  return occupied
}

export function getFilledAffixSlots(affixSlots: EquipmentAffixSlot[], affixSlotCount: number): EquipmentAffixSlot[] {
  const occupied = getOccupiedAffixSlotIndexSet(affixSlots, affixSlotCount)
  return (affixSlots || []).filter((slot) => occupied.has(slot.slotIndex))
}

export function getEmptyAffixSlotIndexes(affixSlots: EquipmentAffixSlot[], affixSlotCount: number): number[] {
  if (!hasAffixCapacity(affixSlotCount)) return []
  const occupied = getOccupiedAffixSlotIndexSet(affixSlots, affixSlotCount)
  return Array.from({ length: affixSlotCount }, (_, idx) => idx).filter((idx) => !occupied.has(idx))
}

export function hasAnyAffix(affixSlots: EquipmentAffixSlot[], affixSlotCount: number): boolean {
  return getOccupiedAffixSlotIndexSet(affixSlots, affixSlotCount).size > 0
}

export function isAffixFull(affixSlots: EquipmentAffixSlot[], affixSlotCount: number): boolean {
  if (!hasAffixCapacity(affixSlotCount)) return false
  return getOccupiedAffixSlotIndexSet(affixSlots, affixSlotCount).size >= affixSlotCount
}
