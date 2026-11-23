import { defineStore } from "pinia"
import { apiGet } from "@services/api"
import type { ItemDetailResponse } from "@/types/equipment"

type CacheMap = Record<number, ItemDetailResponse>

const inflight = new Map<number, Promise<ItemDetailResponse>>()

export const useItemInstanceDetailStore = defineStore("item-instance-detail", {
  state: () => ({
    byInstanceID: {} as CacheMap
  }),
  actions: {
    get(instanceId: number): ItemDetailResponse | null {
      if (instanceId <= 0) return null
      return this.byInstanceID[instanceId] || null
    },
    async fetchByInstance(token: string, instanceId: number, options?: { itemId?: string; force?: boolean }): Promise<ItemDetailResponse> {
      if (!token || instanceId <= 0) {
        throw new Error("invalid instance detail request")
      }
      const force = Boolean(options?.force)
      if (!force) {
        const cached = this.byInstanceID[instanceId]
        if (cached) return cached
      }
      const current = inflight.get(instanceId)
      if (current) return current

      const task = (async () => {
        const query = new URLSearchParams()
        query.set("instanceId", String(instanceId))
        const itemID = String(options?.itemId || "").trim()
        if (itemID) {
          query.set("itemId", itemID)
        }
        const detail = await apiGet<ItemDetailResponse>(`/api/items/detail?${query.toString()}`, token)
        this.byInstanceID[instanceId] = detail
        return detail
      })()
      inflight.set(instanceId, task)
      try {
        return await task
      } finally {
        inflight.delete(instanceId)
      }
    },
    invalidate(instanceId?: number) {
      if (typeof instanceId === "number" && instanceId > 0) {
        delete this.byInstanceID[instanceId]
        inflight.delete(instanceId)
        return
      }
      this.byInstanceID = {}
      inflight.clear()
    }
  }
})
