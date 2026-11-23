import { describe, expect, it } from "vitest"
import {
  battleTransitionCanComplete,
  markBattleRunPreparing,
  shouldPreserveBattleRunSnapshot,
  type BattleIntermissionWait
} from "@/services/battle-run-transition"

describe("battle run transition", () => {
  it("keeps the intermission until the newly started run is playable", () => {
    const waiting: BattleIntermissionWait = {
      finishedAtMs: 1_000,
      intervalMs: 3_000,
      finishedRunId: 41,
      pendingRunId: 0
    }
    expect(battleTransitionCanComplete(waiting, 41)).toBe(false)
    expect(battleTransitionCanComplete(waiting, 42)).toBe(true)

    const preparing = markBattleRunPreparing(waiting, 4_000, 42, 41)

    expect(preparing?.pendingRunId).toBe(42)
    expect(markBattleRunPreparing(preparing, 5_000, 42, 41)).toBe(preparing)
    expect(battleTransitionCanComplete(preparing, 41)).toBe(false)
    expect(battleTransitionCanComplete(preparing, 42)).toBe(true)
  })

  it("ignores a delayed start event for the run that just finished", () => {
    const waiting: BattleIntermissionWait = {
      finishedAtMs: 1_000,
      intervalMs: 3_000,
      finishedRunId: 41,
      pendingRunId: 0
    }

    expect(markBattleRunPreparing(waiting, 4_000, 41, 0)).toBe(waiting)
  })

  it("ignores a delayed start event after a newer run is already selected", () => {
    expect(markBattleRunPreparing(null, 4_000, 41, 42)).toBeNull()
  })

  it("creates a preparation state for an initial run", () => {
    const preparing = markBattleRunPreparing(null, 5_000, 7, 0)

    expect(preparing).toEqual({
      finishedAtMs: 5_000,
      intervalMs: 0,
      finishedRunId: 0,
      pendingRunId: 7
    })
  })

  it("does not let a late HTTP snapshot regress a live websocket run", () => {
    expect(shouldPreserveBattleRunSnapshot({
      currentRunId: 9,
      currentStatus: "playing",
      currentSeq: 0,
      incomingRunId: 9,
      incomingStatus: "compiled",
      incomingSeq: 0
    })).toBe(true)
    expect(shouldPreserveBattleRunSnapshot({
      currentRunId: 9,
      currentStatus: "playing",
      currentSeq: 8,
      incomingRunId: 9,
      incomingStatus: "playing",
      incomingSeq: 3
    })).toBe(true)
    expect(shouldPreserveBattleRunSnapshot({
      currentRunId: 9,
      currentStatus: "playing",
      currentSeq: 8,
      incomingRunId: 10,
      incomingStatus: "compiled",
      incomingSeq: 0
    })).toBe(false)
  })
})
