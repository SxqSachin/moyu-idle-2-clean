type OfflineFastForwardResult = {
  errorCode?: unknown
  requestId?: unknown
  status?: unknown
  targetAtMS?: unknown
  taskId?: unknown
  revision?: unknown
  cumulative?: unknown
  protocolVersion?: unknown
}

export type FastForwardFishingMutationTally = {
  mutations: string[]
  landed: number
  kept: number
  released: number
}

export type FastForwardFishingCatchTally = {
  landed: number
  kept: number
  released: number
  totalWeightKg: number
  mutatedLanded?: number
  mutatedKept?: number
  mutatedReleased?: number
  mutationGroups?: FastForwardFishingMutationTally[]
}

export const OFFLINE_FAST_FORWARD_PROTOCOL_VERSION = 3
export const OFFLINE_FAST_FORWARD_PROJECTION_POLICY = "terminal"

export function createOfflineFastForwardRequestPayload() {
  return {
    protocolVersion: OFFLINE_FAST_FORWARD_PROTOCOL_VERSION,
    projectionPolicy: OFFLINE_FAST_FORWARD_PROJECTION_POLICY,
    reportPolicy: "combined"
  }
}

function asOfflineFastForwardResult(payload: unknown): OfflineFastForwardResult {
  if (!payload || typeof payload !== "object") return {}
  return payload as OfflineFastForwardResult
}

export function isOfflineFastForwardResultFailure(payload: unknown): boolean {
  const result = asOfflineFastForwardResult(payload)
  return String(result.errorCode || "").trim() !== ""
    || String(result.status || "").trim() === "failed_before_commit"
}

export function isOfflineFastForwardResultStalled(payload: unknown): boolean {
  return String(asOfflineFastForwardResult(payload).status || "").trim() === "stalled"
}

export function createOfflineFastForwardRetryPayload(payload: unknown) {
  const result = asOfflineFastForwardResult(payload)
  if (!isOfflineFastForwardResultStalled(result) || isOfflineFastForwardResultFailure(result)) return null
  if (typeof result.taskId !== "string" || !/^[1-9]\d*$/.test(result.taskId)) return null
  if (typeof result.revision !== "number" || !Number.isSafeInteger(result.revision) || result.revision < 0) return null
  return { retryTaskId: result.taskId, retryRevision: result.revision }
}

export function isOfflineFastForwardResultForRequest(payload: unknown, activeRequestId: string): boolean {
  const active = String(activeRequestId || "").trim()
  if (!active) return false
  const responseRequestId = String(asOfflineFastForwardResult(payload).requestId || "").trim()
  return responseRequestId === active || (responseRequestId === "" && Number(asOfflineFastForwardResult(payload).protocolVersion || 0) < 3)
}

export function shouldReplaceOfflineFastForwardAccumulator(base: unknown, next: unknown): boolean {
  const baseTarget = Number(asOfflineFastForwardResult(base).targetAtMS || 0)
  const nextTarget = Number(asOfflineFastForwardResult(next).targetAtMS || 0)
  if (baseTarget === nextTarget) return false
  return baseTarget > 0 || nextTarget > 0
}

function mergeNumberRecord(base: Record<string, number>, next: Record<string, number>) {
  const out: Record<string, number> = { ...(base || {}) }
  for (const [key, value] of Object.entries(next || {})) {
    const delta = Number(value || 0)
    if (!Number.isFinite(delta) || delta === 0) continue
    out[key] = Number(out[key] || 0) + delta
  }
  return out
}

const fastForwardFishingMutationGroupLimit = 24

function normalizedFastForwardMutationIDs(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return [...new Set(raw
    .map((value) => String(value || "").trim().toLowerCase())
    .filter(Boolean))]
    .sort()
}

export function normalizedFastForwardMutationGroups(raw: unknown): FastForwardFishingMutationTally[] {
  if (!Array.isArray(raw)) return []
  const byKey = new Map<string, FastForwardFishingMutationTally>()
  for (const value of raw) {
    if (!value || typeof value !== "object") continue
    const row = value as Partial<FastForwardFishingMutationTally>
    const mutations = normalizedFastForwardMutationIDs(row.mutations)
    const landed = Math.max(0, Math.floor(Number(row.landed || 0)))
    if (!mutations.length || landed <= 0) continue
    const key = mutations.join("\u0000")
    const current = byKey.get(key)
    if (current) {
      current.landed += landed
      current.kept += Math.max(0, Math.floor(Number(row.kept || 0)))
      current.released += Math.max(0, Math.floor(Number(row.released || 0)))
      continue
    }
    if (byKey.size >= fastForwardFishingMutationGroupLimit) continue
    byKey.set(key, {
      mutations,
      landed,
      kept: Math.max(0, Math.floor(Number(row.kept || 0))),
      released: Math.max(0, Math.floor(Number(row.released || 0)))
    })
  }
  return [...byKey.values()]
}

