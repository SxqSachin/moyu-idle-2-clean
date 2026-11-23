import { defineStore } from "pinia"
import type { QueueCardDisplay } from "@components/game/queue/types"

export type RealtimeQueueItem = {
  id: number
  actorType: "player" | "kitty"
  actorId: number
  actionId: string
  status: "pending" | "running" | "done" | "failed"
  totalCount: number
  doneCount: number
  runStartAt?: string
  runEndsAt?: string
  display?: QueueCardDisplay
}

type QueuePayload = {
  items?: RealtimeQueueItem[]
}

type BattleRoomSelfPayload = {
  room?: {
    status?: string
  }
}

type AttributePayload = Record<string, unknown>

export type FastForwardRewardsPayload = {
  completedSteps?: number
  itemRewards?: Record<string, number>
  itemCosts?: Record<string, number>
  attrExp?: Record<string, number>
  actorSummaries?: Array<{
    actorType?: "player" | "kitty" | string
    actorId?: number
    completedSteps?: number
    itemRewards?: Record<string, number>
    itemCosts?: Record<string, number>
    attrExp?: Record<string, number>
    ownerAttrExp?: Record<string, number>
  }>
  queueCompleted?: number[]
  queueCanceled?: Array<{ queueId: number; actionId: string; reason: string }>
  offlineDurationSeconds?: number
  enhanceResults?: RealtimeEnhanceResult[]
  combatItemRewards?: Record<string, number>
  budgetExhausted?: boolean
  hasMoreDue?: boolean
  settledUntilMS?: number
  targetAtMS?: number
}

export type RealtimeEnhanceResult = {
  queueId: number
  actionId: string
  attemptNo: number
  equipmentInstanceId: number
  beforeLevel: number
  afterLevel: number
  success: boolean
  usedProtect: boolean
  baseSuccessRate?: number
  successRateBonus?: number
  successRate: number
  costs: Record<string, number>
  deltaAttrs: Record<string, number>
  stopReason?: string
}

const realtimeEnhanceResultLimit = 100

function mergeEnhanceResults(current: RealtimeEnhanceResult[], incoming: RealtimeEnhanceResult[]): RealtimeEnhanceResult[] {
  const byAttempt = new Map<string, RealtimeEnhanceResult>()
  for (const result of [...current, ...incoming]) {
    const key = `${result.queueId}:${result.attemptNo}:${result.equipmentInstanceId}`
    byAttempt.delete(key)
    byAttempt.set(key, result)
  }
  return Array.from(byAttempt.values()).slice(-realtimeEnhanceResultLimit)
}

