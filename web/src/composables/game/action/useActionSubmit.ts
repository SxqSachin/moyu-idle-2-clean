import { effectScope, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import i18n from "@/i18n"
import { ACTION_QUEUE_LANE_LIMIT, KITTY_ACTION_QUEUE_LANE_LIMIT } from "@/config/shared-global-config"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { bind, send } from "@services/ws"
import { useGameStore } from "@stores/game"
import { useRealtimeStore } from "@stores/realtime"

type PendingFeedback = "run" | "queue"
type ActionActorType = "player" | "kitty"
type ActionSubmitActor = {
  actorType?: ActionActorType
  actorId?: number
}
export type ActionPlanInsertMode = "append" | "run_now_like_prepend"
type EnqueuePlanResult = {
  ok: boolean
  enqueuedCount: number
  skippedCount: number
  droppedCount: number
  failedActionId?: string
}
type EnqueuePlanBatchOptions = {
  insertMode?: ActionPlanInsertMode
}
type PendingSubmission = {
  mode: PendingFeedback
  resolve: (ok: boolean) => void
  silentSuccess: boolean
  silentError: boolean
}
export type ActionPlanStep = {
  actionId: string
  count: number
  args?: Record<string, unknown>
}

const pendingFeedbackQueue = ref<PendingSubmission[]>([])
let initialized = false

function initActionSubmitFeedback() {
  if (initialized) return
  initialized = true

  const realtimeStore = useRealtimeStore()

  bind("error", (payload) => {
    const code = String(payload?.code || "")
    if (!code.startsWith("WS_ACTION_") && !code.startsWith("WS_BUILDING_") && !code.startsWith("WS_ENHANCE_")) return
    if (!pendingFeedbackQueue.value.length) return
    const pending = pendingFeedbackQueue.value.shift()
    pending?.resolve(false)
    if (pending?.silentError) return
    ElMessage.error(resolveErrorMessage(i18n.global.t, payload, "game.error.generic"))
  })

  const scope = effectScope(true)
  scope.run(() => {
    watch(
      () => realtimeStore.actionQueueVersion,
      () => {
        const submission = pendingFeedbackQueue.value.shift()
        if (!submission) return
        submission.resolve(true)
        if (submission.silentSuccess) return
        if (submission.mode === "run") {
          ElMessage.success(i18n.global.t("game.actionRunSubmitted"))
          return
        }
        ElMessage.success(i18n.global.t("game.actionEnqueueSubmitted"))
      }
    )
  })
}

function normalizeCount(value: number) {
  return Math.max(1, Math.floor(value || 1))
}

function normalizeActorLane(actor?: ActionSubmitActor): { actorType: ActionActorType; actorId: number } {
  const actorType = actor?.actorType === "kitty" ? "kitty" : "player"
  const actorId = actorType === "kitty"
    ? Math.max(1, Math.floor(Number(actor?.actorId || 0)))
    : 0
  return { actorType, actorId }
}

function resolveActionQueueLaneLimit(actorType: ActionActorType) {
  return actorType === "kitty" ? KITTY_ACTION_QUEUE_LANE_LIMIT : ACTION_QUEUE_LANE_LIMIT
}

function normalizePlanInsertMode(mode?: ActionPlanInsertMode): ActionPlanInsertMode {
  return mode === "run_now_like_prepend" ? "run_now_like_prepend" : "append"
}

function countQueuedLaneItems(
  items: Array<{ actorType?: unknown; actorId?: unknown; status?: unknown }>,
  actorType: ActionActorType,
  actorId: number
) {
  return items.reduce((count, item) => {
    const normalizedStatus = String(item.status || "").toLowerCase().trim()
    if (normalizedStatus !== "pending" && normalizedStatus !== "running") return count
    const normalizedType = String(item.actorType || "").toLowerCase().trim() === "kitty" ? "kitty" : "player"
    const normalizedId = normalizedType === "kitty" ? Math.max(1, Math.floor(Number(item.actorId || 0))) : 0
    if (normalizedType !== actorType || normalizedId !== actorId) return count
    return count + 1
  }, 0)
}

export function useActionSubmit() {
  initActionSubmitFeedback()

  function submit(
    actionId: string,
    mode: PendingFeedback,
    count: number,
    args?: Record<string, unknown>,
    actor?: ActionSubmitActor,
    options?: { silentSuccess?: boolean; silentError?: boolean }
  ): Promise<boolean> {
    const actorType = actor?.actorType === "kitty" ? "kitty" : "player"
    const actorId = actorType === "kitty"
      ? Math.max(1, Math.floor(Number(actor?.actorId || 0)))
      : 0
    return new Promise((resolve) => {
      pendingFeedbackQueue.value.push({
        mode,
        resolve,
        silentSuccess: !!options?.silentSuccess,
        silentError: !!options?.silentError
      })
      send(mode === "run" ? "action.run" : "action.enqueue", {
        actionId,
        mode: mode === "run" ? "run" : "queue",
        count: normalizeCount(count),
        ...(args ? { args } : {}),
        actorType,
        actorId
      })
    })
  }

  function runNow(actionId: string, count: number, actor?: ActionSubmitActor) {
    void submit(actionId, "run", count, undefined, actor)
  }

  function enqueue(actionId: string, count: number, actor?: ActionSubmitActor) {
    void submit(actionId, "queue", count, undefined, actor)
  }

  function runNowWithArgs(actionId: string, count: number, args: Record<string, unknown> | undefined, actor?: ActionSubmitActor) {
    void submit(actionId, "run", count, args, actor)
  }

  function enqueueWithArgs(actionId: string, count: number, args: Record<string, unknown> | undefined, actor?: ActionSubmitActor) {
    void submit(actionId, "queue", count, args, actor)
  }

  async function enqueuePlanBatch(
    steps: ActionPlanStep[],
    actor?: ActionSubmitActor,
    options?: EnqueuePlanBatchOptions
  ): Promise<EnqueuePlanResult> {
    const gameStore = useGameStore()
    const realtimeStore = useRealtimeStore()
    const insertMode = normalizePlanInsertMode(options?.insertMode)
    const { actorType, actorId } = normalizeActorLane(actor)
    const laneLimit = resolveActionQueueLaneLimit(actorType)
    const normalizedSteps = steps
      .map((step) => ({
        actionId: String(step.actionId || "").trim(),
        count: normalizeCount(step.count),
        ...(step.args ? { args: step.args } : {})
      }))
      .filter((step) => step.actionId !== "")
    if (!normalizedSteps.length) {
      return { ok: true, enqueuedCount: 0, skippedCount: 0, droppedCount: 0 }
    }
    if (insertMode === "run_now_like_prepend") {
      if (!gameStore.token) {
        ElMessage.error(i18n.global.t("game.actionPlanEnqueueFailed"))
        return { ok: false, enqueuedCount: 0, skippedCount: normalizedSteps.length, droppedCount: 0 }
      }
      try {
        const response = await apiPost<{ ok: boolean; enqueuedCount?: number; droppedCount?: number }>(
          "/api/actions/queue/batch",
          gameStore.token,
          {
            steps: normalizedSteps,
            actorType,
            actorId,
            insertMode
          }
        )
        const enqueuedCount = Math.max(0, Number(response?.enqueuedCount ?? normalizedSteps.length))
        const droppedCount = Math.max(0, Number(response?.droppedCount || 0))
        if (droppedCount > 0) {
          ElMessage.warning(i18n.global.t("game.actionPlanStartNowTrimmed", { count: enqueuedCount, dropped: droppedCount }))
        } else {
          ElMessage.success(i18n.global.t("game.actionPlanStartNowSubmitted", { count: enqueuedCount }))
        }
        return {
          ok: true,
          enqueuedCount,
          skippedCount: 0,
          droppedCount
        }
      } catch (error) {
        ElMessage.error(resolveErrorMessage(i18n.global.t, error, "game.actionPlanEnqueueFailed"))
        return {
          ok: false,
          enqueuedCount: 0,
          skippedCount: normalizedSteps.length,
          droppedCount: 0,
          failedActionId: normalizedSteps[0]?.actionId
        }
      }
    }
    const queuedCount = countQueuedLaneItems(realtimeStore.actionQueue, actorType, actorId)
    const availableSlots = Math.max(0, laneLimit - queuedCount)
    const allowedSteps = normalizedSteps.slice(0, availableSlots)
    const skippedCount = normalizedSteps.length - allowedSteps.length
    if (allowedSteps.length <= 0) {
      ElMessage.warning(i18n.global.t("game.error.actionQueueLimitReached", { limit: laneLimit }))
      return { ok: false, enqueuedCount: 0, skippedCount: normalizedSteps.length, droppedCount: 0 }
    }
    if (allowedSteps.length < normalizedSteps.length || !gameStore.token) {
      const result = await enqueuePlanSequential(allowedSteps, actor)
      if (result.enqueuedCount > 0 && skippedCount > 0) {
        ElMessage.warning(i18n.global.t("game.actionPlanEnqueuePartialSubmitted", {
          count: result.enqueuedCount,
          skipped: skippedCount
        }))
        return {
          ok: true,
          enqueuedCount: result.enqueuedCount,
          skippedCount,
          droppedCount: 0
        }
      }
      if (result.enqueuedCount > 0) {
        ElMessage.success(i18n.global.t("game.actionPlanEnqueueSubmitted", { count: result.enqueuedCount }))
        return {
          ok: true,
          enqueuedCount: result.enqueuedCount,
          skippedCount,
          droppedCount: 0
        }
      }
      if (skippedCount > 0) {
        ElMessage.warning(i18n.global.t("game.error.actionQueueLimitReached", { limit: laneLimit }))
      }
      return {
        ok: false,
        enqueuedCount: 0,
        skippedCount: normalizedSteps.length,
        droppedCount: 0,
        failedActionId: allowedSteps[0]?.actionId
      }
    }
    try {
      const response = await apiPost<{ ok: boolean; enqueuedCount?: number; droppedCount?: number }>("/api/actions/queue/batch", gameStore.token, {
        steps: normalizedSteps,
        actorType,
        actorId,
        insertMode
      })
      const enqueuedCount = Math.max(0, Number(response?.enqueuedCount ?? normalizedSteps.length))
      ElMessage.success(i18n.global.t("game.actionPlanEnqueueSubmitted", { count: enqueuedCount }))
      return { ok: true, enqueuedCount, skippedCount: 0, droppedCount: 0 }
    } catch (error) {
      ElMessage.error(resolveErrorMessage(i18n.global.t, error, "game.actionPlanEnqueueFailed"))
      return {
        ok: false,
        enqueuedCount: 0,
        skippedCount: normalizedSteps.length,
        droppedCount: 0,
        failedActionId: normalizedSteps[0]?.actionId
      }
    }
  }

  async function enqueuePlanSequential(steps: ActionPlanStep[], actor?: ActionSubmitActor): Promise<EnqueuePlanResult> {
    let enqueuedCount = 0
    for (const step of steps) {
      const actionId = String(step.actionId || "").trim()
      const count = normalizeCount(step.count)
      if (!actionId || count <= 0) continue
      const ok = await submit(actionId, "queue", count, step.args, actor, { silentSuccess: true, silentError: true })
      if (!ok) {
        return {
          ok: enqueuedCount > 0,
          enqueuedCount,
          skippedCount: steps.length - enqueuedCount,
          droppedCount: 0,
          failedActionId: actionId
        }
      }
      enqueuedCount += 1
    }
    return {
      ok: true,
      enqueuedCount,
      skippedCount: 0,
      droppedCount: 0
    }
  }

  return {
    submit,
    runNow,
    enqueue,
    runNowWithArgs,
    enqueueWithArgs,
    enqueuePlanBatch,
    enqueuePlanSequential
  }
}
