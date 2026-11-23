import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { canApplyInventorySnapshot, useInventoryStore } from "../src/stores/inventory"
import { useWarehouseStore } from "../src/stores/warehouse"
import type { InventoryInstance } from "../src/types/equipment"

function makeInstance(instanceId: number, templateItemId = "skillbook.alpha"): InventoryInstance {
  return {
    instanceId,
    templateItemId,
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: []
  }
}

function makeArchivedInstance(instanceId: number): InventoryInstance {
  return {
    ...makeInstance(instanceId),
    archivedAt: "2026-06-29T00:00:00Z"
  }
}

describe("inventory store versioning", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("accepts initial unversioned snapshot and rejects stale unversioned refreshes after ws versioning starts", () => {
    expect(canApplyInventorySnapshot(0, 0, 0, true)).toBe(true)
    expect(canApplyInventorySnapshot(2, 2, 0, true)).toBe(true)
    expect(canApplyInventorySnapshot(2, 2, 0, false)).toBe(false)
    expect(canApplyInventorySnapshot(2, 1, 0, true)).toBe(false)
  })

  it("keeps newer ws patch over stale full snapshot", () => {
    const store = useInventoryStore()

    store.patchFromWS({
      upsert: [makeInstance(1)],
      remove: [],
      meta: { version: 1 }
    })
    expect(store.version).toBe(1)
    expect(store.instances.map((item) => item.instanceId)).toEqual([1])

    store.patchFromWS({
      items: [makeInstance(2)]
    })
    expect(store.version).toBe(1)
    expect(store.instances.map((item) => item.instanceId)).toEqual([1])

    store.patchFromWS({
      upsert: [makeInstance(3)],
      remove: [],
      meta: { version: 2 }
    })
    expect(store.version).toBe(2)
    expect(store.instances.map((item) => item.instanceId).sort((a, b) => a - b)).toEqual([1, 3])
  })

  it("preserves archive metadata from ws patches", () => {
    const store = useInventoryStore()

    store.patchFromWS({
      upsert: [makeArchivedInstance(8)],
      remove: [],
      meta: { version: 1 }
    })

    expect(store.instances).toHaveLength(1)
    expect(store.instances[0].archivedAt).toBe("2026-06-29T00:00:00Z")
  })

  it("refuses a delta from a different gateway epoch until an authoritative refresh", () => {
    const store = useInventoryStore()
    store.patchFromWS({ upsert: [makeInstance(1)], remove: [], meta: { version: 8, epoch: "gateway-a" } })
    store.patchFromWS({ upsert: [makeInstance(2)], remove: [], meta: { version: 1, epoch: "gateway-b" } })

    expect(store.instances.map((item) => item.instanceId)).toEqual([1])
    expect(store.version).toBe(8)
    expect(store.wsRefreshRequired).toBe(true)
    expect(store.wsRefreshTargetVersion).toBe(1)
    expect(store.wsRefreshTargetEpoch).toBe("gateway-b")
  })
})

describe("warehouse log category cache", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("restores cached log items when switching back to a category", async () => {
    const store = useWarehouseStore()

    store.applySnapshot(
      [
        {
          id: 1,
          eventType: "system_to_warehouse",
          category: "action_queue",
          warehouseId: 1,
          instanceId: 0,
          templateItemId: "wood",
          quantity: 3,
          sourceSurface: "action_queue",
          targetSurface: "inventory",
          createdAt: "2026-05-25T00:00:00Z"
        }
      ],
      "action_queue"
    )
    store.applySnapshot(
      [
        {
          id: 2,
          eventType: "system_to_warehouse",
          category: "battle",
          warehouseId: 1,
          instanceId: 0,
          templateItemId: "iron",
          quantity: 1,
          sourceSurface: "battle_room",
          targetSurface: "inventory",
          createdAt: "2026-05-25T00:01:00Z"
        }
      ],
      "battle"
    )

    await store.initialize("token", false, "action_queue")

    expect(store.activeCategory).toBe("action_queue")
    expect(store.logItems.map((item) => item.id)).toEqual([1])
  })
})
