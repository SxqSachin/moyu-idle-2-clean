import { describe, expect, it } from "vitest"
import { fishingSceneAsset, fishingSceneEnvironment } from "@services/fishing-scene"

describe("fishing scene presentation", () => {
  it.each([
    [0, "night"], [4.99, "night"], [5, "dawn"], [8.99, "dawn"],
    [9, "day"], [16.99, "day"], [17, "dusk"], [20.99, "dusk"], [21, "night"], [23.99, "night"]
  ])("uses server game hour %s for %s", (gameHour, period) => {
    expect(fishingSceneEnvironment({ gameHour: Number(gameHour), weather: "sunny" }).period).toBe(period)
  })

  it("does not invent a time or celestial body when conditions are unavailable", () => {
    expect(fishingSceneEnvironment()).toEqual({ period: "unknown", weather: "unknown", celestial: null, precipitation: null })
    for (const gameHour of [NaN, Infinity, -1, 24]) {
      expect(fishingSceneEnvironment({ gameHour, weather: "clear" })).toMatchObject({ period: "unknown", celestial: null })
    }
  })

  it("keeps weather separate from time and hides celestial bodies under adverse weather", () => {
    for (const gameHour of [0, 6, 12, 18]) {
      for (const weather of ["cloud", "rain", "storm", "mist", "snow", "unsupported"]) {
        expect(fishingSceneEnvironment({ gameHour, weather }).celestial).toBeNull()
      }
    }
    expect(fishingSceneEnvironment({ gameHour: 23, weather: "sunny" })).toMatchObject({ celestial: "moon", weather: "clear" })
    expect(fishingSceneEnvironment({ gameHour: 12, weather: "clear" })).toMatchObject({ celestial: "sun", precipitation: null })
  })

  it.each([["rain", "rain"], ["storm", "rain"], ["snow", "snow"], ["mist", null], ["cloud", null]])("renders %s precipitation as %s", (weather, precipitation) => {
    expect(fishingSceneEnvironment({ gameHour: 12, weather: weather! }).precipitation).toBe(precipitation)
  })

  it("selects only the known local water assets", () => {
    expect(fishingSceneAsset("sunmere_shallows")).toContain("fishing-sunmere-v1.webp")
    expect(fishingSceneAsset("willowrun_valley")).toContain("fishing-willowrun-v1.webp")
    expect(fishingSceneAsset("frostglass_lake")).toContain("fishing-frostglass-v1.webp")
    for (const id of [undefined, "new-water", "constructor", "__proto__"]) {
      expect(fishingSceneAsset(id)).toBe(fishingSceneAsset("sunmere_shallows"))
    }
  })
})
