import { beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import type { InventoryInstance } from "../src/types/equipment"

const { apiGet } = vi.hoisted(() => ({ apiGet: vi.fn() }))

vi.mock("@services/api", () => ({ apiGet }))

import { useInventoryStore } from "../src/stores/inventory"

function makeInstance(instanceId: number): InventoryInstance {
  return {
    instanceId,
    templateItemId: `item.${instanceId}`,
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: []
  }
}

describe("inventory oversized ws recovery", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    apiGet.mockReset()
  })

  it("replaces an omitted oversized patch with an authoritative snapshot", async () => {
    apiGet.mockResolvedValueOnce({ items: [makeInstance(1), makeInstance(2)] })
    const store = useInventoryStore()

    await store.refreshFromWSInvalidation("token", {
      refreshRequired: true,
      meta: { version: 7 }
    })

    expect(store.instances.map((item) => item.instanceId)).toEqual([1, 2])
    expect(store.version).toBe(7)
    expect(store.initialized).toBe(true)
    expect(store.wsRefreshRequired).toBe(false)
  })

  it("retries when a newer incremental patch arrives during the snapshot request", async () => {
    let resolveFirst!: (value: unknown) => void
    apiGet
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce({ items: [makeInstance(1), makeInstance(3)] })
    const store = useInventoryStore()
    const refreshing = store.refreshFromWSInvalidation("token", {
      refreshRequired: true,
      meta: { version: 2 }
    })
    await Promise.resolve()
    const later = store.refreshFromWSInvalidation("token", {
      upsert: [makeInstance(3)],
      remove: [],
      meta: { version: 3 }
    })
    resolveFirst({ items: [makeInstance(1)] })
    await refreshing
    await later

    expect(apiGet).toHaveBeenCalledTimes(2)
    expect(store.instances.map((item) => item.instanceId)).toEqual([1, 3])
    expect(store.version).toBe(3)
  })

  it("does not apply later deltas onto a stale snapshot after refresh failure", async () => {
    apiGet
      .mockRejectedValueOnce(new Error("network unavailable"))
      .mockResolvedValueOnce({ items: [makeInstance(1), makeInstance(3)] })
    const store = useInventoryStore()
    store.applySnapshot([makeInstance(1)], 1)

    await expect(store.refreshFromWSInvalidation("token", {
      refreshRequired: true,
      meta: { version: 2 }
    })).rejects.toThrow("network unavailable")
    expect(store.wsRefreshRequired).toBe(true)

    store.patchFromWS({ upsert: [makeInstance(2)], remove: [], meta: { version: 3 } })
    expect(store.instances.map((item) => item.instanceId)).toEqual([1])
    expect(store.version).toBe(1)

    await store.refreshFromWSInvalidation("token", {
      upsert: [makeInstance(2)],
      remove: [],
      meta: { version: 3 }
    })
    expect(store.instances.map((item) => item.instanceId)).toEqual([1, 3])
    expect(store.version).toBe(3)
    expect(store.wsRefreshRequired).toBe(false)
  })

  it("does not apply an old account refresh after reset", async () => {
    let resolveOld!: (value: unknown) => void
    apiGet
      .mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
      .mockResolvedValueOnce({ items: [makeInstance(2)] })
    const store = useInventoryStore()
    const oldRefresh = store.refreshFromWSInvalidation("old-token", {
      refreshRequired: true,
      meta: { version: 1 }
    })
    await Promise.resolve()

    store.reset()
    const newRefresh = store.refreshFromWSInvalidation("new-token", {
      refreshRequired: true,
      meta: { version: 1 }
    })
    await newRefresh
    resolveOld({ items: [makeInstance(1)] })
    await oldRefresh

    expect(store.instances.map((item) => item.instanceId)).toEqual([2])
    expect(store.version).toBe(1)
  })

  it("recovers when a restarted gateway resets its patch version", async () => {
    apiGet.mockResolvedValueOnce({ items: [makeInstance(7)] })
    const store = useInventoryStore()
    store.applySnapshot([makeInstance(1)], 41)
    store.epoch = "old-gateway"
    const restartedPatch = {
      upsert: [makeInstance(2)],
      remove: [],
      meta: { version: 1, epoch: "new-gateway" }
    }

    expect(store.needsWSAuthoritativeRefresh(restartedPatch)).toBe(true)
    await store.refreshFromWSInvalidation("token", restartedPatch)

    expect(store.instances.map((item) => item.instanceId)).toEqual([7])
    expect(store.version).toBe(1)
    expect(store.epoch).toBe("new-gateway")
    store.patchFromWS({ upsert: [makeInstance(8)], remove: [], meta: { version: 2, epoch: "new-gateway" } })
    expect(store.instances.map((item) => item.instanceId)).toEqual([7, 8])
  })

  it("waits for the shared terminal snapshot without a duplicate download", async () => {
    let resolve!: (value: unknown) => void
    apiGet.mockImplementationOnce(() => new Promise((done) => { resolve = done }))
    const store = useInventoryStore()
    const meta = { version: 9, epoch: "gateway" }
    const refresh = store.refreshFromWSInvalidation("token", { refreshRequired: true, meta })
    let ready = false
    const terminal = store.ensureRecoverySnapshot("token", meta).then(() => { ready = true })
    await Promise.resolve()
    expect(ready).toBe(false)
    expect(apiGet).toHaveBeenCalledWith("/api/inventory/instances?fresh=1", "token")
    resolve({ items: [makeInstance(9)] })
    await refresh
    await terminal
    expect(ready).toBe(true)
    expect(apiGet).toHaveBeenCalledTimes(1)
    await store.ensureRecoverySnapshot("token", meta)
    expect(apiGet).toHaveBeenCalledTimes(1)
  })

  it("does not acknowledge failed or malformed terminal inventory", async () => {
    apiGet.mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({})
    const store = useInventoryStore()
    const meta = { version: 9, epoch: "gateway" }
    await expect(store.ensureRecoverySnapshot("token", meta)).rejects.toThrow("offline")
    expect(store.version).toBe(0)
    await expect(store.ensureRecoverySnapshot("token", meta)).rejects.toThrow("invalid inventory snapshot")
    expect(store.wsRefreshRequired).toBe(true)
    expect(store.version).toBe(0)
  })
})
