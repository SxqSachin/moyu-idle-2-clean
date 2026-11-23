import { describe, expect, it, vi } from "vitest"
import type { FishingLoadout, FishingOwnedTackle } from "../src/services/fishing"
import { fishingLoadoutEquipmentRequirements } from "../src/services/fishing-loadout-requirements"

vi.mock("../src/services/item-definition", () => ({
  getEquipmentStatusRequirement: (id: string) => ({ rod: { fishing: 8 }, reel: { fishing: 18, strength: 5 }, basic: {} })[id] || {}
}))

const loadout: FishingLoadout = {
  loadoutId: "set", name: "set", method: "float", strategy: "balanced", safetyLoadPercent: 82,
  components: { rodInstanceId: 1, reelInstanceId: 2, mainLineItemId: "line", leaderItemId: "leader" }
}
const owned: FishingOwnedTackle[] = [
  { itemId: "rod", instanceId: 1, slot: "rod", quantity: 1 },
  { itemId: "reel", instanceId: 2, slot: "reel", quantity: 1 }
]

describe("fishing loadout equipment requirements", () => {
  it("checks both instances and all requirements against the chosen actor", () => {
    const low = fishingLoadoutEquipmentRequirements(loadout, owned, { fishing: 7, strength: 5 })
    expect(low.filter((row) => !row.met).map((row) => [row.itemId, row.required, row.current])).toEqual([["rod", 8, 7], ["reel", 18, 7]])
    expect(fishingLoadoutEquipmentRequirements(loadout, owned, { fishing: 18, strength: 5 }).every((row) => row.met)).toBe(true)
    expect(fishingLoadoutEquipmentRequirements(loadout, owned, { fishing: 8, strength: 5 })[0]?.met).toBe(true)
  })

  it("distinguishes unloaded attributes from an absent authoritative attribute", () => {
    expect(fishingLoadoutEquipmentRequirements(loadout, owned, null).every((row) => row.current === null && !row.met)).toBe(true)
    expect(fishingLoadoutEquipmentRequirements(loadout, owned, {}).every((row) => row.current === 0 && !row.met)).toBe(true)
    expect(fishingLoadoutEquipmentRequirements(loadout, owned, { fishing: NaN })[0]?.current).toBeNull()
  })

  it("does not gate requirement-free starter equipment while attributes load", () => {
    expect(fishingLoadoutEquipmentRequirements(loadout, owned.map((row) => ({ ...row, itemId: "basic" })), null)).toEqual([])
  })
})
