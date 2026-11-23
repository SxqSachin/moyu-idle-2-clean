import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  clearOfflineRecoveryAcknowledgement,
  isOfflineRecoveryReportReady,
  offlineRecoveryAcknowledgement,
  pendingOfflineRecoveryAcknowledgement,
  presentOfflineRecoveryReport
} from "../src/services/offline-recovery-report"

const report = {
  taskId: "102", revision: 3, cumulative: true, reportKey: "combined-101-102",
  reportTasks: [{ taskId: "101", revision: 2 }, { taskId: "102", revision: 3 }],
  status: "complete", snapshotSynced: true, combatPendingRuns: 0,
  completedSteps: 30, itemRewards: { wood: 10, ore: 20 }
}
let user = 100
beforeEach(() => {
  user++
  const values = new Map<string, string>()
  vi.stubGlobal("localStorage", { getItem: (key: string) => values.get(key), setItem: (key: string, value: string) => values.set(key, value) })
  vi.stubGlobal("navigator", {})
})

describe("offline report delivery", () => {
  it("presents the cumulative backlog once and acknowledges every exact member", async () => {
    const present = vi.fn()
    expect(await presentOfflineRecoveryReport(user, report, () => true, present)).toEqual({ ackTasks: report.reportTasks })
    expect(present).toHaveBeenLastCalledWith(true)
    expect(pendingOfflineRecoveryAcknowledgement(user)).toEqual({ ackTasks: report.reportTasks })
    await presentOfflineRecoveryReport(user, report, () => true, present)
    expect(present).toHaveBeenLastCalledWith(false)
    await clearOfflineRecoveryAcknowledgement(user)
    expect(pendingOfflineRecoveryAcknowledgement(user)).toBeUndefined()
  })

  it("survives a page reload with a lost ACK without reopening the report", async () => {
    await presentOfflineRecoveryReport(user, report, () => true, () => {})
    vi.resetModules()
    const reloaded = await import("../src/services/offline-recovery-report")
    const present = vi.fn()
    expect(reloaded.pendingOfflineRecoveryAcknowledgement(user)).toEqual({ ackTasks: report.reportTasks })
    await reloaded.presentOfflineRecoveryReport(user, report, () => true, present)
    expect(present).toHaveBeenCalledWith(false)
  })

  it("does not hide new absences or another account's report", async () => {
    const present = vi.fn()
    await presentOfflineRecoveryReport(user, report, () => true, present)
    await presentOfflineRecoveryReport(user, { ...report, taskId: "103", reportKey: "new-103", reportTasks: [{ taskId: "103", revision: 0 }] }, () => true, present)
    expect(present).toHaveBeenLastCalledWith(true)
    await presentOfflineRecoveryReport(user + 1000, report, () => true, present)
    expect(present).toHaveBeenLastCalledWith(true)
  })

  it("waits for actions, battle receipts and authoritative inventory before displaying or ACKing", async () => {
    const present = vi.fn()
    for (const patch of [{ status: "progress" }, { status: "stalled" }, { hasMoreDue: true }, { syncPending: true }, { snapshotSynced: false }, { combatPendingRuns: 1 }, { reportTasks: [] }]) {
      expect(isOfflineRecoveryReportReady({ ...report, ...patch })).toBe(false)
      expect(await presentOfflineRecoveryReport(user, { ...report, ...patch }, () => true, present)).toBeNull()
      expect(pendingOfflineRecoveryAcknowledgement(user)).toBeUndefined()
    }
    expect(present).not.toHaveBeenCalled()
    await presentOfflineRecoveryReport(user, report, () => true, present)
    expect(present).toHaveBeenCalledOnce()
  })

  it("fences stale asynchronous results after a newer response, disconnect or unmount", async () => {
    let generation = 1
    let release: (() => void) | undefined
    vi.stubGlobal("navigator", { locks: { request: (_: string, claim: () => unknown) => new Promise((resolve) => { release = () => resolve(claim()) }) } })
    const present = vi.fn()
    const stale = presentOfflineRecoveryReport(user, report, () => generation === 1, present)
    generation++
    release!()
    expect(await stale).toBeNull()
    expect(present).not.toHaveBeenCalled()
    expect(pendingOfflineRecoveryAcknowledgement(user)).toBeUndefined()
  })

  it("serializes concurrent tab display claims using shared storage", async () => {
    let tail = Promise.resolve<unknown>(undefined)
    vi.stubGlobal("navigator", { locks: { request: (_: string, claim: () => unknown) => { tail = tail.then(claim); return tail } } })
    vi.resetModules()
    const otherTab = await import("../src/services/offline-recovery-report")
    const present = vi.fn()
    await Promise.all([
      presentOfflineRecoveryReport(user, report, () => true, present),
      otherTab.presentOfflineRecoveryReport(user, report, () => true, present)
    ])
    expect(present.mock.calls.map(([open]) => open)).toEqual([true, false])
  })

  it("retains in-page deduplication when storage is unavailable", async () => {
    vi.stubGlobal("localStorage", { getItem: () => { throw new Error("blocked") }, setItem: () => { throw new Error("full") } })
    const present = vi.fn()
    await presentOfflineRecoveryReport(user, report, () => true, present)
    await presentOfflineRecoveryReport(user, report, () => true, present)
    expect(present.mock.calls.map(([open]) => open)).toEqual([true, false])
  })

  it("keeps the old-server acknowledgement fallback and rejects invalid member identities", () => {
    expect(offlineRecoveryAcknowledgement({ taskId: "9007199254740993", revision: 0 })).toEqual({ ackTaskId: "9007199254740993", ackRevision: 0 })
    for (const reportTasks of [[], [{ taskId: "1", revision: -1 }], [{ taskId: 1, revision: 0 }], [report.reportTasks[0], report.reportTasks[0]]]) {
      expect(offlineRecoveryAcknowledgement({ ...report, reportTasks })).toBeNull()
    }
  })

  it("uses a bounded digest ACK and does not reopen a partially acknowledged large backlog", async () => {
    const reportTasks = Array.from({ length: 2000 }, (_, index) => ({ taskId: String(index + 1), revision: 3 }))
    const big = { ...report, reportKey: "a".repeat(64), reportTasks }
    const ack = offlineRecoveryAcknowledgement(big)
    expect(ack).toEqual({ ackTaskId: "2000", ackRevision: 3, ackReportKey: big.reportKey })
    expect(JSON.stringify(ack).length).toBeLessThan(200)
    const present = vi.fn()
    await presentOfflineRecoveryReport(user, big, () => true, present)
    await presentOfflineRecoveryReport(user, { ...big, reportKey: "b".repeat(64), reportTasks: reportTasks.slice(1) }, () => true, present)
    expect(present.mock.calls.map(([open]) => open)).toEqual([true, false])
  })

  it("does not erase a newer tab's pending acknowledgement after an older response", async () => {
    const old = await presentOfflineRecoveryReport(user, report, () => true, () => {})
    const latest = await presentOfflineRecoveryReport(user, { ...report, reportKey: "new-103", reportTasks: [{ taskId: "103", revision: 3 }] }, () => true, () => {})
    await clearOfflineRecoveryAcknowledgement(user, old)
    expect(pendingOfflineRecoveryAcknowledgement(user)).toEqual(latest)
  })

  it("falls back when the browser exposes locks but denies access", async () => {
    vi.stubGlobal("navigator", { locks: { request: () => Promise.reject(new DOMException("denied", "SecurityError")) } })
    const present = vi.fn()
    const ack = await presentOfflineRecoveryReport(user, report, () => true, present)
    expect(ack).not.toBeNull()
    expect(present).toHaveBeenCalledWith(true)
    await clearOfflineRecoveryAcknowledgement(user, ack)
    expect(pendingOfflineRecoveryAcknowledgement(user)).toBeUndefined()
  })
})
