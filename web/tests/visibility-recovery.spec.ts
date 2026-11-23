import { describe, expect, it } from "vitest"
import { resolveVisibilityRecoveryAction, visibilityOfflineDue } from "../src/services/visibility-recovery"

describe("visibility recovery state machine", () => {
  const offlineDelayMs = 120_000
  const nowMs = 1_000_000

  it("keeps short hidden intervals in online sync mode", () => {
    const hiddenSinceMs = nowMs - 30_000
    expect(visibilityOfflineDue(hiddenSinceMs, nowMs, offlineDelayMs)).toBe(false)
    expect(resolveVisibilityRecoveryAction({
      presenceStatus: "online",
      hiddenSinceMs,
      nowMs,
      offlineDelayMs
    })).toBe("sync_only")
  })

  it("recovers through online then rewards after offline timer ran", () => {
    expect(resolveVisibilityRecoveryAction({
      presenceStatus: "offline",
      presenceStatusSource: "server",
      hiddenSinceMs: nowMs - 30_000,
      nowMs,
      offlineDelayMs
    })).toBe("online_then_rewards")
  })

  it("recovers through online then rewards when hidden timer was suspended", () => {
    const hiddenSinceMs = nowMs - 15 * 60_000
    expect(visibilityOfflineDue(hiddenSinceMs, nowMs, offlineDelayMs)).toBe(true)
    expect(resolveVisibilityRecoveryAction({
      presenceStatus: "online",
      hiddenSinceMs,
      nowMs,
      offlineDelayMs
    })).toBe("online_then_rewards")
  })

  it("keeps short visible reconnects out of visibility recovery", () => {
    expect(resolveVisibilityRecoveryAction({
      presenceStatus: "offline",
      presenceStatusSource: "disconnect",
      hiddenSinceMs: 0,
      nowMs,
      offlineDelayMs
    })).toBe("sync_only")
  })

  it("recovers disconnected sessions that were hidden beyond the grace window", () => {
    expect(resolveVisibilityRecoveryAction({
      presenceStatus: "offline",
      presenceStatusSource: "disconnect",
      hiddenSinceMs: nowMs - 15 * 60_000,
      nowMs,
      offlineDelayMs
    })).toBe("online_then_rewards")
  })
})
