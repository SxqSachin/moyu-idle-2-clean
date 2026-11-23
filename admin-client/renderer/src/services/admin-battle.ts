import { adminGet, adminPost } from "./client"

export type BattleActiveRoomStatus = "entering" | "playing" | "recovering"

export type BattleSpecializationRuntimeVersion = {
  treeId: string
  rulesetVersion: string
  balanceRevision: number
}

export type BattleWorkerSpecializationHeartbeat = {
  workerId: string
  refreshedAt?: string
  specialization?: BattleSpecializationRuntimeVersion | null
  matchesLoaded: boolean
}

export type BattleWorkerSpecializationSnapshot = {
  available: boolean
  ready: boolean
  workerCount: number
  matchingWorkers: number
  mismatchedWorkers: number
  missingVersionWorkers: number
  workers: BattleWorkerSpecializationHeartbeat[]
}

export type BattleMetricsResult = {
  metrics?: Record<string, unknown>
  serverMemory?: Record<string, unknown>
  runtime?: {
    appEnv?: string
    globalConfigPath?: string
    specialization?: BattleSpecializationRuntimeVersion | null
    battleWorkers?: BattleWorkerSpecializationSnapshot
    battle?: Record<string, unknown>
  }
}

export type BattleRoomsReinitializeResult = {
  ok: boolean
  reinitRooms: number
  interruptedRuns: number
  runStatus: "interrupted"
  statuses: BattleActiveRoomStatus[]
  specialization: BattleSpecializationRuntimeVersion | null
  battleWorkers: BattleWorkerSpecializationSnapshot
}

export function fetchBattleMetrics() {
  return adminGet<BattleMetricsResult>("battle", "/metrics")
}

export function listSlowRuns(limit = 20) {
  return adminGet<Record<string, unknown>>("battle", "/slow-runs", { limit })
}

export function cleanupSlowRuns(payload: Record<string, unknown>) {
  return adminPost<Record<string, unknown>>("battle", "/slow-runs/cleanup", payload)
}

export function cleanupBattleRuns(payload: Record<string, unknown>) {
  return adminPost<Record<string, unknown>>("battle", "/runs/cleanup", payload)
}

export function reinitializeBattleRooms(payload: {
  statuses: BattleActiveRoomStatus[]
  expectedSpecialization: BattleSpecializationRuntimeVersion
}) {
  return adminPost<BattleRoomsReinitializeResult>("battle", "/rooms/reinitialize", payload)
}
