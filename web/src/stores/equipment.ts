import { defineStore } from "pinia"
import { apiGet } from "@services/api"

export type EquippedItem = {
  instanceId: number
  templateItemId: string
  equipType: "work" | "battle"
  enhanceLevel: number
  durability: number
  maxDurability: number
  state: string
  equippedSlot: string
  updatedAt: string
}

export type EquippedPayload = {
  items?: EquippedItem[]
  upsert?: EquippedItem[]
  remove?: number[]
  refreshRequired?: boolean
  meta?: { version?: number; epoch?: string }
}

type EquipmentStatsPayload = {
  attributes?: Record<string, unknown>
  derived?: Record<string, number>
  updatedAt?: string
}

export type EquipmentOperationTarget = {
  instanceId: number
  templateItemId: string
}

function normalizePatchEpoch(value: unknown): string {
  return typeof value === "string" ? value.trim() : ""
}

export function canApplyEquipmentSnapshot(currentVersion: number, requestVersion: number) {
  return Number.isFinite(currentVersion) && Number.isFinite(requestVersion) && currentVersion === requestVersion
}

export const useEquipmentStore = defineStore("equipment", {
  state: () => ({
    initialized: false,
    equippedItems: [] as EquippedItem[],
    operationTarget: null as EquipmentOperationTarget | null,
    equippedVersion: 0,
    epoch: "",
    snapshotRequestSeq: 0,
    wsRefreshInFlight: false,
    wsRefreshGeneration: 0,
    wsRefreshTargetVersion: 0,
    wsRefreshTargetEpoch: "",
    wsRefreshRunId: 0,
    wsRefreshRequired: false,
    stats: {
      attributes: {} as Record<string, number>,
      derived: {} as Record<string, number>,
      updatedAt: ""
    }
  }),
  actions: {
    needsWSAuthoritativeRefresh(payload: EquippedPayload | null | undefined) {
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
      const requestVersion = this.equippedVersion
      const data = await apiGet<EquippedPayload>("/api/equipment/instances", token)
      if (requestSeq !== this.snapshotRequestSeq) return
      if (this.wsRefreshRequired || !canApplyEquipmentSnapshot(this.equippedVersion, requestVersion)) return
      this.applySnapshot(data.items || [])
    },
    async refreshFromWSInvalidation(token: string, payload: EquippedPayload | null | undefined) {
      if (!token || !payload) return
      const incomingEpoch = normalizePatchEpoch(payload.meta?.epoch)
      const epochChanged = Boolean(incomingEpoch && this.epoch && incomingEpoch !== this.epoch)
      if (payload.refreshRequired || epochChanged) this.wsRefreshRequired = true
      if (!this.wsRefreshRequired) return
      const requestedVersion = Math.max(0, Math.trunc(Number(payload.meta?.version || 0)))
      if (incomingEpoch && incomingEpoch !== this.wsRefreshTargetEpoch) {
        this.wsRefreshTargetEpoch = incomingEpoch
        this.wsRefreshTargetVersion = requestedVersion
      } else {
        this.wsRefreshTargetVersion = Math.max(this.wsRefreshTargetVersion, requestedVersion)
      }
      this.wsRefreshGeneration += 1
      if (this.wsRefreshInFlight) return

      this.wsRefreshInFlight = true
      const runId = this.wsRefreshRunId + 1
      this.wsRefreshRunId = runId
      try {
        const maxAttempts = 4
        for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
          const requestGeneration = this.wsRefreshGeneration
          const requestVersion = this.equippedVersion
          const data = await apiGet<EquippedPayload>("/api/equipment/instances", token)
          if (runId !== this.wsRefreshRunId) return
          if (requestGeneration !== this.wsRefreshGeneration || requestVersion !== this.equippedVersion) {
            continue
          }
          this.applySnapshot(data.items || [], this.wsRefreshTargetVersion, this.wsRefreshTargetEpoch)
          this.wsRefreshRequired = false
          return
        }
        throw new Error("equipment changed continuously during authoritative refresh")
      } finally {
        if (runId === this.wsRefreshRunId) this.wsRefreshInFlight = false
      }
    },
    applySnapshot(items: EquippedItem[], snapshotVersion?: number, snapshotEpoch?: string) {
      this.equippedItems = items
      this.initialized = true
      if (snapshotVersion !== undefined && Number.isFinite(snapshotVersion) && snapshotVersion >= 0) {
        this.equippedVersion = Math.trunc(snapshotVersion)
      }
      if (snapshotEpoch !== undefined) this.epoch = normalizePatchEpoch(snapshotEpoch)
    },
    beginRealtimeSession() {
      this.equippedVersion = 0
      this.epoch = ""
      this.snapshotRequestSeq += 1
      this.wsRefreshInFlight = false
      this.wsRefreshGeneration = 0
      this.wsRefreshTargetVersion = 0
      this.wsRefreshTargetEpoch = ""
      this.wsRefreshRunId += 1
      this.wsRefreshRequired = false
    },
    patchEquippedItems(payload: EquippedPayload | null | undefined) {
      if (!payload || this.wsRefreshRequired || payload.refreshRequired) return
      const incomingEpoch = normalizePatchEpoch(payload.meta?.epoch)
      if (incomingEpoch && this.epoch && incomingEpoch !== this.epoch) {
        this.wsRefreshRequired = true
        this.wsRefreshTargetEpoch = incomingEpoch
        this.wsRefreshTargetVersion = Math.max(0, Math.trunc(Number(payload.meta?.version || 0)))
        return
      }
      if (Array.isArray(payload.items)) {
        const version = Math.max(0, Math.trunc(Number(payload.meta?.version || 0)))
        this.snapshotRequestSeq += 1
        this.applySnapshot(payload.items, version, incomingEpoch || this.epoch)
        return
      }
      const version = Number(payload.meta?.version || 0)
      if (!Number.isFinite(version) || version <= this.equippedVersion) return
      const upsert = Array.isArray(payload.upsert) ? payload.upsert : []
      const remove = Array.isArray(payload.remove) ? payload.remove.filter((id): id is number => typeof id === "number") : []
      const byID = new Map<number, EquippedItem>()
      for (const item of this.equippedItems) {
        byID.set(item.instanceId, item)
      }
      for (const item of upsert) {
        byID.set(item.instanceId, item)
      }
      for (const id of remove) {
        byID.delete(id)
      }
      this.snapshotRequestSeq += 1
      this.equippedItems = Array.from(byID.values())
      this.initialized = true
      this.equippedVersion = version
      if (incomingEpoch) this.epoch = incomingEpoch
    },
    patchStats(payload: EquipmentStatsPayload | null | undefined) {
      if (!payload) return
      const attributes: Record<string, number> = {}
      for (const [key, value] of Object.entries(payload.attributes || {})) {
        const node = value as { value?: unknown } | number
        if (typeof node === "number" && Number.isFinite(node)) {
          attributes[key] = node
          continue
        }
        if (typeof node === "object" && node && typeof node.value === "number" && Number.isFinite(node.value)) {
          attributes[key] = node.value
        }
      }
      this.stats.attributes = attributes
      this.stats.derived = payload.derived || {}
      this.stats.updatedAt = payload.updatedAt || ""
    },
    setOperationTarget(target: EquipmentOperationTarget) {
      this.operationTarget = target
    },
    clearOperationTarget() {
      this.operationTarget = null
    },
    reset() {
      this.initialized = false
      this.equippedItems = []
      this.operationTarget = null
      this.equippedVersion = 0
      this.epoch = ""
      this.snapshotRequestSeq += 1
      this.wsRefreshInFlight = false
      this.wsRefreshGeneration = 0
      this.wsRefreshTargetVersion = 0
      this.wsRefreshTargetEpoch = ""
      this.wsRefreshRunId += 1
      this.wsRefreshRequired = false
      this.stats = { attributes: {}, derived: {}, updatedAt: "" }
    }
  }
})