export const useRealtimeStore = defineStore("realtime", {
  state: () => ({
    actionQueue: [] as RealtimeQueueItem[],
    actionQueueVersion: 0,
    battleRoomStatus: null as string | null,
    attributes: {} as Record<string, number>,
    attributesVersion: 0,
    fastForwardRewards: null as FastForwardRewardsPayload | null,
    fastForwardRewardsVersion: 0,
    enhanceResults: [] as RealtimeEnhanceResult[],
    serverClockSkewMs: null as number | null
  }),
  actions: {
    patchActionQueue(payload: QueuePayload | null | undefined) {
      this.actionQueue = payload?.items || []
      this.actionQueueVersion += 1
    },
    setBattleRoomStatus(status: string | null | undefined) {
      const normalized = typeof status === "string" ? status.trim() : ""
      this.battleRoomStatus = normalized || null
    },
    patchBattleRoomSelf(payload: BattleRoomSelfPayload | null | undefined) {
      this.setBattleRoomStatus(payload?.room?.status)
    },
    patchAttributes(payload: AttributePayload | null | undefined) {
      if (!payload || typeof payload !== "object") return
      const patchVersion = Number((payload.meta as { version?: unknown } | undefined)?.version || 0)
      const upsert = payload.upsert
      const remove = payload.remove
      if (patchVersion > 0 && (upsert !== undefined || remove !== undefined)) {
        if (patchVersion <= this.attributesVersion) return
        const next = { ...this.attributes }
        if (upsert && typeof upsert === "object") {
          for (const [key, value] of Object.entries(upsert as Record<string, unknown>)) {
            if (!value || typeof value !== "object") continue
            const node = value as Record<string, unknown>
            if (typeof node.value !== "number" || !Number.isFinite(node.value)) continue
            next[key] = node.value
          }
        }
        if (Array.isArray(remove)) {
          for (const key of remove) {
            if (typeof key !== "string") continue
            delete next[key]
          }
        }
        this.attributes = next
        this.attributesVersion = patchVersion
        return
      }
      const rawAttributes = payload.attributes
      if (!rawAttributes || typeof rawAttributes !== "object") return
      const next: Record<string, number> = {}
      for (const [key, value] of Object.entries(rawAttributes as Record<string, unknown>)) {
        if (!value || typeof value !== "object") continue
        const node = value as Record<string, unknown>
        if (typeof node.value !== "number" || !Number.isFinite(node.value)) continue
        next[key] = node.value
      }
      this.attributes = next
    },
    patchFastForwardRewards(payload: FastForwardRewardsPayload | null | undefined) {
      if (!payload || typeof payload !== "object") return
      this.fastForwardRewards = {
        completedSteps: Number(payload.completedSteps || 0),
        itemRewards: payload.itemRewards || {},
        itemCosts: payload.itemCosts || {},
        attrExp: payload.attrExp || {},
        actorSummaries: Array.isArray(payload.actorSummaries) ? payload.actorSummaries : [],
        queueCompleted: Array.isArray(payload.queueCompleted) ? payload.queueCompleted : [],
        queueCanceled: Array.isArray(payload.queueCanceled) ? payload.queueCanceled : [],
        offlineDurationSeconds: Number(payload.offlineDurationSeconds || 0),
        enhanceResults: Array.isArray(payload.enhanceResults) ? payload.enhanceResults : [],
        combatItemRewards: (payload.combatItemRewards && typeof payload.combatItemRewards === "object") ? payload.combatItemRewards as Record<string, number> : {},
        budgetExhausted: Boolean(payload.budgetExhausted),
        hasMoreDue: Boolean(payload.hasMoreDue),
        settledUntilMS: Number(payload.settledUntilMS || 0),
        targetAtMS: Number(payload.targetAtMS || 0)
      }
      if (Array.isArray(payload.enhanceResults) && payload.enhanceResults.length > 0) {
        this.enhanceResults = mergeEnhanceResults(this.enhanceResults, payload.enhanceResults)
      }
      this.fastForwardRewardsVersion += 1
    },
    appendEnhanceResult(payload: RealtimeEnhanceResult | null | undefined) {
      if (!payload || typeof payload !== "object") return
      this.enhanceResults = mergeEnhanceResults(this.enhanceResults, [payload])
    },
    clearEnhanceResults(instanceId?: number) {
      if (!instanceId || instanceId <= 0) {
        this.enhanceResults = []
        return
      }
      this.enhanceResults = this.enhanceResults.filter((row) => row.equipmentInstanceId !== instanceId)
    },
    setServerClockSkewMs(skew: number) {
      if (this.serverClockSkewMs === null) {
        this.serverClockSkewMs = skew
      } else {
        // EMA (α=0.3): resists transient inflation from stale reconnect-backlog messages
        this.serverClockSkewMs = Math.round(this.serverClockSkewMs * 0.7 + skew * 0.3)
      }
    },
    reset() {
      this.actionQueue = []
      this.actionQueueVersion = 0
      this.battleRoomStatus = null
      this.attributes = {}
      this.attributesVersion = 0
      this.fastForwardRewards = null
      this.fastForwardRewardsVersion = 0
      this.enhanceResults = []
      this.serverClockSkewMs = null
    }
  }
})
