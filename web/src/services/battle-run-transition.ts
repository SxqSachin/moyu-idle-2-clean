export type BattleIntermissionWait = {
  finishedAtMs: number
  intervalMs: number
  finishedRunId: number
  pendingRunId: number
}

function positiveInteger(value: unknown): number {
  const parsed = Math.trunc(Number(value) || 0)
  return parsed > 0 ? parsed : 0
}

export function markBattleRunPreparing(
  current: BattleIntermissionWait | null,
  startedAtMs: number,
  runId: number,
  previousRunId: number
): BattleIntermissionWait | null {
  const pendingRunId = positiveInteger(runId)
  const currentRunId = positiveInteger(previousRunId)
  if (!pendingRunId) return current
  if (currentRunId > pendingRunId) return current
  if (current?.finishedRunId === pendingRunId || current?.pendingRunId === pendingRunId) return current
  return {
    finishedAtMs: current?.finishedAtMs ?? Math.max(0, Math.trunc(Number(startedAtMs) || 0)),
    intervalMs: current?.intervalMs ?? 0,
    finishedRunId: current?.finishedRunId ?? currentRunId,
    pendingRunId
  }
}

export function battleTransitionCanComplete(wait: BattleIntermissionWait | null, runId: number): boolean {
  if (!wait) return false
  const candidateRunId = positiveInteger(runId)
  if (!candidateRunId) return false
  if (wait.pendingRunId > 0) return wait.pendingRunId === candidateRunId
  return wait.finishedRunId <= 0 || wait.finishedRunId !== candidateRunId
}

function battleRunStatusRank(status: string): number {
  switch (String(status || "").trim().toLowerCase()) {
    case "compiled":
      return 0
    case "playing":
      return 1
    case "settling":
      return 2
    case "ended":
    case "failed":
    case "interrupted":
      return 3
    default:
      return -1
  }
}

export function shouldPreserveBattleRunSnapshot(input: {
  currentRunId: number
  currentStatus: string
  currentSeq: number
  incomingRunId: number
  incomingStatus: string
  incomingSeq: number
}): boolean {
  const currentRunId = positiveInteger(input.currentRunId)
  const incomingRunId = positiveInteger(input.incomingRunId)
  if (!currentRunId || currentRunId !== incomingRunId) return false
  if (Math.max(0, Number(input.currentSeq) || 0) > Math.max(0, Number(input.incomingSeq) || 0)) return true
  return battleRunStatusRank(input.currentStatus) > battleRunStatusRank(input.incomingStatus)
}
