import { describe, expect, it } from "vitest"
import {
  FISHING_POND_CAPABILITY,
  OWNER_FISHING_POND_CAPABILITIES,
  restrictFishingPondCapabilitiesToReadOnly
} from "../src/services/fishing-pond-access"

describe("public fishing pond capabilities", () => {
  it("keeps supported presentation actions and removes every mutation", () => {
    const result = restrictFishingPondCapabilitiesToReadOnly([
      FISHING_POND_CAPABILITY.trackFish,
      FISHING_POND_CAPABILITY.inspectVitals,
      FISHING_POND_CAPABILITY.feed,
      "pond.remove_fish",
      FISHING_POND_CAPABILITY.throwStone,
      FISHING_POND_CAPABILITY.trackFish
    ])

    expect(result).toEqual([
      FISHING_POND_CAPABILITY.trackFish,
      FISHING_POND_CAPABILITY.throwStone
    ])
    expect(result).not.toContain(FISHING_POND_CAPABILITY.inspectVitals)
    expect(result).not.toContain(FISHING_POND_CAPABILITY.feed)
  })

  it("keeps owner defaults feature complete without using them for visitors", () => {
    expect(OWNER_FISHING_POND_CAPABILITIES).toEqual([
      FISHING_POND_CAPABILITY.inspectFish,
      FISHING_POND_CAPABILITY.inspectVitals,
      FISHING_POND_CAPABILITY.trackFish,
      FISHING_POND_CAPABILITY.throwStone,
      FISHING_POND_CAPABILITY.skipStone,
      FISHING_POND_CAPABILITY.feed
    ])
    expect(restrictFishingPondCapabilitiesToReadOnly([])).toEqual([])
  })
})
