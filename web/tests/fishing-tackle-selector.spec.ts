import { describe, expect, it } from "vitest"

import {
  listCompatibleFishingTackleItemIds,
  type FishingTackleDefinition
} from "@/services/fishing"

describe("fishing tackle selector", () => {
  const tackle: FishingTackleDefinition[] = [
    { itemId: "reed_float", slot: "float_or_sinker", methods: ["float"] },
    { itemId: "slip_float", slot: "float_or_sinker", methods: ["float"] },
    { itemId: "river_sinker", slot: "float_or_sinker", methods: ["bottom"] },
    { itemId: "clear_leader", slot: "leader", methods: ["float", "bottom", "lure"] },
    { itemId: "reed_float", slot: "float_or_sinker", methods: ["float"] }
  ]

  it("lists every compatible template once in catalog order", () => {
    expect(listCompatibleFishingTackleItemIds(tackle, "float_or_sinker", "float")).toEqual([
      "reed_float",
      "slip_float"
    ])
  })

  it("excludes tackle for another method or slot", () => {
    expect(listCompatibleFishingTackleItemIds(tackle, "float_or_sinker", "bottom")).toEqual(["river_sinker"])
    expect(listCompatibleFishingTackleItemIds(tackle, "leader", "lure")).toEqual(["clear_leader"])
  })

  it("returns an empty reference list when no field is active", () => {
    expect(listCompatibleFishingTackleItemIds(tackle, "", "float")).toEqual([])
  })
})