function mergeFishingCatchTallies(base: Record<string, FastForwardFishingCatchTally>, next: Record<string, FastForwardFishingCatchTally>) {
  const out: Record<string, FastForwardFishingCatchTally> = {}
  for (const source of [base || {}, next || {}]) {
    for (const [speciesId, raw] of Object.entries(source)) {
      const landed = Math.max(0, Math.floor(Number(raw?.landed || 0)))
      if (!speciesId || landed <= 0) continue
      const current = out[speciesId] || { landed: 0, kept: 0, released: 0, totalWeightKg: 0 }
      const mutationGroups = normalizedFastForwardMutationGroups(raw?.mutationGroups)
      const groupedLanded = mutationGroups.reduce((sum, group) => sum + group.landed, 0)
      const groupedKept = mutationGroups.reduce((sum, group) => sum + group.kept, 0)
      const groupedReleased = mutationGroups.reduce((sum, group) => sum + group.released, 0)
      out[speciesId] = {
        landed: current.landed + landed,
        kept: current.kept + Math.max(0, Math.floor(Number(raw?.kept || 0))),
        released: current.released + Math.max(0, Math.floor(Number(raw?.released || 0))),
        totalWeightKg: Number(current.totalWeightKg || 0) + Math.max(0, Number(raw?.totalWeightKg || 0)),
        mutatedLanded: Number(current.mutatedLanded || 0) + Math.max(groupedLanded, Math.max(0, Math.floor(Number(raw?.mutatedLanded || 0)))),
        mutatedKept: Number(current.mutatedKept || 0) + Math.max(groupedKept, Math.max(0, Math.floor(Number(raw?.mutatedKept || 0)))),
        mutatedReleased: Number(current.mutatedReleased || 0) + Math.max(groupedReleased, Math.max(0, Math.floor(Number(raw?.mutatedReleased || 0)))),
        mutationGroups: normalizedFastForwardMutationGroups([...(current.mutationGroups || []), ...mutationGroups])
      }
    }
  }
  return out
}

function normalizeActorType(raw: unknown): "player" | "kitty" {
  return String(raw || "").trim().toLowerCase() === "kitty" ? "kitty" : "player"
}

function mergeActorSummaries(baseRows: any[], nextRows: any[]) {
  const byKey = new Map<string, any>()
  for (const row of [...(baseRows || []), ...(nextRows || [])]) {
    const actorType = normalizeActorType(row?.actorType)
    const actorId = actorType === "kitty" ? Math.max(1, Math.floor(Number(row?.actorId || 0))) : 0
    const key = `${actorType}:${actorId}`
    const current = byKey.get(key) || {
      actorType,
      actorId,
      completedSteps: 0,
      itemRewards: {},
      itemCosts: {},
      attrExp: {}
    }
    current.completedSteps += Math.max(0, Math.floor(Number(row?.completedSteps || 0)))
    current.itemRewards = mergeNumberRecord(current.itemRewards, row?.itemRewards || {})
    current.itemCosts = mergeNumberRecord(current.itemCosts, row?.itemCosts || {})
    current.attrExp = mergeNumberRecord(current.attrExp, row?.attrExp || {})
    if (row?.ownerAttrExp) current.ownerAttrExp = mergeNumberRecord(current.ownerAttrExp || {}, row.ownerAttrExp)
    byKey.set(key, current)
  }
  return Array.from(byKey.values())
}

export function mergeOfflineRewardsPayload(base: any, next: any) {
  if (next?.cumulative === true) {
    if (base?.taskId === next.taskId && Number(base?.revision || 0) > Number(next.revision || 0)) return base
    return next
  }
  if (!base || typeof base !== "object") return next || {}
  if (!next || typeof next !== "object") return base
  return {
    ...next,
    completedSteps: Number(base.completedSteps || 0) + Number(next.completedSteps || 0),
    itemRewards: mergeNumberRecord(base.itemRewards || {}, next.itemRewards || {}),
    itemCosts: mergeNumberRecord(base.itemCosts || {}, next.itemCosts || {}),
    attrExp: mergeNumberRecord(base.attrExp || {}, next.attrExp || {}),
    actorSummaries: mergeActorSummaries(base.actorSummaries || [], next.actorSummaries || []),
    workLoadoutUsages: [...(Array.isArray(base.workLoadoutUsages) ? base.workLoadoutUsages : []), ...(Array.isArray(next.workLoadoutUsages) ? next.workLoadoutUsages : [])],
    finalWorkLoadoutId: String(next.finalWorkLoadoutId || base.finalWorkLoadoutId || ""),
    queueCompleted: [...(Array.isArray(base.queueCompleted) ? base.queueCompleted : []), ...(Array.isArray(next.queueCompleted) ? next.queueCompleted : [])],
    queueCanceled: [...(Array.isArray(base.queueCanceled) ? base.queueCanceled : []), ...(Array.isArray(next.queueCanceled) ? next.queueCanceled : [])],
    enhanceResults: [...(Array.isArray(base.enhanceResults) ? base.enhanceResults : []), ...(Array.isArray(next.enhanceResults) ? next.enhanceResults : [])],
    offlineDurationSeconds: Math.max(Number(base.offlineDurationSeconds || 0), Number(next.offlineDurationSeconds || 0)),
    combatItemRewards: mergeNumberRecord(base.combatItemRewards || {}, next.combatItemRewards || {}),
    fishingCatches: mergeFishingCatchTallies(base.fishingCatches || {}, next.fishingCatches || {}),
    fishingMutations: Array.isArray(next.fishingMutations) && next.fishingMutations.length > 0
      ? next.fishingMutations
      : Array.isArray(base.fishingMutations) ? base.fishingMutations : []
  }
}
