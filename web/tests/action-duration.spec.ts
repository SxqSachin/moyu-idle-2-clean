import { describe, expect, it } from "vitest"

import {
  buildActionEfficiencyIndex,
  resolveActionDurationSeconds,
  resolveActionEfficiency
} from "@/services/action-duration"
import { formatCountdownMillisecondsOneDecimal, formatDurationSecondsOneDecimal } from "@/utils/duration-format"

describe("action duration", () => {
  it("builds efficiency index from related action protocol", () => {
    const index = buildActionEfficiencyIndex([
      { id: "collecting_speed", relatedActionMainType: "collecting", relatedActionEffect: "actionEfficiency" },
      { id: "collecting_speed_legacy", relatedActionMainType: "collecting", relatedActionEffect: "" },
      { id: "collecting_other", relatedActionMainType: "collecting", relatedActionEffect: "otherEffect" },
      { id: "mining_speed", relatedActionMainType: "mining", relatedActionEffect: "actionEfficiency" },
      { id: "cooking_speed", relatedActionMainType: "cooking", relatedActionEffect: "actionEfficiency" }
    ])
    expect(index.get("collecting")).toEqual(["collecting_speed", "collecting_speed_legacy"])
    expect(index.get("mining")).toEqual(["mining_speed"])
    expect(index.get("cooking")).toEqual(["cooking_speed"])
  })

  it("applies split formulas for positive and negative efficiency", () => {
    const playerFullEfficiency = resolveActionDurationSeconds(44, "collecting", "", "", "player", { collecting_speed: 1 })
    expect(playerFullEfficiency).toBe(22)

    const playerDoubleEfficiency = resolveActionDurationSeconds(44, "collecting", "", "", "player", { collecting_speed: 2 })
    expect(playerDoubleEfficiency).toBe(14.666)

    const playerCookingEfficiency = resolveActionDurationSeconds(60, "cooking", "", "", "player", { cooking_speed: 0.5 })
    expect(playerCookingEfficiency).toBe(40)

    const kittyFastDuration = resolveActionDurationSeconds(16, "collecting", "", "", "kitty", { collecting_speed: 99 })
    expect(kittyFastDuration).toBe(8)

    const playerSlowDuration = resolveActionDurationSeconds(44, "collecting", "", "", "player", { collecting_speed: -0.5 })
    expect(playerSlowDuration).toBe(66)

    const playerMinusHundredDuration = resolveActionDurationSeconds(44, "collecting", "", "", "player", { collecting_speed: -1 })
    expect(playerMinusHundredDuration).toBe(88)
  })

  it("returns 0 efficiency for unknown mainType", () => {
    expect(resolveActionEfficiency("unknown_main_type", { collecting_speed: 999 })).toBe(0)
  })
  it('adds training before the efficiency formula, preserves floors and excludes fishing/player', () => {
    const expected = [36,32,28,24,20,18.518,17.241,16.129,15.151,14.285]
    for (let level = 1; level <= 10; level++) {
      const add = level <= 5 ? level * 0.2 : 1 + (level-5)*0.08
      expect(resolveActionDurationSeconds(20,'collecting','','','kitty',{collecting_speed:-1},add)).toBeCloseTo(expected[level-1], 3)
    }
    expect(resolveActionDurationSeconds(20,'collecting','','','kitty',{collecting_speed:-0.7},0.6)).toBe(22)
    expect(resolveActionDurationSeconds(20,'fishing','','','kitty',{},1.4)).toBe(20)
    expect(resolveActionDurationSeconds(20,'collecting','','','player',{},1.4)).toBe(20)
    expect(resolveActionDurationSeconds(1,'collecting','','','kitty',{},1.4)).toBe(2)
  })
  it("formats seconds with one decimal", () => {
    const t = (key: string, values?: Record<string, unknown>) => `${values?.value}${key.endsWith("Second") ? "s" : ""}`
    expect(formatDurationSecondsOneDecimal(3.44, t)).toBe("3.4s")
  })

  it("does not display zero while countdown time remains", () => {
    const t = (key: string, values?: Record<string, unknown>) => `${values?.value}${key.endsWith("Second") ? "s" : ""}`
    expect(formatCountdownMillisecondsOneDecimal(1, t)).toBe("0.1s")
    expect(formatCountdownMillisecondsOneDecimal(100, t)).toBe("0.1s")
    expect(formatCountdownMillisecondsOneDecimal(101, t)).toBe("0.2s")
    expect(formatCountdownMillisecondsOneDecimal(1_000, t)).toBe("1.0s")
    expect(formatCountdownMillisecondsOneDecimal(0, t)).toBe("0.0s")
    expect(formatCountdownMillisecondsOneDecimal(-1, t)).toBe("0.0s")
    expect(formatCountdownMillisecondsOneDecimal(Number.NaN, t)).toBe("0.0s")
  })
})
