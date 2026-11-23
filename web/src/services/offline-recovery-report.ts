type Receipt = { taskId: string; revision: number }
type Acknowledgement = { ackTasks: Receipt[] } | { ackTaskId: string; ackRevision: number; ackReportKey?: string }
type Ledger = { reports: string[]; tasks: string[]; throughTaskId?: string; pending?: Acknowledgement }

const ledgers = new Map<number, Ledger>()
const storageKey = (userID: number) => `offline-recovery-reports:v1:${userID}`

function receipts(payload: any): Receipt[] {
  const rows = Array.isArray(payload?.reportTasks) ? payload.reportTasks : [payload]
  if (!rows.length || rows.some((row: any) => typeof row?.taskId !== "string" || !/^[1-9]\d*$/.test(row.taskId)
    || !Number.isSafeInteger(row.revision) || row.revision < 0)) return []
  if (new Set(rows.map((row: Receipt) => row.taskId)).size !== rows.length) return []
  return rows.map((row: Receipt) => ({ taskId: row.taskId, revision: row.revision }))
}

export function offlineRecoveryAcknowledgement(payload: any): Acknowledgement | null {
  const tasks = receipts(payload)
  if (!tasks.length) return null
  if (Array.isArray(payload.reportTasks) && /^[a-f0-9]{64}$/.test(payload.reportKey)) {
    const last = tasks.reduce((a, b) => BigInt(a.taskId) > BigInt(b.taskId) ? a : b)
    return { ackTaskId: last.taskId, ackRevision: last.revision, ackReportKey: payload.reportKey }
  }
  return Array.isArray(payload.reportTasks)
    ? { ackTasks: tasks }
    : { ackTaskId: tasks[0]!.taskId, ackRevision: tasks[0]!.revision }
}

export function isOfflineRecoveryReportReady(payload: any): boolean {
  return payload?.status === "complete" && !payload?.hasMoreDue && !payload?.syncPending
    && payload?.snapshotSynced === true && Number(payload?.combatPendingRuns || 0) === 0
    && offlineRecoveryAcknowledgement(payload) !== null
}

function readLedger(userID: number): Ledger {
  try {
    const raw = JSON.parse(localStorage.getItem(storageKey(userID)) || "null")
    if (raw && Array.isArray(raw.reports) && Array.isArray(raw.tasks)) {
      ledgers.set(userID, raw)
    }
  } catch { /* Storage is optional; the current page still deduplicates. */ }
  return ledgers.get(userID) || { reports: [], tasks: [] }
}

function writeLedger(userID: number, ledger: Ledger) {
  ledgers.set(userID, ledger)
  try { localStorage.setItem(storageKey(userID), JSON.stringify(ledger)) } catch { /* In-memory fallback. */ }
}

export function pendingOfflineRecoveryAcknowledgement(userID: number): Acknowledgement | undefined {
  return userID > 0 ? readLedger(userID).pending : undefined
}

export async function clearOfflineRecoveryAcknowledgement(userID: number, expected?: unknown) {
  if (userID <= 0) return
  await withReportLock(userID, () => {
    const ledger = readLedger(userID)
    if (expected && JSON.stringify(ledger.pending) !== JSON.stringify(expected)) return
    delete ledger.pending
    writeLedger(userID, ledger)
  })
}

async function withReportLock<T>(userID: number, action: () => T): Promise<T> {
  if (typeof navigator !== "undefined" && navigator.locks) {
    try { return await navigator.locks.request(storageKey(userID), action) } catch (error) {
      if (!["SecurityError", "NotSupportedError"].includes((error as DOMException)?.name)) throw error
    }
  }
  return action()
}

// Store only presentation receipts, never rewards or inventory. The server still
// validates ownership, completion and every revision before acknowledging them.
export async function presentOfflineRecoveryReport(
  userID: number, payload: any, isCurrent: () => boolean, present: (open: boolean) => void
): Promise<Acknowledgement | null> {
  const ack = offlineRecoveryAcknowledgement(payload)
  if (userID <= 0 || !ack || !isOfflineRecoveryReportReady(payload)) return null
  const claim = () => {
    if (!isCurrent()) return null
    const ledger = readLedger(userID)
    const tasks = receipts(payload).map((row) => row.taskId)
    const key = String(payload.reportKey || tasks.join(","))
    const through = /^[1-9]\d*$/.test(ledger.throughTaskId || "") ? BigInt(ledger.throughTaskId!) : 0n
    const open = !ledger.reports.includes(key) && tasks.some((task) => BigInt(task) > through && !ledger.tasks.includes(task))
    const latest = tasks.reduce((max, task) => BigInt(task) > max ? BigInt(task) : max, through)
    present(open)
    writeLedger(userID, {
      reports: [...ledger.reports.filter((report) => report !== key), key].slice(-64),
      tasks: [...new Set([...ledger.tasks, ...tasks])].slice(-256),
      throughTaskId: latest.toString(),
      pending: ack
    })
    return ack
  }
  // Serialize the display claim across tabs when supported. Refresh/reconnect
  // replays the pending ACK rather than opening an already displayed report.
  return withReportLock(userID, claim)
}
