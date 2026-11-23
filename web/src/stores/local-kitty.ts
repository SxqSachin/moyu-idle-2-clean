import { defineStore } from "pinia"
import { apiGet } from "@services/api"

export type PublicKittyProfile = {
  userId: number
  username: string
  kittyId: number
  kittyName: string
  kittyCreatedAt?: string
  runningActionId?: string
}

function kittyCacheKey(userId: number, kittyId: number) {
  return `${userId}:${kittyId}`
}

const inflight = new Map<string, Promise<PublicKittyProfile>>()

export const useLocalKittyStore = defineStore("local-kitty", {
  state: () => ({
    byKey: {} as Record<string, PublicKittyProfile>
  }),
  actions: {
    get(userId: number, kittyId: number): PublicKittyProfile | null {
      const normalizedUserID = Math.floor(Number(userId || 0))
      const normalizedKittyID = Math.floor(Number(kittyId || 0))
      if (normalizedUserID <= 0 || normalizedKittyID <= 0) return null
      return this.byKey[kittyCacheKey(normalizedUserID, normalizedKittyID)] || null
    },
    async fetchByKitty(token: string, userId: number, kittyId: number, options?: { force?: boolean }): Promise<PublicKittyProfile> {
      const normalizedUserID = Math.floor(Number(userId || 0))
      const normalizedKittyID = Math.floor(Number(kittyId || 0))
      if (!token || normalizedUserID <= 0 || normalizedKittyID <= 0) {
        throw new Error("invalid kitty profile request")
      }
      const key = kittyCacheKey(normalizedUserID, normalizedKittyID)
      const force = Boolean(options?.force)
      if (!force) {
        const cached = this.byKey[key]
        if (cached && cached.kittyName) return cached
      }
      const current = inflight.get(key)
      if (current) return current
      const task = (async () => {
        const profile = await apiGet<PublicKittyProfile>(
          `/api/cattery/kitty/public?userId=${encodeURIComponent(String(normalizedUserID))}&kittyId=${encodeURIComponent(String(normalizedKittyID))}`,
          token
        )
        this.byKey[key] = profile
        return profile
      })()
      inflight.set(key, task)
      try {
        return await task
      } finally {
        inflight.delete(key)
      }
    }
  }
})
