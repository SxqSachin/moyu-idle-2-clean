import { reactive, readonly } from "vue"
import { acquireBattleStream, waitForBattleStreamReady } from "@services/battle-stream"

type ArenaReplayOpenOptions = {
  challengeId?: number
  shareToken?: string
  live?: boolean
}

const state = reactive({
  visible: false,
  challengeId: 0,
  shareToken: "",
  live: false,
  version: 0
})
let releaseArenaStream: (() => void) | null = null

function ensureArenaStream() {
  if (!releaseArenaStream) releaseArenaStream = acquireBattleStream("arena")
}

export async function prepareArenaReplayPlayback() {
  ensureArenaStream()
  return waitForBattleStreamReady("arena")
}

export function openArenaReplay(options: ArenaReplayOpenOptions) {
  const challengeId = Math.max(0, Math.trunc(Number(options.challengeId || 0)))
  const shareToken = String(options.shareToken || "").trim()
  const live = Boolean(options.live && challengeId > 0)
  if (!challengeId && !shareToken) return
  if (live) ensureArenaStream()
  state.challengeId = challengeId
  state.shareToken = shareToken
  state.live = live
  state.version += 1
  state.visible = true
}

export function closeArenaReplay() {
  state.visible = false
  state.live = false
  releaseArenaStream?.()
  releaseArenaStream = null
}

export function cancelPreparedArenaReplay() {
  if (state.visible) return
  releaseArenaStream?.()
  releaseArenaStream = null
}

export const arenaReplayViewerState = readonly(state)
