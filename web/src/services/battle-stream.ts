import { bind, send } from "@services/ws"

export type BattleStreamKind = "room" | "arena" | "combat_trial"

const desiredCounts: Record<BattleStreamKind, number> = { room: 0, arena: 0, combat_trial: 0 }
const acknowledged: Record<BattleStreamKind, boolean | null> = { room: null, arena: null, combat_trial: null }
const retryAttempts: Record<BattleStreamKind, number> = { room: 0, arena: 0, combat_trial: 0 }
const retryTimers: Partial<Record<BattleStreamKind, ReturnType<typeof setTimeout>>> = {}
const readyWaiters: Record<BattleStreamKind, Set<(ready: boolean) => void>> = {
  room: new Set(),
  arena: new Set(),
  combat_trial: new Set()
}
let bindingsReady = false

function normalizeKind(raw: unknown): BattleStreamKind {
  const kind = String(raw || "").trim().toLowerCase()
  if (kind === "arena" || kind === "combat_trial") return kind
  return "room"
}

function desired(kind: BattleStreamKind) {
  return desiredCounts[kind] > 0
}

function clearRetry(kind: BattleStreamKind) {
  const timer = retryTimers[kind]
  if (timer) clearTimeout(timer)
  delete retryTimers[kind]
}

function notifyReady(kind: BattleStreamKind, ready: boolean) {
  for (const resolve of readyWaiters[kind]) resolve(ready)
  readyWaiters[kind].clear()
}

function scheduleRetry(kind: BattleStreamKind) {
  clearRetry(kind)
  if (retryAttempts[kind] >= 5 || acknowledged[kind] === desired(kind)) return
  retryTimers[kind] = setTimeout(() => {
    retryAttempts[kind] += 1
    send("battle.stream.set", { kind, enabled: desired(kind) })
    scheduleRetry(kind)
  }, 800)
}

function sync(kind: BattleStreamKind, force = false) {
  const enabled = desired(kind)
  if (!force && acknowledged[kind] === enabled) return
  retryAttempts[kind] = 0
  send("battle.stream.set", { kind, enabled }, { queueIfUnavailable: enabled })
  scheduleRetry(kind)
}

function ensureBindings() {
  if (bindingsReady) return
  bindingsReady = true
  bind("battle.stream.state", (payload) => {
    const state = payload as { kind?: string; enabled?: boolean }
    if (typeof state?.enabled !== "boolean") return
    const kind = normalizeKind(state.kind)
    acknowledged[kind] = state.enabled
    if (state.enabled === desired(kind)) {
      retryAttempts[kind] = 0
      clearRetry(kind)
      if (state.enabled) notifyReady(kind, true)
    } else {
      sync(kind, true)
    }
  })
  bind("auth.bound", () => {
    for (const kind of ["room", "arena", "combat_trial"] as const) {
      acknowledged[kind] = null
      sync(kind, true)
    }
  })
}

export function acquireBattleStream(kind: BattleStreamKind): () => void {
  ensureBindings()
  desiredCounts[kind] += 1
  if (desiredCounts[kind] === 1) sync(kind, true)
  let released = false
  return () => {
    if (released) return
    released = true
    desiredCounts[kind] = Math.max(0, desiredCounts[kind] - 1)
    if (desiredCounts[kind] === 0) sync(kind, true)
  }
}

export function waitForBattleStreamReady(kind: BattleStreamKind, timeoutMs = 1500): Promise<boolean> {
  ensureBindings()
  if (acknowledged[kind] === true && desired(kind)) return Promise.resolve(true)
  return new Promise((resolve) => {
    let settled = false
    const finish = (ready: boolean) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      readyWaiters[kind].delete(finish)
      resolve(ready)
    }
    const timer = setTimeout(() => finish(false), Math.max(100, timeoutMs))
    readyWaiters[kind].add(finish)
    sync(kind, true)
  })
}
