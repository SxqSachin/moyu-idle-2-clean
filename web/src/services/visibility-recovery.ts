export type PresenceStatus = "online" | "offline"
export type PresenceStatusSource = "server" | "disconnect"

export type VisibilityRecoveryAction = "online_then_rewards" | "sync_only"

export type VisibilityRecoveryState = {
  presenceStatus: PresenceStatus
  presenceStatusSource?: PresenceStatusSource
  hiddenSinceMs: number
  nowMs: number
  offlineDelayMs: number
}

export function visibilityOfflineDue(hiddenSinceMs: number, nowMs: number, offlineDelayMs: number): boolean {
  return hiddenSinceMs > 0 && (nowMs - hiddenSinceMs) >= Math.max(0, offlineDelayMs)
}

export function resolveVisibilityRecoveryAction(state: VisibilityRecoveryState): VisibilityRecoveryAction {
  if (visibilityOfflineDue(state.hiddenSinceMs, state.nowMs, state.offlineDelayMs)) return "online_then_rewards"
  if (state.presenceStatus === "offline" && state.presenceStatusSource !== "disconnect") return "online_then_rewards"
  return "sync_only"
}
