import { reactive, readonly } from "vue"
import { acquireBattleStream, waitForBattleStreamReady } from "@services/battle-stream"

type CombatTrialReplayOpenOptions = {
  runId?: number
  shareToken?: string
  live?: boolean
}

const state = reactive({
  visible: false,
  runId: 0,
  shareToken: "",
  live: false,
  version: 0
})
let releaseStream: (() => void) | null = null

function ensureStream() {
  if (!releaseStream) releaseStream = acquireBattleStream("combat_trial")
}

export async function prepareCombatTrialReplayPlayback() {
  ensureStream()
  return waitForBattleStreamReady("combat_trial")
}

export function openCombatTrialReplay(options: CombatTrialReplayOpenOptions) {
  const runId = Math.max(0, Math.trunc(Number(options.runId || 0)))
  const shareToken = String(options.shareToken || "").trim()
  const live = Boolean(options.live && runId > 0)
  if (!runId && !shareToken) return
  if (live) ensureStream()
  state.runId = runId
  state.shareToken = shareToken
  state.live = live
  state.version += 1
  state.visible = true
}

export function closeCombatTrialReplay() {
  state.visible = false
  state.live = false
  releaseStream?.()
  releaseStream = null
}

export function cancelPreparedCombatTrialReplay() {
  if (state.visible) return
  releaseStream?.()
  releaseStream = null
}

export const combatTrialReplayViewerState = readonly(state)
