import type { FishingLoadout, FishingOwnedTackle } from "./fishing"
import { getEquipmentStatusRequirement } from "./item-definition"

export type FishingEquipmentRequirement = {
  instanceId: number
  itemId: string
  attribute: string
  required: number
  current: number | null
  met: boolean
}

export function fishingLoadoutEquipmentRequirements(
  loadout: FishingLoadout,
  ownedTackle: FishingOwnedTackle[],
  attributes: Record<string, number> | null
): FishingEquipmentRequirement[] {
  const rows: FishingEquipmentRequirement[] = []
  for (const instanceId of [loadout.components.rodInstanceId, loadout.components.reelInstanceId]) {
    const item = ownedTackle.find((row) => row.instanceId === instanceId)
    if (!item) continue
    for (const [rawKey, required] of Object.entries(getEquipmentStatusRequirement(item.itemId))) {
      const attribute = rawKey.trim()
      if (!attribute || !Number.isFinite(required) || required <= 0) continue
      // Equip validation uses final attributes, not the map-access progress level.
      const value = attributes === null ? null : attributes[attribute] ?? 0
      const current = value !== null && Number.isFinite(value) ? value : null
      rows.push({ instanceId, itemId: item.itemId, attribute, required, current, met: current !== null && current >= required })
    }
  }
  return rows
}
