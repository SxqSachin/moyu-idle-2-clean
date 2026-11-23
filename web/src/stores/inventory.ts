import { defineStore } from "pinia"
import { apiGet } from "@services/api"
import type { InventoryInstance } from "@/types/equipment"

export type { InventoryInstance } from "@/types/equipment"

type InventoryInstancesPayload = {
  items?: InventoryInstance[]
  upsert?: InventoryInstance[]
  remove?: number[]
  refreshRequired?: boolean
  meta?: { version?: number; epoch?: string }
}

const pendingWSRefreshes = new WeakMap<object, Promise<void>>()

function normalizePatchEpoch(value: unknown): string {
  return typeof value === "string" ? value.trim() : ""
}

export function canApplyInventorySnapshot(
  currentVersion: number,
  requestVersion: number,
  snapshotVersion: number,
  allowUnversioned = false
) {
  if (!Number.isFinite(currentVersion) || !Number.isFinite(requestVersion)) return false
  if (currentVersion !== requestVersion) return false
  if (Number.isFinite(snapshotVersion) && snapshotVersion > 0) {
    return snapshotVersion > currentVersion
  }
  return allowUnversioned || currentVersion === 0
}

export const useInventoryStore = defineStore("inventory", {
  state: () => ({
    initialized: false,
    instances: [] as InventoryInstance[],
    version: 0,
    epoch: "",
    snapshotRequestSeq: 0,
    wsRefreshInFlight: false,
    wsRefreshGeneration: 0,
    wsRefreshTargetVersion: 0,
    wsRefreshTargetEpoch: "",
    wsRefreshRunId: 0,
    wsRefreshRequired: false
  }),
  getters: {
    instancesById: (state) => {
      const byID = new Map<number, InventoryInstance>()
      for (const item of state.instances) {
        byID.set(item.instanceId, item)
      }
      return byID
    }
  },
  actions: {
    needsWSAuthoritativeRefresh(payload: InventoryInstancesPayload | null | undefined) {
      if (!payload) return false
      const incomingEpoch = normalizePatchEpoch(payload.meta?.epoch)
      return Boolean(payload.refreshRequired || this.wsRefreshRequired || (incomingEpoch && this.epoch && incomingEpoch !== this.epoch))
    },
    async initialize(token: string, force = false) {
      if (!token) {
        this.reset()
        return
      }
      if (this.initialized && !force) return
      await this.refreshAll(token)
    },
    async refreshAll(token: string) {
      const requestSeq = this.snapshotRequestSeq + 1
      this.snapshotRequestSeq = requestSeq
      const requestVersion = this.version
      const data = await apiGet<InventoryInstancesPayload>("/api/inventory/instances", token)
      if (requestSeq !== this.snapshotRequestSeq) return
      if (!canApplyInventorySnapshot(this.version, requestVersion, 0, true)) return
      this.applySnapshot(data.items || [], requestVersion)
    },
    refreshFromWSInvalidation(token: string, payload: InventoryInstancesPayload | null | undefined): Promise<void> {
      if (!token || !payload) return Promise.resolve()
      const incomingEpoch = normalizePatchEpoch(payload.meta?.epoch)
      const epochChanged = Boolean(incomingEpoch && this.epoch && incomingEpoch !== this.epoch)
      if (payload.refreshRequired || epochChanged) this.wsRefreshRequired = true
      if (!this.wsRefreshRequired) return Promise.resolve()
      const requestedVersion = Math.max(0, Math.trunc(Number(payload.meta?.version || 0)))
      const targetChanged = !requestedVersion || incomingEpoch !== this.wsRefreshTargetEpoch || requestedVersion > this.wsRefreshTargetVersion
      if (incomingEpoch && incomingEpoch !== this.wsRefreshTargetEpoch) {
        this.wsRefreshTargetEpoch = incomingEpoch
        this.wsRefreshTargetVersion = requestedVersion
      } else {
        this.wsRefreshTargetVersion = Math.max(this.wsRefreshTargetVersion, requestedVersion)
      }
      if (targetChanged || !this.wsRefreshInFlight) this.wsRefreshGeneration += 1
      if (this.wsRefreshInFlight) return pendingWSRefreshes.get(this) || Promise.resolve()

      this.wsRefreshInFlight = true
      const runId = this.wsRefreshRunId + 1
      this.wsRefreshRunId = runId
      const task = (async () => {
        try {
          const maxAttempts = 4
          for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
            const requestGeneration = this.wsRefreshGeneration
            const requestVersion = this.version
            const data = await apiGet<InventoryInstancesPayload>("/api/inventory/instances?fresh=1", token)
            if (runId !== this.wsRefreshRunId) return
            if (requestGeneration !== this.wsRefreshGeneration || requestVersion !== this.version) {
              continue
            }
            if (!Array.isArray(data.items)) throw new Error("invalid inventory snapshot")
            this.applySnapshot(data.items)
            this.version = this.wsRefreshTargetVersion
            if (this.wsRefreshTargetEpoch) this.epoch = this.wsRefreshTargetEpoch
            this.wsRefreshRequired = false
            return
          }
          throw new Error("inventory changed continuously during authoritative refresh")
        } finally {
          if (runId === this.wsRefreshRunId) {
            this.wsRefreshInFlight = false
            pendingWSRefreshes.delete(this)
          }
        }
      })()
      pendingWSRefreshes.set(this, task)
      return task
    },
    async ensureRecoverySnapshot(token: string, meta: InventoryInstancesPayload["meta"]): Promise<void> {
      if (!meta) return
      const version = Number(meta.version)
      const epoch = normalizePatchEpoch(meta.epoch)
      if (!Number.isSafeInteger(version) || version <= 0 || !epoch) throw new Error("invalid recovery inventory version")
      if (!this.wsRefreshRequired && this.epoch === epoch && this.version >= version) return
      await this.refreshFromWSInvalidation(token, { refreshRequired: true, meta })
      if (this.wsRefreshRequired || this.epoch !== epoch || this.version < version) {
        throw new Error("recovery inventory snapshot is not synchronized")
      }
    },
    applySnapshot(items: InventoryInstance[], snapshotVersion = 0) {
      this.instances = items
      this.initialized = true
      if (Number.isFinite(snapshotVersion) && snapshotVersion > this.version) {
        this.version = snapshotVersion
      }
    },
    patchFromWS(payload: InventoryInstancesPayload | null | undefined) {
      if (!payload) return
      if (this.wsRefreshRequired || payload.refreshRequired) return
      const incomingEpoch = normalizePatchEpoch(payload.meta?.epoch)
      if (incomingEpoch && this.epoch && incomingEpoch !== this.epoch) {
        this.wsRefreshRequired = true
        this.wsRefreshTargetEpoch = incomingEpoch
        this.wsRefreshTargetVersion = Math.max(0, Math.trunc(Number(payload.meta?.version || 0)))
        return
      }
      if (Array.isArray(payload.items)) {
        const snapshotVersion = Number(payload.meta?.version || 0)
        if (!canApplyInventorySnapshot(this.version, this.version, snapshotVersion)) return
        this.applySnapshot(payload.items, snapshotVersion)
        if (incomingEpoch) this.epoch = incomingEpoch
        return
      }
      const version = Number(payload.meta?.version || 0)
      if (!Number.isFinite(version) || version <= this.version) return
      const upsert = Array.isArray(payload.upsert) ? payload.upsert : []
      const remove = Array.isArray(payload.remove) ? payload.remove.filter((id): id is number => typeof id === "number") : []
      if (upsert.length === 0 && remove.length === 0) {
        this.version = version
        if (incomingEpoch) this.epoch = incomingEpoch
        return
      }
      const byID = new Map<number, InventoryInstance>()
      for (const item of this.instances) {
        byID.set(item.instanceId, item)
      }
      for (const item of upsert) {
        byID.set(item.instanceId, item)
      }
      for (const id of remove) {
        byID.delete(id)
      }
      this.instances = Array.from(byID.values())
      this.initialized = true
      this.version = version
      if (incomingEpoch) this.epoch = incomingEpoch
    },
    reset() {
      pendingWSRefreshes.delete(this)
      this.instances = []
      this.initialized = false
      this.version = 0
      this.epoch = ""
      this.snapshotRequestSeq += 1
      this.wsRefreshInFlight = false
      this.wsRefreshGeneration = 0
      this.wsRefreshTargetVersion = 0
      this.wsRefreshTargetEpoch = ""
      this.wsRefreshRunId += 1
      this.wsRefreshRequired = false
    }
  }
})
