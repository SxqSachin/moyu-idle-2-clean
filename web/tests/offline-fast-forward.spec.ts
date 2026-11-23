import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { describe, expect, it } from "vitest"
import {
  createOfflineFastForwardRequestPayload,
  createOfflineFastForwardRetryPayload,
  isOfflineFastForwardResultFailure,
  isOfflineFastForwardResultForRequest,
  isOfflineFastForwardResultStalled,
  mergeOfflineRewardsPayload,
  shouldReplaceOfflineFastForwardAccumulator
} from "../src/services/offline-fast-forward"

describe("offline fast-forward result protocol", () => {
  it("replaces cumulative receipts instead of adding retries or overlap", () => {
    const first = { taskId: "15", revision: 2, cumulative: true, completedSteps: 2000, itemRewards: { wood: 2000 } }
    const next = { ...first, revision: 3, completedSteps: 3000, itemRewards: { wood: 3000 } }
    expect(mergeOfflineRewardsPayload(first, first)).toEqual(first)
    expect(mergeOfflineRewardsPayload(first, next)).toEqual(next)
    expect(mergeOfflineRewardsPayload(next, first)).toEqual(next)
    expect(mergeOfflineRewardsPayload(null, next)).toEqual(next)
  })

  it("replaces late battle totals at the same action revision", () => {
    const first = { taskId: "15", revision: 4, cumulative: true, combatItemRewards: { ore: 3 }, combatPendingRuns: 1 }
    const late = { ...first, combatItemRewards: { ore: 8 }, combatPendingRuns: 0 }
    expect(mergeOfflineRewardsPayload(first, late)).toEqual(late)
    expect(mergeOfflineRewardsPayload(late, late).combatItemRewards.ore).toBe(8)
  })

  it("bounds polling and keeps acknowledgement independent of websocket request IDs", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    expect(source).toContain("offlineRecoveryCommand = acknowledgement")
    expect(source).toContain("Math.max(250, Math.min(5000, delay))")
    expect(source).toContain("Date.now() - offlineRecoveryLastProgressAt >= 30_000")
    expect(source).toContain("clearOfflineRecoveryPoll()")
  })

  it("awaits authoritative inventory before recording the synced receipt or ACK", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    const terminal = source.slice(source.indexOf("realtimeStore.patchFastForwardRewards(mergedPayload)"), source.indexOf('registerBinding("auth.bound"'))
    const wait = terminal.indexOf("await inventoryStore.ensureRecoverySnapshot")
    expect(wait).toBeGreaterThan(0)
    expect(wait).toBeLessThan(terminal.indexOf("offlineRecoverySynced ="))
    expect(wait).toBeLessThan(terminal.indexOf("await presentOfflineRecoveryReport"))
    const failure = terminal.slice(terminal.indexOf("catch (error)"), terminal.indexOf("offlineRecoverySynced ="))
    expect(failure).toContain("pollOfflineRecovery(5000, true)")
    expect(failure).toContain("return")
  })

  it("loads gameplay independently and automatically shows the complete report without a toolbar entry", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    expect(source).toContain('v-if="!gameEntryReady"')
    expect(source).not.toContain('(offlineRewardsLoading && !offlineRecoveryBrowse)')
    expect(source).toContain('pollOfflineRecovery(1500, true)')
    expect(source).not.toContain('openOfflineReport')
    expect(source).not.toContain('offlineReportReady')
    expect(source).not.toContain('v-if="offlineRewardsLoading && offlineRecoveryPaused"')
    const entry = source.slice(source.indexOf('function requestEntrySyncBundle()'), source.indexOf('function requestOfflineRecoverySnapshot()'))
    expect(entry.indexOf('send("game.state.get")')).toBeLessThan(entry.indexOf('requestOfflineRewards()'))
    const report = source.slice(source.indexOf('function showFastForwardRewards('), source.indexOf('function normalizeActorType('))
    expect(report).toContain('fastForwardDialogVisible.value = true')
    expect(report).not.toContain('offlineDurationSeconds > 0 && offlineDurationSeconds < 60')
    const terminal = source.slice(source.indexOf('realtimeStore.patchFastForwardRewards(mergedPayload)'), source.indexOf('registerBinding("auth.bound"'))
    expect(terminal.indexOf('!isOfflineRecoveryReportReady(payload)')).toBeLessThan(terminal.indexOf('await presentOfflineRecoveryReport'))
    expect(terminal).toContain('if (!isCurrent()) return')
    expect(terminal).toContain('showFastForwardRewards(mergedPayload, open)')
    expect(report).toContain('if (open) fastForwardDialogVisible.value = true')
    expect(terminal).toContain('offlineRecoverySnapshotTask = snapshotKey')
  })
  it("wakes on committed completion without starting concurrent or acknowledged reads", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    expect(source).toContain('registerBinding("action.offline.recovery.ready", wakeOfflineRecovery)')
    expect(source).toContain('registerBinding("battle.run.finished"')
    const wake = source.slice(source.indexOf('function wakeOfflineRecovery()'), source.indexOf('function requestOfflineRewards('))
    expect(wake).toContain('offlineRecoveryCommand.ackTasks || offlineRewardsInFlight')
    expect(wake).toContain('pollOfflineRecovery(250, true)')
  })
  it("rejects legacy success-shaped error results", () => {
    expect(isOfflineFastForwardResultFailure({ errorCode: "WS_ACTION_QUEUE_TIMEOUT", hasMoreDue: false })).toBe(true)
    expect(isOfflineFastForwardResultFailure({ status: "failed_before_commit" })).toBe(true)
  })

  it("accepts committed progress and completion results", () => {
    expect(isOfflineFastForwardResultFailure({ status: "progress" })).toBe(false)
    expect(isOfflineFastForwardResultFailure({ status: "complete", errorCode: "" })).toBe(false)
    expect(isOfflineFastForwardResultFailure({ status: "committed_sync_pending" })).toBe(false)
  })

  it("stops automatic retries when an executor reports no progress", () => {
    expect(isOfflineFastForwardResultStalled({ status: "stalled", hasMoreDue: true })).toBe(true)
    expect(isOfflineFastForwardResultStalled({ status: "progress", hasMoreDue: true })).toBe(false)
  })

  it("only permits an explicit retry of an identified stalled revision", () => {
    const stalled = { status: "stalled", taskId: "9007199254740993", revision: 0 }
    expect(createOfflineFastForwardRetryPayload(stalled)).toEqual({ retryTaskId: stalled.taskId, retryRevision: 0 })
    expect(createOfflineFastForwardRetryPayload({ ...stalled, revision: 3 })).toEqual({ retryTaskId: stalled.taskId, retryRevision: 3 })
    for (const status of ["progress", "complete", "retryable", "failed_before_commit"]) {
      expect(createOfflineFastForwardRetryPayload({ ...stalled, status })).toBeNull()
    }
    for (const revision of [undefined, null, "1", -1, 1.5, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1]) {
      expect(createOfflineFastForwardRetryPayload({ ...stalled, revision })).toBeNull()
    }
    for (const taskId of [undefined, "", "0", "-1", "1.5", 123, "NaN"]) {
      expect(createOfflineFastForwardRetryPayload({ ...stalled, taskId })).toBeNull()
    }
    expect(createOfflineFastForwardRetryPayload({ ...stalled, errorCode: "OFFLINE_RECOVERY_UNAVAILABLE" })).toBeNull()
  })

  it("exposes stalled recovery without automatically restarting or acknowledging it", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    expect(source).toContain('v-if="offlineRecoveryRetryTarget" role="status"')
    expect(source).toContain('@click="retryOfflineRecovery"')
    const retry = source.slice(source.indexOf('function retryOfflineRecovery()'), source.indexOf('function requestOfflineRewards('))
    expect(retry).toContain('offlineRewardsInFlight.value')
    expect(retry).toContain('offlineRecoveryCommand = { ...offlineRecoveryRetryTarget.value }')
    const stalled = source.slice(source.indexOf('if (stalled) {'), source.indexOf('realtimeStore.patchFastForwardRewards(mergedPayload)'))
    expect(stalled).toContain('pollOfflineRecovery(5000, true)')
    expect(stalled).not.toContain('retryOfflineRecovery()')
    expect(stalled).not.toContain('ackTaskId')
  })

  it("requests terminal-only projection with the versioned protocol", () => {
    expect(createOfflineFastForwardRequestPayload()).toEqual({
      protocolVersion: 3,
      projectionPolicy: "terminal",
      reportPolicy: "combined"
    })
  })

  it("drops stale correlated results while accepting legacy uncorrelated results", () => {
    expect(isOfflineFastForwardResultForRequest({ requestId: "request-2" }, "request-1")).toBe(false)
    expect(isOfflineFastForwardResultForRequest({ requestId: "request-1" }, "request-1")).toBe(true)
    expect(isOfflineFastForwardResultForRequest({ status: "complete" }, "request-1")).toBe(true)
    expect(isOfflineFastForwardResultForRequest({ protocolVersion: 3, status: "complete" }, "request-1")).toBe(false)
    expect(isOfflineFastForwardResultForRequest({ requestId: "request-1" }, "")).toBe(false)
  })

  it("does not merge reward summaries from different offline windows", () => {
    expect(shouldReplaceOfflineFastForwardAccumulator({ targetAtMS: 100 }, { targetAtMS: 100 })).toBe(false)
    expect(shouldReplaceOfflineFastForwardAccumulator({ targetAtMS: 100 }, { targetAtMS: 200 })).toBe(true)
    expect(shouldReplaceOfflineFastForwardAccumulator({ targetAtMS: 100 }, { targetAtMS: 0 })).toBe(true)
    expect(shouldReplaceOfflineFastForwardAccumulator(null, { targetAtMS: 200 })).toBe(true)
  })

  it("merges every cross-chunk reward field without losing terminal metadata", () => {
    const base = {
      status: "progress",
      targetAtMS: 100,
      completedSteps: 2,
      itemRewards: { herb: 2 },
      itemCosts: { coal: 1 },
      attrExp: { strength: 1 },
      actorSummaries: [{ actorType: "kitty", actorId: 5, completedSteps: 1, itemRewards: { fish: 1 } }],
      workLoadoutUsages: [{ actionId: "collect", loadoutId: "a" }],
      finalWorkLoadoutId: "a",
      queueCompleted: [1],
      queueCanceled: [],
      enhanceResults: [{ queueId: 8, attemptNo: 1 }],
      offlineDurationSeconds: 120,
      combatItemRewards: { iron: 1 },
      fishingCatches: {
        carp: {
          landed: 1,
          kept: 1,
          released: 0,
          totalWeightKg: 1,
          mutationGroups: [{ mutations: ["gold"], landed: 1, kept: 1, released: 0 }]
        }
      },
      fishingMutations: []
    }
    const next = {
      status: "complete",
      targetAtMS: 100,
      completedSteps: 3,
      itemRewards: { herb: 1, ore: 4 },
      itemCosts: { coal: 2 },
      attrExp: { strength: 2 },
      actorSummaries: [{ actorType: "kitty", actorId: 5, completedSteps: 2, itemRewards: { fish: 2 } }],
      workLoadoutUsages: [{ actionId: "mine", loadoutId: "b" }],
      finalWorkLoadoutId: "b",
      queueCompleted: [2],
      queueCanceled: [{ queueId: 3, actionId: "mine", reason: "COST_INSUFFICIENT" }],
      enhanceResults: [{ queueId: 8, attemptNo: 2 }],
      offlineDurationSeconds: 90,
      combatItemRewards: { iron: 2 },
      fishingCatches: {
        carp: {
          landed: 2,
          kept: 0,
          released: 2,
          totalWeightKg: 0.5,
          mutationGroups: [{ mutations: ["GOLD"], landed: 1, kept: 0, released: 1 }]
        }
      },
      fishingMutations: [{ id: "gold" }]
    }

    const merged = mergeOfflineRewardsPayload(base, next)

    expect(merged.status).toBe("complete")
    expect(merged.completedSteps).toBe(5)
    expect(merged.itemRewards).toEqual({ herb: 3, ore: 4 })
    expect(merged.itemCosts).toEqual({ coal: 3 })
    expect(merged.attrExp).toEqual({ strength: 3 })
    expect(merged.actorSummaries).toEqual([{
      actorType: "kitty",
      actorId: 5,
      completedSteps: 3,
      itemRewards: { fish: 3 },
      itemCosts: {},
      attrExp: {}
    }])
    expect(merged.workLoadoutUsages).toHaveLength(2)
    expect(merged.finalWorkLoadoutId).toBe("b")
    expect(merged.queueCompleted).toEqual([1, 2])
    expect(merged.queueCanceled).toHaveLength(1)
    expect(merged.enhanceResults).toHaveLength(2)
    expect(merged.offlineDurationSeconds).toBe(120)
    expect(merged.combatItemRewards).toEqual({ iron: 3 })
    expect(merged.fishingCatches.carp).toMatchObject({
      landed: 3,
      kept: 1,
      released: 2,
      totalWeightKg: 1.5,
      mutatedLanded: 2,
      mutatedKept: 1,
      mutatedReleased: 1
    })
    expect(merged.fishingCatches.carp.mutationGroups).toEqual([
      { mutations: ["gold"], landed: 2, kept: 1, released: 1 }
    ])
    expect(merged.fishingMutations).toEqual([{ id: "gold" }])
  })

  it("does not advance or discard a durable recovery on request timeout", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    const timeoutAt = source.indexOf("offlineRewardsRequestTimer = window.setTimeout")
    const requestEnd = source.indexOf("\n  return true", timeoutAt)

    expect(timeoutAt).toBeGreaterThan(0)
    expect(requestEnd).toBeGreaterThan(timeoutAt)
    expect(source.slice(timeoutAt, requestEnd)).toContain("offlineRecoveryPaused.value = true")
    expect(source.slice(timeoutAt, requestEnd)).not.toContain("finishOfflineRecoverySync()")
  })

  it("keeps the loading surface mounted between continuation chunks", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    const handlerAt = source.indexOf('registerBinding("action.offline.rewards.result"')
    const handlerEnd = source.indexOf('registerBinding("auth.bound"', handlerAt)
    const handler = source.slice(handlerAt, handlerEnd)

    expect(handlerAt).toBeGreaterThan(0)
    expect(handlerEnd).toBeGreaterThan(handlerAt)
    expect(handler).toContain("const continuing = hasMoreDue && !stalled")
    expect(handler).toContain("preserveLoading: continuing")
  })

  it("lets the correlated result, not an uncorrelated error event, finish the request", () => {
    const source = readFileSync(resolve(process.cwd(), "src/views/game/shell/index.vue"), "utf8")
    const handlerAt = source.indexOf('registerBinding("error"')
    const handlerEnd = source.indexOf('document.addEventListener("visibilitychange"', handlerAt)
    const handler = source.slice(handlerAt, handlerEnd)

    expect(handlerAt).toBeGreaterThan(0)
    expect(handlerEnd).toBeGreaterThan(handlerAt)
    expect(handler).not.toContain("completeOfflineRewardsRequest")
    expect(handler).not.toContain("finishOfflineRecoverySync")
  })
})
