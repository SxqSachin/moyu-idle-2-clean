import { beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { ref } from "vue"

const { apiGet } = vi.hoisted(() => ({ apiGet: vi.fn() }))

vi.mock("@services/api", () => ({ apiGet }))

import { useEquipmentSlots } from "../src/composables/game/attributes/useEquipmentSlots"
import { useEquipmentStore, type EquippedItem } from "../src/stores/equipment"
import { useInventoryStore } from "../src/stores/inventory"
import type { EquipmentSlotType, InventoryInstance } from "../src/types/equipment"

function equippedItem(instanceId: number, slot: string): EquippedItem {
  return {
    instanceId,
    templateItemId: "wood_sword",
    equipType: "battle",
    enhanceLevel: 0,
    durability: 10,
    maxDurability: 10,
    state: "equipped",
    equippedSlot: slot,
    updatedAt: "2026-08-30T00:00:00Z"
  }
}

function staleInventoryItem(instanceId: number, slot: string): InventoryInstance {
  return {
    ...equippedItem(instanceId, slot),
    quantity: 1,
    affixes: []
  }
}

describe("player equipment state", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    apiGet.mockReset()
  })

  it("renders from the authoritative equipment snapshot and reacts to patches and removals", () => {
    const equipmentStore = useEquipmentStore()
    const inventoryStore = useInventoryStore()
    const equipType = ref<EquipmentSlotType>("battle")
    const { slotCards } = useEquipmentSlots(equipType, { visibleSlots: ["head", "right_hand"] })

    inventoryStore.applySnapshot([staleInventoryItem(99, "head")])
    equipmentStore.applySnapshot([], 0)
    expect(slotCards.value.find((card) => card.slot === "head")?.instance).toBeNull()

    equipmentStore.patchEquippedItems({
      upsert: [equippedItem(1, "head")],
      remove: [],
      meta: { version: 1, epoch: "gateway-a" }
    })
    expect(slotCards.value.find((card) => card.slot === "head")?.instance?.instanceId).toBe(1)

    equipmentStore.patchEquippedItems({
      upsert: [equippedItem(1, "right_hand")],
      remove: [],
      meta: { version: 2, epoch: "gateway-a" }
    })
    expect(slotCards.value.find((card) => card.slot === "head")?.instance).toBeNull()
    expect(slotCards.value.find((card) => card.slot === "right_hand")?.instance?.instanceId).toBe(1)

    equipmentStore.patchEquippedItems({
      upsert: [],
      remove: [1],
      meta: { version: 3, epoch: "gateway-a" }
    })
    expect(slotCards.value.every((card) => card.instance === null)).toBe(true)
  })

  it("offers explicitly transferable player fishing gear to an empty kitty slot", () => {
    const equipType = ref<EquipmentSlotType>("work")
    const kittyEquipment = ref<EquippedItem[]>([])
    const playerFishingEquipment = ref<EquippedItem[]>([
      {
        instanceId: 101,
        templateItemId: "basic_fishing_rod",
        equipType: "work",
        enhanceLevel: 0,
        durability: 160,
        maxDurability: 160,
        state: "equipped",
        equippedSlot: "fishing_rod",
        updatedAt: "2026-09-01T00:00:00Z"
      },
      {
        instanceId: 102,
        templateItemId: "basic_spinning_reel",
        equipType: "work",
        enhanceLevel: 0,
        durability: 100,
        maxDurability: 100,
        state: "equipped",
        equippedSlot: "fishing_reel",
        updatedAt: "2026-09-01T00:00:00Z"
      }
    ])
    const { slotCards } = useEquipmentSlots(equipType, {
      equippedItems: kittyEquipment,
      transferableEquippedItems: playerFishingEquipment,
      visibleSlots: ["fishing_rod", "fishing_reel"]
    })

    expect(slotCards.value.find((card) => card.slot === "fishing_rod")?.candidates.map((item) => item.instanceId)).toEqual([101])
    expect(slotCards.value.find((card) => card.slot === "fishing_reel")?.candidates.map((item) => item.instanceId)).toEqual([102])

    kittyEquipment.value = [playerFishingEquipment.value[0]!]
    expect(slotCards.value.find((card) => card.slot === "fishing_rod")?.candidates).toEqual([])
  })

  it("accepts a reconnect snapshot that rebases a previously higher patch version", () => {
    const equipmentStore = useEquipmentStore()
    equipmentStore.patchEquippedItems({
      upsert: [equippedItem(1, "head")],
      meta: { version: 8, epoch: "gateway-a" }
    })

    equipmentStore.patchEquippedItems({
      items: [equippedItem(2, "right_hand")],
      meta: { version: 1, epoch: "gateway-a" }
    })
    equipmentStore.patchEquippedItems({
      upsert: [equippedItem(2, "head")],
      meta: { version: 2, epoch: "gateway-a" }
    })

    expect(equipmentStore.equippedVersion).toBe(2)
    expect(equipmentStore.equippedItems).toEqual([equippedItem(2, "head")])
  })

  it("requires an authoritative refresh when an incremental packet comes from another gateway", () => {
    const equipmentStore = useEquipmentStore()
    equipmentStore.patchEquippedItems({
      upsert: [equippedItem(1, "head")],
      meta: { version: 5, epoch: "gateway-a" }
    })
    const nextGatewayPatch = {
      upsert: [equippedItem(2, "right_hand")],
      meta: { version: 1, epoch: "gateway-b" }
    }

    expect(equipmentStore.needsWSAuthoritativeRefresh(nextGatewayPatch)).toBe(true)
    equipmentStore.patchEquippedItems(nextGatewayPatch)
    expect(equipmentStore.wsRefreshRequired).toBe(true)
    expect(equipmentStore.equippedItems.map((item) => item.instanceId)).toEqual([1])
  })

  it("replaces cross-gateway deltas with an authoritative equipment snapshot", async () => {
    apiGet.mockResolvedValueOnce({ items: [equippedItem(2, "right_hand")] })
    const equipmentStore = useEquipmentStore()
    equipmentStore.applySnapshot([equippedItem(1, "head")], 7, "gateway-a")
    const nextGatewayPatch = {
      upsert: [equippedItem(3, "head")],
      meta: { version: 1, epoch: "gateway-b" }
    }

    await equipmentStore.refreshFromWSInvalidation("token", nextGatewayPatch)
    equipmentStore.patchEquippedItems({
      upsert: [equippedItem(2, "head")],
      meta: { version: 2, epoch: "gateway-b" }
    })

    expect(apiGet).toHaveBeenCalledWith("/api/equipment/instances", "token")
    expect(equipmentStore.epoch).toBe("gateway-b")
    expect(equipmentStore.equippedVersion).toBe(2)
    expect(equipmentStore.equippedItems).toEqual([equippedItem(2, "head")])
    expect(equipmentStore.wsRefreshRequired).toBe(false)
  })
})
